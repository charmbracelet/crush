package toolcoach

import (
	"context"
	"sync"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/message"
)

// Middleware wraps a primary SessionAgent with real-time tool pattern coaching.
// It intercepts SetTools to store the tool list, then wraps each tool with a
// coachedTool at the start of each Run call so the session ID is known.
type Middleware struct {
	primary  agent.SessionAgent
	cfg      ToolcoachConfig
	messages message.Service
	states   *csync.Map[string, *sessionState]
	store    *Store
	tools    []fantasy.AgentTool

	// sessionSeen tracks unique session IDs for progressive coaching.
	// After cfg.AutoRetrySessions unique sessions, intensity auto-switches
	// from tutor to balanced (unless explicitly overridden).
	sessionSeen   map[string]struct{}
	sessionSeenMu sync.RWMutex
}

// NewMiddleware creates a tool pattern coach middleware.
func NewMiddleware(primary agent.SessionAgent, cfg ToolcoachConfig) *Middleware {
	if primary == nil {
		return nil
	}
	return &Middleware{
		primary:     primary,
		cfg:         cfg,
		states:      csync.NewMap[string, *sessionState](),
		sessionSeen: make(map[string]struct{}),
	}
}

// effectiveIntensity returns the coaching intensity to use, accounting for
// auto-switch from tutor to balanced after enough unique sessions.
func (m *Middleware) effectiveIntensity() CoachingIntensity {
	intensity := m.cfg.Intensity
	if intensity == CoachingTutor {
		m.sessionSeenMu.RLock()
		n := len(m.sessionSeen)
		m.sessionSeenMu.RUnlock()
		if n >= m.cfg.AutoRetrySessions {
			return CoachingBalanced
		}
	}
	return intensity
}

// SetMessageService configures the message service used for ephemeral coach
// indicators.
func (m *Middleware) SetMessageService(svc message.Service) {
	m.messages = svc
}

// SetStore configures the store used to persist effectiveness data.
func (m *Middleware) SetStore(store *Store) {
	m.store = store
}

// Run delegates to the primary agent after wrapping tools with the
// session-specific coach state and resetting per-turn counters.
func (m *Middleware) Run(ctx context.Context, call agent.SessionAgentCall) (*fantasy.AgentResult, error) {
	if call.ToolcoachEnabled != nil && !*call.ToolcoachEnabled {
		return m.primary.Run(ctx, call)
	}
	if !m.cfg.Enabled {
		return m.primary.Run(ctx, call)
	}

	// Reset per-turn counters for this session.
	state := m.getOrCreateState(call.SessionID)
	state.resetTurnCounters()

	// Wrap tools with the session-specific state.
	wrapped := m.wrapToolsForSession(call.SessionID, state)
	m.primary.SetTools(wrapped)

	result, err := m.primary.Run(ctx, call)

	// Export metrics at session end.
	state.exportMetrics(call.SessionID, m.store)

	return result, err
}

// SetTools stores the tool list so it can be wrapped per-session in Run.
func (m *Middleware) SetTools(tools []fantasy.AgentTool) {
	m.tools = tools
	m.primary.SetTools(tools)
}

// wrapToolsForSession returns the tool list wrapped with coachedTools for the
// given session.
func (m *Middleware) wrapToolsForSession(sessionID string, state *sessionState) []fantasy.AgentTool {
	if len(m.tools) == 0 {
		return m.tools
	}
	intensity := m.effectiveIntensity()
	wrapped := make([]fantasy.AgentTool, len(m.tools))
	for i, t := range m.tools {
		wrapped[i] = newCoachedTool(t, state, sessionID, m.messages, m.cfg, intensity)
	}
	return wrapped
}

// getOrCreateState returns the session state for the given session ID,
// creating one if necessary.
func (m *Middleware) getOrCreateState(sessionID string) *sessionState {
	if sessionID == "" {
		// Fallback: return a transient state that won't be reused.
		return newSessionState()
	}
	state, ok := m.states.Get(sessionID)
	if ok && state != nil {
		return state
	}
	state = newSessionState()
	if m.cfg.AdaptiveSeverity {
		state.loadAdaptiveSeverity(m.store, m.cfg.EffectivenessLookbackDays)
	}
	// Track unique session for progressive coaching.
	m.sessionSeenMu.Lock()
	m.sessionSeen[sessionID] = struct{}{}
	m.sessionSeenMu.Unlock()
	m.states.Set(sessionID, state)
	return state
}

// GetCoachSummary returns a coaching summary for the given session, suitable
// for injection into the critic prompt. It implements the
// critic.CoachSummaryProvider interface.
func (m *Middleware) GetCoachSummary(sessionID string) string {
	state, ok := m.states.Get(sessionID)
	if !ok || state == nil {
		return ""
	}
	return state.buildCoachSummary()
}

// SetModels delegates to the primary agent.
func (m *Middleware) SetModels(large agent.Model, small agent.Model) {
	m.primary.SetModels(large, small)
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

// SkipCoach delegates to the primary agent if it supports skipping coach
// evaluation. This allows the skip signal to propagate through the wrapper
// chain when toolcoach sits outside the replacer.
func (m *Middleware) SkipCoach(sessionID string) {
	if s, ok := m.primary.(interface{ SkipCoach(string) }); ok {
		s.SkipCoach(sessionID)
	}
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
