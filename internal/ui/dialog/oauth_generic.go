package dialog

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/oauth/generic"
	"github.com/charmbracelet/crush/internal/ui/common"
)

// NewOAuthGeneric creates an OAuth dialog for a provider whose flow is
// declared in config, which is how a provider shipped by a plugin signs in.
// The declared mode chooses the grant: a browser flow whose loopback
// callback can fall back to the device flow, or the device flow alone.
func NewOAuthGeneric(
	com *common.Common,
	isOnboarding bool,
	provider catwalk.Provider,
	model config.SelectedModel,
	modelType config.SelectedModelType,
	auth *oauth.AuthSpec,
) (*OAuth, tea.Cmd) {
	return newOAuth(com, isOnboarding, provider, model, modelType, &OAuthGeneric{
		spec:    auth,
		display: provider.Name,
	})
}

// OAuthGeneric drives a spec-declared flow through the shared OAuth dialog.
type OAuthGeneric struct {
	spec       *oauth.AuthSpec
	display    string
	browser    *generic.BrowserFlow
	device     *generic.DeviceFlow
	cancelFunc context.CancelFunc
}

var _ OAuthProvider = (*OAuthGeneric)(nil)

func (m *OAuthGeneric) name() string {
	if m.display != "" {
		return m.display
	}
	return "the provider"
}

// initiateAuth starts the declared grant. A browser flow is opened first,
// because it needs a local listener that may fail, and the device flow
// takes over when it does.
func (m *OAuthGeneric) initiateAuth() tea.Msg {
	if m.spec.FlowMode() == oauth.AuthFlowDevice {
		return m.startDevice(context.Background())
	}

	browser, err := generic.StartBrowser(m.spec, m.name())
	if err != nil {
		if m.spec.FlowMode() == oauth.AuthFlowBrowser {
			return ActionOAuthErrored{Error: err}
		}
		// No loopback listener available: the device flow needs no
		// callback, so it can carry the sign-in.
		return m.startDevice(context.Background())
	}
	m.browser = browser

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	startURL, _, err := browser.Start(ctx)
	if err != nil {
		return ActionOAuthErrored{Error: err}
	}
	return ActionInitiateOAuth{VerificationURL: startURL}
}

// startDevice requests a device authorization and shows its code.
func (m *OAuthGeneric) startDevice(ctx context.Context) tea.Msg {
	flow := generic.StartDevice(m.spec)
	verifyURL, userCode, err := flow.Start(ctx)
	if err != nil {
		return ActionOAuthErrored{
			Error: fmt.Errorf("failed to start authorization: %w", err),
		}
	}
	m.device = flow
	authorization := flow.Authorization()
	return ActionInitiateOAuth{
		DeviceCode:      authorization.DeviceCode,
		UserCode:        userCode,
		VerificationURL: verifyURL,
		ExpiresIn:       authorization.ExpiresIn,
		Interval:        authorization.Interval,
	}
}

func (m *OAuthGeneric) startPolling(deviceCode string, expiresIn int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())
		m.cancelFunc = cancel

		if deviceCode != "" {
			if m.device == nil {
				return ActionOAuthErrored{Error: fmt.Errorf("no device flow in progress")}
			}
			token, err := m.device.Wait(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return nil // cancelled, don't report error.
				}
				return ActionOAuthErrored{Error: err}
			}
			return ActionCompleteOAuth{Token: token}
		}

		if m.browser == nil {
			return ActionOAuthErrored{Error: fmt.Errorf("no browser flow in progress")}
		}
		token, err := m.browser.Wait(ctx)
		if err == nil {
			return ActionCompleteOAuth{Token: token}
		}
		if ctx.Err() != nil {
			return nil // cancelled, don't report error.
		}
		if m.spec.FlowMode() == oauth.AuthFlowBrowser {
			return ActionOAuthErrored{Error: err}
		}

		// The browser callback never landed: offer the device flow, whose
		// code the user enters on the provider's verification page.
		m.browser.Close()
		m.browser = nil
		return m.startDevice(ctx)
	}
}

func (m *OAuthGeneric) stopPolling() tea.Msg {
	if m.cancelFunc != nil {
		m.cancelFunc()
	}
	if m.browser != nil {
		m.browser.Close()
	}
	return nil
}

func (m *OAuthGeneric) supportsCodeEntry() bool {
	// The paste fallback belongs to the browser flow: a device spec has a
	// code to enter on the provider's page, not one to paste back.
	return m.spec.FlowMode() != oauth.AuthFlowDevice
}

func (m *OAuthGeneric) submitCode(input string) tea.Cmd {
	return func() tea.Msg {
		if m.browser == nil {
			return ActionOAuthErrored{Error: fmt.Errorf("no browser flow in progress")}
		}
		ctx, cancel := context.WithCancel(context.Background())
		m.cancelFunc = cancel

		token, err := m.browser.CompleteWithCode(ctx, input)
		if err != nil {
			if ctx.Err() != nil {
				return nil // cancelled, don't report error.
			}
			return ActionOAuthErrored{Error: err}
		}
		return ActionCompleteOAuth{Token: token}
	}
}
