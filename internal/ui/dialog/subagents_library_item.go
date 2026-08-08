package dialog

import (
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/sahilm/fuzzy"
)

// LibrarySubagentItemData holds the data for a library subagent list item.
type LibrarySubagentItemData struct {
	Name        string
	Description string
	Color       string
	FilePath    string
	Scope       string
	Disabled    bool
	// Deletable reports whether the workspace will honor a delete for this
	// definition (user-owned global dir only).
	Deletable bool
	// Error carries the discovery diagnostic for a definition file that
	// failed to parse or validate; such items are informational only.
	Error string
}

// LibrarySubagentItem wraps [LibrarySubagentItemData] to implement the
// [ListItem] interface for display in the subagents dialog library tab.
type LibrarySubagentItem struct {
	*list.Versioned
	t       *styles.Styles
	data    LibrarySubagentItemData
	m       fuzzy.Match
	focused bool
}

var _ ListItem = &LibrarySubagentItem{Versioned: list.NewVersioned()}

// NewLibrarySubagentItem creates a new [LibrarySubagentItem].
func NewLibrarySubagentItem(t *styles.Styles, data LibrarySubagentItemData) *LibrarySubagentItem {
	return &LibrarySubagentItem{
		Versioned: list.NewVersioned(),
		t:         t,
		data:      data,
	}
}

// Finished implements list.Item. Library subagent items are considered stable
// outside of explicit state mutations.
func (l *LibrarySubagentItem) Finished() bool {
	return true
}

// Filter implements [list.FilterableItem].
func (l *LibrarySubagentItem) Filter() string {
	return l.data.Name
}

// ID implements [ListItem].
func (l *LibrarySubagentItem) ID() string {
	return l.data.Name
}

// SetFocused implements [list.Focusable].
func (l *LibrarySubagentItem) SetFocused(focused bool) {
	if l.focused == focused {
		return
	}
	l.focused = focused
	if l.Versioned != nil {
		l.Bump()
	}
}

// SetMatch implements [list.MatchSettable].
func (l *LibrarySubagentItem) SetMatch(m fuzzy.Match) {
	if sameFuzzyMatch(l.m, m) {
		return
	}
	l.m = m
	if l.Versioned != nil {
		l.Bump()
	}
}

// Render implements list.Item. It renders the library subagent as two lines:
// the first line shows an enabled/disabled status icon, the colored dot,
// name, and scope badge; the second line shows the description, or the
// discovery diagnostic for broken definitions.
func (l *LibrarySubagentItem) Render(width int) string {
	dot := styles.SubagentDot(l.data.Color)

	status := l.t.Tool.IconSuccess.String()
	if l.data.Disabled || l.data.Error != "" {
		status = l.t.Tool.IconError.String()
	}

	itemStyle := l.t.Dialog.NormalItem
	if l.focused {
		itemStyle = l.t.Dialog.SelectedItem
	}

	innerWidth := max(0, width-itemStyle.GetHorizontalFrameSize())

	scope := l.data.Scope
	if scope == "" {
		scope = "user"
	}

	firstLine := status + " " + dot + " " + l.data.Name + "  " + scope
	firstLine = ansi.Truncate(firstLine, innerWidth, "…")

	details := l.data.Description
	if l.data.Error != "" {
		details = l.data.Error
	}

	var content string
	if details != "" {
		content = firstLine + "\n" + ansi.Truncate(details, innerWidth, "…")
	} else {
		content = firstLine
	}

	return itemStyle.Render(content)
}
