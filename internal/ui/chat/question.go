package chat

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/question"
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
	header := toolHeader(sty, opts.Status, "Question", cappedWidth, opts, headerText)
	if opts.Compact {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(header, earlyState)
	}

	if opts.HasEmptyResult() {
		return header
	}

	body := formatQuestionAnswers(sty, opts.Result, cappedWidth-toolBodyLeftPaddingTotal)
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

// formatQuestionAnswers renders each question with its answer and notes
// from the tool's structured metadata.
func formatQuestionAnswers(sty *styles.Styles, result *message.ToolResult, width int) string {
	var meta tools.QuestionResponseMetadata
	if err := json.Unmarshal([]byte(result.Metadata), &meta); err != nil {
		// Results saved before the tool emitted metadata.
		var lines []string
		for line := range strings.SplitSeq(strings.TrimSpace(result.Content), "\n") {
			lines = append(lines, sty.Tool.TodoStatusNote.Render(ansi.Truncate(line, width, "…")))
		}
		return strings.Join(lines, "\n")
	}

	var lines []string
	for i, a := range meta.Answers {
		var q question.Question
		if i < len(meta.Questions) {
			q = meta.Questions[i]
		}

		line := fmt.Sprintf("%s %s %s",
			sty.Tool.IconSuccess.Render(),
			sty.Tool.TodoStatusNote.Render(q.Text),
			styleAnswer(sty, q, a),
		)
		lines = append(lines, ansi.Truncate(line, width, "…"))

		for _, note := range answerNotes(q, a) {
			lines = append(lines, ansi.Truncate(sty.Tool.TodoStatusNote.Render("  ╰ "+note), width, "…"))
		}
	}
	return strings.Join(lines, "\n")
}

// styleAnswer renders an answer on a single line.
func styleAnswer(sty *styles.Styles, q question.Question, a question.Answer) string {
	switch {
	case a.Yes != nil && *a.Yes:
		return sty.Tool.TodoCompletedIcon.Render("Yes")
	case a.Yes != nil:
		return sty.Tool.StateCancelled.Render("No")
	case len(a.SelectedIDs) == 0 && a.FillInText == "":
		return sty.Tool.StateCancelled.Render("Skipped")
	}

	parts := make([]string, 0, len(a.SelectedIDs)+1)
	for _, id := range a.SelectedIDs {
		parts = append(parts, sty.Tool.ParamMain.Render(choiceLabel(q, id)))
	}
	if a.FillInText != "" {
		parts = append(parts, sty.Tool.ParamMain.Render(strings.Join(strings.Fields(a.FillInText), " ")))
	}
	return strings.Join(parts, sty.Tool.TodoStatusNote.Render(", "))
}

// answerNotes lists the question's own note first, then choice notes in
// choice order.
func answerNotes(q question.Question, a question.Answer) []string {
	var notes []string
	if note, ok := a.Notes[question.QuestionNoteKey]; ok {
		notes = append(notes, note)
	}
	keys := make([]string, 0, len(a.Notes))
	for key := range a.Notes {
		if key != question.QuestionNoteKey {
			keys = append(keys, key)
		}
	}
	slices.SortFunc(keys, func(x, y string) int { return choiceIndex(q, x) - choiceIndex(q, y) })
	for _, key := range keys {
		notes = append(notes, choiceLabel(q, key)+": "+a.Notes[key])
	}
	return notes
}

func choiceLabel(q question.Question, id string) string {
	if i := choiceIndex(q, id); i < len(q.Choices) && q.Choices[i].Label != "" {
		return q.Choices[i].Label
	}
	return id
}

// choiceIndex returns the position of id in q.Choices, or len(q.Choices) if
// it is not a choice.
func choiceIndex(q question.Question, id string) int {
	if i := slices.IndexFunc(q.Choices, func(c question.Choice) bool { return c.ID == id }); i >= 0 {
		return i
	}
	return len(q.Choices)
}
