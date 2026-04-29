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

func TestTouchToolCreatesEmptyFile(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	tool := NewTouchTool(nil, &mockPermissionService{}, &mockHistoryService{}, mockFileTrackerService{}, workingDir)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

	resp := runTouchTool(t, tool, ctx, TouchParams{FilePath: "nested/empty.txt"})
	require.False(t, resp.IsError)

	filePath := filepath.Join(workingDir, "nested", "empty.txt")
	info, err := os.Stat(filePath)
	require.NoError(t, err)
	require.False(t, info.IsDir())
	require.Zero(t, info.Size())
}

func TestTouchToolRefusesExistingFile(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	filePath := filepath.Join(workingDir, "existing.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("content"), 0o644))

	tool := NewTouchTool(nil, &mockPermissionService{}, &mockHistoryService{}, mockFileTrackerService{}, workingDir)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

	resp := runTouchTool(t, tool, ctx, TouchParams{FilePath: "existing.txt"})
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "File already exists")

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	require.Equal(t, "content", string(content))
}

func TestWriteToolEmptyContentPointsToTouch(t *testing.T) {
	t.Parallel()

	tool := NewWriteTool(nil, nil, nil, nil, t.TempDir())

	input, err := json.Marshal(WriteParams{FilePath: "empty.txt"})
	require.NoError(t, err)

	resp, err := tool.Run(context.Background(), fantasy.ToolCall{
		ID:    "test-call",
		Name:  WriteToolName,
		Input: string(input),
	})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Equal(t, `content is required. use the "touch" tool to create an empty file`, resp.Content)
}

func runTouchTool(t *testing.T, tool fantasy.AgentTool, ctx context.Context, params TouchParams) fantasy.ToolResponse {
	t.Helper()

	input, err := json.Marshal(params)
	require.NoError(t, err)

	resp, err := tool.Run(ctx, fantasy.ToolCall{
		ID:    "test-call",
		Name:  TouchToolName,
		Input: string(input),
	})
	require.NoError(t, err)
	return resp
}
