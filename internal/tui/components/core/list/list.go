package list

import (
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/v2/help"
	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/spinner"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/opencode-ai/opencode/internal/tui/components/anim"
	"github.com/opencode-ai/opencode/internal/tui/layout"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

// Constants for special index values and defaults
const (
	NoSelection    = -1 // Indicates no item is currently selected
	NotRendered    = -1 // Indicates an item hasn't been rendered yet
	NoFinalHeight  = -1 // Indicates final height hasn't been calculated
	DefaultGapSize = 0  // Default spacing between list items
)

// ListModel defines the interface for a scrollable, selectable list component.
// It combines the basic Model interface with sizing capabilities and list-specific operations.
type ListModel interface {
	util.Model
	layout.Sizeable
	SetItems([]util.Model) tea.Cmd  // Replace all items in the list
	AppendItem(util.Model) tea.Cmd  // Add an item to the end of the list
	PrependItem(util.Model) tea.Cmd // Add an item to the beginning of the list
	DeleteItem(int)                 // Remove an item at the specified index
	UpdateItem(int, util.Model)     // Replace an item at the specified index
	ResetView()                     // Clear rendering cache and reset scroll position
	Items() []util.Model            // Get all items in the list
}

// HasAnim interface identifies items that support animation.
// Items implementing this interface will receive animation update messages.
type HasAnim interface {
	util.Model
	Spinning() bool // Returns true if the item is currently animating
}

// renderedItem represents a cached rendered item with its position and content.
type renderedItem struct {
	lines  []string // The rendered lines of text for this item
	start  int      // Starting line position in the overall rendered content
	height int      // Number of lines this item occupies
}

// renderState manages the rendering cache and state for the list.
// It tracks which items have been rendered and their positions.
type renderState struct {
	items         map[int]renderedItem // Cache of rendered items by index
	lines         []string             // All rendered lines concatenated
	lastIndex     int                  // Index of the last rendered item
	finalHeight   int                  // Total height when all items are rendered
	needsRerender bool                 // Flag indicating if re-rendering is needed
}

// newRenderState creates a new render state with default values.
func newRenderState() *renderState {
	return &renderState{
		items:         make(map[int]renderedItem),
		lines:         []string{},
		lastIndex:     NotRendered,
		finalHeight:   NoFinalHeight,
		needsRerender: true,
	}
}

// reset clears all cached rendering data and resets state to initial values.
func (rs *renderState) reset() {
	rs.items = make(map[int]renderedItem)
	rs.lines = []string{}
	rs.lastIndex = NotRendered
	rs.finalHeight = NoFinalHeight
	rs.needsRerender = true
}

// viewState manages the visual display properties of the list.
type viewState struct {
	width, height int    // Dimensions of the list viewport
	offset        int    // Current scroll offset in lines
	reverse       bool   // Whether to render in reverse order (bottom-up)
	content       string // The final rendered content to display
}

// selectionState manages which item is currently selected.
type selectionState struct {
	selectedIndex int // Index of the currently selected item, or NoSelection
}

// isValidIndex checks if the selected index is within the valid range of items.
func (ss *selectionState) isValidIndex(itemCount int) bool {
	return ss.selectedIndex >= 0 && ss.selectedIndex < itemCount
}

// model is the main implementation of the ListModel interface.
// It coordinates between view state, render state, and selection state.
type model struct {
	viewState      viewState      // Display and scrolling state
	renderState    *renderState   // Rendering cache and state
	selectionState selectionState // Item selection state
	help           help.Model     // Help system for keyboard shortcuts
	keymap         KeyMap         // Key bindings for navigation
	items          []util.Model   // The actual list items
	gapSize        int            // Number of empty lines between items
	padding        []int          // Padding around the list content
}

// listOptions is a function type for configuring list options.
type listOptions func(*model)

// WithKeyMap sets custom key bindings for the list.
func WithKeyMap(k KeyMap) listOptions {
	return func(m *model) {
		m.keymap = k
	}
}

// WithReverse sets whether the list should render in reverse order (newest items at bottom).
func WithReverse(reverse bool) listOptions {
	return func(m *model) {
		m.setReverse(reverse)
	}
}

// WithGapSize sets the number of empty lines to insert between list items.
func WithGapSize(gapSize int) listOptions {
	return func(m *model) {
		m.gapSize = gapSize
	}
}

// WithPadding sets the padding around the list content.
// Follows CSS padding convention: 1 value = all sides, 2 values = vertical/horizontal,
// 4 values = top/right/bottom/left.
func WithPadding(padding ...int) listOptions {
	return func(m *model) {
		m.padding = padding
	}
}

// WithItems sets the initial items for the list.
func WithItems(items []util.Model) listOptions {
	return func(m *model) {
		m.items = items
	}
}

// New creates a new list model with the specified options.
// The list starts with no items selected and requires SetItems to be called
// or items to be provided via WithItems option.
func New(opts ...listOptions) ListModel {
	m := &model{
		help:           help.New(),
		keymap:         DefaultKeymap(),
		items:          []util.Model{},
		renderState:    newRenderState(),
		gapSize:        DefaultGapSize,
		padding:        []int{},
		selectionState: selectionState{selectedIndex: NoSelection},
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Init initializes the list component and sets up the initial items.
// This is called automatically by the Bubble Tea framework.
func (m *model) Init() tea.Cmd {
	return m.SetItems(m.items)
}

// Update handles incoming messages and updates the list state accordingly.
// It processes keyboard input, animation messages, and forwards other messages
// to the currently selected item.
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.handleKeyPress(msg)
	case anim.ColorCycleMsg, anim.StepCharsMsg, spinner.TickMsg:
		return m.handleAnimationMsg(msg)
	}

	if m.selectionState.isValidIndex(len(m.items)) {
		return m.updateSelectedItem(msg)
	}

	return m, nil
}

// handleKeyPress processes keyboard input for list navigation.
// Supports scrolling, item selection, and navigation to top/bottom.
func (m *model) handleKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keymap.Down) || key.Matches(msg, m.keymap.NDown):
		m.scrollDown(1)
	case key.Matches(msg, m.keymap.Up) || key.Matches(msg, m.keymap.NUp):
		m.scrollUp(1)
	case key.Matches(msg, m.keymap.DownOneItem):
		return m, m.selectNextItem()
	case key.Matches(msg, m.keymap.UpOneItem):
		return m, m.selectPreviousItem()
	case key.Matches(msg, m.keymap.HalfPageDown):
		m.scrollDown(m.listHeight() / 2)
	case key.Matches(msg, m.keymap.HalfPageUp):
		m.scrollUp(m.listHeight() / 2)
	case key.Matches(msg, m.keymap.Home):
		return m, m.goToTop()
	case key.Matches(msg, m.keymap.End):
		return m, m.goToBottom()
	}
	return m, nil
}

