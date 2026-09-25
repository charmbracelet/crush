package model

import (
	"context"
	"errors"
	"image"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/shell"
	"github.com/charmbracelet/crush/internal/terminal"
	"github.com/charmbracelet/crush/internal/ui/attachments"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/workspace"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"
)

var errTestSpawn = errors.New("spawn failed")

func pubsubEventTerminalRequest(req terminal.Request) pubsub.Event[terminal.Request] {
	return pubsub.Event[terminal.Request]{Type: pubsub.CreatedEvent, Payload: req}
}

// runCmdSync executes a tea.Cmd (or batch) and collects the messages it
// produces, recursively unwrapping nested batches. Ancillary refresh
// commands that need more of the workspace than this minimal mock provides
// are skipped.
func runCmdSync(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := safeRunCmd(cmd)
	if batch, ok := msg.(tea.BatchMsg); ok {
		var msgs []tea.Msg
		for _, sub := range batch {
			if sub == nil {
				continue
			}
			msgs = append(msgs, runCmdSync(sub)...)
		}
		return msgs
	}
	if msg == nil {
		return nil
	}
	return []tea.Msg{msg}
}

func safeRunCmd(cmd tea.Cmd) (msg tea.Msg) {
	defer func() {
		if r := recover(); r != nil {
			msg = nil
		}
	}()
	return cmd()
}

// terminalTestWorkspace provides the few workspace methods the terminal
// flow touches.
type terminalTestWorkspace struct {
	workspace.Workspace

	persisted  *shellCommandRecord
	workingDir string
}

type shellCommandRecord struct {
	command  string
	output   string
	exitCode int
}

func (w *terminalTestWorkspace) WorkingDir() string { return w.workingDir }

func (w *terminalTestWorkspace) Config() *config.Config { return &config.Config{} }

func (w *terminalTestWorkspace) PersistShellCommand(_ context.Context, _, command, output string, exitCode int) error {
	w.persisted = &shellCommandRecord{command: command, output: output, exitCode: exitCode}
	return nil
}

func (*terminalTestWorkspace) AgentIsReady() bool { return false }

func (*terminalTestWorkspace) AgentIsBusy() bool { return false }

func (*terminalTestWorkspace) PermissionSkipRequests() bool { return false }

// drainInteractiveManager empties the global interactive session manager.
// Sessions it owns are global, so these tests must not run in parallel.
func drainInteractiveManager(t *testing.T) {
	t.Helper()

	mgr := shell.GetInteractiveSessionManager()
	for _, id := range mgr.List() {
		if session, ok := mgr.Get(id); ok {
			_ = session.Kill()
			_ = session.Close()
		}
		mgr.Remove(id)
	}
}

func newTerminalTestUI(t *testing.T) (*UI, *terminalTestWorkspace) {
	t.Helper()

	drainInteractiveManager(t)

	u := newTestUI()
	u.dialog = dialog.NewOverlay()
	ws := &terminalTestWorkspace{workingDir: t.TempDir()}
	u.com.Workspace = ws
	return u, ws
}

func newAgentSession(t *testing.T, command string) *shell.InteractiveSession {
	t.Helper()

	session, err := shell.NewInteractiveSession(shell.InteractiveSessionOptions{
		Command:    command,
		WorkingDir: t.TempDir(),
	})
	require.NoError(t, err)
	require.NoError(t, shell.GetInteractiveSessionManager().Register(session))
	t.Cleanup(func() {
		_ = session.Kill()
		_ = session.Close()
		shell.GetInteractiveSessionManager().Remove(session.ID())
	})
	return session
}

