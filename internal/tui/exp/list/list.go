package list

import (
	"strings"

	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/tui/components/core/layout"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/lipgloss/v2"
)

type Item interface {
	util.Model
	layout.Sizeable
	ID() string
}

type (
	renderedMsg  struct{}
	List[T Item] interface {
		util.Model
		layout.Sizeable
		layout.Focusable

		// Just change state
		MoveUp(int) tea.Cmd
		MoveDown(int) tea.Cmd
		GoToTop() tea.Cmd
		GoToBottom() tea.Cmd
		SelectItemAbove() tea.Cmd
		SelectItemBelow() tea.Cmd
		SetItems([]T) tea.Cmd
		SetSelected(string) tea.Cmd
		SelectedItem() *T
		Items() []T
		UpdateItem(string, T) tea.Cmd
		DeleteItem(string) tea.Cmd
		PrependItem(T) tea.Cmd
		AppendItem(T) tea.Cmd
	}
)

type direction int

const (
	DirectionForward direction = iota
	DirectionBackward
)

const (
	ItemNotFound              = -1
	ViewportDefaultScrollSize = 2
)

type renderedItem struct {
	id     string
	view   string
	height int
	start  int
	end    int
}

type confOptions struct {
	width, height int
	gap           int
	// if you are at the last item and go down it will wrap to the top
	wrap         bool
	keyMap       KeyMap
	direction    direction
	selectedItem string
	focused      bool
}

type list[T Item] struct {
	*confOptions

	offset int

	indexMap map[string]int
	items    []T

	renderedItems map[string]renderedItem

	rendered string

	movingByItem bool
}

type listOption func(*confOptions)

// WithSize sets the size of the list.
func WithSize(width, height int) listOption {
	return func(l *confOptions) {
		l.width = width
		l.height = height
	}
}

// WithGap sets the gap between items in the list.
func WithGap(gap int) listOption {
	return func(l *confOptions) {
		l.gap = gap
	}
}

// WithDirectionForward sets the direction to forward
func WithDirectionForward() listOption {
	return func(l *confOptions) {
		l.direction = DirectionForward
	}
}

// WithDirectionBackward sets the direction to forward
func WithDirectionBackward() listOption {
	return func(l *confOptions) {
		l.direction = DirectionBackward
	}
}

// WithSelectedItem sets the initially selected item in the list.
func WithSelectedItem(id string) listOption {
	return func(l *confOptions) {
		l.selectedItem = id
	}
}

func WithKeyMap(keyMap KeyMap) listOption {
	return func(l *confOptions) {
		l.keyMap = keyMap
	}
}

func WithWrapNavigation() listOption {
	return func(l *confOptions) {
		l.wrap = true
	}
}

func WithFocus(focus bool) listOption {
	return func(l *confOptions) {
		l.focused = focus
	}
}

func New[T Item](items []T, opts ...listOption) List[T] {
	list := &list[T]{
		confOptions: &confOptions{
			direction: DirectionForward,
			keyMap:    DefaultKeyMap(),
			focused:   true,
		},
		items:         items,
		indexMap:      make(map[string]int),
		renderedItems: map[string]renderedItem{},
	}
	for _, opt := range opts {
		opt(list.confOptions)
	}

	for inx, item := range items {
		list.indexMap[item.ID()] = inx
	}
	return list
}

// Init implements List.
func (l *list[T]) Init() tea.Cmd {
	return l.render()
}

// Update implements List.
func (l *list[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if l.focused {
			switch {
			case key.Matches(msg, l.keyMap.Down):
				return l, l.MoveDown(ViewportDefaultScrollSize)
			case key.Matches(msg, l.keyMap.Up):
				return l, l.MoveUp(ViewportDefaultScrollSize)
			case key.Matches(msg, l.keyMap.DownOneItem):
				return l, l.SelectItemBelow()
			case key.Matches(msg, l.keyMap.UpOneItem):
				return l, l.SelectItemAbove()
			case key.Matches(msg, l.keyMap.HalfPageDown):
				return l, l.MoveDown(l.height / 2)
			case key.Matches(msg, l.keyMap.HalfPageUp):
				return l, l.MoveUp(l.height / 2)
			case key.Matches(msg, l.keyMap.PageDown):
				return l, l.MoveDown(l.height)
			case key.Matches(msg, l.keyMap.PageUp):
				return l, l.MoveUp(l.height)
			case key.Matches(msg, l.keyMap.End):
				return l, l.GoToBottom()
			case key.Matches(msg, l.keyMap.Home):
				return l, l.GoToTop()
			}
		}
	}
	return l, nil
}

// View implements List.
func (l *list[T]) View() string {
	if l.height <= 0 || l.width <= 0 {
		return ""
	}
	view := l.rendered
	lines := strings.Split(view, "\n")

	start, end := l.viewPosition()
	viewStart := max(0, start)
	viewEnd := min(len(lines), end+1)
	lines = lines[viewStart:viewEnd]
	return strings.Join(lines, "\n")
}

