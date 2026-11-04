package chat

import (
	"strings"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/subagents"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

// SubagentReportItem renders the report a background subagent sent back to
// the parent session.
type SubagentReportItem struct {
	*list.Versioned
	*highlightableMessageItem
	*cachedMessageItem
	*focusableMessageItem

	message  *message.Message
	report   message.SubagentReport
	sty      *styles.Styles
	expanded bool
}

var (
	_ HighlightableMessageItem = (*SubagentReportItem)(nil)
	_ FocusableMessageItem     = (*SubagentReportItem)(nil)
	_ Expandable               = (*SubagentReportItem)(nil)
)

// NewSubagentReportItem creates a new [SubagentReportItem] for a message
// that holds a [message.SubagentReport].
func NewSubagentReportItem(sty *styles.Styles, msg *message.Message) MessageItem {
	v := list.NewVersioned()
	report, _ := msg.SubagentReport()
	return &SubagentReportItem{
		Versioned:                v,
		highlightableMessageItem: defaultHighlighter(sty, v),
		cachedMessageItem:        &cachedMessageItem{},
		focusableMessageItem:     newFocusableMessageItem(v),
		message:                  msg,
		report:                   report,
		sty:                      sty,
	}
}

// ID implements MessageItem.
func (m *SubagentReportItem) ID() string {
	return m.message.ID
}

// Finished implements list.Item. Reports are immutable once stored.
func (m *SubagentReportItem) Finished() bool {
	return true
}

// ToggleExpanded implements [Expandable].
func (m *SubagentReportItem) ToggleExpanded() bool {
	m.expanded = !m.expanded
	m.clearCache()
	m.Bump()
	return m.expanded
}

// status maps the report status to the tool status that draws its icon.
func (m *SubagentReportItem) status() ToolStatus {
	switch m.report.Status {
	case subagents.StatusCompleted:
		return ToolStatusSuccess
	case subagents.StatusCancelled:
		return ToolStatusCanceled
	default:
		return ToolStatusError
	}
}

// RawRender implements [MessageItem].
func (m *SubagentReportItem) RawRender(width int) string {
	cappedWidth := cappedMessageWidth(width)

	content, height, ok := m.getCachedRender(cappedWidth)
	if ok {
		return m.renderHighlighted(content, cappedWidth, height)
	}

	header := toolIcon(m.sty, m.status()) + " " +
		m.sty.Tool.NameNormal.Render("Report: "+m.report.Name) + " " +
		m.sty.Tool.ParamMain.Render(m.report.Status)
	header = ansi.Truncate(header, cappedWidth, "…")
	content = header
	if strings.TrimSpace(m.report.Output) != "" {
		body := toolOutputMarkdownContent(m.sty, m.report.Output, cappedWidth-toolBodyLeftPaddingTotal, m.expanded)
		content = joinToolParts(header, body)
	}

	height = strings.Count(content, "\n") + 1
	m.setCachedRender(content, cappedWidth, height)
	return m.renderHighlighted(content, cappedWidth, height)
}

// Render implements MessageItem.
func (m *SubagentReportItem) Render(width int) string {
	useCache := !m.isHighlighted()
	var key uint64
	if m.focused {
		key = 1
	}
	if useCache {
		if cached, ok := m.getCachedPrefixedRender(width, key); ok {
			return cached
		}
	}
	prefix := m.sty.Messages.ToolCallBlurred.Render()
	if m.focused {
		prefix = m.sty.Messages.ToolCallFocused.Render()
	}
	lines := strings.Split(m.RawRender(width), "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	out := strings.Join(lines, "\n")
	if useCache {
		m.setCachedPrefixedRender(out, width, key)
	}
	return out
}
