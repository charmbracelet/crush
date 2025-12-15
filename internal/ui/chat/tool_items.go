package chat

import (
	"cmp"
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/tree"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/fsext"
)

// NewToolItem creates the appropriate tool item for the given context.
func NewToolItem(ctx ToolCallContext) MessageItem {
	switch ctx.Call.Name {
	// Bash tools
	case tools.BashToolName:
		return NewBashToolItem(ctx)
	case tools.JobOutputToolName:
		return NewJobOutputToolItem(ctx)
	case tools.JobKillToolName:
		return NewJobKillToolItem(ctx)

	// File tools
	case tools.ViewToolName:
		return NewViewToolItem(ctx)
	case tools.EditToolName:
		return NewEditToolItem(ctx)
	case tools.MultiEditToolName:
		return NewMultiEditToolItem(ctx)
	case tools.WriteToolName:
		return NewWriteToolItem(ctx)

	// Search tools
	case tools.GlobToolName:
		return NewGlobToolItem(ctx)
	case tools.GrepToolName:
		return NewGrepToolItem(ctx)
	case tools.LSToolName:
		return NewLSToolItem(ctx)
	case tools.SourcegraphToolName:
		return NewSourcegraphToolItem(ctx)

	// Fetch tools
	case tools.FetchToolName:
		return NewFetchToolItem(ctx)
	case tools.AgenticFetchToolName:
		return NewAgenticFetchToolItem(ctx)
	case tools.WebFetchToolName:
		return NewWebFetchToolItem(ctx)
	case tools.WebSearchToolName:
		return NewWebSearchToolItem(ctx)
	case tools.DownloadToolName:
		return NewDownloadToolItem(ctx)

	// LSP tools
	case tools.DiagnosticsToolName:
		return NewDiagnosticsToolItem(ctx)
	case tools.ReferencesToolName:
		return NewReferencesToolItem(ctx)

	// Misc tools
	case tools.TodosToolName:
		return NewTodosToolItem(ctx)
	case agent.AgentToolName:
		return NewAgentToolItem(ctx)

	default:
		return NewGenericToolItem(ctx)
	}
}

// -----------------------------------------------------------------------------
// Bash Tools
// -----------------------------------------------------------------------------

// BashToolItem renders bash command execution.
type BashToolItem struct {
	toolItem
	ctx ToolCallContext
}

func NewBashToolItem(ctx ToolCallContext) *BashToolItem {
	return &BashToolItem{
		toolItem: newToolItem(ctx),
		ctx:      ctx,
	}
}

func (m *BashToolItem) Render(width int) string {
	var params tools.BashParams
	unmarshalParams(m.ctx.Call.Input, &params)

	cmd := strings.ReplaceAll(params.Command, "\n", " ")
	cmd = strings.ReplaceAll(cmd, "\t", "    ")

	// Check if this is a background job that finished
	if m.ctx.Call.Finished {
		var meta tools.BashResponseMetadata
		unmarshalParams(m.ctx.Result.Metadata, &meta)
		if meta.Background {
			return m.renderBackgroundJob(params, meta, width)
		}
	}

	args := NewParamBuilder().
		Main(cmd).
		Flag("background", params.RunInBackground).
		Build()

	header := renderToolHeader(&m.ctx, "Bash", width, args...)

	if result, done := renderEarlyState(&m.ctx, header, width); done {
		return result
	}

	var meta tools.BashResponseMetadata
	unmarshalParams(m.ctx.Result.Metadata, &meta)

	output := meta.Output
	if output == "" && m.ctx.Result.Content != tools.BashNoOutput {
		output = m.ctx.Result.Content
	}

	if output == "" {
		return header
	}

	body := renderPlainContent(output, width-2, m.ctx.Styles, &m.toolItem)
	return joinHeaderBody(header, body, m.ctx.Styles)
}

func (m *BashToolItem) renderBackgroundJob(params tools.BashParams, meta tools.BashResponseMetadata, width int) string {
	description := cmp.Or(meta.Description, params.Command)
	header := renderJobHeader(&m.ctx, "Start", meta.ShellID, description, width)

	if m.ctx.IsNested {
		return header
	}

	if result, done := renderEarlyState(&m.ctx, header, width); done {
		return result
	}

	content := "Command: " + params.Command + "\n" + m.ctx.Result.Content
	body := renderPlainContent(content, width-2, m.ctx.Styles, &m.toolItem)
	return joinHeaderBody(header, body, m.ctx.Styles)
}

