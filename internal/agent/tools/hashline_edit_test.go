package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/filetracker"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

type hashlineEditFileTracker struct {
	lastRead map[string]time.Time
}

func newHashlineEditFileTracker() *hashlineEditFileTracker {
	return &hashlineEditFileTracker{lastRead: make(map[string]time.Time)}
}

func (m *hashlineEditFileTracker) RecordRead(_ context.Context, _ string, path string) {
	m.lastRead[path] = time.Now().Add(time.Second)
}

func (m *hashlineEditFileTracker) LastReadTime(_ context.Context, _ string, path string) time.Time {
	if value, ok := m.lastRead[path]; ok {
		return value
	}
	return time.Time{}
}

func (m *hashlineEditFileTracker) ListReadFiles(context.Context, string) ([]string, error) {
	return nil, nil
}

var _ filetracker.Service = (*hashlineEditFileTracker)(nil)

func runHashlineEditTool(t *testing.T, tool fantasy.AgentTool, ctx context.Context, params HashlineEditParams) (fantasy.ToolResponse, error) {
	t.Helper()

	input, err := json.Marshal(params)
	require.NoError(t, err)

	return tool.Run(ctx, fantasy.ToolCall{
		ID:    "test-call",
		Name:  HashlineEditToolName,
		Input: string(input),
	})
}

func newHashlineEditToolForTest(t *testing.T, workingDir string, tracker filetracker.Service) fantasy.AgentTool {
	t.Helper()

	permissions := &mockPermissionService{Broker: pubsub.NewBroker[permission.PermissionRequest]()}
	historySvc := &mockHistoryService{Broker: pubsub.NewBroker[history.File]()}
	return NewHashlineEditTool(nil, permissions, historySvc, tracker, workingDir)
}

func TestHashlineEditRejectsHashMismatch(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	filePath := filepath.Join(workingDir, "main.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("alpha\nbeta\ngamma\n"), 0o644))

	tracker := newHashlineEditFileTracker()
	tracker.lastRead[filePath] = time.Now().Add(time.Second)
	tool := newHashlineEditToolForTest(t, workingDir, tracker)

	lineRef := formatHashlineReference(2, "beta")
	lineRef = lineRef[:len(lineRef)-1] + mutateHashlineNibble(lineRef[len(lineRef)-1])

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	resp, err := runHashlineEditTool(t, tool, ctx, HashlineEditParams{
		FilePath: filePath,
		Operations: []HashlineEditOperation{
			{
				Operation: hashlineEditOpReplaceLine,
				Line:      lineRef,
				Content:   "BETA",
			},
		},
	})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "hash mismatch")

	data, readErr := os.ReadFile(filePath)
	require.NoError(t, readErr)
	require.Equal(t, "alpha\nbeta\ngamma\n", string(data))
}

func TestHashlineEditAppliesOperationsSuccessfully(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	filePath := filepath.Join(workingDir, "main.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("alpha\nbeta\ngamma\ndelta\n"), 0o644))

	tracker := newHashlineEditFileTracker()
	tracker.lastRead[filePath] = time.Now().Add(time.Second)
	tool := newHashlineEditToolForTest(t, workingDir, tracker)

	ref1 := formatHashlineReference(1, "alpha")
	ref2 := formatHashlineReference(2, "beta")
	ref3 := formatHashlineReference(3, "gamma")
	ref4 := formatHashlineReference(4, "delta")

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	resp, err := runHashlineEditTool(t, tool, ctx, HashlineEditParams{
		FilePath: filePath,
		Operations: []HashlineEditOperation{
			{
				Operation: hashlineEditOpPrepend,
				Line:      ref3,
				Content:   "before-gamma",
			},
			{
				Operation: hashlineEditOpAppend,
				Line:      ref3,
				Content:   "after-gamma",
			},
			{
				Operation: hashlineEditOpReplaceRange,
				Start:     ref1,
				End:       ref2,
				Content:   "HEADER",
			},
			{
				Operation: hashlineEditOpReplaceLine,
				Line:      ref4,
				Content:   "DELTA!",
			},
		},
	})
	require.NoError(t, err)
	require.False(t, resp.IsError)
	require.Contains(t, resp.Content, "Applied 4 hashline operation")

	data, readErr := os.ReadFile(filePath)
	require.NoError(t, readErr)
	require.Equal(t, "HEADER\nbefore-gamma\ngamma\nafter-gamma\nDELTA!\n", string(data))
}

func TestHashlineEditDetectsRemovedAnchoredLineInLaterOperation(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	filePath := filepath.Join(workingDir, "main.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("alpha\nbeta\ngamma\n"), 0o644))

	tracker := newHashlineEditFileTracker()
	tracker.lastRead[filePath] = time.Now().Add(time.Second)
	tool := newHashlineEditToolForTest(t, workingDir, tracker)

	ref2 := formatHashlineReference(2, "beta")

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	resp, err := runHashlineEditTool(t, tool, ctx, HashlineEditParams{
		FilePath: filePath,
		Operations: []HashlineEditOperation{
			{
				Operation: hashlineEditOpReplaceRange,
				Start:     ref2,
				End:       ref2,
				Content:   "",
			},
			{
				Operation: hashlineEditOpAppend,
				Line:      ref2,
				Content:   "extra",
			},
		},
	})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, strings.ToLower(resp.Content), "line 2 no longer exists")

	data, readErr := os.ReadFile(filePath)
	require.NoError(t, readErr)
	require.Equal(t, "alpha\nbeta\ngamma\n", string(data))
}

func mutateHashlineNibble(current byte) string {
	for i := 0; i < len(hashlineNibbleAlphabet); i++ {
		if hashlineNibbleAlphabet[i] != current {
			return string(hashlineNibbleAlphabet[i])
		}
	}
	return string(current)
}
