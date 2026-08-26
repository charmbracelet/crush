package dialog

import (
	"context"
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	openaiauth "github.com/charmbracelet/crush/internal/oauth/openai"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/pkg/browser"
)

// NewOAuthOpenAI creates the browser-based ChatGPT/Codex OAuth dialog.
func NewOAuthOpenAI(com *common.Common, isOnboarding bool, provider catwalk.Provider, model config.SelectedModel, modelType config.SelectedModelType) (*OAuth, tea.Cmd) {
	return newOAuth(com, isOnboarding, provider, model, modelType, &OAuthOpenAI{})
}

// OAuthOpenAI implements the ChatGPT/Codex browser OAuth provider.
type OAuthOpenAI struct {
	flow       *openaiauth.BrowserFlow
	cancelFunc func()
}

var _ OAuthProvider = (*OAuthOpenAI)(nil)

func (m *OAuthOpenAI) name() string { return "ChatGPT/Codex" }

func (m *OAuthOpenAI) initiateAuth() tea.Msg {
	flow, err := openaiauth.StartBrowserFlow(context.Background())
	if err != nil {
		return ActionOAuthErrored{Error: fmt.Errorf("failed to start OAuth callback: %w", err)}
	}
	m.flow = flow
	m.cancelFunc = flow.Close
	return ActionInitiateOAuth{VerificationURL: flow.URL()}
}

func (m *OAuthOpenAI) startPolling(_ string, _ int) tea.Cmd {
	return func() tea.Msg {
		if m.flow == nil {
			return ActionOAuthErrored{Error: fmt.Errorf("OAuth flow was not initialized")}
		}
		if err := browser.OpenURL(m.flow.URL()); err != nil {
			m.flow.Close()
			return ActionOAuthErrored{Error: fmt.Errorf("failed to open browser: %w", err)}
		}
		token, err := m.flow.Wait(context.Background())
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
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
