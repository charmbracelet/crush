package replacer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/event"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
)

// Visual indicator constants.
const (
	coachReadyLabel    = "Coach is ready..."
	coachEvalLabel     = "Coach is evaluating..."
	coachStopLabel     = "✅ Coach is satisfied"
	coachContinueLabel = "🔄 Coach suggests continuing..."
)

// Display durations for transition indicators. Variables so tests can override.
var (
	coachStopDisplay     = 1500 * time.Millisecond
	coachContinueDisplay = 500 * time.Millisecond
)

// flashDoneCh is signalled by async indicator goroutines when they finish.
// Tests can drain it for synchronization.
var flashDoneCh = make(chan struct{}, 10)

// Middleware wraps a primary SessionAgent with replacement agent conversation
// continuation support. After the primary agent responds, the replacement agent
// evaluates whether the conversation should continue with a follow-up prompt.
type Middleware struct {
	primary      agent.SessionAgent
	cfg          ReplacerConfig
	messages     message.Service
	resolveModel func(ctx context.Context) (fantasy.LanguageModel, error)
	busy         atomic.Bool

	// evalCancel cancels an in-flight evaluate() call when the user sends new
	// input or explicitly cancels the session.
	evalCancel context.CancelFunc
	// evalSpinnerID tracks the evaluating spinner so a concurrent Run() can
	// delete it immediately when new input arrives.
	evalSpinnerID string
	evalMu        sync.Mutex

	// skipCoach is set by SkipCoach() to interrupt the current evaluation
	// without disabling the replacer permanently.
	skipCoach atomic.Bool
}

// NewMiddleware creates a replacement agent middleware.
func NewMiddleware(primary agent.SessionAgent, cfg ReplacerConfig) *Middleware {
	if primary == nil {
		return nil
	}
	return &Middleware{primary: primary, cfg: cfg}
}

// SetMessageService configures the message service used to fetch conversation
// history and inject follow-up messages.
func (m *Middleware) SetMessageService(svc message.Service) {
	m.messages = svc
}

// SetModelResolver configures the function used to resolve the replacement
// agent's language model.
func (m *Middleware) SetModelResolver(fn func(ctx context.Context) (fantasy.LanguageModel, error)) {
	m.resolveModel = fn
}

