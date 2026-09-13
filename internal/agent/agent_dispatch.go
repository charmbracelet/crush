package agent

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"charm.land/fantasy"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
)

//go:embed templates/agent_dispatch_tool.md
var agentDispatchToolDescription string

//go:embed templates/agent_send_tool.md
var agentSendToolDescription string

//go:embed templates/agent_stop_tool.md
var agentStopToolDescription string

const (
	AgentDispatchToolName = "agent_dispatch"
	AgentSendToolName     = "agent_send"
	AgentStopToolName     = "agent_stop"
)

// stopForShutdown is the stop reason used when Crush itself is quitting.
// Nobody is left to read a report then, so none is written; the next run on
// the parent session finds the dispatch unanswered and reports it as
// interrupted instead.
const stopForShutdown = "Crush is shutting down"

type AgentDispatchParams struct {
	Prompt string `json:"prompt" description:"The task for the sub-agent to perform"`
	Label  string `json:"label" description:"A short name for this sub-agent, used to address it and to label its report"`
	Access string `json:"access,omitempty" description:"\"read\" (default) for a sub-agent that can only inspect the codebase, or \"write\" for one that can also edit files"`
}

// AgentSendResponseMetadata tells the UI which sub-agent a message went to,
// and whether it restarted a finished one, so the transcript can show that
// sub-agent as working again.
type AgentSendResponseMetadata struct {
	Label    string `json:"label"`
	Reopened bool   `json:"reopened,omitempty"`
}

type AgentSendParams struct {
	Label   string `json:"label" description:"The label of the running sub-agent to send to"`
	Message string `json:"message" description:"The message to deliver to that sub-agent"`
}

type AgentStopParams struct {
	Label  string `json:"label" description:"The label of the running sub-agent to stop"`
	Reason string `json:"reason,omitempty" description:"Why it is being stopped; recorded in its report"`
}

// dispatchState is where a dispatched sub-agent is in its life.
type dispatchState int

const (
	dispatchRunning dispatchState = iota
	dispatchFinished
	dispatchFailed
	dispatchStopped
)

func (s dispatchState) String() string {
	switch s {
	case dispatchRunning:
		return "running"
	case dispatchFinished:
		return "finished"
	case dispatchFailed:
		return "failed"
	case dispatchStopped:
		return "stopped"
	}
	return "unknown"
}

// dispatchedAgent tracks one detached sub-agent across all of its runs: the
// first one, and every follow-up that reopens it.
type dispatchedAgent struct {
	Label           string
	SessionID       string
	ParentSessionID string
	Access          string
	// agent is the SessionAgent instance actually running this sub-agent.
	// Busy state and the message queue live on that instance, not on the
	// coordinator's main agent, so a message meant for a sub-agent has to
	// be addressed to it directly.
	agent SessionAgent

	mu sync.Mutex
	// gen numbers the runs. Each run settles only its own generation, so a
	// run that is slow to unwind cannot mark a newer run as finished.
	gen        uint64
	state      dispatchState
	cancel     context.CancelFunc
	stopReason string
	started    time.Time
	ended      time.Time

	// rolledUpCost is the child session's cumulative cost already added to
	// the parent. Each run rolls up only what it spent since, since the
	// child's own total is cumulative across every turn.
	rolledUpCost atomic.Uint64
}

// newDispatchedAgent returns an entry for a sub-agent that is not running.
// Callers start it with claim.
func newDispatchedAgent(label, access, sessionID, parentSessionID string, agent SessionAgent) *dispatchedAgent {
	return &dispatchedAgent{
		Label:           label,
		Access:          access,
		SessionID:       sessionID,
		ParentSessionID: parentSessionID,
		agent:           agent,
		state:           dispatchFinished,
	}
}

// claim starts a new run if the sub-agent is idle. The returned context is
// detached from whatever turn asked for the run: a sub-agent outlives the
// turn that started it and is stopped only through stop.
func (d *dispatchedAgent) claim() (context.Context, uint64, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.state == dispatchRunning {
		return nil, 0, false
	}
	ctx, cancel := context.WithCancel(context.Background())
	d.gen++
	d.state = dispatchRunning
	d.cancel = cancel
	d.stopReason = ""
	d.started = time.Now()
	d.ended = time.Time{}
	return ctx, d.gen, true
}

