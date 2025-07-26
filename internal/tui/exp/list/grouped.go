package list

import (
	"slices"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/tui/components/core/layout"
	"github.com/charmbracelet/crush/internal/tui/util"
)

type Group[T Item] struct {
	Section ItemSection
	Items   []T
}
type GroupedList[T Item] interface {
	util.Model
	layout.Sizeable
	Items() []Item
	Groups() []Group[T]
	SetGroups([]Group[T]) tea.Cmd
	MoveUp(int) tea.Cmd
	MoveDown(int) tea.Cmd
	GoToTop() tea.Cmd
	GoToBottom() tea.Cmd
	SelectItemAbove() tea.Cmd
	SelectItemBelow() tea.Cmd
	SetSelected(string) tea.Cmd
	SelectedItem() *T
}
type groupedList[T Item] struct {
	*list[Item]
	groups []Group[T]
}

func NewGroupedList[T Item](groups []Group[T], opts ...ListOption) GroupedList[T] {
	list := &list[Item]{
		confOptions: &confOptions{
			direction: DirectionForward,
			keyMap:    DefaultKeyMap(),
			focused:   true,
		},
		items:         csync.NewSlice[Item](),
		indexMap:      csync.NewMap[string, int](),
		renderedItems: csync.NewMap[string, renderedItem](),
	}
	for _, opt := range opts {
		opt(list.confOptions)
	}

	return &groupedList[T]{
		list: list,
	}
}

func (g *groupedList[T]) Init() tea.Cmd {
	g.convertItems()
	return g.render()
}

func (l *groupedList[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	u, cmd := l.list.Update(msg)
	l.list = u.(*list[Item])
	return l, cmd
}

func (g *groupedList[T]) SelectedItem() *T {
	item := g.list.SelectedItem()
	if item == nil {
		return nil
	}
	dRef := *item
	c, ok := any(dRef).(T)
	if !ok {
		return nil
	}
	return &c
}

func (g *groupedList[T]) convertItems() {
	var items []Item
	for _, g := range g.groups {
		items = append(items, g.Section)
		for _, g := range g.Items {
			items = append(items, g)
		}
	}
	g.items.SetSlice(items)
}

func (g *groupedList[T]) SetGroups(groups []Group[T]) tea.Cmd {
	g.groups = groups
	g.convertItems()
	return g.SetItems(slices.Collect(g.items.Seq()))
}

func (g *groupedList[T]) Groups() []Group[T] {
	return g.groups
}

func (g *groupedList[T]) Items() []Item {
	return g.list.Items()
}
