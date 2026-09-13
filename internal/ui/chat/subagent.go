package chat

import (
	"fmt"
	"strings"
	"sync/atomic"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

// subAgentSeq provides unique IDs for SubAgentReportItems even when two
// sub-agents share a label across sessions.
var subAgentSeq atomic.Int64

// SubAgentReportItem renders the result a detached sub-agent reported back.
// It reads as a tool call rather than as a message: a sub-agent finishing is
// the same kind of event as a tool returning, and the transcript stays legible
// when every non-prose block shares one shape.
type SubAgentReportItem struct {
	*list.Versioned
	*highlightableMessageItem
	*cachedMessageItem
	*focusableMessageItem

	id              string
	label           string
	output          string
	failed          bool
	expandedContent bool
	sty             *styles.Styles
}

var _ MessageItem = (*SubAgentReportItem)(nil)

// NewSubAgentReportItem creates a [SubAgentReportItem].
func NewSubAgentReportItem(sty *styles.Styles, label, output string, failed bool) MessageItem {
	v := list.NewVersioned()
	return &SubAgentReportItem{
		Versioned:                v,
		highlightableMessageItem: defaultHighlighter(sty, v),
		cachedMessageItem:        &cachedMessageItem{},
		focusableMessageItem:     newFocusableMessageItem(v),
		id:                       fmt.Sprintf("subagent-%d-%s", subAgentSeq.Add(1), label),
		label:                    label,
		output:                   output,
		failed:                   failed,
		sty:                      sty,
	}
}

func (s *SubAgentReportItem) ID() string          { return s.id }
func (s *SubAgentReportItem) FilterValue() string { return s.label + " " + s.output }

// Finished implements [MessageItem]. A report only exists once the sub-agent
// has finished, so it is never in flight.
func (s *SubAgentReportItem) Finished() bool { return true }

// ToggleExpanded toggles between the collapsed and full report.
func (s *SubAgentReportItem) ToggleExpanded() bool {
	s.expandedContent = !s.expandedContent
	s.clearCache()
	s.Bump()
	return s.expandedContent
}

// HandleMouseClick implements MouseClickable so clicks select this item.
func (s *SubAgentReportItem) HandleMouseClick(btn ansi.MouseButton, x, y int) bool {
	return btn == ansi.MouseLeft
}

// HandleKeyEvent implements KeyEventHandler for copying the report.
func (s *SubAgentReportItem) HandleKeyEvent(key tea.KeyMsg) (bool, tea.Cmd) {
	switch key.String() {
	case "c", "y":
		return true, common.CopyToClipboard(ansi.Strip(s.output), "Sub-agent report copied to clipboard")
	}
	return false, nil
}

func (s *SubAgentReportItem) Render(width int) string {
	// Same left gutter the tool items use, so a report lines up with the
	// tool calls around it instead of sitting in its own column.
	prefix := s.sty.Messages.ToolCallBlurred.Render()
	if s.focused {
		prefix = s.sty.Messages.ToolCallFocused.Render()
	}
	lines := strings.Split(s.RawRender(width-MessageLeftPaddingTotal), "\n")
	for i, ln := range lines {
		lines[i] = prefix + ln
	}
	out := strings.Join(lines, "\n")
	return s.renderHighlighted(out, width, lipgloss.Height(out))
}

func (s *SubAgentReportItem) RawRender(width int) string {
	cappedWidth := cappedMessageWidth(width)

	status := ToolStatusSuccess
	if s.failed {
		status = ToolStatusError
	}
	header := fmt.Sprintf(
		"%s %s %s",
		toolIcon(s.sty, status),
		s.sty.Tool.NameNormal.Render("Sub-agent"),
		s.sty.Tool.ParamMain.Render(s.label),
	)

	bodyWidth := cappedWidth - toolBodyLeftPaddingTotal
	// Sub-agents write prose, and the sync agent tool already renders its
	// result as markdown, so a report reads the same way.
	body := s.sty.Tool.Body.Render(
		toolOutputMarkdownContent(s.sty, s.output, bodyWidth, s.expandedContent),
	)
	return joinToolParts(header, body)
}
