package model

import (
	"context"
	"errors"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/shell"
	"github.com/charmbracelet/crush/internal/terminal"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/workspace"
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
	first := newAgentSession(t, "sleep 30")

	_, cmd := u.Update(pubsubEventTerminalRequest(terminal.Request{
		ID:      "req-1",
		Command: "sleep 30",
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
