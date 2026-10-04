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

func TestParseInfoRoundTripsAttributes(t *testing.T) {
	t.Parallel()

	want := Info{
		ReplacedMessages: 332, ReplacedTokens: 756_906,
		KeptMessages: 231, KeptTokens: 573_814,
		TranscriptPath: "/var/tmp/region.txt",
	}
	got, ok := ParseInfo("checkpoint prose\n\n" + want.Render())
	require.True(t, ok)
	require.Equal(t, want, got)
}

// TestParseInfoReadsPreAttributeFooters covers sessions already on disk from
// builds whose footer carried no attributes. The card would otherwise show no
// counts for them, which reads as a bug in the card.
func TestParseInfoReadsPreAttributeFooters(t *testing.T) {
	t.Parallel()

	legacy := "the summary\n\n<compaction_info>\n" +
		"This checkpoint replaces 12 earlier messages (~4000 tokens).\n" +
		"The 3 most recent messages (~900 tokens) were not summarized and follow verbatim.\n" +
		"Full text of the replaced region: /var/tmp/old.txt\n" +
		"When you need an exact quote, read that file instead of guessing at it.\n" +
		infoEnd

	got, ok := ParseInfo(legacy)
	require.True(t, ok)
	require.Equal(t, Info{
		ReplacedMessages: 12, ReplacedTokens: 4000,
		KeptMessages: 3, KeptTokens: 900,
		TranscriptPath: "/var/tmp/old.txt",
	}, got)
}

func TestParseInfoWithoutFooter(t *testing.T) {
	t.Parallel()

	_, ok := ParseInfo("an ordinary assistant reply")
	require.False(t, ok)
}

func TestBodyStripsAttributedFooter(t *testing.T) {
	t.Parallel()

	info := Info{ReplacedMessages: 4, ReplacedTokens: 40, KeptMessages: 2, KeptTokens: 20}
	summary := "kept prose\n\n" + info.Render()
	require.Equal(t, "kept prose", Body(summary))
	_, ok := ParseInfo(Body(summary))
	require.False(t, ok, "a merged checkpoint must not inherit bookkeeping")
}