// handleAnimationMsg forwards animation messages to items that support animation.
// Only items implementing HasAnim and currently spinning receive these messages.
func (m *model) handleAnimationMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	for inx, item := range m.items {
		if i, ok := item.(HasAnim); ok && i.Spinning() {
			updated, cmd := i.Update(msg)
			cmds = append(cmds, cmd)
			if u, ok := updated.(util.Model); ok {
				m.UpdateItem(inx, u)
			}
		}
	}
	return m, tea.Batch(cmds...)
}

// updateSelectedItem forwards messages to the currently selected item.
// This allows the selected item to handle its own input and state changes.
func (m *model) updateSelectedItem(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	u, cmd := m.items[m.selectionState.selectedIndex].Update(msg)
	cmds = append(cmds, cmd)
	if updated, ok := u.(util.Model); ok {
		m.UpdateItem(m.selectionState.selectedIndex, updated)
	}
	return m, tea.Batch(cmds...)
}

// scrollDown scrolls the list down by the specified amount.
// Direction is automatically adjusted based on reverse mode.
func (m *model) scrollDown(amount int) {
	if m.viewState.reverse {
		m.decreaseOffset(amount)
	} else {
		m.increaseOffset(amount)
	}
}

// scrollUp scrolls the list up by the specified amount.
// Direction is automatically adjusted based on reverse mode.
func (m *model) scrollUp(amount int) {
	if m.viewState.reverse {
		m.increaseOffset(amount)
	} else {
		m.decreaseOffset(amount)
	}
}

// View renders the list to a string for display.
// Returns empty string if the list has no dimensions.
// Triggers re-rendering if needed before returning content.
func (m *model) View() tea.View {
	if m.viewState.height == 0 || m.viewState.width == 0 {
		return tea.NewView("") // No content to display
	}
	if m.renderState.needsRerender {
		m.renderVisible()
	}
	return tea.NewView(
		lipgloss.NewStyle().
			Padding(m.padding...).
			Height(m.viewState.height).
			Render(m.viewState.content),
	)
}

