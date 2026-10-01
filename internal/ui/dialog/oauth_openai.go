package dialog

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/oauth/openai"
	"github.com/charmbracelet/crush/internal/ui/common"
)

// NewOAuthOpenAI creates an OAuth dialog for signing in with a ChatGPT
// account. Unlike the device-flow providers, the browser flow has no code
// to paste: the user authorizes in the browser and the loopback callback
// delivers the result.
func NewOAuthOpenAI(
	com *common.Common,
	isOnboarding bool,
	provider catwalk.Provider,
	model config.SelectedModel,
	modelType config.SelectedModelType,
) (*OAuth, tea.Cmd) {
	return newOAuth(com, isOnboarding, provider, model, modelType, &OAuthOpenAI{})
}

type OAuthOpenAI struct {
	flow       *openai.BrowserFlow
	cancelFunc context.CancelFunc
}

var _ OAuthProvider = (*OAuthOpenAI)(nil)

func (m *OAuthOpenAI) name() string {
	return "ChatGPT"
}

func (m *OAuthOpenAI) initiateAuth() tea.Msg {
	flow, err := openai.StartBrowserFlow()
	if err != nil {
		return ActionOAuthErrored{Error: fmt.Errorf("failed to start browser auth: %w", err)}
	}
	m.flow = flow

	return ActionInitiateOAuth{
		VerificationURL: flow.StartURL(),
	}
}

func (m *OAuthOpenAI) startPolling(_ string, _ int) tea.Cmd {
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

func (m *OAuthOpenAI) stopPolling() tea.Msg {
	if m.cancelFunc != nil {
		m.cancelFunc()
	}
	if m.flow != nil {
		m.flow.Close()
	}
	return nil
}