// settle records how run gen ended, and returns the stop reason if it was
// stopped. A stale generation is ignored.
func (d *dispatchedAgent) settle(gen uint64, failed bool) (stopReason string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if gen != d.gen {
		return ""
	}
	switch {
	case d.stopReason != "":
		d.state = dispatchStopped
	case failed:
		d.state = dispatchFailed
	default:
		d.state = dispatchFinished
	}
	d.ended = time.Now()
	d.cancel()
	return d.stopReason
}

// stop cancels the run in flight. It returns false if nothing is running.
// The entry stays running until the run has actually unwound.
func (d *dispatchedAgent) stop(reason string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.state != dispatchRunning {
		return false
	}
	if d.stopReason == "" {
		d.stopReason = reason
	}
	d.cancel()
	return true
}

func (d *dispatchedAgent) running() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.state == dispatchRunning
}

// stopping reports whether a stop was requested for a run still unwinding.
func (d *dispatchedAgent) stopping() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.state == dispatchRunning && d.stopReason != ""
}

// dispatchSnapshot is a consistent read of an entry's lifecycle fields.
type dispatchSnapshot struct {
	State      dispatchState
	StopReason string
	Started    time.Time
	Ended      time.Time
}

func (d *dispatchedAgent) snapshot() dispatchSnapshot {
	d.mu.Lock()
	defer d.mu.Unlock()
	return dispatchSnapshot{
		State:      d.state,
		StopReason: d.stopReason,
		Started:    d.started,
		Ended:      d.ended,
	}
}

// awaitSettled waits until the run in flight has unwound, or the timeout
// passes. It reports whether the sub-agent is idle.
func (d *dispatchedAgent) awaitSettled(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for d.running() {
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(25 * time.Millisecond)
	}
	return true
}

// awaitDeliverable waits until the sub-agent's session is actually running
// and can accept a message, and reports whether it can. A run is claimed
// before its goroutine has started the session, so a message sent right
// after dispatching arrives while the session is not yet busy; treating that
// window as "finished" would drop the message. Waiting closes the gap
// without blocking indefinitely on a sub-agent that died on startup.
func (d *dispatchedAgent) awaitDeliverable(ctx context.Context) bool {
	const (
		pollEvery = 50 * time.Millisecond
		giveUp    = 10 * time.Second
	)
	deadline := time.NewTimer(giveUp)
	defer deadline.Stop()
	ticker := time.NewTicker(pollEvery)
	defer ticker.Stop()

	for {
		if !d.running() || d.stopping() {
			return false
		}
		if d.agent.IsSessionBusy(d.SessionID) {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-deadline.C:
			return false
		case <-ticker.C:
		}
	}
}

// dispatchKey namespaces a label to its parent session, so two sessions can
// use the same label without colliding.
func dispatchKey(parentSessionID, label string) string {
	return parentSessionID + "\x00" + label
}

