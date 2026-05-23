package critic

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/event"
	"github.com/charmbracelet/crush/internal/filetracker"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/message"
)

// Middleware wraps a primary SessionAgent with critic checkpoint support.
// It is a decorator: all methods delegate to the underlying primary agent by
// default. When critic is enabled and services are configured, Run() will
// snapshot files, compute diffs, fetch diagnostics, and submit checkpoints for
// review.
type Middleware struct {
	primary       agent.SessionAgent   // The underlying agent being decorated.
	cfg           CriticSkillConfig    // Runtime critic configuration.
	filetracker   filetracker.Service  // Tracks which files the agent has read.
	lspMgr        *lsp.Manager         // LSP client manager for diagnostic enrichment.
	criticSvc     *CriticService       // Orchestrates checkpoint reviews.
	messages      message.Service      // Used to inject feedback into conversation history.
	store         *Store               // Persists critic reviews to the database.
	coachProvider CoachSummaryProvider // Supplies coaching summaries for critic enrichment.
}

// NewMiddleware creates a critic middleware wrapping the given primary agent.
// If cfg.Enabled is false, the middleware is still valid but acts as a pure
// pass-through. Returns nil if primary is nil.
func NewMiddleware(primary agent.SessionAgent, cfg CriticSkillConfig) *Middleware {
	if primary == nil {
		return nil
	}
	return &Middleware{
		primary: primary,
		cfg:     cfg,
	}
}

// SetFileTracker configures the file tracker used to determine which files to
// snapshot before a destructive operation.
func (m *Middleware) SetFileTracker(ft filetracker.Service) {
	m.filetracker = ft
}

// SetLSPManager configures the LSP manager used for diagnostic enrichment.
func (m *Middleware) SetLSPManager(lm *lsp.Manager) {
	m.lspMgr = lm
}

// SetCriticService configures the critic service that reviews checkpoints.
func (m *Middleware) SetCriticService(svc *CriticService) {
	m.criticSvc = svc
}

// SetMessageService configures the message service used to inject critic
// feedback into the conversation history.
func (m *Middleware) SetMessageService(svc message.Service) {
	m.messages = svc
}

// SetStore configures the store used to persist critic reviews.
func (m *Middleware) SetStore(store *Store) {
	m.store = store
}

// SetCoachSummaryProvider configures the provider used to fetch coaching
// summaries for critic review enrichment.
func (m *Middleware) SetCoachSummaryProvider(provider CoachSummaryProvider) {
	m.coachProvider = provider
}

