package dialog

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/oauth/copilot"
	"github.com/charmbracelet/crush/internal/ui/common"
)

func NewOAuthCopilot(
	com *common.Common,
	isOnboarding bool,
	provider catwalk.Provider,
	model config.SelectedModel,
	modelType config.SelectedModelType,
) (*OAuth, tea.Cmd) {
	return newOAuth(com, isOnboarding, provider, model, modelType, &OAuthCopilot{})
}

type OAuthCopilot struct {
	deviceCode *copilot.DeviceCode
	cancelFunc context.CancelFunc
}

var _ OAuthProvider = (*OAuthCopilot)(nil)

type copilotOAuthStep struct {
	url  string
	code string
}

func (s copilotOAuthStep) instructions() string {
	return "copy the code below and open the browser."
}

func (s copilotOAuthStep) verificationURL() string {
	return s.url
}

func (s copilotOAuthStep) hyperlinkID() string {
	return "id=copilot-verify"
}

func (s copilotOAuthStep) userCode() string {
	return s.code
}

func (s copilotOAuthStep) copyValue() string {
	return s.code
}

func (m *OAuthCopilot) name() string {
	return "GitHub Copilot"
}

func (m *OAuthCopilot) initiateAuth() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	deviceCode, err := copilot.RequestDeviceCode(ctx)
	if err != nil {
		return ActionOAuthErrored{Error: fmt.Errorf("failed to initiate device auth: %w", err)}
	}

	m.deviceCode = deviceCode

	return ActionInitiateOAuth{Step: copilotOAuthStep{
		url:  deviceCode.VerificationURI,
		code: deviceCode.UserCode,
	}}
}

func (m *OAuthCopilot) waitForAuthorization(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		if m.deviceCode == nil {
			return ActionOAuthErrored{Error: fmt.Errorf("device code is not initialized")}
		}

		ctx, cancel := context.WithCancel(ctx)
		m.cancelFunc = cancel

		token, err := copilot.PollForToken(ctx, m.deviceCode)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return ActionOAuthErrored{Error: err}
		}

		return ActionCompleteOAuth{Token: token}
	}
}

func (m *OAuthCopilot) stopAuthorization() tea.Msg {
	if m.cancelFunc != nil {
		m.cancelFunc()
		m.cancelFunc = nil
	}
	return nil
}
