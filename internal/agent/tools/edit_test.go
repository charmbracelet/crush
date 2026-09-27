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

type mockEditFileTracker struct {
	lastRead time.Time
	reads    []string
}

func (m *mockEditFileTracker) RecordRead(ctx context.Context, sessionID, path string) {
	m.reads = append(m.reads, path)
}

func (m *mockEditFileTracker) LastReadTime(ctx context.Context, sessionID, path string) time.Time {
	return m.lastRead
}

func (m *mockEditFileTracker) ListReadFiles(ctx context.Context, sessionID string) ([]string, error) {
	return m.reads, nil
}

func TestReplaceContentPreservesCRLFAndMetadata(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("alpha\r\nbeta\r\n"), 0o644))

	tracker := &mockEditFileTracker{lastRead: time.Now().Add(time.Second)}
	edit := editContext{
		ctx:         context.WithValue(t.Context(), SessionIDContextKey, "session"),
		permissions: &mockPermissionService{},
		files:       &mockHistoryService{},
		filetracker: tracker,
		workingDir:  dir,
	}

	resp, err := replaceContent(edit, filePath, "beta", "BETA", false, fantasy.ToolCall{ID: "call"})
	require.NoError(t, err)
	require.False(t, resp.IsError)
	require.Equal(t, "Content replaced in file: "+filePath, resp.Content)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	require.Equal(t, "alpha\r\nBETA\r\n", string(content))
	require.Equal(t, []string{filePath}, tracker.reads)

	var meta EditResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
	require.Equal(t, "alpha\nbeta\n", meta.OldContent)
	require.Equal(t, "alpha\r\nBETA\r\n", meta.NewContent)
}

// A file is read as LF so that old_string matches whichever convention the file
// uses, but the bytes on disk are what the file already had. Converting the
// whole result back to CRLF when the file held a single CRLF rewrote every line
// ending in it, so the bytes around the edit have to be asserted, not just the
// line that changed.
func TestReplaceContentKeepsOtherLineEndingsIntact(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		old     string
		new     string
		all     bool
		want    string
	}{
		{
			name:    "one CRLF in an LF file",
			content: "a\nb\r\nc\n",
			old:     "b",
			new:     "B",
			want:    "a\nB\r\nc\n",
		},
		{
			name:    "one LF in a CRLF file",
			content: "a\r\nb\nc\r\n",
			old:     "b",
			new:     "B",
			want:    "a\r\nB\nc\r\n",
		},
		{
			// The added line lands next to the CRLF line, so it joins it.
			name:    "added line follows the line it lands on",
			content: "a\nb\r\nc\n",
			old:     "b",
			new:     "b\nB",
			want:    "a\nb\r\nB\r\nc\n",
		},
		{
			// Both new lines land in the region the CRLF line used to occupy.
			name:    "new_string with a newline keeps the region ending",
			content: "a\nb\r\nc\n",
			old:     "b",
			new:     "B\nB2",
			want:    "a\nB\r\nB2\r\nc\n",
		},
		{
			// The two matches sit in regions with different endings, and each
			// has to come back the way it was.
			name:    "replace_all keeps each match's own ending",
			content: "x\r\nb\ny\nb\r\nz\n",
			old:     "b",
			new:     "B",
			all:     true,
			want:    "x\r\nB\ny\nB\r\nz\n",
		},
		{
			// old_string does not match byte-for-byte, so the edit goes through
			// the whitespace-normalized fallback, which lands on a line range
			// rather than a byte range. The reconstruction only sees the two
			// forms, so the ending still has to be the region's.
			name:    "a whitespace-normalized match keeps the region ending",
			content: "a\n\tb\r\nc\n",
			old:     "    b",
			new:     "    B",
			want:    "a\n\tB\r\nc\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			filePath := filepath.Join(dir, "test.txt")
			require.NoError(t, os.WriteFile(filePath, []byte(tt.content), 0o644))

			edit := editContext{
				ctx:         context.WithValue(t.Context(), SessionIDContextKey, "session"),
				permissions: &mockPermissionService{},
				files:       &mockHistoryService{},
				filetracker: &mockEditFileTracker{lastRead: time.Now().Add(time.Second)},
				workingDir:  dir,
			}

			resp, err := replaceContent(edit, filePath, tt.old, tt.new, tt.all, fantasy.ToolCall{ID: "call"})
			require.NoError(t, err)
			require.False(t, resp.IsError, resp.Content)

			content, err := os.ReadFile(filePath)
			require.NoError(t, err)
			require.Equal(t, tt.want, string(content), "every byte outside the edit has to survive verbatim")
		})
	}
}

// Deleting a line drops it from the file and takes its line ending with it; the
// endings of the lines that are left are not up for negotiation.
func TestDeleteContentKeepsOtherLineEndingsIntact(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("a\nb\r\nc\n"), 0o644))

	edit := editContext{
		ctx:         context.WithValue(t.Context(), SessionIDContextKey, "session"),
		permissions: &mockPermissionService{},
		files:       &mockHistoryService{},
		filetracker: &mockEditFileTracker{lastRead: time.Now().Add(time.Second)},
		workingDir:  dir,
	}

	resp, err := deleteContent(edit, filePath, "b\n", false, fantasy.ToolCall{ID: "call"})
	require.NoError(t, err)
	require.False(t, resp.IsError, resp.Content)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	require.Equal(t, "a\nc\n", string(content))
}

func TestDeleteContentRejectsMultipleMatchesWithoutReplaceAll(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("alpha\nbeta\nalpha\n"), 0o644))

	edit := editContext{
		ctx:         context.WithValue(t.Context(), SessionIDContextKey, "session"),
		permissions: &mockPermissionService{},
		files:       &mockHistoryService{},
		filetracker: &mockEditFileTracker{lastRead: time.Now().Add(time.Second)},
		workingDir:  dir,
	}

	resp, err := deleteContent(edit, filePath, "alpha\n", false, fantasy.ToolCall{ID: "call"})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "old_string appears multiple times")

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	require.Equal(t, "alpha\nbeta\nalpha\n", string(content))
}
