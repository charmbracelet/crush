package dialog

import (
	"strings"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"
)

// TestAuthMethodCardsCenteredWithMargin verifies the cards keep the same
// margin from the dialog frame on both sides, side by side and stacked,
// and that the mouse hit areas still match the drawn cards.
func TestAuthMethodCardsCenteredWithMargin(t *testing.T) {
	t.Parallel()

	for _, w := range []int{100, 99, 40} {
		m := newTestAuthMethod()
		scr := uv.NewScreenBuffer(w, 30)
		m.Draw(scr, scr.Bounds())
		lines := strings.Split(scr.String(), "\n")
		r := m.oauthCardArea
		line := lines[r.Min.Y]
		left := strings.Index(line, "│")
		right := strings.LastIndex(line, "│")
		require.NotEqual(t, -1, left)
		inner := line[left+len("│") : right]
		leftMargin := len(inner) - len(strings.TrimLeft(inner, " "))
		rightMargin := len(inner) - len(strings.TrimRight(inner, " "))
		require.Equal(t, authMethodCardMargin, leftMargin, "left margin at width %d: %q", w, line)
		require.Equal(t, leftMargin, rightMargin, "cards off-center at width %d: %q", w, line)
		requireAuthCardAreasMatch(t, false, w, 30)
	}
}
