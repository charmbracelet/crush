package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"charm.land/fantasy"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/subagents"
)

// cancelAllBudget bounds how long CancelAll waits for background subagents
// to stop and write their reports.
const cancelAllBudget = 5 * time.Second

// reportWriteTimeout bounds a report write that runs detached from any
// caller.
const reportWriteTimeout = 5 * time.Second

// errShuttingDown is returned when background work is requested after
// CancelAll started.
var errShuttingDown = errors.New("crush is shutting down")

// AgentBackgroundMetadata is attached to the agent tool's response when the
// subagent was started in the background, so the UI can tell it apart.
type AgentBackgroundMetadata struct {
	Name       string `json:"name"`
	Background bool   `json:"background"`
}

// backgroundAgent is one subagent running (or finished) in the background.
// Always handled by pointer: it holds a mutex.
type backgroundAgent struct {
	name            string
	parentSessionID string
	childSessionID  string
	agent           SessionAgent
	params          subAgentParams
	// channel is the channel of the context that dispatched the subagent. It
	// is set once before the first launch and never changed.
	channel string

	// mu guards the fields below.
	mu sync.Mutex
	// running is cleared only after runSubAgentTurn has returned, so turns
	// on one child session never overlap.
	running bool
	// billed is the child cost already added to the parent session.
	billed float64
	// cancel stops the current turn.
	cancel context.CancelFunc
	// active is closed once the current turn is registered as running on
	// the child session (or the turn ended). done is closed when it ends.
	active, done chan struct{}
}

// backgroundRegistry tracks background subagents by parent session and name.
// The zero value is ready to use.
type backgroundRegistry struct {
	mu       sync.Mutex
	agents   map[string]*backgroundAgent
	counters map[string]int
}

func backgroundKey(parentSessionID, suffix string) string {
	return parentSessionID + "\x00" + suffix
}

// add registers a new entry named after typ and a per-parent counter, such
// as "task-1".
func (r *backgroundRegistry) add(parentSessionID, typ string, params subAgentParams, childSessionID string) *backgroundAgent {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.agents == nil {
		r.agents = make(map[string]*backgroundAgent)
		r.counters = make(map[string]int)
	}
	counterKey := backgroundKey(parentSessionID, typ)
	r.counters[counterKey]++
	entry := &backgroundAgent{
		name:            fmt.Sprintf("%s-%d", typ, r.counters[counterKey]),
		parentSessionID: parentSessionID,
		childSessionID:  childSessionID,
		agent:           params.Agent,
		params:          params,
	}
	r.agents[backgroundKey(parentSessionID, entry.name)] = entry
	return entry
}

func (r *backgroundRegistry) get(parentSessionID, name string) (*backgroundAgent, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.agents[backgroundKey(parentSessionID, name)]
	return entry, ok
}

// names returns the sorted names of the parent session's entries.
func (r *backgroundRegistry) names(parentSessionID string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var names []string
	for _, entry := range r.agents {
		if entry.parentSessionID == parentSessionID {
			names = append(names, entry.name)
		}
	}
	slices.Sort(names)
	return names
}

// backgroundContextLocked returns the context that is cancelled when the
// coordinator shuts down. Callers hold backgroundMu.
func (c *coordinator) backgroundContextLocked() context.Context {
	if c.backgroundCtx == nil {
		c.backgroundCtx, c.cancelBackground = context.WithCancel(context.Background())
	}
	return c.backgroundCtx
}

func (c *coordinator) backgroundContext() context.Context {
	c.backgroundMu.Lock()
	defer c.backgroundMu.Unlock()
	return c.backgroundContextLocked()
}

func (c *coordinator) shuttingDown() bool {
	return c.backgroundContext().Err() != nil
}

// beginBackground reserves a slot in backgroundWg. It reports false once
// shutdown started. Add happens under backgroundMu, which CancelAll also
// holds when it cancels, so no Add can follow the wait.
func (c *coordinator) beginBackground() bool {
	c.backgroundMu.Lock()
	defer c.backgroundMu.Unlock()
	if c.backgroundContextLocked().Err() != nil {
		return false
	}
	c.backgroundWg.Add(1)
	return true
}