// Items returns a copy of all items in the list.
func (m *model) Items() []util.Model {
	return m.items
}

// renderVisible determines which rendering strategy to use and triggers rendering.
// Uses forward rendering for normal mode and reverse rendering for reverse mode.
func (m *model) renderVisible() {
	if m.viewState.reverse {
		m.renderVisibleReverse()
	} else {
		m.renderVisibleForward()
	}
}

// renderVisibleForward renders items from top to bottom (normal mode).
// Only renders items that are currently visible or near the viewport.
func (m *model) renderVisibleForward() {
	renderer := &forwardRenderer{
		model:   m,
		start:   0,
		cutoff:  m.viewState.offset + m.listHeight(),
		items:   m.items,
		realIdx: m.renderState.lastIndex,
	}

	if m.renderState.lastIndex > NotRendered {
		renderer.items = m.items[m.renderState.lastIndex+1:]
		renderer.start = len(m.renderState.lines)
	}

	renderer.render()
	m.finalizeRender()
}

// renderVisibleReverse renders items from bottom to top (reverse mode).
// Used when new items should appear at the bottom (like chat messages).
func (m *model) renderVisibleReverse() {
	renderer := &reverseRenderer{
		model:   m,
		start:   0,
		cutoff:  m.viewState.offset + m.listHeight(),
		items:   m.items,
		realIdx: m.renderState.lastIndex,
	}

	if m.renderState.lastIndex > NotRendered {
		renderer.items = m.items[:m.renderState.lastIndex]
		renderer.start = len(m.renderState.lines)
	} else {
		m.renderState.lastIndex = len(m.items)
		renderer.realIdx = len(m.items)
	}

	renderer.render()
	m.finalizeRender()
}

// finalizeRender completes the rendering process by updating scroll bounds and content.
func (m *model) finalizeRender() {
	m.renderState.needsRerender = false
	if m.renderState.finalHeight > NoFinalHeight {
		m.viewState.offset = min(m.viewState.offset, m.renderState.finalHeight)
	}
	m.updateContent()
}

// updateContent extracts the visible portion of rendered content for display.
// Handles both normal and reverse rendering modes.
func (m *model) updateContent() {
	maxHeight := min(m.listHeight(), len(m.renderState.lines))
	if m.viewState.offset >= len(m.renderState.lines) {
		m.viewState.content = ""
		return
	}

	if m.viewState.reverse {
		end := len(m.renderState.lines) - m.viewState.offset
		start := max(0, end-maxHeight)
		m.viewState.content = strings.Join(m.renderState.lines[start:end], "\n")
	} else {
		endIdx := min(maxHeight+m.viewState.offset, len(m.renderState.lines))
		m.viewState.content = strings.Join(m.renderState.lines[m.viewState.offset:endIdx], "\n")
	}
}

// forwardRenderer handles rendering items from top to bottom.
// It builds up the rendered content incrementally, caching results for performance.
type forwardRenderer struct {
	model   *model       // Reference to the parent list model
	start   int          // Current line position in the overall content
	cutoff  int          // Line position where we can stop rendering
	items   []util.Model // Items to render (may be a subset)
	realIdx int          // Real index in the full item list
}

// render processes items in forward order, building up the rendered content.
func (r *forwardRenderer) render() {
	for _, item := range r.items {
		r.realIdx++
		if r.start > r.cutoff {
			break
		}

		itemLines := r.getOrRenderItem(item)
		if r.realIdx == len(r.model.items)-1 {
			r.model.renderState.finalHeight = max(0, r.start+len(itemLines)-r.model.listHeight())
		}

		r.model.renderState.lines = append(r.model.renderState.lines, itemLines...)
		r.model.renderState.lastIndex = r.realIdx
		r.start += len(itemLines)
	}
}

// getOrRenderItem retrieves cached content or renders the item if not cached.
func (r *forwardRenderer) getOrRenderItem(item util.Model) []string {
	if cachedContent, ok := r.model.renderState.items[r.realIdx]; ok {
		return cachedContent.lines
	}

	itemLines := r.renderItemLines(item)
	r.model.renderState.items[r.realIdx] = renderedItem{
		lines:  itemLines,
		start:  r.start,
		height: len(itemLines),
	}
	return itemLines
}