// JobOutputToolItem renders job output retrieval.
type JobOutputToolItem struct {
	toolItem
	ctx ToolCallContext
}

func NewJobOutputToolItem(ctx ToolCallContext) *JobOutputToolItem {
	return &JobOutputToolItem{
		toolItem: newToolItem(ctx),
		ctx:      ctx,
	}
}

func (m *JobOutputToolItem) Render(width int) string {
	var params tools.JobOutputParams
	unmarshalParams(m.ctx.Call.Input, &params)

	var meta tools.JobOutputResponseMetadata
	var description string
	if m.ctx.Result != nil && m.ctx.Result.Metadata != "" {
		unmarshalParams(m.ctx.Result.Metadata, &meta)
		description = cmp.Or(meta.Description, meta.Command)
	}

	header := renderJobHeader(&m.ctx, "Output", params.ShellID, description, width)

	if m.ctx.IsNested {
		return header
	}

	if result, done := renderEarlyState(&m.ctx, header, width); done {
		return result
	}

	body := renderPlainContent(m.ctx.Result.Content, width-2, m.ctx.Styles, &m.toolItem)
	return joinHeaderBody(header, body, m.ctx.Styles)
}

// JobKillToolItem renders job termination.
type JobKillToolItem struct {
	toolItem
	ctx ToolCallContext
}

func NewJobKillToolItem(ctx ToolCallContext) *JobKillToolItem {
	return &JobKillToolItem{
		toolItem: newToolItem(ctx),
		ctx:      ctx,
	}
}

func (m *JobKillToolItem) Render(width int) string {
	var params tools.JobKillParams
	unmarshalParams(m.ctx.Call.Input, &params)

	var meta tools.JobKillResponseMetadata
	var description string
	if m.ctx.Result != nil && m.ctx.Result.Metadata != "" {
		unmarshalParams(m.ctx.Result.Metadata, &meta)
		description = cmp.Or(meta.Description, meta.Command)
	}

	header := renderJobHeader(&m.ctx, "Kill", params.ShellID, description, width)

	if m.ctx.IsNested {
		return header
	}

	if result, done := renderEarlyState(&m.ctx, header, width); done {
		return result
	}

	body := renderPlainContent(m.ctx.Result.Content, width-2, m.ctx.Styles, &m.toolItem)
	return joinHeaderBody(header, body, m.ctx.Styles)
}

// renderJobHeader builds a job-specific header with action and PID.
func renderJobHeader(ctx *ToolCallContext, action, pid, description string, width int) string {
	sty := ctx.Styles
	icon := renderToolIcon(ctx.Status(), sty)

	jobPart := sty.Tool.JobToolName.Render("Job")
	actionPart := sty.Tool.JobAction.Render("(" + action + ")")
	pidPart := sty.Tool.JobPID.Render("PID " + pid)

	prefix := fmt.Sprintf("%s %s %s %s", icon, jobPart, actionPart, pidPart)

	if description == "" {
		return prefix
	}

	descPart := " " + sty.Tool.JobDescription.Render(description)
	fullHeader := prefix + descPart

	if lipgloss.Width(fullHeader) > width {
		availableWidth := width - lipgloss.Width(prefix) - 1
		if availableWidth < 10 {
			return prefix
		}
		descPart = " " + sty.Tool.JobDescription.Render(truncateText(description, availableWidth))
		fullHeader = prefix + descPart
	}

	return fullHeader
}

// -----------------------------------------------------------------------------
// File Tools
// -----------------------------------------------------------------------------

// ViewToolItem renders file viewing with syntax highlighting.
type ViewToolItem struct {
	toolItem
	ctx ToolCallContext
}

func NewViewToolItem(ctx ToolCallContext) *ViewToolItem {
	return &ViewToolItem{
		toolItem: newToolItem(ctx),
		ctx:      ctx,
	}
}

