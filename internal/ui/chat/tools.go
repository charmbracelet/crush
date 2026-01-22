package chat

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/tree"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/anim"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

// responseContextHeight limits the number of lines displayed in tool output.
const responseContextHeight = 10

// toolBodyLeftPaddingTotal represents the padding that should be applied to each tool body
const toolBodyLeftPaddingTotal = 2

// ToolStatus represents the current state of a tool call.
type ToolStatus int

const (
	ToolStatusAwaitingPermission ToolStatus = iota
	ToolStatusRunning
	ToolStatusSuccess
	ToolStatusError
	ToolStatusCanceled
)

// ToolMessageItem represents a tool call message in the chat UI.
type ToolMessageItem interface {
	MessageItem

	ToolCall() message.ToolCall
	SetToolCall(tc message.ToolCall)
	SetResult(res *message.ToolResult)
	MessageID() string
	SetMessageID(id string)
	SetStatus(status ToolStatus)
	Status() ToolStatus
}

// Compactable is an interface for tool items that can render in a compacted mode.
// When compact mode is enabled, tools render as a compact single-line header.
type Compactable interface {
	SetCompact(compact bool)
}

// SpinningState contains the state passed to SpinningFunc for custom spinning logic.
type SpinningState struct {
	ToolCall message.ToolCall
	Result   *message.ToolResult
	Status   ToolStatus
}

// IsCanceled returns true if the tool status is canceled.
func (s *SpinningState) IsCanceled() bool {
	return s.Status == ToolStatusCanceled
}

// HasResult returns true if the result is not nil.
func (s *SpinningState) HasResult() bool {
	return s.Result != nil
}

// SpinningFunc is a function type for custom spinning logic.
// Returns true if the tool should show the spinning animation.
type SpinningFunc func(state SpinningState) bool

// DefaultToolRenderContext implements the default [ToolRenderer] interface.
type DefaultToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (d *DefaultToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	return "TODO: Implement Tool Renderer For: " + opts.ToolCall.Name
}

// ToolRenderOpts contains the data needed to render a tool call.
type ToolRenderOpts struct {
	ToolCall        message.ToolCall
	Result          *message.ToolResult
	Anim            *anim.Anim
	ExpandedContent bool
	Compact         bool
	IsSpinning      bool
	Status          ToolStatus
}

// IsPending returns true if the tool call is still pending (not finished and
// not canceled).
func (o *ToolRenderOpts) IsPending() bool {
	return !o.ToolCall.Finished && !o.IsCanceled()
}

// IsCanceled returns true if the tool status is canceled.
func (o *ToolRenderOpts) IsCanceled() bool {
	return o.Status == ToolStatusCanceled
}

// HasResult returns true if the result is not nil.
func (o *ToolRenderOpts) HasResult() bool {
	return o.Result != nil
}

// HasEmptyResult returns true if the result is nil or has empty content.
func (o *ToolRenderOpts) HasEmptyResult() bool {
	return o.Result == nil || o.Result.Content == ""
}

// ToolRenderer represents an interface for rendering tool calls.
type ToolRenderer interface {
	RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string
}

// ToolRendererFunc is a function type that implements the [ToolRenderer] interface.
type ToolRendererFunc func(sty *styles.Styles, width int, opts *ToolRenderOpts) string

// RenderTool implements the ToolRenderer interface.
func (f ToolRendererFunc) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	return f(sty, width, opts)
}

// baseToolMessageItem represents a tool call message that can be displayed in the UI.
type baseToolMessageItem struct {
	*highlightableMessageItem
	*cachedMessageItem
	*focusableMessageItem

	toolRenderer ToolRenderer
	toolCall     message.ToolCall
	result       *message.ToolResult
	messageID    string
	status       ToolStatus
	// we use this so we can efficiently cache
	// tools that have a capped width (e.x bash.. and others)
	hasCappedWidth bool
	// isCompact indicates this tool should render in compact mode.
	isCompact bool
	// spinningFunc allows tools to override the default spinning logic.
	// If nil, uses the default: !toolCall.Finished && !canceled.
	spinningFunc SpinningFunc

	sty             *styles.Styles
	anim            *anim.Anim
	expandedContent bool
}

