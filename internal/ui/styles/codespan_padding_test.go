package styles

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/rivo/uniseg"
	"github.com/stretchr/testify/require"
)

// TestCodespanPaddingIsOneCellWide guards the flicker regression from the
// emoji-presentation variation selector: when the sentinel ended in U+FE0F,
// ansi.StringWidth measured it as two cells wide while terminals rendered it
// as one, and that mismatch made the frame differ repaint the line on every
// frame. The sentinel must measure exactly one cell under every width
// authority in play (glamour wraps with x/ansi, cells are stored with
// uniseg).
func TestCodespanPaddingIsOneCellWide(t *testing.T) {
	t.Parallel()

	require.Equal(t, 1, ansi.StringWidth(CodespanPadding),
		"codespan padding must budget one cell, or word wrap disagrees with the terminal")
	require.Equal(t, 1, uniseg.StringWidth(CodespanPadding),
		"codespan padding must occupy one screen cell")

	state := -1
	cluster, _, w, _ := uniseg.FirstGraphemeClusterInString(CodespanPadding, state)
	require.Equal(t, CodespanPadding, cluster, "padding must be a single grapheme cluster")
	require.Equal(t, 1, w)

	require.NotEqual(t, "\u00a0", CodespanPadding,
		"sentinel must stay distinguishable from a real no-break space")
}