func (m *ViewToolItem) Render(width int) string {
	var params tools.ViewParams
	unmarshalParams(m.ctx.Call.Input, &params)

	file := fsext.PrettyPath(params.FilePath)
	args := NewParamBuilder().
		Main(file).
		KeyValue("limit", formatNonZero(params.Limit)).
		KeyValue("offset", formatNonZero(params.Offset)).
		Build()

	header := renderToolHeader(&m.ctx, "View", width, args...)

	if result, done := renderEarlyState(&m.ctx, header, width); done {
		return result
	}

	// Handle image content
	if m.ctx.Result.Data != "" && strings.HasPrefix(m.ctx.Result.MIMEType, "image/") {
		body := renderImageContent(m.ctx.Result.Data, m.ctx.Result.MIMEType, "", m.ctx.Styles)
		return joinHeaderBody(header, body, m.ctx.Styles)
	}

	var meta tools.ViewResponseMetadata
	unmarshalParams(m.ctx.Result.Metadata, &meta)

	body := renderCodeContent(meta.FilePath, meta.Content, params.Offset, width-2, m.ctx.Styles, &m.toolItem)
	return joinHeaderBody(header, body, m.ctx.Styles)
}

// EditToolItem renders file editing with diff visualization.
type EditToolItem struct {
	toolItem
	ctx ToolCallContext
}

func NewEditToolItem(ctx ToolCallContext) *EditToolItem {
	return &EditToolItem{
		toolItem: newToolItem(ctx),
		ctx:      ctx,
	}
}

func (m *EditToolItem) Render(width int) string {
	var params tools.EditParams
	unmarshalParams(m.ctx.Call.Input, &params)

	file := fsext.PrettyPath(params.FilePath)
	args := NewParamBuilder().Main(file).Build()

	header := renderToolHeader(&m.ctx, "Edit", width, args...)

	if result, done := renderEarlyState(&m.ctx, header, width); done {
		return result
	}

	var meta tools.EditResponseMetadata
	if err := unmarshalParams(m.ctx.Result.Metadata, &meta); err != nil {
		body := renderPlainContent(m.ctx.Result.Content, width-2, m.ctx.Styles, nil)
		return joinHeaderBody(header, body, m.ctx.Styles)
	}

	body := renderDiffContent(file, meta.OldContent, meta.NewContent, width-2, m.ctx.Styles, &m.toolItem)
	return joinHeaderBody(header, body, m.ctx.Styles)
}

// MultiEditToolItem renders multiple file edits with diff visualization.
type MultiEditToolItem struct {
	toolItem
	ctx ToolCallContext
}

func NewMultiEditToolItem(ctx ToolCallContext) *MultiEditToolItem {
	return &MultiEditToolItem{
		toolItem: newToolItem(ctx),
		ctx:      ctx,
	}
}

func (m *MultiEditToolItem) Render(width int) string {
	var params tools.MultiEditParams
	unmarshalParams(m.ctx.Call.Input, &params)

	file := fsext.PrettyPath(params.FilePath)
	args := NewParamBuilder().
		Main(file).
		KeyValue("edits", fmt.Sprintf("%d", len(params.Edits))).
		Build()

	header := renderToolHeader(&m.ctx, "Multi-Edit", width, args...)

	if result, done := renderEarlyState(&m.ctx, header, width); done {
		return result
	}

	var meta tools.MultiEditResponseMetadata
	if err := unmarshalParams(m.ctx.Result.Metadata, &meta); err != nil {
		body := renderPlainContent(m.ctx.Result.Content, width-2, m.ctx.Styles, nil)
		return joinHeaderBody(header, body, m.ctx.Styles)
	}

	body := renderDiffContent(file, meta.OldContent, meta.NewContent, width-2, m.ctx.Styles, &m.toolItem)

	// Add failed edits warning if any exist
	if len(meta.EditsFailed) > 0 {
		sty := m.ctx.Styles
		noteTag := sty.Tool.NoteTag.Render("Note")
		noteMsg := fmt.Sprintf("%d of %d edits succeeded", meta.EditsApplied, len(params.Edits))
		note := fmt.Sprintf("%s %s", noteTag, sty.Tool.NoteMessage.Render(noteMsg))
		body = lipgloss.JoinVertical(lipgloss.Left, body, "", note)
	}

	return joinHeaderBody(header, body, m.ctx.Styles)
}

// WriteToolItem renders file writing with syntax-highlighted content preview.
type WriteToolItem struct {
	toolItem
	ctx ToolCallContext
}

func NewWriteToolItem(ctx ToolCallContext) *WriteToolItem {
	return &WriteToolItem{
		toolItem: newToolItem(ctx),
		ctx:      ctx,
	}
}

