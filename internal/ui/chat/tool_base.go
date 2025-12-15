package chat

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ansiext"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/common/anim"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

// responseContextHeight limits the number of lines displayed in tool output.
const responseContextHeight = 10

// ToolStatus represents the current state of a tool call.
type ToolStatus int

const (
	ToolStatusAwaitingPermission ToolStatus = iota
	ToolStatusRunning
	ToolStatusSuccess
	ToolStatusError
	ToolStatusCancelled
)

// ToolCallContext provides the context needed for rendering a tool call.
type ToolCallContext struct {
	Call                message.ToolCall
	Result              *message.ToolResult
	Cancelled           bool
	PermissionRequested bool
	PermissionGranted   bool
	IsNested            bool
	Styles              *styles.Styles

	NestedCalls []ToolCallContext
}

// Status returns the current status of the tool call.
func (ctx *ToolCallContext) Status() ToolStatus {
	if ctx.Cancelled {
		return ToolStatusCancelled
	}
	if ctx.HasResult() {
		if ctx.Result.IsError {
			return ToolStatusError
		}
		return ToolStatusSuccess
	}
	if ctx.PermissionRequested && !ctx.PermissionGranted {
		return ToolStatusAwaitingPermission
	}
	return ToolStatusRunning
}

// HasResult returns true if the tool call has a completed result.
func (ctx *ToolCallContext) HasResult() bool {
	return ctx.Result != nil && ctx.Result.ToolCallID != ""
}

// toolStyles provides common FocusStylable and HighlightStylable implementations.
type toolStyles struct {
	sty *styles.Styles
}

func (s toolStyles) FocusStyle() lipgloss.Style {
	return s.sty.Chat.Message.ToolCallFocused
}

func (s toolStyles) BlurStyle() lipgloss.Style {
	return s.sty.Chat.Message.ToolCallBlurred
}

func (s toolStyles) HighlightStyle() lipgloss.Style {
	return s.sty.TextSelection
}

// toolItem provides common base functionality for all tool items.
type toolItem struct {
	toolStyles
	id           string
	ctx          ToolCallContext
	expanded     bool
	wasTruncated bool
	spinning     bool
	anim         *anim.Anim
}

// newToolItem creates a new toolItem with the given context.
func newToolItem(ctx ToolCallContext) toolItem {
	animSize := 15
	if ctx.IsNested {
		animSize = 10
	}

	t := toolItem{
		toolStyles: toolStyles{sty: ctx.Styles},
		id:         ctx.Call.ID,
		ctx:        ctx,
		spinning:   shouldSpin(ctx),
		anim: anim.New(anim.Settings{
			Size:        animSize,
			Label:       "Working",
			GradColorA:  ctx.Styles.Primary,
			GradColorB:  ctx.Styles.Secondary,
			LabelColor:  ctx.Styles.FgBase,
			CycleColors: true,
		}),
	}

	return t
}

// shouldSpin returns true if the tool should show animation.
func shouldSpin(ctx ToolCallContext) bool {
	return !ctx.Call.Finished && !ctx.Cancelled
}

// ID implements Identifiable.
func (t *toolItem) ID() string {
	return t.id
}

// HandleMouseClick implements list.MouseClickable.
func (t *toolItem) HandleMouseClick(btn ansi.MouseButton, x, y int) bool {
	if btn != ansi.MouseLeft || !t.wasTruncated {
		return false
	}

	t.expanded = !t.expanded
	return true
}

// HandleKeyPress implements list.KeyPressable.
func (t *toolItem) HandleKeyPress(msg tea.KeyPressMsg) bool {
	if !t.wasTruncated {
		return false
	}

	if key.Matches(msg, key.NewBinding(key.WithKeys("space"))) {
		t.expanded = !t.expanded
		return true
	}

	return false
}

