package chat

import (
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/ui/util"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestParseHyperlinkSpans_BasicLink(t *testing.T) {
	t.Parallel()

	line := "see " + ansi.SetHyperlink("https://stu.mp", "id=1") + "Hell world" + ansi.ResetHyperlink() + " end"
	spans := parseHyperlinkSpans(line)
	require.Len(t, spans, 1)
	require.Equal(t, hyperlinkSpan{row: 0, colStart: 4, colEnd: 14, url: "https://stu.mp"}, spans[0])
}

func TestParseHyperlinkSpans_MultipleLinesAndLinks(t *testing.T) {
	t.Parallel()

	s := "no links here\n" +
		"a " + ansi.SetHyperlink("https://one.test", "id=1") + "one" + ansi.ResetHyperlink() + " b\n" +
		ansi.SetHyperlink("https://two.test", "id=2") + "two" + ansi.ResetHyperlink()
	spans := parseHyperlinkSpans(s)
	require.Len(t, spans, 2)
	require.Equal(t, hyperlinkSpan{row: 1, colStart: 2, colEnd: 5, url: "https://one.test"}, spans[0])
	require.Equal(t, hyperlinkSpan{row: 2, colStart: 0, colEnd: 3, url: "https://two.test"}, spans[1])
}

func TestParseHyperlinkSpans_LinkWrappedAcrossLines(t *testing.T) {
	t.Parallel()

	// A link open before a newline stays open after it: the wrap must
	// produce one span per line segment carrying the same URL.
	s := ansi.SetHyperlink("https://wrap.test", "id=1") + "abc\ndef" + ansi.ResetHyperlink()
	spans := parseHyperlinkSpans(s)
	require.Len(t, spans, 2)
	require.Equal(t, hyperlinkSpan{row: 0, colStart: 0, colEnd: 3, url: "https://wrap.test"}, spans[0])
	require.Equal(t, hyperlinkSpan{row: 1, colStart: 0, colEnd: 3, url: "https://wrap.test"}, spans[1])
}

func TestParseHyperlinkSpans_WideCells(t *testing.T) {
	t.Parallel()

	// CJK characters occupy two cells; the span columns must count cells,
	// not runes, or the click geometry drifts right of the glyph.
	s := "見" + ansi.SetHyperlink("https://cjk.test", "id=1") + "ab" + ansi.ResetHyperlink()
	spans := parseHyperlinkSpans(s)
	require.Len(t, spans, 1)
	require.Equal(t, hyperlinkSpan{row: 0, colStart: 2, colEnd: 4, url: "https://cjk.test"}, spans[0])
}

func TestParseHyperlinkSpans_NoEscapes(t *testing.T) {
	t.Parallel()
	require.Nil(t, parseHyperlinkSpans("plain text, no escapes"))
	require.Nil(t, parseHyperlinkSpans(""))
}

func TestParseHyperlinkSpans_STTerminator(t *testing.T) {
	t.Parallel()

	// OSC sequences may end with ST (ESC \) instead of BEL. Build the
	// string from byte literals so the two-byte ST (ESC backslash) is
	// unambiguous.
	s := "x \x1b]8;;https://st.test\x1b\\go\x1b]8;;\x1b\\ y"
	spans := parseHyperlinkSpans(s)
	require.Len(t, spans, 1)
	require.Equal(t, "https://st.test", spans[0].url)
	require.Equal(t, 2, spans[0].colStart)
	require.Equal(t, 4, spans[0].colEnd)
}

func TestSpanAt(t *testing.T) {
	t.Parallel()

	spans := []hyperlinkSpan{
		{row: 0, colStart: 4, colEnd: 14, url: "https://a.test"},
		{row: 2, colStart: 0, colEnd: 3, url: "https://b.test"},
	}
	require.Equal(t, "https://a.test", spanAt(spans, 0, 4))
	require.Equal(t, "https://a.test", spanAt(spans, 0, 13))
	require.Empty(t, spanAt(spans, 0, 14), "end is exclusive")
	require.Empty(t, spanAt(spans, 1, 4), "wrong row")
	require.Equal(t, "https://b.test", spanAt(spans, 2, 0))
	require.Empty(t, spanAt(nil, 0, 0))
}

func TestHyperlinkizeURLs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		in    string
		want  string
		noOSC bool
	}{
		{name: "no url", in: "just some text", noOSC: true},
		{
			name: "https url", in: "see https://example.com/docs here",
			want: "\x1b]8;;https://example.com/docs\ahttps://example.com/docs\x1b]8;;\a",
		},
		{
			name: "http url", in: "go to http://insecure.test now",
			want: "\x1b]8;;http://insecure.test\ahttp://insecure.test\x1b]8;;\a",
		},
		{
			name: "trailing period trimmed from target", in: "visit https://example.com.",
			want: "\x1b]8;;https://example.com\ahttps://example.com.\x1b]8;;\a",
		},
		{
			name: "two urls", in: "https://a.test and https://b.test",
			want: "\x1b]8;;https://a.test\ahttps://a.test\x1b]8;;\a and \x1b]8;;https://b.test\ahttps://b.test\x1b]8;;\a",
		},
		{
			name: "url in quotes not extended", in: `"https://quoted.test"`,
			want: "\x1b]8;;https://quoted.test\ahttps://quoted.test\x1b]8;;\a",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out := hyperlinkizeURLs(tc.in)
			if tc.noOSC {
				require.Equal(t, tc.in, out, "text without URLs must pass through untouched")
				return
			}
			require.Contains(t, out, tc.want)
		})
	}
}