func (m *WriteToolItem) Render(width int) string {
	var params tools.WriteParams
	unmarshalParams(m.ctx.Call.Input, &params)

	file := fsext.PrettyPath(params.FilePath)
	args := NewParamBuilder().Main(file).Build()

	header := renderToolHeader(&m.ctx, "Write", width, args...)

	if result, done := renderEarlyState(&m.ctx, header, width); done {
		return result
	}

	body := renderCodeContent(file, params.Content, 0, width-2, m.ctx.Styles, &m.toolItem)
	return joinHeaderBody(header, body, m.ctx.Styles)
}

// -----------------------------------------------------------------------------
// Search Tools
// -----------------------------------------------------------------------------

// GlobToolItem renders glob file pattern matching results.
type GlobToolItem struct {
	toolItem
	ctx ToolCallContext
}

func NewGlobToolItem(ctx ToolCallContext) *GlobToolItem {
	return &GlobToolItem{
		toolItem: newToolItem(ctx),
		ctx:      ctx,
	}
}

func (m *GlobToolItem) Render(width int) string {
	var params tools.GlobParams
	unmarshalParams(m.ctx.Call.Input, &params)

	args := NewParamBuilder().
		Main(params.Pattern).
		KeyValue("path", fsext.PrettyPath(params.Path)).
		Build()

	header := renderToolHeader(&m.ctx, "Glob", width, args...)

	if result, done := renderEarlyState(&m.ctx, header, width); done {
		return result
	}

	body := renderPlainContent(m.ctx.Result.Content, width-2, m.ctx.Styles, &m.toolItem)
	return joinHeaderBody(header, body, m.ctx.Styles)
}

// GrepToolItem renders grep content search results.
type GrepToolItem struct {
	toolItem
	ctx ToolCallContext
}

func NewGrepToolItem(ctx ToolCallContext) *GrepToolItem {
	return &GrepToolItem{
		toolItem: newToolItem(ctx),
		ctx:      ctx,
	}
}

func (m *GrepToolItem) Render(width int) string {
	var params tools.GrepParams
	unmarshalParams(m.ctx.Call.Input, &params)

	args := NewParamBuilder().
		Main(params.Pattern).
		KeyValue("path", fsext.PrettyPath(params.Path)).
		KeyValue("include", params.Include).
		Flag("literal", params.LiteralText).
		Build()

	header := renderToolHeader(&m.ctx, "Grep", width, args...)

	if result, done := renderEarlyState(&m.ctx, header, width); done {
		return result
	}

	body := renderPlainContent(m.ctx.Result.Content, width-2, m.ctx.Styles, &m.toolItem)
	return joinHeaderBody(header, body, m.ctx.Styles)
}

// LSToolItem renders directory listing results.
type LSToolItem struct {
	toolItem
	ctx ToolCallContext
}

func NewLSToolItem(ctx ToolCallContext) *LSToolItem {
	return &LSToolItem{
		toolItem: newToolItem(ctx),
		ctx:      ctx,
	}
}

func (m *LSToolItem) Render(width int) string {
	var params tools.LSParams
	unmarshalParams(m.ctx.Call.Input, &params)

	path := cmp.Or(params.Path, ".")
	path = fsext.PrettyPath(path)

	args := NewParamBuilder().Main(path).Build()
	header := renderToolHeader(&m.ctx, "List", width, args...)

	if result, done := renderEarlyState(&m.ctx, header, width); done {
		return result
	}

	body := renderPlainContent(m.ctx.Result.Content, width-2, m.ctx.Styles, &m.toolItem)
	return joinHeaderBody(header, body, m.ctx.Styles)
}

// SourcegraphToolItem renders code search results.
type SourcegraphToolItem struct {
	toolItem
	ctx ToolCallContext
}

func NewSourcegraphToolItem(ctx ToolCallContext) *SourcegraphToolItem {
	return &SourcegraphToolItem{
		toolItem: newToolItem(ctx),
		ctx:      ctx,
	}
}

func (m *SourcegraphToolItem) Render(width int) string {
	var params tools.SourcegraphParams
	unmarshalParams(m.ctx.Call.Input, &params)

	args := NewParamBuilder().
		Main(params.Query).
		KeyValue("count", formatNonZero(params.Count)).
		KeyValue("context", formatNonZero(params.ContextWindow)).
		Build()

	header := renderToolHeader(&m.ctx, "Sourcegraph", width, args...)

	if result, done := renderEarlyState(&m.ctx, header, width); done {
		return result
	}

	body := renderPlainContent(m.ctx.Result.Content, width-2, m.ctx.Styles, &m.toolItem)
	return joinHeaderBody(header, body, m.ctx.Styles)
}

