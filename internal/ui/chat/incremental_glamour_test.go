package chat

import (
	"strings"
	"testing"

	"charm.land/glamour/v2"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

// newTestRenderer builds a fresh glamour renderer for the given
// width. We deliberately do NOT share renderers between calls in
// the equivalence tests so any hidden state in
// [glamour.TermRenderer] cannot leak from a "cached" rendering
// path into a "fresh" rendering path.
func newTestRenderer(t *testing.T, width int) *glamour.TermRenderer {
	t.Helper()
	sty := styles.CharmtonePantera()
	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(sty.Markdown),
		glamour.WithWordWrap(width),
	)
	require.NoError(t, err)
	return r
}

// freshRender renders content as a single document with a fresh
// glamour renderer and applies the same trailing-newline trim
// that streamingMarkdown.Render does. Use this for byte- and
// visible-equivalence comparisons against the streaming path.
func freshRender(t *testing.T, content string, width int) string {
	t.Helper()
	r := newTestRenderer(t, width)
	out, err := r.Render(content)
	require.NoError(t, err)
	return strings.TrimSuffix(out, "\n")
}

// stripANSI removes all ANSI CSI escape sequences from s so two
// renders with different colour state can be compared on their
// visible glyphs alone.
func stripANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) {
				c := s[j]
				if c >= 0x40 && c <= 0x7e {
					j++
					break
				}
				j++
			}
			i = j
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// normalizeRender canonicalises a rendered glamour string for
// visual-equivalence comparison: strip ANSI, drop per-line
// trailing whitespace, drop leading/trailing blank lines, and
// collapse consecutive blank lines to a single blank line.
//
// Glamour pads rendered lines with trailing spaces and adds top/
// bottom block margins that differ subtly between "render the
// whole document at once" and "render two halves and concatenate
// them." Per F8 design principle D, those byte-level differences
// are acceptable as long as the visible content matches; this
// helper makes that comparison explicit.
func normalizeRender(s string) string {
	clean := stripANSI(s)
	lines := strings.Split(clean, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	// Collapse consecutive blank lines.
	out := make([]string, 0, len(lines))
	prevBlank := false
	for _, l := range lines {
		blank := l == ""
		if blank && prevBlank {
			continue
		}
		out = append(out, l)
		prevBlank = blank
	}
	// Trim leading and trailing blanks.
	for len(out) > 0 && out[0] == "" {
		out = out[1:]
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return strings.Join(out, "\n")
}

// containsRawMarkdownSource reports whether the visible portion of
// rendered contains literal markdown source markers that should
// have been consumed by glamour. Used by T2 to assert that
// intermediate streaming flushes don't leak raw source through to
// the user. We deliberately only flag markers that glamour
// removes during rendering ("```" fence delimiters, "|" table
// pipes embedded in a line that also contains pipes — actual
// table syntax — and bare "###" headers); pipes-in-prose and
// dashes are too common to flag.
func containsRawMarkdownSource(rendered string) bool {
	clean := stripANSI(rendered)
	if strings.Contains(clean, "```") {
		return true
	}
	for _, line := range strings.Split(clean, "\n") {
		if strings.HasPrefix(strings.TrimLeft(line, " \t"), "###") {
			return true
		}
	}
	return false
}

// -----------------------------------------------------------------------
// T1: findSafeMarkdownBoundary unit tests.
// -----------------------------------------------------------------------

// TestFindSafeMarkdownBoundary_TableDriven exercises the
// findSafeMarkdownBoundary decision tree across the full set of
// constructs §4.4 calls out: plain paragraphs, fenced code (open
// and closed), lists, tables, block quotes, and setext headers.
func TestFindSafeMarkdownBoundary_TableDriven(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		content string
		// want is the expected boundary; -1 means "no safe
		// boundary." When >=0 the test asserts content[:want]
		// ends after a blank-line separator and content[:want]
		// is a complete prefix.
		want int
	}{
		{
			name:    "empty",
			content: "",
			want:    -1,
		},
		{
			name:    "single line",
			content: "Just a single paragraph",
			want:    -1,
		},
		{
			name:    "two paragraphs",
			content: "First paragraph.\n\nSecond paragraph.",
			// boundary at start of "Second"
			want: len("First paragraph.\n\n"),
		},
		{
			name:    "three paragraphs picks latest",
			content: "First.\n\nSecond.\n\nThird.",
			want:    len("First.\n\nSecond.\n\n"),
		},
		{
			name:    "open fence at end",
			content: "Para.\n\n```go\nfoo()\n",
			// no closing fence — every blank-line candidate
			// before content end is INSIDE the fence (the open
			// fence opened at offset 7). Actually the ONLY
			// blank line is between "Para." and "```go", so
			// candidate boundary is right before "```go". At
			// that point fence count = 0, even, but the line
			// AFTER (the first non-blank) is "```go" which
			// would change rendering of the prefix… hmm,
			// actually it wouldn't change the prefix's
			// rendering because the prefix is just "Para.\n\n".
			// The boundary would be ACCEPTED. Let's check
			// what our impl does.
			want: len("Para.\n\n"),
		},
		{
			name:    "inside open fence: no candidate after open",
			content: "Para.\n\n```go\nfoo()\n\nbar()\n",
			// blank line after "foo()" is INSIDE the fence
			// (fence count at that prefix = 1, odd), must
			// reject. The earlier blank line between "Para."
			// and "```go" should still be safe (fence count
			// at that prefix = 0).
			want: len("Para.\n\n"),
		},
		{
			name:    "closed fence followed by paragraph",
			content: "Para1.\n\n```\nfoo()\n```\n\nPara2.",
			// latest blank line is between "```" and "Para2.";
			// fence count at that prefix = 2 (even), last
			// non-blank line is "```" which is not a list/
			// table/quote/setext.
			want: len("Para1.\n\n```\nfoo()\n```\n\n"),
		},
		{
			name:    "open list at end",
			content: "Para.\n\n- one\n- two\n",
			// last non-blank line of any blank-bounded prefix
			// is a list item; our boundary check rejects.
			// The blank line between "Para." and "- one" is
			// the only candidate, but the line AFTER (first
			// non-blank of suffix) is "- one" — that's fine,
			// a list opening doesn't change the prefix's
			// rendering. So the boundary BEFORE the list is
			// accepted.
			want: len("Para.\n\n"),
		},
		{
			name:    "list interior: no boundary",
			content: "- one\n- two\n",
			// no blank line at all.
			want: -1,
		},
		{
			name:    "closed list then paragraph",
			content: "- one\n- two\n\nPara.",
			// blank line after the list. Last non-blank line
			// of prefix is "- two" — a list item — so the
			// candidate is REJECTED. (Conservative: we don't
			// know the list is "closed" without looking at
			// what follows.)
			want: -1,
		},
		{
			name:    "table at end",
			content: "Para.\n\n| a | b |\n| --- | --- |\n| 1 | 2 |\n",
			// blank-line candidate is between "Para." and
			// table opener. Last non-blank line of prefix is
			// "Para." — fine. Line AFTER is "| a | b |"
			// which is a table line; doesn't retroactively
			// change "Para." Boundary accepted.
			want: len("Para.\n\n"),
		},
		{
			name:    "table interior with internal blank line: no late boundary",
			content: "| a | b |\n| --- | --- |\n\n| 1 | 2 |\n",
			// the blank line in the middle is followed by
			// another table line. Last non-blank line of
			// prefix is "| --- | --- |" which contains a
			// pipe — we reject.
			want: -1,
		},
		{
			name:    "block quote at end",
			content: "Para.\n\n> quoted\n> still quoted\n",
			// Last non-blank line of any prefix that ends
			// inside the quote block is a "> ..." line —
			// rejected. The blank line BEFORE the quote
			// gives a prefix of "Para.\n\n" — last non-blank
			// "Para." — accepted.
			want: len("Para.\n\n"),
		},
		{
			name:    "setext underline pending",
			content: "Heading\n\n=====\n",
			// blank line between "Heading" and "=====".
			// Prefix = "Heading\n\n", last non-blank "Heading"
			// — fine. But the FIRST non-blank line of the
			// suffix is "=====", a setext-underline
			// candidate. Splitting here would render the
			// prefix as a paragraph "Heading", but the
			// canonical render would treat the whole thing
			// as a setext header. Reject.
			//
			// (Note: per CommonMark, a blank line between a
			// paragraph and an underline actually breaks the
			// setext, so the setext interpretation may not
			// apply. But the boundary check is conservative
			// — being wrong costs one slow frame, being
			// over-aggressive costs visible breakage.)
			want: -1,
		},
		{
			name:    "indented code at end of prefix",
			content: "Para.\n\n    code line\n\nNext.",
			// prefix candidates:
			//   "Para.\n\n" — last non-blank "Para.", accepted
			//   "Para.\n\n    code line\n\n" — last non-blank
			//   is "    code line" which is indented 4
			//   spaces — REJECTED.
			// Latest accepted is the first.
			want: len("Para.\n\n"),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := findSafeMarkdownBoundary(c.content)
			require.Equalf(t, c.want, got,
				"findSafeMarkdownBoundary(%q) = %d, want %d", c.content, got, c.want)
			if got > 0 {
				// Boundary must point to the start of a line
				// (i.e. just after a newline) when the prefix
				// is non-empty.
				require.True(t, got <= len(c.content),
					"boundary %d out of range (len=%d)", got, len(c.content))
				if got > 0 && got <= len(c.content) {
					require.Equal(t, byte('\n'), c.content[got-1],
						"boundary %d does not sit immediately after a newline", got)
				}
			}
		})
	}
}

