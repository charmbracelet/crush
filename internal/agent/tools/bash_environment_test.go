package tools

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/shell"
	"github.com/stretchr/testify/require"
)

func TestBashTool_WorkspaceEnvironment(t *testing.T) {
	t.Setenv("CRUSH_TEST_WORKSPACE_VALUE", "server")
	t.Setenv("CRUSH_TEST_SERVER_ONLY", "kept")
	t.Setenv("CRUSH_TEST_CLIENT_ONLY", "")

	for _, tt := range []struct {
		name       string
		env        []string
		background bool
		autoAfter  int
		want       string
	}{
		{
			name: "inherited environment",
			want: "server|kept||1",
		},
		{
			name: "first workspace",
			env:  []string{"CRUSH_TEST_WORKSPACE_VALUE=first workspace", "CRUSH_TEST_CLIENT_ONLY=client"},
			want: "first workspace|kept|client|1",
		},
		{
			name: "second workspace",
			env:  []string{"CRUSH_TEST_WORKSPACE_VALUE=second workspace"},
			want: "second workspace|kept||1",
		},
		{
			name:       "explicit background",
			env:        []string{"CRUSH_TEST_WORKSPACE_VALUE=background"},
			background: true,
			want:       "background|kept||1",
		},
		{
			name:      "automatic background",
			env:       []string{"CRUSH_TEST_WORKSPACE_VALUE=automatic"},
			autoAfter: 1,
			want:      "automatic|kept||1",
		},
		{
			name: "explicit empty override",
			env:  []string{"CRUSH_TEST_WORKSPACE_VALUE="},
			want: "|kept||1",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tool := newBashToolForTest(t.TempDir())
			ctx := context.WithValue(t.Context(), SessionIDContextKey, "workspace-env")
			if tt.env != nil {
				ctx = context.WithValue(ctx, EnvironmentContextKey, tt.env)
			}
			params := BashParams{
				Description:         "Read workspace environment",
				Command:             "printf '%s|%s|%s|%s' \"$CRUSH_TEST_WORKSPACE_VALUE\" \"$CRUSH_TEST_SERVER_ONLY\" \"$CRUSH_TEST_CLIENT_ONLY\" \"$CRUSH\"",
				RunInBackground:     tt.background,
				AutoBackgroundAfter: tt.autoAfter,
			}
			if tt.background || tt.autoAfter > 0 {
				params.Command = "sleep 3; " + params.Command
			}

			resp := runBashTool(t, tool, ctx, params)
			require.False(t, resp.IsError)
			var meta BashResponseMetadata
			require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
			require.Equal(t, tt.background || tt.autoAfter > 0, meta.Background)
			output := meta.Output
			if meta.ShellID != "" {
				manager := shell.GetBackgroundShellManager()
				job, ok := manager.Get(meta.ShellID)
				require.True(t, ok)
				t.Cleanup(func() { _ = manager.Kill(meta.ShellID) })
				waitCtx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
				defer cancel()
				require.True(t, job.WaitContext(waitCtx))
				stdout, stderr, done, err := job.GetOutput()
				require.True(t, done)
				require.NoError(t, err)
				require.Empty(t, stderr)
				output = stdout
			}
			require.Equal(t, tt.want, output)
			require.Equal(t, "server", os.Getenv("CRUSH_TEST_WORKSPACE_VALUE"))
			require.Empty(t, os.Getenv("CRUSH_TEST_CLIENT_ONLY"))
		})
	}
}