// TestHyperlinkizeURLs_RoundTrip verifies the wrapper output parses back
// to spans covering exactly the URL text.
func TestHyperlinkizeURLs_RoundTrip(t *testing.T) {
	t.Parallel()

	out := hyperlinkizeURLs("open https://example.com/path?q=1 for docs")
	spans := parseHyperlinkSpans(out)
	require.Len(t, spans, 1)
	require.Equal(t, "https://example.com/path?q=1", spans[0].url)
	require.Equal(t, 5, spans[0].colStart)
	require.Equal(t, 5+len("https://example.com/path?q=1"), spans[0].colEnd)
}

// TestAssistantMessageItemHyperlinkClick guards the click-to-open contract
// on assistant messages: a plain click on a rendered hyperlink span must
// be handled and produce a command (browser open), without disturbing the
// expansion state machine or the copy footer.
func TestAssistantMessageItemHyperlinkClick(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	msg := &message.Message{
		ID:   "link-1",
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: "docs at https://example.com/guide today"},
			message.Finish{Reason: message.FinishReasonEndTurn, Time: testFinishTime},
		},
	}
	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)
	item.RawRender(60)
	require.NotEmpty(t, item.hyperlinks, "rendered markdown links must be indexed")

	sp := item.hyperlinks[0]
	// Click the middle of the link span (chat-relative x).
	handled, cmd := item.HandleMouseClickCmd(ansi.MouseLeft,
		(sp.colStart+sp.colEnd)/2+MessageLeftPaddingTotal, sp.row)
	require.True(t, handled, "click on a hyperlink must be handled")
	require.NotNil(t, cmd, "click on a hyperlink must produce the open command")
	require.Equal(t, thinkingCollapsed, item.thinkingViewMode,
		"opening a link must not toggle expansion")

	// Click off the link is not the open path.
	_, cmd = item.HandleMouseClickCmd(ansi.MouseLeft, MessageLeftPaddingTotal, sp.row)
	require.Nil(t, cmd)
}

// TestUserMessageItemHyperlinkClick guards click-to-open on user messages,
// which previously had no mouse handling at all.
func TestUserMessageItemHyperlinkClick(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	msg := &message.Message{
		ID:    "ulink-1",
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "check https://example.com out"}},
	}
	item := NewUserMessageItem(&sty, msg, nil).(*UserMessageItem)
	item.RawRender(60)
	require.NotEmpty(t, item.hyperlinks, "user message links must be indexed")

	sp := item.hyperlinks[0]
	handled, cmd := item.HandleMouseClickCmd(ansi.MouseLeft,
		(sp.colStart+sp.colEnd)/2+MessageLeftPaddingTotal, sp.row)
	require.True(t, handled)
	require.NotNil(t, cmd)

	// Off-link clicks are unhandled (no expansion behavior on user items).
	handled, cmd = item.HandleMouseClickCmd(ansi.MouseLeft, MessageLeftPaddingTotal, sp.row+1)
	require.False(t, handled)
	require.Nil(t, cmd)
}

// TestOpenURLBlocksNonHTTP is the regression guard for the scheme gate:
// markdown link targets come from untrusted model- and web-derived
// content, and a plain click has no modifier-click friction, so file://,
// javascript:, and custom scheme handlers must never reach the OS.
func TestOpenURLBlocksNonHTTP(t *testing.T) {
	t.Parallel()

	blocked := []string{
		"file:///etc/passwd",
		"javascript:alert(1)",
		"vscode://malicious/payload",
		"ftp://example.com/file",
	}
	for _, url := range blocked {
		msg := openURL(url)()
		info, ok := msg.(util.InfoMsg)
		require.True(t, ok, "blocked URL must produce an info toast, got %T", msg)
		require.Contains(t, info.Msg, "Blocked non-http link",
			"non-http URL %q must be refused, not opened", url)
	}
}

// TestNonHTTPMarkdownLinkProducesSpanButDoesNotOpen locks in the full
// attack path: a malicious markdown link still renders as a clickable
// span (glamour emits OSC 8 for any target), but clicking it runs the
// gate and yields a blocked toast rather than a browser open.
func TestNonHTTPMarkdownLinkProducesSpanButDoesNotOpen(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	msg := &message.Message{
		ID:   "evil-1",
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: "[click me](file:///etc/passwd)"},
			message.Finish{Reason: message.FinishReasonEndTurn, Time: testFinishTime},
		},
	}
	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)
	item.RawRender(60)
	require.NotEmpty(t, item.hyperlinks, "glamour must still index the malicious link span")
	require.Equal(t, "file:///etc/passwd", item.hyperlinks[0].url)

	sp := item.hyperlinks[0]
	handled, cmd := item.HandleMouseClickCmd(ansi.MouseLeft,
		(sp.colStart+sp.colEnd)/2+MessageLeftPaddingTotal, sp.row)
	require.True(t, handled, "click on the span is still handled (it shows the block toast)")
	require.NotNil(t, cmd)

	info, ok := cmd().(util.InfoMsg)
	require.True(t, ok, "blocked click must produce an info toast, got %T", cmd())
	require.Contains(t, info.Msg, "Blocked non-http link: file")
}
