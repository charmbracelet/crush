package dialog

import (
	"context"
	"errors"
	"fmt"
	"sync"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/orcarouter"
	"github.com/charmbracelet/crush/internal/ui/common"
)

// NewOAuthOrcaRouter creates the OrcaRouter loopback PKCE dialog.
func NewOAuthOrcaRouter(
	com *common.Common,
	isOnboarding bool,
	provider catwalk.Provider,
	model config.SelectedModel,
	modelType config.SelectedModelType,
) (*OAuth, tea.Cmd) {
	return newOAuth(com, isOnboarding, provider, model, modelType, &OAuthOrcaRouter{})
}

// OAuthOrcaRouter adapts OrcaRouter's authorization-code flow to the shared
// Crush OAuth dialog.
type OAuthOrcaRouter struct {
	mu            sync.Mutex
	authorization *orcarouter.Authorization
}

var _ OAuthProvider = (*OAuthOrcaRouter)(nil)

func (m *OAuthOrcaRouter) name() string {
	return "OrcaRouter"
}

func (m *OAuthOrcaRouter) initiateAuth() tea.Msg {
	authorization, err := orcarouter.StartAuthorization()
	if err != nil {
		return ActionOAuthErrored{Error: fmt.Errorf("failed to initiate OrcaRouter PKCE: %w", err)}
	}
	m.mu.Lock()
	m.authorization = authorization
	m.mu.Unlock()

	return ActionInitiateOAuth{
		ExpiresIn:       600,
		VerificationURL: authorization.URL,
	}
}

func (m *OAuthOrcaRouter) startPolling(_ string, _ int) tea.Cmd {
	return func() tea.Msg {
		m.mu.Lock()
		authorization := m.authorization
		m.mu.Unlock()
		if authorization == nil {
			return ActionOAuthErrored{Error: fmt.Errorf("OrcaRouter PKCE was not initialized")}
		}

		key, scope, err := authorization.Wait(context.Background())
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return ActionOAuthErrored{Error: err}
		}
		return ActionCompleteOAuth{APIKey: key, Scope: scope}
	}
}

func (m *OAuthOrcaRouter) stopPolling() tea.Msg {
	m.mu.Lock()
	authorization := m.authorization
	m.authorization = nil
	m.mu.Unlock()
	if authorization != nil {
		authorization.Cancel()
	}
	return nil
}