// Run delegates to the primary agent. When the replacement agent is enabled and
// wired, it additionally evaluates the conversation and may inject follow-up
// prompts to continue the dialogue until the replacement agent decides to stop.
func (m *Middleware) Run(ctx context.Context, call agent.SessionAgentCall) (*fantasy.AgentResult, error) {
	if call.ReplacerEnabled != nil && !*call.ReplacerEnabled {
		slog.Debug("Replacer disabled for this session", "session_id", call.SessionID)
		return m.primary.Run(ctx, call)
	}
	if !m.cfg.Enabled || m.messages == nil || m.resolveModel == nil {
		slog.Debug("Replacer bypassed", "enabled", m.cfg.Enabled, "has_messages", m.messages != nil, "has_resolver", m.resolveModel != nil)
		return m.primary.Run(ctx, call)
	}

	// If the user already sent new input while we were evaluating a previous
	// turn, cancel that evaluation and wipe its spinner immediately.
	m.evalMu.Lock()
	if m.evalCancel != nil {
		m.evalCancel()
		m.evalCancel = nil
	}
	oldSpinnerID := m.evalSpinnerID
	m.evalSpinnerID = ""
	m.evalMu.Unlock()
	if oldSpinnerID != "" && m.messages != nil {
		go func(id string) {
			delCtx, delCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer delCancel()
			if delErr := m.messages.Delete(delCtx, id); delErr != nil {
				slog.Warn("Replacer failed to delete stale eval indicator", "session_id", call.SessionID, "error", delErr)
			}
		}(oldSpinnerID)
	}

	slog.Info("Replacer started", "session_id", call.SessionID, "max_iterations", m.cfg.MaxIterations)

	// Run the primary agent first.
	result, err, evalMsg := m.runPrimaryWithEval(ctx, call)
	if err != nil {
		slog.Info("Replacer primary returned error", "session_id", call.SessionID, "error", err)
		m.deleteEvalIndicator(evalMsg.ID)
		return result, err
	}
	if result == nil {
		slog.Info("Replacer primary returned nil", "session_id", call.SessionID)
		m.deleteEvalIndicator(evalMsg.ID)
		return nil, nil
	}

	// Track coach prompts used in this Run() call to avoid repeating them
	// without needing an extra database round-trip.
	seenPrompts := make(map[string]struct{})

	// Replacement agent loop: evaluate and potentially continue.
	for iteration := 0; iteration < m.cfg.MaxIterations; iteration++ {
		// Fast-path: if the context is already cancelled, bail out before
		// starting any work.
		if ctx.Err() != nil {
			slog.Info("Replacer evaluation skipped because context was cancelled", "session_id", call.SessionID, "iteration", iteration)
			m.deleteEvalIndicator(evalMsg.ID)
			return result, nil
		}

		// Check if the user requested a one-time skip via /skipcoach.
		if m.skipCoach.Load() {
			m.skipCoach.Store(false)
			slog.Info("Replacer evaluation skipped by user", "session_id", call.SessionID, "iteration", iteration)
			m.deleteEvalIndicator(evalMsg.ID)
			event.TrackReplacerDecision(call.SessionID, "user_skip", iteration)
			event.TrackReplacerLoopCompleted(call.SessionID, iteration+1, "user_skip")
			return result, nil
		}

		// Transition the indicator from "ready" to "evaluating" so the user
		// sees exactly when the coach LLM call starts. Fire-and-forget so a
		// stuck database write can never block the evaluation.
		if evalMsg.ID != "" && m.messages != nil {
			evalMsg.SpinnerLabel = coachEvalLabel
			go func(msg message.Message) {
				upCtx, upCancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer upCancel()
				if upErr := m.messages.Update(upCtx, msg); upErr != nil {
					slog.Warn("Replacer failed to update eval indicator", "session_id", call.SessionID, "error", upErr)
				}
			}(evalMsg)
		}

		evalCtx, cancel := context.WithCancel(ctx)
		m.evalMu.Lock()
		m.evalCancel = cancel
		m.evalMu.Unlock()

		decision, evalErr := m.evaluate(evalCtx, call.SessionID, iteration)

		m.evalMu.Lock()
		m.evalCancel = nil
		m.evalMu.Unlock()

		// Clean up the evaluating indicator. Fire-and-forget so a stuck
		// database delete can never block the loop.
		if evalMsg.ID != "" && m.messages != nil {
			go func(id string) {
				delCtx, delCancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer delCancel()
				if delErr := m.messages.Delete(delCtx, id); delErr != nil {
					slog.Warn("Replacer failed to delete eval indicator", "session_id", call.SessionID, "error", delErr)
				}
			}(evalMsg.ID)
			evalMsg.ID = ""
			m.evalMu.Lock()
			m.evalSpinnerID = ""
			m.evalMu.Unlock()
		}

		if evalErr != nil {
			// Distinguish a user-initiated skip from genuine errors.
			if m.skipCoach.Load() {
				m.skipCoach.Store(false)
				slog.Info("Replacer evaluation skipped by user", "session_id", call.SessionID, "iteration", iteration)
				event.TrackReplacerDecision(call.SessionID, "user_skip", iteration)
				event.TrackReplacerLoopCompleted(call.SessionID, iteration+1, "user_skip")
				return result, nil
			}
			// If the coach timed out, treat it as satisfied rather than an error.
			if errors.Is(evalErr, context.DeadlineExceeded) {
				slog.Info("Replacer evaluation timed out, treating as satisfied", "session_id", call.SessionID, "iteration", iteration)
				event.TrackReplacerDecision(call.SessionID, "timeout_stop", iteration)
				event.TrackReplacerLoopCompleted(call.SessionID, iteration+1, "timeout_stop")
				m.flashStopIndicator(ctx, call.SessionID)
				return result, nil
			}
			slog.Warn("Replacer evaluation failed", "session_id", call.SessionID, "error", evalErr)
			event.TrackReplacerLoopCompleted(call.SessionID, iteration, "error")
			return result, nil
		}
		if decision == nil || decision.Action == "stop" {
			action := "stop"
			if decision == nil {
				action = "nil"
			}
			event.TrackReplacerDecision(call.SessionID, action, iteration)
			event.TrackReplacerLoopCompleted(call.SessionID, iteration+1, action)
			if decision != nil && decision.Action == "stop" {
				m.flashStopIndicator(ctx, call.SessionID)
			}
			return result, nil
		}

		// Guard against repeating the same generic follow-up across turns.
		// First check the in-memory map (no DB call), then fall back to the
		// database scan.
		normalized := strings.ToLower(strings.TrimSpace(decision.Prompt))
		if _, dup := seenPrompts[normalized]; dup {
			slog.Info("Replacer stopping to avoid repeating a prompt from this run", "session_id", call.SessionID, "prompt", decision.Prompt)
			event.TrackReplacerDecision(call.SessionID, "duplicate_stop", iteration)
			event.TrackReplacerLoopCompleted(call.SessionID, iteration+1, "duplicate_stop")
			m.flashStopIndicator(ctx, call.SessionID)
			return result, nil
		}
		if isDuplicateCoachPrompt(ctx, m.messages, call.SessionID, decision.Prompt) {
			slog.Info("Replacer stopping to avoid repeating a previous coach prompt", "session_id", call.SessionID, "prompt", decision.Prompt)
			event.TrackReplacerDecision(call.SessionID, "duplicate_stop", iteration)
			event.TrackReplacerLoopCompleted(call.SessionID, iteration+1, "duplicate_stop")
			m.flashStopIndicator(ctx, call.SessionID)
			return result, nil
		}
		seenPrompts[normalized] = struct{}{}

		event.TrackReplacerDecision(call.SessionID, "continue", iteration)

		// Show a brief continue indicator before re-running the primary.
		m.flashContinueIndicator(ctx, call.SessionID)

		// Re-run primary agent with the follow-up prompt.
		newCall := call
		newCall.Prompt = coachPrompt(decision.Prompt)
		newCall.Attachments = nil
		result, err, evalMsg = m.runPrimaryWithEval(ctx, newCall)
		if err != nil {
			event.TrackReplacerLoopCompleted(call.SessionID, iteration+1, "primary_error")
			m.deleteEvalIndicator(evalMsg.ID)
			return result, err
		}
		if result == nil {
			event.TrackReplacerLoopCompleted(call.SessionID, iteration+1, "nil_result")
			m.deleteEvalIndicator(evalMsg.ID)
			return nil, nil
		}
	}

	slog.Info("Replacer max iterations reached", "session_id", call.SessionID, "max_iterations", m.cfg.MaxIterations)
	event.TrackReplacerLoopCompleted(call.SessionID, m.cfg.MaxIterations, "max_iterations")
	return result, nil
}

