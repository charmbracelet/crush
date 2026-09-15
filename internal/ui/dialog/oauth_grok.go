package dialog

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/oauth/grok"
	"github.com/charmbracelet/crush/internal/ui/common"
)

// NewOAuthGrok creates an OAuth dialog for signing in with a Grok
// account. The sign-in starts with the ChatGPT-style browser flow; when
// the user declines that permission (or the loopback callback is out of
// reach), it falls back to the device flow, showing a code the user
// enters on accounts.x.ai.
func NewOAuthGrok(
	com *common.Common,
	isOnboarding bool,
	provider catwalk.Provider,
	model config.SelectedModel,
	modelType config.SelectedModelType,
) (*OAuth, tea.Cmd) {
	return newOAuth(com, isOnboarding, provider, model, modelType, &OAuthGrok{})
}

type OAuthGrok struct {
	flow       *grok.BrowserFlow
	cancelFunc context.CancelFunc
}

var _ OAuthProvider = (*OAuthGrok)(nil)

func (m *OAuthGrok) name() string {
	return "Grok"
}

func (m *OAuthGrok) initiateAuth() tea.Msg {
	flow, err := grok.StartBrowserFlow()
	if err == nil {
		m.flow = flow

		return ActionInitiateOAuth{
			VerificationURL: flow.StartURL(),
		}
	}

	// The loopback callback listener could not start (port unavailable,
	// remote machine): go straight to the device flow, where the user
	// opens the verification page and enters the code themselves.
	deviceCode, deviceErr := grok.RequestDeviceCode(context.Background())
	if deviceErr != nil {
		return ActionOAuthErrored{Error: fmt.Errorf("failed to start browser auth: %w (device auth: %v)", err, deviceErr)}
	}
	return deviceCodeInitiate(deviceCode)
}

func (m *OAuthGrok) startPolling(deviceCode string, expiresIn int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())
		m.cancelFunc = cancel

		if deviceCode != "" {
			token, err := grok.PollForToken(ctx, deviceCode, expiresIn)
			if err != nil {
				if ctx.Err() != nil {
					return nil // cancelled, don't report error.
				}
				return ActionOAuthErrored{Error: err}
			}
			return ActionCompleteOAuth{Token: token}
		}

		token, err := m.flow.Wait(ctx)
		if err == nil {
			return ActionCompleteOAuth{Token: token}
		}
		if ctx.Err() != nil {
			return nil // cancelled, don't report error.
		}

		// The browser permission was declined (or the callback never
		// arrived): offer the device flow instead, whose code the user
		// enters on accounts.x.ai to finish signing in.
		deviceCode, deviceErr := grok.RequestDeviceCode(ctx)
		if deviceErr != nil {
			return ActionOAuthErrored{Error: err}
		}
		return deviceCodeInitiate(deviceCode)
	}
}

func (m *OAuthGrok) stopPolling() tea.Msg {
	if m.cancelFunc != nil {
		m.cancelFunc()
	}
	if m.flow != nil {
		m.flow.Close()
	}
	return nil
}

func (m *OAuthGrok) supportsCodeEntry() bool {
	return true
}

func (m *OAuthGrok) submitCode(input string) tea.Cmd {
	return func() tea.Msg {
		if m.flow == nil {
			return ActionOAuthErrored{Error: fmt.Errorf("no browser flow in progress")}
		}
		ctx, cancel := context.WithCancel(context.Background())
		m.cancelFunc = cancel

		token, err := m.flow.CompleteWithCode(ctx, input)
		if err != nil {
			if ctx.Err() != nil {
				return nil // cancelled, don't report error.
			}
			return ActionOAuthErrored{Error: err}
		}
		return ActionCompleteOAuth{Token: token}
	}
}

// deviceCodeInitiate turns a device authorization into the message that
// moves the dialog to its code-display state.
func deviceCodeInitiate(deviceCode *grok.DeviceCode) tea.Msg {
	return ActionInitiateOAuth{
		DeviceCode:      deviceCode.DeviceCode,
		UserCode:        deviceCode.UserCode,
		VerificationURL: deviceCode.VerificationURI,
		ExpiresIn:       deviceCode.ExpiresIn,
		Interval:        deviceCode.Interval,
	}
}