// Run delegates to the primary agent. When critic mode is enabled and all
// required services are wired, it additionally snapshots files, computes diffs,
// fetches LSP diagnostics, submits a checkpoint for review, and handles gate
// decisions (halt triggers rollback). If the critic returns revise, the
// middleware rolls back, injects feedback, and re-drives the primary agent up
// to MaxIterations.
func (m *Middleware) Run(ctx context.Context, call agent.SessionAgentCall) (*fantasy.AgentResult, error) {
	if call.CriticEnabled != nil && !*call.CriticEnabled {
		slog.Debug("Critic disabled for this session", "session_id", call.SessionID)
		return m.primary.Run(ctx, call)
	}
	if !m.cfg.Enabled {
		slog.Debug("Critic bypassed: disabled in config", "session_id", call.SessionID)
		return m.primary.Run(ctx, call)
	}
	if m.criticSvc == nil {
		slog.Debug("Critic bypassed: no service configured", "session_id", call.SessionID)
		return m.primary.Run(ctx, call)
	}
	if !m.criticSvc.Enabled() {
		slog.Debug("Critic bypassed: service reports disabled", "session_id", call.SessionID)
		return m.primary.Run(ctx, call)
	}

	var lastResult *fantasy.AgentResult
	finalVerdict := ""
	var firstReviewID string

	for iteration := 0; iteration <= m.cfg.MaxIterations; iteration++ {
		iterationStart := time.Now()
		snapshot := NewSnapshotStore()
		snapshot.SetMaxFileSize(int64(m.cfg.MaxFileSize))

		// 1. Snapshot files that the agent has read or written in this session.
		snapshotStart := time.Now()
		if m.filetracker != nil {
			readFiles, err := m.filetracker.ListReadFiles(ctx, call.SessionID)
			if err != nil {
				slog.Warn("Failed to list read files for critic snapshot", "error", err)
			}
			writtenFiles, err := m.filetracker.ListWrittenFiles(ctx, call.SessionID)
			if err != nil {
				slog.Warn("Failed to list written files for critic snapshot", "error", err)
			}
			allFiles := make([]string, 0, len(readFiles)+len(writtenFiles))
			seen := make(map[string]struct{})
			for _, p := range readFiles {
				if _, ok := seen[p]; !ok {
					seen[p] = struct{}{}
					allFiles = append(allFiles, p)
				}
			}
			for _, p := range writtenFiles {
				if _, ok := seen[p]; !ok {
					seen[p] = struct{}{}
					allFiles = append(allFiles, p)
				}
			}
			if capErr := snapshot.Capture(allFiles); capErr != nil {
				slog.Warn("Failed to capture critic snapshot", "error", capErr)
			}
		}
		snapshotMs := time.Since(snapshotStart).Milliseconds()

		// 2. Run primary agent.
		result, err := m.primary.Run(ctx, call)
		if err != nil {
			snapshot.Clear()
			m.criticSvc.PublishLoopCompleted(call.SessionID, iteration, finalVerdict)
			return result, err
		}
		// Queued calls return nil result; bail out of the loop.
		if result == nil {
			snapshot.Clear()
			m.criticSvc.PublishLoopCompleted(call.SessionID, iteration, finalVerdict)
			return nil, nil
		}
		lastResult = result

		// 3. Detect changed files.
		changedPaths, after, err := snapshot.Changed()
		if err != nil {
			slog.Warn("Failed to detect changed files", "error", err)
			snapshot.Clear()
			m.criticSvc.PublishLoopCompleted(call.SessionID, iteration, finalVerdict)
			return result, nil
		}

		// 3b. Detect files that were read or written during this turn but were
		// not in the pre-run snapshot. This eliminates the first-run blind spot
		// where newly created files are missed because they weren't tracked yet
		// when the snapshot was taken.
		if m.filetracker != nil {
			stashPaths := make(map[string]struct{}, len(snapshot.Paths()))
			for _, p := range snapshot.Paths() {
				stashPaths[p] = struct{}{}
			}

			newReadFiles, err := m.filetracker.ListReadFiles(ctx, call.SessionID)
			if err != nil {
				slog.Warn("Failed to list read files post-run", "error", err)
			}
			newWrittenFiles, err := m.filetracker.ListWrittenFiles(ctx, call.SessionID)
			if err != nil {
				slog.Warn("Failed to list written files post-run", "error", err)
			}

			for _, p := range newReadFiles {
				if _, ok := stashPaths[p]; ok {
					continue
				}
				b, readErr := os.ReadFile(p)
				if readErr != nil {
					if !os.IsNotExist(readErr) {
						slog.Warn("Failed to read file for critic diff", "path", p, "error", readErr)
					}
					continue
				}
				changedPaths = append(changedPaths, p)
				after[p] = b
				stashPaths[p] = struct{}{}
			}
			for _, p := range newWrittenFiles {
				if _, ok := stashPaths[p]; ok {
					continue
				}
				b, readErr := os.ReadFile(p)
				if readErr != nil {
					if !os.IsNotExist(readErr) {
						slog.Warn("Failed to read file for critic diff", "path", p, "error", readErr)
					}
					continue
				}
				changedPaths = append(changedPaths, p)
				after[p] = b
				stashPaths[p] = struct{}{}
			}
		}

		var checkpoint Checkpoint
		var diffStr string
		var truncated bool
		var diags []DiagnosticSnapshot
		var diffMs, diagsMs int64

		msgText := result.Response.Content.Text()

		if len(changedPaths) > 0 {
			// 4a. File-edit path: compute diff, fetch diagnostics.
			diffStart := time.Now()
			diffStr, truncated, err = ComputeDiff(changedPaths, snapshot, after, m.cfg.MaxDiffSize)
			if err != nil {
				slog.Warn("Failed to compute critic diff", "error", err)
				snapshot.Clear()
				m.criticSvc.PublishLoopCompleted(call.SessionID, iteration, finalVerdict)
				return result, nil
			}
			diffMs = time.Since(diffStart).Milliseconds()
			if truncated {
				slog.Debug("Critic diff truncated", "max_size", m.cfg.MaxDiffSize, "path_count", len(changedPaths))
			}

			diagsStart := time.Now()
			if m.lspMgr != nil {
				ctx2, cancel := context.WithTimeout(ctx, 3*time.Second)
				var diagErr error
				diags, diagErr = FetchLSPDiagnostics(ctx2, m.lspMgr, changedPaths, 2*time.Second)
				if diagErr != nil {
					slog.Warn("Failed to fetch LSP diagnostics", "error", diagErr)
				}
				cancel()
			}
			diagsMs = time.Since(diagsStart).Milliseconds()

			checkpoint = Checkpoint{
				Type:           CheckpointEdit,
				UserPrompt:     call.Prompt,
				PrimaryDiff:    diffStr,
				MessageContent: msgText,
				LSPDiagnostics: diags,
				Iteration:      iteration,
			}
			slog.Debug("Critic edit checkpoint created", "session_id", call.SessionID, "changed_files", len(changedPaths), "message_len", len(msgText))
		} else {
			// 4b. Message-review path: no file changes, review the agent's response.
			if msgText == "" {
				slog.Debug("Critic skipping message review: empty response text", "session_id", call.SessionID)
				snapshot.Clear()
				m.criticSvc.PublishLoopCompleted(call.SessionID, iteration, finalVerdict)
				return result, nil
			}
			checkpoint = Checkpoint{
				Type:           CheckpointMessage,
				UserPrompt:     call.Prompt,
				MessageContent: msgText,
				Iteration:      iteration,
			}
			slog.Debug("Critic message checkpoint created", "session_id", call.SessionID, "text_len", len(msgText))
		}

		// Enrich checkpoint with coaching observations from the primary agent turn.
		if m.coachProvider != nil {
			checkpoint.CoachSummary = m.coachProvider.GetCoachSummary(call.SessionID)
			if checkpoint.CoachSummary != "" {
				slog.Debug("Enriched critic checkpoint with coach summary", "session_id", call.SessionID, "summary_len", len(checkpoint.CoachSummary))
			}
		}

		reviewStart := time.Now()
		feedback, err := m.criticSvc.Review(ctx, call.SessionID, checkpoint)
		reviewMs := time.Since(reviewStart).Milliseconds()
		if err != nil {
			slog.Warn("Critic review failed, failing open", "session_id", call.SessionID, "error", err)
			snapshot.Clear()
			m.criticSvc.PublishLoopCompleted(call.SessionID, iteration, finalVerdict)
			return result, nil // Fail-open.
		}

		finalVerdict = feedback.Verdict

		totalMs := time.Since(iterationStart).Milliseconds()
		slog.Info("Critic review completed",
			"session_id", call.SessionID,
			"iteration", iteration,
			"verdict", feedback.Verdict,
			"confidence", feedback.Confidence,
			"concerns", len(feedback.Concerns),
			"diff_bytes", len(diffStr),
			"diagnostics", len(diags),
			"snapshot_ms", snapshotMs,
			"diff_ms", diffMs,
			"diagnostics_ms", diagsMs,
			"review_ms", reviewMs,
			"total_middleware_ms", totalMs,
		)

		// 7. Persist review and resolve message ID.
		msgID := m.latestAssistantMessageID(ctx, call.SessionID)
		if m.store != nil && msgID != "" {
			record, storeErr := m.store.Create(ctx, call.SessionID, msgID, feedback, diffStr, diags)
			if storeErr != nil {
				slog.Warn("Failed to persist critic review", "error", storeErr)
			} else if iteration == 0 {
				firstReviewID = record.ID
			}
		}

		// 8. Gate decision.
		switch Gate(feedback) {
		case GateHalt:
			if rbErr := snapshot.Rollback(); rbErr != nil {
				slog.Error("Failed to rollback critic snapshot", "error", rbErr)
			}
			event.TrackCriticRollback(call.SessionID)
			snapshot.Clear()
			m.updateOutcome(ctx, firstReviewID, "halted")
			m.criticSvc.PublishLoopCompleted(call.SessionID, iteration+1, finalVerdict)
			return result, fmt.Errorf("critic halted: %s", feedback.Summary)
		case GateApprove:
			snapshot.Clear()
			m.updateOutcome(ctx, firstReviewID, "approved")
			m.criticSvc.PublishLoopCompleted(call.SessionID, iteration+1, finalVerdict)
			return result, nil
		case GateRevise:
			if iteration >= m.cfg.MaxIterations {
				snapshot.Clear()
				m.updateOutcome(ctx, firstReviewID, "max_iterations")
				m.criticSvc.PublishLoopCompleted(call.SessionID, iteration+1, finalVerdict)
				return result, fmt.Errorf("critic max iterations (%d) exceeded", m.cfg.MaxIterations)
			}
			// If auto-approve is off and confidence is low, skip revision.
			if !m.criticSvc.ShouldAutoApprove(feedback) {
				snapshot.Clear()
				m.updateOutcome(ctx, firstReviewID, "approved")
				m.criticSvc.PublishLoopCompleted(call.SessionID, iteration+1, finalVerdict)
				return result, nil
			}
			if checkpoint.Type == CheckpointEdit {
				// Rollback file changes before re-run.
				if rbErr := snapshot.Rollback(); rbErr != nil {
					slog.Error("Failed to rollback critic snapshot", "error", rbErr)
				}
				event.TrackCriticRollback(call.SessionID)
			} else if checkpoint.Type == CheckpointMessage {
				// Remove the last assistant message so the agent regenerates cleanly.
				if msgID := m.latestAssistantMessageID(ctx, call.SessionID); msgID != "" && m.messages != nil {
					if delErr := m.messages.Delete(ctx, msgID); delErr != nil {
						slog.Warn("Failed to delete assistant message for revision", "error", delErr)
					}
				}
			}
			snapshot.Clear()
			if injectErr := m.injectFeedback(ctx, call.SessionID, feedback, iteration); injectErr != nil {
				m.criticSvc.PublishLoopCompleted(call.SessionID, iteration+1, finalVerdict)
				return result, fmt.Errorf("failed to inject critic feedback: %w", injectErr)
			}
			// Loop continues; primary agent will re-run with feedback in context.
		}
	}

	m.updateOutcome(ctx, firstReviewID, "max_iterations")
	m.criticSvc.PublishLoopCompleted(call.SessionID, m.cfg.MaxIterations+1, finalVerdict)
	return lastResult, nil
}

