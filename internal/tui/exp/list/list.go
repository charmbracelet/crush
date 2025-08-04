package list

import (
	"slices"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/tui/components/anim"
	"github.com/charmbracelet/crush/internal/tui/components/core/layout"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/lipgloss/v2"
)

type Item interface {
	util.Model
	layout.Sizeable
	ID() string
}

type HasAnim interface {
	Item
	Spinning() bool
}

type List[T Item] interface {
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
	resize       bool
	enableMouse  bool
}

type list[T Item] struct {
	*confOptions

	offset int

	indexMap *csync.Map[string, int]
	items    *csync.Slice[T]

	renderedItems *csync.Map[string, renderedItem]

	renderMu sync.Mutex
	rendered string

	movingByItem bool
}

type ListOption func(*confOptions)

// WithSize sets the size of the list.
func WithSize(width, height int) ListOption {
	return func(l *confOptions) {
		l.width = width
		l.height = height
	}
}

// WithGap sets the gap between items in the list.
func WithGap(gap int) ListOption {
	return func(l *confOptions) {
		l.gap = gap
	}
}

// WithDirectionForward sets the direction to forward
func WithDirectionForward() ListOption {
	return func(l *confOptions) {
		l.direction = DirectionForward
	}
}

// WithDirectionBackward sets the direction to forward
func WithDirectionBackward() ListOption {
	return func(l *confOptions) {
		l.direction = DirectionBackward
	}
}

// WithSelectedItem sets the initially selected item in the list.
func WithSelectedItem(id string) ListOption {
	return func(l *confOptions) {
		l.selectedItem = id
	}
}

func WithKeyMap(keyMap KeyMap) ListOption {
	return func(l *confOptions) {
		l.keyMap = keyMap
	}
}

func WithWrapNavigation() ListOption {
	return func(l *confOptions) {
		l.wrap = true
	}
}

func WithFocus(focus bool) ListOption {
	return func(l *confOptions) {
		l.focused = focus
	}
}

func WithResizeByList() ListOption {
	return func(l *confOptions) {
		l.resize = true
	}
}

func WithEnableMouse() ListOption {
	return func(l *confOptions) {
		l.enableMouse = true
	}
}

func New[T Item](items []T, opts ...ListOption) List[T] {
	list := &list[T]{
		confOptions: &confOptions{
			direction: DirectionForward,
			keyMap:    DefaultKeyMap(),
			focused:   true,
		},
		items:         csync.NewSliceFrom(items),
		indexMap:      csync.NewMap[string, int](),
		renderedItems: csync.NewMap[string, renderedItem](),
	}
	for _, opt := range opts {
		opt(list.confOptions)
	}

	for inx, item := range items {
		if i, ok := any(item).(Indexable); ok {
			i.SetIndex(inx)
		}
		list.indexMap.Set(item.ID(), inx)
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
	case tea.MouseWheelMsg:
		if l.enableMouse {
			return l.handleMouseWheel(msg)
		}
		return l, nil
	case anim.StepMsg:
		var cmds []tea.Cmd
		for _, item := range slices.Collect(l.items.Seq()) {
			if i, ok := any(item).(HasAnim); ok && i.Spinning() {
				updated, cmd := i.Update(msg)
				cmds = append(cmds, cmd)
				if u, ok := updated.(T); ok {
					cmds = append(cmds, l.UpdateItem(u.ID(), u))
				}
			}
		}
		return l, tea.Batch(cmds...)
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
			s := l.SelectedItem()
			if s == nil {
				return l, nil
			}
			item := *s
			var cmds []tea.Cmd
			updated, cmd := item.Update(msg)
			cmds = append(cmds, cmd)
			if u, ok := updated.(T); ok {
				cmds = append(cmds, l.UpdateItem(u.ID(), u))
			}
			return l, tea.Batch(cmds...)
		}
	}
	return l, nil
}

