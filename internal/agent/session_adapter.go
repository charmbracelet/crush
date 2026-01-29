package agent

import (
	"context"
	"sync"

	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/plugin"
)

// SessionInfoAdapter adapts coordinator session/model data to plugin.SessionInfoProvider.
type SessionInfoAdapter struct {
	sessions  session.Service
	sessionID string

	mu       sync.RWMutex
	model    string
	provider string
}

// NewSessionInfoAdapter creates a new adapter for session info.
func NewSessionInfoAdapter(sessions session.Service) *SessionInfoAdapter {
	return &SessionInfoAdapter{
		sessions: sessions,
	}
}

// SetSessionID sets the current session ID.
func (a *SessionInfoAdapter) SetSessionID(sessionID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sessionID = sessionID
}

// SetModel sets the current model and provider.
func (a *SessionInfoAdapter) SetModel(model, provider string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.model = model
	a.provider = provider
}

// SessionInfo returns the current session info.
func (a *SessionInfoAdapter) SessionInfo() *plugin.SessionInfo {
	a.mu.RLock()
	sessionID := a.sessionID
	model := a.model
	provider := a.provider
	a.mu.RUnlock()

	if sessionID == "" {
		return nil
	}

	// Get session from service.
	sess, err := a.sessions.Get(context.Background(), sessionID)
	if err != nil {
		return nil
	}

	return &plugin.SessionInfo{
		Model:    model,
		Provider: provider,
		CostUSD:  sess.Cost,
		Tokens: plugin.TokenUsage{
			Input:  sess.PromptTokens,
			Output: sess.CompletionTokens,
			// Note: CacheRead and CacheWrite not stored in session.
			// They would need to be tracked separately if needed.
		},
	}
}