// injectFeedback appends a System message containing the critic review to the
// session conversation history.
func (m *Middleware) injectFeedback(
	ctx context.Context,
	sessionID string,
	feedback *CriticFeedback,
	iteration int,
) error {
	if m.messages == nil {
		return fmt.Errorf("no message service configured")
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "[[CRITIC REVIEW — Iteration %d]]\n\n", iteration+1)
	fmt.Fprintf(&sb, "Verdict: %s (confidence: %.2f)\n\n", feedback.Verdict, feedback.Confidence)

	if len(feedback.Concerns) > 0 {
		sb.WriteString("Concerns:\n")
		for _, c := range feedback.Concerns {
			fmt.Fprintf(&sb, "- [%s | %s] %s Suggestion: %s\n",
				c.Severity, c.Dimension, c.Summary, c.Suggestion)
		}
		sb.WriteString("\n")
	}

	if feedback.Summary != "" {
		fmt.Fprintf(&sb, "Summary: %s\n", feedback.Summary)
	}

	// Note: message.Service.Create appends a Finish{Reason:"stop"} part
	// automatically for non-Assistant roles. This is harmless but leaves a
	// marker in the conversation history.
	_, err := m.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role:  message.System,
		Parts: []message.ContentPart{message.TextContent{Text: sb.String()}},
	})
	return err
}

