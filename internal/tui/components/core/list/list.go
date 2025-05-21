package list

import (
	"slices"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/v2/help"
	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/opencode-ai/opencode/internal/logging"
	"github.com/opencode-ai/opencode/internal/tui/components/anim"
	"github.com/opencode-ai/opencode/internal/tui/layout"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

type ListModel interface {
	util.Model
	layout.Sizeable
	SetItems([]util.Model) tea.Cmd
	AppendItem(util.Model) tea.Cmd
	PrependItem(util.Model) tea.Cmd
	DeleteItem(int)
	UpdateItem(int, util.Model)
	ResetView()
	Items() []util.Model
}

type HasAnim interface {
	util.Model
	Spinning() bool
}

type renderedItem struct {
	lines  []string
	start  int
	height int
}
type model struct {
	width, height, offset int
	finalHight            int // this gets set when the last item is rendered to mark the max offset
	reverse               bool
	help                  help.Model
	keymap                KeyMap
	items                 []util.Model
	renderedItems         *sync.Map // item index to rendered string
	needsRerender         bool
	renderedLines         []string
	selectedItemInx       int
	lastRenderedInx       int
	content               string
	gapSize               int
	padding               []int
}

type listOptions func(*model)

func WithKeyMap(k KeyMap) listOptions {
	return func(m *model) {
		m.keymap = k
	}
}

func WithReverse(reverse bool) listOptions {
	return func(m *model) {
		m.setReverse(reverse)
	}
}

func WithGapSize(gapSize int) listOptions {
	return func(m *model) {
		m.gapSize = gapSize
	}
}

func WithPadding(padding ...int) listOptions {
	return func(m *model) {
		m.padding = padding
	}
}

func WithItems(items []util.Model) listOptions {
	return func(m *model) {
		m.items = items
	}
}

func New(opts ...listOptions) ListModel {
	m := &model{
		help:            help.New(),
		keymap:          defaultKeymap(),
		items:           []util.Model{},
		needsRerender:   true,
		gapSize:         0,
		padding:         []int{},
		selectedItemInx: -1,
		finalHight:      -1,
		lastRenderedInx: -1,
		renderedItems:   new(sync.Map),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Init implements List.
func (m *model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		m.SetItems(m.items),
	}
	return tea.Batch(cmds...)
}

// Update implements List.
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keymap.Down) || key.Matches(msg, m.keymap.NDown):
			if m.reverse {
				m.decreaseOffset(1)
			} else {
				m.increaseOffset(1)
			}
			return m, nil
		case key.Matches(msg, m.keymap.Up) || key.Matches(msg, m.keymap.NUp):
			if m.reverse {
				m.increaseOffset(1)
			} else {
				m.decreaseOffset(1)
			}
			return m, nil
		case key.Matches(msg, m.keymap.DownOneItem):
			m.downOneItem()
			return m, nil
		case key.Matches(msg, m.keymap.UpOneItem):
			m.upOneItem()
			return m, nil
		case key.Matches(msg, m.keymap.HalfPageDown):
			if m.reverse {
				m.decreaseOffset(m.listHeight() / 2)
			} else {
				m.increaseOffset(m.listHeight() / 2)
			}
			return m, nil
		case key.Matches(msg, m.keymap.HalfPageUp):
			if m.reverse {
				m.increaseOffset(m.listHeight() / 2)
			} else {
				m.decreaseOffset(m.listHeight() / 2)
			}
			return m, nil
		case key.Matches(msg, m.keymap.Home):
			m.goToTop()
			return m, nil
		case key.Matches(msg, m.keymap.End):
			m.goToBottom()
			return m, nil
		}
	case anim.ColorCycleMsg:
		logging.Info("ColorCycleMsg", "msg", msg)
		for inx, item := range m.items {
			if i, ok := item.(HasAnim); ok {
				if i.Spinning() {
					updated, cmd := i.Update(msg)
					cmds = append(cmds, cmd)
					m.UpdateItem(inx, updated.(util.Model))
				}
			}
		}
		return m, tea.Batch(cmds...)
	case anim.StepCharsMsg:
		logging.Info("ColorCycleMsg", "msg", msg)
		for inx, item := range m.items {
			if i, ok := item.(HasAnim); ok {
				if i.Spinning() {
					updated, cmd := i.Update(msg)
					cmds = append(cmds, cmd)
					m.UpdateItem(inx, updated.(util.Model))
				}
			}
		}
		return m, tea.Batch(cmds...)
	}
	if m.selectedItemInx > -1 {
		u, cmd := m.items[m.selectedItemInx].Update(msg)
		cmds = append(cmds, cmd)
		m.UpdateItem(m.selectedItemInx, u.(util.Model))
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)
	}

	return m, nil
}