// agentDispatchTool launches a sub-agent that runs detached from this tool
// call. The tool returns as soon as the sub-agent starts; the result is
// delivered later as a message into the parent session.
func (c *coordinator) agentDispatchTool(ctx context.Context, parentAgentID string) (fantasy.AgentTool, error) {
	readOnly := readOnlyParent(parentAgentID)
	agents, err := c.taskAgents(ctx, readOnly)
	if err != nil {
		return nil, err
	}

	return fantasy.NewParallelAgentTool(
		AgentDispatchToolName,
		agentDispatchToolDescription,
		func(ctx context.Context, params AgentDispatchParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Prompt == "" {
				return fantasy.NewTextErrorResponse("prompt is required"), nil
			}
			label := strings.TrimSpace(params.Label)
			if label == "" {
				return fantasy.NewTextErrorResponse("label is required"), nil
			}

			agent, refusal := resolveSubAgent(agents, params.Access, readOnly)
			if refusal != "" {
				return fantasy.NewTextErrorResponse(refusal), nil
			}
			access := params.Access
			if access == "" {
				access = accessRead
			}

			parentSessionID := tools.GetSessionFromContext(ctx)
			if parentSessionID == "" {
				return fantasy.ToolResponse{}, errors.New("session id missing from context")
			}
			agentMessageID := tools.GetMessageFromContext(ctx)
			if agentMessageID == "" {
				return fantasy.ToolResponse{}, errors.New("agent message id missing from context")
			}

			// The sub-session ID derives from the bare tool call ID so the
			// UI can walk back from a child-session event to this tool's
			// chat item and nest the sub-agent's tool calls under it.
			toolCallID := call.ID
			entry := newDispatchedAgent(
				label,
				access,
				c.sessions.CreateAgentToolSessionID(agentMessageID, toolCallID),
				parentSessionID,
				agent,
			)
			runCtx, gen, _ := entry.claim()

			// Checking the label and registering the entry happen under
			// one lock, so two parallel dispatches cannot both take it.
			key := dispatchKey(parentSessionID, label)
			c.dispatchMu.Lock()
			if existing, ok := c.dispatched.Get(key); ok && existing.running() {
				c.dispatchMu.Unlock()
				entry.settle(gen, true)
				return fantasy.NewTextErrorResponse(fmt.Sprintf("a sub-agent labeled %q is already running; pick another label, or stop it with %s first", label, AgentStopToolName)), nil
			}
			c.dispatched.Set(key, entry)
			c.dispatchMu.Unlock()

			c.runDispatched(runCtx, entry, gen, func(ctx context.Context) (fantasy.ToolResponse, error) {
				return c.runSubAgent(ctx, subAgentParams{
					Agent:          agent,
					SessionID:      parentSessionID,
					AgentMessageID: agentMessageID,
					ToolCallID:     toolCallID,
					Prompt:         params.Prompt,
					SessionTitle:   "Sub-agent: " + label,
					SkipCostRollup: true,
				})
			})

			return fantasy.NewTextResponse(fmt.Sprintf(
				"Sub-agent %q started. It reports back on its own; carry on with other work rather than waiting for it. Use %s to check on it.",
				label, AgentStatusToolName,
			)), nil
		},
	), nil
}

// runDispatched runs one generation of a sub-agent in the background, then
// settles the entry and reports the outcome to the parent.
func (c *coordinator) runDispatched(ctx context.Context, entry *dispatchedAgent, gen uint64, run func(context.Context) (fantasy.ToolResponse, error)) {
	go func() {
		resp, err := run(ctx)
		c.rollUpDispatchCost(context.Background(), entry)
		// Settle before reporting: the report starts a parent turn, and
		// that turn may well send this sub-agent a follow-up, which needs
		// it idle.
		stopReason := entry.settle(gen, err != nil || resp.IsError)

		switch stopReason {
		case "":
			c.reportDispatchResult(entry, resp, err)
		case stopForShutdown:
		default:
			// A stopped sub-agent has nothing to report and its parent
			// already knows it was stopped, so record the outcome
			// without starting a turn over it.
			c.recordDispatchReport(entry.ParentSessionID, message.SubAgentReport{
				Label:  entry.Label,
				Output: "Stopped: " + stopReason + ". Its session is kept; agent_send resumes it with its work in context.",
				Failed: true,
			})
		}
	}()
}

// reopen runs another turn on a finished sub-agent's existing session, so it
// answers with its whole task still in context. This is what makes a
// sub-agent answerable rather than a one-shot: the session is real and its
// history is intact, so a follow-up is just the next turn on it.
//
// Returns false if the sub-agent is not idle, meaning somebody else claimed
// it first or it is still working.
func (c *coordinator) reopen(entry *dispatchedAgent, prompt string) bool {
	agentCall, _, _, err := c.subAgentCall(entry.agent, entry.SessionID, prompt)
	if err != nil {
		return false
	}
	// Claiming makes two concurrent follow-ups resolve to one reopen rather
	// than two turns racing on one session.
	runCtx, gen, ok := entry.claim()
	if !ok {
		return false
	}

	c.runDispatched(runCtx, entry, gen, func(ctx context.Context) (fantasy.ToolResponse, error) {
		done := c.trackSubSession(entry.SessionID)
		defer done()
		result, err := entry.agent.Run(ctx, agentCall)
		if err != nil {
			return fantasy.NewTextErrorResponse(fmt.Sprintf("Failed to generate response: %s", err)), nil
		}
		return fantasy.NewTextResponse(subAgentOutput(result)), nil
	})
	return true
}