// startBackground creates the child session and runs the subagent without
// blocking the caller. The report arrives later as a message.
func (c *coordinator) startBackground(ctx context.Context, params subAgentParams) (fantasy.ToolResponse, error) {
	if !c.beginBackground() {
		return fantasy.NewTextErrorResponse("Crush is shutting down"), nil
	}
	session, err := c.startSubAgentSession(ctx, params)
	if err != nil {
		c.backgroundWg.Done()
		return fantasy.ToolResponse{}, err
	}
	entry := c.background.add(params.SessionID, params.AgentName, params, session.ID)
	entry.mu.Lock()
	entry.channel = ChannelFromContext(ctx)
	c.launch(ctx, entry, params.Prompt)
	entry.mu.Unlock()

	text := fmt.Sprintf(
		"Started in the background as %q. Its report arrives in a later message when it finishes. "+
			"Do not edit files it may be changing. "+
			"Use %s to steer or follow up, %s to stop it.",
		entry.name, AgentSendToolName, AgentStopToolName,
	)
	return fantasy.WithResponseMetadata(
		fantasy.NewTextResponse(text),
		AgentBackgroundMetadata{Name: entry.name, Background: true},
	), nil
}

// launch runs one turn of entry's child session in a goroutine. The caller
// holds entry.mu (or owns entry exclusively) and has reserved a slot with
// beginBackground. The turn keeps ctx's values but not its cancellation, so
// cancelling the parent turn leaves the subagent running.
func (c *coordinator) launch(ctx context.Context, entry *backgroundAgent, prompt string) {
	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	active := make(chan struct{})
	done := make(chan struct{})
	var once sync.Once
	markActive := func() { once.Do(func() { close(active) }) }

	entry.running = true
	entry.cancel = cancel
	entry.active, entry.done = active, done
	// Registered here as well as in runSubAgentTurn, so a stop right after
	// start finds the run before its goroutine is scheduled.
	if c.subagentCancels != nil {
		c.subagentCancels.Set(entry.childSessionID, cancel)
	}
	billed := entry.billed

	go func() {
		defer c.backgroundWg.Done()
		defer cancel()
		window := &turnWindow{startedAt: time.Now()}
		if snap, ok := takeWorktreeSnapshot(runCtx, c.cfg.WorkingDir()); ok {
			window.before = snap
		}
		turn := c.runSubAgentTurn(runCtx, subAgentTurnArgs{
			params:   entry.params,
			childID:  entry.childSessionID,
			prompt:   prompt,
			billed:   billed,
			onActive: markActive,
		})
		entry.mu.Lock()
		entry.billed = turn.billed
		entry.running = false
		markActive()
		close(done)
		entry.mu.Unlock()
		c.deliverReport(entry, turn.status, turn.resp.Content, window)
	}()
}

// deliverReport hands a finished turn's result to the parent session. A
// cancelled turn or a shutdown only records the report; anything else also
// starts a parent turn when the parent is idle, or queues the report when it
// is busy. window, when its snapshot is set, adds the files changed outside
// the child's edit tools.
func (c *coordinator) deliverReport(entry *backgroundAgent, status, output string, window *turnWindow) {
	report := message.SubagentReport{Name: entry.name, Status: status, Output: output}
	if status == subagents.StatusCancelled && c.shuttingDown() {
		report.Output = "Stopped: Crush exited"
	} else {
		// The turn context may already be cancelled, so detach from it.
		ctx, cancel := context.WithTimeout(context.WithoutCancel(c.backgroundContext()), reportWriteTimeout)
		report.Output += c.changedFilesNote(ctx, entry.childSessionID)
		if window != nil && window.before != nil {
			if after, ok := takeWorktreeSnapshot(ctx, c.cfg.WorkingDir()); ok {
				changed := changedBetween(ctx, window.before, after)
				report.Output += c.otherChangesNote(ctx, changed, entry.childSessionID, entry.parentSessionID, window.startedAt)
			}
		}
		cancel()
	}
	if status == subagents.StatusCancelled || c.shuttingDown() {
		c.writeReport(entry.parentSessionID, report)
		return
	}
	// The report turn would otherwise run without a channel, which clears the
	// parent's channel binding and drops the reply to the dispatching channel.
	ctx := WithChannel(c.backgroundContext(), entry.channel)
	if _, err := c.runCall(ctx, nil, entry.parentSessionID, report.PromptText(), &report, nil); err != nil {
		slog.Warn(
			"Failed to deliver subagent report",
			"subagent", entry.name,
			"parent_session", entry.parentSessionID,
			"error", err,
		)
		// A failure before the report message exists would drop the only
		// record of the subagent's work, so store it without a turn.
		if !c.reportStored(entry.parentSessionID, report) {
			c.writeReport(entry.parentSessionID, report)
		}
	}
}

