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
	view := l.rendered
	lines := strings.Split(view, "\n")

	start, end := l.viewPosition()
	viewStart := max(0, start)
	viewEnd := min(len(lines), end+1)

	if viewStart > viewEnd {
		viewStart = viewEnd
	}
	lines = lines[viewStart:viewEnd]

	if l.resize {
		return strings.Join(lines, "\n")
	}
	view = t.S().Base.
		Height(l.height).
		Width(l.width).
		Render(strings.Join(lines, "\n"))

	if !l.hasSelection() {
		return view
	}

	return l.selectionView(view, false)
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
	start = min(start, end)
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

// renderFragment holds updated rendered view fragments
type renderFragment struct {
	view string
	gap  int
}

// renderIterator renders items starting from the specific index and limits height if limitHeight != -1
// returns the last index and the rendered content so far
// we pass the rendered content around and don't use l.rendered to prevent jumping of the content
func (l *list[T]) renderIterator(startInx int, limitHeight bool, rendered string) (string, int) {
	var fragments []renderFragment

	currentContentHeight := lipgloss.Height(rendered) - 1
	itemsLen := l.items.Len()
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
	if l.direction == DirectionForward {
		b.WriteString(rendered)
		for _, f := range fragments {
			b.WriteString(f.view)
			for range f.gap {
				b.WriteByte('\n')
			}
		}

		return b.String(), finalIndex
	}

	// iterate backwards as fragments are in reversed order
	for i := len(fragments) - 1; i >= 0; i-- {
		f := fragments[i]
		b.WriteString(f.view)
		for range f.gap {
			b.WriteByte('\n')
		}
	}
	b.WriteString(rendered)

	return b.String(), finalIndex
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
