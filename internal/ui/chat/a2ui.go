package chat

import (
	"strings"

	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/joestump-agent/a2tea"
)

// contentHasA2UI reports whether the assistant content carries any A2UI, so the
// renderer only takes the a2tea path when there is UI to draw. Detection (and
// all parsing) lives in a2tea — crush does not hand-roll it.
func contentHasA2UI(content string) bool {
	return a2tea.Contains(content)
}

// renderContentWithA2UI renders assistant content that contains A2UI. a2tea
// scans the content into ordered parts of prose text and typed A2UI messages;
// crush renders the prose as markdown and hands each part's messages to
// a2tea.Render, stitching the rendered surface in place.
//
// If the content advertised A2UI (contentHasA2UI gated us here) but the parser
// produced no messages at all — malformed or unsupported JSON, which a2tea/
// a2uistream drops silently — an alert element is appended so the block is
// never silently lost. Messages that parse but describe nothing to draw (e.g.
// a data-model update) are skipped without an alert.
//
// This deliberately bypasses the streaming-markdown prefix cache (which assumes
// a single glamour render per item) and renders each segment directly. The
// renderer is shared, so the whole multi-render sequence holds its lock.
func (a *AssistantMessageItem) renderContentWithA2UI(content string, width int) string {
	parts, err := a2tea.Scan(content)
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

	messageBearingParts := 0
	for _, p := range parts {
		writeChunk(renderMarkdown(p.Text))
		if len(p.Messages) == 0 {
			continue
		}
		messageBearingParts++
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

	// Each <a2ui-json> block that a2tea parsed becomes one message-bearing
	// part; a block that was malformed or used unsupported components is
	// dropped by the parser and yields no part. If fewer blocks parsed than
	// were advertised, at least one was dropped — alert rather than silently
	// losing it.
	if messageBearingParts < strings.Count(content, "<a2ui-json>") {
		writeChunk(a.renderA2UIAlert(width))
	}

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
