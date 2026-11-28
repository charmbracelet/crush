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
// subagents share a label across sessions.
var subAgentSeq atomic.Int64

// SubAgentReportItem renders the result a detached subagent reported back.
// It reads as a tool call rather than as a message: a subagent finishing is
// the same kind of event as a tool returning, and the transcript stays legible
// when every non-prose block shares one shape.
type SubAgentReportItem struct {
	*list.Versioned
	*highlightableMessageItem
	*cachedMessageItem
	*focusableMessageItem

	id              string
	label           string
	model           string
	output          string
	failed          bool
	expandedContent bool
	sty             *styles.Styles
}

var _ MessageItem = (*SubAgentReportItem)(nil)

// NewSubAgentReportItem creates a [SubAgentReportItem].
func NewSubAgentReportItem(sty *styles.Styles, label, model, output string, failed bool) MessageItem {
	v := list.NewVersioned()
	return &SubAgentReportItem{
		Versioned:                v,
		highlightableMessageItem: defaultHighlighter(sty, v),
		cachedMessageItem:        &cachedMessageItem{},
		focusableMessageItem:     newFocusableMessageItem(v),
		id:                       fmt.Sprintf("subagent-%d-%s", subAgentSeq.Add(1), label),
		label:                    label,
		model:                    model,
		output:                   output,
		failed:                   failed,
		sty:                      sty,
	}
}

func (s *SubAgentReportItem) ID() string          { return s.id }
func (s *SubAgentReportItem) FilterValue() string { return s.label + " " + s.output }

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
		return true, common.CopyToClipboard(ansi.Strip(s.output), "Subagent report copied to clipboard")
	}
	return false, nil
}

func (s *SubAgentReportItem) Render(width int) string {
	// Same left gutter the tool items use, so a report lines up with the
	// tool calls around it instead of sitting in its own column. The
	// prefixed output only depends on the width and the focus variant; a
	// highlight is applied on top and is never cached, since it changes on
	// every step of a selection drag.
	var key uint64
	if s.focused {
		key = 1
	}
	out, ok := s.getCachedPrefixedRender(width, key)
	if !ok {
		prefix := s.sty.Messages.ToolCallBlurred.Render()
		if s.focused {
			prefix = s.sty.Messages.ToolCallFocused.Render()
		}
		raw := s.RawRender(width - MessageLeftPaddingTotal)
		out = prefix + strings.ReplaceAll(raw, "\n", "\n"+prefix)
		s.setCachedPrefixedRender(out, width, key)
	}
	if !s.isHighlighted() {
		return out
	}
	return s.renderHighlighted(out, width, lipgloss.Height(out))
}

// RawRender renders the report without the gutter prefix. The result is
// cached by width: the report text never changes after construction, and
// ToggleExpanded and theme changes clear the cache.
func (s *SubAgentReportItem) RawRender(width int) string {
	if cached, _, ok := s.getCachedRender(width); ok {
		return cached
	}
	cappedWidth := cappedMessageWidth(width)

	status := ToolStatusSuccess
	if s.failed {
		status = ToolStatusError
	}
	header := subagentHeader(s.sty, status, "Subagent Report", s.label, s.model, cappedWidth, nil)

	bodyWidth := cappedWidth - toolBodyLeftPaddingTotal
	// Already padded to sit under the header text.
	body := toolOutputMarkdownContent(s.sty, s.output, bodyWidth, s.expandedContent)
	out := joinToolParts(header, body)
	s.setCachedRender(out, width, lipgloss.Height(out))
	return out
}
