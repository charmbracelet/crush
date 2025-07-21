package list

import (
	"slices"
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

type List[T Item] interface {
	util.Model
	layout.Sizeable
	layout.Focusable
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
	UpdateItem(string, T)
	DeleteItem(string)
	PrependItem(T) tea.Cmd
	AppendItem(T) tea.Cmd
}

type direction int

const (
	Forward direction = iota
	Backward
)

const (
	NotFound          = -1
	DefaultScrollSize = 2
)

type setSelectedMsg struct {
	selectedItemID string
}

type renderedItem struct {
	id     string
	view   string
	height int
}

type confOptions struct {
	width, height int
	gap           int
	// if you are at the last item and go down it will wrap to the top
	wrap         bool
	keyMap       KeyMap
	direction    direction
	selectedItem string
}
type list[T Item] struct {
	*confOptions

	focused       bool
	offset        int
	items         []T
	renderedItems []renderedItem
	rendered      string
	isReady       bool
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

// WithDirection sets the direction of the list.
func WithDirection(dir direction) listOption {
	return func(l *confOptions) {
		l.direction = dir
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

func New[T Item](items []T, opts ...listOption) List[T] {
	list := &list[T]{
		confOptions: &confOptions{
			direction: Forward,
			keyMap:    DefaultKeyMap(),
		},
		items: items,
	}
	for _, opt := range opts {
		opt(list.confOptions)
	}
	return list
}

// Init implements List.
func (l *list[T]) Init() tea.Cmd {
	var cmds []tea.Cmd
	for _, item := range l.items {
		cmd := item.Init()
		cmds = append(cmds, cmd)
	}
	cmds = append(cmds, l.renderItems())
	return tea.Batch(cmds...)
}

// Update implements List.
func (l *list[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case setSelectedMsg:
		return l, l.SetSelected(msg.selectedItemID)
	case tea.KeyPressMsg:
		if l.focused {
			switch {
			case key.Matches(msg, l.keyMap.Down):
				return l, l.MoveDown(DefaultScrollSize)
			case key.Matches(msg, l.keyMap.Up):
				return l, l.MoveUp(DefaultScrollSize)
			case key.Matches(msg, l.keyMap.DownOneItem):
				return l, l.SelectItemBelow()
			case key.Matches(msg, l.keyMap.UpOneItem):
				return l, l.SelectItemAbove()
			case key.Matches(msg, l.keyMap.HalfPageDown):
				return l, l.MoveDown(l.listHeight() / 2)
			case key.Matches(msg, l.keyMap.HalfPageUp):
				return l, l.MoveUp(l.listHeight() / 2)
			case key.Matches(msg, l.keyMap.PageDown):
				return l, l.MoveDown(l.listHeight())
			case key.Matches(msg, l.keyMap.PageUp):
				return l, l.MoveUp(l.listHeight())
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
	lines = lines[start : end+1]
	return strings.Join(lines, "\n")
}

func (l *list[T]) viewPosition() (int, int) {
	start, end := 0, 0
	renderedLines := lipgloss.Height(l.rendered) - 1
	if l.direction == Forward {
		start = max(0, l.offset)
		end = min(l.offset+l.listHeight()-1, renderedLines)
	} else {
		start = max(0, renderedLines-l.offset-l.listHeight()+1)
		end = max(0, renderedLines-l.offset)
	}
	return start, end
}

func (l *list[T]) renderItem(item Item) renderedItem {
	view := item.View()
	return renderedItem{
		id:     item.ID(),
		view:   view,
		height: lipgloss.Height(view),
	}
}

func (l *list[T]) renderView() {
	var sb strings.Builder
	for i, rendered := range l.renderedItems {
		sb.WriteString(rendered.view)
		if i < len(l.renderedItems)-1 {
			sb.WriteString(strings.Repeat("\n", l.gap+1))
		}
	}
	l.rendered = sb.String()
}

func (l *list[T]) incrementOffset(n int) {
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

func (l *list[T]) decrementOffset(n int) {
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

// changeSelectedWhenNotVisible is called so we make sure we move to the next available selected that is visible
func (l *list[T]) changeSelectedWhenNotVisible() tea.Cmd {
	var cmds []tea.Cmd
	start, end := l.viewPosition()
	currentPosition := 0
	itemWithinView := NotFound
	needsMove := false

	for i, item := range l.items {
		rendered := l.renderedItems[i]
		itemStart := currentPosition
		// we remove 1 so that we actually have the row, e.x 1 row => height 1 => start 0, end 0
		itemEnd := itemStart + rendered.height - 1
		if itemStart >= start && itemEnd <= end {
			itemWithinView = i
		}
		if item.ID() == l.selectedItem {
			// item is completely above the viewport
			if itemStart < start && itemEnd < start {
				needsMove = true
			}
			// item is completely below the viewport
			if itemStart > end && itemEnd > end {
				needsMove = true
			}
			if needsMove {
				if focusable, ok := any(item).(layout.Focusable); ok {
					cmds = append(cmds, focusable.Blur())
				}
				l.renderedItems[i] = l.renderItem(item)
			} else {
				return nil
			}
		}
		if itemWithinView != NotFound && needsMove {
			newSelection := l.items[itemWithinView]
			l.selectedItem = newSelection.ID()
			if focusable, ok := any(newSelection).(layout.Focusable); ok {
				cmds = append(cmds, focusable.Focus())
			}
			l.renderedItems[itemWithinView] = l.renderItem(newSelection)
			break
		}
		currentPosition += rendered.height + l.gap
	}
	l.renderView()
	return tea.Batch(cmds...)
}

func (l *list[T]) MoveUp(n int) tea.Cmd {
	if l.direction == Forward {
		l.decrementOffset(n)
	} else {
		l.incrementOffset(n)
	}
	return l.changeSelectedWhenNotVisible()
}

func (l *list[T]) MoveDown(n int) tea.Cmd {
	if l.direction == Forward {
		l.incrementOffset(n)
	} else {
		l.decrementOffset(n)
	}
	return l.changeSelectedWhenNotVisible()
}

func (l *list[T]) firstSelectableItemBefore(inx int) int {
	for i := inx - 1; i >= 0; i-- {
		if _, ok := any(l.items[i]).(layout.Focusable); ok {
			return i
		}
	}
	if inx == 0 && l.wrap {
		return l.firstSelectableItemBefore(len(l.items))
	}
	return NotFound
}

func (l *list[T]) firstSelectableItemAfter(inx int) int {
	for i := inx + 1; i < len(l.items); i++ {
		if _, ok := any(l.items[i]).(layout.Focusable); ok {
			return i
		}
	}
	if inx == len(l.items)-1 && l.wrap {
		return l.firstSelectableItemAfter(-1)
	}
	return NotFound
}

// moveToSelected needs to be called after the view is rendered
func (l *list[T]) moveToSelected(center bool) tea.Cmd {
	var cmds []tea.Cmd
	if l.selectedItem == "" || !l.isReady {
		return nil
	}
	currentPosition := 0
	start, end := l.viewPosition()
	for _, item := range l.renderedItems {
		if item.id == l.selectedItem {
			itemStart := currentPosition
			itemEnd := currentPosition + item.height - 1

			if start <= itemStart && itemEnd <= end {
				return nil
			}

			if center {
				viewportCenter := l.listHeight() / 2
				itemCenter := itemStart + item.height/2
				targetOffset := itemCenter - viewportCenter
				if l.direction == Forward {
					if targetOffset > l.offset {
						cmds = append(cmds, l.MoveDown(targetOffset-l.offset))
					} else if targetOffset < l.offset {
						cmds = append(cmds, l.MoveUp(l.offset-targetOffset))
					}
				} else {
					renderedHeight := lipgloss.Height(l.rendered)
					backwardTargetOffset := renderedHeight - targetOffset - l.listHeight()
					if backwardTargetOffset > l.offset {
						cmds = append(cmds, l.MoveUp(backwardTargetOffset-l.offset))
					} else if backwardTargetOffset < l.offset {
						cmds = append(cmds, l.MoveDown(l.offset-backwardTargetOffset))
					}
				}
			} else {
				if currentPosition < start {
					cmds = append(cmds, l.MoveUp(start-currentPosition))
				}
				if currentPosition > end {
					cmds = append(cmds, l.MoveDown(currentPosition-end))
				}
			}
		}
		currentPosition += item.height + l.gap
	}
	return tea.Batch(cmds...)
}

func (l *list[T]) SelectItemAbove() tea.Cmd {
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
			if focusable, ok := any(item).(layout.Focusable); ok {
				cmds = append(cmds, focusable.Blur())
			}
			// rerender the item
			l.renderedItems[i] = l.renderItem(item)
			// focus the item above
			above := l.items[inx]
			if focusable, ok := any(above).(layout.Focusable); ok {
				cmds = append(cmds, focusable.Focus())
			}
			// rerender the item
			l.renderedItems[inx] = l.renderItem(above)
			l.selectedItem = above.ID()
			break
		}
	}
	l.renderView()
	l.moveToSelected(false)
	return tea.Batch(cmds...)
}

func (l *list[T]) SelectItemBelow() tea.Cmd {
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
			if focusable, ok := any(item).(layout.Focusable); ok {
				cmds = append(cmds, focusable.Blur())
			}
			// rerender the item
			l.renderedItems[i] = l.renderItem(item)

			// focus the item below
			below := l.items[inx]
			if focusable, ok := any(below).(layout.Focusable); ok {
				cmds = append(cmds, focusable.Focus())
			}
			// rerender the item
			l.renderedItems[inx] = l.renderItem(below)
			l.selectedItem = below.ID()
			break
		}
	}

	l.renderView()
	l.moveToSelected(false)
	return tea.Batch(cmds...)
}

func (l *list[T]) GoToTop() tea.Cmd {
	if !l.isReady {
		return nil
	}
	l.offset = 0
	l.direction = Forward
	return tea.Batch(l.selectFirstItem(), l.renderForward())
}

func (l *list[T]) GoToBottom() tea.Cmd {
	if !l.isReady {
		return nil
	}
	l.offset = 0
	l.direction = Backward

	return tea.Batch(l.selectLastItem(), l.renderBackward())
}

func (l *list[T]) renderForward() tea.Cmd {
	// TODO: figure out a way to preserve items that did not change
	l.renderedItems = make([]renderedItem, 0)
	currentHeight := 0
	currentIndex := 0
	for i, item := range l.items {
		currentIndex = i
		if currentHeight-1 > l.listHeight() {
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

func (l *list[T]) renderBackward() tea.Cmd {
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
	if currentIndex == 0 {
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

func (l *list[T]) selectFirstItem() tea.Cmd {
	var cmd tea.Cmd
	inx := l.firstSelectableItemAfter(-1)
	if inx != NotFound {
		l.selectedItem = l.items[inx].ID()
		if focusable, ok := any(l.items[inx]).(layout.Focusable); ok {
			cmd = focusable.Focus()
		}
	}
	return cmd
}

func (l *list[T]) selectLastItem() tea.Cmd {
	var cmd tea.Cmd
	inx := l.firstSelectableItemBefore(len(l.items))
	if inx != NotFound {
		l.selectedItem = l.items[inx].ID()
		if focusable, ok := any(l.items[inx]).(layout.Focusable); ok {
			cmd = focusable.Focus()
		}
	}
	return cmd
}

func (l *list[T]) renderItems() tea.Cmd {
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
	if l.direction == Forward {
		return l.renderForward()
	}
	return l.renderBackward()
}

func (l *list[T]) listHeight() int {
	// for the moment its the same
	return l.height
}

func (l *list[T]) SetItems(items []T) tea.Cmd {
	l.items = items
	var cmds []tea.Cmd
	for _, item := range l.items {
		cmds = append(cmds, item.Init())
		// Set height to 0 to let the item calculate its own height
		cmds = append(cmds, item.SetSize(l.width, 0))
	}

	cmds = append(cmds, l.renderItems())
	if l.selectedItem != "" {
		cmds = append(cmds, l.moveToSelected(true))
	}
	return tea.Batch(cmds...)
}

// GetSize implements List.
func (l *list[T]) GetSize() (int, int) {
	return l.width, l.height
}

// SetSize implements List.
func (l *list[T]) SetSize(width int, height int) tea.Cmd {
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
func (l *list[T]) Blur() tea.Cmd {
	var cmd tea.Cmd
	l.focused = false
	for i, item := range l.items {
		if item.ID() != l.selectedItem {
			continue
		}
		if focusable, ok := any(item).(layout.Focusable); ok {
			cmd = focusable.Blur()
		}
		l.renderedItems[i] = l.renderItem(item)
	}
	l.renderView()
	return cmd
}

// Focus implements List.
func (l *list[T]) Focus() tea.Cmd {
	var cmd tea.Cmd
	l.focused = true
	if l.selectedItem != "" {
		for i, item := range l.items {
			if item.ID() != l.selectedItem {
				continue
			}
			if focusable, ok := any(item).(layout.Focusable); ok {
				cmd = focusable.Focus()
			}
			if len(l.renderedItems) > i {
				l.renderedItems[i] = l.renderItem(item)
			}
		}
		l.renderView()
	}
	return cmd
}

func (l *list[T]) SetSelected(id string) tea.Cmd {
	if l.selectedItem == id {
		return nil
	}
	var cmds []tea.Cmd
	for i, item := range l.items {
		if item.ID() == l.selectedItem {
			if focusable, ok := any(item).(layout.Focusable); ok {
				cmds = append(cmds, focusable.Blur())
			}
			if len(l.renderedItems) > i {
				l.renderedItems[i] = l.renderItem(item)
			}
		} else if item.ID() == id {
			if focusable, ok := any(item).(layout.Focusable); ok {
				cmds = append(cmds, focusable.Focus())
			}
			if len(l.renderedItems) > i {
				l.renderedItems[i] = l.renderItem(item)
			}
		}
	}
	l.selectedItem = id
	l.renderView()
	cmds = append(cmds, l.moveToSelected(true))
	return tea.Batch(cmds...)
}

func (l *list[T]) SelectedItem() *T {
	for _, item := range l.items {
		if item.ID() == l.selectedItem {
			return &item
		}
	}
	return nil
}

// IsFocused implements List.
func (l *list[T]) IsFocused() bool {
	return l.focused
}

func (l *list[T]) Items() []T {
	return l.items
}

func (l *list[T]) UpdateItem(id string, item T) {
	// TODO: preserve offset
	for inx, item := range l.items {
		if item.ID() == id {
			l.items[inx] = item
			l.renderedItems[inx] = l.renderItem(item)
			l.renderView()
			return
		}
	}
}

func (l *list[T]) DeleteItem(id string) {
	// TODO: preserve offset
	inx := NotFound
	for i, item := range l.items {
		if item.ID() == id {
			inx = i
			break
		}
	}

	l.items = slices.Delete(l.items, inx, inx+1)
	l.renderedItems = slices.Delete(l.renderedItems, inx, inx+1)
	l.renderView()
}

func (l *list[T]) PrependItem(item T) tea.Cmd {
	// TODO: preserve offset
	var cmd tea.Cmd
	l.items = append([]T{item}, l.items...)
	l.renderedItems = append([]renderedItem{l.renderItem(item)}, l.renderedItems...)
	if len(l.items) == 1 {
		cmd = l.SetSelected(item.ID())
	}
	// the viewport did not move and the last item was focused
	if l.direction == Backward && l.offset == 0 && l.selectedItem == l.items[0].ID() {
		cmd = l.SetSelected(item.ID())
	}
	l.renderView()
	return cmd
}

func (l *list[T]) AppendItem(item T) tea.Cmd {
	// TODO: preserve offset
	var cmd tea.Cmd
	l.items = append(l.items, item)
	l.renderedItems = append(l.renderedItems, l.renderItem(item))
	if len(l.items) == 1 {
		cmd = l.SetSelected(item.ID())
	} else if l.direction == Backward && l.offset == 0 && l.selectedItem == l.items[len(l.items)-2].ID() {
		// the viewport did not move and the last item was focused
		cmd = l.SetSelected(item.ID())
	} else {
		l.renderView()
	}
	return cmd
}
