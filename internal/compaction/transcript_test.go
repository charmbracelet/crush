package compaction

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

func TestSerializeRendersStructure(t *testing.T) {
	t.Parallel()

	out := Serialize([]message.Message{
		userMsg("read the config"),
		{
			ID:    "a-1",
			Role:  message.Assistant,
			Model: "some-model",
			Parts: []message.ContentPart{
				message.TextContent{Text: "looking"},
				message.ToolCall{ID: "call-1", Name: "view", Input: `{"file_path":"x"}`},
			},
		},
		{
			ID:   "t-1",
			Role: message.Tool,
			Parts: []message.ContentPart{
				message.ToolResult{ToolCallID: "call-1", Name: "view", Content: "contents", IsError: true},
			},
		},
		{
			ID:    "b-1",
			Role:  message.User,
			Parts: []message.ContentPart{message.BinaryContent{Path: "img.png", MIMEType: "image/png", Data: make([]byte, 12)}},
		},
	})

	require.Contains(t, out, "--- [1] user ---\nread the config")
	require.Contains(t, out, "--- [2] assistant model=some-model ---")
	require.Contains(t, out, `<tool_call name="view" id="call-1">`)
	require.Contains(t, out, `<tool_result name="view" id="call-1" error>`)
	require.Contains(t, out, `<attachment path="img.png" mime="image/png" bytes=12 />`)
}

func TestSerializeCapsByDroppingOldest(t *testing.T) {
	t.Parallel()

	big := strings.Repeat("z", MaxTranscriptBytes/2+1)
	out := Serialize([]message.Message{
		userMsg(big),
		assistantMsg(big),
		userMsg("newest"),
	})

	require.Contains(t, out, "earlier messages omitted")
	require.Contains(t, out, "newest")
	require.Less(t, len(out), MaxTranscriptBytes+1024)
	require.NotContains(t, out, "--- [1] user ---\n"+big)
}

func TestWriteTranscriptRequiresADirectory(t *testing.T) {
	t.Parallel()

	path, err := WriteTranscript("", "session", "summary", []message.Message{userMsg("hi")})
	require.NoError(t, err)
	require.Empty(t, path)

	dir := t.TempDir()
	path, err = WriteTranscript(dir, "session", "summary", nil)
	require.NoError(t, err)
	require.Empty(t, path)
}

func TestWriteTranscriptWritesAndPrunes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	for i := range KeptTranscripts + 3 {
		path, err := WriteTranscript(dir, "sess-1", fmt.Sprintf("summary-%d", i), []message.Message{userMsg("hello")})
		require.NoError(t, err)
		require.FileExists(t, path)
		body, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Contains(t, string(body), "hello")
	}

	entries, err := os.ReadDir(filepath.Join(dir, "sess-1"))
	require.NoError(t, err)
	require.Len(t, entries, KeptTranscripts)
}

func TestWriteTranscriptIsolatesSessionsAndRefusesTraversal(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path, err := WriteTranscript(dir, "../../etc/passwd", "summary", []message.Message{userMsg("hi")})
	require.NoError(t, err)
	require.Equal(t, dir, filepath.Dir(filepath.Dir(path)), "session ids must not escape the data directory")

	// Separate sessions keep separate regions.
	other, err := WriteTranscript(dir, "sess-2", "summary", []message.Message{userMsg("hi")})
	require.NoError(t, err)
	require.NotEqual(t, filepath.Dir(path), filepath.Dir(other))
}

func TestSanitize(t *testing.T) {
	t.Parallel()

	require.Equal(t, "session", sanitize(""))
	require.Equal(t, "abc_123_DEF.ghi", sanitize("abc 123/DEF.ghi"))
	require.Equal(t, "__", sanitize(".."), "a dot-only component would escape the data directory")
	require.Len(t, sanitize(strings.Repeat("x", 200)), 64)
}
