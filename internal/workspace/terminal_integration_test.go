package workspace_test

import (
	"encoding/json"
	"runtime"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/shell"
	"github.com/charmbracelet/crush/internal/terminal"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

// TestClientWorkspace_RunsAgentTerminals checks the client/server path of
// the interactive terminal end to end: the server's terminal service
// forwards the agent's calls to the client viewing the session, which runs
// them on its own machine and shows the terminal in its TUI, and terminals
// the user ran on the client are stored on the server.
func TestClientWorkspace_RunsAgentTerminals(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs a POSIX shell")
	}
	xdgIsolate(t)
	rt := newRuntimeServer(t)
	ctx := t.Context()

	cwd := t.TempDir()
	c := rt.newClient(t, cwd)
	wsProto, err := c.CreateWorkspace(ctx, proto.Workspace{Path: cwd, DataDir: t.TempDir()})
	require.NoError(t, err)

	ws := workspace.NewClientWorkspace(c, *wsProto)
	shown := make(chan pubsub.Event[terminal.Request], 1)
	ws.AttachTerminalsForTest(func(msg tea.Msg) {
		if ev, ok := msg.(pubsub.Event[terminal.Request]); ok {
			shown <- ev
		}
	})
	evc, err := c.SubscribeEvents(ctx, wsProto.ID)
	require.NoError(t, err)
	go ws.ConsumeEventsForTest(evc, func(tea.Msg) {})
	t.Cleanup(shell.GetInteractiveSessionManager().KillAll)

	sess, err := ws.CreateSession(ctx, "chat")
	require.NoError(t, err)
	require.NoError(t, ws.SetCurrentSession(ctx, sess.ID))

	server, err := rt.srv.Backend().GetWorkspace(wsProto.ID)
	require.NoError(t, err)
	exec := func(params tools.TerminalParams) terminal.Result {
		t.Helper()
		raw, err := json.Marshal(params)
		require.NoError(t, err)
		result, remote, err := server.Terminal.Exec(ctx, terminal.Call{
			SessionID:  sess.ID,
			ToolCallID: "tool-call",
			Op:         params.Action,
			Params:     raw,
		})
		require.NoError(t, err)
		require.True(t, remote, "a server forwards terminal calls to its client")
		require.False(t, result.IsError, result.Content)
		return result
	}

	noWait := false
	exec(tools.TerminalParams{Action: "start", Command: "printf remote-hello", Wait: &noWait})
	select {
	case ev := <-shown:
		require.NotNil(t, ev.Payload.Session, "the client's TUI gets the session it runs")
		require.Equal(t, sess.ID, ev.Payload.SessionID)
	case <-time.After(5 * time.Second):
		t.Fatal("the terminal was not shown on the client")
	}

	read := exec(tools.TerminalParams{Action: "read", WaitSeconds: 5})
	require.Contains(t, read.Content, "remote-hello")

	require.NoError(t, ws.PersistShellCommand(ctx, sess.ID, "htop", "load average", 0))
	msgs, err := ws.ListMessages(ctx, sess.ID)
	require.NoError(t, err)
	var recorded []message.ShellCommand
	for _, msg := range msgs {
		for _, part := range msg.Parts {
			if cmd, ok := part.(message.ShellCommand); ok {
				recorded = append(recorded, cmd)
			}
		}
	}
	require.Len(t, recorded, 1)
	require.Equal(t, "htop", recorded[0].Command)
	require.Equal(t, "load average", recorded[0].Output)
}
