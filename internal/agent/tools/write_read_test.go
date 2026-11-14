package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/stretchr/testify/require"
)

// recordingPermission captures the params the tool asks the user to approve, so a test
// can assert on what the preview would have shown.
type recordingPermission struct {
	*mockPermissionService
	requests []permission.CreatePermissionRequest
}

func (r *recordingPermission) Request(_ context.Context, req permission.CreatePermissionRequest) (bool, error) {
	r.requests = append(r.requests, req)
	return true, nil
}

func (r *recordingPermission) writeParams() []WritePermissionsParams {
	var out []WritePermissionsParams
	for _, req := range r.requests {
		if wp, ok := req.Params.(WritePermissionsParams); ok {
			out = append(out, wp)
		}
	}
	return out
}

func writeOnlyFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses permission bits")
	}
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o200))
	return path
}

func TestWriteToolRefusesToWriteAFileItCannotRead(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	writeOnlyFile(t, workingDir, "secret.txt", "original body line\nsecond line\n")

	perm := &recordingPermission{mockPermissionService: &mockPermissionService{}}
	tool := NewWriteTool(nil, perm, &mockHistoryService{}, mockFileTrackerService{}, workingDir)

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	input, err := json.Marshal(WriteParams{FilePath: "secret.txt", Content: "new body"})
	require.NoError(t, err)

	_, runErr := tool.Run(ctx, fantasy.ToolCall{ID: "c", Name: WriteToolName, Input: string(input)})

	// Silently continuing here would show the user a whole-file "add" in the preview
	// and store an empty previous version, so the write has to fail instead.
	require.Error(t, runErr)
	require.ErrorContains(t, runErr, "secret.txt")
	require.Empty(t, perm.writeParams(), "no approval should be requested for a write we cannot describe")
}

func TestWriteToolPreviewCarriesTheRealPreviousContent(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	target := filepath.Join(workingDir, "readable.txt")
	require.NoError(t, os.WriteFile(target, []byte("first version\n"), 0o644))

	perm := &recordingPermission{mockPermissionService: &mockPermissionService{}}
	tool := NewWriteTool(nil, perm, &mockHistoryService{}, mockFileTrackerService{}, workingDir)

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	input, err := json.Marshal(WriteParams{FilePath: "readable.txt", Content: "second version\n"})
	require.NoError(t, err)

	_, runErr := tool.Run(ctx, fantasy.ToolCall{ID: "c", Name: WriteToolName, Input: string(input)})
	require.NoError(t, runErr)

	params := perm.writeParams()
	require.Len(t, params, 1)
	require.Equal(t, "first version\n", params[0].OldContent,
		"the approval preview must show what is actually being replaced")
	require.Equal(t, "second version\n", params[0].NewContent)

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, "second version\n", string(got))
}

func TestWriteToolStillCreatesANewFile(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()

	perm := &recordingPermission{mockPermissionService: &mockPermissionService{}}
	tool := NewWriteTool(nil, perm, &mockHistoryService{}, mockFileTrackerService{}, workingDir)

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	input, err := json.Marshal(WriteParams{FilePath: "fresh.txt", Content: "brand new\n"})
	require.NoError(t, err)

	resp, runErr := tool.Run(ctx, fantasy.ToolCall{ID: "c", Name: WriteToolName, Input: string(input)})
	require.NoError(t, runErr)
	require.False(t, resp.IsError)

	got, err := os.ReadFile(filepath.Join(workingDir, "fresh.txt"))
	require.NoError(t, err)
	require.Equal(t, "brand new\n", string(got))

	// A file that did not exist has no previous content, which is legitimately empty.
	params := perm.writeParams()
	require.Len(t, params, 1)
	require.Empty(t, params[0].OldContent)
}
