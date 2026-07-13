package chat

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/joestump-agent/a2tea"
)

var fenceRe = regexp.MustCompile("(?s)```[^\n]*\n.*?```")

// stripFencedCode removes markdown fenced code blocks entirely. Used for A2UI
// detection where tags inside fences are examples, not live surfaces (#6).
func stripFencedCode(content string) string {
	if !strings.Contains(content, "```") {
		return content
	}
	return fenceRe.ReplaceAllString(content, "")
}

// fenceReplacement tracks a masked fenced-code span for later restoration.
type fenceReplacement struct {
	placeholder string
	original    string
}

// maskFencedCode replaces fenced code blocks with unique placeholders so
// a2tea's markdown-unaware scanner does not extract A2UI JSON from code
// examples (#6). Use unmaskFencedCode to restore the originals after
// scanning.
func maskFencedCode(content string) (string, []fenceReplacement) {
	if !strings.Contains(content, "```") {
		return content, nil
	}
	var reps []fenceReplacement
	masked := fenceRe.ReplaceAllStringFunc(content, func(match string) string {
		p := fmt.Sprintf("\x00FENCE%d\x00", len(reps))
		reps = append(reps, fenceReplacement{p, match})
		return p
	})
	return masked, reps
}

// unmaskFencedCode restores fenced code blocks replaced by maskFencedCode.
func unmaskFencedCode(text string, reps []fenceReplacement) string {
	for _, r := range reps {
		text = strings.ReplaceAll(text, r.placeholder, r.original)
	}
	return text
}

// contentHasA2UI reports whether the assistant content carries any A2UI
// outside fenced code blocks, so the renderer only takes the a2tea path when
// there is live UI to draw.
func contentHasA2UI(content string) bool {
	return a2tea.Contains(stripFencedCode(content))
}

// hasUnclosedA2UITag reports whether content contains an opening
// <a2ui-json> tag whose matching </a2ui-json> close never arrives — a
// truncated block (#5). Only meaningful for finished messages; while
// streaming the close tag simply hasn't arrived yet.
func hasUnclosedA2UITag(content string) bool {
	openIdx := strings.Index(content, "<a2ui-json>")
	if openIdx < 0 {
		return false
	}
	afterOpen := content[openIdx+len("<a2ui-json>"):]
	return !strings.Contains(afterOpen, "</a2ui-json>")
}

// contentHasUnclosedA2UI is the gate-level check (fences stripped) for
// truncated A2UI blocks in finished messages.
func contentHasUnclosedA2UI(content string) bool {
	return hasUnclosedA2UITag(stripFencedCode(content))
}

// countDroppedTaggedBlocks scans content for complete
// <a2ui-json>...</a2ui-json> pairs whose JSON body cannot be unmarshalled —
// blocks a2tea silently drops. This is independent of bare-JSON parts, which
// must not mask the alert (#7).
func countDroppedTaggedBlocks(content string) int {
	const openTag, closeTag = "<a2ui-json>", "</a2ui-json>"
	var dropped int
	s := content
	for {
		i := strings.Index(s, openTag)
		if i < 0 {
			break
		}
		s = s[i+len(openTag):]
		j := strings.Index(s, closeTag)
		if j < 0 {
			break
		}
		body := strings.TrimSpace(s[:j])
		s = s[j+len(closeTag):]
		var raw map[string]any
		if json.Unmarshal([]byte(body), &raw) != nil {
			dropped++
		}
	}
	return dropped
}

// renderContentWithA2UI renders assistant content that contains A2UI. a2tea
// scans the content into ordered parts of prose text and typed A2UI messages;
// crush renders the prose as markdown and hands each part's messages to
// a2tea.Render, stitching the rendered surface in place.
//
// Fenced code blocks are masked before scanning so that A2UI examples inside
// code fences are not extracted as live surfaces (#6). If any complete tag
// pair contains malformed JSON — a block a2tea drops silently — an alert
// element is appended (#7). An unclosed <a2ui-json> tag (truncated
// generation) also triggers the alert (#5).
//
// This deliberately bypasses the streaming-markdown prefix cache (which assumes
// a single glamour render per item) and renders each segment directly. The
// renderer is shared, so the whole multi-render sequence holds its lock.
func (a *AssistantMessageItem) renderContentWithA2UI(content string, width int) string {
	masked, fenceReps := maskFencedCode(content)

	parts, err := a2tea.Scan(masked)
	if err != nil {
		// Not parseable as A2UI — render everything as markdown so nothing is
		// lost.
		return a.renderMarkdown(content, width)
	}

	renderer := common.MarkdownRenderer(a.sty, width)
	mu := common.LockMarkdownRenderer(renderer)
	mu.Lock()
	defer mu.Unlock()

	renderMarkdown := func(text string) string {
		if strings.TrimSpace(text) == "" {
			return ""
		}
		// Restore fenced code blocks before markdown rendering.
		text = unmaskFencedCode(text, fenceReps)
		out, err := renderer.Render(text)
		if err != nil {
			return strings.TrimSpace(text)
		}
		return trimGlamourMargins(out)
	}

	var b strings.Builder
	writeChunk := func(s string) {
		if s == "" {
			return
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(s)
	}

	for _, p := range parts {
		writeChunk(renderMarkdown(p.Text))
		if len(p.Messages) == 0 {
			continue
		}
		model, err := a2tea.Render(p.Messages)
		if err != nil {
			// Valid A2UI messages with nothing to draw (e.g. a data-model
			// update). Not an error worth alarming the user about.
			continue
		}
		if sz, ok := model.(interface{ SetSize(width, height int) }); ok {
			sz.SetSize(width, 0)
		}
		writeChunk(strings.TrimRight(model.View().Content, "\n"))
	}

	// Alert when a complete tag pair was dropped by the parser (#7) —
	// checked directly so bare-JSON parts cannot mask the count — or when
	// generation was truncated mid-block (#5).
	if countDroppedTaggedBlocks(masked) > 0 || hasUnclosedA2UITag(masked) {
		writeChunk(a.renderA2UIAlert(width))
	}

	return b.String()
}

// renderTruncatedA2UI handles a finished message whose <a2ui-json> block was
// never closed — generation was truncated mid-block (#5). The prose before
// the unclosed tag is rendered as markdown, and the truncated block is
// surfaced through the standard A2UI alert instead of leaving a wall of raw
// JSON.
func (a *AssistantMessageItem) renderTruncatedA2UI(content string, width int) string {
	stripped := stripFencedCode(content)
	idx := strings.Index(stripped, "<a2ui-json>")

	var b strings.Builder
	if idx > 0 {
		prose := stripped[:idx]
		if strings.TrimSpace(prose) != "" {
			b.WriteString(a.renderMarkdown(prose, width))
		}
	}
	if b.Len() > 0 {
		b.WriteString("\n\n")
	}
	b.WriteString(a.renderA2UIAlert(width))
	return b.String()
}

// renderA2UIAlert builds an alert element shown when content advertised A2UI but
// a2tea could not turn it into a surface. Styled in crush's existing
// error-message language.
func (a *AssistantMessageItem) renderA2UIAlert(width int) string {
	inner := max(width-2, 1)
	tag := a.sty.Messages.ErrorTag.Render("A2UI")
	title := a.sty.Messages.ErrorTitle.Render("couldn't render a UI block in this message")
	reason := a.sty.Messages.ErrorDetails.Width(inner).Render(
		"The A2UI content was malformed or used unsupported components.")
	return tag + " " + title + "\n\n" + reason
}