// newBaseToolMessageItem is the internal constructor for base tool message items.
func newBaseToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	toolRenderer ToolRenderer,
	canceled bool,
) *baseToolMessageItem {
	// we only do full width for diffs (as far as I know)
	hasCappedWidth := toolCall.Name != tools.EditToolName && toolCall.Name != tools.MultiEditToolName

	status := ToolStatusRunning
	if canceled {
		status = ToolStatusCanceled
	}

	t := &baseToolMessageItem{
		highlightableMessageItem: defaultHighlighter(sty),
		cachedMessageItem:        &cachedMessageItem{},
		focusableMessageItem:     &focusableMessageItem{},
		sty:                      sty,
		toolRenderer:             toolRenderer,
		toolCall:                 toolCall,
		result:                   result,
		status:                   status,
		hasCappedWidth:           hasCappedWidth,
	}
	t.anim = anim.New(anim.Settings{
		ID:          toolCall.ID,
		Size:        15,
		GradColorA:  sty.Primary,
		GradColorB:  sty.Secondary,
		LabelColor:  sty.FgBase,
		CycleColors: true,
	})

	return t
}

// NewToolMessageItem creates a new [ToolMessageItem] based on the tool call name.
//
// It returns a specific tool message item type if implemented, otherwise it
// returns a generic tool message item. The messageID is the ID of the assistant
// message containing this tool call.
func NewToolMessageItem(
	sty *styles.Styles,
	messageID string,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	var item ToolMessageItem
	switch toolCall.Name {
	case tools.BashToolName:
		item = NewBashToolMessageItem(sty, toolCall, result, canceled)
	case tools.JobOutputToolName:
		item = NewJobOutputToolMessageItem(sty, toolCall, result, canceled)
	case tools.JobKillToolName:
		item = NewJobKillToolMessageItem(sty, toolCall, result, canceled)
	case tools.ViewToolName:
		item = NewViewToolMessageItem(sty, toolCall, result, canceled)
	case tools.WriteToolName:
		item = NewWriteToolMessageItem(sty, toolCall, result, canceled)
	case tools.EditToolName:
		item = NewEditToolMessageItem(sty, toolCall, result, canceled)
	case tools.MultiEditToolName:
		item = NewMultiEditToolMessageItem(sty, toolCall, result, canceled)
	case tools.GlobToolName:
		item = NewGlobToolMessageItem(sty, toolCall, result, canceled)
	case tools.GrepToolName:
		item = NewGrepToolMessageItem(sty, toolCall, result, canceled)
	case tools.LSToolName:
		item = NewLSToolMessageItem(sty, toolCall, result, canceled)
	case tools.DownloadToolName:
		item = NewDownloadToolMessageItem(sty, toolCall, result, canceled)
	case tools.FetchToolName:
		item = NewFetchToolMessageItem(sty, toolCall, result, canceled)
	case tools.SourcegraphToolName:
		item = NewSourcegraphToolMessageItem(sty, toolCall, result, canceled)
	case tools.DiagnosticsToolName:
		item = NewDiagnosticsToolMessageItem(sty, toolCall, result, canceled)
	case agent.AgentToolName:
		item = NewAgentToolMessageItem(sty, toolCall, result, canceled)
	case tools.AgenticFetchToolName:
		item = NewAgenticFetchToolMessageItem(sty, toolCall, result, canceled)
	case tools.WebFetchToolName:
		item = NewWebFetchToolMessageItem(sty, toolCall, result, canceled)
	case tools.WebSearchToolName:
		item = NewWebSearchToolMessageItem(sty, toolCall, result, canceled)
	case tools.TodosToolName:
		item = NewTodosToolMessageItem(sty, toolCall, result, canceled)
	default:
		if strings.HasPrefix(toolCall.Name, "mcp_") {
			item = NewMCPToolMessageItem(sty, toolCall, result, canceled)
		} else {
			// TODO: Implement other tool items
			item = newBaseToolMessageItem(
				sty,
				toolCall,
				result,
				&DefaultToolRenderContext{},
				canceled,
			)
		}
	}
	item.SetMessageID(messageID)
	return item
}

// SetCompact implements the Compactable interface.
func (t *baseToolMessageItem) SetCompact(compact bool) {
	t.isCompact = compact
	t.clearCache()
}

// ID returns the unique identifier for this tool message item.
func (t *baseToolMessageItem) ID() string {
	return t.toolCall.ID
}