func (l *list[T]) handleMouseWheel(msg tea.MouseWheelMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.Button {
	case tea.MouseWheelDown:
		cmd = l.MoveDown(ViewportDefaultScrollSize)
	case tea.MouseWheelUp:
		cmd = l.MoveUp(ViewportDefaultScrollSize)
	}
	return l, cmd
}

// View implements List.
func (l *list[T]) View() string {
	if l.height <= 0 || l.width <= 0 {
		return ""
	}
	t := styles.CurrentTheme()
	view := l.rendered
	lines := strings.Split(view, "\n")

	start, end := l.viewPosition()
	viewStart := max(0, start)
	viewEnd := min(len(lines), end+1)
	lines = lines[viewStart:viewEnd]
	if l.resize {
		return strings.Join(lines, "\n")
	}
	return t.S().Base.
		Height(l.height).
		Width(l.width).
		Render(strings.Join(lines, "\n"))
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
	for _, item := range slices.Collect(l.items.Seq()) {
		rItem, ok := l.renderedItems.Get(item.ID())
		if !ok {
			continue
		}
		rItem.start = currentContentHeight
		rItem.end = currentContentHeight + rItem.height - 1
		l.renderedItems.Set(item.ID(), rItem)
		currentContentHeight = rItem.end + 1 + l.gap
	}
}

func (l *list[T]) render() tea.Cmd {
	if l.width <= 0 || l.height <= 0 || l.items.Len() == 0 {
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
		// rerender everything will mostly hit cache
		l.renderMu.Lock()
		l.rendered, _ = l.renderIterator(0, false, "")
		l.renderMu.Unlock()
		if l.direction == DirectionBackward {
			l.recalculateItemPositions()
		}
		// in the end scroll to the selected item
		if l.focused {
			l.scrollToSelection()
		}
		return focusChangeCmd
	}
	l.renderMu.Lock()
	rendered, finishIndex := l.renderIterator(0, true, "")
	l.rendered = rendered
	l.renderMu.Unlock()
	// recalculate for the initial items
	if l.direction == DirectionBackward {
		l.recalculateItemPositions()
	}
	renderCmd := func() tea.Msg {
		l.offset = 0
		// render the rest

		l.renderMu.Lock()
		l.rendered, _ = l.renderIterator(finishIndex, false, l.rendered)
		l.renderMu.Unlock()
		// needed for backwards
		if l.direction == DirectionBackward {
			l.recalculateItemPositions()
		}
		// in the end scroll to the selected item
		if l.focused {
			l.scrollToSelection()
		}
		return nil
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
	rItem, ok := l.renderedItems.Get(l.selectedItem)
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
	rItem, ok := l.renderedItems.Get(l.selectedItem)
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
		inx, ok := l.indexMap.Get(rItem.id)
		if !ok {
			return nil
		}
		for {
			inx = l.firstSelectableItemBelow(inx)
			if inx == ItemNotFound {
				return nil
			}
			item, ok := l.items.Get(inx)
			if !ok {
				continue
			}
			renderedItem, ok := l.renderedItems.Get(item.ID())
			if !ok {
				continue
			}

			// If the item is bigger than the viewport, select it
			if renderedItem.start <= start && renderedItem.end >= end {
				l.selectedItem = renderedItem.id
				return l.render()
			}
			// item is in the view
			if renderedItem.start >= start && renderedItem.start <= end {
				l.selectedItem = renderedItem.id
				return l.render()
			}
		}
	} else if itemMiddle > end {
		// select the first item in the viewport
		// the item is most likely an item coming after this item
		inx, ok := l.indexMap.Get(rItem.id)
		if !ok {
			return nil
		}
		for {
			inx = l.firstSelectableItemAbove(inx)
			if inx == ItemNotFound {
				return nil
			}
			item, ok := l.items.Get(inx)
			if !ok {
				continue
			}
			renderedItem, ok := l.renderedItems.Get(item.ID())
			if !ok {
				continue
			}

			// If the item is bigger than the viewport, select it
			if renderedItem.start <= start && renderedItem.end >= end {
				l.selectedItem = renderedItem.id
				return l.render()
			}
			// item is in the view
			if renderedItem.end >= start && renderedItem.end <= end {
				l.selectedItem = renderedItem.id
				return l.render()
			}
		}
	}
	return nil
}

