package dialog

import (
	"fmt"

	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/sahilm/fuzzy"
)

// RunningSubagentItemData holds the data for a running subagent list item.
type RunningSubagentItemData struct {
	ChildSessionID   string
	Name             string
	Color            string
	Model            string
	PromptTokens     int64
	CompletionTokens int64
}

// RunningSubagentItem wraps [RunningSubagentItemData] to implement the
// [ListItem] interface for display in the subagents dialog running tab.
type RunningSubagentItem struct {
	*list.Versioned
	t       *styles.Styles
	data    RunningSubagentItemData
	m       fuzzy.Match
	focused bool
}

var _ ListItem = &RunningSubagentItem{Versioned: list.NewVersioned()}

// NewRunningSubagentItem creates a new [RunningSubagentItem].
func NewRunningSubagentItem(t *styles.Styles, data RunningSubagentItemData) *RunningSubagentItem {
	return &RunningSubagentItem{
		Versioned: list.NewVersioned(),
		t:         t,
		data:      data,
	}
}

// Finished implements list.Item. Running subagent items are considered stable
// outside of explicit state mutations.
func (r *RunningSubagentItem) Finished() bool {
	return true
}

// Filter implements [list.FilterableItem].
func (r *RunningSubagentItem) Filter() string {
	return r.data.Name
}

// ID implements [ListItem].
func (r *RunningSubagentItem) ID() string {
	return r.data.ChildSessionID
}

// SetFocused implements [list.Focusable].
func (r *RunningSubagentItem) SetFocused(focused bool) {
	if r.focused == focused {
		return
	}
	r.focused = focused
	if r.Versioned != nil {
		r.Bump()
	}
}

// SetMatch implements [list.MatchSettable].
func (r *RunningSubagentItem) SetMatch(m fuzzy.Match) {
	if sameFuzzyMatch(r.m, m) {
		return
	}
	r.m = m
	if r.Versioned != nil {
		r.Bump()
	}
}

// Render implements list.Item. It renders the running subagent as a single
// line showing the colored dot, name, model, and total token count.
func (r *RunningSubagentItem) Render(width int) string {
	dot := styles.SubagentDot(r.data.Color)
	totalTokens := r.data.PromptTokens + r.data.CompletionTokens
	tokStr := fmt.Sprintf("%d tok", totalTokens)

	itemStyle := r.t.Dialog.NormalItem
	if r.focused {
		itemStyle = r.t.Dialog.SelectedItem
	}

	content := dot + " " + r.data.Name + "  " + r.data.Model + "  " + tokStr
	content = ansi.Truncate(content, max(0, width-itemStyle.GetHorizontalFrameSize()), "…")
	return itemStyle.Render(content)
}