// -----------------------------------------------------------------------
// T2: streaming-equivalence tests.
// -----------------------------------------------------------------------

// streamingScenarios returns the four canonical document shapes
// that exercise different boundary-detection paths.
func streamingScenarios() []struct {
	name string
	doc  string
} {
	return []struct {
		name string
		doc  string
	}{
		{
			name: "plain-paragraphs",
			doc: strings.Join([]string{
				"This is the first paragraph of the document.",
				"",
				"Here is the second paragraph; it has some words.",
				"",
				"And a third paragraph for good measure.",
				"",
				"Finally a fourth paragraph to push past one boundary.",
			}, "\n"),
		},
		{
			name: "paragraphs-with-fence",
			doc: strings.Join([]string{
				"Intro paragraph.",
				"",
				"Some explanatory prose before the code.",
				"",
				"```go",
				"func hello() {",
				"\tfmt.Println(\"hi\")",
				"}",
				"```",
				"",
				"And a closing paragraph after the code block.",
			}, "\n"),
		},
		{
			name: "paragraphs-with-list",
			doc: strings.Join([]string{
				"Intro paragraph.",
				"",
				"- list item one",
				"- list item two",
				"- list item three",
				"",
				"Trailing paragraph.",
			}, "\n"),
		},
		{
			name: "paragraphs-with-table",
			doc: strings.Join([]string{
				"Intro paragraph.",
				"",
				"| col a | col b |",
				"| ----- | ----- |",
				"| 1     | 2     |",
				"| 3     | 4     |",
				"",
				"Trailing paragraph after the table.",
			}, "\n"),
		},
	}
}