// StartAnimation starts the assistant message animation if it should be spinning.
func (t *baseToolMessageItem) StartAnimation() tea.Cmd {
	if !t.isSpinning() {
		return nil
	}
	return t.anim.Start()
}

// Animate progresses the assistant message animation if it should be spinning.
func (t *baseToolMessageItem) Animate(msg anim.StepMsg) tea.Cmd {
	if !t.isSpinning() {
		return nil
	}
	return t.anim.Animate(msg)
}

// RawRender implements [MessageItem].
func (t *baseToolMessageItem) RawRender(width int) string {
	toolItemWidth := width - messageLeftPaddingTotal
	if t.hasCappedWidth {
		toolItemWidth = cappedMessageWidth(width)
	}

	content, height, ok := t.getCachedRender(toolItemWidth)
	// if we are spinning or there is no cache rerender
	if !ok || t.isSpinning() {
		content = t.toolRenderer.RenderTool(t.sty, toolItemWidth, &ToolRenderOpts{
			ToolCall:        t.toolCall,
			Result:          t.result,
			Anim:            t.anim,
			ExpandedContent: t.expandedContent,
			Compact:         t.isCompact,
			IsSpinning:      t.isSpinning(),
			Status:          t.computeStatus(),
		})
		height = lipgloss.Height(content)
		// cache the rendered content
		t.setCachedRender(content, toolItemWidth, height)
	}

	return t.renderHighlighted(content, toolItemWidth, height)
}

// Render renders the tool message item at the given width.
func (t *baseToolMessageItem) Render(width int) string {
	style := t.sty.Chat.Message.ToolCallBlurred
	if t.focused {
		style = t.sty.Chat.Message.ToolCallFocused
	}

	if t.isCompact {
		style = t.sty.Chat.Message.ToolCallCompact
	}

	return style.Render(t.RawRender(width))
}

// ToolCall returns the tool call associated with this message item.
func (t *baseToolMessageItem) ToolCall() message.ToolCall {
	return t.toolCall
}

// SetToolCall sets the tool call associated with this message item.
func (t *baseToolMessageItem) SetToolCall(tc message.ToolCall) {
	t.toolCall = tc
	t.clearCache()
}

// SetResult sets the tool result associated with this message item.
func (t *baseToolMessageItem) SetResult(res *message.ToolResult) {
	t.result = res
	t.clearCache()
}

// MessageID returns the ID of the message containing this tool call.
func (t *baseToolMessageItem) MessageID() string {
	return t.messageID
}

// SetMessageID sets the ID of the message containing this tool call.
func (t *baseToolMessageItem) SetMessageID(id string) {
	t.messageID = id
}

// SetStatus sets the tool status.
func (t *baseToolMessageItem) SetStatus(status ToolStatus) {
	t.status = status
	t.clearCache()
}

// Status returns the current tool status.
func (t *baseToolMessageItem) Status() ToolStatus {
	return t.status
}

// computeStatus computes the effective status considering the result.
func (t *baseToolMessageItem) computeStatus() ToolStatus {
	if t.result != nil {
		if t.result.IsError {
			return ToolStatusError
		}
		return ToolStatusSuccess
	}
	return t.status
}

// isSpinning returns true if the tool should show animation.
func (t *baseToolMessageItem) isSpinning() bool {
	if t.spinningFunc != nil {
		return t.spinningFunc(SpinningState{
			ToolCall: t.toolCall,
			Result:   t.result,
			Status:   t.status,
		})
	}
	return !t.toolCall.Finished && t.status != ToolStatusCanceled
}

// SetSpinningFunc sets a custom function to determine if the tool should spin.
func (t *baseToolMessageItem) SetSpinningFunc(fn SpinningFunc) {
	t.spinningFunc = fn
}

// ToggleExpanded toggles the expanded state of the thinking box.
func (t *baseToolMessageItem) ToggleExpanded() {
	t.expandedContent = !t.expandedContent
	t.clearCache()
}

// HandleMouseClick implements MouseClickable.
func (t *baseToolMessageItem) HandleMouseClick(btn ansi.MouseButton, x, y int) bool {
	if btn != ansi.MouseLeft {
		return false
	}
	t.ToggleExpanded()
	return true
}

