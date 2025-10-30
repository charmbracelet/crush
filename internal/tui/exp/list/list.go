package list

import (
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
	"github.com/charmbracelet/x/exp/ordered"
	"github.com/rivo/uniseg"
)

const maxGapSize = 100

var newlineBuffer = strings.Repeat("\n", maxGapSize)

var (
	specialCharsMap  map[string]struct{}
	specialCharsOnce sync.Once
)

func getSpecialCharsMap() map[string]struct{} {
	specialCharsOnce.Do(func() {
		specialCharsMap = make(map[string]struct{}, len(styles.SelectionIgnoreIcons))
		for _, icon := range styles.SelectionIgnoreIcons {
			specialCharsMap[icon] = struct{}{}
		}
	})
	return specialCharsMap
}

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
	ViewportDefaultScrollSize = 5
)

type renderedItem struct {
	view   string
	height int
	start  int
	end    int
}

type confOptions struct {
	width, height int
	gap           int
	wrap          bool
	keyMap        KeyMap
	direction     direction
	selectedItem  string
	focused       bool
	resize        bool
	enableMouse   bool
}

type list[T Item] struct {
	*confOptions

	offset int

	indexMap *csync.Map[string, int]
	items    *csync.Slice[T]

	renderedItems *csync.Map[string, renderedItem]

	renderMu       sync.Mutex
	rendered       string
	renderedHeight int   // cached height of rendered content
	lineOffsets    []int // cached byte offsets for each line (for fast slicing)

	cachedView       string
	cachedViewOffset int
	cachedViewDirty  bool

	movingByItem       bool
	prevSelectedItem   string
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
		items:              csync.NewSliceFrom(items),
		indexMap:           csync.NewMap[string, int](),
		renderedItems:      csync.NewMap[string, renderedItem](),
		selectionStartCol:  -1,
		selectionStartLine: -1,
		selectionEndLine:   -1,
		selectionEndCol:    -1,
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
func (l *list[T]) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseWheelMsg:
		if l.enableMouse {
			return l.handleMouseWheel(msg)
		}
		return l, nil
	case anim.StepMsg:
		// Fast path: if no items, skip processing
		if l.items.Len() == 0 {
			return l, nil
		}

		// Fast path: check if ANY items are actually spinning before processing
		if !l.hasSpinningItems() {
			return l, nil
		}

		var cmds []tea.Cmd
		itemsLen := l.items.Len()
		for i := range itemsLen {
			item, ok := l.items.Get(i)
			if !ok {
				continue
			}
			if animItem, ok := any(item).(HasAnim); ok && animItem.Spinning() {
				updated, cmd := animItem.Update(msg)
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

func (l *list[T]) handleMouseWheel(msg tea.MouseWheelMsg) (util.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.Button {
	case tea.MouseWheelDown:
		cmd = l.MoveDown(ViewportDefaultScrollSize)
	case tea.MouseWheelUp:
		cmd = l.MoveUp(ViewportDefaultScrollSize)
	}
	return l, cmd
}

func (l *list[T]) hasSpinningItems() bool {
	itemsLen := l.items.Len()
	for i := range itemsLen {
		if item, ok := l.items.Get(i); ok {
			if animItem, ok := any(item).(HasAnim); ok && animItem.Spinning() {
				return true
			}
		}
	}
	return false
}

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

	specialChars := getSpecialCharsMap()

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
				_, isSpecial := specialChars[cellStr]

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
			if len(cellStr) > 0 {
				if _, isSpecial := specialChars[cellStr]; isSpecial {
					continue
				}
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

func (l *list[T]) View() string {
	if l.height <= 0 || l.width <= 0 {
		return ""
	}

	if !l.cachedViewDirty && l.cachedViewOffset == l.offset && !l.hasSelection() && l.cachedView != "" {
		return l.cachedView
	}

	t := styles.CurrentTheme()

	start, end := l.viewPosition()
	viewStart := max(0, start)
	viewEnd := end

	if viewStart > viewEnd {
		return ""
	}

	view := l.getLines(viewStart, viewEnd)

	if l.resize {
		return view
	}

	view = t.S().Base.
		Height(l.height).
		Width(l.width).
		Render(view)

	if !l.hasSelection() {
		l.cachedView = view
		l.cachedViewOffset = l.offset
		l.cachedViewDirty = false
		return view
	}

	return l.selectionView(view, false)
}

func (l *list[T]) viewPosition() (int, int) {
	start, end := 0, 0
	renderedLines := l.renderedHeight - 1
	if l.direction == DirectionForward {
		start = max(0, l.offset)
		end = min(l.offset+l.height-1, renderedLines)
	} else {
		start = max(0, renderedLines-l.offset-l.height+1)
		end = max(0, renderedLines-l.offset)
	}
	start = min(start, end)
	return start, end
}

func (l *list[T]) setRendered(rendered string) {
	l.rendered = rendered
	l.renderedHeight = lipgloss.Height(rendered)
	l.cachedViewDirty = true // Mark view cache as dirty

	if len(rendered) > 0 {
		l.lineOffsets = make([]int, 0, l.renderedHeight)
		l.lineOffsets = append(l.lineOffsets, 0)

		offset := 0
		for {
			idx := strings.IndexByte(rendered[offset:], '\n')
			if idx == -1 {
				break
			}
			offset += idx + 1
			l.lineOffsets = append(l.lineOffsets, offset)
		}
	} else {
		l.lineOffsets = nil
	}
}

func (l *list[T]) getLines(start, end int) string {
	if len(l.lineOffsets) == 0 || start >= len(l.lineOffsets) {
		return ""
	}

	if end >= len(l.lineOffsets) {
		end = len(l.lineOffsets) - 1
	}
	if start > end {
		return ""
	}

	startOffset := l.lineOffsets[start]
	var endOffset int
	if end+1 < len(l.lineOffsets) {
		endOffset = l.lineOffsets[end+1] - 1
	} else {
		endOffset = len(l.rendered)
	}

	if startOffset >= len(l.rendered) {
		return ""
	}
	endOffset = min(endOffset, len(l.rendered))

	return l.rendered[startOffset:endOffset]
}

func (l *list[T]) recalculateItemPositions() {
	l.recalculateItemPositionsFrom(0)
}

func (l *list[T]) recalculateItemPositionsFrom(startIdx int) {
	var currentContentHeight int

	if startIdx > 0 {
		if prevItem, ok := l.items.Get(startIdx - 1); ok {
			if rItem, ok := l.renderedItems.Get(prevItem.ID()); ok {
				currentContentHeight = rItem.end + 1 + l.gap
			}
		}
	}

	itemsLen := l.items.Len()
	for i := startIdx; i < itemsLen; i++ {
		item, ok := l.items.Get(i)
		if !ok {
			continue
		}
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
	if l.rendered != "" {
		l.renderMu.Lock()
		rendered, _ := l.renderIterator(0, false, "")
		l.setRendered(rendered)
		l.renderMu.Unlock()
		if l.direction == DirectionBackward {
			l.recalculateItemPositions()
		}
		if l.focused {
			l.scrollToSelection()
		}
		return focusChangeCmd
	}
	l.renderMu.Lock()
	rendered, finishIndex := l.renderIterator(0, true, "")
	l.setRendered(rendered)
	l.renderMu.Unlock()
	if l.direction == DirectionBackward {
		l.recalculateItemPositions()
	}
	renderCmd := func() tea.Msg {
		l.offset = 0

		l.renderMu.Lock()
		rendered, _ := l.renderIterator(finishIndex, false, l.rendered)
		l.setRendered(rendered)
		l.renderMu.Unlock()
		if l.direction == DirectionBackward {
			l.recalculateItemPositions()
		}
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
	if rItem.start <= start && rItem.end >= end {
		return
	}
	if l.movingByItem {
		if rItem.start >= start && rItem.end <= end {
			return
		}
		defer func() { l.movingByItem = false }()
	} else {
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
			l.offset = max(0, l.renderedHeight-(rItem.start+l.height))
		}
		return
	}

	renderedLines := l.renderedHeight - 1

	if rItem.start < start {
		if l.direction == DirectionForward {
			l.offset = rItem.start
		} else {
			l.offset = max(0, renderedLines-rItem.start-l.height+1)
		}
	} else if rItem.end > end {
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
		inx, ok := l.indexMap.Get(l.selectedItem)
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
				l.selectedItem = item.ID()
				return l.render()
			}
			// item is in the view
			if renderedItem.start >= start && renderedItem.start <= end {
				l.selectedItem = item.ID()
				return l.render()
			}
		}
	} else if itemMiddle > end {
		// select the first item in the viewport
		// the item is most likely an item coming after this item
		inx, ok := l.indexMap.Get(l.selectedItem)
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
				l.selectedItem = item.ID()
				return l.render()
			}
			// item is in the view
			if renderedItem.end >= start && renderedItem.end <= end {
				l.selectedItem = item.ID()
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
	// Pre-allocate with expected capacity
	cmds := make([]tea.Cmd, 0, 2)

	// Blur the previously selected item if it's different
	if l.prevSelectedItem != "" && l.prevSelectedItem != l.selectedItem {
		if prevIdx, ok := l.indexMap.Get(l.prevSelectedItem); ok {
			if prevItem, ok := l.items.Get(prevIdx); ok {
				if f, ok := any(prevItem).(layout.Focusable); ok && f.IsFocused() {
					cmds = append(cmds, f.Blur())
					// Mark cache as needing update, but don't delete yet
					// This allows the render to potentially reuse it
					l.renderedItems.Del(prevItem.ID())
				}
			}
		}
	}

	// Focus the currently selected item
	if idx, ok := l.indexMap.Get(l.selectedItem); ok {
		if item, ok := l.items.Get(idx); ok {
			if f, ok := any(item).(layout.Focusable); ok && !f.IsFocused() {
				cmds = append(cmds, f.Focus())
				// Mark for re-render
				l.renderedItems.Del(item.ID())
			}
		}
	}

	l.prevSelectedItem = l.selectedItem
	return tea.Batch(cmds...)
}

func (l *list[T]) blurSelectedItem() tea.Cmd {
	if l.selectedItem == "" || l.focused {
		return nil
	}

	// Blur the currently selected item
	if idx, ok := l.indexMap.Get(l.selectedItem); ok {
		if item, ok := l.items.Get(idx); ok {
			if f, ok := any(item).(layout.Focusable); ok && f.IsFocused() {
				l.renderedItems.Del(item.ID())
				return f.Blur()
			}
		}
	}

	return nil
}

// renderFragment holds updated rendered view fragments
type renderFragment struct {
	view string
	gap  int
}

// renderIterator renders items starting from the specific index and limits height if limitHeight != -1
// returns the last index and the rendered content so far
// we pass the rendered content around and don't use l.rendered to prevent jumping of the content
func (l *list[T]) renderIterator(startInx int, limitHeight bool, rendered string) (string, int) {
	// Pre-allocate fragments with expected capacity
	itemsLen := l.items.Len()
	expectedFragments := itemsLen - startInx
	if limitHeight && l.height > 0 {
		expectedFragments = min(expectedFragments, l.height)
	}
	fragments := make([]renderFragment, 0, expectedFragments)

	currentContentHeight := lipgloss.Height(rendered) - 1
	finalIndex := itemsLen

	// first pass: accumulate all fragments to render until the height limit is
	// reached
	for i := startInx; i < itemsLen; i++ {
		if limitHeight && currentContentHeight >= l.height {
			finalIndex = i
			break
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

		fragments = append(fragments, renderFragment{view: rItem.view, gap: gap})

		currentContentHeight = rItem.end + 1 + l.gap
	}

	// second pass: build rendered string efficiently
	var b strings.Builder

	// Pre-size the builder to reduce allocations
	estimatedSize := len(rendered)
	for _, f := range fragments {
		estimatedSize += len(f.view) + f.gap
	}
	b.Grow(estimatedSize)

	if l.direction == DirectionForward {
		b.WriteString(rendered)
		for i := range fragments {
			f := &fragments[i]
			b.WriteString(f.view)
			// Optimized gap writing using pre-allocated buffer
			if f.gap > 0 {
				if f.gap <= maxGapSize {
					b.WriteString(newlineBuffer[:f.gap])
				} else {
					b.WriteString(strings.Repeat("\n", f.gap))
				}
			}
		}

		return b.String(), finalIndex
	}

	// iterate backwards as fragments are in reversed order
	for i := len(fragments) - 1; i >= 0; i-- {
		f := &fragments[i]
		b.WriteString(f.view)
		// Optimized gap writing using pre-allocated buffer
		if f.gap > 0 {
			if f.gap <= maxGapSize {
				b.WriteString(newlineBuffer[:f.gap])
			} else {
				b.WriteString(strings.Repeat("\n", f.gap))
			}
		}
	}
	b.WriteString(rendered)

	return b.String(), finalIndex
}

func (l *list[T]) renderItem(item Item) renderedItem {
	view := item.View()
	return renderedItem{
		view:   view,
		height: lipgloss.Height(view),
	}
}

// AppendItem implements List.
func (l *list[T]) AppendItem(item T) tea.Cmd {
	// Pre-allocate with expected capacity
	cmds := make([]tea.Cmd, 0, 4)
	cmd := item.Init()
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	newIndex := l.items.Len()
	l.items.Append(item)
	l.indexMap.Set(item.ID(), newIndex)

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
				l.offset = min(l.renderedHeight-1, l.offset+newLines)
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
	l.indexMap.Del(id)

	// Only update indices for items after the deleted one
	itemsLen := l.items.Len()
	for i := inx; i < itemsLen; i++ {
		if item, ok := l.items.Get(i); ok {
			l.indexMap.Set(item.ID(), i)
		}
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
		if l.renderedHeight <= l.height {
			l.offset = 0
		} else {
			maxOffset := l.renderedHeight - l.height
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
	itemsLen := l.items.Len()
	result := make([]T, 0, itemsLen)
	for i := range itemsLen {
		if item, ok := l.items.Get(i); ok {
			result = append(result, item)
		}
	}
	return result
}

func (l *list[T]) incrementOffset(n int) {
	// no need for offset
	if l.renderedHeight <= l.height {
		return
	}
	maxOffset := l.renderedHeight - l.height
	n = min(n, maxOffset-l.offset)
	if n <= 0 {
		return
	}
	l.offset += n
	l.cachedViewDirty = true
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
	l.cachedViewDirty = true
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
		// no change in offset, so no need to change selection
		return nil
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
		// no change in offset, so no need to change selection
		return nil
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
	// Pre-allocate with expected capacity
	cmds := make([]tea.Cmd, 0, 4)
	cmds = append(cmds, item.Init())

	l.items.Prepend(item)

	// Rebuild index map (prepend requires updating all indices)
	// But do it more efficiently by using direct iteration
	l.indexMap = csync.NewMap[string, int]()
	itemsLen := l.items.Len()
	for i := range itemsLen {
		if it, ok := l.items.Get(i); ok {
			l.indexMap.Set(it.ID(), i)
		}
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
				l.offset = min(l.renderedHeight-1, l.offset+newLines)
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
	// Pre-allocate with expected capacity
	cmds := make([]tea.Cmd, 0, 2)
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
	l.prevSelectedItem = l.selectedItem
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
	l.prevSelectedItem = l.selectedItem
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
	for inx, item := range items {
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
	l.prevSelectedItem = l.selectedItem
	l.selectedItem = id
	return l.render()
}

func (l *list[T]) reset(selectedItem string) tea.Cmd {
	var cmds []tea.Cmd
	l.rendered = ""
	l.renderedHeight = 0
	l.offset = 0
	l.selectedItem = selectedItem
	l.indexMap = csync.NewMap[string, int]()
	l.renderedItems = csync.NewMap[string, renderedItem]()
	itemsLen := l.items.Len()
	for i := range itemsLen {
		item, ok := l.items.Get(i)
		if !ok {
			continue
		}
		l.indexMap.Set(item.ID(), i)
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
	// Pre-allocate with expected capacity
	cmds := make([]tea.Cmd, 0, 1)
	if inx, ok := l.indexMap.Get(id); ok {
		l.items.Set(inx, item)
		oldItem, hasOldItem := l.renderedItems.Get(id)
		oldPosition := l.offset
		if l.direction == DirectionBackward {
			oldPosition = (l.renderedHeight - 1) - l.offset
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
					l.offset = ordered.Clamp(l.offset+newLines, 0, l.renderedHeight-1)
				}
			}
		} else if hasOldItem && l.offset > oldItem.start {
			newItem, ok := l.renderedItems.Get(item.ID())
			if ok {
				newLines := newItem.height - oldItem.height
				l.offset = ordered.Clamp(l.offset+newLines, 0, l.renderedHeight-1)
			}
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
	return startCol, endCol
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
