package dialog

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/oauth/antigravity"
	"github.com/charmbracelet/crush/internal/ui/common"
)

// NewOAuthGemini creates an OAuth dialog for signing in with a Google AI
// subscription. Like ChatGPT, this is a browser flow with a loopback
// callback, so there is no code to paste.
func NewOAuthGemini(
	com *common.Common,
	isOnboarding bool,
	provider catwalk.Provider,
	model config.SelectedModel,
	modelType config.SelectedModelType,
) (*OAuth, tea.Cmd) {
	return newOAuth(com, isOnboarding, provider, model, modelType, &OAuthGemini{com: com})
}

type OAuthGemini struct {
	com        *common.Common
	flow       *antigravity.BrowserFlow
	cancelFunc context.CancelFunc
}

var _ OAuthProvider = (*OAuthGemini)(nil)

func (m *OAuthGemini) name() string {
	return "Google"
}

func (m *OAuthGemini) initiateAuth() tea.Msg {
	// Someone already signed in with the Antigravity CLI should not have to
	// visit a browser again: reuse that session when it is there. The
	// browser flow is the fallback, not the first attempt.
	if token, err := antigravity.ImportFromKeyring(context.Background()); err == nil {
		return ActionCompleteOAuth{Token: token}
	}

	// The dialog that got here already asked for the secret when nothing
	// supplied it, so a missing one now means the configuration changed
	// underneath the flow.
	flow, err := antigravity.StartBrowserFlow(m.com.Config().GoogleSubscriptionClientSecret())
	if err != nil {
		return ActionOAuthErrored{Error: fmt.Errorf("failed to start browser auth: %w", err)}
	}
	m.flow = flow

	return ActionInitiateOAuth{
		VerificationURL: flow.StartURL(),
	}
}

func (m *OAuthGemini) startPolling(_ string, _ int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())
		m.cancelFunc = cancel

		token, err := m.flow.Wait(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil // cancelled, don't report error.
			}
			return ActionOAuthErrored{Error: err}
		}

		return ActionCompleteOAuth{Token: token}
	}
}

func (m *OAuthGemini) stopPolling() tea.Msg {
	if m.cancelFunc != nil {
		m.cancelFunc()
	}
	if m.flow != nil {
		m.flow.Close()
	}
	return nil
}
