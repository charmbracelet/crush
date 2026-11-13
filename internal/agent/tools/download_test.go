package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/stretchr/testify/require"
)

// A PreToolUse hook that returns "allow" pre-approves the permission prompt
// for its tool call. The download tool must tie its permission request to
// that call, or the approval is ignored and the request waits for a prompt
// nobody was supposed to see.
func TestDownloadTool_HookApprovalSkipsPrompt(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hello"))
	}))
	t.Cleanup(srv.Close)

	workingDir := t.TempDir()
	perms := permission.NewPermissionService(workingDir, false, nil)
	tool := NewDownloadTool(perms, workingDir, srv.Client())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	ctx = context.WithValue(ctx, SessionIDContextKey, "test-session")
	ctx = permission.WithHookApproval(ctx, "call-1")

	input, err := json.Marshal(DownloadParams{URL: srv.URL, FilePath: "out.txt"})
	require.NoError(t, err)

	resp, err := tool.Run(ctx, fantasy.ToolCall{
		ID:    "call-1",
		Name:  DownloadToolName,
		Input: string(input),
	})
	require.NoError(t, err)
	require.False(t, resp.IsError, resp.Content)

	got, err := os.ReadFile(filepath.Join(workingDir, "out.txt"))
	require.NoError(t, err)
	require.Equal(t, "hello", string(got))
}