// latestAssistantMessageID returns the ID of the most recent assistant message
// in the session, or empty string if none exists or the service is unavailable.
func (m *Middleware) latestAssistantMessageID(ctx context.Context, sessionID string) string {
	if m.messages == nil {
		return ""
	}
	msgs, err := m.messages.List(ctx, sessionID)
	if err != nil || len(msgs) == 0 {
		return ""
	}
	// Messages are returned in created_at ASC order; the last one is newest.
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == message.Assistant {
			return msgs[i].ID
		}
	}
	return ""
}

// SetModels delegates to the primary agent.
func (m *Middleware) SetModels(large agent.Model, small agent.Model) {
	m.primary.SetModels(large, small)
}

// SetTools delegates to the primary agent.
func (m *Middleware) SetTools(tools []fantasy.AgentTool) {
	m.primary.SetTools(tools)
}

// SetSystemPrompt delegates to the primary agent.
func (m *Middleware) SetSystemPrompt(systemPrompt string) {
	m.primary.SetSystemPrompt(systemPrompt)
}

// Cancel delegates to the primary agent.
func (m *Middleware) Cancel(sessionID string) {
	m.primary.Cancel(sessionID)
}

// CancelAll delegates to the primary agent.
func (m *Middleware) CancelAll() {
	m.primary.CancelAll()
}

