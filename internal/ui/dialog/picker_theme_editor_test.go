package dialog

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
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
