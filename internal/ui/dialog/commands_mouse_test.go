package dialog

import (
	"fmt"
	"image"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/commands"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"
)

// commandsMouseWorkspace supplies a config so the commands dialog can build
// its default groups without a real workspace.
type commandsMouseWorkspace struct {
	workspace.Workspace
	cfg *config.Config
}

func (w *commandsMouseWorkspace) Config() *config.Config { return w.cfg }

func newCommandsMouseDialog(t *testing.T, custom []commands.CustomCommand) *Commands {
	t.Helper()
	sty := styles.CharmtonePantera()
	com := &common.Common{
		Workspace: &commandsMouseWorkspace{cfg: &config.Config{}},
		Styles:    &sty,
	}
	d, err := NewCommands(com, "", false, false, false, custom, nil)
	require.NoError(t, err)
	return d
}

// newOverflowingCommandsDialog returns a commands dialog whose user tab has
// more commands than the palette can show, so wheel scrolling has room to
// move. The flat user tab maps each visible row to one command.
func newOverflowingCommandsDialog(t *testing.T) *Commands {
	t.Helper()
	custom := make([]commands.CustomCommand, 30)
	for i := range custom {
		custom[i] = commands.CustomCommand{
			ID:   fmt.Sprintf("c%02d", i),
			Name: fmt.Sprintf("Command %02d", i),
		}
	}
	d := newCommandsMouseDialog(t, custom)
	d.setCommandItems(UserCommands)
	return d
}

func drawCommands(t *testing.T, d *Commands) image.Rectangle {
	t.Helper()
	scr := uv.NewScreenBuffer(80, 30)
	d.Draw(scr, image.Rect(0, 0, 80, 30))
	require.False(t, d.bodyArea.Empty(), "expected the command list to have a drawable area")
	return d.bodyArea
}

func TestCommandsMouseWheelScrollsListUnderPointer(t *testing.T) {
	t.Parallel()

	d := newOverflowingCommandsDialog(t)
	area := drawCommands(t, d)
	require.True(t, d.list.Overflows(d.list.Height()), "list must overflow for wheel scrolling")
	require.Zero(t, d.list.Offset())

	d.HandleMsg(common.CoalescedWheelMsg{
		Mouse:  tea.Mouse{X: area.Min.X, Y: area.Min.Y},
		DeltaY: 3,
	})
	require.Equal(t, 3, d.list.Offset())

	// Rendering must preserve the offset chosen by the wheel event.
	drawCommands(t, d)
	require.Equal(t, 3, d.list.Offset())

	// Wheel movement outside the rendered list has no effect.
	d.HandleMsg(common.CoalescedWheelMsg{
		Mouse:  tea.Mouse{X: 0, Y: 0},
		DeltaY: 3,
	})
	require.Equal(t, 3, d.list.Offset())
}

func TestCommandsMouseClickHighlightsCommand(t *testing.T) {
	t.Parallel()

	d := newOverflowingCommandsDialog(t)
	area := drawCommands(t, d)
	require.Zero(t, d.list.Selected())

	action := d.HandleMsg(tea.MouseClickMsg(tea.Mouse{
		X:      area.Min.X,
		Y:      area.Min.Y + 1,
		Button: tea.MouseLeft,
	}))

	require.Nil(t, action)
	require.Equal(t, 1, d.list.Selected())
}

func TestCommandsMouseDoubleClickRunsCommandLikeEnter(t *testing.T) {
	t.Parallel()

	d := newOverflowingCommandsDialog(t)
	area := drawCommands(t, d)
	click := tea.MouseClickMsg(tea.Mouse{
		X:      area.Min.X,
		Y:      area.Min.Y + 1,
		Button: tea.MouseLeft,
	})

	require.Nil(t, d.HandleMsg(click))
	enterAction := d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	doubleClickAction := d.HandleMsg(click)

	require.NotNil(t, enterAction)
	require.Equal(t, enterAction, doubleClickAction)
}

func TestCommandsMouseDoubleClickExpires(t *testing.T) {
	t.Parallel()

	d := newOverflowingCommandsDialog(t)
	area := drawCommands(t, d)
	click := tea.MouseClickMsg(tea.Mouse{
		X:      area.Min.X,
		Y:      area.Min.Y + 1,
		Button: tea.MouseLeft,
	})

	require.Nil(t, d.HandleMsg(click))
	d.lastClickTime = time.Now().Add(-dialogDoubleClickThreshold - time.Millisecond)
	require.Nil(t, d.HandleMsg(click))
	require.Equal(t, 1, d.list.Selected())
}

func TestCommandsMouseClickSelectsFirstVisibleCommandAfterScroll(t *testing.T) {
	t.Parallel()

	d := newOverflowingCommandsDialog(t)
	area := drawCommands(t, d)
	d.HandleMsg(common.CoalescedWheelMsg{
		Mouse:  tea.Mouse{X: area.Min.X, Y: area.Min.Y},
		DeltaY: 3,
	})
	require.Equal(t, 3, d.list.Offset())

	action := d.HandleMsg(tea.MouseClickMsg(tea.Mouse{
		X:      area.Min.X,
		Y:      area.Min.Y,
		Button: tea.MouseLeft,
	}))

	require.Nil(t, action)
	require.Equal(t, 3, d.list.Selected())
}

func TestCommandsMouseClickSkipsSectionHeaders(t *testing.T) {
	t.Parallel()

	// The system tab interleaves section headers with its commands; the
	// first visible row is a header, so clicking it must not change the
	// selection.
	d := newCommandsMouseDialog(t, nil)
	area := drawCommands(t, d)
	require.NotZero(t, d.list.Selected(), "first command item must be selected initially")

	action := d.HandleMsg(tea.MouseClickMsg(tea.Mouse{
		X:      area.Min.X,
		Y:      area.Min.Y,
		Button: tea.MouseLeft,
	}))

	require.Nil(t, action)
	require.Equal(t, 1, d.list.Selected())
}

func TestCommandsMouseClickIgnoresUnsupportedClicks(t *testing.T) {
	t.Parallel()

	d := newOverflowingCommandsDialog(t)
	area := drawCommands(t, d)
	inList := image.Pt(area.Min.X, area.Min.Y+1)

	tests := []struct {
		name  string
		click tea.MouseClickMsg
	}{
		{
			name:  "outside list",
			click: tea.MouseClickMsg(tea.Mouse{X: 0, Y: 0, Button: tea.MouseLeft}),
		},
		{
			name:  "right button",
			click: tea.MouseClickMsg(tea.Mouse{X: inList.X, Y: inList.Y, Button: tea.MouseRight}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action := d.HandleMsg(tt.click)

			require.Nil(t, action)
			require.Zero(t, d.list.Selected())
		})
	}
}
