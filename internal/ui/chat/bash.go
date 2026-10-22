package chat

import (
	"cmp"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/common"
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
	workingDir string,
) ToolMessageItem {
	base := newBaseToolMessageItem(sty, toolCall, result, &BashToolRenderContext{workingDir: workingDir}, canceled)
	return &BashToolMessageItem{baseToolMessageItem: base}
}

// BashToolRenderContext renders bash tool messages.
type BashToolRenderContext struct {
	workingDir string
}

// RenderTool implements the [ToolRenderer] interface.
func (b *BashToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if opts.IsPending() {
		return pendingTool(sty, "Bash", opts.Anim, opts.Compact)
	}

	var params tools.BashParams
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err != nil {
		params.Command = "failed to parse command"
	}

	// Check if this is a background job.
	var meta tools.BashResponseMetadata
	if opts.HasResult() {
		_ = json.Unmarshal([]byte(opts.Result.Metadata), &meta)
	}

	if meta.Background {
		description := cmp.Or(meta.Description, params.Command)
		content := "Command: " + params.Command + "\n" + opts.Result.Content
		dirChip := renderDirChip(sty, b.workingDir, cmp.Or(meta.WorkingDirectory, params.WorkingDir), cappedWidth)
		return renderJobTool(sty, opts, cappedWidth, "Start", meta.ShellID, description, content, dirChip)
	}

	// Regular bash command. The command is always rendered expanded
	// (newlines preserved); expansion only controls the output body.
	cmd := params.Command
	cmd = strings.ReplaceAll(cmd, "\t", "    ")
	cmd = common.StripBashDisplayPrefix(cmd, b.workingDir)
	if highlighted, err := common.SyntaxHighlightLexerName(sty, cmd, "bash", nil); err == nil {
		cmd = highlighted
	}

	// The working directory is rendered as a chip between the tool name and
	// the command when it ran outside the workspace directory.
	dirChip := renderDirChip(sty, b.workingDir, cmp.Or(meta.WorkingDirectory, params.WorkingDir), cappedWidth)

	// The description is prepared like the command: outer blank lines
	// trimmed and tabs indented with spaces. Only its first line leads the
	// header when collapsed.
	desc := strings.TrimSpace(params.Description)
	desc = strings.ReplaceAll(desc, "\t", "    ")

	// The "background=true" pair rides on either header.
	var extraParams []string
	if params.RunInBackground {
		extraParams = append(extraParams, "background", "true")
	}

	// The command is always rendered expanded (wrapped, never truncated);
	// expansion only controls the output body.
	// When the model provides a description, it becomes the header
	// (truncated to one line, full when expanded)
	headerOpts := *opts
	headerOpts.ExpandedContent = true
	var header string
	if desc != "" && !opts.Compact {
		// Nested tool calls (compact) keep the plain command header.
		header = renderDescriptionHeader(sty, opts, cappedWidth, desc, cmd, dirChip, extraParams...)
	} else {
		header = toolHeaderWithChip(sty, opts.Status, "Bash", dirChip, cappedWidth, &headerOpts, sty.Tool.ParamMain, append([]string{cmd}, extraParams...)...)
	}
	if opts.Compact {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(header, earlyState)
	}

	if !opts.HasResult() {
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
	body := sty.Tool.Body.Render(toolOutputPlainContent(sty, output, bodyWidth, opts.ExpandedContent))
	return joinToolParts(header, body)
}

// renderDescriptionHeader renders a two-line header: the description (its
// first line truncated when collapsed, the whole description wrapped when
// expanded) followed by the full command, indented to align under the tool
// name. The working directory chip sits between the tool name and the
// description.
func renderDescriptionHeader(sty *styles.Styles, opts *ToolRenderOpts, width int, desc, cmd, dirChip string, extraParams ...string) string {
	if !opts.ExpandedContent {
		if first, _, found := strings.Cut(desc, "\n"); found {
			desc = first
		}
	}
	header := toolHeaderWithChip(sty, opts.Status, "Bash", dirChip, width, opts, sty.Tool.Description, append([]string{desc}, extraParams...)...)

	// The command wraps below the description as if the chip was absent,
	// indented under the tool name, and is never truncated.
	indent := lipgloss.Width(toolBasePrefix(sty, opts.Status, "Bash", opts))
	expandedOpts := ToolRenderOpts{ExpandedContent: true}
	cmdStr := toolParamListMainStyle(sty, []string{cmd}, width-indent, &expandedOpts, sty.Tool.ParamMain)
	return header + "\n" + indentLines(cmdStr, indent)
}

// relativeDir returns dir relative to the workspace directory workingDir, or
// an empty string when it matches the workspace or cannot be determined.
func relativeDir(workingDir, dir string) string {
	if dir == "" || workingDir == "" {
		return ""
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(workingDir, dir)
	}
	rel, err := filepath.Rel(workingDir, dir)
	if err != nil || rel == "." {
		return ""
	}
	return rel
}

// renderDirChip renders the working directory chip shown between the tool
// name and the command in tool headers. It returns an empty string when the
// chip does not apply. The relative path is capped at half the header width
// so the command keeps enough room.
func renderDirChip(sty *styles.Styles, workingDir, dir string, width int) string {
	rel := relativeDir(workingDir, dir)
	if rel == "" {
		return ""
	}
	maxRel := width/2 - len("cd ") - len("/")
	if maxRel < 4 {
		return ""
	}
	rel = ansi.Truncate(rel, maxRel, "…")
	if !strings.HasSuffix(rel, "…") {
		rel += "/"
	}
	return sty.Tool.WorkingDirChip.Render("cd " + rel)
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
	workingDir string,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &JobOutputToolRenderContext{workingDir: workingDir}, canceled)
}