// updateAnimation handles animation updates and returns true if changed.
func (t *toolItem) updateAnimation(msg tea.Msg) (tea.Cmd, bool) {
	if !t.spinning || t.anim == nil {
		return nil, false
	}

	switch msg.(type) {
	case anim.StepMsg:
		updatedAnim, cmd := t.anim.Update(msg)
		t.anim = updatedAnim
		return cmd, cmd != nil
	}

	return nil, false
}

// InitAnimation initializes and starts the animation.
func (t *toolItem) InitAnimation() tea.Cmd {
	t.spinning = shouldSpin(t.ctx)
	return t.anim.Init()
}

// SetResult updates the tool call with a result.
func (t *toolItem) SetResult(result message.ToolResult) {
	t.ctx.Result = &result
	t.ctx.Call.Finished = true
	t.spinning = false
}

// SetCancelled marks the tool call as cancelled.
func (t *toolItem) SetCancelled() {
	t.ctx.Cancelled = true
	t.spinning = false
}

// UpdateCall updates the tool call data.
func (t *toolItem) UpdateCall(call message.ToolCall) {
	t.ctx.Call = call
	if call.Finished {
		t.spinning = false
	}
}

// SetNestedCalls sets the nested tool calls for agent tools.
func (t *toolItem) SetNestedCalls(calls []ToolCallContext) {
	t.ctx.NestedCalls = calls
}

// Context returns the current tool call context.
func (t *toolItem) Context() *ToolCallContext {
	return &t.ctx
}

// renderPending returns the pending state view with animation.
func (t *toolItem) renderPending() string {
	icon := t.sty.Tool.IconPending.Render()

	var toolName string
	if t.ctx.IsNested {
		toolName = t.sty.Tool.NameNested.Render(prettifyToolName(t.ctx.Call.Name))
	} else {
		toolName = t.sty.Tool.NameNormal.Render(prettifyToolName(t.ctx.Call.Name))
	}

	var animView string
	if t.anim != nil {
		animView = t.anim.View()
	}

	return fmt.Sprintf("%s %s %s", icon, toolName, animView)
}

// unmarshalParams unmarshals JSON input into the target struct.
func unmarshalParams(input string, target any) error {
	return json.Unmarshal([]byte(input), target)
}

// ParamBuilder helps construct parameter lists for tool headers.
type ParamBuilder struct {
	args []string
}

// NewParamBuilder creates a new parameter builder.
func NewParamBuilder() *ParamBuilder {
	return &ParamBuilder{args: make([]string, 0, 4)}
}

// Main adds the main parameter (first positional argument).
func (pb *ParamBuilder) Main(value string) *ParamBuilder {
	if value != "" {
		pb.args = append(pb.args, value)
	}
	return pb
}

// KeyValue adds a key-value pair parameter.
func (pb *ParamBuilder) KeyValue(key, value string) *ParamBuilder {
	if value != "" {
		pb.args = append(pb.args, key, value)
	}
	return pb
}

// Flag adds a boolean flag parameter (only if true).
func (pb *ParamBuilder) Flag(key string, value bool) *ParamBuilder {
	if value {
		pb.args = append(pb.args, key, "true")
	}
	return pb
}

// Build returns the parameter list.
func (pb *ParamBuilder) Build() []string {
	return pb.args
}

// renderToolIcon returns the status icon for a tool call.
func renderToolIcon(status ToolStatus, sty *styles.Styles) string {
	switch status {
	case ToolStatusSuccess:
		return sty.Tool.IconSuccess.String()
	case ToolStatusError:
		return sty.Tool.IconError.String()
	case ToolStatusCancelled:
		return sty.Tool.IconCancelled.String()
	default:
		return sty.Tool.IconPending.String()
	}
}

// renderToolHeader builds the tool header line: "● ToolName params..."
func renderToolHeader(ctx *ToolCallContext, name string, width int, params ...string) string {
	sty := ctx.Styles
	icon := renderToolIcon(ctx.Status(), sty)

	var toolName string
	if ctx.IsNested {
		toolName = sty.Tool.NameNested.Render(name)
	} else {
		toolName = sty.Tool.NameNormal.Render(name)
	}

	prefix := fmt.Sprintf("%s %s ", icon, toolName)
	prefixWidth := lipgloss.Width(prefix)
	remainingWidth := width - prefixWidth

	paramsStr := renderParamList(params, remainingWidth, sty)
	return prefix + paramsStr
}