// runPrimaryWithEval runs the primary agent and creates the evaluating spinner
// as soon as the agent's final assistant message gets a FinishPart. This makes
// the indicator pop up immediately after the last response rather than waiting
// for primary.Run() to fully return.
func (m *Middleware) runPrimaryWithEval(ctx context.Context, call agent.SessionAgentCall) (*fantasy.AgentResult, error, message.Message) {
	if m.messages == nil {
		result, err := m.primary.Run(ctx, call)
		return result, err, message.Message{}
	}

	evalCtx, evalCancel := context.WithCancel(ctx)
	defer evalCancel()
	events := m.messages.Subscribe(evalCtx)

	// Drain the subscription after cancellation so the broker goroutine can
	// clean up without blocking on a full channel.
	if events != nil {
		defer func() {
			go func() {
				for range events {
				}
			}()
		}()
	}

	var evalMsg message.Message
	var evalMu sync.Mutex

	// Watch for the primary agent's final assistant message to finish and
	// create the evaluating spinner immediately.
	if events != nil {
		go func() {
			for event := range events {
				if event.Type != pubsub.UpdatedEvent {
					continue
				}
				msg := event.Payload
				if msg.Role != message.Assistant || msg.SessionID != call.SessionID || msg.SpinnerLabel != "" {
					continue
				}
				finish := msg.FinishPart()
				if finish == nil || finish.Reason == message.FinishReasonToolUse {
					continue
				}
				evalMu.Lock()
				if evalMsg.ID == "" {
					created, createErr := m.messages.Create(ctx, call.SessionID, message.CreateMessageParams{
						Role:         message.Assistant,
						SpinnerLabel: coachReadyLabel,
					})
					if createErr != nil {
						slog.Warn("Replacer failed to create eval indicator", "session_id", call.SessionID, "error", createErr)
					} else {
						evalMsg = created
						m.evalMu.Lock()
						m.evalSpinnerID = created.ID
						m.evalMu.Unlock()
					}
				}
				evalMu.Unlock()
				return
			}
		}()
	}

	result, err := m.primary.Run(ctx, call)

	evalMu.Lock()
	if evalMsg.ID == "" {
		created, createErr := m.messages.Create(ctx, call.SessionID, message.CreateMessageParams{
			Role:         message.Assistant,
			SpinnerLabel: coachReadyLabel,
		})
		if createErr != nil {
			slog.Warn("Replacer failed to create eval indicator", "session_id", call.SessionID, "error", createErr)
		} else {
			evalMsg = created
			m.evalMu.Lock()
			m.evalSpinnerID = created.ID
			m.evalMu.Unlock()
		}
	}
	evalMu.Unlock()

	return result, err, evalMsg
}