func TestTerminalRequestAttachesDialogAndClosesOnExit(t *testing.T) {
	u, _ := newTerminalTestUI(t)
	session := newAgentSession(t, "printf 'flow-output\\n'")

	_, cmd := u.Update(pubsubEventTerminalRequest(terminal.Request{
		ID:         "req-1",
		ToolCallID: "call-1",
		Command:    "printf 'flow-output\\n'",
		WorkingDir: session.WorkingDir(),
		Session:    session,
	}))

	// The dialog opens around the session that came with the request.
	require.NotNil(t, cmd)
	require.True(t, u.dialog.ContainsDialog(dialog.TerminalID))
	require.NotNil(t, u.activeTerminal)
	require.True(t, u.activeTerminal.agentStarted)

	// Drive the watcher until the session exits.
	watchCmd := u.watchTerminalSession(session)
	deadline := time.Now().Add(15 * time.Second)
	for {
		msg := watchCmd()
		if exitMsg, exited := msg.(dialog.TerminalExitMsg); exited {
			_, _ = u.Update(exitMsg)
			break
		}
		if outputMsg, output := msg.(dialog.TerminalOutputMsg); output {
			_, watchCmd = u.Update(outputMsg)
			require.True(t, time.Now().Before(deadline), "session did not exit in time")
			continue
		}
		require.True(t, time.Now().Before(deadline), "session did not exit in time")
	}

	require.False(t, u.dialog.ContainsDialog(dialog.TerminalID))
	require.Nil(t, u.activeTerminal)
}

func TestTerminalRequestIgnoredWhenDialogOpen(t *testing.T) {
	u, _ := newTerminalTestUI(t)
	first := newAgentSession(t, "sh -c 'printf first; sleep 30'")

	_, cmd := u.Update(pubsubEventTerminalRequest(terminal.Request{
		ID:      "req-1",
		Command: "sh -c 'printf first; sleep 30'",
		Session: first,
	}))
	_ = runCmdSync(cmd)
	require.True(t, u.dialog.ContainsDialog(dialog.TerminalID))
	require.Same(t, first, u.activeTerminal.session)
}

func TestUserInteractiveShellPersistsOnExit(t *testing.T) {
	u, ws := newTerminalTestUI(t)
	u.session = &session.Session{ID: "user-session"}

	session := newAgentSession(t, "printf 'user-shell\\n'")
	_, cmd := u.Update(terminalSessionMsg{Session: session, UserCommand: "printf 'user-shell\\n'"})
	_ = runCmdSync(cmd)
	require.NotNil(t, u.activeTerminal)
	require.False(t, u.activeTerminal.agentStarted)

	// The process exits; the dialog resolves and the result is persisted
	// like a bang-mode command.
	select {
	case <-session.Done():
	case <-time.After(15 * time.Second):
		t.Fatal("session did not exit")
	}

	_, done := u.Update(dialog.TerminalExitMsg{})
	_ = runCmdSync(done)
	require.Nil(t, u.activeTerminal)
	require.NotNil(t, ws.persisted, "user sessions are persisted")
	require.Equal(t, "printf 'user-shell\\n'", ws.persisted.command)
	require.Contains(t, ws.persisted.output, "user-shell")
}

func TestTerminalSpawnErrorReportsInfo(t *testing.T) {
	u, _ := newTerminalTestUI(t)
	_, cmd := u.Update(terminalSpawnErrorMsg{Err: errTestSpawn})
	require.NotNil(t, cmd)
	_ = runCmdSync(cmd)
}

func TestTerminalDockPanelHeight(t *testing.T) {
	t.Parallel()

	// Plenty of room: the content height is capped at
	// terminalDockMaxContentRows and chrome is added on top.
	require.Equal(t, terminalDockMaxContentRows+terminalDockChrome, terminalDockPanelHeight(80))

	// Mid-size: the panel takes about half the column, never less than
	// the minimum emulator height.
	require.Equal(t, max(shell.MinInteractiveRows, 15/2-terminalDockChrome)+terminalDockChrome, terminalDockPanelHeight(15))

	// Too short: no room for a usable chat above the panel.
	require.Zero(t, terminalDockPanelHeight(terminalDockChrome+shell.MinInteractiveRows+terminalDockMinChatRows-1))
	require.NotZero(t, terminalDockPanelHeight(terminalDockChrome+shell.MinInteractiveRows+terminalDockMinChatRows))
}

