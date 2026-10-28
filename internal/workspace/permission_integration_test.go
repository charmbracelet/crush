package workspace_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

// TestClientWorkspace_PermissionParamsSurviveRoundTrip drives permission
// prompts through the real server and client. The prompt the TUI receives
// must carry the params the tool raised, otherwise the dialog renders
// without them and the user approves blind, and answering it must still
// resolve the request.
func TestClientWorkspace_PermissionParamsSurviveRoundTrip(t *testing.T) {
	xdgIsolate(t)
	t.Setenv("CRUSH_DISABLE_PROVIDER_AUTO_UPDATE", "1")
	rt := newRuntimeServer(t)

	cwd := t.TempDir()
	c := rt.newClient(t, cwd)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	wsProto, err := c.CreateWorkspace(ctx, proto.Workspace{Path: cwd, DataDir: t.TempDir()})
	require.NoError(t, err)
	srvWS, err := rt.srv.Backend().GetWorkspace(wsProto.ID)
	require.NoError(t, err)

	evc, err := c.SubscribeEvents(ctx, wsProto.ID)
	require.NoError(t, err)
	ws := workspace.NewClientWorkspace(c, *wsProto)
	msgs := make(chan tea.Msg, 16)
	go ws.ConsumeEventsForTest(evc, func(msg tea.Msg) { msgs <- msg })

	tests := []struct {
		name     string
		toolName string
		params   any
	}{
		{
			// MCP tools raise the model's raw JSON input as a string.
			name:     "mcp",
			toolName: "mcp_github_create_issue",
			params:   `{"repo":"charmbracelet/crush","title":"bug"}`,
		},
		{
			name:     "replace_symbol",
			toolName: tools.ReplaceSymbolToolName,
			params: tools.ReplaceSymbolPermissionsParams{
				FilePath:   filepath.Join(cwd, "main.go"),
				OldContent: "old",
				NewContent: "new",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Cancelling releases the server's request lock if the
			// prompt is never answered, so later cases still run.
			reqCtx, reqCancel := context.WithCancel(ctx)
			t.Cleanup(reqCancel)

			toolCallID := "tc-" + tc.name
			granted := make(chan bool, 1)
			go func() {
				ok, _ := srvWS.Permissions.Request(reqCtx, permission.CreatePermissionRequest{
					SessionID:  "sess-1",
					ToolCallID: toolCallID,
					ToolName:   tc.toolName,
					Action:     "execute",
					Path:       cwd,
					Params:     tc.params,
				})
				granted <- ok
			}()

			var req permission.PermissionRequest
			deadline := time.After(5 * time.Second)
			for req.ToolCallID != toolCallID {
				select {
				case msg := <-msgs:
					if ev, ok := msg.(pubsub.Event[permission.PermissionRequest]); ok {
						req = ev.Payload
					}
				case <-deadline:
					require.FailNow(t, "timed out waiting for the permission request")
				}
			}
			require.Equal(t, tc.params, req.Params, "the prompt must carry the tool's params")

			require.True(t, ws.PermissionGrant(req), "answering the prompt must resolve it")
			require.True(t, <-granted)
		})
	}
}