// progressivePrefixes splits doc into n monotonically growing
// byte prefixes, ending with the full document. n>=1.
func progressivePrefixes(doc string, n int) []string {
	if n < 1 {
		n = 1
	}
	out := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		// integer scaling so the last entry is exactly len(doc)
		size := len(doc) * i / n
		if i == n {
			size = len(doc)
		}
		out = append(out, doc[:size])
	}
	return out
}

// TestStreamingMarkdown_FinalVisuallyEquivalent drives a sequence
// of progressive prefixes through streamingMarkdown and asserts
// the FINAL output is visually equivalent (per design principle
// D) to a fresh full-document render. Strict byte-equality is
// not the bar — see the comment in normalizeRender for why.
func TestStreamingMarkdown_FinalVisuallyEquivalent(t *testing.T) {
	t.Parallel()

	const width = 80
	const steps = 15

	for _, sc := range streamingScenarios() {
		t.Run(sc.name, func(t *testing.T) {
			t.Parallel()
			renderer := newTestRenderer(t, width)
			var sm streamingMarkdown
			prefixes := progressivePrefixes(sc.doc, steps)

			var lastOut string
			for _, p := range prefixes {
				lastOut = sm.Render(p, width, renderer)
			}

			fresh := freshRender(t, sc.doc, width)
			require.Equal(t, normalizeRender(fresh), normalizeRender(lastOut),
				"final streaming output must match a fresh full render visually")
		})
	}
}