// pendingTool renders a tool that is still in progress with an animation.
func pendingTool(sty *styles.Styles, name string, anim *anim.Anim) string {
	icon := sty.Tool.IconPending.Render()
	toolName := sty.Tool.NameNormal.Render(name)

	var animView string
	if anim != nil {
		animView = anim.Render()
	}

	return fmt.Sprintf("%s %s %s", icon, toolName, animView)
}

// toolEarlyStateContent handles error/cancelled/pending states before content rendering.
// Returns the rendered output and true if early state was handled.
func toolEarlyStateContent(sty *styles.Styles, opts *ToolRenderOpts, width int) (string, bool) {
	var msg string
	switch opts.Status {
	case ToolStatusError:
		msg = toolErrorContent(sty, opts.Result, width)
	case ToolStatusCanceled:
		msg = sty.Tool.StateCancelled.Render("Canceled.")
	case ToolStatusAwaitingPermission:
		msg = sty.Tool.StateWaiting.Render("Requesting permission...")
	case ToolStatusRunning:
		msg = sty.Tool.StateWaiting.Render("Waiting for tool response...")
	default:
		return "", false
	}
	return msg, true
}

// toolErrorContent formats an error message with ERROR tag.
func toolErrorContent(sty *styles.Styles, result *message.ToolResult, width int) string {
	if result == nil {
		return ""
	}
	errContent := strings.ReplaceAll(result.Content, "\n", " ")
	errTag := sty.Tool.ErrorTag.Render("ERROR")
	tagWidth := lipgloss.Width(errTag)
	errContent = ansi.Truncate(errContent, width-tagWidth-3, "…")
	return fmt.Sprintf("%s %s", errTag, sty.Tool.ErrorMessage.Render(errContent))
}

// toolIcon returns the status icon for a tool call.
// toolIcon returns the status icon for a tool call based on its status.
func toolIcon(sty *styles.Styles, status ToolStatus) string {
	switch status {
	case ToolStatusSuccess:
		return sty.Tool.IconSuccess.String()
	case ToolStatusError:
		return sty.Tool.IconError.String()
	case ToolStatusCanceled:
		return sty.Tool.IconCancelled.String()
	default:
		return sty.Tool.IconPending.String()
	}
}

// toolParamList formats parameters as "main (key=value, ...)" with truncation.
// toolParamList formats tool parameters as "main (key=value, ...)" with truncation.
func toolParamList(sty *styles.Styles, params []string, width int) string {
	// minSpaceForMainParam is the min space required for the main param
	// if this is less that the value set we will only show the main param nothing else
	const minSpaceForMainParam = 30
	if len(params) == 0 {
		return ""
	}

	mainParam := params[0]

	// Build key=value pairs from remaining params (consecutive key, value pairs).
	var kvPairs []string
	for i := 1; i+1 < len(params); i += 2 {
		if params[i+1] != "" {
			kvPairs = append(kvPairs, fmt.Sprintf("%s=%s", params[i], params[i+1]))
		}
	}

	// Try to include key=value pairs if there's enough space.
	output := mainParam
	if len(kvPairs) > 0 {
		partsStr := strings.Join(kvPairs, ", ")
		if remaining := width - lipgloss.Width(partsStr) - 3; remaining >= minSpaceForMainParam {
			output = fmt.Sprintf("%s (%s)", mainParam, partsStr)
		}
	}

	if width >= 0 {
		output = ansi.Truncate(output, width, "…")
	}
	return sty.Tool.ParamMain.Render(output)
}

// toolHeader builds the tool header line: "● ToolName params..."
func toolHeader(sty *styles.Styles, status ToolStatus, name string, width int, nested bool, params ...string) string {
	icon := toolIcon(sty, status)
	nameStyle := sty.Tool.NameNormal
	if nested {
		nameStyle = sty.Tool.NameNested
	}
	toolName := nameStyle.Render(name)
	prefix := fmt.Sprintf("%s %s ", icon, toolName)
	prefixWidth := lipgloss.Width(prefix)
	remainingWidth := width - prefixWidth
	paramsStr := toolParamList(sty, params, remainingWidth)
	return prefix + paramsStr
}

