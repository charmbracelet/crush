package dialog

import (
	"context"
	"errors"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/v3/pkg/catwalk"
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
	return newOAuth(com, isOnboarding, provider, model, modelType, newOAuthGeneric(auth, provider.Name))
}

// OAuthGeneric drives a spec-declared flow through the shared OAuth dialog.
// The dialog's lifetime is one context, made with the dialog: cancelling it
// aborts discovery, polling and pasted-code exchange, and closes the flow
// however far along it got. No command writes a field another one reads.
type OAuthGeneric struct {
	spec    *oauth.AuthSpec
	display string
	ctx     context.Context
	cancel  context.CancelFunc
	// start runs the flow once; every command that needs the flow calls it
	// and gets the same result.
	start func() (*startedFlow, error)
}

// startedFlow is a running flow with the prompt that opened it.
type startedFlow struct {
	flow   generic.Flow
	prompt generic.Prompt
}

var _ OAuthProvider = (*OAuthGeneric)(nil)

func newOAuthGeneric(spec *oauth.AuthSpec, display string) *OAuthGeneric {
	m := &OAuthGeneric{spec: spec, display: display}
	m.ctx, m.cancel = context.WithCancel(context.Background())
	m.start = sync.OnceValues(m.startFlow)
	return m
}

func (m *OAuthGeneric) name() string {
	if m.display != "" {
		return m.display
	}
	return "the provider"
}

// startFlow starts the grant the spec selects; generic owns the fallback.
func (m *OAuthGeneric) startFlow() (*startedFlow, error) {
	ctx, cancel := context.WithTimeout(m.ctx, 30*time.Second)
	defer cancel()
	flow, prompt, err := generic.StartFlow(ctx, m.spec, m.name())
	if err != nil {
		return nil, err
	}
	// A flow outliving its dialog would keep serving its callback port.
	context.AfterFunc(m.ctx, flow.Close)
	return &startedFlow{flow: flow, prompt: prompt}, nil
}

func (m *OAuthGeneric) initiateAuth() tea.Msg {
	started, err := m.start()
	if err != nil {
		if m.ctx.Err() != nil {
			return nil // dismissed, don't report error.
		}
		return ActionOAuthErrored{Error: err}
	}
	prompt := started.prompt
	return ActionInitiateOAuth{
		DeviceCode:      prompt.DeviceCode,
		UserCode:        prompt.UserCode,
		VerificationURL: prompt.URL,
		ExpiresIn:       prompt.ExpiresIn,
		Interval:        prompt.Interval,
	}
}

func (m *OAuthGeneric) startPolling(string, int) tea.Cmd {
	return func() tea.Msg {
		started, err := m.start()
		if err != nil {
			return ActionOAuthErrored{Error: err}
		}
		token, err := started.flow.Wait(m.ctx)
		if err != nil {
			if m.ctx.Err() != nil || errors.Is(err, generic.ErrFlowClosed) {
				return nil // cancelled, don't report error.
			}
			return ActionOAuthErrored{Error: err}
		}
		return ActionCompleteOAuth{Token: token}
	}
}

func (m *OAuthGeneric) stopPolling() tea.Msg {
	m.cancel()
	return nil
}

func (m *OAuthGeneric) supportsCodeEntry() bool {
	// The paste fallback belongs to the browser flow: a device spec has a
	// code to enter on the provider's page, not one to paste back.
	return m.spec.FlowMode() != oauth.AuthFlowDevice
}

func (m *OAuthGeneric) submitCode(input string) tea.Cmd {
	return func() tea.Msg {
		started, err := m.start()
		if err != nil {
			return ActionOAuthErrored{Error: err}
		}
		completer, ok := started.flow.(generic.CodeCompleter)
		if !ok {
			return ActionOAuthErrored{Error: errors.New("this sign-in has no code to paste")}
		}
		token, err := completer.CompleteWithCode(m.ctx, input)
		if err != nil {
			if m.ctx.Err() != nil {
				return nil // cancelled, don't report error.
			}
			return ActionOAuthErrored{Error: err}
		}
		return ActionCompleteOAuth{Token: token}
	}
}
