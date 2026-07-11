package telegram

import (
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/charmbracelet/crush/internal/proto"
	"github.com/stretchr/testify/require"
)

func TestChunkEmpty(t *testing.T) {
	t.Parallel()
	require.Empty(t, Chunk("", 3900))
}

func TestChunkShort(t *testing.T) {
	t.Parallel()
	got := Chunk("hello", 3900)
	require.Equal(t, []string{"hello"}, got)
}

func TestChunkExactLimit(t *testing.T) {
	t.Parallel()
	s := strings.Repeat("a", 100)
	got := Chunk(s, 100)
	require.Equal(t, []string{s}, got)
}

func TestChunkPrefersNewlines(t *testing.T) {
	t.Parallel()
	// Build text with newlines so the split prefers them.
	var parts []string
	for range 20 {
		parts = append(parts, strings.Repeat("x", 50))
	}
	text := strings.Join(parts, "\n")
	chunks := Chunk(text, 200)
	require.Greater(t, len(chunks), 1)
	for _, c := range chunks {
		require.LessOrEqual(t, len(utf16.Encode([]rune(c))), 200)
	}
	require.Equal(t, text, strings.Join(chunks, ""))
}

func TestChunkHardSplit(t *testing.T) {
	t.Parallel()
	text := strings.Repeat("a", 500)
	chunks := Chunk(text, 100)
	require.Greater(t, len(chunks), 1)
	for _, c := range chunks {
		require.LessOrEqual(t, len(utf16.Encode([]rune(c))), 100)
	}
	require.Equal(t, text, strings.Join(chunks, ""))
}

func TestChunkEmojiSurrogates(t *testing.T) {
	t.Parallel()
	// Each emoji is typically 2 UTF-16 code units.
	text := strings.Repeat("😀", 100)
	limit := 50
	chunks := Chunk(text, limit)
	require.Greater(t, len(chunks), 1)
	for _, c := range chunks {
		require.LessOrEqual(t, len(utf16.Encode([]rune(c))), limit)
	}
	// Re-join should equal original (no prefixes in raw Chunk).
	require.Equal(t, text, strings.Join(chunks, ""))
}

func TestTruncateMiddle(t *testing.T) {
	t.Parallel()
	require.Equal(t, "short", TruncateMiddle("short", 100))
	require.Equal(t, "short", TruncateMiddle("short", 0))
	long := strings.Repeat("a", 100)
	got := TruncateMiddle(long, 30)
	require.Contains(t, got, "…(truncated)…")
	// head=20, tail=10 for max=30.
	require.True(t, strings.HasPrefix(got, strings.Repeat("a", 20)))
	require.True(t, strings.HasSuffix(got, strings.Repeat("a", 10)))
}

func TestPermissionSummaryBashEscapes(t *testing.T) {
	t.Parallel()
	s := PermissionSummary(proto.PermissionRequest{
		ToolName:    "bash",
		Description: "run <script>",
		Params: proto.BashPermissionsParams{
			Command:    "echo <foo> && rm -rf /",
			WorkingDir: "/tmp",
		},
	})
	require.Contains(t, s, "&lt;foo&gt;")
	require.Contains(t, s, "run &lt;script&gt;")
	require.Contains(t, s, "<pre>")
	require.Contains(t, s, "dir: /tmp")
}

func TestPermissionSummaryEdit(t *testing.T) {
	t.Parallel()
	s := PermissionSummary(proto.PermissionRequest{
		ToolName: "edit",
		Params: proto.EditPermissionsParams{
			FilePath:   "main.go",
			OldContent: "old",
			NewContent: "new",
		},
	})
	require.Contains(t, s, "main.go")
	require.Contains(t, s, "− old")
	require.Contains(t, s, "+ new")
}

func TestPermissionSummaryFallbackMap(t *testing.T) {
	t.Parallel()
	s := PermissionSummary(proto.PermissionRequest{
		ToolName: "custom",
		Params: map[string]any{
			"key": "value",
		},
	})
	require.Contains(t, s, "key")
	require.Contains(t, s, "value")
}

func TestPermissionSummaryFetch(t *testing.T) {
	t.Parallel()
	s := PermissionSummary(proto.PermissionRequest{
		ToolName: "fetch",
		Params: proto.FetchPermissionsParams{
			URL:    "https://example.com",
			Format: "markdown",
		},
	})
	require.Contains(t, s, "https://example.com")
	require.Contains(t, s, "markdown")
}

func TestFormatChunks(t *testing.T) {
	t.Parallel()
	require.Equal(t, []string{"only"}, formatChunks([]string{"only"}))
	got := formatChunks([]string{"a", "b", "c"})
	require.Equal(t, "a", got[0])
	require.Equal(t, "[2/3]\nb", got[1])
	require.Equal(t, "[3/3]\nc", got[2])
}
