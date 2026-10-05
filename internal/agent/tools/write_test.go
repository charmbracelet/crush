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

func TestWriteToolEmptyFilePath(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

	tool := NewWriteTool(nil, &mockPermissionService{}, &mockHistoryService{}, mockFileTrackerService{}, workingDir)

	t.Run("empty_file_path", func(t *testing.T) {
		t.Parallel()

		input, err := json.Marshal(WriteParams{FilePath: "", Content: "hello"})
		require.NoError(t, err)

		resp, err := tool.Run(ctx, fantasy.ToolCall{
			ID:    "test-call",
			Name:  WriteToolName,
			Input: string(input),
		})
		require.NoError(t, err)
		require.True(t, resp.IsError)
		require.Contains(t, resp.Content, "file_path is required")
	})

	t.Run("missing_file_path_from_empty_json", func(t *testing.T) {
		t.Parallel()

		resp, err := tool.Run(ctx, fantasy.ToolCall{
			ID:    "test-call",
			Name:  WriteToolName,
			Input: `{}`,
		})
		require.NoError(t, err)
		require.True(t, resp.IsError)
		require.Contains(t, resp.Content, "file_path is required")
	})

	t.Run("invalid_json_input", func(t *testing.T) {
		t.Parallel()

		resp, err := tool.Run(ctx, fantasy.ToolCall{
			ID:    "test-call",
			Name:  WriteToolName,
			Input: `not json`,
		})
		require.NoError(t, err)
		require.True(t, resp.IsError)
		require.Contains(t, resp.Content, "invalid parameters")
	})
}
