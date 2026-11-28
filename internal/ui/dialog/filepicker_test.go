package dialog

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
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

	dir, path := writeTestPNG(t)
	fp, initCmd := newPreviewTestPicker(dir)
	fp.HandleMsg(initCmd())
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

// writeTestPNG writes a small PNG into a new temporary directory and
// returns the directory and the image path.
func writeTestPNG(t *testing.T) (dir, path string) {
	t.Helper()
	dir = t.TempDir()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	img.Set(0, 0, color.RGBA{R: 0xff, A: 0xff})
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	path = filepath.Join(dir, "a.png")
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0o644))
	return dir, path
}

// newPreviewTestPicker returns a picker listing dir that previews with the
// Kitty encoding; with no cell size that skips the block rasterizer,
// keeping the tests cheap.
func newPreviewTestPicker(dir string) (*FilePicker, tea.Cmd) {
	// The constructor's command reads the original directory; return one
	// reading the test directory instead.
	fp, _ := NewFilePicker(newPickerTestCommon())
	fp.imgEnc = fimage.EncodingKitty
	fp.fp.CurrentDirectory = dir
	return fp, fp.fp.Init()
}

// runCmds runs cmd the way Bubble Tea would, expanding batches and
// sequences, and returns every message it produced in order.
func runCmds(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if msg == nil {
		return nil
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		var msgs []tea.Msg
		for _, c := range batch {
			msgs = append(msgs, runCmds(c)...)
		}
		return msgs
	}
	// tea.Sequence produces an unexported slice of commands.
	if v := reflect.ValueOf(msg); v.Kind() == reflect.Slice &&
		v.Type().Elem() == reflect.TypeFor[tea.Cmd]() {
		var msgs []tea.Msg
		for i := range v.Len() {
			c, _ := v.Index(i).Interface().(tea.Cmd)
			msgs = append(msgs, runCmds(c)...)
		}
		return msgs
	}
	return []tea.Msg{msg}
}

// deliver hands msg to the picker and runs the command it returns, feeding
// back any preview-loaded message. It reports whether a preview load
// finished.
func deliver(t *testing.T, fp *FilePicker, msg tea.Msg) bool {
	t.Helper()
	action := fp.HandleMsg(msg)
	ac, ok := action.(ActionCmd)
	if !ok {
		return false
	}
	loaded := false
	for _, m := range runCmds(ac.Cmd) {
		if _, ok := m.(filePickerPreviewLoadedMsg); ok {
			loaded = true
			fp.HandleMsg(m)
		}
	}
	return loaded
}

// TestFilePickerPreviewLoadsOnFirstOpen drives the real open sequence (the
// first draw before the directory has loaded, then the directory listing)
// and verifies the highlighted image loads without any further message.
func TestFilePickerPreviewLoadsOnFirstOpen(t *testing.T) {
	t.Parallel()

	dir, path := writeTestPNG(t)
	fp, initCmd := newPreviewTestPicker(dir)

	// Bubble Tea draws right after the dialog opens, before the
	// directory listing arrives.
	drawFits(t, fp, 100, 40)
	require.Empty(t, fp.fp.HighlightedPath())

	require.True(t, deliver(t, fp, initCmd()))
	require.Equal(t, path, fp.fp.HighlightedPath())
	drawFits(t, fp, 100, 40)
	require.True(t, fp.previewingImage)
}

// TestFilePickerPreviewSurvivesWheel verifies a wheel message, whose action
// the UI discards, does not swallow the preview load: a later message
// still schedules it.
func TestFilePickerPreviewSurvivesWheel(t *testing.T) {
	t.Parallel()

	dir, path := writeTestPNG(t)
	fp, initCmd := newPreviewTestPicker(dir)

	// The listing arrives before any draw has sized the preview.
	require.False(t, deliver(t, fp, initCmd()))
	require.Equal(t, path, fp.fp.HighlightedPath())
	drawFits(t, fp, 100, 40)

	fp.HandleMsg(common.CoalescedWheelMsg{DeltaY: 1})
	fp.HandleMsg(tea.MouseMotionMsg{})
	fp.HandleMsg(tea.MouseReleaseMsg{})
	require.False(t, fp.previewingImage)

	require.True(t, deliver(t, fp, struct{}{}))
	drawFits(t, fp, 100, 40)
	require.True(t, fp.previewingImage)
}

// TestFilePickerEmptyDirectoryPadded verifies the empty directory notice is
// indented from the border instead of touching it.
func TestFilePickerEmptyDirectoryPadded(t *testing.T) {
	t.Parallel()
	fp, _ := NewFilePicker(newPickerTestCommon())
	lines := drawFits(t, fp, 100, 30)
	require.Contains(t, strings.Join(lines, "\n"), "│  Empty directory")
}