// renderParamList formats parameters as "main (key=value, ...)" with truncation.
func renderParamList(params []string, width int, sty *styles.Styles) string {
	if len(params) == 0 {
		return ""
	}

	mainParam := params[0]
	if width >= 0 && lipgloss.Width(mainParam) > width {
		mainParam = ansi.Truncate(mainParam, width, "…")
	}

	if len(params) == 1 {
		return sty.Tool.ParamMain.Render(mainParam)
	}

	// Build key=value pairs from remaining params.
	otherParams := params[1:]
	if len(otherParams)%2 != 0 {
		otherParams = append(otherParams, "")
	}

	var parts []string
	for i := 0; i < len(otherParams); i += 2 {
		key := otherParams[i]
		value := otherParams[i+1]
		if value == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", key, value))
	}

	if len(parts) == 0 {
		return sty.Tool.ParamMain.Render(ansi.Truncate(mainParam, width, "…"))
	}

	partsRendered := strings.Join(parts, ", ")
	remainingWidth := width - lipgloss.Width(partsRendered) - 3 // " ()"
	if remainingWidth < 30 {
		// Not enough space for params, just show main.
		return sty.Tool.ParamMain.Render(ansi.Truncate(mainParam, width, "…"))
	}

	fullParam := fmt.Sprintf("%s (%s)", mainParam, partsRendered)
	return sty.Tool.ParamMain.Render(ansi.Truncate(fullParam, width, "…"))
}

// renderEarlyState handles error/cancelled/pending states before content rendering.
// Returns the rendered output and true if early state was handled.
func renderEarlyState(ctx *ToolCallContext, header string, width int) (string, bool) {
	sty := ctx.Styles

	var msg string
	switch ctx.Status() {
	case ToolStatusError:
		msg = renderToolError(ctx, width)
	case ToolStatusCancelled:
		msg = sty.Tool.StateCancelled.Render("Canceled.")
	case ToolStatusAwaitingPermission:
		msg = sty.Tool.StateWaiting.Render("Requesting permission...")
	case ToolStatusRunning:
		msg = sty.Tool.StateWaiting.Render("Waiting for tool response...")
	default:
		return "", false
	}

	msg = sty.Tool.BodyPadding.Render(msg)
	return lipgloss.JoinVertical(lipgloss.Left, header, "", msg), true
}

// renderToolError formats an error message with ERROR tag.
func renderToolError(ctx *ToolCallContext, width int) string {
	sty := ctx.Styles
	errContent := strings.ReplaceAll(ctx.Result.Content, "\n", " ")
	errTag := sty.Tool.ErrorTag.Render("ERROR")
	tagWidth := lipgloss.Width(errTag)
	errContent = ansi.Truncate(errContent, width-tagWidth-3, "…")
	return fmt.Sprintf("%s %s", errTag, sty.Tool.ErrorMessage.Render(errContent))
}

// joinHeaderBody combines header and body with proper padding.
func joinHeaderBody(header, body string, sty *styles.Styles) string {
	if body == "" {
		return header
	}
	body = sty.Tool.BodyPadding.Render(body)
	return lipgloss.JoinVertical(lipgloss.Left, header, "", body)
}

// renderPlainContent renders plain text with optional expansion support.
func renderPlainContent(content string, width int, sty *styles.Styles, item *toolItem) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\t", "    ")
	content = strings.TrimSpace(content)
	lines := strings.Split(content, "\n")

	expanded := item != nil && item.expanded
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
	if item != nil {
		item.wasTruncated = wasTruncated
	}

	if !expanded && wasTruncated {
		out = append(out, sty.Tool.ContentTruncation.
			Width(width).
			Render(fmt.Sprintf("… (%d lines) [click or space to expand]", len(lines)-responseContextHeight)))
	}

	return strings.Join(out, "\n")
}

