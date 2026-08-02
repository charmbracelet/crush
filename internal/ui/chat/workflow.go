package chat

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/tree"

	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/anim"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// WorkflowToolMessageItem is a message item that represents a workflow tool call.
type WorkflowToolMessageItem struct {
	*baseToolMessageItem

	nestedTools []ToolMessageItem
	running     int
	completed   int
	total       int
	lastLog     string
	labels      map[int]string // agent index → label
}

var (
	_ ToolMessageItem     = (*WorkflowToolMessageItem)(nil)
	_ NestedToolContainer = (*WorkflowToolMessageItem)(nil)
)

// NewWorkflowToolMessageItem creates a new [WorkflowToolMessageItem].
func NewWorkflowToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) *WorkflowToolMessageItem {
	t := &WorkflowToolMessageItem{}
	t.baseToolMessageItem = newBaseToolMessageItem(sty, toolCall, result, &WorkflowToolRenderContext{workflow: t}, canceled)
	t.spinningFunc = func(state SpinningState) bool {
		return !state.HasResult() && !state.IsCanceled()
	}
	return t
}

// Animate progresses the message animation if it should be spinning.
func (w *WorkflowToolMessageItem) Animate(msg anim.StepMsg) tea.Cmd {
	if w.result != nil || w.Status() == ToolStatusCanceled {
		return nil
	}
	if msg.ID == w.ID() {
		w.Bump()
		return w.anim.Animate(msg)
	}
	for _, nestedTool := range w.nestedTools {
		if msg.ID != nestedTool.ID() {
			continue
		}
		if s, ok := nestedTool.(Animatable); ok {
			w.Bump()
			return s.Animate(msg)
		}
	}
	return nil
}

// NestedTools returns the nested tools.
func (w *WorkflowToolMessageItem) NestedTools() []ToolMessageItem {
	return w.nestedTools
}

// SetNestedTools sets the nested tools.
func (w *WorkflowToolMessageItem) SetNestedTools(tools []ToolMessageItem) {
	w.nestedTools = tools
	w.clearCache()
	w.Bump()
}

// AddNestedTool adds a nested tool.
func (w *WorkflowToolMessageItem) AddNestedTool(tool ToolMessageItem) {
	if s, ok := tool.(Compactable); ok {
		s.SetCompact(true)
	}
	w.nestedTools = append(w.nestedTools, tool)
	w.clearCache()
	w.Bump()
}

// SetProgress updates live progress state for the workflow.
func (w *WorkflowToolMessageItem) SetProgress(running, completed, total, index int, kind, label, msg string) {
	w.running = running
	w.completed = completed
	w.total = total
	if kind == "log" && msg != "" {
		w.lastLog = msg
	}
	if kind == "agent_start" && label != "" && index >= 0 {
		if w.labels == nil {
			w.labels = make(map[int]string)
		}
		w.labels[index] = label
	}
	w.clearCache()
	w.Bump()
}

// Description returns the description from the workflow tool parameters.
func (w *WorkflowToolMessageItem) Description() string {
	if w == nil {
		return ""
	}
	var params agent.WorkflowParams
	_ = json.Unmarshal([]byte(w.ToolCall().Input), &params)
	return params.Description
}

// agentLabelIndexRe extracts the agent index from a synthetic tool call ID.
var agentLabelIndexRe = regexp.MustCompile(`-a(\d+)$`)

// truncateString truncates s to at most max runes, appending "…" if truncated.
func truncateString(s string, max int) string {
	if max < 1 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}

// WorkflowToolRenderContext renders workflow tool messages.
type WorkflowToolRenderContext struct {
	workflow *WorkflowToolMessageItem
}

// RenderTool implements the [ToolRenderer] interface.
func (r *WorkflowToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if !opts.ToolCall.Finished && !opts.IsCanceled() && len(r.workflow.nestedTools) == 0 && r.workflow.total == 0 {
		return pendingTool(sty, "Workflow", opts.Anim, opts.Compact)
	}

	var params agent.WorkflowParams
	_ = json.Unmarshal([]byte(opts.ToolCall.Input), &params)

	description := params.Description
	if !opts.ExpandedContent {
		description = strings.ReplaceAll(description, "\n", " ")
	}

	header := toolHeader(sty, opts.Status, "Workflow", cappedWidth, opts)
	if opts.Compact {
		return header
	}

	taskTag := sty.Tool.AgentTaskTag.Render("Workflow")
	taskTagWidth := lipgloss.Width(taskTag)

	remainingWidth := min(cappedWidth-taskTagWidth-3, maxTextWidth-taskTagWidth-3)

	promptText := sty.Tool.AgentPrompt.Width(remainingWidth).Render(description)

	header = lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		lipgloss.JoinHorizontal(
			lipgloss.Left,
			taskTag,
			" ",
			promptText,
		),
	)

	childTools := tree.Root(header)

	for _, nestedTool := range r.workflow.nestedTools {
		childView := nestedTool.Render(remainingWidth)
		// Prefix with agent label when available.
		if m := agentLabelIndexRe.FindStringSubmatch(nestedTool.ToolCall().ID); len(m) >= 2 {
			if idx, err := strconv.Atoi(m[1]); err == nil {
				if label, ok := r.workflow.labels[idx]; ok {
					childView = fmt.Sprintf("%s %s", label, childView)
				}
			}
		}
		childTools.Child(childView)
	}

	var parts []string
	parts = append(parts, childTools.Enumerator(roundedEnumerator(2, taskTagWidth-5)).String())

	if !opts.HasResult() && !opts.IsCanceled() {
		if r.workflow.total > 0 {
			countsLine := fmt.Sprintf("%d running · %d/%d done", r.workflow.running, r.workflow.completed, r.workflow.total)
			parts = append(parts, "", countsLine)
			if r.workflow.lastLog != "" {
				logWidth := cappedWidth - toolBodyLeftPaddingTotal - 4
				if logWidth < 20 {
					logWidth = 20
				}
				parts = append(parts, truncateString(r.workflow.lastLog, logWidth))
			}
		}
		parts = append(parts, "", opts.Anim.Render())
	}

	result := lipgloss.JoinVertical(lipgloss.Left, parts...)

	if opts.HasResult() && opts.Result.Content != "" {
		body := toolOutputMarkdownContent(sty, opts.Result.Content, cappedWidth-toolBodyLeftPaddingTotal, opts.ExpandedContent)
		return joinToolParts(result, body)
	}

	return result
}
