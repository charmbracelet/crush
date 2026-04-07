package agent

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/memory"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/oauth/copilot"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
)

const (
	memoryDreamMinHours            = 24
	memoryDreamMinSessions         = 5
	memoryDreamStaleLockWindow     = time.Hour
	memoryDreamMaxSessions         = 8
	memoryDreamMaxChars            = 24000
	memoryFreshnessWarnAfter       = 30 * 24 * time.Hour
	memoryDreamSessionScanInterval = 10 * time.Minute
)

var memoryDreamLastSessionScanUnix int64

const memoryDreamPrompt = `You are a memory consolidation agent. Review existing long-term memories and recent session transcripts, then synthesize durable knowledge that future sessions should retain.

Rules:
- Prefer updating or strengthening durable project knowledge over copying transcripts.
- Capture user preferences, stable workflows, project context, and repeated decisions.
- Avoid transient implementation details, temporary file state, and one-off logs.
- Reuse existing keys when refining the same memory; create new keys only when needed.
- Return JSON with array of memories: [{"key":"...","description":"...","content":"...","type":"...","scope":"..."}]
- Return [] when there is nothing worth saving.`

type memoryDreamDecision struct {
	ShouldRun bool
	Reason    string
	LastAt    time.Time
	Sessions  []session.Session
}

type MemoryFreshnessStatus struct {
	LastConsolidatedAt time.Time
	Warning            string
	HasMemories        bool
}

func shouldRunMemoryDream(now, lastAt time.Time, candidateCount int, force bool) bool {
	if force {
		return true
	}
	if !lastAt.IsZero() && now.Sub(lastAt) < time.Duration(memoryDreamMinHours)*time.Hour {
		return false
	}
	return candidateCount >= memoryDreamMinSessions
}

func selectDreamCandidateSessions(sessions []session.Session, lastAt time.Time, currentSessionID string) []session.Session {
	filtered := make([]session.Session, 0, len(sessions))
	lastUnix := lastAt.Unix()
	for _, sess := range sessions {
		if sess.ID == "" || sess.ID == currentSessionID {
			continue
		}
		if sess.ParentSessionID != "" || sess.Kind != session.KindNormal {
			continue
		}
		if !lastAt.IsZero() && sess.UpdatedAt <= lastUnix {
			continue
		}
		filtered = append(filtered, sess)
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].UpdatedAt == filtered[j].UpdatedAt {
			return filtered[i].ID < filtered[j].ID
		}
		return filtered[i].UpdatedAt > filtered[j].UpdatedAt
	})
	if len(filtered) > memoryDreamMaxSessions {
		filtered = filtered[:memoryDreamMaxSessions]
	}
	return filtered
}

func formatMemoryFreshness(now time.Time, lastAt time.Time, hasMemories bool) string {
	if !hasMemories {
		return ""
	}
	if lastAt.IsZero() {
		return "Memory stale: never consolidated — run /dream"
	}
	age := now.Sub(lastAt)
	if age < memoryFreshnessWarnAfter {
		return ""
	}
	return fmt.Sprintf("Memory stale: last consolidated %s ago — run /dream", humanizeDuration(age))
}

func humanizeDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1 day"
	}
	if days < 60 {
		return fmt.Sprintf("%d days", days)
	}
	months := days / 30
	if months <= 1 {
		return "1 month"
	}
	return fmt.Sprintf("%d months", months)
}

func (c *coordinator) MemoryFreshness(ctx context.Context) (MemoryFreshnessStatus, error) {
	if c.longTermMemory == nil {
		return MemoryFreshnessStatus{}, nil
	}
	infos, err := c.longTermMemory.ListMemoryFiles()
	if err != nil {
		return MemoryFreshnessStatus{}, err
	}
	lastAt, err := c.longTermMemory.ReadLastConsolidatedAt()
	if err != nil {
		return MemoryFreshnessStatus{}, err
	}
	return MemoryFreshnessStatus{
		LastConsolidatedAt: lastAt,
		Warning:            formatMemoryFreshness(time.Now(), lastAt, len(infos) > 0),
		HasMemories:        len(infos) > 0,
	}, nil
}