func (l *list[T]) viewPosition() (int, int) {
	start, end := 0, 0
	renderedLines := lipgloss.Height(l.rendered) - 1
	if l.direction == DirectionForward {
		start = max(0, l.offset)
		end = min(l.offset+l.height-1, renderedLines)
	} else {
		start = max(0, renderedLines-l.offset-l.height+1)
		end = max(0, renderedLines-l.offset)
	}
	return start, end
}

func (l *list[T]) recalculateItemPositions() {
	currentContentHeight := 0
	for _, item := range l.items {
		rItem, ok := l.renderedItems[item.ID()]
		if !ok {
			continue
		}
		rItem.start = currentContentHeight
		rItem.end = currentContentHeight + rItem.height - 1
		l.renderedItems[item.ID()] = rItem
		currentContentHeight = rItem.end + 1 + l.gap
	}
}

func (l *list[T]) render() tea.Cmd {
	if l.width <= 0 || l.height <= 0 || len(l.items) == 0 {
		return nil
	}
	l.setDefaultSelected()

	var focusChangeCmd tea.Cmd
	if l.focused {
		focusChangeCmd = l.focusSelectedItem()
	} else {
		focusChangeCmd = l.blurSelectedItem()
	}
	// we are not rendering the first time
	if l.rendered != "" {
		l.rendered = ""
		// rerender everything will mostly hit cache
		_ = l.renderIterator(0, false)
		if l.direction == DirectionBackward {
			l.recalculateItemPositions()
		}
		// in the end scroll to the selected item
		if l.focused {
			l.scrollToSelection()
		}
		return focusChangeCmd
	}
	finishIndex := l.renderIterator(0, true)
	// recalculate for the initial items
	if l.direction == DirectionBackward {
		l.recalculateItemPositions()
	}
	renderCmd := func() tea.Msg {
		// render the rest
		_ = l.renderIterator(finishIndex, false)
		// needed for backwards
		if l.direction == DirectionBackward {
			l.recalculateItemPositions()
		}
		// in the end scroll to the selected item
		if l.focused {
			l.scrollToSelection()
		}

		return renderedMsg{}
	}
	return tea.Batch(focusChangeCmd, renderCmd)
}

func (l *list[T]) setDefaultSelected() {
	if l.selectedItem == "" {
		if l.direction == DirectionForward {
			l.selectFirstItem()
		} else {
			l.selectLastItem()
		}
	}
}

func (l *list[T]) scrollToSelection() {
	rItem, ok := l.renderedItems[l.selectedItem]
	if !ok {
		l.selectedItem = ""
		l.setDefaultSelected()
		return
	}

	start, end := l.viewPosition()
	// item bigger or equal to the viewport do nothing
	if rItem.start <= start && rItem.end >= end {
		return
	}
	// if we are moving by item we want to move the offset so that the
	// whole item is visible not just portions of it
	if l.movingByItem {
		if rItem.start >= start && rItem.end <= end {
			return
		}
		defer func() { l.movingByItem = false }()
	} else {
		// item already in view do nothing
		if rItem.start >= start && rItem.start <= end {
			return
		}
		if rItem.end >= start && rItem.end <= end {
			return
		}
	}

	if rItem.height >= l.height {
		if l.direction == DirectionForward {
			l.offset = rItem.start
		} else {
			l.offset = max(0, lipgloss.Height(l.rendered)-(rItem.start+l.height))
		}
		return
	}

	renderedLines := lipgloss.Height(l.rendered) - 1

	// If item is above the viewport, make it the first item
	if rItem.start < start {
		if l.direction == DirectionForward {
			l.offset = rItem.start
		} else {
			l.offset = max(0, renderedLines-rItem.start-l.height+1)
		}
	} else if rItem.end > end {
		// If item is below the viewport, make it the last item
		if l.direction == DirectionForward {
			l.offset = max(0, rItem.end-l.height+1)
		} else {
			l.offset = max(0, renderedLines-rItem.end)
		}
	}
}