func (l *list[T]) selectFirstItem() {
	inx := l.firstSelectableItemBelow(-1)
	if inx != ItemNotFound {
		item, ok := l.items.Get(inx)
		if ok {
			l.selectedItem = item.ID()
		}
	}
}

func (l *list[T]) selectLastItem() {
	inx := l.firstSelectableItemAbove(l.items.Len())
	if inx != ItemNotFound {
		item, ok := l.items.Get(inx)
		if ok {
			l.selectedItem = item.ID()
		}
	}
}

func (l *list[T]) firstSelectableItemAbove(inx int) int {
	for i := inx - 1; i >= 0; i-- {
		item, ok := l.items.Get(i)
		if !ok {
			continue
		}
		if _, ok := any(item).(layout.Focusable); ok {
			return i
		}
	}
	if inx == 0 && l.wrap {
		return l.firstSelectableItemAbove(l.items.Len())
	}
	return ItemNotFound
}

func (l *list[T]) firstSelectableItemBelow(inx int) int {
	itemsLen := l.items.Len()
	for i := inx + 1; i < itemsLen; i++ {
		item, ok := l.items.Get(i)
		if !ok {
			continue
		}
		if _, ok := any(item).(layout.Focusable); ok {
			return i
		}
	}
	if inx == itemsLen-1 && l.wrap {
		return l.firstSelectableItemBelow(-1)
	}
	return ItemNotFound
}

