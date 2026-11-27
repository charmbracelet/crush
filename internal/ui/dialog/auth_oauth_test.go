package dialog

import (
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

func (p testOAuthProvider) name() string                     { return "Grok" }
func (p testOAuthProvider) initiateAuth() tea.Msg            { return nil }
func (p testOAuthProvider) startPolling(string, int) tea.Cmd { return nil }
func (p testOAuthProvider) stopPolling() tea.Msg             { return nil }
func (p testOAuthProvider) supportsCodeEntry() bool          { return p.codeEntry }
func (p testOAuthProvider) submitCode(string) tea.Cmd        { return nil }

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
