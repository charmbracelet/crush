package dialog

import (
	"image"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
	return newTerminalDialogForTestOptions(t, command, TerminalDialogOptions{Command: command})
}

func newTerminalDialogForTestOptions(t *testing.T, command string, opts TerminalDialogOptions) (*TerminalDialog, *shell.InteractiveSession) {
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
	dialog := NewTerminalDialog(&common.Common{Styles: &s}, session, opts)
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

func TestTerminalDialogFullscreenKeyToggles(t *testing.T) {
	t.Parallel()

	dialog, _ := newTerminalDialogForTest(t, "sleep 5")

	action := dialog.HandleMsg(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
	_, ok := action.(ActionTerminalFullscreen)
	require.True(t, ok, "ctrl+f should ask to toggle fullscreen, got %T", action)
}

func TestTerminalDialogHeaderShowsNoKeybinds(t *testing.T) {
	t.Parallel()

	dialog, _ := newTerminalDialogForTest(t, "sleep 5")
	scr, _ := drawTerminal(t, dialog, 80, 24)
	rendered := scr.Render()
	// Keybinds live in the status bar and editor hint row, not on the
	// panel.
	require.NotContains(t, rendered, "ctrl+q")
	require.NotContains(t, rendered, "ctrl+f")
	require.Contains(t, rendered, "sleep 5", "the command still shows")
}

func TestTerminalDialogHeaderFitsNarrowPanel(t *testing.T) {
	t.Parallel()

	dialog, _ := newTerminalDialogForTest(t, "a-very-long-command-name-here")

	// Status chip and hints must not wrap the header on narrow panels.
	for _, width := range []int{10, 20, 40, 80} {
		require.LessOrEqual(t, lipgloss.Width(dialog.renderHeader(width)), width,
			"header must fit a %d-column panel", width)
	}
}

func TestTerminalDialogStatusChip(t *testing.T) {
	t.Parallel()

	dialog, session := newTerminalDialogForTest(t, "printf 'status-done\\n'")
	select {
	case <-session.Done():
	case <-time.After(15 * time.Second):
		t.Fatal("session did not exit in time")
	}

	scr, _ := drawTerminal(t, dialog, 80, 24)
	require.Contains(t, scr.Render(), "exited", "an exited session is labelled")
}

func TestTerminalDialogOwnsLabel(t *testing.T) {
	t.Parallel()

	// A terminal the agent drives is labelled "agent"; one the user works
	// in says "user" even when the agent opened it.
	agent, _ := newTerminalDialogForTestOptions(t, "sleep 30", TerminalDialogOptions{
		Command:     "sleep 30",
		AgentDriven: true,
	})
	scr, _ := drawTerminal(t, agent, 80, 24)
	rendered := scr.Render()
	require.Contains(t, rendered, "agent", "an agent-driven session is labelled")
	require.Contains(t, rendered, "running")

	user, _ := newTerminalDialogForTestOptions(t, "sleep 30", TerminalDialogOptions{Command: "sleep 30"})
	scr, _ = drawTerminal(t, user, 80, 24)
	require.Contains(t, scr.Render(), "user", "a user-owned session is labelled")
}

func TestTerminalDialogAgentDrivenRefusesInput(t *testing.T) {
	t.Parallel()

	dialog, _ := newTerminalDialogForTestOptions(t, "sleep 30", TerminalDialogOptions{
		Command:     "sleep 30",
		AgentDriven: true,
	})

	// Typing, pasting, and mouse input never reach a command the agent is
	// driving.
	action := dialog.HandleMsg(tea.KeyPressMsg{Code: 'a', Text: "a"})
	require.Nil(t, action, "keys must be refused on an agent-driven terminal")
	require.Nil(t, dialog.HandleMsg(tea.PasteMsg{Content: "pasted"}))
	require.Nil(t, dialog.HandleMsg(tea.MouseClickMsg{}))
	require.Nil(t, dialog.HandleMsg(tea.MouseMotionMsg{}))

	// The escape hatches still work.
	action = dialog.HandleMsg(tea.KeyPressMsg{Code: 'q', Mod: tea.ModCtrl})
	require.IsType(t, ActionTerminalKill{}, action)
	action = dialog.HandleMsg(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
	require.IsType(t, ActionTerminalFullscreen{}, action)
}

func TestTerminalDialogAgentDrivenReadonlyHint(t *testing.T) {
	t.Parallel()

	dialog, _ := newTerminalDialogForTestOptions(t, "sleep 30", TerminalDialogOptions{
		Command:     "sleep 30",
		AgentDriven: true,
	})

	scr, _ := drawTerminal(t, dialog, 100, 24)
	require.Contains(t, scr.Render(), "agent")
}

func TestTerminalDialogResolvesThemedColors(t *testing.T) {
	t.Parallel()

	// The child paints a letter with the basic ANSI red code. The host
	// terminal's palette must not decide that color: it has to resolve
	// through the theme's palette so the embedded terminal matches the
	// rest of Crush.
	dialog, session := newTerminalDialogForTest(t, "sh -c 'printf \"\\033[31mR\\033[0m\"; sleep 5'")
	select {
	case <-session.Dirty():
	case <-time.After(15 * time.Second):
		t.Fatal("session did not produce output in time")
	}

	scr := uvScreenBuffer(80, 24)
	dialog.Draw(scr, image.Rect(0, 0, 80, 24))

	// Emulator content starts inside the frame, below the header row.
	cell := scr.CellAt(1, 2)
	require.NotNil(t, cell)
	require.Equal(t, "R", cell.Content)
	require.Equal(t, styles.CharmtonePantera().ANSI[1], cell.Style.Fg,
		"ANSI red must resolve through the theme palette")
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

	require.Contains(t, rendered, "printf", "command should be drawn")
	require.Contains(t, rendered, "drawn-output", "emulator content should be drawn")
	require.NotContains(t, rendered, "ctrl+q", "keybinds are not drawn on the panel")
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

func TestTerminalDialogCursorVisibility(t *testing.T) {
	t.Parallel()

	dialog, session := newTerminalDialogForTest(t, `printf '\033[?25l hidden\033[?25h\n'`)
	dialog.SetFocused(true)

	select {
	case <-session.Done():
	case <-time.After(15 * time.Second):
		t.Fatal("session did not exit in time")
	}

	// The child left the cursor visible again.
	_, area := drawTerminal(t, dialog, 80, 24)
	cur := dialog.Draw(mustScreen(t), area)
	require.NotNil(t, cur, "cursor should be shown when the child shows it")
}

func TestTerminalDialogCursorHiddenWhenChildHides(t *testing.T) {
	t.Parallel()

	dialog, session := newTerminalDialogForTest(t, `printf '\033[?25l hidden\n'`)
	dialog.SetFocused(true)

	select {
	case <-session.Done():
	case <-time.After(15 * time.Second):
		t.Fatal("session did not exit in time")
	}

	_, area := drawTerminal(t, dialog, 80, 24)
	cur := dialog.Draw(mustScreen(t), area)
	require.Nil(t, cur, "cursor must be hidden when the child hides it")
}

func TestTerminalDialogUnfocusedPaintsGhostCursor(t *testing.T) {
	t.Parallel()

	// An unfocused terminal cannot use the host cursor, but the caret must
	// stay visible: it is what makes the agent's (or the session's)
	// position legible on a read-only panel.
	dialog, session := newTerminalDialogForTest(t, `printf '\033[?25l hidden\033[?25h\n'`)

	select {
	case <-session.Done():
	case <-time.After(15 * time.Second):
		t.Fatal("session did not exit in time")
	}

	// Reference draw with the terminal focused: real cursor, no inversion.
	dialog.SetFocused(true)
	scrFocused := uvScreenBuffer(80, 24)
	dialog.Draw(scrFocused, image.Rect(0, 0, 80, 24))
	require.NotNil(t, dialog.Draw(mustScreen(t), image.Rect(0, 0, 80, 24)),
		"a focused terminal shows the real cursor")

	// Unfocused: the ghost caret inverts the cell under the emulator
	// cursor instead of claiming the host cursor.
	dialog.SetFocused(false)
	scr := uvScreenBuffer(80, 24)
	require.Nil(t, dialog.Draw(scr, image.Rect(0, 0, 80, 24)))

	emu := session.Emulator()
	pos := emu.CursorPosition()
	x := 1 + pos.X
	y := 2 + pos.Y

	want := scrFocused.CellAt(x, y)
	got := scr.CellAt(x, y)
	require.NotNil(t, want)
	require.NotNil(t, got)
	require.Equal(t, want.Style.Fg, got.Style.Bg, "the ghost caret swaps the cell's colors")
	require.Equal(t, want.Style.Bg, got.Style.Fg)
}

func mustScreen(t *testing.T) uv.ScreenBuffer {
	t.Helper()
	return uv.NewScreenBuffer(80, 24)
}