// toolOutputPlainContent renders plain text with optional expansion support.
func toolOutputPlainContent(sty *styles.Styles, content string, width int, expanded bool) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\t", "    ")
	content = strings.TrimSpace(content)
	lines := strings.Split(content, "\n")

	maxLines := responseContextHeight
	if expanded {
		maxLines = len(lines) // Show all
	}

	var out []string
	for i, ln := range lines {
		if i >= maxLines {
			break
		}
		ln = " " + ln
		if lipgloss.Width(ln) > width {
			ln = ansi.Truncate(ln, width, "…")
		}
		out = append(out, sty.Tool.ContentLine.Width(width).Render(ln))
	}

	wasTruncated := len(lines) > responseContextHeight

	if !expanded && wasTruncated {
		out = append(out, sty.Tool.ContentTruncation.
			Width(width).
			Render(fmt.Sprintf(assistantMessageTruncateFormat, len(lines)-responseContextHeight)))
	}

	return strings.Join(out, "\n")
}

// toolOutputCodeContent renders code with syntax highlighting and line numbers.
func toolOutputCodeContent(sty *styles.Styles, path, content string, offset, width int, expanded bool) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\t", "    ")

	lines := strings.Split(content, "\n")
	maxLines := responseContextHeight
	if expanded {
		maxLines = len(lines)
	}

	// Truncate if needed.
	displayLines := lines
	if len(lines) > maxLines {
		displayLines = lines[:maxLines]
	}

	bg := sty.Tool.ContentCodeBg
	highlighted, _ := common.SyntaxHighlight(sty, strings.Join(displayLines, "\n"), path, bg)
	highlightedLines := strings.Split(highlighted, "\n")

	// Calculate line number width.
	maxLineNumber := len(displayLines) + offset
	maxDigits := getDigits(maxLineNumber)
	numFmt := fmt.Sprintf("%%%dd", maxDigits)

	bodyWidth := width - toolBodyLeftPaddingTotal
	codeWidth := bodyWidth - maxDigits - 4 // -4 for line number padding

	var out []string
	for i, ln := range highlightedLines {
		lineNum := sty.Tool.ContentLineNumber.Render(fmt.Sprintf(numFmt, i+1+offset))

		if lipgloss.Width(ln) > codeWidth {
			ln = ansi.Truncate(ln, codeWidth, "…")
		}

		codeLine := sty.Tool.ContentCodeLine.
			Width(codeWidth).
			PaddingLeft(2).
			Render(ln)

		out = append(out, lipgloss.JoinHorizontal(lipgloss.Left, lineNum, codeLine))
	}

	// Add truncation message if needed.
	if len(lines) > maxLines && !expanded {
		out = append(out, sty.Tool.ContentCodeTruncation.
			Width(bodyWidth).
			Render(fmt.Sprintf(assistantMessageTruncateFormat, len(lines)-maxLines)),
		)
	}

	return sty.Tool.Body.Render(strings.Join(out, "\n"))
}

// toolOutputImageContent renders image data with size info.
func toolOutputImageContent(sty *styles.Styles, data, mediaType string) string {
	dataSize := len(data) * 3 / 4
	sizeStr := formatSize(dataSize)

	loaded := sty.Base.Foreground(sty.Green).Render("Loaded")
	arrow := sty.Base.Foreground(sty.GreenDark).Render("→")
	typeStyled := sty.Base.Render(mediaType)
	sizeStyled := sty.Subtle.Render(sizeStr)

	return sty.Tool.Body.Render(fmt.Sprintf("%s %s %s %s", loaded, arrow, typeStyled, sizeStyled))
}

// getDigits returns the number of digits in a number.
func getDigits(n int) int {
	if n == 0 {
		return 1
	}
	if n < 0 {
		n = -n
	}
	digits := 0
	for n > 0 {
		n /= 10
		digits++
	}
	return digits
}

