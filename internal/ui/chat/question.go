package chat

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

// QuestionToolMessageItem renders question tool calls in the chat.
type QuestionToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*QuestionToolMessageItem)(nil)

// NewQuestionToolMessageItem creates a new [QuestionToolMessageItem].
func NewQuestionToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &QuestionToolRenderContext{}, canceled)
}

// QuestionToolRenderContext renders question tool messages.
type QuestionToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (q *QuestionToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if opts.IsPending() {
		return pendingTool(sty, "Question", opts.Anim, opts.Compact)
	}

	var params tools.QuestionParams
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err != nil {
		return toolErrorContent(sty, &message.ToolResult{Content: "Invalid parameters"}, cappedWidth)
	}

	headerText := questionSummary(params)
	header := toolHeader(sty, opts.Status, "Question", cappedWidth, opts.Compact, headerText)
	if opts.Compact {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(header, earlyState)
	}

	if opts.HasEmptyResult() {
		return header
	}

	body := formatQuestionAnswers(sty, opts.Result.Content, cappedWidth-toolBodyLeftPaddingTotal)
	if body == "" {
		return header
	}

	return joinToolParts(header, sty.Tool.Body.Render(body))
}

// questionSummary builds a short header summary from the question params.
func questionSummary(params tools.QuestionParams) string {
	n := len(params.Questions)
	if n == 0 {
		return ""
	}
	if n == 1 {
		text := params.Questions[0].Question
		if len(text) > 60 {
			text = text[:59] + "…"
		}
		return text
	}
	first := params.Questions[0].Question
	if len(first) > 40 {
		first = first[:39] + "…"
	}
	return fmt.Sprintf("%s (+%d more)", first, n-1)
}

// questionBlock holds a parsed Q&A block from the tool result.
type questionBlock struct {
	question string
	answer   string
	notes    []string
}

// parseQuestionBlocks splits the tool result into per-question blocks,
// correctly handling the Notes subsection within each block.
func parseQuestionBlocks(content string) []questionBlock {
	// Split on "QN: " boundaries rather than \n\n since notes
	// introduce extra \n\n within a single question block.
	var rawBlocks []string
	lines := strings.Split(content, "\n")
	var current []string
	for _, line := range lines {
		if strings.HasPrefix(line, "Q") && len(current) > 0 {
			rest := strings.TrimPrefix(line, "Q")
			if idx := strings.IndexByte(rest, ':'); idx > 0 {
				allDigits := true
				for _, c := range rest[:idx] {
					if c < '0' || c > '9' {
						allDigits = false
						break
					}
				}
				if allDigits {
					rawBlocks = append(rawBlocks, strings.Join(current, "\n"))
					current = nil
				}
			}
		}
		current = append(current, line)
	}
	if len(current) > 0 {
		rawBlocks = append(rawBlocks, strings.Join(current, "\n"))
	}

	blocks := make([]questionBlock, 0, len(rawBlocks))
	for _, raw := range rawBlocks {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}

		var b questionBlock

		// Strip the "QN: <question>\n" prefix.
		if nlIdx := strings.IndexByte(raw, '\n'); nlIdx >= 0 {
			b.question = strings.TrimSpace(raw[:nlIdx])
			// Remove the "QN: " prefix from the question text.
			if colonIdx := strings.Index(b.question, ": "); colonIdx >= 0 {
				b.question = b.question[colonIdx+2:]
			}
			raw = raw[nlIdx+1:]
		}

		// Split off notes section if present.
		if notesIdx := strings.Index(raw, "\n\nNotes:"); notesIdx >= 0 {
			b.answer = strings.TrimSpace(raw[:notesIdx])
			notesRaw := raw[notesIdx+len("\n\nNotes:"):]
			for _, noteLine := range strings.Split(notesRaw, "\n") {
				noteLine = strings.TrimSpace(noteLine)
				if strings.HasPrefix(noteLine, "- ") {
					b.notes = append(b.notes, strings.TrimPrefix(noteLine, "- "))
				}
			}
		} else {
			b.answer = strings.TrimSpace(raw)
		}

		blocks = append(blocks, b)
	}

	return blocks
}

// formatQuestionAnswers parses the tool result and formats answers with
// styling for display in the chat body.
func formatQuestionAnswers(sty *styles.Styles, content string, width int) string {
	if content == "" {
		return ""
	}

	blocks := parseQuestionBlocks(content)
	if len(blocks) == 0 {
		return ""
	}

	var lines []string
	for _, b := range blocks {
		icon := sty.Tool.IconSuccess.Render()
		answer := styleAnswer(sty, b.answer)

		// Show question text in subtle style, answer on same line.
		qText := sty.Tool.TodoStatusNote.Render(b.question)
		line := fmt.Sprintf("%s %s %s", icon, qText, answer)
		line = ansi.Truncate(line, width, "…")
		lines = append(lines, line)

		for _, note := range b.notes {
			noteLine := sty.Tool.TodoStatusNote.Render("  ╰ " + note)
			noteLine = ansi.Truncate(noteLine, width, "…")
			lines = append(lines, noteLine)
		}
	}

	return strings.Join(lines, "\n")
}

// styleAnswer extracts the meaningful part of an answer string and styles it.
// styleAnswer extracts the meaningful part of an answer string and styles it.
// An answer may span multiple lines (e.g. multi-choice selections plus a
// custom fill-in), so each line is styled independently and rejoined.
func styleAnswer(sty *styles.Styles, answer string) string {
	answer = strings.TrimSpace(answer)
	lines := strings.Split(answer, "\n")
	styled := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		styled = append(styled, styleAnswerLine(sty, line))
	}
	return strings.Join(styled, sty.Tool.TodoStatusNote.Render(", "))
}

// styleAnswerLine styles a single answer line.
func styleAnswerLine(sty *styles.Styles, answer string) string {
	switch {
	case answer == "User answered: yes":
		return sty.Tool.TodoCompletedIcon.Render("Yes")
	case answer == "User answered: no":
		return sty.Tool.StateCancelled.Render("No")
	case strings.HasPrefix(answer, "User selected:"):
		selected := strings.TrimPrefix(answer, "User selected: ")
		selected = strings.Trim(selected, "[]\"")
		selected = strings.ReplaceAll(selected, "\",\"", ", ")
		return sty.Tool.ParamMain.Render(selected)
	case strings.HasPrefix(answer, "User provided:"):
		text := strings.TrimPrefix(answer, "User provided: ")
		return sty.Tool.ParamMain.Render(text)
	case answer == "User skipped this question":
		return sty.Tool.StateCancelled.Render("Skipped")
	default:
		return answer
	}
}