func TestTerminalDocksIntoChatColumn(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.updateLayoutAndSize()
	fullMain := u.layout.main
	require.True(t, u.layout.terminal.Empty())

	// A live terminal session is docked; generateLayout only needs the
	// non-nil marker.
	u.activeTerminal = &activeTerminalSession{}
	u.updateLayoutAndSize()

	term := u.layout.terminal
	require.False(t, term.Empty())
	// The chat column keeps a usable slice above the panel.
	require.GreaterOrEqual(t, u.layout.main.Dy(), terminalDockMinChatRows)
	// The panel sits at the bottom of the original chat column.
	require.Equal(t, fullMain.Max.Y, term.Max.Y)
	require.Equal(t, fullMain.Min.Y, u.layout.main.Min.Y)
	require.Equal(t, fullMain.Min.X, term.Min.X)
	require.Equal(t, fullMain.Max.X, term.Max.X)
}

func TestTerminalFallsBackWhenChatTooShort(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	// Shrink the window until the chat column cannot host the docked
	// panel and a usable chat.
	u.height = 12
	u.activeTerminal = &activeTerminalSession{}
	u.updateLayoutAndSize()

	require.True(t, u.layout.terminal.Empty(), "no room to dock: terminal must stay an overlay")
}

func TestTerminalDockContentRespectsMinInteractiveRows(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.activeTerminal = &activeTerminalSession{}
	u.updateLayoutAndSize()

	term := u.layout.terminal
	require.False(t, term.Empty())
	// The emulator always keeps at least MinInteractiveRows of content
	// inside the panel chrome.
	require.GreaterOrEqual(t, term.Dy()-terminalDockChrome, shell.MinInteractiveRows)
}

// newAttachedTerminalUI returns a chat UI with a docked interactive
// terminal around a live session. The command prints first so the session
// watcher resolves without waiting for the sleep to end.
func newAttachedTerminalUI(t *testing.T) (*UI, *shell.InteractiveSession) {
	return newTerminalUIOfKind(t, false)
}

// newAgentDrivenTerminalUI returns a chat UI with a docked terminal the
// agent owns: a read-only view the user cannot type into.
func newAgentDrivenTerminalUI(t *testing.T) (*UI, *shell.InteractiveSession) {
	return newTerminalUIOfKind(t, true)
}

func newTerminalUIOfKind(t *testing.T, agentDriven bool) (*UI, *shell.InteractiveSession) {
	t.Helper()

	u, _ := newTerminalTestUI(t)
	session := newAgentSession(t, "sh -c 'printf attached; sleep 30'")
	_, cmd := u.Update(pubsubEventTerminalRequest(terminal.Request{
		ID:          "req-" + t.Name(),
		ToolCallID:  "call-" + t.Name(),
		Command:     "sh -c 'printf attached; sleep 30'",
		AgentDriven: agentDriven,
		Session:     session,
	}))
	_ = runCmdSync(cmd)
	require.True(t, u.dialog.ContainsDialog(dialog.TerminalID))
	if !agentDriven {
		require.Equal(t, uiFocusTerminal, u.focus, "attaching a user-owned terminal focuses it")
	}
	return u, session
}

func TestTerminalTakesMouseInsidePanelOnly(t *testing.T) {
	u, _ := newAttachedTerminalUI(t)
	term := u.layout.terminal
	require.False(t, term.Empty())

	// A click inside the panel is the terminal's, and focuses it.
	u.focus = uiFocusEditor
	require.True(t, u.terminalTakesMouse(image.Pt(term.Min.X+1, term.Min.Y+1), true))
	require.Equal(t, uiFocusTerminal, u.focus)

	// Hover inside the panel never steals focus.
	u.focus = uiFocusEditor
	require.True(t, u.terminalTakesMouse(image.Pt(term.Min.X+1, term.Min.Y+1), false))
	require.Equal(t, uiFocusEditor, u.focus)

	// Over the chat, the event falls through to the chat.
	require.False(t, u.terminalTakesMouse(image.Pt(u.layout.main.Min.X+1, u.layout.main.Min.Y+1), true))
}

