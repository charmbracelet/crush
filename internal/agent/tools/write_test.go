package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

type mockFileTrackerService struct{}

func (m mockFileTrackerService) RecordRead(ctx context.Context, sessionID, path string) {}

func (m mockFileTrackerService) LastReadTime(ctx context.Context, sessionID, path string) time.Time {
	return time.Now()
}

func (m mockFileTrackerService) ListReadFiles(ctx context.Context, sessionID string) ([]string, error) {
	return nil, nil
}

func TestWriteToolWritesEmptyNewFile(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

	tool := NewWriteTool(nil, &mockPermissionService{}, &mockHistoryService{}, mockFileTrackerService{}, workingDir)

	input, err := json.Marshal(WriteParams{FilePath: "empty.txt", Content: ""})
	require.NoError(t, err)

	resp, err := tool.Run(ctx, fantasy.ToolCall{
		ID:    "test-call",
		Name:  WriteToolName,
		Input: string(input),
	})
	require.NoError(t, err)
	require.False(t, resp.IsError)

	b, err := os.ReadFile(filepath.Join(workingDir, "empty.txt"))
	require.NoError(t, err)
	require.Equal(t, "", string(b))
}

func TestWriteToolRespectsCrushignore(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	product := filepath.Join(workingDir, "product")
	require.NoError(t, os.MkdirAll(product, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(workingDir, ".crushignore"), []byte("**/_*.md\n"), 0o644))

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	tool := NewWriteTool(nil, &mockPermissionService{}, &mockHistoryService{}, mockFileTrackerService{}, workingDir)

	input, err := json.Marshal(WriteParams{
		FilePath: filepath.Join("product", "_draft.md"),
		Content:  "secret",
	})
	require.NoError(t, err)

	resp, err := tool.Run(ctx, fantasy.ToolCall{
		ID:    "test-call",
		Name:  WriteToolName,
		Input: string(input),
	})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, ".crushignore")

	_, err = os.Stat(filepath.Join(product, "_draft.md"))
	require.True(t, os.IsNotExist(err))
}
