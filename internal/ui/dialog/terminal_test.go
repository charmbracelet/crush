package dialog

import (
	"image"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/shell"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"
)

func uvScreenBuffer(width, height int) uv.ScreenBuffer {
	return uv.NewScreenBuffer(width, height)
}

func newTerminalDialogForTest(t *testing.T, command string) (*TerminalDialog, *shell.InteractiveSession) {
	t.Helper()

	session, err := shell.NewInteractiveSession(shell.InteractiveSessionOptions{
		Command:    command,
		WorkingDir: t.TempDir(),
		Cols:       40,
		Rows:       10,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = session.Kill()
		_ = session.Close()
	})

	s := styles.CharmtonePantera()
	dialog := NewTerminalDialog(&common.Common{Styles: &s}, session, command)
	return dialog, session
}

func TestTerminalDialogIDAndSession(t *testing.T) {
	t.Parallel()

	dialog, session := newTerminalDialogForTest(t, "printf hi")
	require.Equal(t, TerminalID, dialog.ID())
	require.Same(t, session, dialog.Session())
	require.Equal(t, "printf hi", dialog.Command())
}

func TestTerminalDialogForwardsKeysAsInput(t *testing.T) {
	t.Parallel()

	dialog, _ := newTerminalDialogForTest(t, "sleep 5")

	action := dialog.HandleMsg(tea.KeyPressMsg{Code: 'a', Text: "a"})
	input, ok := action.(ActionTerminalInput)
	require.True(t, ok, "regular keys become terminal input, got %T", action)
	require.NotNil(t, input.Apply)

	action = dialog.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEscape})
	_, ok = action.(ActionTerminalInput)
	require.True(t, ok, "escape is passed through to the child, got %T", action)

	action = dialog.HandleMsg(tea.PasteMsg{Content: "pasted"})
	_, ok = action.(ActionTerminalInput)
	require.True(t, ok, "paste becomes terminal input, got %T", action)
}

func TestTerminalDialogCloseKeyKills(t *testing.T) {
	t.Parallel()

	dialog, session := newTerminalDialogForTest(t, "sleep 60")

	action := dialog.HandleMsg(tea.KeyPressMsg{Code: 'q', Mod: tea.ModCtrl})
	_, ok := action.(ActionTerminalKill)
	require.True(t, ok, "ctrl+q should ask to terminate, got %T", action)
	require.False(t, session.Exited(), "the dialog must not kill the session itself")
}

func TestTerminalDialogExitCompletes(t *testing.T) {
	t.Parallel()

	dialog, session := newTerminalDialogForTest(t, "printf 'hello-terminal\\n'")

	select {
	case <-session.Done():
	case <-time.After(15 * time.Second):
		t.Fatal("session did not exit in time")
	}

	action := dialog.HandleMsg(TerminalExitMsg{})
	complete, ok := action.(ActionTerminalComplete)
	require.True(t, ok, "exit should complete the session, got %T", action)
	require.Contains(t, complete.Result.Output, "hello-terminal")
	require.Equal(t, 0, complete.Result.ExitCode)
	require.False(t, complete.Result.Terminated)
}

func TestTerminalDialogTerminatedResult(t *testing.T) {
	t.Parallel()

	dialog, _ := newTerminalDialogForTest(t, "printf bye")

	// The user closed the terminal before the command exited.
	dialog.HandleMsg(tea.KeyPressMsg{Code: 'q', Mod: tea.ModCtrl})

	action := dialog.HandleMsg(TerminalExitMsg{})
	complete, ok := action.(ActionTerminalComplete)
	require.True(t, ok)
	require.True(t, complete.Result.Terminated)
}

func TestTerminalDialogOutputMsgIsNilAction(t *testing.T) {
	t.Parallel()

	dialog, _ := newTerminalDialogForTest(t, "sleep 5")
	require.Nil(t, dialog.HandleMsg(TerminalOutputMsg{}))
}

func TestTerminalDialogMouseTranslation(t *testing.T) {
	t.Parallel()

	dialog, _ := newTerminalDialogForTest(t, "sleep 5")

	scr, area := drawTerminal(t, dialog, 100, 30)
	require.False(t, dialog.contentRect.Empty())

	// Click inside the content area becomes input.
	inside := tea.MouseClickMsg(tea.Mouse{
		X:      dialog.contentRect.Min.X + 2,
		Y:      dialog.contentRect.Min.Y + 2,
		Button: tea.MouseLeft,
	})
	_, ok := dialog.HandleMsg(inside).(ActionTerminalInput)
	require.True(t, ok, "click inside content should become input")

	// A click on the frame is dropped.
	outside := tea.MouseClickMsg(tea.Mouse{X: 0, Y: 0, Button: tea.MouseLeft})
	require.Nil(t, dialog.HandleMsg(outside))

	_ = scr
	_ = area
}

func TestTerminalDialogDraw(t *testing.T) {
	t.Parallel()

	dialog, session := newTerminalDialogForTest(t, "printf 'drawn-output\\n'")

	select {
	case <-session.Done():
	case <-time.After(15 * time.Second):
		t.Fatal("session did not exit in time")
	}

	scr, _ := drawTerminal(t, dialog, 100, 30)
	rendered := scr.Render()

	require.Contains(t, rendered, "ctrl+q", "hint should be drawn")
	require.Contains(t, rendered, "printf", "command should be drawn")
	require.Contains(t, rendered, "drawn-output", "emulator content should be drawn")
}

func TestTerminalDialogTooSmall(t *testing.T) {
	t.Parallel()

	dialog, _ := newTerminalDialogForTest(t, "sleep 5")

	scr := uvScreenBuffer(30, 6)
	dialog.Draw(scr, image.Rect(0, 0, 30, 6))
	require.Contains(t, scr.Render(), "terminal too small")
}

func TestTerminalDialogResizesSessionToContentArea(t *testing.T) {
	t.Parallel()

	dialog, session := newTerminalDialogForTest(t, "sleep 5")

	drawTerminal(t, dialog, 80, 24)

	cols, rows := session.Size()
	require.Equal(t, 78, cols, "width minus frame")
	require.Equal(t, 21, rows, "height minus frame and header")
}

func drawTerminal(t *testing.T, dialog *TerminalDialog, width, height int) (uv.ScreenBuffer, image.Rectangle) {
	t.Helper()

	scr := uvScreenBuffer(width, height)
	area := image.Rect(0, 0, width, height)
	dialog.Draw(scr, area)
	return scr, area
}