// View implements List.
func (m *model) View() string {
	if m.height == 0 || m.width == 0 {
		return ""
	}
	if m.needsRerender {
		m.renderVisible()
	}
	return lipgloss.NewStyle().Padding(m.padding...).Height(m.height).Render(m.content)
}

// Items implements ListModel.
func (m *model) Items() []util.Model {
	return m.items
}

func (m *model) renderVisibleReverse() {
	start := 0
	cutoff := m.offset + m.listHeight()
	items := m.items
	if m.lastRenderedInx > -1 {
		items = m.items[:m.lastRenderedInx]
		start = len(m.renderedLines)
	} else {
		// reveresed so that it starts at the end
		m.lastRenderedInx = len(m.items)
	}
	realIndex := m.lastRenderedInx
	for i := len(items) - 1; i >= 0; i-- {
		realIndex--
		var itemLines []string
		cachedContent, ok := m.renderedItems.Load(realIndex)
		if ok {
			itemLines = cachedContent.(renderedItem).lines
		} else {
			itemLines = strings.Split(items[i].View(), "\n")
			if m.gapSize > 0 && realIndex != len(m.items)-1 {
				for range m.gapSize {
					itemLines = append(itemLines, "")
				}
			}
			m.renderedItems.Store(realIndex, renderedItem{
				lines:  itemLines,
				start:  start,
				height: len(itemLines),
			})
		}

		if realIndex == 0 {
			m.finalHight = max(0, start+len(itemLines)-m.listHeight())
		}
		m.renderedLines = append(itemLines, m.renderedLines...)
		m.lastRenderedInx = realIndex
		// always render the next item
		if start > cutoff {
			break
		}
		start += len(itemLines)
	}
	m.needsRerender = false
	if m.finalHight > -1 {
		// make sure we don't go over the final height, this can happen if we did not render the last item and we overshot the offset
		m.offset = min(m.offset, m.finalHight)
	}
	maxHeight := min(m.listHeight(), len(m.renderedLines))
	if m.offset < len(m.renderedLines) {
		end := len(m.renderedLines) - m.offset
		start := max(0, end-maxHeight)
		m.content = strings.Join(m.renderedLines[start:end], "\n")
	} else {
		m.content = ""
	}
}

func (m *model) renderVisible() {
	if m.reverse {
		m.renderVisibleReverse()
		return
	}
	start := 0
	cutoff := m.offset + m.listHeight()
	items := m.items
	if m.lastRenderedInx > -1 {
		items = m.items[m.lastRenderedInx+1:]
		start = len(m.renderedLines)
	}

	realIndex := m.lastRenderedInx
	for _, item := range items {
		realIndex++

		var itemLines []string
		cachedContent, ok := m.renderedItems.Load(realIndex)
		if ok {
			itemLines = cachedContent.(renderedItem).lines
		} else {
			itemLines = strings.Split(item.View(), "\n")
			if m.gapSize > 0 && realIndex != len(m.items)-1 {
				for range m.gapSize {
					itemLines = append(itemLines, "")
				}
			}
			m.renderedItems.Store(realIndex, renderedItem{
				lines:  itemLines,
				start:  start,
				height: len(itemLines),
			})
		}
		// always render the next item
		if start > cutoff {
			break
		}

		if realIndex == len(m.items)-1 {
			m.finalHight = max(0, start+len(itemLines)-m.listHeight())
		}

		m.renderedLines = append(m.renderedLines, itemLines...)
		m.lastRenderedInx = realIndex
		start += len(itemLines)
	}
	m.needsRerender = false
	maxHeight := min(m.listHeight(), len(m.renderedLines))
	if m.finalHight > -1 {
		// make sure we don't go over the final height, this can happen if we did not render the last item and we overshot the offset
		m.offset = min(m.offset, m.finalHight)
	}
	if m.offset < len(m.renderedLines) {
		m.content = strings.Join(m.renderedLines[m.offset:maxHeight+m.offset], "\n")
	} else {
		m.content = ""
	}
}

func (m *model) upOneItem() tea.Cmd {
	var cmds []tea.Cmd
	if m.selectedItemInx > 0 {
		cmd := m.blurSelected()
		cmds = append(cmds, cmd)
		m.selectedItemInx--
		cmd = m.focusSelected()
		cmds = append(cmds, cmd)
	}

	cached, ok := m.renderedItems.Load(m.selectedItemInx)
	if ok {
		// already rendered
		if !m.reverse {
			cachedItem, _ := cached.(renderedItem)
			// might not fit on the screen move the offset to the start of the item
			if cachedItem.height >= m.listHeight() {
				changeNeeded := m.offset - cachedItem.start
				m.decreaseOffset(changeNeeded)
			}
			if cachedItem.start < m.offset {
				changeNeeded := m.offset - cachedItem.start
				m.decreaseOffset(changeNeeded)
			}
		} else {
			cachedItem, _ := cached.(renderedItem)
			// might not fit on the screen move the offset to the start of the item
			if cachedItem.height >= m.listHeight() || cachedItem.start+cachedItem.height > m.offset+m.listHeight() {
				changeNeeded := (cachedItem.start + cachedItem.height - m.listHeight()) - m.offset
				m.increaseOffset(changeNeeded)
			}
		}
	}
	m.needsRerender = true
	return tea.Batch(cmds...)
}