func (l *list[T]) changeSelectionWhenScrolling() tea.Cmd {
	rItem, ok := l.renderedItems[l.selectedItem]
	if !ok {
		return nil
	}
	start, end := l.viewPosition()
	// item bigger than the viewport do nothing
	if rItem.start <= start && rItem.end >= end {
		return nil
	}
	// item already in view do nothing
	if rItem.start >= start && rItem.end <= end {
		return nil
	}

	itemMiddle := rItem.start + rItem.height/2

	if itemMiddle < start {
		// select the first item in the viewport
		// the item is most likely an item coming after this item
		inx := l.indexMap[rItem.id]
		for {
			inx = l.firstSelectableItemBelow(inx)
			if inx == ItemNotFound {
				return nil
			}
			item, ok := l.renderedItems[l.items[inx].ID()]
			if !ok {
				continue
			}

			// If the item is bigger than the viewport, select it
			if item.start <= start && item.end >= end {
				l.selectedItem = item.id
				return l.render()
			}
			// item is in the view
			if item.start >= start && item.start <= end {
				l.selectedItem = item.id
				return l.render()
			}
		}
	} else if itemMiddle > end {
		// select the first item in the viewport
		// the item is most likely an item coming after this item
		inx := l.indexMap[rItem.id]
		for {
			inx = l.firstSelectableItemAbove(inx)
			if inx == ItemNotFound {
				return nil
			}
			item, ok := l.renderedItems[l.items[inx].ID()]
			if !ok {
				continue
			}

			// If the item is bigger than the viewport, select it
			if item.start <= start && item.end >= end {
				l.selectedItem = item.id
				return l.render()
			}
			// item is in the view
			if item.end >= start && item.end <= end {
				l.selectedItem = item.id
				return l.render()
			}
		}
	}
	return nil
}

func (l *list[T]) selectFirstItem() {
	inx := l.firstSelectableItemBelow(-1)
	if inx != ItemNotFound {
		l.selectedItem = l.items[inx].ID()
	}
}

func (l *list[T]) selectLastItem() {
	inx := l.firstSelectableItemAbove(len(l.items))
	if inx != ItemNotFound {
		l.selectedItem = l.items[inx].ID()
	}
}

func (l *list[T]) firstSelectableItemAbove(inx int) int {
	for i := inx - 1; i >= 0; i-- {
		if _, ok := any(l.items[i]).(layout.Focusable); ok {
			return i
		}
	}
	if inx == 0 && l.wrap {
		return l.firstSelectableItemAbove(len(l.items))
	}
	return ItemNotFound
}

func (l *list[T]) firstSelectableItemBelow(inx int) int {
	for i := inx + 1; i < len(l.items); i++ {
		if _, ok := any(l.items[i]).(layout.Focusable); ok {
			return i
		}
	}
	if inx == len(l.items)-1 && l.wrap {
		return l.firstSelectableItemBelow(-1)
	}
	return ItemNotFound
}

func (l *list[T]) focusSelectedItem() tea.Cmd {
	if l.selectedItem == "" || !l.focused {
		return nil
	}
	var cmds []tea.Cmd
	for _, item := range l.items {
		if f, ok := any(item).(layout.Focusable); ok {
			if item.ID() == l.selectedItem && !f.IsFocused() {
				cmds = append(cmds, f.Focus())
				delete(l.renderedItems, item.ID())
			} else if item.ID() != l.selectedItem && f.IsFocused() {
				cmds = append(cmds, f.Blur())
				delete(l.renderedItems, item.ID())
			}
		}
	}
	return tea.Batch(cmds...)
}

func (l *list[T]) blurSelectedItem() tea.Cmd {
	if l.selectedItem == "" || l.focused {
		return nil
	}
	var cmds []tea.Cmd
	for _, item := range l.items {
		if f, ok := any(item).(layout.Focusable); ok {
			if item.ID() == l.selectedItem && f.IsFocused() {
				cmds = append(cmds, f.Blur())
				delete(l.renderedItems, item.ID())
			}
		}
	}
	return tea.Batch(cmds...)
}

// render iterator renders items starting from the specific index and limits hight if limitHeight != -1
// returns the last index
func (l *list[T]) renderIterator(startInx int, limitHeight bool) int {
	currentContentHeight := lipgloss.Height(l.rendered) - 1
	for i := startInx; i < len(l.items); i++ {
		if currentContentHeight >= l.height && limitHeight {
			return i
		}
		// cool way to go through the list in both directions
		inx := i

		if l.direction != DirectionForward {
			inx = (len(l.items) - 1) - i
		}

		item := l.items[inx]
		var rItem renderedItem
		if cache, ok := l.renderedItems[item.ID()]; ok {
			rItem = cache
		} else {
			rItem = l.renderItem(item)
			rItem.start = currentContentHeight
			rItem.end = currentContentHeight + rItem.height - 1
			l.renderedItems[item.ID()] = rItem
		}
		gap := l.gap + 1
		if inx == len(l.items)-1 {
			gap = 0
		}

		if l.direction == DirectionForward {
			l.rendered += rItem.view + strings.Repeat("\n", gap)
		} else {
			l.rendered = rItem.view + strings.Repeat("\n", gap) + l.rendered
		}
		currentContentHeight = rItem.end + 1 + l.gap
	}
	return len(l.items)
}

func (l *list[T]) renderItem(item Item) renderedItem {
	view := item.View()
	return renderedItem{
		id:     item.ID(),
		view:   view,
		height: lipgloss.Height(view),
	}
}

