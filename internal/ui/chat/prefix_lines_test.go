package chat

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// prefixLines matches splitting on newlines, prefixing every segment and
// joining again, empty and trailing segments included.
func TestPrefixLinesMatchesSplitJoin(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"", "a", "a\nb", "\n", "a\n", "\na", "a\n\n\nb\n", "\x1b[31mred\x1b[0m\n  x"} {
		lines := strings.Split(s, "\n")
		for i, ln := range lines {
			lines[i] = "│ " + ln
		}
		require.Equal(t, strings.Join(lines, "\n"), prefixLines(s, "│ "), "input %q", s)
	}
}

// The memoized content hash follows the content as it changes.
func TestAssistantContentHashFollowsContent(t *testing.T) {
	t.Parallel()
	a := &AssistantMessageItem{}
	first := a.contentHashOf("hello")
	require.Equal(t, fnv64("hello"), first)
	require.Equal(t, first, a.contentHashOf("hello"))
	require.Equal(t, fnv64("hello world"), a.contentHashOf("hello world"))
	require.Equal(t, fnv64(""), a.contentHashOf(""))
	require.Equal(t, first, a.contentHashOf(strings.Clone("hello")))
}
