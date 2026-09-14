package login

import (
	"context"
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

// fakeCodeEntryFlow is a browser flow stub whose CompleteWithCode accepts
// exactly one good code; every other input fails.
type fakeCodeEntryFlow struct{}

func (fakeCodeEntryFlow) Start(context.Context) (string, string, error) {
	return "https://auth.example/authorize", "", nil
}

func (fakeCodeEntryFlow) Wait(context.Context) (*oauth.Token, error) {
	// The callback never lands in these tests; only the paste does.
	<-context.Background().Done()
	return nil, nil
}

func (fakeCodeEntryFlow) Close() {}

func (fakeCodeEntryFlow) CompleteWithCode(_ context.Context, input string) (*oauth.Token, error) {
	if input != "good-code" {
		return nil, errors.New("invalid code")
	}
	return &oauth.Token{AccessToken: "at-pasted"}, nil
}

// step drives one message through the auth model and returns the updated
// model.
func step(t *testing.T, m authModel, msg tea.Msg) authModel {
	t.Helper()
	next, _ := m.Update(msg)
	model, ok := next.(authModel)
	require.True(t, ok, "Update must return an authModel, got %T", next)
	return model
}

// waitingModel returns an auth model driven to its waiting state with the
// paste field up, ready for key or paste messages.
func waitingModel(t *testing.T) authModel {
	t.Helper()
	m := newAuthModel(PlatformGrok, func() flow { return fakeCodeEntryFlow{} })
	m = step(t, m, authReadyMsg{
		flow: fakeCodeEntryFlow{},
		url:  "https://auth.example/authorize",
	})
	require.True(t, m.codeEntry, "a code-entry flow with no user code must show the paste field")
	require.True(t, m.keymap.Submit.Enabled(), "submit must be enabled while waiting with the paste field")
	return m
}

func TestAuthTUIPasteFieldShowsForCodeEntryFlows(t *testing.T) {
	m := waitingModel(t)
	view := m.content()
	require.Contains(t, view, "Enter the code from the page")
	require.Contains(t, view, "Paste the code from the browser...")
}

func TestAuthTUIPasteFieldHiddenForDeviceFlows(t *testing.T) {
	m := newAuthModel(PlatformCopilot, func() flow { return &copilotFlow{} })
	m = step(t, m, authReadyMsg{
		flow:     &copilotFlow{},
		url:      "https://example/verify",
		userCode: "ABCD-1234",
	})
	require.False(t, m.codeEntry, "a user code to copy out means no paste field")
	require.False(t, m.keymap.Submit.Enabled())
	require.NotContains(t, m.content(), "Enter the code")
}

func TestAuthTUITypeAndPasteIntoField(t *testing.T) {
	m := waitingModel(t)

	m = step(t, m, tea.KeyPressMsg{Code: 'h', Text: "h"})
	m = step(t, m, tea.PasteMsg{Content: "ello"})
	require.Equal(t, "hello", m.codeInput.Value())
}

func TestAuthTUISubmitCodeSuccess(t *testing.T) {
	m := waitingModel(t)

	for _, r := range "good-code" {
		m = step(t, m, tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd, "enter with a value must submit the code")

	msg := cmd()
	result, ok := msg.(authCodeResultMsg)
	require.True(t, ok, "the submit command must produce a code result, got %T", msg)
	require.NoError(t, result.err)
	require.Equal(t, "at-pasted", result.token.AccessToken)

	model, _ := next.(authModel).Update(result)
	m = model.(authModel)
	require.True(t, m.quitting, "a successful paste finishes the flow")
	require.Equal(t, "at-pasted", m.token.AccessToken)
}

func TestAuthTUISubmitCodeFailureKeepsWaiting(t *testing.T) {
	m := waitingModel(t)

	for _, r := range "bad-code" {
		m = step(t, m, tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)

	msg := cmd()
	result, ok := msg.(authCodeResultMsg)
	require.True(t, ok)
	require.Error(t, result.err)

	model, _ := next.(authModel).Update(result)
	m = model.(authModel)
	require.False(t, m.quitting, "a failed paste keeps waiting for the callback")
	require.Contains(t, m.content(), "invalid code")
	require.Empty(t, m.codeInput.Value(), "the field clears so the paste can be retried")
}

func TestAuthTUIEmptySubmitDoesNothing(t *testing.T) {
	m := waitingModel(t)

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Nil(t, cmd, "enter with an empty field must not submit")
}

func TestAuthTUICancelStillQuits(t *testing.T) {
	m := waitingModel(t)

	m = step(t, m, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	require.True(t, m.quitting)
	require.True(t, m.canceled)
}

func TestGrokFlowCompleteWithCodeWithoutBrowserFlow(t *testing.T) {
	f := &grokFlow{deviceCode: "dc", expiresIn: 60}
	_, err := f.CompleteWithCode(context.Background(), "some-code")
	require.ErrorContains(t, err, "no browser flow")
}
