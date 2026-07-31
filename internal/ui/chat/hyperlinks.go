package chat

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/util"
	"github.com/charmbracelet/x/ansi"
	"github.com/pkg/browser"
	"github.com/rivo/uniseg"
)

// hyperlinkSpan records the cell geometry of one OSC 8 hyperlink inside a
// rendered (post-glamour) string. Rows are zero-based line offsets within
// the string; columns are zero-based cell offsets within the row, start
// inclusive and end exclusive. Spans are produced by [parseHyperlinkSpans]
// and consumed by MouseClickCommandable implementations to open links on
// plain (unmodified) clicks, since terminals only act on OSC 8 links for
// modifier-clicks while the application holds mouse reporting.
type hyperlinkSpan struct {
	row      int
	colStart int
	colEnd   int
	url      string
}

// parseHyperlinkSpans walks a rendered string, decodes OSC 8 hyperlink
// sequences, and returns one span per contiguous linked run per line.
// Columns are measured in terminal cells, matching the coordinates the
// chat click dispatcher hands to items (click handlers offset x for the
// prefix columns, so spans here are purely content-relative).
//
// A link that wraps across a line break produces one span per line
// segment, all carrying the same URL. Terminals render those as one
// logical link via the OSC 8 id parameter; treating each segment as its
// own span keeps hit-testing a simple rectangle check.
func parseHyperlinkSpans(s string) []hyperlinkSpan {
	if !strings.ContainsRune(s, 0x1b) {
		return nil
	}

	var spans []hyperlinkSpan
	row, col := 0, 0
	active := false
	activeURL := ""
	segStart := 0

	closeSegment := func() {
		if active && col > segStart {
			spans = append(spans, hyperlinkSpan{
				row:      row,
				colStart: segStart,
				colEnd:   col,
				url:      activeURL,
			})
		}
	}

	parser := ansi.GetParser()
	defer ansi.PutParser(parser)

	var state byte
	for len(s) > 0 {
		if s[0] != 0x1b {
			if s[0] == '\n' {
				closeSegment()
				row++
				col = 0
				segStart = 0
				s = s[1:]
				continue
			}
			// Advance one grapheme cluster so wide cells (CJK, emoji)
			// count the same width the terminal will give them.
			cluster, width, rest := firstGrapheme(s)
			col += width
			s = rest
			_ = cluster
			continue
		}

		parser.Reset()
		seq, _, n, newState := ansi.DecodeSequence(s, state, parser)
		if seq == "" {
			break
		}
		state = newState
		s = s[n:]

		// OSC 8 hyperlink: ESC ] 8 ; params ; url, terminated by BEL or ST.
		if strings.HasPrefix(seq, "\x1b]8;") {
			body := strings.TrimPrefix(seq, "\x1b]8;")
			body = strings.TrimSuffix(body, "\a")
			body = strings.TrimSuffix(body, "\x1b\\")
			var url string
			if parts := strings.SplitN(body, ";", 2); len(parts) == 2 {
				url = parts[1]
			}
			if url == "" {
				closeSegment()
				active = false
				activeURL = ""
			} else {
				active = true
				activeURL = url
				segStart = col
			}
		}
	}
	// Trailing segment with no explicit reset.
	closeSegment()
	return spans
}

// firstGrapheme returns the first grapheme cluster in s, its width in
// terminal cells, and the remainder of s.
func firstGrapheme(s string) (string, int, string) {
	cluster, rest, width, _ := uniseg.FirstGraphemeClusterInString(s, -1)
	return cluster, width, rest
}

// spanAt returns the URL of the hyperlink span covering (row, col), or ""
// when the position is not inside any span.
func spanAt(spans []hyperlinkSpan, row, col int) string {
	for _, sp := range spans {
		if sp.row == row && col >= sp.colStart && col < sp.colEnd {
			return sp.url
		}
	}
	return ""
}

// openURL returns a command that opens url in the system browser and
// reports the outcome as a toast. It exists because terminals hand every
// mouse event to the application once mouse reporting is enabled, so the
// terminal's own link-click handling (and its modifier-click affordances)
// never fires inside the TUI.
//
// Only http(s) URLs are opened. Markdown link targets come from
// untrusted model- and web-derived content, and a plain (unmodified)
// click has none of the friction a terminal's modifier-click affords, so
// an ungated browser.OpenURL would hand file://, javascript:, and custom
// scheme handlers (vscode://, deep links) straight to the OS. Non-http
// targets are refused with a toast instead.
func openURL(url string) tea.Cmd {
	return func() tea.Msg {
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			scheme := url
			if i := strings.Index(url, ":"); i >= 0 {
				scheme = url[:i]
			}
			return util.ReportInfo("Blocked non-http link: " + scheme)()
		}
		if err := browser.OpenURL(url); err != nil {
			return util.ReportError(err)()
		}
		return util.ReportInfo("Opened " + url)()
	}
}

// hyperlinkizeURLs wraps bare http(s) URLs in plain text with OSC 8
// hyperlink sequences so they are clickable in the TUI (and in terminals
// whenever the application releases the mouse). Glamour already emits OSC
// 8 for markdown content; this helper exists for plain-text surfaces like
// tool and shell output that never pass through markdown rendering.
//
// The scan is byte-oriented and scheme-anchored ("http://" or "https://"),
// terminating a URL at whitespace or at characters that cannot legally
// trail a URL (quotes, angle brackets). Trailing punctuation that is
// almost certainly prose rather than address (period, comma, semicolon,
// colon, closing parens and brackets) is trimmed from the link target but
// left in the visible text.
func hyperlinkizeURLs(s string) string {
	if !strings.Contains(s, "://") {
		return s
	}
	var buf strings.Builder
	buf.Grow(len(s) + 64)
	i := 0
	for i < len(s) {
		idx := urlSchemeAt(s, i)
		if idx < 0 {
			buf.WriteString(s[i:])
			break
		}
		buf.WriteString(s[i:idx])
		end := urlEnd(s, idx)
		url := strings.TrimRight(s[idx:end], ".,;:)]}\"'")
		buf.WriteString(ansi.SetHyperlink(url, ""))
		buf.WriteString(s[idx:end])
		buf.WriteString(ansi.ResetHyperlink())
		i = end
	}
	return buf.String()
}

// urlSchemeAt returns the index of the first "http://" or "https://"
// occurrence in s at or after offset from, or -1.
func urlSchemeAt(s string, from int) int {
	http := strings.Index(s[from:], "http://")
	https := strings.Index(s[from:], "https://")
	switch {
	case http < 0:
		if https < 0 {
			return -1
		}
		return from + https
	case https < 0:
		return from + http
	default:
		return from + min(http, https)
	}
}

// urlEnd returns the offset just past the URL beginning at start (which
// points at a scheme). The URL runs until whitespace or a quote/bracket
// that cannot be part of an address in prose.
func urlEnd(s string, start int) int {
	i := start
	for i < len(s) {
		c := s[i]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' ||
			c == '"' || c == '<' || c == '>' || c == 0x1b {
			break
		}
		i++
	}
	return i
}