func TestAgentDrivenTerminalIsReadOnly(t *testing.T) {
	u, session := newAgentDrivenTerminalUI(t)

	// Attaching leaves the current focus alone: the panel is a view.
	require.Equal(t, uiFocusEditor, u.focus)
	require.True(t, u.activeTerminal.agentStarted)

	// The session keeps its ghost fullscreen size, which is what the agent
	// reads, while the docked panel shows only a slice of it.
	cols, rows := u.terminalGhostSize()
	gotCols, gotRows := session.Size()
	require.Equal(t, cols, gotCols, "ghost fullscreen width")
	require.Equal(t, rows, gotRows, "ghost fullscreen height")
	require.Less(t, u.layout.terminal.Dy()-terminalDockChrome, rows,
		"the docked panel is a slice of the ghost screen")

	// The pane is focusable like a user-owned one: a click focuses it.
	u.focus = uiFocusEditor
	require.True(t, u.terminalTakesMouse(image.Pt(u.layout.terminal.Min.X+1, u.layout.terminal.Min.Y+1), true))
	require.Equal(t, uiFocusTerminal, u.focus)

	// Tab cycles editor -> chat -> terminal -> editor.
	u.focus = uiFocusEditor
	_ = u.cycleTerminalFocus()
	require.Equal(t, uiFocusMain, u.focus)
	_ = u.cycleTerminalFocus()
	require.Equal(t, uiFocusTerminal, u.focus)
	_ = u.cycleTerminalFocus()
	require.Equal(t, uiFocusEditor, u.focus)
}