// evaluate asks the small LLM whether to stop or continue.
func (m *Middleware) evaluate(ctx context.Context, sessionID string, iteration int) (*Decision, error) {
	// Fast path: force-continue for testing skips the LLM call entirely.
	if os.Getenv("CRUSH_REPLACER_FORCE_CONTINUE") == "1" {
		slog.Info("Replacer force-continuing", "session_id", sessionID, "iteration", iteration)
		return &Decision{Action: "continue", Prompt: "What would you like help with today?"}, nil
	}

	slog.Info("Replacer evaluation starting", "session_id", sessionID, "iteration", iteration)

	// List messages with a goroutine+timeout so a stuck database query
	// cannot hang the evaluation indefinitely.
	listCtx, listCancel := context.WithTimeout(ctx, 5*time.Second)
	defer listCancel()
	type listResult struct {
		msgs []message.Message
		err  error
	}
	listDone := make(chan listResult, 1)
	go func() {
		msgs, err := m.messages.List(listCtx, sessionID)
		listDone <- listResult{msgs: msgs, err: err}
	}()
	var msgs []message.Message
	var err error
	select {
	case <-listCtx.Done():
		select {
		case lr := <-listDone:
			msgs, err = lr.msgs, lr.err
		default:
			slog.Warn("Replacer list messages timed out", "session_id", sessionID, "iteration", iteration)
			return nil, fmt.Errorf("list messages: %w", listCtx.Err())
		}
	case lr := <-listDone:
		msgs, err = lr.msgs, lr.err
	}
	if err != nil {
		slog.Warn("Replacer list messages failed", "session_id", sessionID, "iteration", iteration, "error", err)
		return nil, fmt.Errorf("list messages: %w", err)
	}
	slog.Info("Replacer listed messages", "session_id", sessionID, "iteration", iteration, "count", len(msgs))

	promptText, err := BuildReplacerPrompt(msgs)
	if err != nil {
		return nil, fmt.Errorf("build prompt: %w", err)
	}

	// Resolve the model with a hard timeout so a stuck provider setup cannot
	// hang the evaluation indefinitely.
	resolveCtx, resolveCancel := context.WithTimeout(ctx, 5*time.Second)
	model, err := m.resolveModel(resolveCtx)
	resolveCancel()
	if err != nil {
		slog.Warn("Replacer resolve model failed", "session_id", sessionID, "iteration", iteration, "error", err)
		return nil, fmt.Errorf("resolve model: %w", err)
	}
	slog.Info("Replacer model resolved", "session_id", sessionID, "iteration", iteration)

	m.busy.Store(true)

	timeout := m.cfg.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	ctx2, cancel := context.WithTimeout(ctx, timeout)

	maxTokens := int64(512)
	temp := 0.3
	call := fantasy.Call{
		Prompt:          fantasy.Prompt{fantasy.NewUserMessage(promptText)},
		MaxOutputTokens: &maxTokens,
		Temperature:     &temp,
	}

	// Run generation in a goroutine so that even if the provider does not
	// respect context cancellation we still return when the timeout fires.
	type generateResult struct {
		resp *fantasy.Response
		err  error
	}
	done := make(chan generateResult, 1)
	go func() {
		resp, err := model.Generate(ctx2, call)
		done <- generateResult{resp: resp, err: err}
	}()

	var resp *fantasy.Response
	select {
	case <-ctx2.Done():
		cancel()
		// The goroutine may have finished at the exact moment the timer
		// fired. Try a non-blocking receive to avoid discarding a valid
		// result and returning a spurious timeout error.
		select {
		case gr := <-done:
			resp, err = gr.resp, gr.err
		default:
			err = ctx2.Err()
		}
	case gr := <-done:
		cancel()
		resp, err = gr.resp, gr.err
	}

	m.busy.Store(false)
	if err != nil {
		slog.Warn("Replacer generate failed", "session_id", sessionID, "iteration", iteration, "error", err)
		return nil, fmt.Errorf("generate: %w", err)
	}
	if resp == nil {
		return nil, fmt.Errorf("generate: nil response")
	}

	raw := resp.Content.Text()
	slog.Info("Replacer raw response", "session_id", sessionID, "iteration", iteration, "raw", raw)

	decision, err := ParseDecision(raw)
	if err != nil {
		return nil, fmt.Errorf("parse decision: %w", err)
	}

	slog.Info("Replacer decision", "session_id", sessionID, "iteration", iteration, "action", decision.Action, "prompt", decision.Prompt)
	return decision, nil
}

