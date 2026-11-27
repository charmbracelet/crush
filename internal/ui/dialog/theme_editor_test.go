package dialog

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestThemeEditorFitsArea verifies the theme editor, help line and border
// included, fits within the screen at common and short sizes.
func TestThemeEditorFitsArea(t *testing.T) {
	t.Parallel()
	for _, sz := range [][2]int{{100, 30}, {40, 30}, {100, 12}} {
		drawFits(t, NewThemeEditor(newPickerTestCommon(), "charmtone-panther"), sz[0], sz[1])
	}
}

// TestThemeEditorScrollbarAgainstBorder verifies the theme editor's
// scrollbar sits against the right border like other pickers.
func TestThemeEditorScrollbarAgainstBorder(t *testing.T) {
	t.Parallel()
	lines := drawFits(t, NewThemeEditor(newPickerTestCommon(), "charmtone-panther"), 100, 30)
	require.Contains(t, strings.Join(lines, "\n"), "┃│")
}

// TestThemeEditorTallerListClampsScroll verifies that growing the slot list
// while the last slot is selected does not leave it scrolled past the end.
func TestThemeEditorTallerListClampsScroll(t *testing.T) {
	t.Parallel()
	ed := &ThemeEditor{slots: newPaletteSlots()}
	ed.index = len(ed.slots) - 1

	ed.keepSelectedVisible(5)
	ed.keepSelectedVisible(18)

	require.Equal(t, len(ed.slots)-18, ed.scroll)
}
