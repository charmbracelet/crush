package dialog

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/honcho"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/util"
)

// NewOAuthHoncho creates an OAuth dialog for connecting Honcho memory.
//
// Unlike the other flows this one authenticates no LLM provider and
// selects no model: it stores a credential that lets Crush remember
// across sessions, then closes. The provider and model fields the
// shared dialog carries are unused here and left zero.
func NewOAuthHoncho(com *common.Common) (*OAuth, tea.Cmd) {
	return newOAuth(com, false, catwalk.Provider{}, config.SelectedModel{}, "", &OAuthHoncho{})
}

// OAuthHoncho drives the Honcho browser flow.
//
// The flow is held in an [atomic.Pointer] because the dialog can be
// dismissed while authorization is still starting, which reads the
// field from a different goroutine than the one that set it. The
// token is held the same way: Honcho issues its own credential type
// rather than an LLM provider API key, so it never travels through
// the dialog's shared token field.
type OAuthHoncho struct {
	flow  atomic.Pointer[honcho.AuthFlow]
	token atomic.Pointer[honcho.OAuthToken]
}

var _ OAuthProvider = (*OAuthHoncho)(nil)

func (m *OAuthHoncho) name() string {
	return "Honcho"
}

func (m *OAuthHoncho) initiateAuth() tea.Msg {
	baseURL := honcho.Resolve(honcho.Config{}).BaseURL
	if baseURL == "" {
		baseURL = honcho.DefaultBaseURL
	}

	flow, err := honcho.StartAuth(context.Background(), baseURL)
	if err != nil {
		return ActionOAuthErrored{Error: fmt.Errorf("failed to start browser auth: %w", err)}
	}
	m.flow.Store(flow)

	return ActionInitiateOAuth{VerificationURL: flow.URL}
}

// startPolling waits for the browser redirect. The device-flow
// arguments do not apply: this flow is woken by its own callback
// listener rather than by polling a device code.
func (m *OAuthHoncho) startPolling(_ string, _ int) tea.Cmd {
	return func() tea.Msg {
		flow := m.flow.Load()
		if flow == nil {
			return nil
		}
		token, err := flow.Wait(context.Background())
		switch {
		case errors.Is(err, context.Canceled):
			// The dialog was dismissed; there is nobody left to tell.
			return nil
		case err != nil:
			return ActionOAuthErrored{Error: err}
		}
		m.token.Store(token)
		// The shared token field carries LLM provider API keys. This
		// credential is Honcho's own type, so it stays on the
		// provider and persist reads it from there.
		return ActionCompleteOAuth{}
	}
}

// stopPolling closes the callback listener, which also wakes a Wait
// still blocked on a redirect that is never going to arrive.
func (m *OAuthHoncho) stopPolling() tea.Msg {
	if flow := m.flow.Load(); flow != nil {
		flow.Close()
	}
	return nil
}

// persist writes the token so later sessions can use it.
func (m *OAuthHoncho) persist(_ *OAuth) tea.Cmd {
	return func() tea.Msg {
		token := m.token.Load()
		if token == nil {
			return oauthSaveErrMsg{err: errors.New("no Honcho credential to save")}
		}
		if err := honcho.SaveToken(token); err != nil {
			return oauthSaveErrMsg{err: fmt.Errorf("failed to save Honcho credential: %w", err)}
		}
		return oauthSaveDoneMsg{}
	}
}

func (m *OAuthHoncho) savingMessage() string { return " Connecting memory..." }

// completed closes the dialog and brings memory up for the running
// session.
//
// The credential is on disk by now, so refreshing the agent is enough:
// the rebuild re-resolves the memory config, starts the service, and
// attaches the memory tools. Without this the user would have to
// restart to use what they just connected.
func (m *OAuthHoncho) completed(d *OAuth) Action {
	return ActionCloseOAuth{Cmd: tea.Batch(
		m.stopPolling,
		refreshMemory(d, "Memory connected."),
	)}
}

// refreshMemory rebuilds the agent so a memory connect or disconnect
// takes effect now, reporting success or the reason it did not.
func refreshMemory(d *OAuth, okMsg string) tea.Cmd {
	return func() tea.Msg {
		if d == nil || d.com == nil || d.com.Workspace == nil {
			return util.InfoMsg{Type: util.InfoTypeInfo, Msg: okMsg}
		}
		if err := d.com.Workspace.UpdateAgentModel(context.Background()); err != nil {
			return util.InfoMsg{
				Type: util.InfoTypeWarn,
				Msg:  okMsg + " Restart Crush to start using it: " + err.Error(),
			}
		}
		return util.InfoMsg{Type: util.InfoTypeInfo, Msg: okMsg}
	}
}
