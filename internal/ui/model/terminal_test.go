package model

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/terminal"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

func pubsubEventTerminalRequest(req terminal.Request) pubsub.Event[terminal.Request] {
	return pubsub.Event[terminal.Request]{Type: pubsub.CreatedEvent, Payload: req}
}

func pubsubEventTerminalNotification(n terminal.Notification) pubsub.Event[terminal.Notification] {
	return pubsub.Event[terminal.Notification]{Type: pubsub.CreatedEvent, Payload: n}
}

// runCmdSync executes a tea.Cmd (or batch) and collects the messages it
// produces. Ancillary refresh commands that need more of the workspace
// than this minimal mock provides are skipped.
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
			if m := safeRunCmd(sub); m != nil {
				msgs = append(msgs, m)
			}
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

func findMsg[T any](msgs []tea.Msg) (T, bool) {
	for _, msg := range msgs {
		if v, ok := msg.(T); ok {
			return v, true
		}
	}
	var zero T
	return zero, false
}

// terminalTestWorkspace records how the UI resolves interactive terminal
// sessions.
type terminalTestWorkspace struct {
	workspace.Workspace

	completed  *terminal.Result
	workingDir string
}

func (w *terminalTestWorkspace) TerminalComplete(result terminal.Result) bool {
	w.completed = &result
	return true
}

func (w *terminalTestWorkspace) WorkingDir() string {
	return w.workingDir
}

func (w *terminalTestWorkspace) Config() *config.Config {
	return &config.Config{}
}

func (*terminalTestWorkspace) AgentIsReady() bool { return false }

func (*terminalTestWorkspace) AgentIsBusy() bool { return false }

func (*terminalTestWorkspace) PermissionSkipRequests() bool { return false }

func TestTerminalRequestSpawnsDialogAndResolvesOnExit(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.dialog = dialog.NewOverlay()
	ws := &terminalTestWorkspace{workingDir: t.TempDir()}
	u.com.Workspace = ws

	// The agent asked for an interactive session.
	_, cmd := u.Update(pubsubEventTerminalRequest(terminal.Request{
		ID:         "req-1",
		SessionID:  "session-1",
		ToolCallID: "call-1",
		Command:    "printf 'integration-flow\\n'",
		WorkingDir: ws.workingDir,
	}))
	require.NotNil(t, cmd)

	// Run the spawn command synchronously.
	msgs := runCmdSync(cmd)
	msg, ok := findMsg[terminalSessionMsg](msgs)
	require.True(t, ok, "expected a spawned session, got %v", msgs)

	t.Cleanup(func() {
		_ = msg.Session.Kill()
		_ = msg.Session.Close()
	})
	require.NotNil(t, msg.Request)

	// Attaching opens the dialog and arms the watcher.
	_, watchCmd := u.Update(msg)
	require.NotNil(t, watchCmd)
	require.True(t, u.dialog.ContainsDialog(dialog.TerminalID))
	require.NotNil(t, u.activeTerminal)

	// Drive the watcher until the session exits.
	deadline := time.Now().Add(15 * time.Second)
	for {
		watchMsg := watchCmd()
		if _, exited := watchMsg.(dialog.TerminalExitMsg); exited {
			break
		}
		// Output messages repaint and re-arm the watcher.
		if _, output := watchMsg.(dialog.TerminalOutputMsg); output {
			_, watchCmd = u.Update(watchMsg)
			continue
		}
		require.True(t, time.Now().Before(deadline), "terminal session did not exit in time")
	}

	// The exit message flows through the dialog and resolves the request.
	_, done := u.Update(dialog.TerminalExitMsg{})
	require.NotNil(t, done)
	_ = done()

	require.False(t, u.dialog.ContainsDialog(dialog.TerminalID))
	require.Nil(t, u.activeTerminal)
	require.NotNil(t, ws.completed)
	require.Contains(t, ws.completed.Output, "integration-flow")
	require.Equal(t, 0, ws.completed.ExitCode)
}

func TestTerminalNotificationTearsDownWithoutResolving(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.dialog = dialog.NewOverlay()
	ws := &terminalTestWorkspace{workingDir: t.TempDir()}
	u.com.Workspace = ws

	_, cmd := u.Update(pubsubEventTerminalRequest(terminal.Request{
		ID:         "req-2",
		Command:    "sleep 60",
		WorkingDir: ws.workingDir,
	}))
	msg, ok := findMsg[terminalSessionMsg](runCmdSync(cmd))
	require.True(t, ok)
	t.Cleanup(func() {
		_ = msg.Session.Kill()
		_ = msg.Session.Close()
	})

	_, watchCmd := u.Update(msg)
	require.NotNil(t, watchCmd)
	require.True(t, u.dialog.ContainsDialog(dialog.TerminalID))

	// The run was cancelled elsewhere: the notification must tear the
	// dialog down without resolving the request.
	_, teardown := u.Update(pubsubEventTerminalNotification(terminal.Notification{ID: "req-2"}))
	require.NotNil(t, teardown)
	_ = teardown()

	require.False(t, u.dialog.ContainsDialog(dialog.TerminalID))
	require.Nil(t, u.activeTerminal)
	require.Nil(t, ws.completed, "a cancelled session must not resolve")
}
