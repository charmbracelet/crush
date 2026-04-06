package tools

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"charm.land/fantasy"
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
		{
			name:        "offset at eof",
			offset:      5,
			limit:       3,
			wantContent: "",
			wantHasMore: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotContent, gotHasMore, err := readTextFile(filePath, tt.offset, tt.limit)
			if tt.offset > len(allLines) {
				require.ErrorIs(t, err, errViewOffsetBeyondEOF)
				require.Equal(t, "", gotContent)
				require.False(t, gotHasMore)
				return
			}
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

func TestViewTool_AllowsLargeImageWithCompression(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "large.png")
	largePNG := createLargePNG(t)
	require.Greater(t, len(largePNG), MaxViewSize)
	require.NoError(t, os.WriteFile(filePath, largePNG, 0o644))

	permissions := &mockPermissionService{Broker: pubsub.NewBroker[permission.PermissionRequest]()}
	tool := NewViewTool(nil, permissions, &mockFileTracker{}, tmpDir)

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	ctx = context.WithValue(ctx, SupportsImagesContextKey, true)
	ctx = context.WithValue(ctx, ModelNameContextKey, "test-model")

	resp, err := runViewTool(t, tool, ctx, ViewParams{FilePath: "large.png"})
	require.NoError(t, err)
	require.False(t, resp.IsError, resp.Content)
	require.NotContains(t, resp.Content, "File is too large")
}

func TestViewTool_RejectsLargeNonImageFiles(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "large.txt")
	largeText := bytes.Repeat([]byte("a"), MaxViewSize+1)
	require.NoError(t, os.WriteFile(filePath, largeText, 0o644))

	permissions := &mockPermissionService{Broker: pubsub.NewBroker[permission.PermissionRequest]()}
	tool := NewViewTool(nil, permissions, &mockFileTracker{}, tmpDir)

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

	resp, err := runViewTool(t, tool, ctx, ViewParams{FilePath: "large.txt"})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "File is too large")
}

func TestViewTool_InvalidPathSyntaxReturnsToolErrorResponse(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "windows" {
		t.Skip("windows-specific invalid path syntax behavior")
	}

	tmpDir := t.TempDir()
	permissions := &mockPermissionService{Broker: pubsub.NewBroker[permission.PermissionRequest]()}
	tool := NewViewTool(nil, permissions, &mockFileTracker{}, tmpDir)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

	resp, err := runViewTool(t, tool, ctx, ViewParams{FilePath: "*.go"})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "error accessing file")
}

func runViewTool(t *testing.T, tool fantasy.AgentTool, ctx context.Context, params ViewParams) (fantasy.ToolResponse, error) {
	t.Helper()

	input, err := json.Marshal(params)
	require.NoError(t, err)

	return tool.Run(ctx, fantasy.ToolCall{
		ID:    "test-call",
		Name:  ViewToolName,
		Input: string(input),
	})
}

func createLargePNG(t *testing.T) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 1024, 1024))
	_, err := rand.Read(img.Pix)
	require.NoError(t, err)

	for i := 3; i < len(img.Pix); i += 4 {
		img.Pix[i] = 255
	}

	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))

	return buf.Bytes()
}