// rollUpDispatchCost adds what a sub-agent spent since the last roll-up to
// its parent session.
func (c *coordinator) rollUpDispatchCost(ctx context.Context, entry *dispatchedAgent) {
	child, err := c.sessions.Get(ctx, entry.SessionID)
	if err != nil {
		return
	}
	previous := math.Float64frombits(entry.rolledUpCost.Load())
	delta := child.Cost - previous
	if delta <= 0 {
		return
	}
	entry.rolledUpCost.Store(math.Float64bits(child.Cost))

	parent, err := c.sessions.Get(ctx, entry.ParentSessionID)
	if err != nil {
		return
	}
	parent.Cost += delta
	if _, err := c.sessions.Save(ctx, parent); err != nil {
		slog.Warn("Failed to roll up sub-agent cost", "label", entry.Label, "error", err)
	}
}

// reportDispatchResult delivers a finished sub-agent's output back to the
// parent session. If the parent is mid-turn the message folds into it at the
// next step; if the parent is idle it starts a new turn. Both branches are
// already handled by sessionAgent.Run.
func (c *coordinator) reportDispatchResult(entry *dispatchedAgent, resp fantasy.ToolResponse, err error) {
	var (
		body   string
		failed bool
	)
	switch {
	case err != nil:
		body, failed = fmt.Sprintf("Sub-agent failed: %s", err), true
	case resp.Content == "":
		body, failed = "Sub-agent finished without producing any output.", true
	default:
		body, failed = resp.Content, resp.IsError
	}

	report := message.SubAgentReport{
		Label:  entry.Label,
		Output: body,
		Failed: failed,
	}
	if _, err := c.runWithParts(
		context.Background(),
		entry.ParentSessionID,
		reportPrompt(report),
		[]message.ContentPart{report},
	); err != nil {
		slog.Error(
			"Failed to deliver sub-agent report",
			"label", entry.Label,
			"parent_session", entry.ParentSessionID,
			"error", err,
		)
	}
}

// reportPrompt mirrors what a report part renders into for the model; the
// part is what the UI draws.
func reportPrompt(report message.SubAgentReport) string {
	return fmt.Sprintf("<sub-agent-report label=%q>\n%s\n</sub-agent-report>", report.Label, report.Output)
}

// recordDispatchReport stores a report in the parent session without starting
// a turn. The model reads it with the rest of the history on its next turn,
// and the UI settles the sub-agent's item when it lands.
func (c *coordinator) recordDispatchReport(parentSessionID string, report message.SubAgentReport) {
	if _, err := c.messages.Create(context.Background(), parentSessionID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{report},
	}); err != nil {
		slog.Error(
			"Failed to record sub-agent report",
			"label", report.Label,
			"parent_session", parentSessionID,
			"error", err,
		)
	}
}

