package dialog

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"
)

// testOAuthProvider is a no-op OAuth provider for rendering tests.
type testOAuthProvider struct{ codeEntry bool }

func (p testOAuthProvider) name() string { return "Grok" }

func (p testOAuthProvider) initiateAuth() tea.Msg { return nil }

func (p testOAuthProvider) startPolling(string, int) tea.Cmd { return nil }

func (p testOAuthProvider) stopPolling() tea.Msg { return nil }

func (p testOAuthProvider) supportsCodeEntry() bool { return p.codeEntry }

func (p testOAuthProvider) submitCode(string) tea.Cmd { return nil }

const testOAuthURL = "https://accounts.x.ai/oauth2/authorize?client-id=abc-def-ghi&redirect-uri=http%3A%2F%2F127.0.0.1%3A5678&state=xyz-123"

func newTestOAuth(codeEntry bool) *OAuth {
	s := styles.CharmtonePantera()
	m, _ := newOAuth(&common.Common{Styles: &s}, false,
		catwalk.Provider{ID: catwalk.InferenceProviderXAI, Name: "xAI"},
		config.SelectedModel{}, config.SelectedModelTypeLarge,
		testOAuthProvider{codeEntry: codeEntry})
	return m
}

// TestOAuthPasteFieldCursor verifies the cursor sits on the paste field
// row when it is focused, below the instructions and URL above it.
func TestOAuthPasteFieldCursor(t *testing.T) {
	t.Parallel()

	for _, w := range []int{100, 40} {
		m := newTestOAuth(true)
		m.HandleMsg(ActionInitiateOAuth{VerificationURL: testOAuthURL})
		m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyTab})
		lines := drawFits(t, m, w, 30)
		scr := uv.NewScreenBuffer(w, 30)
		cur := m.Draw(scr, scr.Bounds())
		require.NotNil(t, cur, "width %d", w)
		require.Contains(t, lines[cur.Y], "> Paste", "width %d", w)
		require.Equal(t, "P", scr.CellAt(cur.X, cur.Y).Content, "width %d", w)
		require.True(t, strings.HasPrefix(strings.TrimSpace(lines[cur.Y]), "│"))
	}
}

// TestOAuthPasteFieldFitsOneLine verifies the paste field's placeholder
// stays on one line instead of wrapping inside the dialog.
func TestOAuthPasteFieldFitsOneLine(t *testing.T) {
	t.Parallel()

	m := newTestOAuth(true)
	m.HandleMsg(ActionInitiateOAuth{VerificationURL: testOAuthURL})
	lines := drawFits(t, m, 40, 30)
	out := strings.Join(lines, "\n")
	require.Contains(t, out, "> Paste the code from the browser")
	require.NotContains(t, out, "browser..  ")
}

// TestOAuthBusyStatesShowTitleAndHelp verifies the initializing and
// saving screens keep the dialog title, and that initializing tells the
// user esc cancels.
func TestOAuthBusyStatesShowTitleAndHelp(t *testing.T) {
	t.Parallel()

	for _, w := range []int{100, 40} {
		m := newTestOAuth(false)
		out := strings.Join(drawFits(t, m, w, 30), "\n")
		require.Contains(t, out, "Grok")
		require.Contains(t, out, "Initializing...")
		require.Contains(t, out, "esc "+escLabel(m.ShortHelp()))
		require.Equal(t, cancelKey.Help().Desc, escLabel(m.ShortHelp()))

		m.State = OAuthStateSaving
		out = strings.Join(drawFits(t, m, w, 30), "\n")
		require.Contains(t, out, "Grok")
		require.Contains(t, out, "Fetching models...")
	}
}

// TestOAuthHelpKeepsEscWhenNarrow verifies esc stays in the help bar
// when the optional hints are truncated.
func TestOAuthHelpKeepsEscWhenNarrow(t *testing.T) {
	t.Parallel()

	for _, codeEntry := range []bool{false, true} {
		for _, userCode := range []string{"", "ABCD-EFGH"} {
			m := newTestOAuth(codeEntry)
			m.HandleMsg(ActionInitiateOAuth{UserCode: userCode, VerificationURL: "https://example.com/device"})
			require.Equal(t, cancelKey.Help().Desc, escLabel(m.ShortHelp()), "codeEntry=%v userCode=%q", codeEntry, userCode)
			out := strings.Join(drawFits(t, m, 40, 30), "\n")
			require.Contains(t, out, "esc "+cancelKey.Help().Desc, "codeEntry=%v userCode=%q", codeEntry, userCode)
		}
	}
}

