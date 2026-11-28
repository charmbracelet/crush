package chat

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// lastNonBlankLineForward is the forward scan lastNonBlankLine replaced.
func lastNonBlankLineForward(s string) string {
	last := ""
	for line := range splitLines(s) {
		if strings.TrimSpace(line) != "" {
			last = line
		}
	}
	return last
}

// The backward scan finds the same line as a forward scan over every
// line.
func TestLastNonBlankLineMatchesForwardScan(t *testing.T) {
	t.Parallel()
	cases := []string{
		"", "\n", "\n\n", " \t\n", "a", "a\n", "a\n\n", "\na", "a\nb",
		"a\n  \n", "a\r\n\r\n", "  x  \n\n\t\n", "a\nb\n \n\v\n",
		"- item\n    continued\n\n",
	}
	r := rand.New(rand.NewPCG(1, 2))
	pieces := []string{"a", "b c", " ", "\t", "\n", "\r", "\n\n", "- x", "    y", " "}
	for range 500 {
		var b strings.Builder
		for range r.IntN(12) {
			b.WriteString(pieces[r.IntN(len(pieces))])
		}
		cases = append(cases, b.String())
	}
	for _, s := range cases {
		require.Equal(t, lastNonBlankLineForward(s), lastNonBlankLine(s), "input %q", s)
	}
}

// The case-insensitive tag check matches lowercasing ASCII lines first.
// Lowercasing also folds a few non-ASCII letters into ASCII ones, as in
// "<scrİpt", but any line starting with '<' and an ASCII letter is an HTML
// block opener anyway, so the opener check still agrees.
func TestHasPrefixFoldASCIIMatchesToLower(t *testing.T) {
	t.Parallel()
	for _, s := range []string{
		"", "<", "<pre", "<PRE>", "<Pre x", "<pr", "<prex", "<SCRIPT",
		"<ſcript", "<style>", "<TextArea\t", "<texta", "pre",
	} {
		for _, prefix := range []string{"<script", "<pre", "<style", "<textarea"} {
			require.Equal(t, strings.HasPrefix(strings.ToLower(s), prefix),
				hasPrefixFoldASCII(s, prefix), "input %q prefix %q", s, prefix)
		}
	}
	require.True(t, isHTMLBlockOpener("<scrİpt"))
	require.True(t, isHTMLBlockOpener("<PRE>"))
	require.False(t, isHTMLBlockOpener("<3 pre"))
}