// renderItemLines converts an item to its string representation with gaps.
func (r *forwardRenderer) renderItemLines(item util.Model) []string {
	return r.model.getItemLines(item)
}

// reverseRenderer handles rendering items from bottom to top.
// Used in reverse mode where new items appear at the bottom.
type reverseRenderer struct {
	model   *model       // Reference to the parent list model
	start   int          // Current line position in the overall content
	cutoff  int          // Line position where we can stop rendering
	items   []util.Model // Items to render (may be a subset)
	realIdx int          // Real index in the full item list
}

// render processes items in reverse order, prepending to the rendered content.
func (r *reverseRenderer) render() {
	for i := len(r.items) - 1; i >= 0; i-- {
		r.realIdx--
		if r.start > r.cutoff {
			break
		}

		itemLines := r.getOrRenderItem(r.items[i])
		if r.realIdx == 0 {
			r.model.renderState.finalHeight = max(0, r.start+len(itemLines)-r.model.listHeight())
		}

		r.model.renderState.lines = append(itemLines, r.model.renderState.lines...)
		r.model.renderState.lastIndex = r.realIdx
		r.start += len(itemLines)
	}
}

// getOrRenderItem retrieves cached content or renders the item if not cached.
func (r *reverseRenderer) getOrRenderItem(item util.Model) []string {
	if cachedContent, ok := r.model.renderState.items[r.realIdx]; ok {
		return cachedContent.lines
	}

	itemLines := r.renderItemLines(item)
	r.model.renderState.items[r.realIdx] = renderedItem{
		lines:  itemLines,
		start:  r.start,
		height: len(itemLines),
	}
	return itemLines
}

// renderItemLines converts an item to its string representation with gaps.
func (r *reverseRenderer) renderItemLines(item util.Model) []string {
	return r.model.getItemLines(item)
}

// selectPreviousItem moves selection to the previous item in the list.
// Handles focus management and ensures the selected item remains visible.
func (m *model) selectPreviousItem() tea.Cmd {
	if m.selectionState.selectedIndex <= 0 {
		return nil
	}

	cmds := []tea.Cmd{m.blurSelected()}
	m.selectionState.selectedIndex--
	cmds = append(cmds, m.focusSelected())
	m.ensureSelectedItemVisible()
	return tea.Batch(cmds...)
}

// selectNextItem moves selection to the next item in the list.
// Handles focus management and ensures the selected item remains visible.
func (m *model) selectNextItem() tea.Cmd {
	if m.selectionState.selectedIndex >= len(m.items)-1 || m.selectionState.selectedIndex < 0 {
		return nil
	}

	cmds := []tea.Cmd{m.blurSelected()}
	m.selectionState.selectedIndex++
	cmds = append(cmds, m.focusSelected())
	m.ensureSelectedItemVisible()
	return tea.Batch(cmds...)
}

// ensureSelectedItemVisible scrolls the list to make the selected item visible.
// Uses different strategies for forward and reverse rendering modes.
func (m *model) ensureSelectedItemVisible() {
	cachedItem, ok := m.renderState.items[m.selectionState.selectedIndex]
	if !ok {
		m.renderState.needsRerender = true
		return
	}

	if m.viewState.reverse {
		m.ensureVisibleReverse(cachedItem)
	} else {
		m.ensureVisibleForward(cachedItem)
	}
	m.renderState.needsRerender = true
}

// ensureVisibleForward ensures the selected item is visible in forward rendering mode.
// Handles both large items (taller than viewport) and normal items.
func (m *model) ensureVisibleForward(cachedItem renderedItem) {
	if cachedItem.height >= m.listHeight() {
		if m.selectionState.selectedIndex > 0 {
			changeNeeded := m.viewState.offset - cachedItem.start
			m.decreaseOffset(changeNeeded)
		} else {
			changeNeeded := cachedItem.start - m.viewState.offset
			m.increaseOffset(changeNeeded)
		}
		return
	}

	if cachedItem.start < m.viewState.offset {
		changeNeeded := m.viewState.offset - cachedItem.start
		m.decreaseOffset(changeNeeded)
	} else {
		end := cachedItem.start + cachedItem.height
		if end > m.viewState.offset+m.listHeight() {
			changeNeeded := end - (m.viewState.offset + m.listHeight())
			m.increaseOffset(changeNeeded)
		}
	}
}