// deleteEvalIndicator removes a spinner message by ID. It uses a background
// context so cancellation never blocks cleanup, and retries once on failure to
// avoid leaving orphaned indicators in the UI.
func (m *Middleware) deleteEvalIndicator(id string) {
	if id == "" || m.messages == nil {
		return
	}
	go func(msgID string) {
		delCtx, delCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer delCancel()
		if delErr := m.messages.Delete(delCtx, msgID); delErr != nil {
			// Retry once after a short delay in case of transient errors.
			time.Sleep(50 * time.Millisecond)
			_ = m.messages.Delete(delCtx, msgID)
		}
	}(id)
}

// flashStopIndicator shows a brief "coach is satisfied" spinner and then
// removes it after a short delay so the user gets feedback when the coach
// decides to stop.
func (m *Middleware) flashStopIndicator(ctx context.Context, sessionID string) {
	if m.messages == nil {
		return
	}
	if ctx.Err() != nil {
		return
	}
	go func() {
		defer func() { flashDoneCh <- struct{}{} }()
		msg, err := m.messages.Create(ctx, sessionID, message.CreateMessageParams{
			Role:         message.Assistant,
			SpinnerLabel: coachStopLabel,
		})
		if err != nil {
			slog.Warn("Replacer failed to create stop indicator", "session_id", sessionID, "error", err)
			return
		}
		time.Sleep(coachStopDisplay)
		if delErr := m.messages.Delete(ctx, msg.ID); delErr != nil {
			slog.Warn("Replacer failed to delete stop indicator", "session_id", sessionID, "error", delErr)
		}
	}()
}