// formatNonZero returns string representation of non-zero integers, empty for zero.
func formatNonZero(value int) string {
	if value == 0 {
		return ""
	}
	return fmt.Sprintf("%d", value)
}

// renderCodeContent renders syntax-highlighted code with line numbers and optional expansion.
func renderCodeContent(path, content string, offset, width int, sty *styles.Styles, item *toolItem) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\t", "    ")

	lines := strings.Split(content, "\n")

	maxLines := responseContextHeight
	if item != nil && item.expanded {
		maxLines = len(lines)
	}

	truncated := lines
	if len(lines) > maxLines {
		truncated = lines[:maxLines]
	}

	// Escape ANSI sequences in content.
	for i, ln := range truncated {
		truncated[i] = ansiext.Escape(ln)
	}

	// Apply syntax highlighting.
	bg := sty.Tool.ContentCodeBg
	highlighted, _ := common.SyntaxHighlight(sty, strings.Join(truncated, "\n"), path, bg)
	highlightedLines := strings.Split(highlighted, "\n")

	// Calculate gutter width for line numbers.
	maxLineNum := offset + len(highlightedLines)
	maxDigits := getDigits(maxLineNum)
	numFmt := fmt.Sprintf("%%%dd", maxDigits)

	// Calculate available width for code (accounting for gutter).
	const numPR, numPL, codePR, codePL = 1, 1, 1, 2
	codeWidth := width - maxDigits - numPL - numPR - 2

	var out []string
	for i, ln := range highlightedLines {
		lineNum := sty.Base.
			Foreground(sty.FgMuted).
			Background(bg).
			PaddingRight(numPR).
			PaddingLeft(numPL).
			Render(fmt.Sprintf(numFmt, offset+i+1))

		codeLine := sty.Base.
			Width(codeWidth).
			Background(bg).
			PaddingRight(codePR).
			PaddingLeft(codePL).
			Render(ansi.Truncate(ln, codeWidth-codePL-codePR, "…"))

		out = append(out, lipgloss.JoinHorizontal(lipgloss.Left, lineNum, codeLine))
	}

	wasTruncated := len(lines) > responseContextHeight
	if item != nil {
		item.wasTruncated = wasTruncated
	}

	expanded := item != nil && item.expanded

	if !expanded && wasTruncated {
		msg := fmt.Sprintf(" …(%d lines) [click or space to expand]", len(lines)-responseContextHeight)
		out = append(out, sty.Muted.Background(bg).Render(msg))
	}

	return lipgloss.JoinVertical(lipgloss.Left, out...)
}

// renderMarkdownContent renders markdown with optional expansion support.
func renderMarkdownContent(content string, width int, sty *styles.Styles, item *toolItem) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\t", "    ")
	content = strings.TrimSpace(content)

	cappedWidth := min(width, 120)
	renderer := common.PlainMarkdownRenderer(sty, cappedWidth)
	rendered, err := renderer.Render(content)
	if err != nil {
		return renderPlainContent(content, width, sty, nil)
	}

	lines := strings.Split(rendered, "\n")

	maxLines := responseContextHeight
	if item != nil && item.expanded {
		maxLines = len(lines)
	}

	var out []string
	for i, ln := range lines {
		if i >= maxLines {
			break
		}
		out = append(out, ln)
	}

	wasTruncated := len(lines) > responseContextHeight
	if item != nil {
		item.wasTruncated = wasTruncated
	}

	expanded := item != nil && item.expanded

	if !expanded && wasTruncated {
		out = append(out, sty.Tool.ContentTruncation.
			Width(cappedWidth-2).
			Render(fmt.Sprintf("… (%d lines) [click or space to expand]", len(lines)-responseContextHeight)))
	}

	return sty.Tool.ContentLine.Render(strings.Join(out, "\n"))
}