// ensureVisibleReverse ensures the selected item is visible in reverse rendering mode.
// Handles both large items (taller than viewport) and normal items.
func (m *model) ensureVisibleReverse(cachedItem renderedItem) {
	if cachedItem.height >= m.listHeight() {
		if m.selectionState.selectedIndex < len(m.items)-1 {
			changeNeeded := m.viewState.offset - (cachedItem.start + cachedItem.height - m.listHeight())
			m.decreaseOffset(changeNeeded)
		} else {
			changeNeeded := (cachedItem.start + cachedItem.height - m.listHeight()) - m.viewState.offset
			m.increaseOffset(changeNeeded)
		}
		return
	}

	if cachedItem.start+cachedItem.height > m.viewState.offset+m.listHeight() {
		changeNeeded := (cachedItem.start + cachedItem.height - m.listHeight()) - m.viewState.offset
		m.increaseOffset(changeNeeded)
	} else if cachedItem.start < m.viewState.offset {
		changeNeeded := m.viewState.offset - cachedItem.start
		m.decreaseOffset(changeNeeded)
	}
}

// goToBottom switches to reverse mode and selects the last item.
// Commonly used for chat-like interfaces where new content appears at the bottom.
func (m *model) goToBottom() tea.Cmd {
	cmds := []tea.Cmd{m.blurSelected()}
	m.viewState.reverse = true
	m.selectionState.selectedIndex = len(m.items) - 1
	cmds = append(cmds, m.focusSelected())
	m.ResetView()
	return tea.Batch(cmds...)
}

// goToTop switches to forward mode and selects the first item.
// Standard behavior for most list interfaces.
func (m *model) goToTop() tea.Cmd {
	cmds := []tea.Cmd{m.blurSelected()}
	m.viewState.reverse = false
	if len(m.items) > 0 {
		m.selectionState.selectedIndex = 0
	}
	cmds = append(cmds, m.focusSelected())
	m.ResetView()
	return tea.Batch(cmds...)
}

// ResetView clears all cached rendering data and resets scroll position.
// Forces a complete re-render on the next View() call.
func (m *model) ResetView() {
	m.renderState.reset()
	m.viewState.offset = 0
}

// focusSelected gives focus to the currently selected item if it supports focus.
// Triggers a re-render of the item to show its focused state.
func (m *model) focusSelected() tea.Cmd {
	if !m.selectionState.isValidIndex(len(m.items)) {
		return nil
	}
	if i, ok := m.items[m.selectionState.selectedIndex].(layout.Focusable); ok {
		cmd := i.Focus()
		m.rerenderItem(m.selectionState.selectedIndex)
		return cmd
	}
	return nil
}

// blurSelected removes focus from the currently selected item if it supports focus.
// Triggers a re-render of the item to show its unfocused state.
func (m *model) blurSelected() tea.Cmd {
	if !m.selectionState.isValidIndex(len(m.items)) {
		return nil
	}
	if i, ok := m.items[m.selectionState.selectedIndex].(layout.Focusable); ok {
		cmd := i.Blur()
		m.rerenderItem(m.selectionState.selectedIndex)
		return cmd
	}
	return nil
}

// rerenderItem updates the cached rendering of a specific item.
// This is called when an item's state changes (e.g., focus/blur) and needs to be re-displayed.
// It efficiently updates only the changed item and adjusts positions of subsequent items if needed.
func (m *model) rerenderItem(inx int) {
	if inx < 0 || inx >= len(m.items) || len(m.renderState.lines) == 0 {
		return
	}

	cachedItem, ok := m.renderState.items[inx]
	if !ok {
		return
	}

	rerenderedLines := m.getItemLines(m.items[inx])
	if slices.Equal(cachedItem.lines, rerenderedLines) {
		return
	}

	m.updateRenderedLines(cachedItem, rerenderedLines)
	m.updateItemPositions(inx, cachedItem, len(rerenderedLines))
	m.updateCachedItem(inx, cachedItem, rerenderedLines)
	m.renderState.needsRerender = true
}

// getItemLines converts an item to its rendered lines, including any gap spacing.
func (m *model) getItemLines(item util.Model) []string {
	itemLines := strings.Split(item.View().String(), "\n")
	if m.gapSize > 0 {
		gap := make([]string, m.gapSize)
		itemLines = append(itemLines, gap...)
	}
	return itemLines
}

