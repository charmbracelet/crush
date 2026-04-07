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

// RequestUserInputToolMessageItem renders request_user_input tool calls with
// structured question/answer summaries.
type RequestUserInputToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*RequestUserInputToolMessageItem)(nil)

func NewRequestUserInputToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &RequestUserInputToolRenderContext{}, canceled)
}

// RequestUserInputToolRenderContext renders request_user_input tool output.
type RequestUserInputToolRenderContext struct{}

func (r *RequestUserInputToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if opts.IsPending() {
		return pendingTool(sty, "Request User Input", opts.Anim, opts.Compact)
	}

	var params tools.RequestUserInputParams
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err != nil {
		return toolErrorContent(sty, &message.ToolResult{Content: "Invalid parameters"}, cappedWidth)
	}

	totalQuestions := len(params.Questions)
	headerSummary := fmt.Sprintf("%d question", totalQuestions)
	if totalQuestions != 1 {
		headerSummary += "s"
	}

	header := toolHeader(sty, opts.Status, "Request User Input", cappedWidth, opts.Compact, headerSummary)
	if opts.Compact {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(header, earlyState)
	}

	if !opts.HasResult() || opts.Result.Content == "" {
		return header
	}

	bodyWidth := cappedWidth - toolBodyLeftPaddingTotal
	body := renderRequestUserInputBody(sty, params, opts.Result.Content, bodyWidth, opts.ExpandedContent)
	if body == "" {
		return header
	}
	return joinToolParts(header, sty.Tool.Body.Render(body))
}

func renderRequestUserInputBody(sty *styles.Styles, params tools.RequestUserInputParams, resultContent string, bodyWidth int, expanded bool) string {
	var parsed tools.RequestUserInputResult
	if err := json.Unmarshal([]byte(resultContent), &parsed); err != nil {
		return toolOutputPlainContent(sty, resultContent, bodyWidth, expanded)
	}

	lines := make([]string, 0, len(params.Questions)+len(parsed.Answers)+4)
	status := strings.TrimSpace(parsed.Status)
	if status != "" {
		lines = append(lines, fmt.Sprintf("Status: %s", status))
	}
	if cancelReason := strings.TrimSpace(parsed.CancelReason); cancelReason != "" {
		lines = append(lines, fmt.Sprintf("Reason: %s", cancelReason))
	}
	if len(lines) > 0 {
		lines = append(lines, "")
	}

	questionByID := make(map[string]tools.RequestUserInputQuestion, len(params.Questions))
	for _, question := range params.Questions {
		questionByID[question.ID] = question
	}

	for _, answer := range parsed.Answers {
		question := questionByID[answer.QuestionID]
		questionText := strings.TrimSpace(question.Question)
		if questionText == "" {
			questionText = answer.QuestionID
		}
		lines = append(lines, fmt.Sprintf("Q: %s", questionText))

		summary := strings.TrimSpace(answer.SelectedOption)
		if custom := strings.TrimSpace(answer.CustomInput); custom != "" {
			summary = fmt.Sprintf("Custom: %s", custom)
		}
		if summary == "" {
			summary = "(no answer)"
		}
		lines = append(lines, "A: "+summary)
		lines = append(lines, "")
	}

	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return ""
	}

	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], bodyWidth, "…")
	}
	return toolOutputPlainContent(sty, strings.Join(lines, "\n"), bodyWidth, expanded)
}
