package dialog

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/oauth/hyper"
	"github.com/charmbracelet/crush/internal/ui/common"
)

func NewOAuthHyper(
	com *common.Common,
	isOnboarding bool,
	provider catwalk.Provider,
	model config.SelectedModel,
	modelType config.SelectedModelType,
) (*OAuth, tea.Cmd) {
	return newOAuth(com, isOnboarding, provider, model, modelType, &OAuthHyper{})
}

type OAuthHyper struct {
	deviceCode string
	expiresIn  int
	cancelFunc context.CancelFunc
}

var _ OAuthProvider = (*OAuthHyper)(nil)

type hyperOAuthStep struct {
	url  string
	code string
}

func (s hyperOAuthStep) instructions() string {
	return "copy the code below and open the browser."
}

func (s hyperOAuthStep) verificationURL() string {
	return s.url
}

func (s hyperOAuthStep) hyperlinkID() string {
	return "id=hyper-verify"
}

func (s hyperOAuthStep) userCode() string {
	return s.code
}

func (s hyperOAuthStep) copyValue() string {
	return s.code
}

func (m *OAuthHyper) name() string {
	return "Hyper"
}

func (m *OAuthHyper) initiateAuth() tea.Msg {
	minimumWait := 750 * time.Millisecond
	startTime := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	authResp, err := hyper.InitiateDeviceAuth(ctx)

	elapsed := time.Since(startTime)
	if elapsed < minimumWait {
		time.Sleep(minimumWait - elapsed)
	}

	if err != nil {
		return ActionOAuthErrored{fmt.Errorf("failed to initiate device auth: %w", err)}
	}

	m.deviceCode = authResp.DeviceCode
	m.expiresIn = authResp.ExpiresIn

	return ActionInitiateOAuth{Step: hyperOAuthStep{
		url:  authResp.VerificationURL,
		code: authResp.UserCode,
	}}
}

func (m *OAuthHyper) waitForAuthorization(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		if m.deviceCode == "" {
			return ActionOAuthErrored{fmt.Errorf("device code is not initialized")}
		}

		ctx, cancel := context.WithCancel(ctx)
		m.cancelFunc = cancel

		refreshToken, err := hyper.PollForToken(ctx, m.deviceCode, m.expiresIn)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return ActionOAuthErrored{err}
		}

		token, err := hyper.ExchangeToken(ctx, refreshToken)
		if err != nil {
			return ActionOAuthErrored{fmt.Errorf("token exchange failed: %w", err)}
		}

		introspect, err := hyper.IntrospectToken(ctx, token.AccessToken)
		if err != nil {
			return ActionOAuthErrored{fmt.Errorf("token introspection failed: %w", err)}
		}
		if !introspect.Active {
			return ActionOAuthErrored{fmt.Errorf("access token is not active")}
		}

		return ActionCompleteOAuth{token}
	}
}

func (m *OAuthHyper) stopAuthorization() tea.Msg {
	if m.cancelFunc != nil {
		m.cancelFunc()
		m.cancelFunc = nil
	}
	return nil
}
