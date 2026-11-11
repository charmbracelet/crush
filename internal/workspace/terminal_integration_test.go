package workspace_test

import (
	"context"
	"io"
	"runtime"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/client"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/shell"
	"github.com/charmbracelet/crush/internal/terminal"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

// recorderModel is a TUI that only records the messages it receives.
type recorderModel struct{ msgs chan<- tea.Msg }

func (recorderModel) Init() tea.Cmd { return nil }

func (m recorderModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	select {
	case m.msgs <- msg:
	default:
	}
	return m, nil
}

func (recorderModel) View() tea.View { return tea.NewView("") }

// TestClientWorkspace_RunsAgentTerminals checks the client/server path of
// the interactive terminal end to end, with the agent's own tool on the
// server and a real TUI subscription on the client: the call is forwarded
// to the TUI viewing the session, which runs it on its own machine and
// shows the terminal. An editor plugin that follows the same session (and
// sorts first) must not be picked: it would never answer. Terminals the
// user ran on the client are stored on the server.
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
	msgs := make(chan tea.Msg, 1024)
	program := tea.NewProgram(recorderModel{msgs},
		tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutRenderer(), tea.WithoutSignals())
	go func() { _, _ = program.Run() }()
	t.Cleanup(program.Kill)
	go ws.Subscribe(program)
	t.Cleanup(shell.GetInteractiveSessionManager().KillAll)

	sess, err := ws.CreateSession(ctx, "chat")
	require.NoError(t, err)
	require.Eventually(t, func() bool { return ws.SetCurrentSession(ctx, sess.ID) == nil },
		5*time.Second, 20*time.Millisecond, "the TUI attaches its event stream")

	// The editor plugin: an API client of the same workspace that follows
	// the TUI's session but knows nothing about terminals.
	var editor *client.Client
	for range 64 {
		editor = rt.newClient(t, cwd)
		if editor.ClientID() < c.ClientID() {
			break
		}
	}
	require.Less(t, editor.ClientID(), c.ClientID())
	_, err = editor.CreateWorkspace(ctx, proto.Workspace{Path: cwd, DataDir: wsProto.DataDir})
	require.NoError(t, err)
	_, err = editor.SubscribeEvents(ctx, wsProto.ID)
	require.NoError(t, err)
	require.NoError(t, editor.SetCurrentSession(ctx, wsProto.ID, sess.ID))

	server, err := rt.srv.Backend().GetWorkspace(wsProto.ID)
	require.NoError(t, err)
	server.Permissions.AutoApproveSession(sess.ID)
	tool := tools.NewTerminalTool(server.Permissions, server.Terminal, cwd, t.TempDir(), nil)
	run := func(input string) fantasy.ToolResponse {
		t.Helper()
		done := make(chan fantasy.ToolResponse, 1)
		go func() {
			resp, err := tool.Run(context.WithValue(ctx, tools.SessionIDContextKey, sess.ID),
				fantasy.ToolCall{ID: "tool-call", Name: tools.TerminalToolName, Input: input})
			if err != nil {
				resp = fantasy.NewTextErrorResponse(err.Error())
			}
			done <- resp
		}()
		select {
		case resp := <-done:
			require.False(t, resp.IsError, resp.Content)
			return resp
		case <-time.After(10 * time.Second):
			t.Fatalf("the terminal call hung: %s", input)
			return fantasy.ToolResponse{}
		}
	}

	run(`{"action":"start","command":"printf remote-hello; sleep 30","wait":false}`)
	shown := false
	for !shown {
		select {
		case msg := <-msgs:
			if ev, ok := msg.(pubsub.Event[terminal.Request]); ok {
				require.NotNil(t, ev.Payload.Session, "the client's TUI gets the session it runs")
				require.Equal(t, sess.ID, ev.Payload.SessionID)
				shown = true
			}
		case <-time.After(5 * time.Second):
			t.Fatal("the terminal was not shown on the client")
		}
	}

	require.Eventually(t, func() bool {
		return strings.Contains(run(`{"action":"read"}`).Content, "remote-hello")
	}, 5*time.Second, 100*time.Millisecond)
	run(`{"action":"kill"}`)

	require.NoError(t, ws.PersistShellCommand(ctx, sess.ID, "htop", "load average", 0))
	stored, err := ws.ListMessages(ctx, sess.ID)
	require.NoError(t, err)
	var recorded []message.ShellCommand
	for _, msg := range stored {
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