// agentSendTool delivers a message to a running detached sub-agent. The
// message is queued on the sub-agent's session, where sessionAgent folds it
// into the sub-agent's turn at its next step.
func (c *coordinator) agentSendTool() fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(
		AgentSendToolName,
		agentSendToolDescription,
		func(ctx context.Context, params AgentSendParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			label := strings.TrimSpace(params.Label)
			if label == "" {
				return fantasy.NewTextErrorResponse("label is required"), nil
			}
			if params.Message == "" {
				return fantasy.NewTextErrorResponse("message is required"), nil
			}

			parentSessionID := tools.GetSessionFromContext(ctx)
			if parentSessionID == "" {
				return fantasy.ToolResponse{}, errors.New("session id missing from context")
			}

			reopened := func(entry *dispatchedAgent) fantasy.ToolResponse {
				if !c.reopen(entry, params.Message) {
					return fantasy.NewTextErrorResponse(fmt.Sprintf(
						"could not reach sub-agent %q. Dispatch a new one instead.", label,
					))
				}
				return fantasy.WithResponseMetadata(fantasy.NewTextResponse(fmt.Sprintf(
					"Sub-agent %q had finished, so it picked this up as a follow-up with its earlier work still in context. Its answer arrives as a new report.",
					label,
				)), AgentSendResponseMetadata{Label: label, Reopened: true})
			}

			entry, ok := c.dispatched.Get(dispatchKey(parentSessionID, label))
			if !ok {
				// Not in the registry, which after a restart is every
				// sub-agent. The transcript still records the dispatch and
				// the child session still holds the work, so rebuild the
				// entry from those rather than refusing.
				entry, err := c.rehydrateDispatch(ctx, parentSessionID, label)
				if err != nil {
					return fantasy.NewTextErrorResponse(fmt.Sprintf(
						"no sub-agent labeled %q. Running sub-agents: %s",
						label, c.runningDispatchLabels(parentSessionID),
					)), nil
				}
				return reopened(entry), nil
			}
			if entry.stopping() {
				// Wait out the stop rather than queueing onto a run that
				// is about to drop its queue.
				entry.awaitSettled(10 * time.Second)
			}
			if !entry.awaitDeliverable(ctx) {
				// Finished, not gone: its session still holds everything
				// it did, so a follow-up reopens it rather than failing.
				return reopened(entry), nil
			}

			// Addressed to the sub-agent's own SessionAgent and queued
			// without a RunID, so it folds into that sub-agent's active
			// turn rather than starting a turn of its own.
			agentCall, _, _, err := c.subAgentCall(entry.agent, entry.SessionID, params.Message)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("could not deliver the message: %s", err)), nil
			}
			if _, err := entry.agent.Run(context.WithoutCancel(ctx), agentCall); err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("could not deliver the message: %s", err)), nil
			}
			return fantasy.WithResponseMetadata(fantasy.NewTextResponse(fmt.Sprintf(
				"Delivered to %q. It picks the message up at its next step and folds the answer into its final report.",
				label,
			)), AgentSendResponseMetadata{Label: label}), nil
		},
	)
}

// agentStopTool stops a running detached sub-agent. Its session is kept, so a
// later agent_send resumes it with its work still in context.
func (c *coordinator) agentStopTool() fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(
		AgentStopToolName,
		agentStopToolDescription,
		func(ctx context.Context, params AgentStopParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			label := strings.TrimSpace(params.Label)
			if label == "" {
				return fantasy.NewTextErrorResponse("label is required"), nil
			}
			parentSessionID := tools.GetSessionFromContext(ctx)
			if parentSessionID == "" {
				return fantasy.ToolResponse{}, errors.New("session id missing from context")
			}

			entry, ok := c.dispatched.Get(dispatchKey(parentSessionID, label))
			if !ok || !entry.running() {
				return fantasy.NewTextErrorResponse(fmt.Sprintf(
					"sub-agent %q is not running. Running sub-agents: %s",
					label, c.runningDispatchLabels(parentSessionID),
				)), nil
			}
			reason := strings.TrimSpace(params.Reason)
			if reason == "" {
				reason = "stopped by the main agent"
			}
			entry.stop(reason)
			if !entry.awaitSettled(10 * time.Second) {
				return fantasy.NewTextResponse(fmt.Sprintf(
					"Asked sub-agent %q to stop; it is still winding down.", label,
				)), nil
			}
			return fantasy.NewTextResponse(fmt.Sprintf(
				"Stopped sub-agent %q. Its session is kept; agent_send resumes it with its work in context.", label,
			)), nil
		},
	)
}

// runningDispatchLabels lists the sub-agents still running for a session.
func (c *coordinator) runningDispatchLabels(parentSessionID string) string {
	var labels []string
	for _, entry := range c.dispatched.Seq2() {
		if entry.ParentSessionID == parentSessionID && entry.running() {
			labels = append(labels, entry.Label)
		}
	}
	if len(labels) == 0 {
		return "none"
	}
	sort.Strings(labels)
	return strings.Join(labels, ", ")
}