// reportStored reports whether the parent session already holds report as a
// message. A failed report turn may have stored it before failing, and
// writing it again would show the parent the same report twice.
func (c *coordinator) reportStored(parentSessionID string, report message.SubagentReport) bool {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.backgroundContext()), reportWriteTimeout)
	defer cancel()
	msgs, err := c.messages.List(ctx, parentSessionID)
	if err != nil {
		slog.Warn("Failed to check for stored subagent report", "subagent", report.Name, "parent_session", parentSessionID, "error", err)
		return false
	}
	return slices.ContainsFunc(msgs, func(m message.Message) bool {
		stored, ok := m.SubagentReport()
		return ok && stored == report
	})
}

// changedFilesNote lists the files the child session touched, relative to
// the working directory when possible, or returns "" when there are none.
func (c *coordinator) changedFilesNote(ctx context.Context, childSessionID string) string {
	if c.history == nil {
		return ""
	}
	files, err := c.history.ListLatestSessionFiles(ctx, childSessionID)
	if err != nil {
		slog.Warn("Failed to list files changed by subagent", "session", childSessionID, "error", err)
		return ""
	}
	workingDir := c.cfg.WorkingDir()
	paths := make([]string, 0, len(files))
	for _, f := range files {
		paths = append(paths, displayPath(workingDir, f.Path))
	}
	slices.Sort(paths)
	paths = slices.Compact(paths)
	if len(paths) == 0 {
		return ""
	}
	return "\n\nFiles changed by this subagent so far:\n- " + strings.Join(paths, "\n- ")
}

// displayPath shows path relative to dir when it is inside dir, and
// unchanged otherwise.
func displayPath(dir, path string) string {
	rel, err := filepath.Rel(dir, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return path
	}
	return rel
}

// canonicalPath resolves symlinks in path. A path that does not exist, such
// as a deleted file, keeps its missing tail and resolves the part that does.
func canonicalPath(path string) string {
	path = filepath.Clean(path)
	missing := ""
	for dir := path; ; {
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			return filepath.Join(resolved, missing)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return path
		}
		missing = filepath.Join(filepath.Base(dir), missing)
		dir = parent
	}
}

// maxOtherChanges caps how many paths otherChangesNote lists.
const maxOtherChanges = 100

// otherChangesNote lists the changed files that neither the child session
// nor the parent session (since the turn started) wrote through their edit
// tools, or returns "" when none remain.
func (c *coordinator) otherChangesNote(ctx context.Context, changed []string, childSessionID, parentSessionID string, since time.Time) string {
	if c.history != nil {
		known := map[string]bool{}
		if files, err := c.history.ListLatestSessionFiles(ctx, childSessionID); err != nil {
			slog.Warn("Failed to list files changed by subagent", "session", childSessionID, "error", err)
		} else {
			for _, f := range files {
				known[canonicalPath(f.Path)] = true
			}
		}
		if files, err := c.history.ListLatestSessionFiles(ctx, parentSessionID); err != nil {
			slog.Warn("Failed to list files changed by parent session", "session", parentSessionID, "error", err)
		} else {
			for _, f := range files {
				if f.UpdatedAt >= since.Unix() {
					known[canonicalPath(f.Path)] = true
				}
			}
		}
		changed = slices.DeleteFunc(slices.Clone(changed), func(path string) bool {
			return known[canonicalPath(path)]
		})
	}
	if len(changed) == 0 {
		return ""
	}
	workingDir := canonicalPath(c.cfg.WorkingDir())
	paths := make([]string, 0, len(changed))
	for _, path := range changed {
		paths = append(paths, displayPath(workingDir, canonicalPath(path)))
	}
	// Files inside the working directory come first, then absolute paths.
	slices.SortFunc(paths, func(a, b string) int {
		if aAbs, bAbs := filepath.IsAbs(a), filepath.IsAbs(b); aAbs != bAbs {
			if aAbs {
				return 1
			}
			return -1
		}
		return strings.Compare(a, b)
	})
	paths = slices.Compact(paths)
	more := 0
	if len(paths) > maxOtherChanges {
		more = len(paths) - maxOtherChanges
		paths = paths[:maxOtherChanges]
	}
	note := "\n\nOther files changed while it ran (bash, you, or the main agent):\n- " + strings.Join(paths, "\n- ")
	if more > 0 {
		note += fmt.Sprintf("\n- ... and %d more", more)
	}
	return note
}