func (m *model) downOneItem() tea.Cmd {
	var cmds []tea.Cmd
	if m.selectedItemInx < len(m.items)-1 {
		cmd := m.blurSelected()
		cmds = append(cmds, cmd)
		m.selectedItemInx++
		cmd = m.focusSelected()
		cmds = append(cmds, cmd)
	}
	cached, ok := m.renderedItems.Load(m.selectedItemInx)
	if ok {
		// already rendered
		if !m.reverse {
			cachedItem, _ := cached.(renderedItem)
			// might not fit on the screen move the offset to the start of the item
			if cachedItem.height >= m.listHeight() {
				changeNeeded := cachedItem.start - m.offset
				m.increaseOffset(changeNeeded)
			} else {
				end := cachedItem.start + cachedItem.height
				if end > m.offset+m.listHeight() {
					changeNeeded := end - (m.offset + m.listHeight())
					m.increaseOffset(changeNeeded)
				}
			}
		} else {
			cachedItem, _ := cached.(renderedItem)
			// might not fit on the screen move the offset to the start of the item
			if cachedItem.height >= m.listHeight() {
				changeNeeded := m.offset - (cachedItem.start + cachedItem.height - m.listHeight())
				m.decreaseOffset(changeNeeded)
			} else {
				if cachedItem.start < m.offset {
					changeNeeded := m.offset - cachedItem.start
					m.decreaseOffset(changeNeeded)
				}
			}
		}
	}

	m.needsRerender = true
	return tea.Batch(cmds...)
}

func (m *model) goToBottom() tea.Cmd {
	var cmds []tea.Cmd
	m.reverse = true
	cmd := m.blurSelected()
	cmds = append(cmds, cmd)
	m.selectedItemInx = len(m.items) - 1
	cmd = m.focusSelected()
	cmds = append(cmds, cmd)
	m.ResetView()
	return tea.Batch(cmds...)
}

func (m *model) ResetView() {
	m.renderedItems.Clear()
	m.renderedLines = []string{}
	m.offset = 0
	m.lastRenderedInx = -1
	m.finalHight = -1
	m.needsRerender = true
}

func (m *model) goToTop() tea.Cmd {
	var cmds []tea.Cmd
	m.reverse = false
	cmd := m.blurSelected()
	cmds = append(cmds, cmd)
	m.selectedItemInx = 0
	cmd = m.focusSelected()
	cmds = append(cmds, cmd)
	m.ResetView()
	return tea.Batch(cmds...)
}

func (m *model) focusSelected() tea.Cmd {
	if m.selectedItemInx == -1 {
		return nil
	}
	if i, ok := m.items[m.selectedItemInx].(layout.Focusable); ok {
		cmd := i.Focus()
		m.rerenderItem(m.selectedItemInx)
		return cmd
	}
	return nil
}

func (m *model) blurSelected() tea.Cmd {
	if m.selectedItemInx == -1 {
		return nil
	}
	if i, ok := m.items[m.selectedItemInx].(layout.Focusable); ok {
		cmd := i.Blur()
		m.rerenderItem(m.selectedItemInx)
		return cmd
	}
	return nil
}

func (m *model) rerenderItem(inx int) {
	if inx < 0 || len(m.renderedLines) == 0 {
		return
	}
	cached, ok := m.renderedItems.Load(inx)
	cachedItem, _ := cached.(renderedItem)
	if !ok {
		// No need to rerender
		return
	}
	rerenderedItem := m.items[inx].View()
	rerenderedLines := strings.Split(rerenderedItem, "\n")
	if m.gapSize > 0 && inx != len(m.items)-1 {
		for range m.gapSize {
			rerenderedLines = append(rerenderedLines, "")
		}
	}
	// check if lines are the same
	if slices.Equal(cachedItem.lines, rerenderedLines) {
		// No changes
		return
	}
	// check if the item is in the content
	start := cachedItem.start
	logging.Info("rerenderItem", "inx", inx, "start", start, "cachedItem.start", cachedItem.start, "cachedItem.height", cachedItem.height)
	end := start + cachedItem.height
	totalLines := len(m.renderedLines)
	if m.reverse {
		end = totalLines - cachedItem.start
		start = end - cachedItem.height
	}
	if start <= totalLines && end <= totalLines {
		m.renderedLines = slices.Delete(m.renderedLines, start, end)
		m.renderedLines = slices.Insert(m.renderedLines, start, rerenderedLines...)
	}
	// TODO: if hight changed do something
	if cachedItem.height != len(rerenderedLines) && inx != len(m.items)-1 {
		if inx == len(m.items)-1 {
			m.finalHight = max(0, start+len(rerenderedLines)-m.listHeight())
		}
	}
	m.renderedItems.Store(inx, renderedItem{
		lines:  rerenderedLines,
		start:  cachedItem.start,
		height: len(rerenderedLines),
	})
	m.needsRerender = true
}