// -----------------------------------------------------------------------------
// Fetch Tools
// -----------------------------------------------------------------------------

// FetchToolItem renders URL fetching with format-specific content display.
type FetchToolItem struct {
	toolItem
	ctx ToolCallContext
}

func NewFetchToolItem(ctx ToolCallContext) *FetchToolItem {
	return &FetchToolItem{
		toolItem: newToolItem(ctx),
		ctx:      ctx,
	}
}

func (m *FetchToolItem) Render(width int) string {
	var params tools.FetchParams
	unmarshalParams(m.ctx.Call.Input, &params)

	args := NewParamBuilder().
		Main(params.URL).
		KeyValue("format", params.Format).
		KeyValue("timeout", formatTimeout(params.Timeout)).
		Build()

	header := renderToolHeader(&m.ctx, "Fetch", width, args...)

	if result, done := renderEarlyState(&m.ctx, header, width); done {
		return result
	}

	// Use appropriate extension for syntax highlighting
	file := "fetch.md"
	switch params.Format {
	case "text":
		file = "fetch.txt"
	case "html":
		file = "fetch.html"
	}

	body := renderCodeContent(file, m.ctx.Result.Content, 0, width-2, m.ctx.Styles, &m.toolItem)
	return joinHeaderBody(header, body, m.ctx.Styles)
}

// AgenticFetchToolItem renders agentic URL fetching with nested tool calls.
type AgenticFetchToolItem struct {
	toolItem
	ctx ToolCallContext
}

func NewAgenticFetchToolItem(ctx ToolCallContext) *AgenticFetchToolItem {
	return &AgenticFetchToolItem{
		toolItem: newToolItem(ctx),
		ctx:      ctx,
	}
}

func (m *AgenticFetchToolItem) Render(width int) string {
	var params tools.AgenticFetchParams
	unmarshalParams(m.ctx.Call.Input, &params)

	var args []string
	if params.URL != "" {
		args = NewParamBuilder().Main(params.URL).Build()
	}

	header := renderToolHeader(&m.ctx, "Agentic Fetch", width, args...)

	// Render with nested tool calls tree
	body := renderAgentBody(&m.ctx, params.Prompt, "Prompt", header, width)
	return body
}

// WebFetchToolItem renders web page fetching.
type WebFetchToolItem struct {
	toolItem
	ctx ToolCallContext
}

func NewWebFetchToolItem(ctx ToolCallContext) *WebFetchToolItem {
	return &WebFetchToolItem{
		toolItem: newToolItem(ctx),
		ctx:      ctx,
	}
}

func (m *WebFetchToolItem) Render(width int) string {
	var params tools.WebFetchParams
	unmarshalParams(m.ctx.Call.Input, &params)

	args := NewParamBuilder().Main(params.URL).Build()
	header := renderToolHeader(&m.ctx, "Fetch", width, args...)

	if result, done := renderEarlyState(&m.ctx, header, width); done {
		return result
	}

	body := renderMarkdownContent(m.ctx.Result.Content, width-2, m.ctx.Styles, &m.toolItem)
	return joinHeaderBody(header, body, m.ctx.Styles)
}

// WebSearchToolItem renders web search results.
type WebSearchToolItem struct {
	toolItem
	ctx ToolCallContext
}

func NewWebSearchToolItem(ctx ToolCallContext) *WebSearchToolItem {
	return &WebSearchToolItem{
		toolItem: newToolItem(ctx),
		ctx:      ctx,
	}
}

func (m *WebSearchToolItem) Render(width int) string {
	var params tools.WebSearchParams
	unmarshalParams(m.ctx.Call.Input, &params)

	args := NewParamBuilder().Main(params.Query).Build()
	header := renderToolHeader(&m.ctx, "Search", width, args...)

	if result, done := renderEarlyState(&m.ctx, header, width); done {
		return result
	}

	body := renderMarkdownContent(m.ctx.Result.Content, width-2, m.ctx.Styles, &m.toolItem)
	return joinHeaderBody(header, body, m.ctx.Styles)
}

// DownloadToolItem renders file downloading.
type DownloadToolItem struct {
	toolItem
	ctx ToolCallContext
}