// AppendItem implements List.
func (l *list[T]) AppendItem(T) tea.Cmd {
	panic("unimplemented")
}

// Blur implements List.
func (l *list[T]) Blur() tea.Cmd {
	l.focused = false
	return l.render()
}

// DeleteItem implements List.
func (l *list[T]) DeleteItem(string) tea.Cmd {
	panic("unimplemented")
}

// Focus implements List.
func (l *list[T]) Focus() tea.Cmd {
	l.focused = true
	return l.render()
}

// GetSize implements List.
func (l *list[T]) GetSize() (int, int) {
	return l.width, l.height
}

// GoToBottom implements List.
func (l *list[T]) GoToBottom() tea.Cmd {
	l.offset = 0
	l.direction = DirectionBackward
	l.selectedItem = ""
	return l.render()
}

// GoToTop implements List.
func (l *list[T]) GoToTop() tea.Cmd {
	l.offset = 0
	l.direction = DirectionForward
	l.selectedItem = ""
	return l.render()
}

// IsFocused implements List.
func (l *list[T]) IsFocused() bool {
	return l.focused
}

// Items implements List.
func (l *list[T]) Items() []T {
	return l.items
}

func (l *list[T]) incrementOffset(n int) {
	renderedHeight := lipgloss.Height(l.rendered)
	// no need for offset
	if renderedHeight <= l.height {
		return
	}
	maxOffset := renderedHeight - l.height
	n = min(n, maxOffset-l.offset)
	if n <= 0 {
		return
	}
	l.offset += n
}

func (l *list[T]) decrementOffset(n int) {
	n = min(n, l.offset)
	if n <= 0 {
		return
	}
	l.offset -= n
	if l.offset < 0 {
		l.offset = 0
	}
}

// MoveDown implements List.
func (l *list[T]) MoveDown(n int) tea.Cmd {
	if l.direction == DirectionForward {
		l.incrementOffset(n)
	} else {
		l.decrementOffset(n)
	}
	return l.changeSelectionWhenScrolling()
}

// MoveUp implements List.
func (l *list[T]) MoveUp(n int) tea.Cmd {
	if l.direction == DirectionForward {
		l.decrementOffset(n)
	} else {
		l.incrementOffset(n)
	}
	return l.changeSelectionWhenScrolling()
}

// PrependItem implements List.
func (l *list[T]) PrependItem(T) tea.Cmd {
	panic("unimplemented")
}

// SelectItemAbove implements List.
func (l *list[T]) SelectItemAbove() tea.Cmd {
	inx, ok := l.indexMap[l.selectedItem]
	if !ok {
		return nil
	}

	newIndex := l.firstSelectableItemAbove(inx)
	if newIndex == ItemNotFound {
		// no item above
		return nil
	}
	item := l.items[newIndex]
	l.selectedItem = item.ID()
	l.movingByItem = true
	return l.render()
}

// SelectItemBelow implements List.
func (l *list[T]) SelectItemBelow() tea.Cmd {
	inx, ok := l.indexMap[l.selectedItem]
	if !ok {
		return nil
	}

	newIndex := l.firstSelectableItemBelow(inx)
	if newIndex == ItemNotFound {
		// no item above
		return nil
	}
	item := l.items[newIndex]
	l.selectedItem = item.ID()
	l.movingByItem = true
	return l.render()
}

// SelectedItem implements List.
func (l *list[T]) SelectedItem() *T {
	inx, ok := l.indexMap[l.selectedItem]
	if !ok {
		return nil
	}
	if inx > len(l.items)-1 {
		return nil
	}
	item := l.items[inx]
	return &item
}

// SetItems implements List.
func (l *list[T]) SetItems(items []T) tea.Cmd {
	l.items = items
	return l.reset()
}

// SetSelected implements List.
func (l *list[T]) SetSelected(id string) tea.Cmd {
	l.selectedItem = id
	return l.render()
}

func (l *list[T]) reset() tea.Cmd {
	var cmds []tea.Cmd
	l.rendered = ""
	l.offset = 0
	l.selectedItem = ""
	l.indexMap = make(map[string]int)
	l.renderedItems = make(map[string]renderedItem)
	for inx, item := range l.items {
		l.indexMap[item.ID()] = inx
		if l.width > 0 && l.height > 0 {
			cmds = append(cmds, item.SetSize(l.width, l.height))
		}
	}
	cmds = append(cmds, l.render())
	return tea.Batch(cmds...)
}

// SetSize implements List.
func (l *list[T]) SetSize(width int, height int) tea.Cmd {
	oldWidth := l.width
	l.width = width
	l.height = height
	if oldWidth != width {
		return l.reset()
	}
	return nil
}

// UpdateItem implements List.
func (l *list[T]) UpdateItem(string, T) tea.Cmd {
	panic("unimplemented")
}