func (c *coordinator) Dream(ctx context.Context, sessionID string, force bool) error {
	decision, err := c.memoryDreamDecision(ctx, sessionID, force)
	if err != nil {
		return err
	}
	if !decision.ShouldRun {
		if force {
			return fmt.Errorf("not enough recent sessions or memories to consolidate")
		}
		return nil
	}

	owner := memoryDreamOwner(sessionID)
	acquired, err := c.longTermMemory.TryAcquireConsolidationLock(owner, memoryDreamStaleLockWindow)
	if err != nil {
		return err
	}
	if !acquired {
		if force {
			return fmt.Errorf("memory dream already in progress")
		}
		return nil
	}

	title := c.memoryDreamSessionTitle(ctx, sessionID)
	c.publishMemoryDreamNotification(sessionID, title, notify.TypeMemoryDreamStarted)
	go c.runMemoryDream(owner, sessionID, title, decision)
	return nil
}

func (c *coordinator) maybeStartMemoryDream(ctx context.Context, sessionID string) {
	if err := c.Dream(ctx, sessionID, false); err != nil {
		slog.Debug("Skipping memory dream", "error", err)
	}
}

func shouldSkipMemoryDreamSessionScan(now time.Time, force bool) bool {
	if force {
		return false
	}
	last := atomic.LoadInt64(&memoryDreamLastSessionScanUnix)
	if last <= 0 {
		return false
	}
	return now.Sub(time.Unix(last, 0)) < memoryDreamSessionScanInterval
}

func markMemoryDreamSessionScan(now time.Time) {
	atomic.StoreInt64(&memoryDreamLastSessionScanUnix, now.Unix())
}

func (c *coordinator) memoryDreamDecision(ctx context.Context, currentSessionID string, force bool) (memoryDreamDecision, error) {
	if c.longTermMemory == nil {
		return memoryDreamDecision{Reason: "memory service unavailable"}, nil
	}
	bgModel := c.resolveBackgroundModel(ctx)
	if bgModel == nil {
		return memoryDreamDecision{Reason: "background model unavailable"}, nil
	}
	if c.sessions == nil || c.messages == nil {
		return memoryDreamDecision{Reason: "session services unavailable"}, nil
	}

	lastAt, err := c.longTermMemory.ReadLastConsolidatedAt()
	if err != nil {
		return memoryDreamDecision{}, err
	}
	if shouldSkipMemoryDreamSessionScan(time.Now(), force) {
		return memoryDreamDecision{Reason: "scan_throttled", LastAt: lastAt}, nil
	}
	markMemoryDreamSessionScan(time.Now())
	allSessions, err := c.sessions.List(ctx)
	if err != nil {
		return memoryDreamDecision{}, err
	}
	candidates := selectDreamCandidateSessions(allSessions, lastAt, currentSessionID)
	shouldRun := shouldRunMemoryDream(time.Now(), lastAt, len(candidates), force)
	return memoryDreamDecision{
		ShouldRun: shouldRun,
		Reason:    "gate_closed",
		LastAt:    lastAt,
		Sessions:  candidates,
	}, nil
}

