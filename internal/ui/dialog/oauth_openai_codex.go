package dialog

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/oauth/openai_codex"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/pkg/browser"
)

func NewOAuthOpenAICodex(
	com *common.Common,
	isOnboarding bool,
	provider catwalk.Provider,
	model config.SelectedModel,
	modelType config.SelectedModelType,
) (*OAuth, tea.Cmd) {
	return newOAuth(com, isOnboarding, provider, model, modelType, &OAuthOpenAICodex{})
}

type OAuthOpenAICodex struct {
	flow       *openai_codex.AuthFlow
	server     *openai_codex.CallbackServer
	cancelFunc func()
}

var _ OAuthProvider = (*OAuthOpenAICodex)(nil)

func (m *OAuthOpenAICodex) name() string {
	return openai_codex.ProviderName
}

func (m *OAuthOpenAICodex) initiateAuth() tea.Msg {
	flow, err := openai_codex.StartAuthFlow("crush")
	if err != nil {
		return ActionOAuthErrored{Error: fmt.Errorf("failed to initialize OpenAI Codex OAuth flow: %w", err)}
	}

	server, err := openai_codex.StartLocalCallbackServer(flow.State)
	if err != nil {
		return ActionOAuthErrored{Error: fmt.Errorf("failed to start OAuth callback server: %w", err)}
	}

	m.flow = flow
	m.server = server

	return ActionInitiateOAuth{
		DeviceCode:      flow.State,
		UserCode:        "Browser Authentication",
		VerificationURL: flow.AuthURL,
		ExpiresIn:       300,
		Interval:        1,
	}
}

func (m *OAuthOpenAICodex) startPolling(_ string, expiresIn int) tea.Cmd {
	return func() tea.Msg {
		if m.flow == nil || m.server == nil {
			return ActionOAuthErrored{Error: fmt.Errorf("OAuth flow is not initialized")}
		}

		ctx, cancel := context.WithCancel(context.Background())
		m.cancelFunc = cancel
		defer m.server.Close(context.Background())

		_ = browser.OpenURL(m.flow.AuthURL)

		waitCtx, waitCancel := context.WithTimeout(ctx, time.Duration(expiresIn)*time.Second)
		defer waitCancel()

		code, err := m.server.WaitForCode(waitCtx)
		if err != nil {
			if waitCtx.Err() != nil {
				return nil
			}
			return ActionOAuthErrored{Error: err}
		}

		token, err := openai_codex.ExchangeAuthorizationCode(waitCtx, code, m.flow.Verifier)
		if err != nil {
			return ActionOAuthErrored{Error: fmt.Errorf("token exchange failed: %w", err)}
		}

		return ActionCompleteOAuth{Token: token}
	}
}

func (m *OAuthOpenAICodex) stopPolling() tea.Msg {
	if m.cancelFunc != nil {
		m.cancelFunc()
	}
	if m.server != nil {
		_ = m.server.Close(context.Background())
	}
	return nil
}