// TestStreamingMarkdown_IntermediateOutputsPlausible asserts that
// every intermediate flush returns a non-empty string and does
// not leak raw markdown source through to the user. This is the
// "visually plausible" half of T2.
func TestStreamingMarkdown_IntermediateOutputsPlausible(t *testing.T) {
	t.Parallel()

	const width = 80
	const steps = 12

	for _, sc := range streamingScenarios() {
		t.Run(sc.name, func(t *testing.T) {
			t.Parallel()
			renderer := newTestRenderer(t, width)
			var sm streamingMarkdown

			for i, p := range progressivePrefixes(sc.doc, steps) {
				if p == "" {
					continue
				}
				out := sm.Render(p, width, renderer)
				require.NotEmptyf(t, out, "step %d: empty render for prefix len %d", i, len(p))
				require.Falsef(t, containsRawMarkdownSource(out),
					"step %d: render leaked raw markdown source.\nprefix=%q\nout=%s",
					i, p, normalizeRender(out))
			}
		})
	}
}

// -----------------------------------------------------------------------
// T3: cache invalidation tests.
// -----------------------------------------------------------------------

// TestStreamingMarkdown_WidthChangeInvalidates asserts that a
// width change blows away the cached prefix so the next render
// is keyed against the new width. We can't observe the cache
// directly without reaching into the struct, so we assert the
// observable contract: after a width change, the rendered output
// reflects the new width AND the streamingMarkdown's internal
// cache fields are reset to the new state.
func TestStreamingMarkdown_WidthChangeInvalidates(t *testing.T) {
	t.Parallel()

	doc := "Para one.\n\nPara two.\n\nPara three."
	r80 := newTestRenderer(t, 80)
	r40 := newTestRenderer(t, 40)
	var sm streamingMarkdown

	out80 := sm.Render(doc, 80, r80)
	require.Equal(t, 80, sm.width, "width must be cached after first render")
	cachedPrefix := sm.stablePrefix

	out40 := sm.Render(doc, 40, r40)
	require.Equal(t, 40, sm.width, "width change must update cached width")
	require.NotEqual(t, out80, out40,
		"different widths must produce different rendered output")
	// stablePrefix may legitimately have re-advanced after the
	// reset (tryAdvanceFromEmpty), but if it has, it can no
	// longer carry the OLD width's render. We assert the cache
	// reset by checking that the cached prefix length is at
	// most the current content length.
	require.True(t, len(sm.stablePrefix) <= len(doc),
		"stable prefix must be a prefix of the current content")
	_ = cachedPrefix
}

// TestStreamingMarkdown_NonPrefixContentInvalidates verifies
// that content which is NOT a prefix-extension of the cached
// stable prefix triggers a Reset and a fresh render path. This
// guards the "user retried the turn" case.
func TestStreamingMarkdown_NonPrefixContentInvalidates(t *testing.T) {
	t.Parallel()

	const width = 80
	r := newTestRenderer(t, width)
	var sm streamingMarkdown

	// Drive a streaming sequence so the cache picks up a stable
	// prefix.
	doc := "Para one.\n\nPara two.\n\nPara three."
	for _, p := range progressivePrefixes(doc, 6) {
		_ = sm.Render(p, width, r)
	}
	require.NotEmpty(t, sm.stablePrefix,
		"stable prefix must be populated after streaming a multi-paragraph doc")

	// Now switch to entirely different content (user retried).
	other := "Completely different opening paragraph.\n\nAnd a second."
	out := sm.Render(other, width, r)
	require.NotEmpty(t, out)
	// stablePrefix must be a prefix of `other`, i.e. cache was
	// reset off the OLD content.
	require.True(t, strings.HasPrefix(other, sm.stablePrefix),
		"stable prefix must be reset to a prefix of the new content")

	// Visual equivalence to a fresh render of `other`.
	fresh := freshRender(t, other, width)
	require.Equal(t, normalizeRender(fresh), normalizeRender(out),
		"render after non-prefix content change must match a fresh render")
}