// JobOutputToolRenderContext renders job_output tool messages.
type JobOutputToolRenderContext struct {
	workingDir string
}

// RenderTool implements the [ToolRenderer] interface.
func (j *JobOutputToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if opts.IsPending() {
		return pendingTool(sty, "Job", opts.Anim, opts.Compact)
	}

	var params tools.JobOutputParams
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err != nil {
		return toolErrorContent(sty, &message.ToolResult{Content: "Invalid parameters"}, cappedWidth)
	}

	var description, dirChip string
	if opts.HasResult() && opts.Result.Metadata != "" {
		var meta tools.JobOutputResponseMetadata
		if err := json.Unmarshal([]byte(opts.Result.Metadata), &meta); err == nil {
			description = cmp.Or(meta.Description, meta.Command)
			dirChip = renderDirChip(sty, j.workingDir, meta.WorkingDirectory, cappedWidth)
		}
	}

	content := ""
	if opts.HasResult() {
		content = opts.Result.Content
	}
	return renderJobTool(sty, opts, cappedWidth, "Output", params.ShellID, description, content, dirChip)
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
	if opts.IsPending() {
		return pendingTool(sty, "Job", opts.Anim, opts.Compact)
	}

	var params tools.JobKillParams
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err != nil {
		return toolErrorContent(sty, &message.ToolResult{Content: "Invalid parameters"}, cappedWidth)
	}

	var description string
	if opts.HasResult() && opts.Result.Metadata != "" {
		var meta tools.JobKillResponseMetadata
		if err := json.Unmarshal([]byte(opts.Result.Metadata), &meta); err == nil {
			description = cmp.Or(meta.Description, meta.Command)
		}
	}

	content := ""
	if opts.HasResult() {
		content = opts.Result.Content
	}
	return renderJobTool(sty, opts, cappedWidth, "Kill", params.ShellID, description, content, "")
}

// renderJobTool renders a job-related tool with the common pattern:
// header → nested check → early state → body. The chip, when non-empty, is
// shown in the header.
func renderJobTool(sty *styles.Styles, opts *ToolRenderOpts, width int, action, shellID, description, content, chip string) string {
	header := jobHeader(sty, opts.Status, action, shellID, description, width, chip)
	if opts.Compact {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, width); ok {
		return joinToolParts(header, earlyState)
	}

	if content == "" {
		return header
	}

	bodyWidth := width - toolBodyLeftPaddingTotal
	body := sty.Tool.Body.Render(toolOutputPlainContent(sty, content, bodyWidth, opts.ExpandedContent))
	return joinToolParts(header, body)
}

// jobHeader builds a header for job-related tools.
// Format: "● Job (Action) PID shellID [chip] description..."
func jobHeader(sty *styles.Styles, status ToolStatus, action, shellID, description string, width int, chip string) string {
	icon := toolIcon(sty, status)
	jobPart := sty.Tool.JobToolName.Render("Job")
	actionPart := sty.Tool.JobAction.Render("(" + action + ")")
	pidPart := sty.Tool.JobPID.Render("PID " + shellID)

	prefix := fmt.Sprintf("%s %s %s %s", icon, jobPart, actionPart, pidPart)
	if chip != "" {
		prefix += " " + chip
	}

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
