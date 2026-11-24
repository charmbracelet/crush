package chat

import (
	"fmt"
	"strings"
	"sync/atomic"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

// reportSeq provides unique IDs for report items even when two share a
// label across sessions.
var reportSeq atomic.Int64

// reportItem is what subagent and job reports share. They read as a tool
// call rather than as a message: work finishing in the background is the
// same kind of event as a tool returning, and the transcript stays legible
// when every non-prose block shares one shape.
type reportItem struct {
	*list.Versioned
	*highlightableMessageItem
	*cachedMessageItem
	*focusableMessageItem

	id              string
	label           string
	output          string
	failed          bool
	copyNotice      string
	expandedContent bool
	sty             *styles.Styles
}

func newReportItem(sty *styles.Styles, kind string, report message.SubAgentReport, copyNotice string) *reportItem {
	v := list.NewVersioned()
	return &reportItem{
		Versioned:                v,
		highlightableMessageItem: defaultHighlighter(sty, v),
		cachedMessageItem:        &cachedMessageItem{},
		focusableMessageItem:     newFocusableMessageItem(v),
		id:                       fmt.Sprintf("%s-%d-%s", kind, reportSeq.Add(1), report.Label),
		label:                    report.Label,
		output:                   report.Output,
		failed:                   report.Failed,
		copyNotice:               copyNotice,
		sty:                      sty,
	}
}

func (r *reportItem) ID() string          { return r.id }
func (r *reportItem) FilterValue() string { return r.label + " " + r.output }

// ToggleExpanded toggles between the collapsed and full report.
func (r *reportItem) ToggleExpanded() bool {
	r.expandedContent = !r.expandedContent
	r.clearCache()
	r.Bump()
	return r.expandedContent
}

// HandleMouseClick implements MouseClickable so clicks select this item.
func (r *reportItem) HandleMouseClick(btn ansi.MouseButton, x, y int) bool {
	return btn == ansi.MouseLeft
}

// HandleKeyEvent implements KeyEventHandler for copying the report.
func (r *reportItem) HandleKeyEvent(key tea.KeyMsg) (bool, tea.Cmd) {
	switch key.String() {
	case "c", "y":
		return true, common.CopyToClipboard(ansi.Strip(r.output), r.copyNotice)
	}
	return false, nil
}

func (r *reportItem) status() ToolStatus {
	if r.failed {
		return ToolStatusError
	}
	return ToolStatusSuccess
}

// render puts raw, the report rendered at width-MessageLeftPaddingTotal, in
// the same left gutter the tool items use, so a report lines up with the
// tool calls around it instead of sitting in its own column.
func (r *reportItem) render(raw func(int) string, width int) string {
	prefix := r.sty.Messages.ToolCallBlurred.Render()
	if r.focused {
		prefix = r.sty.Messages.ToolCallFocused.Render()
	}
	lines := strings.Split(raw(width-MessageLeftPaddingTotal), "\n")
	for i, ln := range lines {
		lines[i] = prefix + ln
	}
	out := strings.Join(lines, "\n")
	return r.renderHighlighted(out, width, lipgloss.Height(out))
}

// SubAgentReportItem renders the result a detached subagent reported back.
type SubAgentReportItem struct {
	*reportItem
	model string
}

var _ MessageItem = (*SubAgentReportItem)(nil)

// NewSubAgentReportItem creates a [SubAgentReportItem].
func NewSubAgentReportItem(sty *styles.Styles, report message.SubAgentReport) MessageItem {
	return &SubAgentReportItem{
		reportItem: newReportItem(sty, "subagent", report, "Subagent report copied to clipboard"),
		model:      report.Model,
	}
}

func (s *SubAgentReportItem) Render(width int) string { return s.render(s.RawRender, width) }

func (s *SubAgentReportItem) RawRender(width int) string {
	cappedWidth := cappedMessageWidth(width)
	header := subagentHeader(s.sty, s.status(), "Subagent Report", s.label, s.model, cappedWidth, nil)
	bodyWidth := cappedWidth - toolBodyLeftPaddingTotal
	// Already padded to sit under the header text.
	body := toolOutputMarkdownContent(s.sty, s.output, bodyWidth, s.expandedContent)
	return joinToolParts(header, body)
}

// JobReportItem renders the report a finished background job sends.
type JobReportItem struct {
	*reportItem
}

var _ MessageItem = (*JobReportItem)(nil)

// NewJobReportItem creates a [JobReportItem].
func NewJobReportItem(sty *styles.Styles, report message.SubAgentReport) MessageItem {
	return &JobReportItem{newReportItem(sty, "job", report, "Job report copied to clipboard")}
}

func (j *JobReportItem) Render(width int) string { return j.render(j.RawRender, width) }

func (j *JobReportItem) RawRender(width int) string {
	cappedWidth := cappedMessageWidth(width)
	header := toolHeader(j.sty, j.status(), jobReportName, cappedWidth, nil, jobTarget(j.sty, j.label))
	// Shell output, which markdown would mangle.
	body := toolOutputPlainContent(j.sty, j.output, cappedWidth-toolBodyLeftPaddingTotal, j.expandedContent)
	return joinToolParts(header, body)
}