// updateRenderedLines replaces the lines for a specific item in the overall rendered content.
func (m *model) updateRenderedLines(cachedItem renderedItem, newLines []string) {
	start, end := m.getItemBounds(cachedItem)
	totalLines := len(m.renderState.lines)

	if start >= 0 && start <= totalLines && end >= 0 && end <= totalLines {
		m.renderState.lines = slices.Delete(m.renderState.lines, start, end)
		m.renderState.lines = slices.Insert(m.renderState.lines, start, newLines...)
	}
}

// getItemBounds calculates the start and end line positions for an item.
// Handles both forward and reverse rendering modes.
func (m *model) getItemBounds(cachedItem renderedItem) (start, end int) {
	start = cachedItem.start
	end = start + cachedItem.height

	if m.viewState.reverse {
		totalLines := len(m.renderState.lines)
		end = totalLines - cachedItem.start
		start = end - cachedItem.height
	}
	return start, end
}

// updateItemPositions recalculates positions for items after the changed item.
// This is necessary when an item's height changes, affecting subsequent items.
func (m *model) updateItemPositions(inx int, cachedItem renderedItem, newHeight int) {
	if cachedItem.height == newHeight {
		return
	}

	if inx == len(m.items)-1 {
		m.renderState.finalHeight = max(0, cachedItem.start+newHeight-m.listHeight())
	}

	currentStart := cachedItem.start + newHeight
	if m.viewState.reverse {
		m.updatePositionsReverse(inx, currentStart)
	} else {
		m.updatePositionsForward(inx, currentStart)
	}
}

// updatePositionsForward updates positions for items after the changed item in forward mode.
func (m *model) updatePositionsForward(inx int, currentStart int) {
	for i := inx + 1; i < len(m.items); i++ {
		if existing, ok := m.renderState.items[i]; ok {
			existing.start = currentStart
			currentStart += existing.height
			m.renderState.items[i] = existing
		} else {
			break
		}
	}
}

// updatePositionsReverse updates positions for items before the changed item in reverse mode.
func (m *model) updatePositionsReverse(inx int, currentStart int) {
	for i := inx - 1; i >= 0; i-- {
		if existing, ok := m.renderState.items[i]; ok {
			existing.start = currentStart
			currentStart += existing.height
			m.renderState.items[i] = existing
		} else {
			break
		}
	}
}

// updateCachedItem updates the cached rendering information for a specific item.
func (m *model) updateCachedItem(inx int, cachedItem renderedItem, newLines []string) {
	m.renderState.items[inx] = renderedItem{
		lines:  newLines,
		start:  cachedItem.start,
		height: len(newLines),
	}
}

// increaseOffset scrolls the list down by increasing the offset.
// Respects the final height limit to prevent scrolling past the end.
func (m *model) increaseOffset(n int) {
	if m.renderState.finalHeight > NoFinalHeight {
		if m.viewState.offset < m.renderState.finalHeight {
			m.viewState.offset += n
			if m.viewState.offset > m.renderState.finalHeight {
				m.viewState.offset = m.renderState.finalHeight
			}
			m.renderState.needsRerender = true
		}
	} else {
		m.viewState.offset += n
		m.renderState.needsRerender = true
	}
}

// decreaseOffset scrolls the list up by decreasing the offset.
// Prevents scrolling above the beginning of the list.
func (m *model) decreaseOffset(n int) {
	if m.viewState.offset > 0 {
		m.viewState.offset -= n
		if m.viewState.offset < 0 {
			m.viewState.offset = 0
		}
		m.renderState.needsRerender = true
	}
}

// UpdateItem replaces an item at the specified index with a new item.
// Handles focus management and triggers re-rendering as needed.
func (m *model) UpdateItem(inx int, item util.Model) {
	if inx < 0 || inx >= len(m.items) {
		return
	}
	m.items[inx] = item
	if m.selectionState.selectedIndex == inx {
		if i, ok := m.items[m.selectionState.selectedIndex].(layout.Focusable); ok {
			i.Focus()
		}
	}
	m.setItemSize(inx)
	m.rerenderItem(inx)
	m.renderState.needsRerender = true
}

// GetSize returns the current dimensions of the list.
func (m *model) GetSize() (int, int) {
	return m.viewState.width, m.viewState.height
}

