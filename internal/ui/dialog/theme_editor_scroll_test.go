package dialog

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