func NewDownloadToolItem(ctx ToolCallContext) *DownloadToolItem {
	return &DownloadToolItem{
		toolItem: newToolItem(ctx),
		ctx:      ctx,
	}
}

func (m *DownloadToolItem) Render(width int) string {
	var params tools.DownloadParams
	unmarshalParams(m.ctx.Call.Input, &params)

	args := NewParamBuilder().
		Main(params.URL).
		KeyValue("file_path", fsext.PrettyPath(params.FilePath)).
		KeyValue("timeout", formatTimeout(params.Timeout)).
		Build()

	header := renderToolHeader(&m.ctx, "Download", width, args...)

	if result, done := renderEarlyState(&m.ctx, header, width); done {
		return result
	}

	body := renderPlainContent(m.ctx.Result.Content, width-2, m.ctx.Styles, &m.toolItem)
	return joinHeaderBody(header, body, m.ctx.Styles)
}

// -----------------------------------------------------------------------------
// LSP Tools
// -----------------------------------------------------------------------------

// DiagnosticsToolItem renders project-wide diagnostic information.
type DiagnosticsToolItem struct {
	toolItem
	ctx ToolCallContext
}

func NewDiagnosticsToolItem(ctx ToolCallContext) *DiagnosticsToolItem {
	return &DiagnosticsToolItem{
		toolItem: newToolItem(ctx),
		ctx:      ctx,
	}
}

func (m *DiagnosticsToolItem) Render(width int) string {
	args := NewParamBuilder().Main("project").Build()
	header := renderToolHeader(&m.ctx, "Diagnostics", width, args...)

	if result, done := renderEarlyState(&m.ctx, header, width); done {
		return result
	}

	body := renderPlainContent(m.ctx.Result.Content, width-2, m.ctx.Styles, &m.toolItem)
	return joinHeaderBody(header, body, m.ctx.Styles)
}

// ReferencesToolItem renders LSP references search results.
type ReferencesToolItem struct {
	toolItem
	ctx ToolCallContext
}

func NewReferencesToolItem(ctx ToolCallContext) *ReferencesToolItem {
	return &ReferencesToolItem{
		toolItem: newToolItem(ctx),
		ctx:      ctx,
	}
}

func (m *ReferencesToolItem) Render(width int) string {
	var params tools.ReferencesParams
	unmarshalParams(m.ctx.Call.Input, &params)

	args := NewParamBuilder().
		Main(params.Symbol).
		KeyValue("path", fsext.PrettyPath(params.Path)).
		Build()

	header := renderToolHeader(&m.ctx, "References", width, args...)

	if result, done := renderEarlyState(&m.ctx, header, width); done {
		return result
	}

	body := renderPlainContent(m.ctx.Result.Content, width-2, m.ctx.Styles, &m.toolItem)
	return joinHeaderBody(header, body, m.ctx.Styles)
}

// -----------------------------------------------------------------------------
// Misc Tools
// -----------------------------------------------------------------------------

// TodosToolItem renders todo list management.
type TodosToolItem struct {
	toolItem
	ctx ToolCallContext
}

func NewTodosToolItem(ctx ToolCallContext) *TodosToolItem {
	return &TodosToolItem{
		toolItem: newToolItem(ctx),
		ctx:      ctx,
	}
}

func (m *TodosToolItem) Render(width int) string {
	sty := m.ctx.Styles
	var params tools.TodosParams
	var meta tools.TodosResponseMetadata
	var headerText string
	var body string

	// Parse params for pending state
	if err := unmarshalParams(m.ctx.Call.Input, &params); err == nil {
		completedCount := 0
		inProgressTask := ""
		for _, todo := range params.Todos {
			if todo.Status == "completed" {
				completedCount++
			}
			if todo.Status == "in_progress" {
				inProgressTask = cmp.Or(todo.ActiveForm, todo.Content)
			}
		}

		// Default display from params
		ratio := sty.Tool.JobAction.Render(fmt.Sprintf("%d/%d", completedCount, len(params.Todos)))
		headerText = ratio
		if inProgressTask != "" {
			headerText = fmt.Sprintf("%s · %s", ratio, inProgressTask)
		}

		// If we have metadata, use it for richer display
		if m.ctx.Result != nil && m.ctx.Result.Metadata != "" {
			if err := unmarshalParams(m.ctx.Result.Metadata, &meta); err == nil {
				headerText, body = m.formatTodosFromMeta(meta, width)
			}
		}
	}

	args := NewParamBuilder().Main(headerText).Build()
	header := renderToolHeader(&m.ctx, "To-Do", width, args...)

	if result, done := renderEarlyState(&m.ctx, header, width); done {
		return result
	}

	if body == "" {
		return header
	}
	return joinHeaderBody(header, body, m.ctx.Styles)
}

