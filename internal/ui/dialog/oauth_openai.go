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

func NewOAuthOpenAI(
	com *common.Common,
	isOnboarding bool,
	provider catwalk.Provider,
	model config.SelectedModel,
	modelType config.SelectedModelType,
) (*OAuth, tea.Cmd) {
	nameLabel := provider.Name
	if nameLabel == "" {
		nameLabel = "OpenAI Codex"
	}
	return newOAuth(com, isOnboarding, provider, model, modelType, &OAuthOpenAI{
		nameLabel: nameLabel,
	})
}

type OAuthOpenAI struct {
	oauthServer *openai.OAuthServer
	nameLabel   string
}

var _ OAuthProvider = (*OAuthOpenAI)(nil)

type openAIOAuthStep struct {
	url string
}

func (s openAIOAuthStep) instructions() string {
	return "open the browser and continue signing in."
}

func (s openAIOAuthStep) verificationURL() string {
	return s.url
}

func (s openAIOAuthStep) hyperlinkID() string {
	return "id=openai-verify"
}

func (m *OAuthOpenAI) name() string {
	return m.nameLabel
}

func (m *OAuthOpenAI) initiateAuth() tea.Msg {
	oauthServer, err := openai.NewOAuthServer()
	if err != nil {
		return ActionOAuthErrored{Error: fmt.Errorf("failed to initiate oauth server: %w", err)}
	}

	m.oauthServer = oauthServer

	return ActionInitiateOAuth{Step: openAIOAuthStep{
		url: m.oauthServer.AuthURL(),
	}}
}

func (m *OAuthOpenAI) waitForAuthorization(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		if m.oauthServer == nil {
			return ActionOAuthErrored{Error: fmt.Errorf("oauth server is not initialized")}
		}

		go func() {
			_ = m.oauthServer.Start()
		}()

		if err := m.oauthServer.WaitForAuthorization(ctx); err != nil {
			return ActionOAuthErrored{Error: fmt.Errorf("failed to wait for authorization: %w", err)}
		}

		bundle := m.oauthServer.AuthBundle()
		if bundle == nil {
			return ActionOAuthErrored{Error: fmt.Errorf("authorization failed")}
		}

		return ActionCompleteOAuth{Token: bundle.TokenData.ToOAuthToken(bundle.APIKey)}
	}
}

func (m *OAuthOpenAI) stopAuthorization() tea.Msg {
	if m.oauthServer == nil {
		return nil
	}

	_ = m.oauthServer.Stop()
	return nil
}
