package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

func TestEditToolRespectsCrushignore(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workingDir, ".crushignore"), []byte("_*.md\n"), 0o644))
	testFile := filepath.Join(workingDir, "_secret.md")
	require.NoError(t, os.WriteFile(testFile, []byte("line 1\n"), 0o644))

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	tool := NewEditTool(nil, &mockPermissionService{}, &mockHistoryService{}, mockFileTrackerService{}, workingDir)

	input, err := json.Marshal(EditParams{
		FilePath:  "_secret.md",
		OldString: "line 1",
		NewString: "LINE 1",
	})
	require.NoError(t, err)

	resp, err := tool.Run(ctx, fantasy.ToolCall{
		ID:    "test-call",
		Name:  EditToolName,
		Input: string(input),
	})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "ignored")

	content, err := os.ReadFile(testFile)
	require.NoError(t, err)
	require.Equal(t, "line 1\n", string(content), "file should not have been modified")
}
