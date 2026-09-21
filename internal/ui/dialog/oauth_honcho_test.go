package dialog

import (
	"fmt"
	"image"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// honchoTestDialog builds the memory sign-in dialog without running
// the command it returns, so no network call is made.
func honchoTestDialog(t *testing.T) *OAuth {
	t.Helper()
	sty := styles.CharmtonePantera()
	com := &common.Common{Styles: &sty}

	dlg, cmd := NewOAuthHoncho(com)
	require.NotNil(t, dlg)
	// The command starts browser authorization. Returning it without
	// running it is the point: a test must never reach the network.
	require.NotNil(t, cmd)
	return dlg
}

// honchoRender draws the dialog and returns its lines, stripped of
// ANSI so widths can be measured.
func honchoRender(t *testing.T, dlg *OAuth, width, height int) []string {
	t.Helper()
	scr := uv.NewScreenBuffer(width, height)
	dlg.Draw(scr, image.Rect(0, 0, width, height))
	return strings.Split(lipgloss.NewStyle().Render(scr.Render()), "\n")
}

func TestNewOAuthHonchoDoesNotPanic(t *testing.T) {
	t.Parallel()

	require.NotPanics(t, func() { honchoTestDialog(t) })
}

// TestOAuthHonchoRendersEveryState draws each state the dialog can
// reach. Dialogs are routinely copy-pasted, and a state nobody
// rendered in a test is where a panic or an overflow hides.
func TestOAuthHonchoRendersEveryState(t *testing.T) {
	t.Parallel()

	states := map[string]OAuthState{
		"initializing": OAuthStateInitializing,
		"display":      OAuthStateDisplay,
		"saving":       OAuthStateSaving,
		"success":      OAuthStateSuccess,
		"error":        OAuthStateError,
	}

	for name, state := range states {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dlg := honchoTestDialog(t)
			dlg.State = state
			dlg.verificationURL = "https://app.honcho.dev/authorize?client_id=abc&state=xyz"

			require.NotPanics(t, func() { honchoRender(t, dlg, 80, 24) })
		})
	}
}

// TestOAuthHonchoStaysInsideItsWidth is the guard against the classic
// dialog bug: content sized to the outer width instead of the content
// area, so the frame re-wraps the last few characters.
func TestOAuthHonchoStaysInsideItsWidth(t *testing.T) {
	t.Parallel()

	// A long URL is the realistic overflow case, since an authorize
	// URL carries a client id, scopes, state, and a PKCE challenge.
	longURL := "https://app.honcho.dev/authorize?response_type=code&client_id=5ooboechdgdplam7vxtvyn44trap8c4q&redirect_uri=http%3A%2F%2F127.0.0.1%3A54321%2Fcallback&scope=read+write&state=Zm9vYmFyYmF6&code_challenge=E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM&code_challenge_method=S256"

	for _, width := range []int{40, 60, 80, 120} {
		t.Run(fmt.Sprintf("width %d", width), func(t *testing.T) {
			t.Parallel()

			dlg := honchoTestDialog(t)
			dlg.State = OAuthStateDisplay
			dlg.verificationURL = longURL

			for _, line := range honchoRender(t, dlg, width, 24) {
				require.LessOrEqual(t, ansi.StringWidth(line), width,
					"a rendered line overflowed the terminal width")
			}
		})
	}
}

// TestOAuthHonchoFitsSmallTerminals checks the dialog clamps to the
// drawable area rather than assuming it has room.
func TestOAuthHonchoFitsSmallTerminals(t *testing.T) {
	t.Parallel()

	dlg := honchoTestDialog(t)
	dlg.State = OAuthStateDisplay
	dlg.verificationURL = "https://app.honcho.dev/authorize"

	require.NotPanics(t, func() { honchoRender(t, dlg, 24, 8) })
}

// TestOAuthHonchoPersistWithoutTokenErrors checks the save path
// refuses to report success when authorization never produced a
// credential, rather than writing nothing and claiming it worked.
func TestOAuthHonchoPersistWithoutTokenErrors(t *testing.T) {
	t.Parallel()

	provider := &OAuthHoncho{}
	msg := provider.persist(nil)()
	require.IsType(t, oauthSaveErrMsg{}, msg)
}

// TestOAuthHonchoStopPollingIsSafeWithoutFlow checks that dismissing
// the dialog before authorization started does not panic.
func TestOAuthHonchoStopPollingIsSafeWithoutFlow(t *testing.T) {
	t.Parallel()

	provider := &OAuthHoncho{}
	require.NotPanics(t, func() { provider.stopPolling() })
	require.Nil(t, provider.startPolling("", 0)())
}

// TestOAuthHonchoNameAndMessages checks the strings the shared dialog
// shows, since the default ones describe fetching LLM models and
// would be wrong here.
func TestOAuthHonchoNameAndMessages(t *testing.T) {
	t.Parallel()

	provider := &OAuthHoncho{}
	require.Equal(t, "Honcho", provider.name())
	require.NotContains(t, provider.savingMessage(), "models",
		"memory sign-in must not claim to be fetching models")
}
