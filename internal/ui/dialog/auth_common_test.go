package dialog

import (
	"errors"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// TestWrappedLinkBreaksAtFixedWidths verifies a URL is broken at fixed
// widths rather than at its hyphens, so every line but the last fills
// the width, the lines rejoin into the exact URL, and each line links to
// the whole URL.
func TestWrappedLinkBreaksAtFixedWidths(t *testing.T) {
	t.Parallel()

	const url = "https://accounts.x.ai/oauth2/authorize?client-id=abc-def-ghi&redirect-uri=http%3A%2F%2F127.0.0.1&state=xyz-123"
	for _, w := range []int{56, 34, 7} {
		out := wrappedLink(lipgloss.NewStyle(), url, "id=test", w)
		lines := strings.Split(out, "\n")
		require.Greater(t, len(lines), 1, "width %d", w)
		var plain []string
		for i, l := range lines {
			require.Contains(t, l, "id=test;"+url, "line %d links the whole URL at width %d", i, w)
			text := ansi.Strip(l)
			if i < len(lines)-1 {
				require.Len(t, text, w, "line %d broke early at width %d", i, w)
			}
			plain = append(plain, text)
		}
		require.Equal(t, url, strings.Join(plain, ""), "width %d", w)
	}
}

// TestAuthEscKey verifies esc cancels while a sign-in runs and exits once
// it is done, and that finishing takes enter and both esc keys.
func TestAuthEscKey(t *testing.T) {
	t.Parallel()

	require.Equal(t, cancelKey.Help(), authEscKey(false).Help())
	require.Equal(t, CloseKey.Help(), authEscKey(true).Help())
	finish := finishKey("finish")
	require.Equal(t, "finish", finish.Help().Desc)
	require.ElementsMatch(t, append([]string{"enter", "ctrl+y"}, CloseKey.Keys()...), finish.Keys())
}

// TestAuthErrorBlockFlattensReason verifies a multi-line reason is shown
// on one wrapped line under the header, and that a missing error shows
// the header alone.
func TestAuthErrorBlockFlattensReason(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	out := ansi.Strip(authErrorBlock(&s, 60, errText(errors.New("listen tcp:\n  bind:   address in use"))))
	require.Contains(t, out, "Authentication failed.")
	require.Contains(t, out, "listen tcp: bind: address in use")

	require.Empty(t, errText(nil))
	out = ansi.Strip(authErrorBlock(&s, 60, errText(nil)))
	require.Equal(t, 3, lipgloss.Height(out), "blank, header, blank")
}

// requireLinkDrawn asserts every line of url, broken as the sign-in
// dialogs break it inside a screen w cells wide, was drawn.
func requireLinkDrawn(t *testing.T, lines []string, url string, w int) {
	t.Helper()
	s := styles.CharmtonePantera()
	_, inner := authWidths(&s, uvRect(w, 1), authDialogMaxWidth)
	screen := strings.Join(lines, "\n")
	for _, l := range strings.Split(ansi.Strip(authLinkBlock(&s, inner, url, "test", "")), "\n") {
		require.Contains(t, screen, l, "width %d", w)
	}
}
