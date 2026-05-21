package dialog

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/oauth/xai"
	"github.com/charmbracelet/crush/internal/ui/common"
)

// NewOAuthXAI creates an OAuth dialog for the xAI browser sign-in flow.
func NewOAuthXAI(
	com *common.Common,
	isOnboarding bool,
	provider catwalk.Provider,
	model config.SelectedModel,
	modelType config.SelectedModelType,
) (*OAuth, tea.Cmd) {
	return newOAuth(com, isOnboarding, provider, model, modelType, &OAuthXAI{})
}

// OAuthXAI drives the xAI authorization-code (PKCE) browser flow. Unlike the
// device-code providers, there is no user code to display: the dialog opens a
// browser and waits for the loopback OAuth callback.
type OAuthXAI struct {
	authorizer *xai.Authorizer
	cancelFunc func()
}

var _ OAuthProvider = (*OAuthXAI)(nil)

func (m *OAuthXAI) name() string { return "xAI" }

func (m *OAuthXAI) initiateAuth() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	authorizer, err := xai.NewAuthorizer(ctx)
	if err != nil {
		return ActionOAuthErrored{Error: fmt.Errorf("failed to initiate xAI OAuth: %w", err)}
	}
	m.authorizer = authorizer

	// No DeviceCode/UserCode: VerificationURL is the authorization URL.
	return ActionInitiateOAuth{
		VerificationURL: authorizer.AuthURL,
	}
}

func (m *OAuthXAI) startPolling(_ string, _ int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())
		m.cancelFunc = cancel

		token, err := m.authorizer.Wait(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil // cancelled, don't report error.
			}
			return ActionOAuthErrored{Error: err}
		}

		return ActionCompleteOAuth{Token: token}
	}
}

func (m *OAuthXAI) stopPolling() tea.Msg {
	if m.cancelFunc != nil {
		m.cancelFunc()
	}
	if m.authorizer != nil {
		m.authorizer.Close()
	}
	return nil
}