// IsSessionBusy delegates to the primary agent.
func (m *Middleware) IsSessionBusy(sessionID string) bool {
	return m.primary.IsSessionBusy(sessionID)
}

// IsBusy delegates to the primary agent.
func (m *Middleware) IsBusy() bool {
	return m.primary.IsBusy()
}

// QueuedPrompts delegates to the primary agent.
func (m *Middleware) QueuedPrompts(sessionID string) int {
	return m.primary.QueuedPrompts(sessionID)
}

// QueuedPromptsList delegates to the primary agent.
func (m *Middleware) QueuedPromptsList(sessionID string) []string {
	return m.primary.QueuedPromptsList(sessionID)
}

// ClearQueue delegates to the primary agent.
func (m *Middleware) ClearQueue(sessionID string) {
	m.primary.ClearQueue(sessionID)
}

// Summarize delegates to the primary agent.
func (m *Middleware) Summarize(ctx context.Context, sessionID string, opts fantasy.ProviderOptions) error {
	return m.primary.Summarize(ctx, sessionID, opts)
}

// Model delegates to the primary agent.
func (m *Middleware) Model() agent.Model {
	return m.primary.Model()
}

// updateOutcome sets the revision outcome on the first review record, if any.
func (m *Middleware) updateOutcome(ctx context.Context, reviewID string, outcome string) {
	if m.store == nil || reviewID == "" {
		return
	}
	if err := m.store.UpdateOutcome(ctx, reviewID, outcome); err != nil {
		slog.Warn("Failed to update critic review outcome", "error", err, "review_id", reviewID)
	}
}

// compile-time interface check.
var _ agent.SessionAgent = (*Middleware)(nil)
