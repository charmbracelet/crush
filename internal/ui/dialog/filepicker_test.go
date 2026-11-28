package dialog

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	fimage "github.com/charmbracelet/crush/internal/ui/image"
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

// TestFilePickerPreviewLoadsOnce verifies the highlighted image is loaded by
// a single command rather than on every message, that loads the user moved
// past are dropped, and that the preview shows once the load reports back.
func TestFilePickerPreviewLoadsOnce(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	img.Set(0, 0, color.RGBA{R: 0xff, A: 0xff})
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	path := filepath.Join(dir, "a.png")
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0o644))

	fp, _ := NewFilePicker(newPickerTestCommon())
	// Kitty with no cell size skips the block rasterizer, keeping the
	// test cheap.
	fp.imgEnc = fimage.EncodingKitty
	fp.fp.CurrentDirectory = dir
	fp.HandleMsg(fp.fp.Init()())
	require.Equal(t, path, fp.fp.HighlightedPath())

	// Nothing is loaded before the first draw sizes the preview.
	require.Nil(t, fp.updatePreview())
	drawFits(t, fp, 100, 40)
	require.Positive(t, fp.imgPrevHeight)

	require.NotNil(t, fp.updatePreview())
	require.False(t, fp.previewingImage)
	// Further messages while the load is in flight schedule nothing more.
	require.Nil(t, fp.updatePreview())
	require.Nil(t, fp.updatePreview())

	// A load for a preview the user has moved past does nothing.
	stale := filePickerPreviewKey{path: path, width: 3, height: 3}
	require.Nil(t, fp.previewLoader(stale, fp.previewGen.Load()-1)())
	require.False(t, fimage.HasTransmitted(path, 3, 3))

	msg := fp.previewLoader(fp.previewKey, fp.previewGen.Load())()
	require.IsType(t, tea.RawMsg{}, msg)
	fp.HandleMsg(filePickerPreviewLoadedMsg{})
	require.True(t, fp.previewingImage)
	require.Nil(t, fp.updatePreview())
}

// TestFilePickerEmptyDirectoryPadded verifies the empty directory notice is
// indented from the border instead of touching it.
func TestFilePickerEmptyDirectoryPadded(t *testing.T) {
	t.Parallel()
	fp, _ := NewFilePicker(newPickerTestCommon())
	lines := drawFits(t, fp, 100, 30)
	require.Contains(t, strings.Join(lines, "\n"), "│  Empty directory")
}