func (m *TodosToolItem) formatTodosFromMeta(meta tools.TodosResponseMetadata, width int) (string, string) {
	sty := m.ctx.Styles
	var headerText, body string

	if meta.IsNew {
		if meta.JustStarted != "" {
			headerText = fmt.Sprintf("created %d todos, starting first", meta.Total)
		} else {
			headerText = fmt.Sprintf("created %d todos", meta.Total)
		}
		body = formatTodosList(meta.Todos, width, sty)
	} else {
		hasCompleted := len(meta.JustCompleted) > 0
		hasStarted := meta.JustStarted != ""
		allCompleted := meta.Completed == meta.Total

		ratio := sty.Tool.JobAction.Render(fmt.Sprintf("%d/%d", meta.Completed, meta.Total))
		if hasCompleted && hasStarted {
			text := sty.Tool.JobDescription.Render(fmt.Sprintf(" · completed %d, starting next", len(meta.JustCompleted)))
			headerText = ratio + text
		} else if hasCompleted {
			text := " · completed all"
			if !allCompleted {
				text = fmt.Sprintf(" · completed %d", len(meta.JustCompleted))
			}
			headerText = ratio + sty.Tool.JobDescription.Render(text)
		} else if hasStarted {
			headerText = ratio + sty.Tool.JobDescription.Render(" · starting task")
		} else {
			headerText = ratio
		}

		if allCompleted {
			body = formatTodosList(meta.Todos, width, sty)
		} else if meta.JustStarted != "" {
			body = sty.Tool.IconSuccess.String() + " " + sty.Base.Render(meta.JustStarted)
		}
	}

	return headerText, body
}

// AgentToolItem renders agent task execution with nested tool calls.
type AgentToolItem struct {
	toolItem
	ctx ToolCallContext
}

func NewAgentToolItem(ctx ToolCallContext) *AgentToolItem {
	return &AgentToolItem{
		toolItem: newToolItem(ctx),
		ctx:      ctx,
	}
}

func (m *AgentToolItem) Render(width int) string {
	var params agent.AgentParams
	unmarshalParams(m.ctx.Call.Input, &params)

	header := renderToolHeader(&m.ctx, "Agent", width)
	body := renderAgentBody(&m.ctx, params.Prompt, "Task", header, width)
	return body
}

// renderAgentBody renders agent/agentic_fetch body with prompt tag and nested calls tree.
func renderAgentBody(ctx *ToolCallContext, prompt, tagLabel, header string, width int) string {
	sty := ctx.Styles

	if ctx.Cancelled {
		if result, done := renderEarlyState(ctx, header, width); done {
			return result
		}
	}

	// Build prompt tag
	prompt = strings.ReplaceAll(prompt, "\n", " ")
	taskTag := sty.Tool.AgentTaskTag.Render(tagLabel)
	tagWidth := lipgloss.Width(taskTag)
	remainingWidth := min(width-tagWidth-2, 120-tagWidth-2)
	promptStyled := sty.Tool.AgentPrompt.Width(remainingWidth).Render(prompt)

	headerWithPrompt := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		lipgloss.JoinHorizontal(lipgloss.Left, taskTag, " ", promptStyled),
	)

	// Build tree with nested tool calls
	childTools := tree.Root(headerWithPrompt)
	for _, nestedCtx := range ctx.NestedCalls {
		nestedCtx.IsNested = true
		nestedItem := NewToolItem(nestedCtx)
		childTools.Child(nestedItem.Render(remainingWidth))
	}

	parts := []string{
		childTools.Enumerator(roundedEnumerator(2, tagWidth-5)).String(),
	}

	// Add pending indicator if not complete
	if !ctx.HasResult() {
		parts = append(parts, "", sty.Tool.StateWaiting.Render("Working..."))
	}

	treeOutput := lipgloss.JoinVertical(lipgloss.Left, parts...)

	if !ctx.HasResult() {
		return treeOutput
	}

	body := renderMarkdownContent(ctx.Result.Content, width-2, sty, nil)
	return joinHeaderBody(treeOutput, body, sty)
}

