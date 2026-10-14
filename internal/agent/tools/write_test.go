package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
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

// A denied create must leave no trace: the parent directories are made only
// once the permission is granted.
func TestCreateToolsMakeParentDirsOnlyWhenGranted(t *testing.T) {
	t.Parallel()

	for _, allow := range []bool{false, true} {
		for _, name := range []string{WriteToolName, EditToolName, MultiEditToolName} {
			t.Run(fmt.Sprintf("%s/allow=%v", name, allow), func(t *testing.T) {
				dir := t.TempDir()
				file := filepath.Join(dir, "new", "sub", "f.txt")
				perms := &recordingPermissionService{Broker: pubsub.NewBroker[permission.PermissionRequest](), allow: allow}
				var (
					tool  fantasy.AgentTool
					input any
				)
				switch name {
				case WriteToolName:
					tool, input = NewWriteTool(nil, perms, &mockHistoryService{}, mockFileTrackerService{}, dir), WriteParams{FilePath: file, Content: "x"}
				case EditToolName:
					tool, input = NewEditTool(nil, perms, &mockHistoryService{}, mockFileTrackerService{}, dir), EditParams{FilePath: file, NewString: "x"}
				default:
					tool, input = NewMultiEditTool(nil, perms, &mockHistoryService{}, mockFileTrackerService{}, dir), MultiEditParams{FilePath: file, Edits: []MultiEditOperation{{NewString: "x"}}}
				}
				raw, err := json.Marshal(input)
				require.NoError(t, err)
				ctx := context.WithValue(t.Context(), SessionIDContextKey, "s")

				resp, err := tool.Run(ctx, fantasy.ToolCall{ID: name, Name: name, Input: string(raw)})

				require.NoError(t, err, name)
				require.Equal(t, 1, perms.requestCount, name)
				if !allow {
					require.Contains(t, resp.Content, "User denied permission", name)
					require.NoDirExists(t, filepath.Join(dir, "new"), "%s: denied create made the parent dir", name)
					return
				}
				require.False(t, resp.IsError, "%s: %s", name, resp.Content)
				got, err := os.ReadFile(file)
				require.NoError(t, err, name)
				require.Equal(t, "x", string(got), name)
			})
		}
	}
}
