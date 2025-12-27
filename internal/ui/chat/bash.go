package chat

import (
	"cmp"
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

// -----------------------------------------------------------------------------
// Bash Tool
// -----------------------------------------------------------------------------

// BashToolMessageItem is a message item that represents a bash tool call.
type BashToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*BashToolMessageItem)(nil)

// NewBashToolMessageItem creates a new [BashToolMessageItem].
func NewBashToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &BashToolRenderContext{}, canceled)
}

// BashToolRenderContext renders bash tool messages.
type BashToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (b *BashToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if !opts.ToolCall.Finished && !opts.Canceled {
		return pendingTool(sty, "Bash", opts.Anim)
	}

	var params tools.BashParams
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err != nil {
		params.Command = "failed to parse command"
	}

	// Check if this is a background job.
	var meta tools.BashResponseMetadata
	if opts.Result != nil {
		_ = json.Unmarshal([]byte(opts.Result.Metadata), &meta)
	}

	if meta.Background {
		description := cmp.Or(meta.Description, params.Command)
		content := "Command: " + params.Command + "\n" + opts.Result.Content
		return renderJobTool(sty, opts, cappedWidth, "Start", meta.ShellID, description, content)
	}

	// Regular bash command.
	cmd := strings.ReplaceAll(params.Command, "\n", " ")
	cmd = strings.ReplaceAll(cmd, "\t", "    ")
	toolParams := []string{cmd}
	if params.RunInBackground {
		toolParams = append(toolParams, "background", "true")
	}

	header := toolHeader(sty, opts.Status(), "Bash", cappedWidth, opts.Nested, toolParams...)
	if opts.Nested {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(header, earlyState)
	}

	if opts.Result == nil {
		return header
	}

	output := meta.Output
	if output == "" && opts.Result.Content != tools.BashNoOutput {
		output = opts.Result.Content
	}
	if output == "" {
		return header
	}

	bodyWidth := cappedWidth - toolBodyLeftPaddingTotal
	body := sty.Tool.Body.Render(toolOutputPlainContent(sty, output, bodyWidth, opts.Expanded))
	return joinToolParts(header, body)
}

// -----------------------------------------------------------------------------
// Job Output Tool
// -----------------------------------------------------------------------------

// JobOutputToolMessageItem is a message item for job_output tool calls.
type JobOutputToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*JobOutputToolMessageItem)(nil)

// NewJobOutputToolMessageItem creates a new [JobOutputToolMessageItem].
func NewJobOutputToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &JobOutputToolRenderContext{}, canceled)
}

// JobOutputToolRenderContext renders job_output tool messages.
type JobOutputToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (j *JobOutputToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if !opts.ToolCall.Finished && !opts.Canceled {
		return pendingTool(sty, "Job", opts.Anim)
	}

	var params tools.JobOutputParams
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err != nil {
		return toolErrorContent(sty, &message.ToolResult{Content: "Invalid parameters"}, cappedWidth)
	}

	var description string
	if opts.Result != nil && opts.Result.Metadata != "" {
		var meta tools.JobOutputResponseMetadata
		if err := json.Unmarshal([]byte(opts.Result.Metadata), &meta); err == nil {
			description = cmp.Or(meta.Description, meta.Command)
		}
	}

	content := ""
	if opts.Result != nil {
		content = opts.Result.Content
	}
	return renderJobTool(sty, opts, cappedWidth, "Output", params.ShellID, description, content)
}

// -----------------------------------------------------------------------------
// Job Kill Tool
// -----------------------------------------------------------------------------

// JobKillToolMessageItem is a message item for job_kill tool calls.
type JobKillToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*JobKillToolMessageItem)(nil)

// NewJobKillToolMessageItem creates a new [JobKillToolMessageItem].
func NewJobKillToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &JobKillToolRenderContext{}, canceled)
}

// JobKillToolRenderContext renders job_kill tool messages.
type JobKillToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (j *JobKillToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if !opts.ToolCall.Finished && !opts.Canceled {
		return pendingTool(sty, "Job", opts.Anim)
	}

	var params tools.JobKillParams
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err != nil {
		return toolErrorContent(sty, &message.ToolResult{Content: "Invalid parameters"}, cappedWidth)
	}

	var description string
	if opts.Result != nil && opts.Result.Metadata != "" {
		var meta tools.JobKillResponseMetadata
		if err := json.Unmarshal([]byte(opts.Result.Metadata), &meta); err == nil {
			description = cmp.Or(meta.Description, meta.Command)
		}
	}

	content := ""
	if opts.Result != nil {
		content = opts.Result.Content
	}
	return renderJobTool(sty, opts, cappedWidth, "Kill", params.ShellID, description, content)
}

// renderJobTool renders a job-related tool with the common pattern:
// header → nested check → early state → body.
func renderJobTool(sty *styles.Styles, opts *ToolRenderOpts, width int, action, shellID, description, content string) string {
	header := jobHeader(sty, opts.Status(), action, shellID, description, width)
	if opts.Nested {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, width); ok {
		return joinToolParts(header, earlyState)
	}

	if content == "" {
		return header
	}

	bodyWidth := width - toolBodyLeftPaddingTotal
	body := sty.Tool.Body.Render(toolOutputPlainContent(sty, content, bodyWidth, opts.Expanded))
	return joinToolParts(header, body)
}

// jobHeader builds a header for job-related tools.
// Format: "● Job (Action) PID shellID description..."
func jobHeader(sty *styles.Styles, status ToolStatus, action, shellID, description string, width int) string {
	icon := toolIcon(sty, status)
	jobPart := sty.Tool.JobToolName.Render("Job")
	actionPart := sty.Tool.JobAction.Render("(" + action + ")")
	pidPart := sty.Tool.JobPID.Render("PID " + shellID)

	prefix := fmt.Sprintf("%s %s %s %s", icon, jobPart, actionPart, pidPart)

	if description == "" {
		return prefix
	}

	prefixWidth := lipgloss.Width(prefix)
	availableWidth := width - prefixWidth - 1
	if availableWidth < 10 {
		return prefix
	}

	truncatedDesc := ansi.Truncate(description, availableWidth, "…")
	return prefix + " " + sty.Tool.JobDescription.Render(truncatedDesc)
}

// joinToolParts joins header and body with a blank line separator.
func joinToolParts(header, body string) string {
	return strings.Join([]string{header, "", body}, "\n")
}
