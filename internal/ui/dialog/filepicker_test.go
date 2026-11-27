package dialog

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestFilePickerFitsArea verifies the file picker keeps its file list, help
// and border on screen at short sizes instead of being filled by the image
// preview, and never draws a solid placeholder block.
func TestFilePickerFitsArea(t *testing.T) {
	t.Parallel()
	for _, sz := range [][2]int{{100, 30}, {40, 30}, {100, 12}} {
		fp, _ := NewFilePicker(newPickerTestCommon())
		lines := drawFits(t, fp, sz[0], sz[1])
		require.NotContains(t, strings.Join(lines, "\n"), "█")
	}
}

// TestFilePickerEmptyDirectoryPadded verifies the empty directory notice is
// indented from the border instead of touching it.
func TestFilePickerEmptyDirectoryPadded(t *testing.T) {
	t.Parallel()
	fp, _ := NewFilePicker(newPickerTestCommon())
	lines := drawFits(t, fp, 100, 30)
	require.Contains(t, strings.Join(lines, "\n"), "│  Empty directory")
}
