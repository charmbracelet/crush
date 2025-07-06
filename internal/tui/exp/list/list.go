package list

import (
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/tui/components/core/layout"
	"github.com/charmbracelet/crush/internal/tui/util"
)

type Item interface {
	util.Model
	layout.Sizeable
}

type List interface {
	util.Model
}

type list struct {
	width, height int
	gap           int

	items []Item

	renderedView string

	// Filter options
	filterable        bool
	filterPlaceholder string
}

type listOption func(*list)

// WithFilterable enables filtering on the list.
func WithFilterable(placeholder string) listOption {
	return func(l *list) {
		l.filterable = true
		l.filterPlaceholder = placeholder
	}
}

// WithItems sets the initial items for the list.
func WithItems(items ...Item) listOption {
	return func(l *list) {
		l.items = items
	}
}

// WithSize sets the size of the list.
func WithSize(width, height int) listOption {
	return func(l *list) {
		l.width = width
		l.height = height
	}
}

// WithGap sets the gap between items in the list.
func WithGap(gap int) listOption {
	return func(l *list) {
		l.gap = gap
	}
}

func New(opts ...listOption) List {
	list := &list{
		items: make([]Item, 0),
	}
	for _, opt := range opts {
		opt(list)
	}
	return list
}

// Init implements List.
func (l *list) Init() tea.Cmd {
	if l.height <= 0 || l.width <= 0 {
		return nil
	}
	return nil
}

// Update implements List.
func (l *list) Update(tea.Msg) (tea.Model, tea.Cmd) {
	panic("unimplemented")
}

// View implements List.
func (l *list) View() tea.View {
	panic("unimplemented")
}
