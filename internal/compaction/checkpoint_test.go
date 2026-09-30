package compaction

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInfoRender(t *testing.T) {
	t.Parallel()

	text := Info{
		ReplacedMessages: 42,
		ReplacedTokens:   128_000,
		KeptMessages:     7,
		KeptTokens:       18_000,
		TranscriptPath:   "/home/u/.crush/compaction/abc/1700-summary.txt",
	}.Render()

	require.True(t, strings.HasPrefix(text, infoStart))
	require.True(t, strings.HasSuffix(text, infoEnd))
	require.Contains(t, text, "42 earlier messages")
	require.Contains(t, text, "7 most recent messages")
	require.Contains(t, text, "/home/u/.crush/compaction/abc/1700-summary.txt")
	require.Contains(t, text, "read that file instead of guessing")
}

func TestInfoRenderWithoutRetainedTailOrPointer(t *testing.T) {
	t.Parallel()

	text := Info{ReplacedMessages: 3, ReplacedTokens: 900}.Render()
	require.NotContains(t, text, "most recent messages")
	require.NotContains(t, text, "Full text")
}

func TestBodyRoundTripsTheFooter(t *testing.T) {
	t.Parallel()

	footer := Info{ReplacedMessages: 1, TranscriptPath: "/tmp/x.txt"}.Render()
	require.Equal(t, "the checkpoint prose", Body("the checkpoint prose\n\n"+footer))
	require.Equal(t, "no footer here", Body("no footer here"))
	unclosed := "unclosed" + infoStart
	require.Equal(t, unclosed, Body(unclosed))
}
