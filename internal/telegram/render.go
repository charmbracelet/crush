package telegram

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"
	"unicode/utf16"

	"github.com/charmbracelet/crush/internal/proto"
)

// Chunk splits text into pieces that fit within Telegram's message size
// limit, measured in UTF-16 code units. Prefer newline boundaries; fall
// back to a hard rune-boundary split. Empty input returns an empty slice.
func Chunk(text string, limit int) []string {
	if text == "" {
		return []string{}
	}
	if limit <= 0 {
		limit = 3900
	}
	if utf16Len(text) <= limit {
		return []string{text}
	}

	var chunks []string
	runes := []rune(text)
	for len(runes) > 0 {
		// Binary-search the largest rune prefix whose UTF-16 length
		// is <= limit.
		lo, hi := 1, len(runes)
		best := 1
		for lo <= hi {
			mid := (lo + hi) / 2
			if utf16Len(string(runes[:mid])) <= limit {
				best = mid
				lo = mid + 1
			} else {
				hi = mid - 1
			}
		}
		// Prefer a newline split inside the window.
		split := best
		if best < len(runes) {
			window := string(runes[:best])
			if idx := strings.LastIndex(window, "\n"); idx > 0 {
				// idx is a byte offset into window; convert to rune count.
				split = len([]rune(window[:idx+1]))
			}
		}
		chunks = append(chunks, string(runes[:split]))
		runes = runes[split:]
	}
	return chunks
}

func utf16Len(s string) int {
	return len(utf16.Encode([]rune(s)))
}

// TruncateMiddle shortens s to at most max runes, keeping the head and
// tail joined by a truncation marker.
func TruncateMiddle(s string, max int) string {
	runes := []rune(s)
	if max <= 0 || len(runes) <= max {
		return s
	}
	head := max * 2 / 3
	tail := max / 3
	if head+tail > max {
		tail = max - head
	}
	if head < 1 {
		head = 1
	}
	if tail < 1 {
		tail = 1
	}
	if head+tail > len(runes) {
		return s
	}
	return string(runes[:head]) + "\n…(truncated)…\n" + string(runes[len(runes)-tail:])
}

// PermissionSummary builds an HTML-mode summary of a permission request.
func PermissionSummary(req proto.PermissionRequest) string {
	var b strings.Builder
	b.WriteString("🔐 <b>Permission request</b>\n")
	b.WriteString(toolEmoji(req.ToolName))
	b.WriteString(" <b>")
	b.WriteString(html.EscapeString(req.ToolName))
	b.WriteString("</b>")
	if req.Description != "" {
		b.WriteString(" — ")
		b.WriteString(html.EscapeString(req.Description))
	}
	b.WriteByte('\n')

	switch p := req.Params.(type) {
	case proto.BashPermissionsParams:
		b.WriteString("<pre>")
		b.WriteString(html.EscapeString(TruncateMiddle(p.Command, 1000)))
		b.WriteString("</pre>")
		if p.WorkingDir != "" {
			b.WriteString("\ndir: ")
			b.WriteString(html.EscapeString(p.WorkingDir))
		}
		if p.RunInBackground {
			b.WriteString("\n(background)")
		}
	case proto.EditPermissionsParams:
		writeFileDiff(&b, p.FilePath, p.OldContent, p.NewContent)
	case proto.WritePermissionsParams:
		writeFileDiff(&b, p.FilePath, p.OldContent, p.NewContent)
	case proto.MultiEditPermissionsParams:
		writeFileDiff(&b, p.FilePath, p.OldContent, p.NewContent)
	case proto.DownloadPermissionsParams:
		b.WriteString("<code>")
		b.WriteString(html.EscapeString(p.URL))
		b.WriteString("</code>")
		if p.FilePath != "" {
			b.WriteString("\n→ <code>")
			b.WriteString(html.EscapeString(p.FilePath))
			b.WriteString("</code>")
		}
	case proto.FetchPermissionsParams:
		b.WriteString("<code>")
		b.WriteString(html.EscapeString(p.URL))
		b.WriteString("</code>")
		if p.Format != "" {
			b.WriteString(" (")
			b.WriteString(html.EscapeString(p.Format))
			b.WriteString(")")
		}
	case proto.AgenticFetchPermissionsParams:
		b.WriteString("<code>")
		b.WriteString(html.EscapeString(p.URL))
		b.WriteString("</code>")
		if p.Prompt != "" {
			b.WriteString("\n")
			b.WriteString(html.EscapeString(TruncateMiddle(p.Prompt, 400)))
		}
	case proto.ViewPermissionsParams:
		b.WriteString("<code>")
		b.WriteString(html.EscapeString(p.FilePath))
		b.WriteString("</code>")
	case proto.LSPermissionsParams:
		b.WriteString("<code>")
		b.WriteString(html.EscapeString(p.Path))
		b.WriteString("</code>")
	default:
		if req.Params != nil {
			raw, err := json.Marshal(req.Params)
			if err == nil {
				b.WriteString("<pre>")
				b.WriteString(html.EscapeString(TruncateMiddle(string(raw), 1000)))
				b.WriteString("</pre>")
			}
		}
	}
	return b.String()
}

func writeFileDiff(b *strings.Builder, path, oldContent, newContent string) {
	b.WriteString("<code>")
	b.WriteString(html.EscapeString(path))
	b.WriteString("</code>\n<pre>")
	oldLines := firstLines(TruncateMiddle(oldContent, 600), 20)
	newLines := firstLines(TruncateMiddle(newContent, 600), 20)
	for _, line := range oldLines {
		b.WriteString("− ")
		b.WriteString(html.EscapeString(line))
		b.WriteByte('\n')
	}
	for _, line := range newLines {
		b.WriteString("+ ")
		b.WriteString(html.EscapeString(line))
		b.WriteByte('\n')
	}
	b.WriteString("</pre>")
}

func firstLines(s string, n int) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return lines
}

func toolEmoji(name string) string {
	switch name {
	case "bash":
		return "💻"
	case "edit", "write", "multiedit":
		return "✏️"
	case "download", "fetch", "agentic_fetch":
		return "🌐"
	case "view", "ls":
		return "📂"
	default:
		return "🔧"
	}
}

// formatChunks prepares multi-chunk messages with [i/n] prefixes on
// chunks after the first.
func formatChunks(chunks []string) []string {
	if len(chunks) <= 1 {
		return chunks
	}
	out := make([]string, len(chunks))
	out[0] = chunks[0]
	for i := 1; i < len(chunks); i++ {
		out[i] = fmt.Sprintf("[%d/%d]\n%s", i+1, len(chunks), chunks[i])
	}
	return out
}