// SetSize updates the list dimensions and triggers a complete re-render.
// Also updates the size of all items that support sizing.
func (m *model) SetSize(width int, height int) tea.Cmd {
	if m.viewState.width == width && m.viewState.height == height {
		return nil
	}
	if m.viewState.height != height {
		m.renderState.finalHeight = NoFinalHeight
		m.viewState.height = height
	}
	m.viewState.width = width
	m.ResetView()
	return m.setAllItemsSize()
}

// getItemSize calculates the available width for items, accounting for padding.
func (m *model) getItemSize() int {
	width := m.viewState.width
	switch len(m.padding) {
	case 1:
		width -= m.padding[0] * 2
	case 2, 3:
		width -= m.padding[1] * 2
	case 4:
		width -= m.padding[1] + m.padding[3]
	}
	return max(0, width)
}

// setItemSize updates the size of a specific item if it supports sizing.
func (m *model) setItemSize(inx int) tea.Cmd {
	if inx < 0 || inx >= len(m.items) {
		return nil
	}
	if i, ok := m.items[inx].(layout.Sizeable); ok {
		return i.SetSize(m.getItemSize(), 0)
	}
	return nil
}

// setAllItemsSize updates the size of all items that support sizing.
func (m *model) setAllItemsSize() tea.Cmd {
	var cmds []tea.Cmd
	for i := range m.items {
		if cmd := m.setItemSize(i); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

// listHeight calculates the available height for list content, accounting for padding.
func (m *model) listHeight() int {
	height := m.viewState.height
	switch len(m.padding) {
	case 1:
		height -= m.padding[0] * 2
	case 2:
		height -= m.padding[0] * 2
	case 3, 4:
		height -= m.padding[0] + m.padding[2]
	}
	return max(0, height)
}

// AppendItem adds a new item to the end of the list.
// Automatically switches to reverse mode and scrolls to show the new item.
func (m *model) AppendItem(item util.Model) tea.Cmd {
	cmds := []tea.Cmd{
		item.Init(),
	}
	m.items = append(m.items, item)
	cmds = append(cmds, m.setItemSize(len(m.items)-1))
	cmds = append(cmds, m.goToBottom())
	m.renderState.needsRerender = true
	return tea.Batch(cmds...)
}

// DeleteItem removes an item at the specified index.
// Adjusts selection if necessary and triggers a complete re-render.
func (m *model) DeleteItem(i int) {
	if i < 0 || i >= len(m.items) {
		return
	}
	m.items = slices.Delete(m.items, i, i+1)
	delete(m.renderState.items, i)

	if m.selectionState.selectedIndex == i && m.selectionState.selectedIndex > 0 {
		m.selectionState.selectedIndex--
	} else if m.selectionState.selectedIndex > i {
		m.selectionState.selectedIndex--
	}

	m.ResetView()
	m.renderState.needsRerender = true
}

// PrependItem adds a new item to the beginning of the list.
// Adjusts cached positions and selection index, then switches to forward mode.
func (m *model) PrependItem(item util.Model) tea.Cmd {
	cmds := []tea.Cmd{item.Init()}
	m.items = append([]util.Model{item}, m.items...)

	// Shift all cached item indices by 1
	newItems := make(map[int]renderedItem, len(m.renderState.items))
	for k, v := range m.renderState.items {
		newItems[k+1] = v
	}
	m.renderState.items = newItems

	if m.selectionState.selectedIndex >= 0 {
		m.selectionState.selectedIndex++
	}

	cmds = append(cmds, m.goToTop())
	cmds = append(cmds, m.setItemSize(0))
	m.renderState.needsRerender = true
	return tea.Batch(cmds...)
}

// setReverse switches between forward and reverse rendering modes.
func (m *model) setReverse(reverse bool) {
	if reverse {
		m.goToBottom()
	} else {
		m.goToTop()
	}
}

// SetItems replaces all items in the list with a new set.
// Initializes all items, sets their sizes, and establishes initial selection.
func (m *model) SetItems(items []util.Model) tea.Cmd {
	m.items = items
	cmds := []tea.Cmd{m.setAllItemsSize()}

	for _, item := range m.items {
		cmds = append(cmds, item.Init())
	}

	if len(m.items) > 0 {
		if m.viewState.reverse {
			m.selectionState.selectedIndex = len(m.items) - 1
		} else {
			m.selectionState.selectedIndex = 0
		}
		if cmd := m.focusSelected(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	} else {
		m.selectionState.selectedIndex = NoSelection
	}

	m.ResetView()
	return tea.Batch(cmds...)
}
