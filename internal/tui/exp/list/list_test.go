package list

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/tui/components/core/layout"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestList(t *testing.T) {
	t.Parallel()
	t.Run("should have correct positions in list that fits the items", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 5 {
			item := NewSelectableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := New(items, WithDirectionForward(), WithSize(10, 20)).(*list[Item])
		execCmd(l, l.Init())

		// should select the last item
		assert.Equal(t, 0, l.selectedItemIdx)
		assert.Equal(t, 0, l.offset)
		require.Equal(t, 5, len(l.indexMap))
		require.Equal(t, 5, len(l.items))
		require.Equal(t, 5, len(l.renderedItems))
		assert.Equal(t, 5, lipgloss.Height(l.rendered))
		assert.NotEqual(t, "\n", string(l.rendered[len(l.rendered)-1]), "should not end in newline")
		start, end := l.viewPosition()
		assert.Equal(t, 0, start)
		assert.Equal(t, 4, end)
		for i := range 5 {
			item, ok := l.renderedItems[items[i].ID()]
			require.True(t, ok)
			assert.Equal(t, i, item.start)
			assert.Equal(t, i, item.end)
		}

		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("should have correct positions in list that fits the items backwards", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 5 {
			item := NewSelectableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := New(items, WithDirectionBackward(), WithSize(10, 20)).(*list[Item])
		execCmd(l, l.Init())

		// should select the last item
		assert.Equal(t, 4, l.selectedItemIdx)
		assert.Equal(t, 0, l.offset)
		require.Equal(t, 5, len(l.indexMap))
		require.Equal(t, 5, len(l.items))
		require.Equal(t, 5, len(l.renderedItems))
		assert.Equal(t, 5, lipgloss.Height(l.rendered))
		assert.NotEqual(t, "\n", string(l.rendered[len(l.rendered)-1]), "should not end in newline")
		start, end := l.viewPosition()
		assert.Equal(t, 0, start)
		assert.Equal(t, 4, end)
		for i := range 5 {
			item, ok := l.renderedItems[items[i].ID()]
			require.True(t, ok)
			assert.Equal(t, i, item.start)
			assert.Equal(t, i, item.end)
		}

		golden.RequireEqual(t, []byte(l.View()))
	})

	t.Run("should have correct positions in list that does not fits the items", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 30 {
			item := NewSelectableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := New(items, WithDirectionForward(), WithSize(10, 10)).(*list[Item])
		execCmd(l, l.Init())

		// should select the last item
		assert.Equal(t, 0, l.selectedItemIdx)
		assert.Equal(t, 0, l.offset)
		require.Equal(t, 30, len(l.indexMap))
		require.Equal(t, 30, len(l.items))
		require.Equal(t, 30, len(l.renderedItems))
		assert.Equal(t, 30, lipgloss.Height(l.rendered))
		assert.NotEqual(t, "\n", string(l.rendered[len(l.rendered)-1]), "should not end in newline")
		start, end := l.viewPosition()
		assert.Equal(t, 0, start)
		assert.Equal(t, 9, end)
		for i := range 30 {
			item, ok := l.renderedItems[items[i].ID()]
			require.True(t, ok)
			assert.Equal(t, i, item.start)
			assert.Equal(t, i, item.end)
		}

		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("should have correct positions in list that does not fits the items backwards", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 30 {
			item := NewSelectableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := New(items, WithDirectionBackward(), WithSize(10, 10)).(*list[Item])
		execCmd(l, l.Init())

		// should select the last item
		assert.Equal(t, 29, l.selectedItemIdx)
		assert.Equal(t, 0, l.offset)
		require.Equal(t, 30, len(l.indexMap))
		require.Equal(t, 30, len(l.items))
		require.Equal(t, 30, len(l.renderedItems))
		assert.Equal(t, 30, lipgloss.Height(l.rendered))
		assert.NotEqual(t, "\n", string(l.rendered[len(l.rendered)-1]), "should not end in newline")
		start, end := l.viewPosition()
		assert.Equal(t, 20, start)
		assert.Equal(t, 29, end)
		for i := range 30 {
			item, ok := l.renderedItems[items[i].ID()]
			require.True(t, ok)
			assert.Equal(t, i, item.start)
			assert.Equal(t, i, item.end)
		}

		golden.RequireEqual(t, []byte(l.View()))
	})

	t.Run("should have correct positions in list that does not fits the items and has multi line items", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 30 {
			content := strings.Repeat(fmt.Sprintf("Item %d\n", i), i+1)
			content = strings.TrimSuffix(content, "\n")
			item := NewSelectableItem(content)
			items = append(items, item)
		}
		l := New(items, WithDirectionForward(), WithSize(10, 10)).(*list[Item])
		execCmd(l, l.Init())

		// should select the last item
		assert.Equal(t, 0, l.selectedItemIdx)
		assert.Equal(t, 0, l.offset)
		require.Equal(t, 30, len(l.indexMap))
		require.Equal(t, 30, len(l.items))
		require.Equal(t, 30, len(l.renderedItems))
		expectedLines := 0
		for i := range 30 {
			expectedLines += (i + 1) * 1
		}
		assert.Equal(t, expectedLines, lipgloss.Height(l.rendered))
		assert.NotEqual(t, "\n", string(l.rendered[len(l.rendered)-1]), "should not end in newline")
		start, end := l.viewPosition()
		assert.Equal(t, 0, start)
		assert.Equal(t, 9, end)
		currentPosition := 0
		for i := range 30 {
			rItem, ok := l.renderedItems[items[i].ID()]
			require.True(t, ok)
			assert.Equal(t, currentPosition, rItem.start)
			assert.Equal(t, currentPosition+i, rItem.end)
			currentPosition += i + 1
		}

		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("should have correct positions in list that does not fits the items and has multi line items backwards", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 30 {
			content := strings.Repeat(fmt.Sprintf("Item %d\n", i), i+1)
			content = strings.TrimSuffix(content, "\n")
			item := NewSelectableItem(content)
			items = append(items, item)
		}
		l := New(items, WithDirectionBackward(), WithSize(10, 10)).(*list[Item])
		execCmd(l, l.Init())

		// should select the last item
		assert.Equal(t, 29, l.selectedItemIdx)
		assert.Equal(t, 0, l.offset)
		require.Equal(t, 30, len(l.indexMap))
		require.Equal(t, 30, len(l.items))
		require.Equal(t, 30, len(l.renderedItems))
		expectedLines := 0
		for i := range 30 {
			expectedLines += (i + 1) * 1
		}
		assert.Equal(t, expectedLines, lipgloss.Height(l.rendered))
		assert.NotEqual(t, "\n", string(l.rendered[len(l.rendered)-1]), "should not end in newline")
		start, end := l.viewPosition()
		assert.Equal(t, expectedLines-10, start)
		assert.Equal(t, expectedLines-1, end)
		currentPosition := 0
		for i := range 30 {
			rItem, ok := l.renderedItems[items[i].ID()]
			require.True(t, ok)
			assert.Equal(t, currentPosition, rItem.start)
			assert.Equal(t, currentPosition+i, rItem.end)
			currentPosition += i + 1
		}

		golden.RequireEqual(t, []byte(l.View()))
	})

	t.Run("should go to selected item at the beginning", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 30 {
			content := strings.Repeat(fmt.Sprintf("Item %d\n", i), i+1)
			content = strings.TrimSuffix(content, "\n")
			item := NewSelectableItem(content)
			items = append(items, item)
		}
		l := New(items, WithDirectionForward(), WithSize(10, 10), WithSelectedItem(items[10].ID())).(*list[Item])
		execCmd(l, l.Init())

		// should select the last item
		assert.Equal(t, 10, l.selectedItemIdx)

		golden.RequireEqual(t, []byte(l.View()))
	})

	t.Run("should go to selected item at the beginning backwards", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 30 {
			content := strings.Repeat(fmt.Sprintf("Item %d\n", i), i+1)
			content = strings.TrimSuffix(content, "\n")
			item := NewSelectableItem(content)
			items = append(items, item)
		}
		l := New(items, WithDirectionBackward(), WithSize(10, 10), WithSelectedItem(items[10].ID())).(*list[Item])
		execCmd(l, l.Init())

		// should select the last item
		assert.Equal(t, 10, l.selectedItemIdx)

		golden.RequireEqual(t, []byte(l.View()))
	})
}

func TestListMovement(t *testing.T) {
	t.Parallel()
	t.Run("should move viewport up", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 30 {
			content := strings.Repeat(fmt.Sprintf("Item %d\n", i), i+1)
			content = strings.TrimSuffix(content, "\n")
			item := NewSelectableItem(content)
			items = append(items, item)
		}
		l := New(items, WithDirectionBackward(), WithSize(10, 10)).(*list[Item])
		execCmd(l, l.Init())

		execCmd(l, l.MoveUp(25))

		assert.Equal(t, 25, l.offset)
		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("should move viewport up and down", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 30 {
			content := strings.Repeat(fmt.Sprintf("Item %d\n", i), i+1)
			content = strings.TrimSuffix(content, "\n")
			item := NewSelectableItem(content)
			items = append(items, item)
		}
		l := New(items, WithDirectionBackward(), WithSize(10, 10)).(*list[Item])
		execCmd(l, l.Init())

		execCmd(l, l.MoveUp(25))
		execCmd(l, l.MoveDown(25))

		assert.Equal(t, 0, l.offset)
		golden.RequireEqual(t, []byte(l.View()))
	})

	t.Run("should move viewport down", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 30 {
			content := strings.Repeat(fmt.Sprintf("Item %d\n", i), i+1)
			content = strings.TrimSuffix(content, "\n")
			item := NewSelectableItem(content)
			items = append(items, item)
		}
		l := New(items, WithDirectionForward(), WithSize(10, 10)).(*list[Item])
		execCmd(l, l.Init())

		execCmd(l, l.MoveDown(25))

		assert.Equal(t, 25, l.offset)
		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("should move viewport down and up", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 30 {
			content := strings.Repeat(fmt.Sprintf("Item %d\n", i), i+1)
			content = strings.TrimSuffix(content, "\n")
			item := NewSelectableItem(content)
			items = append(items, item)
		}
		l := New(items, WithDirectionForward(), WithSize(10, 10)).(*list[Item])
		execCmd(l, l.Init())

		execCmd(l, l.MoveDown(25))
		execCmd(l, l.MoveUp(25))

		assert.Equal(t, 0, l.offset)
		golden.RequireEqual(t, []byte(l.View()))
	})

	t.Run("should not change offset when new items are appended and we are at the bottom in backwards list", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 30 {
			content := strings.Repeat(fmt.Sprintf("Item %d\n", i), i+1)
			content = strings.TrimSuffix(content, "\n")
			item := NewSelectableItem(content)
			items = append(items, item)
		}
		l := New(items, WithDirectionBackward(), WithSize(10, 10)).(*list[Item])
		execCmd(l, l.Init())
		execCmd(l, l.AppendItem(NewSelectableItem("Testing")))

		assert.Equal(t, 0, l.offset)
		golden.RequireEqual(t, []byte(l.View()))
	})

	t.Run("should stay at the position it is when new items are added but we moved up in backwards list", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 30 {
			item := NewSelectableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := New(items, WithDirectionBackward(), WithSize(10, 10)).(*list[Item])
		execCmd(l, l.Init())

		execCmd(l, l.MoveUp(2))
		viewBefore := l.View()
		execCmd(l, l.AppendItem(NewSelectableItem("Testing\nHello\n")))
		viewAfter := l.View()
		assert.Equal(t, viewBefore, viewAfter)
		assert.Equal(t, 5, l.offset)
		assert.Equal(t, 33, lipgloss.Height(l.rendered))
		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("should stay at the position it is when the hight of an item below is increased in backwards list", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 30 {
			item := NewSelectableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := New(items, WithDirectionBackward(), WithSize(10, 10)).(*list[Item])
		execCmd(l, l.Init())

		execCmd(l, l.MoveUp(2))
		viewBefore := l.View()
		item := items[29]
		execCmd(l, l.UpdateItem(item.ID(), NewSelectableItem("Item 29\nLine 2\nLine 3")))
		viewAfter := l.View()
		assert.Equal(t, viewBefore, viewAfter)
		assert.Equal(t, 4, l.offset)
		assert.Equal(t, 32, lipgloss.Height(l.rendered))
		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("should stay at the position it is when the hight of an item below is decreases in backwards list", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 30 {
			item := NewSelectableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		items = append(items, NewSelectableItem("Item 30\nLine 2\nLine 3"))
		l := New(items, WithDirectionBackward(), WithSize(10, 10)).(*list[Item])
		execCmd(l, l.Init())

		execCmd(l, l.MoveUp(2))
		viewBefore := l.View()
		item := items[30]
		execCmd(l, l.UpdateItem(item.ID(), NewSelectableItem("Item 30")))
		viewAfter := l.View()
		assert.Equal(t, viewBefore, viewAfter)
		assert.Equal(t, 0, l.offset)
		assert.Equal(t, 31, lipgloss.Height(l.rendered))
		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("should stay at the position it is when the hight of an item above is increased in backwards list", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 30 {
			item := NewSelectableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := New(items, WithDirectionBackward(), WithSize(10, 10)).(*list[Item])
		execCmd(l, l.Init())

		execCmd(l, l.MoveUp(2))
		viewBefore := l.View()
		item := items[1]
		execCmd(l, l.UpdateItem(item.ID(), NewSelectableItem("Item 1\nLine 2\nLine 3")))
		viewAfter := l.View()
		assert.Equal(t, viewBefore, viewAfter)
		assert.Equal(t, 2, l.offset)
		assert.Equal(t, 32, lipgloss.Height(l.rendered))
		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("should stay at the position it is if an item is prepended and we are in backwards list", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 30 {
			item := NewSelectableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := New(items, WithDirectionBackward(), WithSize(10, 10)).(*list[Item])
		execCmd(l, l.Init())

		execCmd(l, l.MoveUp(2))
		viewBefore := l.View()
		execCmd(l, l.PrependItem(NewSelectableItem("New")))
		viewAfter := l.View()
		assert.Equal(t, viewBefore, viewAfter)
		assert.Equal(t, 2, l.offset)
		assert.Equal(t, 31, lipgloss.Height(l.rendered))
		golden.RequireEqual(t, []byte(l.View()))
	})

	t.Run("should not change offset when new items are prepended and we are at the top in forward list", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 30 {
			content := strings.Repeat(fmt.Sprintf("Item %d\n", i), i+1)
			content = strings.TrimSuffix(content, "\n")
			item := NewSelectableItem(content)
			items = append(items, item)
		}
		l := New(items, WithDirectionForward(), WithSize(10, 10)).(*list[Item])
		execCmd(l, l.Init())
		execCmd(l, l.PrependItem(NewSelectableItem("Testing")))

		assert.Equal(t, 0, l.offset)
		golden.RequireEqual(t, []byte(l.View()))
	})

	t.Run("should stay at the position it is when new items are added but we moved down in forward list", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 30 {
			item := NewSelectableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := New(items, WithDirectionForward(), WithSize(10, 10)).(*list[Item])
		execCmd(l, l.Init())

		execCmd(l, l.MoveDown(2))
		viewBefore := l.View()
		execCmd(l, l.PrependItem(NewSelectableItem("Testing\nHello\n")))
		viewAfter := l.View()
		assert.Equal(t, viewBefore, viewAfter)
		assert.Equal(t, 5, l.offset)
		assert.Equal(t, 33, lipgloss.Height(l.rendered))
		golden.RequireEqual(t, []byte(l.View()))
	})

	t.Run("should stay at the position it is when the hight of an item above is increased in forward list", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 30 {
			item := NewSelectableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := New(items, WithDirectionForward(), WithSize(10, 10)).(*list[Item])
		execCmd(l, l.Init())

		execCmd(l, l.MoveDown(2))
		viewBefore := l.View()
		item := items[0]
		execCmd(l, l.UpdateItem(item.ID(), NewSelectableItem("Item 29\nLine 2\nLine 3")))
		viewAfter := l.View()
		assert.Equal(t, viewBefore, viewAfter)
		assert.Equal(t, 4, l.offset)
		assert.Equal(t, 32, lipgloss.Height(l.rendered))
		golden.RequireEqual(t, []byte(l.View()))
	})

	t.Run("should stay at the position it is when the hight of an item above is decreases in forward list", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		items = append(items, NewSelectableItem("At top\nLine 2\nLine 3"))
		for i := range 30 {
			item := NewSelectableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := New(items, WithDirectionForward(), WithSize(10, 10)).(*list[Item])
		execCmd(l, l.Init())

		execCmd(l, l.MoveDown(3))
		viewBefore := l.View()
		item := items[0]
		execCmd(l, l.UpdateItem(item.ID(), NewSelectableItem("At top")))
		viewAfter := l.View()
		assert.Equal(t, viewBefore, viewAfter)
		assert.Equal(t, 1, l.offset)
		assert.Equal(t, 31, lipgloss.Height(l.rendered))
		golden.RequireEqual(t, []byte(l.View()))
	})

	t.Run("should stay at the position it is when the hight of an item below is increased in forward list", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 30 {
			item := NewSelectableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := New(items, WithDirectionForward(), WithSize(10, 10)).(*list[Item])
		execCmd(l, l.Init())

		execCmd(l, l.MoveDown(2))
		viewBefore := l.View()
		item := items[29]
		execCmd(l, l.UpdateItem(item.ID(), NewSelectableItem("Item 29\nLine 2\nLine 3")))
		viewAfter := l.View()
		assert.Equal(t, viewBefore, viewAfter)
		assert.Equal(t, 2, l.offset)
		assert.Equal(t, 32, lipgloss.Height(l.rendered))
		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("should stay at the position it is if an item is appended and we are in forward list", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 30 {
			item := NewSelectableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := New(items, WithDirectionForward(), WithSize(10, 10)).(*list[Item])
		execCmd(l, l.Init())

		execCmd(l, l.MoveDown(2))
		viewBefore := l.View()
		execCmd(l, l.AppendItem(NewSelectableItem("New")))
		viewAfter := l.View()
		assert.Equal(t, viewBefore, viewAfter)
		assert.Equal(t, 2, l.offset)
		assert.Equal(t, 31, lipgloss.Height(l.rendered))
		golden.RequireEqual(t, []byte(l.View()))
	})
}

type SelectableItem interface {
	Item
	layout.Focusable
}

type simpleItem struct {
	width   int
	content string
	id      string
}
type selectableItem struct {
	*simpleItem
	focused bool
}

func NewSimpleItem(content string) *simpleItem {
	return &simpleItem{
		id:      uuid.NewString(),
		width:   0,
		content: content,
	}
}

func NewSelectableItem(content string) SelectableItem {
	return &selectableItem{
		simpleItem: NewSimpleItem(content),
		focused:    false,
	}
}

func (s *simpleItem) ID() string {
	return s.id
}

func (s *simpleItem) Init() tea.Cmd {
	return nil
}

func (s *simpleItem) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	return s, nil
}

func (s *simpleItem) View() string {
	return lipgloss.NewStyle().Width(s.width).Render(s.content)
}

func (l *simpleItem) GetSize() (int, int) {
	return l.width, 0
}

// SetSize implements Item.
func (s *simpleItem) SetSize(width int, height int) tea.Cmd {
	s.width = width
	return nil
}

func (s *selectableItem) View() string {
	if s.focused {
		return lipgloss.NewStyle().BorderLeft(true).BorderStyle(lipgloss.NormalBorder()).Width(s.width).Render(s.content)
	}
	return lipgloss.NewStyle().Width(s.width).Render(s.content)
}

// Blur implements SimpleItem.
func (s *selectableItem) Blur() tea.Cmd {
	s.focused = false
	return nil
}

// Focus implements SimpleItem.
func (s *selectableItem) Focus() tea.Cmd {
	s.focused = true
	return nil
}

// IsFocused implements SimpleItem.
func (s *selectableItem) IsFocused() bool {
	return s.focused
}

func execCmd(m util.Model, cmd tea.Cmd) {
	for cmd != nil {
		msg := cmd()
		m, cmd = m.Update(msg)
	}
}