// TestOAuthErrorShowsReason verifies the failure reason is shown in the
// dialog, wrapped inside it, rather than only in the status bar.
func TestOAuthErrorShowsReason(t *testing.T) {
	t.Parallel()

	for _, w := range []int{100, 40} {
		m := newTestOAuth(false)
		m.HandleMsg(ActionOAuthErrored{Error: errors.New("failed to start browser auth: listen tcp 127.0.0.1:56121: bind: address already in use")})
		out := strings.Join(drawFits(t, m, w, 30), "\n")
		require.Contains(t, out, "Authentication failed.")
		require.Contains(t, out, "failed to start browser")
		require.Contains(t, out, "already in use")
	}
}

// TestOAuthDrawsWholeURL verifies the verification URL is drawn whole,
// broken to the content width rather than truncated.
func TestOAuthDrawsWholeURL(t *testing.T) {
	t.Parallel()

	for _, w := range []int{100, 40} {
		m := newTestOAuth(false)
		m.HandleMsg(ActionInitiateOAuth{VerificationURL: testOAuthURL})
		requireLinkDrawn(t, drawFits(t, m, w, 30), testOAuthURL, w)
	}
}

// TestOAuthInstructionsMatchFlow verifies each flow's "Press enter"
// instruction says what enter does, and that the help bar agrees.
func TestOAuthInstructionsMatchFlow(t *testing.T) {
	t.Parallel()

	browser := newTestOAuth(false)
	browser.HandleMsg(ActionInitiateOAuth{VerificationURL: testOAuthURL})
	device := newTestOAuth(false)
	device.HandleMsg(ActionInitiateOAuth{UserCode: "ABCD-EFGH", VerificationURL: "https://example.com/device"})

	for name, c := range map[string]struct {
		m    *OAuth
		copy oauthFlowCopy
	}{"browser": {browser, browserFlowCopy}, "device": {device, deviceFlowCopy}} {
		out := strings.Join(drawFits(t, c.m, 100, 30), "\n")
		require.Contains(t, out, "Press enter to "+c.copy.enterHint, name)
		require.Contains(t, out, c.copy.waiting, name)
		require.Equal(t, c.copy.enterHelp, c.m.ShortHelp()[0].Help().Desc, name)
	}
	require.NotEqual(t, browserFlowCopy, deviceFlowCopy)
}

// TestOAuthEscLabels verifies esc cancels while signing in and exits once
// the sign-in has failed, and that a finished sign-in is acknowledged
// with the shared finish keys.
func TestOAuthEscLabels(t *testing.T) {
	t.Parallel()

	m := newTestOAuth(false)
	require.Equal(t, cancelKey.Help().Desc, escLabel(m.ShortHelp()), "starting")
	m.HandleMsg(ActionInitiateOAuth{VerificationURL: "https://example.com/device"})
	require.Equal(t, cancelKey.Help().Desc, escLabel(m.ShortHelp()), "waiting")
	m.State = OAuthStateError
	require.Equal(t, CloseKey.Help().Desc, escLabel(m.ShortHelp()), "failed")
	m.State = OAuthStateSuccess
	require.Equal(t, finishKey("finish").Keys(), m.ShortHelp()[0].Keys(), "finished")
}

// TestOAuthTitle verifies the dialog names the provider in the shared
// sign-in title phrasing.
func TestOAuthTitle(t *testing.T) {
	t.Parallel()

	out := strings.Join(drawFits(t, newTestOAuth(false), 100, 30), "\n")
	require.Contains(t, out, "Authenticate with Grok")
}

// TestOAuthSpinnerSingleSpace verifies the saving status puts exactly one
// space between the spinner and its text.
func TestOAuthSpinnerSingleSpace(t *testing.T) {
	t.Parallel()

	m := newTestOAuth(false)
	m.State = OAuthStateSaving
	out := strings.Join(drawFits(t, m, 100, 30), "\n")
	require.Contains(t, out, "Fetching models...")
	require.NotContains(t, out, "  Fetching models...")
}