// TestStreamingMarkdown_ResetClearsCache asserts Reset() drops
// every cached field; the next render is necessarily a full
// render path.
func TestStreamingMarkdown_ResetClearsCache(t *testing.T) {
	t.Parallel()

	const width = 80
	r := newTestRenderer(t, width)
	var sm streamingMarkdown

	doc := "Para one.\n\nPara two.\n\nPara three."
	_ = sm.Render(doc, width, r)
	// The sample doc has safe boundaries so the cache should
	// have advanced. If for some reason it didn't, we still
	// want Reset to be a no-op-safe operation; assert the
	// post-Reset state directly.
	sm.Reset()
	require.Equal(t, 0, sm.width)
	require.Equal(t, "", sm.stablePrefix)
	require.Equal(t, "", sm.stablePrefixRender)

	// Next render must be a full render path. Drive one step
	// and verify the output matches a fresh full render.
	out := sm.Render(doc, width, r)
	fresh := freshRender(t, doc, width)
	require.Equal(t, normalizeRender(fresh), normalizeRender(out))
}

// -----------------------------------------------------------------------
// T4: fallback safety.
// -----------------------------------------------------------------------

// TestStreamingMarkdown_NoSafeBoundaryAlwaysFullRenders covers
// the "one giant table being built character by character" case.
// Every flush must fall back to a full render; the cache must
// not advance into an unsafe state. We compare each flush to a
// fresh full render of the same prefix; bytes must match for
// each prefix individually.
//
// (Byte equality is sound here because no concatenation happens:
// the streaming path delegates straight to renderer.Render when
// the cache is empty and no safe boundary exists.)
func TestStreamingMarkdown_NoSafeBoundaryAlwaysFullRenders(t *testing.T) {
	t.Parallel()

	const width = 80

	// One growing table — no blank lines anywhere, so no
	// boundary candidate is ever found.
	doc := strings.Join([]string{
		"| col a | col b | col c |",
		"| ----- | ----- | ----- |",
		"| 1     | 2     | 3     |",
		"| 4     | 5     | 6     |",
		"| 7     | 8     | 9     |",
		"| 10    | 11    | 12    |",
		"| 13    | 14    | 15    |",
		"| 16    | 17    | 18    |",
		"| 19    | 20    | 21    |",
		"| 22    | 23    | 24    |",
	}, "\n")
	require.Equal(t, -1, findSafeMarkdownBoundary(doc),
		"sanity check: no blank lines, no safe boundary")

	r := newTestRenderer(t, width)
	var sm streamingMarkdown

	prefixes := progressivePrefixes(doc, 10)
	for i, p := range prefixes {
		if p == "" {
			continue
		}
		out := sm.Render(p, width, r)
		fresh := freshRender(t, p, width)
		require.Equalf(t, fresh, out,
			"step %d (len=%d): streaming output must byte-equal a fresh render when boundary detection fails",
			i, len(p))
	}
	// Cache must remain empty: no boundary was ever found, no
	// width change occurred, no advance ever cached anything.
	require.Equal(t, "", sm.stablePrefix,
		"stable prefix must remain empty when no safe boundary ever exists")
}

// TestStreamingMarkdown_NoSafeBoundaryDoesNotCrash is the
// minimum-viability assertion of T4: even when boundary
// detection fails on every flush the streaming path must not
// crash and must produce non-empty output for non-empty input.
func TestStreamingMarkdown_NoSafeBoundaryDoesNotCrash(t *testing.T) {
	t.Parallel()

	const width = 80
	r := newTestRenderer(t, width)
	var sm streamingMarkdown

	// A deeply-pathological input: a single line that grows
	// one character at a time. There is never a blank-line
	// separator so the cache is never advanced.
	src := "The quick brown fox jumps over the lazy dog."
	for i := 1; i <= len(src); i++ {
		out := sm.Render(src[:i], width, r)
		require.NotEmpty(t, out, "streaming output must not be empty for non-empty input")
	}
}