// roundedEnumerator creates a tree enumerator with rounded connectors.
func roundedEnumerator(lPadding, lineWidth int) tree.Enumerator {
	if lineWidth == 0 {
		lineWidth = 2
	}
	if lPadding == 0 {
		lPadding = 1
	}
	return func(children tree.Children, index int) string {
		line := strings.Repeat("─", lineWidth)
		padding := strings.Repeat(" ", lPadding)
		if children.Length()-1 == index {
			return padding + "╰" + line
		}
		return padding + "├" + line
	}
}

// GenericToolItem renders unknown tool types with basic parameter display.
type GenericToolItem struct {
	toolItem
	ctx ToolCallContext
}

func NewGenericToolItem(ctx ToolCallContext) *GenericToolItem {
	return &GenericToolItem{
		toolItem: newToolItem(ctx),
		ctx:      ctx,
	}
}

func (m *GenericToolItem) Render(width int) string {
	name := prettifyToolName(m.ctx.Call.Name)

	// Handle media content
	if m.ctx.Result != nil && m.ctx.Result.Data != "" {
		if strings.HasPrefix(m.ctx.Result.MIMEType, "image/") {
			args := NewParamBuilder().Main(m.ctx.Call.Input).Build()
			header := renderToolHeader(&m.ctx, name, width, args...)
			body := renderImageContent(m.ctx.Result.Data, m.ctx.Result.MIMEType, m.ctx.Result.Content, m.ctx.Styles)
			return joinHeaderBody(header, body, m.ctx.Styles)
		}
		args := NewParamBuilder().Main(m.ctx.Call.Input).Build()
		header := renderToolHeader(&m.ctx, name, width, args...)
		body := renderMediaContent(m.ctx.Result.MIMEType, m.ctx.Result.Content, m.ctx.Styles)
		return joinHeaderBody(header, body, m.ctx.Styles)
	}

	args := NewParamBuilder().Main(m.ctx.Call.Input).Build()
	header := renderToolHeader(&m.ctx, name, width, args...)

	if result, done := renderEarlyState(&m.ctx, header, width); done {
		return result
	}

	if m.ctx.Result == nil || m.ctx.Result.Content == "" {
		return header
	}

	body := renderPlainContent(m.ctx.Result.Content, width-2, m.ctx.Styles, &m.toolItem)
	return joinHeaderBody(header, body, m.ctx.Styles)
}

// -----------------------------------------------------------------------------
// Helper Functions
// -----------------------------------------------------------------------------

// prettifyToolName converts tool names to display-friendly format.
func prettifyToolName(name string) string {
	switch name {
	case agent.AgentToolName:
		return "Agent"
	case tools.BashToolName:
		return "Bash"
	case tools.JobOutputToolName:
		return "Job: Output"
	case tools.JobKillToolName:
		return "Job: Kill"
	case tools.DownloadToolName:
		return "Download"
	case tools.EditToolName:
		return "Edit"
	case tools.MultiEditToolName:
		return "Multi-Edit"
	case tools.FetchToolName:
		return "Fetch"
	case tools.AgenticFetchToolName:
		return "Agentic Fetch"
	case tools.WebFetchToolName:
		return "Fetch"
	case tools.WebSearchToolName:
		return "Search"
	case tools.GlobToolName:
		return "Glob"
	case tools.GrepToolName:
		return "Grep"
	case tools.LSToolName:
		return "List"
	case tools.SourcegraphToolName:
		return "Sourcegraph"
	case tools.TodosToolName:
		return "To-Do"
	case tools.ViewToolName:
		return "View"
	case tools.WriteToolName:
		return "Write"
	case tools.DiagnosticsToolName:
		return "Diagnostics"
	case tools.ReferencesToolName:
		return "References"
	default:
		// Handle MCP tools and others
		name = strings.TrimPrefix(name, "mcp_")
		if name == "" {
			return "Tool"
		}
		return strings.ToUpper(name[:1]) + name[1:]
	}
}

// formatTimeout converts timeout seconds to duration string.
func formatTimeout(timeout int) string {
	if timeout == 0 {
		return ""
	}
	return (time.Duration(timeout) * time.Second).String()
}

// truncateText truncates text to fit within width with ellipsis.
func truncateText(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	for i := len(s) - 1; i >= 0; i-- {
		truncated := s[:i] + "…"
		if lipgloss.Width(truncated) <= width {
			return truncated
		}
	}
	return "…"
}
