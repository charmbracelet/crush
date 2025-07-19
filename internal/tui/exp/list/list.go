package list

import (
	"strings"

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

type List interface {
	util.Model
	layout.Sizeable
	layout.Focusable
	SetItems(items []Item) tea.Cmd
}

type direction int

const (
	Forward direction = iota
	Backward
)

const (
	NotFound = -1
)

type renderedItem struct {
	id     string
	view   string
	height int
}

type list struct {
	width, height int
	offset        int
	gap           int
	direction     direction
	selectedItem  string
	focused       bool

	items         []Item
	renderedItems []renderedItem
	rendered      string
	isReady       bool
}

type listOption func(*list)

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

// WithDirection sets the direction of the list.
func WithDirection(dir direction) listOption {
	return func(l *list) {
		l.direction = dir
	}
}

// WithSelectedItem sets the initially selected item in the list.
func WithSelectedItem(id string) listOption {
	return func(l *list) {
		l.selectedItem = id
	}
}

func New(opts ...listOption) List {
	list := &list{
		items:     make([]Item, 0),
		direction: Forward,
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
	if len(l.items) == 0 {
		return nil
	}
	var cmds []tea.Cmd
	for _, item := range l.items {
		cmd := item.Init()
		cmds = append(cmds, cmd)
	}
	cmds = append(cmds, l.renderItems())
	return tea.Batch(cmds...)
}

// Update implements List.
func (l *list) Update(tea.Msg) (tea.Model, tea.Cmd) {
	return l, nil
}

// View implements List.
func (l *list) View() string {
	if l.height <= 0 || l.width <= 0 {
		return ""
	}
	view := l.rendered
	lines := strings.Split(view, "\n")

	start, end := l.viewPosition(len(lines))
	lines = lines[start:end]
	return strings.Join(lines, "\n")
}

func (l *list) viewPosition(total int) (int, int) {
	start, end := 0, 0
	if l.direction == Forward {
		start = max(0, l.offset)
		end = min(l.offset+l.listHeight(), total)
	} else {
		start = max(0, total-l.offset-l.listHeight())
		end = max(0, total-l.offset)
	}
	return start, end
}

func (l *list) renderItem(item Item) renderedItem {
	view := item.View()
	return renderedItem{
		id:     item.ID(),
		view:   view,
		height: lipgloss.Height(view),
	}
}

func (l *list) renderView() {
	var sb strings.Builder
	for i, rendered := range l.renderedItems {
		sb.WriteString(rendered.view)
		if i < len(l.renderedItems)-1 {
			sb.WriteString(strings.Repeat("\n", l.gap+1))
		}
	}
	l.rendered = sb.String()
}

func (l *list) incrementOffset(n int) {
	if !l.isReady {
		return
	}
	renderedHeight := lipgloss.Height(l.rendered)
	// no need for offset
	if renderedHeight <= l.listHeight() {
		return
	}
	maxOffset := renderedHeight - l.listHeight()
	n = min(n, maxOffset-l.offset)
	if n <= 0 {
		return
	}
	l.offset += n
}

func (l *list) decrementOffset(n int) {
	if !l.isReady {
		return
	}
	n = min(n, l.offset)
	if n <= 0 {
		return
	}
	l.offset -= n
	if l.offset < 0 {
		l.offset = 0
	}
}

func (l *list) MoveUp(n int) {
	if l.direction == Forward {
		l.decrementOffset(n)
	} else {
		l.incrementOffset(n)
	}
}

func (l *list) MoveDown(n int) {
	if l.direction == Forward {
		l.incrementOffset(n)
	} else {
		l.decrementOffset(n)
	}
}

func (l *list) firstSelectableItemBefore(inx int) int {
	for i := inx - 1; i >= 0; i-- {
		if _, ok := l.items[i].(layout.Focusable); ok {
			return i
		}
	}
	return NotFound
}

func (l *list) firstSelectableItemAfter(inx int) int {
	for i := inx + 1; i < len(l.items); i++ {
		if _, ok := l.items[i].(layout.Focusable); ok {
			return i
		}
	}
	return NotFound
}

func (l *list) moveToSelected() {
	if l.selectedItem == "" || !l.isReady {
		return
	}
	currentPosition := 0
	start, end := l.viewPosition(lipgloss.Height(l.rendered))
	for _, item := range l.renderedItems {
		if item.id == l.selectedItem {
			if start <= currentPosition && currentPosition <= end {
				return
			}
			// we need to go up
			if currentPosition < start {
				l.MoveUp(start - currentPosition)
			}
			// we need to go down
			if currentPosition > end {
				l.MoveDown(currentPosition - end)
			}
		}
		currentPosition += item.height + l.gap
	}
}

func (l *list) SelectItemAbove() tea.Cmd {
	if !l.isReady {
		return nil
	}
	var cmds []tea.Cmd
	for i, item := range l.items {
		if l.selectedItem == item.ID() {
			inx := l.firstSelectableItemBefore(i)
			if inx == NotFound {
				// no item above
				return nil
			}
			// blur the current item
			if focusable, ok := item.(layout.Focusable); ok {
				cmds = append(cmds, focusable.Blur())
			}
			// rerender the item
			l.renderedItems[i] = l.renderItem(item)
			// focus the item above
			above := l.items[inx]
			if focusable, ok := above.(layout.Focusable); ok {
				cmds = append(cmds, focusable.Focus())
			}
			// rerender the item
			l.renderedItems[inx] = l.renderItem(above)
			l.selectedItem = above.ID()
			break
		}
	}
	l.renderView()
	l.moveToSelected()
	return tea.Batch(cmds...)
}

func (l *list) SelectItemBelow() tea.Cmd {
	if !l.isReady {
		return nil
	}
	var cmds []tea.Cmd
	for i, item := range l.items {
		if l.selectedItem == item.ID() {
			inx := l.firstSelectableItemAfter(i)
			if inx == NotFound {
				// no item below
				return nil
			}
			// blur the current item
			if focusable, ok := item.(layout.Focusable); ok {
				cmds = append(cmds, focusable.Blur())
			}
			// rerender the item
			l.renderedItems[i] = l.renderItem(item)

			// focus the item below
			below := l.items[inx]
			if focusable, ok := below.(layout.Focusable); ok {
				cmds = append(cmds, focusable.Focus())
			}
			// rerender the item
			l.renderedItems[inx] = l.renderItem(below)
			l.selectedItem = below.ID()
			break
		}
	}

	l.renderView()
	l.moveToSelected()
	return tea.Batch(cmds...)
}

func (l *list) GoToTop() tea.Cmd {
	if !l.isReady {
		return nil
	}
	l.offset = 0
	l.direction = Forward
	return tea.Batch(l.selectFirstItem(), l.renderForward())
}

func (l *list) GoToBottom() tea.Cmd {
	if !l.isReady {
		return nil
	}
	l.offset = 0
	l.direction = Backward

	return tea.Batch(l.selectLastItem(), l.renderBackward())
}

func (l *list) renderForward() tea.Cmd {
	// TODO: figure out a way to preserve items that did not change
	l.renderedItems = make([]renderedItem, 0)
	currentHeight := 0
	currentIndex := 0
	for i, item := range l.items {
		currentIndex = i
		if currentHeight > l.listHeight() {
			break
		}
		rendered := l.renderItem(item)
		l.renderedItems = append(l.renderedItems, rendered)
		currentHeight += rendered.height + l.gap
	}

	// initial render
	l.renderView()

	if currentIndex == len(l.items)-1 {
		l.isReady = true
		return nil
	}
	// render the rest
	return func() tea.Msg {
		for i := currentIndex; i < len(l.items); i++ {
			rendered := l.renderItem(l.items[i])
			l.renderedItems = append(l.renderedItems, rendered)
		}
		l.renderView()
		l.isReady = true
		return nil
	}
}

func (l *list) renderBackward() tea.Cmd {
	// TODO: figure out a way to preserve items that did not change
	l.renderedItems = make([]renderedItem, 0)
	currentHeight := 0
	currentIndex := 0
	for i := len(l.items) - 1; i >= 0; i-- {
		currentIndex = i
		if currentHeight > l.listHeight() {
			break
		}
		rendered := l.renderItem(l.items[i])
		l.renderedItems = append([]renderedItem{rendered}, l.renderedItems...)
		currentHeight += rendered.height + l.gap
	}
	// initial render
	l.renderView()
	if currentIndex == len(l.items)-1 {
		l.isReady = true
		return nil
	}
	return func() tea.Msg {
		for i := currentIndex; i >= 0; i-- {
			rendered := l.renderItem(l.items[i])
			l.renderedItems = append([]renderedItem{rendered}, l.renderedItems...)
		}
		l.renderView()
		l.isReady = true
		return nil
	}
}

func (l *list) selectFirstItem() tea.Cmd {
	var cmd tea.Cmd
	inx := l.firstSelectableItemAfter(-1)
	if inx != NotFound {
		l.selectedItem = l.items[inx].ID()
		if focusable, ok := l.items[inx].(layout.Focusable); ok {
			cmd = focusable.Focus()
		}
	}
	return cmd
}

func (l *list) selectLastItem() tea.Cmd {
	var cmd tea.Cmd
	inx := l.firstSelectableItemBefore(len(l.items))
	if inx != NotFound {
		l.selectedItem = l.items[inx].ID()
		if focusable, ok := l.items[inx].(layout.Focusable); ok {
			cmd = focusable.Focus()
		}
	}
	return cmd
}

func (l *list) renderItems() tea.Cmd {
	if l.height <= 0 || l.width <= 0 {
		return nil
	}
	if len(l.items) == 0 {
		return nil
	}

	if l.selectedItem == "" {
		if l.direction == Forward {
			l.selectFirstItem()
		} else {
			l.selectLastItem()
		}
	}
	return l.renderBackward()
}

func (l *list) listHeight() int {
	// for the moment its the same
	return l.height
}

func (l *list) SetItems(items []Item) tea.Cmd {
	l.items = items
	var cmds []tea.Cmd
	for _, item := range l.items {
		cmds = append(cmds, item.Init())
		// Set height to 0 to let the item calculate its own height
		cmds = append(cmds, item.SetSize(l.width, 0))
	}
	cmds = append(cmds, l.renderItems())
	return tea.Batch(cmds...)
}

// GetSize implements List.
func (l *list) GetSize() (int, int) {
	return l.width, l.height
}

// SetSize implements List.
func (l *list) SetSize(width int, height int) tea.Cmd {
	l.width = width
	l.height = height
	var cmds []tea.Cmd
	for _, item := range l.items {
		cmds = append(cmds, item.SetSize(width, height))
	}
	cmds = append(cmds, l.renderItems())
	return tea.Batch(cmds...)
}

// Blur implements List.
func (l *list) Blur() tea.Cmd {
	var cmd tea.Cmd
	l.focused = false
	for i, item := range l.items {
		if item.ID() != l.selectedItem {
			continue
		}
		if focusable, ok := item.(layout.Focusable); ok {
			cmd = focusable.Blur()
		}
		l.renderedItems[i] = l.renderItem(item)
	}
	l.renderView()
	return cmd
}

// Focus implements List.
func (l *list) Focus() tea.Cmd {
	var cmd tea.Cmd
	l.focused = true
	for i, item := range l.items {
		if item.ID() != l.selectedItem {
			continue
		}
		if focusable, ok := item.(layout.Focusable); ok {
			cmd = focusable.Focus()
		}
		l.renderedItems[i] = l.renderItem(item)
	}
	l.renderView()
	return cmd
}

// IsFocused implements List.
func (l *list) IsFocused() bool {
	return l.focused
}