func (l *list[T]) focusSelectedItem() tea.Cmd {
	if l.selectedItem == "" || !l.focused {
		return nil
	}
	var cmds []tea.Cmd
	for _, item := range slices.Collect(l.items.Seq()) {
		if f, ok := any(item).(layout.Focusable); ok {
			if item.ID() == l.selectedItem && !f.IsFocused() {
				cmds = append(cmds, f.Focus())
				l.renderedItems.Del(item.ID())
			} else if item.ID() != l.selectedItem && f.IsFocused() {
				cmds = append(cmds, f.Blur())
				l.renderedItems.Del(item.ID())
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
	for _, item := range slices.Collect(l.items.Seq()) {
		if f, ok := any(item).(layout.Focusable); ok {
			if item.ID() == l.selectedItem && f.IsFocused() {
				cmds = append(cmds, f.Blur())
				l.renderedItems.Del(item.ID())
			}
		}
	}
	return tea.Batch(cmds...)
}

// render iterator renders items starting from the specific index and limits hight if limitHeight != -1
// returns the last index and the rendered content so far
// we pass the rendered content around and don't use l.rendered to prevent jumping of the content
func (l *list[T]) renderIterator(startInx int, limitHeight bool, rendered string) (string, int) {
	currentContentHeight := lipgloss.Height(rendered) - 1
	itemsLen := l.items.Len()
	for i := startInx; i < itemsLen; i++ {
		if currentContentHeight >= l.height && limitHeight {
			return rendered, i
		}
		// cool way to go through the list in both directions
		inx := i

		if l.direction != DirectionForward {
			inx = (itemsLen - 1) - i
		}

		item, ok := l.items.Get(inx)
		if !ok {
			continue
		}
		var rItem renderedItem
		if cache, ok := l.renderedItems.Get(item.ID()); ok {
			rItem = cache
		} else {
			rItem = l.renderItem(item)
			rItem.start = currentContentHeight
			rItem.end = currentContentHeight + rItem.height - 1
			l.renderedItems.Set(item.ID(), rItem)
		}
		gap := l.gap + 1
		if inx == itemsLen-1 {
			gap = 0
		}

		if l.direction == DirectionForward {
			rendered += rItem.view + strings.Repeat("\n", gap)
		} else {
			rendered = rItem.view + strings.Repeat("\n", gap) + rendered
		}
		currentContentHeight = rItem.end + 1 + l.gap
	}
	return rendered, itemsLen
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
func (l *list[T]) AppendItem(item T) tea.Cmd {
	var cmds []tea.Cmd
	cmd := item.Init()
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	l.items.Append(item)
	l.indexMap = csync.NewMap[string, int]()
	for inx, item := range slices.Collect(l.items.Seq()) {
		l.indexMap.Set(item.ID(), inx)
	}
	if l.width > 0 && l.height > 0 {
		cmd = item.SetSize(l.width, l.height)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	cmd = l.render()
	if cmd != nil {
		cmds = append(cmds, cmd)
	}
	if l.direction == DirectionBackward {
		if l.offset == 0 {
			cmd = l.GoToBottom()
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		} else {
			newItem, ok := l.renderedItems.Get(item.ID())
			if ok {
				newLines := newItem.height
				if l.items.Len() > 1 {
					newLines += l.gap
				}
				l.offset = min(lipgloss.Height(l.rendered)-1, l.offset+newLines)
			}
		}
	}
	return tea.Sequence(cmds...)
}

// Blur implements List.
func (l *list[T]) Blur() tea.Cmd {
	l.focused = false
	return l.render()
}

// DeleteItem implements List.
func (l *list[T]) DeleteItem(id string) tea.Cmd {
	inx, ok := l.indexMap.Get(id)
	if !ok {
		return nil
	}
	l.items.Delete(inx)
	l.renderedItems.Del(id)
	for inx, item := range slices.Collect(l.items.Seq()) {
		l.indexMap.Set(item.ID(), inx)
	}

	if l.selectedItem == id {
		if inx > 0 {
			item, ok := l.items.Get(inx - 1)
			if ok {
				l.selectedItem = item.ID()
			} else {
				l.selectedItem = ""
			}
		} else {
			l.selectedItem = ""
		}
	}
	cmd := l.render()
	if l.rendered != "" {
		renderedHeight := lipgloss.Height(l.rendered)
		if renderedHeight <= l.height {
			l.offset = 0
		} else {
			maxOffset := renderedHeight - l.height
			if l.offset > maxOffset {
				l.offset = maxOffset
			}
		}
	}
	return cmd
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
	l.selectedItem = ""
	l.direction = DirectionBackward
	return l.render()
}

// GoToTop implements List.
func (l *list[T]) GoToTop() tea.Cmd {
	l.offset = 0
	l.selectedItem = ""
	l.direction = DirectionForward
	return l.render()
}

// IsFocused implements List.
func (l *list[T]) IsFocused() bool {
	return l.focused
}

// Items implements List.
func (l *list[T]) Items() []T {
	return slices.Collect(l.items.Seq())
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
func (l *list[T]) PrependItem(item T) tea.Cmd {
	cmds := []tea.Cmd{
		item.Init(),
	}
	l.items.Prepend(item)
	l.indexMap = csync.NewMap[string, int]()
	for inx, item := range slices.Collect(l.items.Seq()) {
		l.indexMap.Set(item.ID(), inx)
	}
	if l.width > 0 && l.height > 0 {
		cmds = append(cmds, item.SetSize(l.width, l.height))
	}
	cmds = append(cmds, l.render())
	if l.direction == DirectionForward {
		if l.offset == 0 {
			cmd := l.GoToTop()
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		} else {
			newItem, ok := l.renderedItems.Get(item.ID())
			if ok {
				newLines := newItem.height
				if l.items.Len() > 1 {
					newLines += l.gap
				}
				l.offset = min(lipgloss.Height(l.rendered)-1, l.offset+newLines)
			}
		}
	}
	return tea.Batch(cmds...)
}

// SelectItemAbove implements List.
func (l *list[T]) SelectItemAbove() tea.Cmd {
	inx, ok := l.indexMap.Get(l.selectedItem)
	if !ok {
		return nil
	}

	newIndex := l.firstSelectableItemAbove(inx)
	if newIndex == ItemNotFound {
		// no item above
		return nil
	}
	var cmds []tea.Cmd
	if newIndex == 1 {
		peakAboveIndex := l.firstSelectableItemAbove(newIndex)
		if peakAboveIndex == ItemNotFound {
			// this means there is a section above move to the top
			cmd := l.GoToTop()
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}
	item, ok := l.items.Get(newIndex)
	if !ok {
		return nil
	}
	l.selectedItem = item.ID()
	l.movingByItem = true
	renderCmd := l.render()
	if renderCmd != nil {
		cmds = append(cmds, renderCmd)
	}
	return tea.Sequence(cmds...)
}

// SelectItemBelow implements List.
func (l *list[T]) SelectItemBelow() tea.Cmd {
	inx, ok := l.indexMap.Get(l.selectedItem)
	if !ok {
		return nil
	}

	newIndex := l.firstSelectableItemBelow(inx)
	if newIndex == ItemNotFound {
		// no item above
		return nil
	}
	item, ok := l.items.Get(newIndex)
	if !ok {
		return nil
	}
	l.selectedItem = item.ID()
	l.movingByItem = true
	return l.render()
}

// SelectedItem implements List.
func (l *list[T]) SelectedItem() *T {
	inx, ok := l.indexMap.Get(l.selectedItem)
	if !ok {
		return nil
	}
	if inx > l.items.Len()-1 {
		return nil
	}
	item, ok := l.items.Get(inx)
	if !ok {
		return nil
	}
	return &item
}

// SetItems implements List.
func (l *list[T]) SetItems(items []T) tea.Cmd {
	l.items.SetSlice(items)
	var cmds []tea.Cmd
	for inx, item := range slices.Collect(l.items.Seq()) {
		if i, ok := any(item).(Indexable); ok {
			i.SetIndex(inx)
		}
		cmds = append(cmds, item.Init())
	}
	cmds = append(cmds, l.reset(""))
	return tea.Batch(cmds...)
}

// SetSelected implements List.
func (l *list[T]) SetSelected(id string) tea.Cmd {
	l.selectedItem = id
	return l.render()
}

func (l *list[T]) reset(selectedItem string) tea.Cmd {
	var cmds []tea.Cmd
	l.rendered = ""
	l.offset = 0
	l.selectedItem = selectedItem
	l.indexMap = csync.NewMap[string, int]()
	l.renderedItems = csync.NewMap[string, renderedItem]()
	for inx, item := range slices.Collect(l.items.Seq()) {
		l.indexMap.Set(item.ID(), inx)
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
		cmd := l.reset(l.selectedItem)
		return cmd
	}
	return nil
}

// UpdateItem implements List.
func (l *list[T]) UpdateItem(id string, item T) tea.Cmd {
	var cmds []tea.Cmd
	if inx, ok := l.indexMap.Get(id); ok {
		l.items.Set(inx, item)
		oldItem, hasOldItem := l.renderedItems.Get(id)
		oldPosition := l.offset
		if l.direction == DirectionBackward {
			oldPosition = (lipgloss.Height(l.rendered) - 1) - l.offset
		}

		l.renderedItems.Del(id)
		cmd := l.render()

		// need to check for nil because of sequence not handling nil
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		if hasOldItem && l.direction == DirectionBackward {
			// if we are the last item and there is no offset
			// make sure to go to the bottom
			if oldPosition < oldItem.end {
				newItem, ok := l.renderedItems.Get(item.ID())
				if ok {
					newLines := newItem.height - oldItem.height
					l.offset = util.Clamp(l.offset+newLines, 0, lipgloss.Height(l.rendered)-1)
				}
			}
		} else if hasOldItem && l.offset > oldItem.start {
			newItem, ok := l.renderedItems.Get(item.ID())
			if ok {
				newLines := newItem.height - oldItem.height
				l.offset = util.Clamp(l.offset+newLines, 0, lipgloss.Height(l.rendered)-1)
			}
		}
	}
	return tea.Sequence(cmds...)
}