// formatSize formats byte size into human readable format.
func formatSize(bytes int) string {
	const (
		kb = 1024
		mb = kb * 1024
	)
	switch {
	case bytes >= mb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// toolOutputDiffContent renders a diff between old and new content.
func toolOutputDiffContent(sty *styles.Styles, file, oldContent, newContent string, width int, expanded bool) string {
	bodyWidth := width - toolBodyLeftPaddingTotal

	formatter := common.DiffFormatter(sty).
		Before(file, oldContent).
		After(file, newContent).
		Width(bodyWidth)

	// Use split view for wide terminals.
	if width > maxTextWidth {
		formatter = formatter.Split()
	}

	formatted := formatter.String()
	lines := strings.Split(formatted, "\n")

	// Truncate if needed.
	maxLines := responseContextHeight
	if expanded {
		maxLines = len(lines)
	}

	if len(lines) > maxLines && !expanded {
		truncMsg := sty.Tool.DiffTruncation.
			Width(bodyWidth).
			Render(fmt.Sprintf(assistantMessageTruncateFormat, len(lines)-maxLines))
		formatted = truncMsg + "\n" + strings.Join(lines[:maxLines], "\n")
	}

	return sty.Tool.Body.Render(formatted)
}

// formatTimeout converts timeout seconds to a duration string (e.g., "30s").
// Returns empty string if timeout is 0.
func formatTimeout(timeout int) string {
	if timeout == 0 {
		return ""
	}
	return fmt.Sprintf("%ds", timeout)
}

// formatNonZero returns string representation of non-zero integers, empty string for zero.
func formatNonZero(value int) string {
	if value == 0 {
		return ""
	}
	return fmt.Sprintf("%d", value)
}

// toolOutputMultiEditDiffContent renders a diff with optional failed edits note.
func toolOutputMultiEditDiffContent(sty *styles.Styles, file string, meta tools.MultiEditResponseMetadata, totalEdits, width int, expanded bool) string {
	bodyWidth := width - toolBodyLeftPaddingTotal

	formatter := common.DiffFormatter(sty).
		Before(file, meta.OldContent).
		After(file, meta.NewContent).
		Width(bodyWidth)

	// Use split view for wide terminals.
	if width > maxTextWidth {
		formatter = formatter.Split()
	}

	formatted := formatter.String()
	lines := strings.Split(formatted, "\n")

	// Truncate if needed.
	maxLines := responseContextHeight
	if expanded {
		maxLines = len(lines)
	}

	if len(lines) > maxLines && !expanded {
		truncMsg := sty.Tool.DiffTruncation.
			Width(bodyWidth).
			Render(fmt.Sprintf(assistantMessageTruncateFormat, len(lines)-maxLines))
		formatted = truncMsg + "\n" + strings.Join(lines[:maxLines], "\n")
	}

	// Add failed edits note if any exist.
	if len(meta.EditsFailed) > 0 {
		noteTag := sty.Tool.NoteTag.Render("Note")
		noteMsg := fmt.Sprintf("%d of %d edits succeeded", meta.EditsApplied, totalEdits)
		note := fmt.Sprintf("%s %s", noteTag, sty.Tool.NoteMessage.Render(noteMsg))
		formatted = formatted + "\n\n" + note
	}

	return sty.Tool.Body.Render(formatted)
}

// roundedEnumerator creates a tree enumerator with rounded corners.
func roundedEnumerator(lPadding, width int) tree.Enumerator {
	if width == 0 {
		width = 2
	}
	if lPadding == 0 {
		lPadding = 1
	}
	return func(children tree.Children, index int) string {
		line := strings.Repeat("─", width)
		padding := strings.Repeat(" ", lPadding)
		if children.Length()-1 == index {
			return padding + "╰" + line
		}
		return padding + "├" + line
	}
}

// toolOutputMarkdownContent renders markdown content with optional truncation.
func toolOutputMarkdownContent(sty *styles.Styles, content string, width int, expanded bool) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\t", "    ")
	content = strings.TrimSpace(content)

	// Cap width for readability.
	if width > maxTextWidth {
		width = maxTextWidth
	}

	renderer := common.PlainMarkdownRenderer(sty, width)
	rendered, err := renderer.Render(content)
	if err != nil {
		return toolOutputPlainContent(sty, content, width, expanded)
	}

	lines := strings.Split(rendered, "\n")
	maxLines := responseContextHeight
	if expanded {
		maxLines = len(lines)
	}

	var out []string
	for i, ln := range lines {
		if i >= maxLines {
			break
		}
		out = append(out, ln)
	}

	if len(lines) > maxLines && !expanded {
		out = append(out, sty.Tool.ContentTruncation.
			Width(width).
			Render(fmt.Sprintf(assistantMessageTruncateFormat, len(lines)-maxLines)),
		)
	}

	return sty.Tool.Body.Render(strings.Join(out, "\n"))
}
