package chat

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/joestump-agent/a2tea"
)

// a2uiFence matches a fenced ```a2ui code block. The single capture group is
// the JSON body between the fences. This is how an agent signals that a chunk
// of its reply is an A2UI document rather than prose, e.g.
//
//	```a2ui
//	{ "kind": "card", "title": "Hi", "buttons": [ ... ] }
//	```
var a2uiFence = regexp.MustCompile("(?s)```a2ui[ \\t]*\\r?\\n(.*?)```")

// contentHasA2UI reports whether content contains at least one a2ui block, so
// the renderer can take the (slightly more expensive) segmented path only when
// there is actually something for a2tea to render.
func contentHasA2UI(content string) bool {
	return strings.Contains(content, "```a2ui")
}

// renderA2UI renders a single A2UI JSON document to a display string via the
// a2tea bridge. It returns the a2tea.Render error (unwrapped) when the document
// is not valid A2UI, so the caller can show an alert element rather than
// silently dropping or misrendering the payload — this relies on a2tea.Render
// returning a real error for bad/unknown documents rather than a silent
// placeholder.
func renderA2UI(raw string, width int) (string, error) {
	model, err := a2tea.Render(json.RawMessage(strings.TrimSpace(raw)))
	if err != nil {
		return "", err
	}
	// a2tea renderers are embeddable children: give the element the width the
	// host allocated to the message. (Stub renderers ignore it today.)
	if sz, ok := model.(interface{ SetSize(width, height int) }); ok {
		sz.SetSize(width, 0)
	}
	return strings.TrimRight(model.View().Content, "\n"), nil
}

// a2uiErrorMaxLines bounds how much of an unrenderable payload the alert
// element echoes back, so a large malformed document cannot flood the chat.
const a2uiErrorMaxLines = 12

// renderA2UIError builds an alert element shown in place of an a2ui block that
// a2tea could not render. It tells the user the embedded content (usually JSON)
// failed to render, why, and echoes the offending payload so it is inspectable
// rather than lost. Styled in crush's existing error-message language.
func (a *AssistantMessageItem) renderA2UIError(raw string, err error, width int) string {
	inner := max(width-2, 1)

	tag := a.sty.Messages.ErrorTag.Render("A2UI")
	title := a.sty.Messages.ErrorTitle.Render("couldn't render this element")
	header := tag + " " + title

	reason := a.sty.Messages.ErrorDetails.Width(inner).Render(err.Error())

	payload := clampLines(strings.TrimSpace(raw), a2uiErrorMaxLines)
	body := a.sty.Messages.ErrorDetails.Width(inner).Render(payload)

	return header + "\n\n" + reason + "\n\n" + body
}

// clampLines trims s to at most n lines, appending an elision note when it had
// to cut.
func clampLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	kept := lines[:n]
	return strings.Join(kept, "\n") + fmt.Sprintf("\n… (%d more lines)", len(lines)-n)
}

// renderContentWithA2UI renders assistant content that contains one or more
// a2ui blocks. Prose segments are rendered as markdown as usual; each a2ui
// block is handed to a2tea and its rendered element is stitched in place. A
// block that is not valid A2UI falls back to being rendered as its original
// fenced markdown, so nothing is ever dropped.
//
// This deliberately bypasses the streaming-markdown prefix cache (which
// assumes a single glamour render per item) and renders each segment directly.
// The renderer is shared, so the whole multi-render sequence holds its lock.
func (a *AssistantMessageItem) renderContentWithA2UI(content string, width int) string {
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

	// Split() yields the prose between blocks; FindAllStringSubmatch() yields
	// the blocks. len(segments) == len(blocks)+1, so they interleave cleanly.
	segments := a2uiFence.Split(content, -1)
	blocks := a2uiFence.FindAllStringSubmatch(content, -1)
	for i, seg := range segments {
		writeChunk(renderMarkdown(seg))
		if i < len(blocks) {
			if rendered, err := renderA2UI(blocks[i][1], width); err == nil {
				writeChunk(rendered)
			} else {
				// Not valid A2UI — show an alert element echoing the payload
				// instead of dropping it or dumping raw JSON as prose.
				writeChunk(a.renderA2UIError(blocks[i][1], err, width))
			}
		}
	}
	return b.String()
}