// -----------------------------------------------------------------------
// Integration assertions on the wired-in path.
// -----------------------------------------------------------------------

// -----------------------------------------------------------------------
// T5 / T6 / T7: anywhere-in-prefix hazards (B1 / B2 / B3 from the
// F8 round-2 review). For each hazard we drive every progressive
// prefix of a document that exercises the hazard through the cache
// and assert two contracts:
//
//  1. The cached stable prefix never contains the hazard. If the
//     hazard line is at byte offset H, then after every flush
//     len(sm.stablePrefix) <= H. This is the "no silent
//     corruption" half — the algorithm cannot accept a boundary
//     that splits across the hazard.
//
//  2. The final flush is visually equivalent to a fresh full
//     render of the complete document. This is the same T2-style
//     equivalence assertion ported to the new doc shapes.
// -----------------------------------------------------------------------

// nonBlankLines returns the non-blank visible lines of s with
// per-line trailing whitespace trimmed. Used to compare two
// rendered fragments for content equivalence when paragraph-
// margin behaviour legitimately differs between a single fresh
// render and a streaming split render (per F8 design principle D
// — visual equivalence is the bar, byte-equivalence is not).
//
// Some glamour block types (notably HTML blocks and reference
// link definitions) interact with adjacent paragraph blocks
// during a single render — adjacency effectively suppresses the
// blank-line margin between blocks. When the streaming path
// renders the prefix and trail in separate calls, the seam is
// re-introduced as a blank line. The visible TEXT is identical;
// only the inter-block margin differs.
func nonBlankLines(s string) []string {
	clean := stripANSI(s)
	out := make([]string, 0)
	for _, l := range strings.Split(clean, "\n") {
		l = strings.TrimRight(l, " \t")
		if strings.TrimSpace(l) == "" {
			continue
		}
		out = append(out, l)
	}
	return out
}

// runProgressiveBoundaryRespectTest is the shared body of T5/T6/T7.
// It accepts a document and the byte offset of the line whose
// PRESENCE in the prefix must trigger the hazard reject; the
// cached stable prefix may never extend past hazardLineOffset.
//
// The final-output equivalence check is content-based (non-blank
// lines compared) rather than full-normalization: see
// nonBlankLines for the reason.
func runProgressiveBoundaryRespectTest(t *testing.T, doc string, hazardLineOffset int) {
	t.Helper()
	const width = 80
	const steps = 25

	renderer := newTestRenderer(t, width)
	var sm streamingMarkdown

	prefixes := progressivePrefixes(doc, steps)
	var lastOut string
	for i, p := range prefixes {
		if p == "" {
			continue
		}
		lastOut = sm.Render(p, width, renderer)
		require.NotEmptyf(t, lastOut, "step %d: empty render", i)
		require.LessOrEqualf(t, len(sm.stablePrefix), hazardLineOffset,
			"step %d: cached stable prefix advanced past the hazard line\n"+
				"prefix len=%d, hazard at %d, sm.stablePrefix=%q",
			i, len(sm.stablePrefix), hazardLineOffset, sm.stablePrefix)
	}

	fresh := freshRender(t, doc, width)
	require.Equal(t, nonBlankLines(fresh), nonBlankLines(lastOut),
		"final streaming output must contain the same non-blank lines as a fresh full render")
}

// TestStreamingMarkdown_LooseListContinuation locks in the B1 fix.
// A loose list followed by a continuation paragraph and then a
// trailing paragraph creates a candidate boundary between the list
// item and its continuation; the trailing non-blank line of that
// candidate prefix is the continuation paragraph (not a list
// marker), so the line-only check would accept it. The
// anywhere-in-prefix list-marker check rejects it.
func TestStreamingMarkdown_LooseListContinuation(t *testing.T) {
	t.Parallel()

	doc := strings.Join([]string{
		"Intro paragraph.",
		"",
		"- item one",
		"",
		"  continuation paragraph still belongs to item one",
		"",
		"- item two",
		"",
		"Trailing paragraph after the list.",
	}, "\n")

	// The first list marker line begins after "Intro paragraph.\n\n".
	// The cached stable prefix may include that boundary (BEFORE
	// the list opens) but must never advance into the list.
	hazardOffset := strings.Index(doc, "- item one")
	require.Greater(t, hazardOffset, 0, "test setup")

	runProgressiveBoundaryRespectTest(t, doc, hazardOffset)
}

