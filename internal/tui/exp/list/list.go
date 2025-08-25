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
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/rivo/uniseg"
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
	SelectedItemIndex() int
	Items() []T
	UpdateItem(string, T) tea.Cmd
	DeleteItem(string) tea.Cmd
	PrependItem(T) tea.Cmd
	AppendItem(T) tea.Cmd
	StartSelection(col, line int)
	EndSelection(col, line int)
	SelectionStop()
	SelectionClear()
	SelectWord(col, line int)
	SelectParagraph(col, line int)
	GetSelectedText(paddingLeft int) string
	HasSelection() bool
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

type itemPosition struct {
	height int
	start  int
	end    int
}

type confOptions struct {
	width, height int
	gap           int
	// if you are at the last item and go down it will wrap to the top
	wrap          bool
	keyMap        KeyMap
	direction     direction
	selectedIndex int
	focused       bool
	resize        bool
	enableMouse   bool
}

type list[T Item] struct {
	*confOptions

	offset int

	indexMap *csync.Map[string, int]
	items    *csync.Slice[T]

	itemPositions                []itemPosition
	virtualHeight                int
	viewCache                    *csync.Map[string, string]
	shouldCalculateItemPositions bool

	renderMu sync.Mutex
	rendered string

	movingByItem       bool
	selectionStartCol  int
	selectionStartLine int
	selectionEndCol    int
	selectionEndLine   int

	selectionActive bool
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

// WithSelectedItem sets the initially selected item in the list by ID.
// This will be converted to an index when the list is created.
func WithSelectedItem(id string) ListOption {
	return func(l *confOptions) {
		// Store temporarily, will be converted to index in New()
		l.selectedIndex = -1 // Will be resolved later
	}
}

// WithSelectedIndex sets the initially selected item in the list by index.
func WithSelectedIndex(index int) ListOption {
	return func(l *confOptions) {
		l.selectedIndex = index
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
			direction:     DirectionForward,
			keyMap:        DefaultKeyMap(),
			focused:       true,
			selectedIndex: -1, // Initialize to -1 to indicate no selection
		},
		items:                        csync.NewSliceFrom(items),
		indexMap:                     csync.NewMap[string, int](),
		itemPositions:                make([]itemPosition, len(items)),
		viewCache:                    csync.NewMap[string, string](),
		shouldCalculateItemPositions: true,
		selectionStartCol:            -1,
		selectionStartLine:           -1,
		selectionEndLine:             -1,
		selectionEndCol:              -1,
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
	// Ensure we have width and height
	if l.width <= 0 || l.height <= 0 {
		// Can't calculate positions without dimensions
		return nil
	}

	// Set size for all items
	var cmds []tea.Cmd
	for _, item := range slices.Collect(l.items.Seq()) {
		if cmd := item.SetSize(l.width, l.height); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	// Calculate positions for all items
	l.calculateItemPositions()

	// Select initial item based on direction
	if l.selectedIndex < 0 && l.items.Len() > 0 {
		if l.direction == DirectionForward {
			l.selectFirstItem()
		} else {
			l.selectLastItem()
		}
	}

	// For backward lists, we need to position at the bottom after initial render
	if l.direction == DirectionBackward && l.offset == 0 && l.items.Len() > 0 {
		// Set offset to show the bottom of the list
		if l.virtualHeight > l.height {
			l.offset = 0 // In backward mode, offset 0 means bottom
		}
	}

	// Scroll to the selected item for initial positioning
	if l.focused {
		l.scrollToSelection()
	}

	renderCmd := l.render()
	if renderCmd != nil {
		cmds = append(cmds, renderCmd)
	}

	return tea.Batch(cmds...)
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
		// Only update animations for visible items to avoid unnecessary renders
		viewStart, viewEnd := l.viewPosition()
		var needsRender bool
		var cmds []tea.Cmd

		for inx, item := range slices.Collect(l.items.Seq()) {
			if i, ok := any(item).(HasAnim); ok && i.Spinning() {
				// Check if item is visible
				isVisible := false
				if inx < len(l.itemPositions) {
					pos := l.itemPositions[inx]
					isVisible = pos.end >= viewStart && pos.start <= viewEnd
				}

				// Always update the animation state
				updated, cmd := i.Update(msg)
				cmds = append(cmds, cmd)

				// Only trigger render if the spinning item is visible
				if isVisible {
					needsRender = true
					// Clear the cache for this item so it re-renders
					if u, ok := updated.(T); ok {
						l.viewCache.Del(u.ID())
					}
				}
			}
		}

		// Only re-render if we have visible spinning items
		if needsRender {
			l.renderMu.Lock()
			l.rendered = l.renderVirtualScrolling()
			l.renderMu.Unlock()
		}

		return l, tea.Batch(cmds...)
	case tea.KeyPressMsg:
		if l.focused {
			switch {
			case key.Matches(msg, l.keyMap.Down):
				return l, l.SelectItemBelow()
			case key.Matches(msg, l.keyMap.Up):
				return l, l.SelectItemAbove()
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

// selectionView renders the highlighted selection in the view and returns it
// as a string. If textOnly is true, it won't render any styles.
func (l *list[T]) selectionView(view string, textOnly bool) string {
	t := styles.CurrentTheme()
	area := uv.Rect(0, 0, l.width, l.height)
	scr := uv.NewScreenBuffer(area.Dx(), area.Dy())
	uv.NewStyledString(view).Draw(scr, area)

	selArea := uv.Rectangle{
		Min: uv.Pos(l.selectionStartCol, l.selectionStartLine),
		Max: uv.Pos(l.selectionEndCol, l.selectionEndLine),
	}
	selArea = selArea.Canon()

	specialChars := make(map[string]bool, len(styles.SelectionIgnoreIcons))
	for _, icon := range styles.SelectionIgnoreIcons {
		specialChars[icon] = true
	}

	isNonWhitespace := func(r rune) bool {
		return r != ' ' && r != '\t' && r != 0 && r != '\n' && r != '\r'
	}

	type selectionBounds struct {
		startX, endX int
		inSelection  bool
	}
	lineSelections := make([]selectionBounds, scr.Height())

	for y := range scr.Height() {
		bounds := selectionBounds{startX: -1, endX: -1, inSelection: false}

		if y >= selArea.Min.Y && y <= selArea.Max.Y {
			bounds.inSelection = true
			if selArea.Min.Y == selArea.Max.Y {
				// Single line selection
				bounds.startX = selArea.Min.X
				bounds.endX = selArea.Max.X
			} else if y == selArea.Min.Y {
				// First line of multi-line selection
				bounds.startX = selArea.Min.X
				bounds.endX = scr.Width()
			} else if y == selArea.Max.Y {
				// Last line of multi-line selection
				bounds.startX = 0
				bounds.endX = selArea.Max.X
			} else {
				// Middle lines
				bounds.startX = 0
				bounds.endX = scr.Width()
			}
		}
		lineSelections[y] = bounds
	}

	type lineBounds struct {
		start, end int
	}
	lineTextBounds := make([]lineBounds, scr.Height())

	// First pass: find text bounds for lines that have selections
	for y := range scr.Height() {
		bounds := lineBounds{start: -1, end: -1}

		// Only process lines that might have selections
		if lineSelections[y].inSelection {
			for x := range scr.Width() {
				cell := scr.CellAt(x, y)
				if cell == nil {
					continue
				}

				cellStr := cell.String()
				if len(cellStr) == 0 {
					continue
				}

				char := rune(cellStr[0])
				isSpecial := specialChars[cellStr]

				if (isNonWhitespace(char) && !isSpecial) || cell.Style.Bg != nil {
					if bounds.start == -1 {
						bounds.start = x
					}
					bounds.end = x + 1 // Position after last character
				}
			}
		}
		lineTextBounds[y] = bounds
	}

	var selectedText strings.Builder

	// Second pass: apply selection highlighting
	for y := range scr.Height() {
		selBounds := lineSelections[y]
		if !selBounds.inSelection {
			continue
		}

		textBounds := lineTextBounds[y]
		if textBounds.start < 0 {
			if textOnly {
				// We don't want to get rid of all empty lines in text-only mode
				selectedText.WriteByte('\n')
			}

			continue // No text on this line
		}

		// Only scan within the intersection of text bounds and selection bounds
		scanStart := max(textBounds.start, selBounds.startX)
		scanEnd := min(textBounds.end, selBounds.endX)

		for x := scanStart; x < scanEnd; x++ {
			cell := scr.CellAt(x, y)
			if cell == nil {
				continue
			}

			cellStr := cell.String()
			if len(cellStr) > 0 && !specialChars[cellStr] {
				if textOnly {
					// Collect selected text without styles
					selectedText.WriteString(cell.String())
					continue
				}

				// Text selection styling, which is a Lip Gloss style. We must
				// extract the values to use in a UV style, below.
				ts := t.TextSelection

				cell = cell.Clone()
				cell.Style = cell.Style.Background(ts.GetBackground()).Foreground(ts.GetForeground())
				scr.SetCell(x, y, cell)
			}
		}

		if textOnly {
			// Make sure we add a newline after each line of selected text
			selectedText.WriteByte('\n')
		}
	}

	if textOnly {
		return strings.TrimSpace(selectedText.String())
	}

	return scr.Render()
}

// View implements List.
func (l *list[T]) View() string {
	if l.height <= 0 || l.width <= 0 {
		return ""
	}
	t := styles.CurrentTheme()

	// With virtual scrolling, rendered already contains only visible content
	view := l.rendered

	if l.resize {
		return view
	}

	view = t.S().Base.
		Height(l.height).
		Width(l.width).
		Render(view)

	if !l.hasSelection() {
		return view
	}

	return l.selectionView(view, false)
}

func (l *list[T]) viewPosition() (int, int) {
	// View position in the virtual space
	start, end := 0, 0
	if l.direction == DirectionForward {
		start = l.offset
		if l.virtualHeight > 0 {
			end = min(l.offset+l.height-1, l.virtualHeight-1)
		} else {
			end = l.offset + l.height - 1
		}
	} else {
		// For backward direction
		if l.virtualHeight > 0 {
			end = l.virtualHeight - l.offset - 1
			start = max(0, end-l.height+1)
		} else {
			end = 0
			start = 0
		}
	}
	return start, end
}

func (l *list[T]) render() tea.Cmd {
	return l.renderWithScrollToSelection(true)
}

func (l *list[T]) renderWithScrollToSelection(scrollToSelection bool) tea.Cmd {
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

	if l.shouldCalculateItemPositions {
		l.calculateItemPositions()
		l.shouldCalculateItemPositions = false
	}

	// Scroll to selected item BEFORE rendering if focused and requested
	if l.focused && scrollToSelection {
		l.scrollToSelection()
	}

	// Render only visible items
	l.renderMu.Lock()
	l.rendered = l.renderVirtualScrolling()
	l.renderMu.Unlock()

	return focusChangeCmd
}

func (l *list[T]) setDefaultSelected() {
	if l.selectedIndex < 0 {
		if l.direction == DirectionForward {
			l.selectFirstItem()
		} else {
			l.selectLastItem()
		}
	}
}

func (l *list[T]) scrollToSelection() {
	if l.selectedIndex < 0 || l.selectedIndex >= l.items.Len() {
		return
	}

	inx := l.selectedIndex
	if inx < 0 || inx >= len(l.itemPositions) {
		l.selectedIndex = -1
		l.setDefaultSelected()
		return
	}

	rItem := l.itemPositions[inx]

	start, end := l.viewPosition()

	// item bigger or equal to the viewport - show from start
	if rItem.height >= l.height {
		if l.direction == DirectionForward {
			l.offset = rItem.start
		} else {
			// For backward direction, we want to show the bottom of the item
			// offset = 0 means bottom of list is visible
			l.offset = 0
		}
		return
	}

	// if we are moving by item we want to move the offset so that the
	// whole item is visible not just portions of it
	if l.movingByItem {
		if rItem.start >= start && rItem.end <= end {
			// Item is fully visible, no need to scroll
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

	// If item is above the viewport, make it the first item
	if rItem.start < start {
		if l.direction == DirectionForward {
			l.offset = rItem.start
		} else {
			if l.virtualHeight > 0 {
				l.offset = l.virtualHeight - rItem.end
			} else {
				l.offset = 0
			}
		}
	} else if rItem.end > end {
		// If item is below the viewport, make it the last item
		if l.direction == DirectionForward {
			l.offset = max(0, rItem.end-l.height+1)
		} else {
			if l.virtualHeight > 0 {
				l.offset = max(0, l.virtualHeight-rItem.start-l.height+1)
			} else {
				l.offset = 0
			}
		}
	}
}

func (l *list[T]) changeSelectionWhenScrolling() tea.Cmd {
	if l.selectedIndex < 0 || l.selectedIndex >= len(l.itemPositions) {
		return nil
	}

	rItem := l.itemPositions[l.selectedIndex]
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
		for {
			inx := l.firstSelectableItemBelow(l.selectedIndex)
			if inx == ItemNotFound {
				return nil
			}
			if inx >= len(l.itemPositions) {
				continue
			}
			renderedItem := l.itemPositions[inx]

			// If the item is bigger than the viewport, select it
			if renderedItem.start <= start && renderedItem.end >= end {
				l.selectedIndex = inx
				return l.renderWithScrollToSelection(false)
			}
			// item is in the view
			if renderedItem.start >= start && renderedItem.start <= end {
				l.selectedIndex = inx
				return l.renderWithScrollToSelection(false)
			}
		}
	} else if itemMiddle > end {
		// select the first item in the viewport
		// the item is most likely an item coming after this item
		for {
			inx := l.firstSelectableItemAbove(l.selectedIndex)
			if inx == ItemNotFound {
				return nil
			}
			if inx >= len(l.itemPositions) {
				continue
			}
			renderedItem := l.itemPositions[inx]

			// If the item is bigger than the viewport, select it
			if renderedItem.start <= start && renderedItem.end >= end {
				l.selectedIndex = inx
				return l.renderWithScrollToSelection(false)
			}
			// item is in the view
			if renderedItem.end >= start && renderedItem.end <= end {
				l.selectedIndex = inx
				return l.renderWithScrollToSelection(false)
			}
		}
	}
	return nil
}

func (l *list[T]) selectFirstItem() {
	inx := l.firstSelectableItemBelow(-1)
	if inx != ItemNotFound {
		l.selectedIndex = inx
	}
}

func (l *list[T]) selectLastItem() {
	inx := l.firstSelectableItemAbove(l.items.Len())
	if inx != ItemNotFound {
		l.selectedIndex = inx
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
	if l.selectedIndex < 0 || !l.focused {
		return nil
	}
	var cmds []tea.Cmd
	for inx, item := range slices.Collect(l.items.Seq()) {
		if f, ok := any(item).(layout.Focusable); ok {
			if inx == l.selectedIndex && !f.IsFocused() {
				cmds = append(cmds, f.Focus())
				l.viewCache.Del(item.ID())
			} else if inx != l.selectedIndex && f.IsFocused() {
				cmds = append(cmds, f.Blur())
				l.viewCache.Del(item.ID())
			}
		}
	}
	return tea.Batch(cmds...)
}

func (l *list[T]) blurSelectedItem() tea.Cmd {
	if l.selectedIndex < 0 || l.focused {
		return nil
	}
	var cmds []tea.Cmd
	for inx, item := range slices.Collect(l.items.Seq()) {
		if f, ok := any(item).(layout.Focusable); ok {
			if inx == l.selectedIndex && f.IsFocused() {
				cmds = append(cmds, f.Blur())
				l.viewCache.Del(item.ID())
			}
		}
	}
	return tea.Batch(cmds...)
}

// calculateItemPositions calculates and caches the position and height of all items.
// This is O(n) but only called when the list structure changes significantly.
func (l *list[T]) calculateItemPositions() {
	itemsLen := l.items.Len()

	// Resize positions slice if needed
	if len(l.itemPositions) != itemsLen {
		l.itemPositions = make([]itemPosition, itemsLen)
	}

	currentHeight := 0
	// Always calculate positions in forward order (logical positions)
	for i := 0; i < itemsLen; i++ {
		item, ok := l.items.Get(i)
		if !ok {
			continue
		}

		// Get cached view or render new one
		var view string
		if cached, ok := l.viewCache.Get(item.ID()); ok {
			view = cached
		} else {
			view = item.View()
			l.viewCache.Set(item.ID(), view)
		}

		height := lipgloss.Height(view)

		l.itemPositions[i] = itemPosition{
			height: height,
			start:  currentHeight,
			end:    currentHeight + height - 1,
		}

		currentHeight += height
		if i < itemsLen-1 {
			currentHeight += l.gap
		}
	}

	l.virtualHeight = currentHeight
}

// updateItemPosition updates a single item's position and adjusts subsequent items.
// This is O(n) in worst case but only for items after the changed one.
func (l *list[T]) updateItemPosition(index int) {
	itemsLen := l.items.Len()
	if index < 0 || index >= itemsLen {
		return
	}

	item, ok := l.items.Get(index)
	if !ok {
		return
	}

	// Get new height
	view := item.View()
	l.viewCache.Set(item.ID(), view)
	newHeight := lipgloss.Height(view)

	// If height hasn't changed, no need to update
	if index < len(l.itemPositions) && l.itemPositions[index].height == newHeight {
		return
	}

	// Calculate starting position (from previous item or 0)
	var startPos int
	if index > 0 {
		startPos = l.itemPositions[index-1].end + 1 + l.gap
	}

	// Update this item
	oldHeight := 0
	if index < len(l.itemPositions) {
		oldHeight = l.itemPositions[index].height
	}
	heightDiff := newHeight - oldHeight

	l.itemPositions[index] = itemPosition{
		height: newHeight,
		start:  startPos,
		end:    startPos + newHeight - 1,
	}

	// Update all subsequent items' positions (shift by heightDiff)
	for i := index + 1; i < len(l.itemPositions); i++ {
		l.itemPositions[i].start += heightDiff
		l.itemPositions[i].end += heightDiff
	}

	// Update total height
	l.virtualHeight += heightDiff
}

// renderVirtualScrolling renders only the visible portion of the list.
func (l *list[T]) renderVirtualScrolling() string {
	if l.items.Len() == 0 {
		return ""
	}

	// Calculate viewport bounds
	viewStart, viewEnd := l.viewPosition()

	// Check if we have any positions calculated
	if len(l.itemPositions) == 0 {
		// No positions calculated yet, return empty viewport
		return ""
	}

	// Find which items are visible
	var visibleItems []struct {
		item  T
		pos   itemPosition
		index int
	}

	itemsLen := l.items.Len()
	for i := 0; i < itemsLen; i++ {
		if i >= len(l.itemPositions) {
			continue
		}

		pos := l.itemPositions[i]

		// Check if item is visible (overlaps with viewport)
		if pos.end >= viewStart && pos.start <= viewEnd {
			item, ok := l.items.Get(i)
			if !ok {
				continue
			}
			visibleItems = append(visibleItems, struct {
				item  T
				pos   itemPosition
				index int
			}{item, pos, i})
		}

		// Early exit if we've passed the viewport
		if pos.start > viewEnd {
			break
		}
	}

	// Build the rendered output
	var lines []string
	currentLine := viewStart

	for _, vis := range visibleItems {
		// Get or render the item's view
		var view string
		if cached, ok := l.viewCache.Get(vis.item.ID()); ok {
			view = cached
		} else {
			view = vis.item.View()
			l.viewCache.Set(vis.item.ID(), view)
		}

		itemLines := strings.Split(view, "\n")

		// Add gap lines before item if needed (except for first item)
		if vis.index > 0 && currentLine < vis.pos.start {
			gapLines := vis.pos.start - currentLine
			for i := 0; i < gapLines; i++ {
				lines = append(lines, "")
				currentLine++
			}
		}

		// Determine which lines of this item to include
		startLine := 0
		if vis.pos.start < viewStart {
			// Item starts before viewport, skip some lines
			startLine = viewStart - vis.pos.start
		}

		// Add the item's visible lines
		for i := startLine; i < len(itemLines) && currentLine <= viewEnd; i++ {
			lines = append(lines, itemLines[i])
			currentLine++
		}
	}

	// For content that fits entirely in viewport, don't pad with empty lines
	// Only pad if we have scrolled or if content is larger than viewport
	if l.virtualHeight > l.height || l.offset > 0 {
		// Fill remaining viewport with empty lines if needed
		for len(lines) < l.height {
			lines = append(lines, "")
		}

		// Trim to viewport height
		if len(lines) > l.height {
			lines = lines[:l.height]
		}
	}

	return strings.Join(lines, "\n")
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

	l.shouldCalculateItemPositions = true

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
		}
		// Note: We can't adjust offset based on item height here since positions aren't calculated yet
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

	// Check if we're deleting the selected item
	if l.selectedIndex == inx {
		// Adjust selection
		if inx > 0 {
			l.selectedIndex = inx - 1
		} else if l.items.Len() > 1 {
			l.selectedIndex = 0 // Will be valid after deletion
		} else {
			l.selectedIndex = -1 // No items left
		}
	} else if l.selectedIndex > inx {
		// Adjust index if selected item is after deleted item
		l.selectedIndex--
	}

	l.items.Delete(inx)
	l.viewCache.Del(id)
	// Rebuild index map
	l.indexMap = csync.NewMap[string, int]()
	for inx, item := range slices.Collect(l.items.Seq()) {
		l.indexMap.Set(item.ID(), inx)
	}

	cmd := l.render()
	if l.rendered != "" {
		renderedHeight := l.virtualHeight
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
	l.selectedIndex = -1
	l.direction = DirectionBackward
	return l.render()
}

// GoToTop implements List.
func (l *list[T]) GoToTop() tea.Cmd {
	l.offset = 0
	l.selectedIndex = -1
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
	renderedHeight := l.virtualHeight
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
	oldOffset := l.offset
	if l.direction == DirectionForward {
		l.incrementOffset(n)
	} else {
		l.decrementOffset(n)
	}

	if oldOffset == l.offset {
		// Even if offset didn't change, we might need to change selection
		// if we're at the edge of the scrollable area
		return l.changeSelectionWhenScrolling()
	}
	// if we are not actively selecting move the whole selection down
	if l.hasSelection() && !l.selectionActive {
		if l.selectionStartLine < l.selectionEndLine {
			l.selectionStartLine -= n
			l.selectionEndLine -= n
		} else {
			l.selectionStartLine -= n
			l.selectionEndLine -= n
		}
	}
	if l.selectionActive {
		if l.selectionStartLine < l.selectionEndLine {
			l.selectionStartLine -= n
		} else {
			l.selectionEndLine -= n
		}
	}
	return l.changeSelectionWhenScrolling()
}

// MoveUp implements List.
func (l *list[T]) MoveUp(n int) tea.Cmd {
	oldOffset := l.offset
	if l.direction == DirectionForward {
		l.decrementOffset(n)
	} else {
		l.incrementOffset(n)
	}

	if oldOffset == l.offset {
		// Even if offset didn't change, we might need to change selection
		// if we're at the edge of the scrollable area
		return l.changeSelectionWhenScrolling()
	}

	if l.hasSelection() && !l.selectionActive {
		if l.selectionStartLine > l.selectionEndLine {
			l.selectionStartLine += n
			l.selectionEndLine += n
		} else {
			l.selectionStartLine += n
			l.selectionEndLine += n
		}
	}
	if l.selectionActive {
		if l.selectionStartLine > l.selectionEndLine {
			l.selectionStartLine += n
		} else {
			l.selectionEndLine += n
		}
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

	l.shouldCalculateItemPositions = true

	if l.direction == DirectionForward {
		if l.offset == 0 {
			// If we're at the top, stay at the top
			cmds = append(cmds, l.render())
			cmd := l.GoToTop()
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		} else {
			// Note: We need to calculate positions to adjust offset properly
			// This is one case where we might need to calculate immediately
			l.calculateItemPositions()
			l.shouldCalculateItemPositions = false

			// Adjust offset to maintain viewport position
			// The prepended item is at index 0
			if len(l.itemPositions) > 0 {
				newItem := l.itemPositions[0]
				newLines := newItem.height
				if l.items.Len() > 1 {
					newLines += l.gap
				}
				// Increase offset to keep the same content visible
				if l.virtualHeight > 0 {
					l.offset = min(l.virtualHeight-l.height, l.offset+newLines)
				}
			}
			cmds = append(cmds, l.renderWithScrollToSelection(false))
		}
	} else {
		// For backward direction, prepending doesn't affect the offset
		// since offset is from the bottom
		cmds = append(cmds, l.render())
	}

	// Adjust selected index since we prepended
	if l.selectedIndex >= 0 {
		l.selectedIndex++
	}

	return tea.Batch(cmds...)
}

// SelectItemAbove implements List.
func (l *list[T]) SelectItemAbove() tea.Cmd {
	if l.selectedIndex < 0 {
		return nil
	}

	newIndex := l.firstSelectableItemAbove(l.selectedIndex)
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
	l.selectedIndex = newIndex
	l.movingByItem = true
	renderCmd := l.render()
	if renderCmd != nil {
		cmds = append(cmds, renderCmd)
	}
	return tea.Sequence(cmds...)
}

// SelectItemBelow implements List.
func (l *list[T]) SelectItemBelow() tea.Cmd {
	if l.selectedIndex < 0 {
		return nil
	}

	newIndex := l.firstSelectableItemBelow(l.selectedIndex)
	if newIndex == ItemNotFound {
		// no item below
		return nil
	}
	l.selectedIndex = newIndex
	l.movingByItem = true
	return l.render()
}

// SelectedItem implements List.
func (l *list[T]) SelectedItem() *T {
	if l.selectedIndex < 0 || l.selectedIndex >= l.items.Len() {
		return nil
	}
	item, ok := l.items.Get(l.selectedIndex)
	if !ok {
		return nil
	}
	return &item
}

// SelectedItemID returns the ID of the currently selected item (for testing).
func (l *list[T]) SelectedItemID() string {
	if l.selectedIndex < 0 || l.selectedIndex >= l.items.Len() {
		return ""
	}
	item, ok := l.items.Get(l.selectedIndex)
	if !ok {
		return ""
	}
	return item.ID()
}

// SelectedItemIndex returns the index of the currently selected item.
// Returns -1 if no item is selected.
func (l *list[T]) SelectedItemIndex() int {
	return l.selectedIndex
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
	inx, ok := l.indexMap.Get(id)
	if ok {
		l.selectedIndex = inx
	} else {
		l.selectedIndex = -1
	}
	return l.render()
}

func (l *list[T]) reset(selectedItemID string) tea.Cmd {
	var cmds []tea.Cmd
	l.rendered = ""
	l.offset = 0

	// Convert ID to index if provided
	if selectedItemID != "" {
		if inx, ok := l.indexMap.Get(selectedItemID); ok {
			l.selectedIndex = inx
		} else {
			l.selectedIndex = -1
		}
	} else {
		l.selectedIndex = -1
	}

	l.indexMap = csync.NewMap[string, int]()
	l.viewCache = csync.NewMap[string, string]()
	l.itemPositions = nil // Will be recalculated
	l.virtualHeight = 0
	l.shouldCalculateItemPositions = true
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
		// Get current selected item ID to preserve selection
		var selectedID string
		if l.selectedIndex >= 0 && l.selectedIndex < l.items.Len() {
			if item, ok := l.items.Get(l.selectedIndex); ok {
				selectedID = item.ID()
			}
		}
		cmd := l.reset(selectedID)
		return cmd
	}
	return nil
}

// UpdateItem implements List.
func (l *list[T]) UpdateItem(id string, item T) tea.Cmd {
	var cmds []tea.Cmd
	if inx, ok := l.indexMap.Get(id); ok {
		// Update the item
		l.items.Set(inx, item)

		// Clear cache for this item
		l.viewCache.Del(id)

		// Mark positions as dirty for recalculation
		l.shouldCalculateItemPositions = true

		// Re-render with updated positions
		cmd := l.renderWithScrollToSelection(false)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

		cmds = append(cmds, item.Init())
		if l.width > 0 && l.height > 0 {
			cmds = append(cmds, item.SetSize(l.width, l.height))
		}
	}
	return tea.Sequence(cmds...)
}

func (l *list[T]) hasSelection() bool {
	return l.selectionEndCol != l.selectionStartCol || l.selectionEndLine != l.selectionStartLine
}

// StartSelection implements List.
func (l *list[T]) StartSelection(col, line int) {
	l.selectionStartCol = col
	l.selectionStartLine = line
	l.selectionEndCol = col
	l.selectionEndLine = line
	l.selectionActive = true
}

// EndSelection implements List.
func (l *list[T]) EndSelection(col, line int) {
	if !l.selectionActive {
		return
	}
	l.selectionEndCol = col
	l.selectionEndLine = line
}

func (l *list[T]) SelectionStop() {
	l.selectionActive = false
}

func (l *list[T]) SelectionClear() {
	l.selectionStartCol = -1
	l.selectionStartLine = -1
	l.selectionEndCol = -1
	l.selectionEndLine = -1
	l.selectionActive = false
}

func (l *list[T]) findWordBoundaries(col, line int) (startCol, endCol int) {
	lines := strings.Split(l.rendered, "\n")
	for i, l := range lines {
		lines[i] = ansi.Strip(l)
	}

	if l.direction == DirectionBackward && len(lines) > l.height {
		line = ((len(lines) - 1) - l.height) + line + 1
	}

	if l.offset > 0 {
		if l.direction == DirectionBackward {
			line -= l.offset
		} else {
			line += l.offset
		}
	}

	if line < 0 || line >= len(lines) {
		return 0, 0
	}

	currentLine := lines[line]
	gr := uniseg.NewGraphemes(currentLine)
	startCol = -1
	upTo := col
	for gr.Next() {
		if gr.IsWordBoundary() && upTo > 0 {
			startCol = col - upTo + 1
		} else if gr.IsWordBoundary() && upTo < 0 {
			endCol = col - upTo + 1
			break
		}
		if upTo == 0 && gr.Str() == " " {
			return 0, 0
		}
		upTo -= 1
	}
	if startCol == -1 {
		return 0, 0
	}
	return
}

func (l *list[T]) findParagraphBoundaries(line int) (startLine, endLine int, found bool) {
	lines := strings.Split(l.rendered, "\n")
	for i, l := range lines {
		lines[i] = ansi.Strip(l)
		for _, icon := range styles.SelectionIgnoreIcons {
			lines[i] = strings.ReplaceAll(lines[i], icon, " ")
		}
	}
	if l.direction == DirectionBackward && len(lines) > l.height {
		line = (len(lines) - 1) - l.height + line + 1
	}

	if l.offset > 0 {
		if l.direction == DirectionBackward {
			line -= l.offset
		} else {
			line += l.offset
		}
	}

	// Ensure line is within bounds
	if line < 0 || line >= len(lines) {
		return 0, 0, false
	}

	if strings.TrimSpace(lines[line]) == "" {
		return 0, 0, false
	}

	// Find start of paragraph (search backwards for empty line or start of text)
	startLine = line
	for startLine > 0 && strings.TrimSpace(lines[startLine-1]) != "" {
		startLine--
	}

	// Find end of paragraph (search forwards for empty line or end of text)
	endLine = line
	for endLine < len(lines)-1 && strings.TrimSpace(lines[endLine+1]) != "" {
		endLine++
	}

	// revert the line numbers if we are in backward direction
	if l.direction == DirectionBackward && len(lines) > l.height {
		startLine = startLine - (len(lines) - 1) + l.height - 1
		endLine = endLine - (len(lines) - 1) + l.height - 1
	}
	if l.offset > 0 {
		if l.direction == DirectionBackward {
			startLine += l.offset
			endLine += l.offset
		} else {
			startLine -= l.offset
			endLine -= l.offset
		}
	}
	return startLine, endLine, true
}

// SelectWord selects the word at the given position.
func (l *list[T]) SelectWord(col, line int) {
	startCol, endCol := l.findWordBoundaries(col, line)
	l.selectionStartCol = startCol
	l.selectionStartLine = line
	l.selectionEndCol = endCol
	l.selectionEndLine = line
	l.selectionActive = false // Not actively selecting, just selected
}

// SelectParagraph selects the paragraph at the given position.
func (l *list[T]) SelectParagraph(col, line int) {
	startLine, endLine, found := l.findParagraphBoundaries(line)
	if !found {
		return
	}
	l.selectionStartCol = 0
	l.selectionStartLine = startLine
	l.selectionEndCol = l.width - 1
	l.selectionEndLine = endLine
	l.selectionActive = false // Not actively selecting, just selected
}

// HasSelection returns whether there is an active selection.
func (l *list[T]) HasSelection() bool {
	return l.hasSelection()
}

// GetSelectedText returns the currently selected text.
func (l *list[T]) GetSelectedText(paddingLeft int) string {
	if !l.hasSelection() {
		return ""
	}

	return l.selectionView(l.View(), true)
}
