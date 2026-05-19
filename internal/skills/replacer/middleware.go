package replacer

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
)

// Visual indicator constants.
const (
	coachEvalLabel     = "Coach is evaluating..."
	coachStopLabel     = "✅ Coach is satisfied"
	coachContinueLabel = "🔄 Coach suggests continuing..."
)

// Display durations for transition indicators. Variables so tests can override.
var (
	coachStopDisplay     = 1500 * time.Millisecond
	coachContinueDisplay = 500 * time.Millisecond
)

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
		if delErr := m.messages.Delete(ctx, oldSpinnerID); delErr != nil {
			slog.Warn("Replacer failed to delete stale eval indicator", "session_id", call.SessionID, "error", delErr)
		}
	}

	slog.Info("Replacer started", "session_id", call.SessionID, "max_iterations", m.cfg.MaxIterations)

	// Run the primary agent first.
	result, err, evalMsg := m.runPrimaryWithEval(ctx, call)
	if err != nil {
		slog.Info("Replacer primary returned error", "session_id", call.SessionID, "error", err)
		return result, err
	}
	if result == nil {
		slog.Info("Replacer primary returned nil", "session_id", call.SessionID)
		return nil, nil
	}

	// Replacement agent loop: evaluate and potentially continue.
	for iteration := 0; iteration < m.cfg.MaxIterations; iteration++ {
		evalCtx, cancel := context.WithCancel(ctx)
		m.evalMu.Lock()
		m.evalCancel = cancel
		m.evalMu.Unlock()

		decision, evalErr := m.evaluate(evalCtx, call.SessionID, iteration)

		m.evalMu.Lock()
		m.evalCancel = nil
		m.evalMu.Unlock()

		// Clean up the evaluating indicator.
		if evalMsg.ID != "" && m.messages != nil {
			if delErr := m.messages.Delete(ctx, evalMsg.ID); delErr != nil {
				slog.Warn("Replacer failed to delete eval indicator", "session_id", call.SessionID, "error", delErr)
			}
			evalMsg.ID = ""
			m.evalMu.Lock()
			m.evalSpinnerID = ""
			m.evalMu.Unlock()
		}

		if evalErr != nil {
			slog.Warn("Replacer evaluation failed", "session_id", call.SessionID, "error", evalErr)
			return result, nil
		}
		if decision == nil || decision.Action == "stop" {
			if decision != nil && decision.Action == "stop" {
				m.flashStopIndicator(ctx, call.SessionID)
			}
			return result, nil
		}

		// Show a brief continue indicator before re-running the primary.
		m.flashContinueIndicator(ctx, call.SessionID)

		// Re-run primary agent with the follow-up prompt.
		newCall := call
		newCall.Prompt = coachPrompt(decision.Prompt)
		newCall.Attachments = nil
		result, err, evalMsg = m.runPrimaryWithEval(ctx, newCall)
		if err != nil {
			return result, err
		}
		if result == nil {
			return nil, nil
		}
	}

	slog.Info("Replacer max iterations reached", "session_id", call.SessionID, "max_iterations", m.cfg.MaxIterations)
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
						SpinnerLabel: coachEvalLabel,
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
			SpinnerLabel: coachEvalLabel,
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

	msgs, err := m.messages.List(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}

	promptText, err := BuildReplacerPrompt(msgs)
	if err != nil {
		return nil, fmt.Errorf("build prompt: %w", err)
	}

	model, err := m.resolveModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve model: %w", err)
	}

	m.busy.Store(true)
	ctx2, cancel := context.WithTimeout(ctx, m.cfg.Timeout)
	maxTokens := int64(512)
	temp := 0.3
	resp, err := model.Generate(ctx2, fantasy.Call{
		Prompt:          fantasy.Prompt{fantasy.NewUserMessage(promptText)},
		MaxOutputTokens: &maxTokens,
		Temperature:     &temp,
	})
	cancel()
	m.busy.Store(false)
	if err != nil {
		return nil, fmt.Errorf("generate: %w", err)
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

// flashStopIndicator shows a brief "coach is satisfied" spinner and then
// removes it after a short delay so the user gets feedback when the coach
// decides to stop.
func (m *Middleware) flashStopIndicator(ctx context.Context, sessionID string) {
	if m.messages == nil {
		return
	}
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
}

// flashContinueIndicator shows a brief transition spinner so the user sees
// feedback when the coach decides to continue before the primary agent starts
// its next turn.
func (m *Middleware) flashContinueIndicator(ctx context.Context, sessionID string) {
	if m.messages == nil {
		return
	}
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