// TestAgentDrivenTerminalCtrlFTogglesFullscreen checks that the focused
// agent-driven pane toggles fullscreen like a user-owned one.
func TestAgentDrivenTerminalCtrlFTogglesFullscreen(t *testing.T) {
	u, session := newAgentDrivenTerminalUI(t)
	u.attachments = attachments.New(nil, attachments.Keymap{})
	u.keyMap = DefaultKeyMap()

	// The dialog absorbs keystrokes during its input grace period after
	// opening; wait it out before pressing keys.
	time.Sleep(1700 * time.Millisecond)

	// Focus the pane, then ctrl+f toggles fullscreen on and off.
	u.focusTerminal()

	u.handleKeyPressMsg(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
	require.True(t, u.activeTerminal.fullscreen)
	require.True(t, u.layout.terminal.Empty(), "fullscreen is not docked")
	gotCols, gotRows := session.Size()
	require.Equal(t, u.width-2, gotCols, "fullscreen width")
	require.Equal(t, u.height-3, gotRows, "fullscreen height")

	u.handleKeyPressMsg(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
	require.False(t, u.activeTerminal.fullscreen)
	require.False(t, u.layout.terminal.Empty())
	gotCols, gotRows = session.Size()
	ghostCols, ghostRows := u.terminalGhostSize()
	require.Equal(t, ghostCols, gotCols)
	require.Equal(t, ghostRows, gotRows)
}

// TestUserTerminalCtrlFStaysEditorKey checks that a user-owned terminal
// keeps ctrl+f as the editor's attachment key while the user is typing;
// only a focused terminal toggles fullscreen.
func TestUserTerminalCtrlFStaysEditorKey(t *testing.T) {
	u, _ := newAttachedTerminalUI(t)
	u.attachments = attachments.New(nil, attachments.Keymap{})
	u.keyMap = DefaultKeyMap()

	// The user tabbed away to the editor.
	_ = u.cycleTerminalFocus()
	require.Equal(t, uiFocusEditor, u.focus)

	u.handleKeyPressMsg(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
	require.False(t, u.activeTerminal.fullscreen, "ctrl+f must stay the editor's attach key")
}

func TestAgentDrivenTerminalFullscreenAbsorbsKeys(t *testing.T) {
	u, _ := newAgentDrivenTerminalUI(t)

	// Fullscreen covers everything, so the terminal absorbs the keyboard
	// even though the agent owns the session.
	u.setTerminalFullscreen(true)
	require.Equal(t, uiFocusTerminal, u.focus)
	require.True(t, u.activeTerminal.fullscreen)

	_ = u.cycleTerminalFocus()
	require.False(t, u.activeTerminal.fullscreen)
	require.Equal(t, uiFocusEditor, u.focus)
}

func TestTerminalTakesEverythingWhenFullscreen(t *testing.T) {
	u, _ := newAttachedTerminalUI(t)

	u.setTerminalFullscreen(true)
	require.True(t, u.activeTerminal.fullscreen)
	require.True(t, u.layout.terminal.Empty(), "fullscreen covers the whole window")
	require.True(t, u.terminalTakesMouse(image.Pt(2, 2), true))
	require.Equal(t, uiFocusTerminal, u.focus, "fullscreen keeps the keyboard on the terminal")

	u.setTerminalFullscreen(false)
	require.False(t, u.activeTerminal.fullscreen)
	require.False(t, u.layout.terminal.Empty())
}

func TestTerminalFocusCycle(t *testing.T) {
	u, _ := newAttachedTerminalUI(t)
	require.Equal(t, uiFocusTerminal, u.focus)

	// terminal -> editor
	_ = u.cycleTerminalFocus()
	require.Equal(t, uiFocusEditor, u.focus)

	// editor -> chat
	_ = u.cycleTerminalFocus()
	require.Equal(t, uiFocusMain, u.focus)

	// chat -> terminal
	_ = u.cycleTerminalFocus()
	require.Equal(t, uiFocusTerminal, u.focus)
}

func TestTerminalFullscreenDropsWhenCyclingAway(t *testing.T) {
	u, _ := newAttachedTerminalUI(t)
	u.setTerminalFullscreen(true)

	_ = u.cycleTerminalFocus()
	require.False(t, u.activeTerminal.fullscreen, "leaving the terminal docks it again")
	require.Equal(t, uiFocusEditor, u.focus)
}

func TestTerminalCloseReturnsEditorFocus(t *testing.T) {
	u, _ := newAttachedTerminalUI(t)
	require.Equal(t, uiFocusTerminal, u.focus)

	_, cmd := u.Update(dialog.TerminalExitMsg{})
	_ = runCmdSync(cmd)
	require.Nil(t, u.activeTerminal)
	require.Equal(t, uiFocusEditor, u.focus, "closing hands focus back to the editor")
}

func TestTerminalFocusRoutesKeys(t *testing.T) {
	u, _ := newAttachedTerminalUI(t)
	u.attachments = attachments.New(nil, attachments.Keymap{})
	u.keyMap = DefaultKeyMap()

	// While the terminal holds focus, global shortcuts belong to the child
	// process, not the UI.
	require.Equal(t, uiFocusTerminal, u.focus)
	u.handleKeyPressMsg(tea.KeyPressMsg{Code: 'g', Mod: tea.ModCtrl})
	require.False(t, u.status.ShowingAll(), "focused terminal swallows ctrl+g")

	// Tabbed away, the editor gets its keys back.
	_ = u.cycleTerminalFocus()
	require.Equal(t, uiFocusEditor, u.focus)
	u.handleKeyPressMsg(tea.KeyPressMsg{Code: 'g', Mod: tea.ModCtrl})
	require.True(t, u.status.ShowingAll(), "tabbed away, ctrl+g reaches the UI again")
}

func TestDrawTerminalHintsReplacesEditor(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.dialog = dialog.NewOverlay()
	u.keyMap = DefaultKeyMap()
	u.activeTerminal = &activeTerminalSession{}
	u.focus = uiFocusTerminal
	u.updateLayoutAndSize()
	require.False(t, u.layout.terminal.Empty(), "a docked terminal leaves the editor visible")

	scr := uv.NewScreenBuffer(u.width, u.height)
	u.drawTerminalHints(scr, u.layout.editor)

	rendered := scr.Render()
	require.Contains(t, rendered, "quit")
	require.Contains(t, rendered, "fullscreen")
	// The blurred textarea prompt (the row of colons) is what the hints
	// replace.
	require.NotContains(t, rendered, ":::")
}
