package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/filetracker"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

func TestReadTextFileBoundaryCases(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "sample.txt")

	var allLines []string
	for i := range 5 {
		allLines = append(allLines, fmt.Sprintf("line %d", i+1))
	}
	require.NoError(t, os.WriteFile(filePath, []byte(strings.Join(allLines, "\n")), 0o644))

	tests := []struct {
		name        string
		offset      int
		limit       int
		wantContent string
		wantHasMore bool
	}{
		{
			name:        "exactly limit lines remaining",
			offset:      0,
			limit:       5,
			wantContent: "line 1\nline 2\nline 3\nline 4\nline 5",
			wantHasMore: false,
		},
		{
			name:        "limit plus one line remaining",
			offset:      0,
			limit:       4,
			wantContent: "line 1\nline 2\nline 3\nline 4",
			wantHasMore: true,
		},
		{
			name:        "offset at last line",
			offset:      4,
			limit:       3,
			wantContent: "line 5",
			wantHasMore: false,
		},
		{
			name:        "offset beyond eof",
			offset:      10,
			limit:       3,
			wantContent: "",
			wantHasMore: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotContent, gotHasMore, err := readTextFile(filePath, tt.offset, tt.limit)
			require.NoError(t, err)
			require.Equal(t, tt.wantContent, gotContent)
			require.Equal(t, tt.wantHasMore, gotHasMore)
		})
	}
}

func TestReadTextFileTruncatesLongLines(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "longline.txt")

	longLine := strings.Repeat("a", MaxLineLength+10)
	require.NoError(t, os.WriteFile(filePath, []byte(longLine), 0o644))

	content, hasMore, err := readTextFile(filePath, 0, 1)
	require.NoError(t, err)
	require.False(t, hasMore)
	require.Equal(t, strings.Repeat("a", MaxLineLength)+"...", content)
}

func TestViewToolAllowsSmallSectionsOfLargeFiles(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	filePath := filepath.Join(workingDir, "large.txt")
	lines := []string{strings.Repeat("a", MaxViewSize+1), "target line", "after target"}
	require.NoError(t, os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0o644))

	tool := newViewToolForTest(workingDir)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	resp := runViewTool(t, tool, ctx, ViewParams{
		FilePath: filePath,
		Offset:   1,
		Limit:    1,
	})

	require.False(t, resp.IsError)
	require.Contains(t, resp.Content, "     2|target line")
	require.NotContains(t, resp.Content, "File is too large")

	var meta ViewResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
	require.Equal(t, "target line", meta.Content)
}

func TestViewToolBlocksOversizedReturnedSections(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	filePath := filepath.Join(workingDir, "large-section.txt")
	lines := make([]string, DefaultReadLimit)
	for i := range lines {
		lines[i] = strings.Repeat("a", MaxLineLength)
	}
	require.NoError(t, os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0o644))

	tool := newViewToolForTest(workingDir)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	resp := runViewTool(t, tool, ctx, ViewParams{
		FilePath: filePath,
	})

	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "Content section is too large")
}

type mockViewPermissionService struct {
	*pubsub.Broker[permission.PermissionRequest]
}

func (m *mockViewPermissionService) Request(ctx context.Context, req permission.CreatePermissionRequest) (bool, error) {
	return true, nil
}

func (m *mockViewPermissionService) Grant(req permission.PermissionRequest) {}

func (m *mockViewPermissionService) Deny(req permission.PermissionRequest) {}

func (m *mockViewPermissionService) GrantPersistent(req permission.PermissionRequest) {}

func (m *mockViewPermissionService) AutoApproveSession(sessionID string) {}

func (m *mockViewPermissionService) SetSkipRequests(skip bool) {}

func (m *mockViewPermissionService) SkipRequests() bool {
	return false
}

func (m *mockViewPermissionService) SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[permission.PermissionNotification] {
	return make(<-chan pubsub.Event[permission.PermissionNotification])
}

type mockFileTracker struct{}

func (m mockFileTracker) RecordRead(ctx context.Context, sessionID, path string) {}

func (m mockFileTracker) LastReadTime(ctx context.Context, sessionID, path string) time.Time {
	return time.Time{}
}

func (m mockFileTracker) ListReadFiles(ctx context.Context, sessionID string) ([]string, error) {
	return nil, nil
}

func newViewToolForTest(workingDir string) fantasy.AgentTool {
	permissions := &mockViewPermissionService{Broker: pubsub.NewBroker[permission.PermissionRequest]()}
	return NewViewTool(nil, permissions, mockFileTracker{}, nil, workingDir)
}

func runViewTool(t *testing.T, tool fantasy.AgentTool, ctx context.Context, params ViewParams) fantasy.ToolResponse {
	t.Helper()

	input, err := json.Marshal(params)
	require.NoError(t, err)

	call := fantasy.ToolCall{
		ID:    "test-call",
		Name:  ViewToolName,
		Input: string(input),
	}

	resp, err := tool.Run(ctx, call)
	require.NoError(t, err)
	return resp
}

var _ filetracker.Service = mockFileTracker{}

func TestReadBuiltinFile(t *testing.T) {
	t.Parallel()

	t.Run("reads crush-config skill", func(t *testing.T) {
		t.Parallel()

		resp, err := readBuiltinFile(ViewParams{
			FilePath: "crush://skills/crush-config/SKILL.md",
		}, nil)
		require.NoError(t, err)
		require.NotEmpty(t, resp.Content)
		require.Contains(t, resp.Content, "Crush Configuration")
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		resp, err := readBuiltinFile(ViewParams{
			FilePath: "crush://skills/nonexistent/SKILL.md",
		}, nil)
		require.NoError(t, err)
		require.True(t, resp.IsError)
	})

	t.Run("metadata has skill info", func(t *testing.T) {
		t.Parallel()

		resp, err := readBuiltinFile(ViewParams{
			FilePath: "crush://skills/crush-config/SKILL.md",
		}, nil)
		require.NoError(t, err)

		var meta ViewResponseMetadata
		require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
		require.Equal(t, ViewResourceSkill, meta.ResourceType)
		require.Equal(t, "crush-config", meta.ResourceName)
		require.NotEmpty(t, meta.ResourceDescription)
	})

	t.Run("respects offset", func(t *testing.T) {
		t.Parallel()

		resp, err := readBuiltinFile(ViewParams{
			FilePath: "crush://skills/crush-config/SKILL.md",
			Offset:   5,
		}, nil)
		require.NoError(t, err)
		require.NotContains(t, resp.Content, "     1|")
	})
}