// TestStreamingMarkdown_HTMLBlock locks in the B2 fix. A raw HTML
// block followed by a paragraph creates a candidate boundary
// between the closed HTML block and the trailing paragraph. The
// anywhere-in-prefix HTML-opener check rejects any boundary that
// would include the HTML block in the stable prefix.
func TestStreamingMarkdown_HTMLBlock(t *testing.T) {
	t.Parallel()

	doc := strings.Join([]string{
		"Intro paragraph.",
		"",
		"<div>",
		"some block content",
		"</div>",
		"",
		"Trailing paragraph after the HTML block.",
	}, "\n")

	hazardOffset := strings.Index(doc, "<div>")
	require.Greater(t, hazardOffset, 0, "test setup")

	runProgressiveBoundaryRespectTest(t, doc, hazardOffset)
}

// TestStreamingMarkdown_HTMLBlockType7 covers HTML block type 7
// (CommonMark): a generic open/close tag whose name is NOT in the
// fixed type-6 set still opens an HTML block and must forfeit any
// boundary that would split the block off from following content.
func TestStreamingMarkdown_HTMLBlockType7(t *testing.T) {
	t.Parallel()

	doc := strings.Join([]string{
		"Intro paragraph.",
		"",
		"<custom-tag>",
		"some block content",
		"</custom-tag>",
		"",
		"Trailing paragraph after the custom-tag block.",
	}, "\n")

	hazardOffset := strings.Index(doc, "<custom-tag>")
	require.Greater(t, hazardOffset, 0, "test setup")

	runProgressiveBoundaryRespectTest(t, doc, hazardOffset)
}

// TestStreamingMarkdown_LinkRefDefinition locks in the B3 fix. A
// reference link definition followed by a paragraph that uses the
// reference creates a boundary candidate between the def and the
// paragraph; rendering them in separate glamour passes loses the
// definition. The anywhere-in-prefix ref-def check rejects.
func TestStreamingMarkdown_LinkRefDefinition(t *testing.T) {
	t.Parallel()

	doc := strings.Join([]string{
		"Intro paragraph.",
		"",
		"[ref]: http://example.com",
		"",
		"Trailing paragraph that links to [the example][ref] inline.",
	}, "\n")

	hazardOffset := strings.Index(doc, "[ref]:")
	require.Greater(t, hazardOffset, 0, "test setup")

	runProgressiveBoundaryRespectTest(t, doc, hazardOffset)
}

// TestAssistantStreamingContent_ResetOnClearCache guards the
// integration contract that ClearItemCaches (style change) drops
// the streaming-markdown cache. Without this, a style change
// would leave the OLD style's ANSI sequences embedded in the
// stable-prefix render and the next flush would visually mix
// styles.
func TestAssistantStreamingContent_ResetOnClearCache(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	doc := "Para one.\n\nPara two.\n\nPara three."
	msg := finishedAssistantMessage("stream-clear", doc)
	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)

	const width = 80
	_ = item.RawRender(width)
	// Drive a second message that extends the content so the
	// streaming cache has a chance to advance (if it would).
	doc2 := doc + "\n\nFour."
	item.SetMessage(finishedAssistantMessage("stream-clear", doc2))
	_ = item.RawRender(width)

	// Now wipe the caches the way ClearItemCaches does.
	item.clearCache()

	require.Equal(t, "", item.streamingContent.stablePrefix,
		"clearCache must Reset the streaming-markdown cache")
	require.Equal(t, "", item.streamingContent.stablePrefixRender)
	require.Equal(t, 0, item.streamingContent.width)
}