// renderDiffContent renders a diff with optional expansion support.
func renderDiffContent(file, oldContent, newContent string, width int, sty *styles.Styles, item *toolItem) string {
	formatter := common.DiffFormatter(sty).
		Before(file, oldContent).
		After(file, newContent).
		Width(width)

	if width > 120 {
		formatter = formatter.Split()
	}

	formatted := formatter.String()
	lines := strings.Split(formatted, "\n")

	wasTruncated := len(lines) > responseContextHeight
	if item != nil {
		item.wasTruncated = wasTruncated
	}

	expanded := item != nil && item.expanded

	if !expanded && wasTruncated {
		truncateMsg := sty.Tool.DiffTruncation.
			Width(width).
			Render(fmt.Sprintf("… (%d lines) [click or space to expand]", len(lines)-responseContextHeight))
		formatted = strings.Join(lines[:responseContextHeight], "\n") + "\n" + truncateMsg
	}

	return formatted
}

// renderImageContent renders image data with optional text content.
func renderImageContent(data, mediaType, textContent string, sty *styles.Styles) string {
	dataSize := len(data) * 3 / 4 // Base64 to bytes approximation.
	sizeStr := formatSize(dataSize)

	loaded := sty.Tool.IconSuccess.String()
	arrow := sty.Tool.NameNested.Render("→")
	typeStyled := sty.Base.Render(mediaType)
	sizeStyled := sty.Subtle.Render(sizeStr)

	imageDisplay := fmt.Sprintf("%s %s %s %s", loaded, arrow, typeStyled, sizeStyled)

	if strings.TrimSpace(textContent) != "" {
		textDisplay := sty.Tool.ContentLine.Render(textContent)
		return lipgloss.JoinVertical(lipgloss.Left, textDisplay, "", imageDisplay)
	}

	return imageDisplay
}

// renderMediaContent renders non-image media content.
func renderMediaContent(mediaType, textContent string, sty *styles.Styles) string {
	loaded := sty.Tool.IconSuccess.String()
	arrow := sty.Tool.NameNested.Render("→")
	typeStyled := sty.Base.Render(mediaType)
	mediaDisplay := fmt.Sprintf("%s %s %s", loaded, arrow, typeStyled)

	if strings.TrimSpace(textContent) != "" {
		textDisplay := sty.Tool.ContentLine.Render(textContent)
		return lipgloss.JoinVertical(lipgloss.Left, textDisplay, "", mediaDisplay)
	}

	return mediaDisplay
}

// formatSize formats byte count as human-readable size.
func formatSize(bytes int) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
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

// formatTodosList formats a list of todos with status icons.
func formatTodosList(todos []session.Todo, width int, sty *styles.Styles) string {
	if len(todos) == 0 {
		return ""
	}

	sorted := make([]session.Todo, len(todos))
	copy(sorted, todos)
	slices.SortStableFunc(sorted, func(a, b session.Todo) int {
		return todoStatusOrder(a.Status) - todoStatusOrder(b.Status)
	})

	var lines []string
	for _, todo := range sorted {
		var prefix string
		var textStyle lipgloss.Style

		switch todo.Status {
		case session.TodoStatusCompleted:
			prefix = sty.Base.Foreground(sty.Green).Render(styles.TodoCompletedIcon) + " "
			textStyle = sty.Base
		case session.TodoStatusInProgress:
			prefix = sty.Base.Foreground(sty.GreenDark).Render(styles.ArrowRightIcon) + " "
			textStyle = sty.Base
		default:
			prefix = sty.Muted.Render(styles.TodoPendingIcon) + " "
			textStyle = sty.Base
		}

		text := todo.Content
		if todo.Status == session.TodoStatusInProgress && todo.ActiveForm != "" {
			text = todo.ActiveForm
		}

		line := prefix + textStyle.Render(text)
		line = ansi.Truncate(line, width, "…")
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// todoStatusOrder returns sort order for todo statuses.
func todoStatusOrder(s session.TodoStatus) int {
	switch s {
	case session.TodoStatusCompleted:
		return 0
	case session.TodoStatusInProgress:
		return 1
	default:
		return 2
	}
}