func (c *coordinator) runMemoryDream(owner, sessionID, title string, decision memoryDreamDecision) {
	defer func() {
		if err := c.longTermMemory.ReleaseConsolidationLock(owner); err != nil {
			slog.Warn("Failed to release memory consolidation lock", "error", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	if err := c.executeMemoryDream(ctx, decision); err != nil {
		c.publishMemoryDreamNotification(sessionID, title, notify.TypeMemoryDreamFailed)
		slog.Warn("Memory dream failed", "error", err)
		return
	}

	c.publishMemoryDreamNotification(sessionID, title, notify.TypeMemoryDreamFinished)
}

func (c *coordinator) executeMemoryDream(ctx context.Context, decision memoryDreamDecision) error {
	bgModel := c.resolveBackgroundModel(ctx)
	if bgModel == nil {
		return errors.New("background model unavailable")
	}
	if c.longTermMemory == nil {
		return errors.New("memory service unavailable")
	}

	existingMemories, err := c.longTermMemory.ListMemoryFiles()
	if err != nil {
		return fmt.Errorf("listing memory files: %w", err)
	}
	sessionBlock, err := c.memoryDreamSessionBlock(ctx, decision.Sessions)
	if err != nil {
		return err
	}
	if strings.TrimSpace(sessionBlock) == "" && len(existingMemories) == 0 {
		return errors.New("nothing to consolidate")
	}

	manifest := "(none)"
	if len(existingMemories) > 0 {
		manifest = buildMemoryManifest(existingMemories[:min(len(existingMemories), 20)])
	}

	prompt := fmt.Sprintf("Existing memories:\n%s\n\nRecent sessions:\n%s\n\nConsolidate the durable knowledge into memory entries.", manifest, sessionBlock)
	agent := fantasy.NewAgent(
		bgModel.model.Model,
		fantasy.WithSystemPrompt(memoryDreamPrompt),
		fantasy.WithMaxOutputTokens(3072),
		fantasy.WithUserAgent(memoryUserAgent),
	)

	resp, err := agent.Stream(copilot.ContextWithInitiatorType(ctx, copilot.InitiatorAgent), fantasy.AgentStreamCall{
		Prompt:          prompt,
		ProviderOptions: getProviderOptions(bgModel.model, bgModel.provider),
		PrepareStep: func(callCtx context.Context, options fantasy.PrepareStepFunctionOptions) (_ context.Context, prepared fantasy.PrepareStepResult, err error) {
			callCtx = copilot.ContextWithInitiatorType(callCtx, copilot.InitiatorAgent)
			prepared.Messages = options.Messages
			if bgModel.provider.SystemPromptPrefix != "" {
				prepared.Messages = append([]fantasy.Message{
					fantasy.NewSystemMessage(bgModel.provider.SystemPromptPrefix),
				}, prepared.Messages...)
			}
			return callCtx, prepared, nil
		},
	})
	if err != nil {
		return fmt.Errorf("running memory dream: %w", err)
	}
	if resp == nil {
		return errors.New("memory dream returned no response")
	}

	memories := parseExtractedMemories(resp.Response.Content.Text())
	for _, mem := range memories {
		fullContent := fmt.Sprintf("# %s\n\n%s", mem.Description, mem.Content)
		params := memory.StoreParams{
			Key:   mem.Key,
			Value: fullContent,
			Type:  cmp.Or(mem.Type, "general"),
		}
		if mem.Scope != "" {
			params.Scope = mem.Scope
		}
		if err := c.longTermMemory.Store(ctx, params); err != nil {
			return fmt.Errorf("storing consolidated memory %q: %w", mem.Key, err)
		}
	}

	if err := c.longTermMemory.WriteLastConsolidatedAt(time.Now()); err != nil {
		return fmt.Errorf("writing consolidated timestamp: %w", err)
	}
	return nil
}

func (c *coordinator) memoryDreamSessionBlock(ctx context.Context, sessions []session.Session) (string, error) {
	if len(sessions) == 0 {
		return "", nil
	}
	var sb strings.Builder
	for _, sess := range sessions {
		transcript, err := c.sessionTranscript(ctx, sess.ID)
		if err != nil {
			return "", err
		}
		if transcript == "" {
			continue
		}
		updatedAt := time.Unix(sess.UpdatedAt, 0).Format(time.RFC3339)
		fmt.Fprintf(&sb, "Session %s (%s, updated %s):\n%s\n\n", sess.ID, strings.TrimSpace(sess.Title), updatedAt, transcript)
		if sb.Len() >= memoryDreamMaxChars {
			break
		}
	}
	result := sb.String()
	if len(result) > memoryDreamMaxChars {
		result = result[:memoryDreamMaxChars]
	}
	return strings.TrimSpace(result), nil
}

func (c *coordinator) sessionTranscript(ctx context.Context, sessionID string) (string, error) {
	msgs, err := c.messages.List(ctx, sessionID)
	if err != nil {
		return "", fmt.Errorf("listing session messages: %w", err)
	}
	var lines []string
	for _, msg := range msgs {
		switch msg.Role {
		case message.User:
			if text := strings.TrimSpace(msg.Content().Text); text != "" {
				lines = append(lines, "USER: "+text)
			}
		case message.Assistant:
			if text := strings.TrimSpace(msg.Content().Text); text != "" {
				lines = append(lines, "ASSISTANT: "+text)
			}
		}
		if len(strings.Join(lines, "\n")) >= 4000 {
			break
		}
	}
	return strings.Join(lines, "\n"), nil
}

func (c *coordinator) memoryDreamSessionTitle(ctx context.Context, sessionID string) string {
	if strings.TrimSpace(sessionID) == "" || c.sessions == nil {
		return ""
	}
	sess, err := c.sessions.Get(ctx, sessionID)
	if err != nil {
		return ""
	}
	return sess.Title
}

func (c *coordinator) publishMemoryDreamNotification(sessionID, sessionTitle string, typ notify.Type) {
	if c.notify == nil {
		return
	}
	c.notify.Publish(pubsub.CreatedEvent, notify.Notification{SessionID: sessionID, SessionTitle: sessionTitle, Type: typ})
}

func memoryDreamOwner(sessionID string) string {
	pid := os.Getpid()
	if strings.TrimSpace(sessionID) == "" {
		return fmt.Sprintf("pid:%d", pid)
	}
	return fmt.Sprintf("pid:%d:%s", pid, sessionID)
}