// writeReport stores report as a user message without starting a turn.
func (c *coordinator) writeReport(parentSessionID string, report message.SubagentReport) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.backgroundContext()), reportWriteTimeout)
	defer cancel()
	_, err := c.messages.Create(ctx, parentSessionID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{report},
	})
	if err != nil {
		slog.Warn(
			"Failed to write subagent report",
			"subagent", report.Name,
			"parent_session", parentSessionID,
			"error", err,
		)
	}
}

// unknownBackgroundText names the valid subagents when name matches none.
func (c *coordinator) unknownBackgroundText(parentSessionID, name string) string {
	names := c.background.names(parentSessionID)
	if len(names) == 0 {
		return fmt.Sprintf("No background subagent named %q. This session has none.", name)
	}
	return fmt.Sprintf("No background subagent named %q. This session has: %s.", name, strings.Join(names, ", "))
}

// sendToBackground steers a running background subagent or, when it has
// finished, reopens its child session with text as the new prompt.
func (c *coordinator) sendToBackground(ctx context.Context, parentSessionID, name, text string) (string, error) {
	entry, ok := c.background.get(parentSessionID, name)
	if !ok {
		return "", errors.New(c.unknownBackgroundText(parentSessionID, name))
	}
	for {
		entry.mu.Lock()
		if !entry.running {
			if !c.beginBackground() {
				entry.mu.Unlock()
				return "", errShuttingDown
			}
			c.launch(ctx, entry, text)
			entry.mu.Unlock()
			return fmt.Sprintf("Reopened %q with your message. Its report arrives in a later message when it finishes.", name), nil
		}
		active, done := entry.active, entry.done
		entry.mu.Unlock()

		// Queue only once the turn is registered as running on the child
		// session, or the call would find it idle.
		select {
		case <-active:
		case <-done:
		case <-ctx.Done():
			return "", ctx.Err()
		}
		call, _, _, ok := c.subAgentCall(entry.agent, entry.childSessionID, text)
		if !ok {
			return "", fmt.Errorf("send to %s: %w", name, errModelProviderNotConfigured)
		}
		call.QueueOnly = true
		_, err := entry.agent.Run(ctx, call)
		switch {
		case err == nil:
			return fmt.Sprintf("Delivered to %q. It will read your message at its next step.", name), nil
		case errors.Is(err, ErrSessionIdle):
			// The turn ended between the check and the call. Wait for the
			// bookkeeping to settle, then reopen.
			select {
			case <-done:
			case <-ctx.Done():
				return "", ctx.Err()
			}
		default:
			return "", fmt.Errorf("send to %s: %w", name, err)
		}
	}
}

// stopBackground cancels a running background subagent. Its cancelled report
// follows once the turn ends.
func (c *coordinator) stopBackground(parentSessionID, name string) string {
	entry, ok := c.background.get(parentSessionID, name)
	if !ok {
		return c.unknownBackgroundText(parentSessionID, name)
	}
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if !entry.running {
		return fmt.Sprintf("%q already finished.", name)
	}
	entry.cancel()
	return fmt.Sprintf("Stopping %s. Its report arrives when it stops.", name)
}

// stopBackgroundWork cancels the background context and the running
// subagents, then waits for them to write their reports within what is left
// of cancelAllBudget since start.
func (c *coordinator) stopBackgroundWork(start time.Time) {
	c.backgroundMu.Lock()
	c.backgroundContextLocked()
	c.cancelBackground()
	c.backgroundMu.Unlock()

	if c.subagentCancels != nil {
		for cancel := range c.subagentCancels.Seq() {
			if cancel != nil {
				cancel()
			}
		}
	}

	settled := make(chan struct{})
	go func() {
		c.backgroundWg.Wait()
		close(settled)
	}()
	select {
	case <-settled:
	case <-time.After(max(0, cancelAllBudget-time.Since(start))):
	}
}