func (m *model) increaseOffset(n int) {
	if m.finalHight > -1 {
		if m.offset < m.finalHight {
			m.offset += n
			if m.offset > m.finalHight {
				m.offset = m.finalHight
			}
			m.needsRerender = true
		}
	} else {
		m.offset += n
		m.needsRerender = true
	}
}

func (m *model) decreaseOffset(n int) {
	if m.offset > 0 {
		m.offset -= n
		if m.offset < 0 {
			m.offset = 0
		}
		m.needsRerender = true
	}
}

// UpdateItem implements List.
func (m *model) UpdateItem(inx int, item util.Model) {
	m.items[inx] = item
	if m.selectedItemInx == inx {
		if i, ok := m.items[m.selectedItemInx].(layout.Focusable); ok {
			i.Focus()
		}
	}
	m.ResetView()
	m.needsRerender = true
}

// GetSize implements List.
func (m *model) GetSize() (int, int) {
	return m.width, m.height
}

// SetSize implements List.
func (m *model) SetSize(width int, height int) tea.Cmd {
	if m.width == width && m.height == height {
		return nil
	}
	if m.height != height {
		m.finalHight = -1
		m.height = height
	}
	m.width = width
	m.ResetView()
	return m.setItemsSize()
}

func (m *model) setItemsSize() tea.Cmd {
	var cmds []tea.Cmd
	width := m.width
	if m.padding != nil {
		if len(m.padding) == 1 {
			width -= m.padding[0] * 2
		} else if len(m.padding) == 2 || len(m.padding) == 3 {
			width -= m.padding[1] * 2
		} else if len(m.padding) == 4 {
			width -= m.padding[1] + m.padding[3]
		}
	}
	for _, item := range m.items {
		if i, ok := item.(layout.Sizeable); ok {
			cmd := i.SetSize(width, 0) // height is not limited
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

func (m *model) listHeight() int {
	height := m.height
	if m.padding != nil {
		if len(m.padding) == 1 {
			height -= m.padding[0] * 2
		} else if len(m.padding) == 2 {
			height -= m.padding[1] * 2
		} else if len(m.padding) == 3 {
			height -= m.padding[0] + m.padding[2]
		} else if len(m.padding) == 4 {
			height -= m.padding[0] + m.padding[2]
		}
	}
	return height
}

// AppendItem implements List.
func (m *model) AppendItem(item util.Model) tea.Cmd {
	cmd := item.Init()
	m.items = append(m.items, item)
	m.goToBottom()
	m.needsRerender = true
	return cmd
}

// DeleteItem implements List.
func (m *model) DeleteItem(i int) {
	m.items = slices.Delete(m.items, i, i+1)
	m.renderedItems.Delete(i)
	if m.selectedItemInx == i {
		m.selectedItemInx--
	}
	m.ResetView()
	m.needsRerender = true
}

// PrependItem implements List.
func (m *model) PrependItem(item util.Model) tea.Cmd {
	cmd := item.Init()
	m.items = append([]util.Model{item}, m.items...)
	// update the indices of the rendered items
	newRenderedItems := make(map[int]renderedItem)
	m.renderedItems.Range(func(key any, value any) bool {
		keyInt := key.(int)
		renderedItem := value.(renderedItem)
		newKey := keyInt + 1
		newRenderedItems[newKey] = renderedItem
		return false
	})
	m.renderedItems.Clear()
	for k, v := range newRenderedItems {
		m.renderedItems.Store(k, v)
	}
	m.goToTop()
	m.needsRerender = true
	return cmd
}

func (m *model) setReverse(reverse bool) {
	if reverse {
		m.goToBottom()
	} else {
		m.goToTop()
	}
}

// SetItems implements List.
func (m *model) SetItems(items []util.Model) tea.Cmd {
	m.items = items
	var cmds []tea.Cmd
	cmd := m.setItemsSize()
	cmds = append(cmds, cmd)
	for _, item := range m.items {
		cmds = append(cmds, item.Init())
	}
	if m.reverse {
		m.selectedItemInx = len(m.items) - 1
		cmd := m.focusSelected()
		cmds = append(cmds, cmd)
	} else {
		m.selectedItemInx = 0
		cmd := m.focusSelected()
		cmds = append(cmds, cmd)
	}
	m.needsRerender = true
	m.ResetView()
	return tea.Batch(cmds...)
}