// flashContinueIndicator shows a brief transition spinner so the user sees
// feedback when the coach decides to continue before the primary agent starts
// its next turn.
func (m *Middleware) flashContinueIndicator(ctx context.Context, sessionID string) {
	if m.messages == nil {
		return
	}
	if ctx.Err() != nil {
		return
	}
	go func() {
		defer func() { flashDoneCh <- struct{}{} }()
		msg, err := m.messages.Create(ctx, sessionID, message.CreateMessageParams{
			Role:         message.Assistant,
			SpinnerLabel: coachContinueLabel,
		})
		if err != nil {
			slog.Warn("Replacer failed to create continue indicator", "session_id", sessionID, "error", err)
			return
		}
		time.Sleep(coachContinueDisplay)
		if delErr := m.messages.Delete(ctx, msg.ID); delErr != nil {
			slog.Warn("Replacer failed to delete continue indicator", "session_id", sessionID, "error", delErr)
		}
	}()
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

// Cancel delegates to the primary agent and aborts any in-flight evaluation.
func (m *Middleware) Cancel(sessionID string) {
	m.evalMu.Lock()
	if m.evalCancel != nil {
		m.evalCancel()
		m.evalCancel = nil
	}
	m.evalMu.Unlock()
	m.primary.Cancel(sessionID)
}

// CancelAll delegates to the primary agent and aborts any in-flight evaluation.
func (m *Middleware) CancelAll() {
	m.evalMu.Lock()
	if m.evalCancel != nil {
		m.evalCancel()
		m.evalCancel = nil
	}
	m.evalMu.Unlock()
	m.primary.CancelAll()
}

// SkipCoach interrupts the current evaluation without disabling the replacer
// permanently. It sets a flag that the Run() loop checks so the next (or
// in-flight) evaluation is treated as a user skip rather than an error.
func (m *Middleware) SkipCoach(sessionID string) {
	m.skipCoach.Store(true)
	m.evalMu.Lock()
	if m.evalCancel != nil {
		m.evalCancel()
		m.evalCancel = nil
	}
	m.evalMu.Unlock()
}

// IsSessionBusy delegates to the primary agent, but also reports busy when the
// replacer itself is evaluating.
func (m *Middleware) IsSessionBusy(sessionID string) bool {
	return m.busy.Load() || m.primary.IsSessionBusy(sessionID)
}

// IsBusy reports true when either the primary agent or the replacer itself is
// actively working.
func (m *Middleware) IsBusy() bool {
	return m.busy.Load() || m.primary.IsBusy()
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

// coachPrompt prefixes auto-generated follow-up prompts with a coach label so
// they are visually distinguishable from real user input in the TUI.
func coachPrompt(prompt string) string {
	if strings.HasPrefix(prompt, "[") {
		return prompt
	}
	return "[Coach] " + prompt
}

// coachPrefix is the marker injected by coachPrompt. It is used by
// isDuplicateCoachPrompt to identify prior coach-generated user prompts.
const coachPrefix = "[Coach] "

// isDuplicateCoachPrompt scans the session history for previous coach prompts
// and returns true if the supplied prompt is substantially identical to one
// that was already injected earlier in the session. The comparison is
// case-insensitive and ignores leading/trailing whitespace.
func isDuplicateCoachPrompt(ctx context.Context, svc message.Service, sessionID, prompt string) bool {
	if svc == nil || prompt == "" {
		return false
	}
	listCtx, listCancel := context.WithTimeout(ctx, 5*time.Second)
	defer listCancel()
	type listResult struct {
		msgs []message.Message
		err  error
	}
	listDone := make(chan listResult, 1)
	go func() {
		msgs, err := svc.List(listCtx, sessionID)
		listDone <- listResult{msgs: msgs, err: err}
	}()
	var msgs []message.Message
	var err error
	select {
	case <-listCtx.Done():
		select {
		case lr := <-listDone:
			msgs, err = lr.msgs, lr.err
		default:
			slog.Warn("Replacer deduplication list timed out", "session_id", sessionID)
			return false
		}
	case lr := <-listDone:
		msgs, err = lr.msgs, lr.err
	}
	if err != nil {
		slog.Warn("Replacer failed to list messages for deduplication", "session_id", sessionID, "error", err)
		return false
	}
	normalized := strings.ToLower(strings.TrimSpace(prompt))
	for _, msg := range msgs {
		if msg.Role != message.User {
			continue
		}
		text := messageText(msg)
		if !strings.HasPrefix(text, coachPrefix) {
			continue
		}
		prev := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(text, coachPrefix)))
		if prev == normalized {
			return true
		}
	}
	return false
}