// rehydrateDispatch rebuilds a sub-agent's registry entry from what is on
// disk. The registry is in-memory, so a restart empties it, but nothing that
// matters was in memory to begin with: the parent's transcript records the
// dispatch call, the child session ID derives from that call's IDs, and the
// child session holds the whole conversation. The entry is reconstructed as a
// finished sub-agent, which is what it is, so the caller reopens it.
func (c *coordinator) rehydrateDispatch(ctx context.Context, parentSessionID, label string) (*dispatchedAgent, error) {
	messages, err := c.messages.List(ctx, parentSessionID)
	if err != nil {
		return nil, err
	}
	record, ok := dispatchHistory(messages).byLabel[label]
	if !ok {
		return nil, fmt.Errorf("no dispatch recorded for label %q", label)
	}

	// Rehydration replays the access level recorded in the transcript, so
	// both variants have to be available to look it up. A read-only parent
	// could never have recorded a write dispatch in the first place.
	agents, err := c.taskAgents(ctx, false)
	if err != nil {
		return nil, err
	}
	agent, ok := agents[record.Access]
	if !ok {
		return nil, fmt.Errorf("unknown access %q", record.Access)
	}

	sessionID := c.sessions.CreateAgentToolSessionID(record.MessageID, record.CallID)
	child, err := c.sessions.Get(ctx, sessionID)
	if err != nil {
		// The dispatch was recorded but its session is gone, so there is
		// no history to reopen into.
		return nil, err
	}

	entry := newDispatchedAgent(label, record.Access, sessionID, parentSessionID, agent)
	// Seed the roll-up with what the child has already cost. The parent was
	// billed for all of it during the original run, so starting from zero
	// would bill the whole total a second time.
	entry.rolledUpCost.Store(math.Float64bits(child.Cost))

	c.dispatchMu.Lock()
	defer c.dispatchMu.Unlock()
	if existing, ok := c.dispatched.Get(dispatchKey(parentSessionID, label)); ok {
		return existing, nil
	}
	c.dispatched.Set(dispatchKey(parentSessionID, label), entry)
	return entry, nil
}

// stopAllDispatched stops every running sub-agent and waits, up to timeout,
// for them to unwind.
func (c *coordinator) stopAllDispatched(reason string, timeout time.Duration) {
	var stopped []*dispatchedAgent
	for _, entry := range c.dispatched.Seq2() {
		if entry.stop(reason) {
			stopped = append(stopped, entry)
		}
	}
	deadline := time.Now().Add(timeout)
	for _, entry := range stopped {
		if !entry.awaitSettled(time.Until(deadline)) {
			slog.Warn("Sub-agent still running after being stopped", "label", entry.Label)
		}
	}
}

// dispatchedSessionBusy reports whether sessionID belongs to a dispatched
// sub-agent with a run in flight.
func (c *coordinator) dispatchedSessionBusy(sessionID string) bool {
	for _, entry := range c.dispatched.Seq2() {
		if entry.SessionID == sessionID && entry.running() {
			return true
		}
	}
	return false
}

// trackSubSession marks a sub-agent session as live until the returned func
// is called. The coordinator's busy check consults this, so a reader settling
// half-finished sessions never mistakes a working sub-agent for a dead one.
func (c *coordinator) trackSubSession(sessionID string) func() {
	c.subSessionsMu.Lock()
	if c.subSessions == nil {
		c.subSessions = map[string]int{}
	}
	c.subSessions[sessionID]++
	c.subSessionsMu.Unlock()
	return func() {
		c.subSessionsMu.Lock()
		defer c.subSessionsMu.Unlock()
		if c.subSessions[sessionID]--; c.subSessions[sessionID] <= 0 {
			delete(c.subSessions, sessionID)
		}
	}
}

func (c *coordinator) subSessionBusy(sessionID string) bool {
	c.subSessionsMu.Lock()
	defer c.subSessionsMu.Unlock()
	return c.subSessions[sessionID] > 0
}
