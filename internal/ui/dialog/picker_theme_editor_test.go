package dialog

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

// pickerTestWorkspace is a minimal workspace for rendering picker dialogs.
type pickerTestWorkspace struct{ workspace.Workspace }

func (pickerTestWorkspace) Config() *config.Config {
	return &config.Config{Options: &config.Options{TUI: &config.TUIOptions{}}}
}

func (pickerTestWorkspace) WorkingDir() string { return "/nonexistent-crush-test-dir" }

func newPickerTestCommon() *common.Common {
	s := styles.CharmtonePantera()
	return &common.Common{Styles: &s, Workspace: pickerTestWorkspace{}}
}

// TestThemeEditorFitsArea verifies the theme editor, help line and border
// included, fits within the screen at common and short sizes.
func TestThemeEditorFitsArea(t *testing.T) {
	t.Parallel()
	for _, sz := range [][2]int{{100, 30}, {40, 30}, {100, 12}} {
		drawFits(t, NewThemeEditor(newPickerTestCommon(), "charmtone-panther"), sz[0], sz[1])
	}
}

// TestThemeNewFitsArea verifies the new theme prompt keeps its whole border
// on narrow screens.
func TestThemeNewFitsArea(t *testing.T) {
	t.Parallel()
	for _, sz := range [][2]int{{100, 30}, {40, 30}, {100, 12}} {
		drawFits(t, NewThemeNew(newPickerTestCommon(), ""), sz[0], sz[1])
	}
}

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

// TestThemeNewNoReservedErrorSpace verifies the new theme prompt leaves a
// single blank line between the input and help, and fits an error when
// one appears.
func TestThemeNewNoReservedErrorSpace(t *testing.T) {
	t.Parallel()
	d := NewThemeNew(newPickerTestCommon(), "")
	rows := func() []string {
		var out []string
		for _, l := range drawFits(t, d, 100, 30) {
			if l = strings.TrimSpace(l); l != "" {
				out = append(out, strings.Trim(l, "│ "))
			}
		}
		return out
	}
	// Border, title, blank, input, blank, help, border.
	got := rows()
	require.Len(t, got, 7, strings.Join(got, "\n"))
	require.Contains(t, got[5], "enter create")

	d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotEmpty(t, d.err)
	got = rows()
	require.Greater(t, len(got), 7)
	require.Contains(t, got[len(got)-2], "enter create")
	require.Empty(t, got[len(got)-3])
}
