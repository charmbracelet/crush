package tools

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOutputCache_StoreAndGet(t *testing.T) {
	t.Parallel()

	cache := &OutputCache{
		entries: make(map[string]*outputEntry),
	}

	sessionID := "test-session"
	toolCallID := "tool-123"
	content := "line1\nline2\nline3\nline4\nline5"

	cache.Store(sessionID, toolCallID, content)

	got, ok := cache.Get(sessionID, toolCallID)
	require.True(t, ok)
	require.Equal(t, content, got)

	// Non-existent entry
	_, ok = cache.Get("other-session", toolCallID)
	require.False(t, ok)
}

func TestOutputCache_Head(t *testing.T) {
	t.Parallel()

	cache := &OutputCache{
		entries: make(map[string]*outputEntry),
	}

	sessionID := "test-session"
	toolCallID := "tool-123"
	content := "line1\nline2\nline3\nline4\nline5"

	cache.Store(sessionID, toolCallID, content)

	// Get first 2 lines
	result, total, hasMore, ok := cache.Head(sessionID, toolCallID, 2, 0)
	require.True(t, ok)
	require.Equal(t, 5, total)
	require.True(t, hasMore)
	require.Equal(t, "line1\nline2", result)

	// Get with offset
	result, _, hasMore, ok = cache.Head(sessionID, toolCallID, 2, 2)
	require.True(t, ok)
	require.True(t, hasMore)
	require.Equal(t, "line3\nline4", result)

	// Get all
	result, _, hasMore, ok = cache.Head(sessionID, toolCallID, 10, 0)
	require.True(t, ok)
	require.False(t, hasMore)
	require.Equal(t, content, result)
}

func TestOutputCache_Tail(t *testing.T) {
	t.Parallel()

	cache := &OutputCache{
		entries: make(map[string]*outputEntry),
	}

	sessionID := "test-session"
	toolCallID := "tool-123"
	content := "line1\nline2\nline3\nline4\nline5"

	cache.Store(sessionID, toolCallID, content)

	// Get last 2 lines
	result, total, hasMore, ok := cache.Tail(sessionID, toolCallID, 2, 0)
	require.True(t, ok)
	require.Equal(t, 5, total)
	require.True(t, hasMore)
	require.Equal(t, "line4\nline5", result)

	// Get with offset from end
	result, _, hasMore, ok = cache.Tail(sessionID, toolCallID, 2, 2)
	require.True(t, ok)
	require.True(t, hasMore)
	require.Equal(t, "line2\nline3", result)

	// Get all
	result, _, hasMore, ok = cache.Tail(sessionID, toolCallID, 10, 0)
	require.True(t, ok)
	require.False(t, hasMore)
	require.Equal(t, content, result)
}

func TestOutputCache_Grep(t *testing.T) {
	t.Parallel()

	cache := &OutputCache{
		entries: make(map[string]*outputEntry),
	}

	sessionID := "test-session"
	toolCallID := "tool-123"
	content := "error: something failed\ninfo: all good\nwarn: check this\nerror: another failure"

	cache.Store(sessionID, toolCallID, content)

	// Search for errors
	matches, total, ok, err := cache.Grep(sessionID, toolCallID, "error", 0)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 4, total)
	require.Len(t, matches, 2)
	require.Equal(t, 1, matches[0].LineNum)
	require.Equal(t, 4, matches[1].LineNum)

	// No matches
	matches, _, ok, err = cache.Grep(sessionID, toolCallID, "fatal", 0)
	require.NoError(t, err)
	require.True(t, ok)
	require.Empty(t, matches)

	// Invalid regex
	_, _, ok, err = cache.Grep(sessionID, toolCallID, "[invalid", 0)
	require.Error(t, err)
	require.True(t, ok)
}

func TestTailOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		n         int
		expected  string
		truncated bool
	}{
		{
			name:      "empty",
			input:     "",
			n:         5,
			expected:  "",
			truncated: false,
		},
		{
			name:      "fewer lines than requested",
			input:     "line1\nline2\nline3",
			n:         5,
			expected:  "line1\nline2\nline3",
			truncated: false,
		},
		{
			name:      "exact lines",
			input:     "line1\nline2\nline3",
			n:         3,
			expected:  "line1\nline2\nline3",
			truncated: false,
		},
		{
			name:      "truncated",
			input:     "line1\nline2\nline3\nline4\nline5",
			n:         2,
			expected:  "line4\nline5",
			truncated: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, truncated := tailOutput(tt.input, tt.n)
			require.Equal(t, tt.expected, result)
			require.Equal(t, tt.truncated, truncated)
		})
	}
}

func TestTailOutput_LargeOutput(t *testing.T) {
	t.Parallel()

	// Generate a large output
	var lines []string
	for i := 0; i < 500; i++ {
		lines = append(lines, "line "+string(rune('0'+i%10)))
	}
	content := strings.Join(lines, "\n")

	result, truncated := tailOutput(content, DefaultOutputLines)
	require.True(t, truncated)

	resultLines := strings.Split(result, "\n")
	require.Len(t, resultLines, DefaultOutputLines)
}
