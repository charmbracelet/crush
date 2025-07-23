package list

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/tui/components/core/layout"
	"github.com/charmbracelet/lipgloss/v2"
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
		assert.Equal(t, items[0].ID(), l.selectedItem)
		assert.Equal(t, 0, l.offset)
		require.Len(t, l.indexMap, 5)
		require.Len(t, l.items, 5)
		require.Len(t, l.renderedItems, 5)
		assert.Equal(t, 5, lipgloss.Height(l.rendered))
		assert.NotEqual(t, "\n", string(l.rendered[len(l.rendered)-1]), "should not end in newline")
		start, end := l.viewPosition()
		assert.Equal(t, 0, start)
		assert.Equal(t, 4, end)
		for i := range 5 {
			assert.Equal(t, i, l.renderedItems[items[i].ID()].start)
			assert.Equal(t, i, l.renderedItems[items[i].ID()].end)
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
		assert.Equal(t, items[4].ID(), l.selectedItem)
		assert.Equal(t, 0, l.offset)
		require.Len(t, l.indexMap, 5)
		require.Len(t, l.items, 5)
		require.Len(t, l.renderedItems, 5)
		assert.Equal(t, 5, lipgloss.Height(l.rendered))
		assert.NotEqual(t, "\n", string(l.rendered[len(l.rendered)-1]), "should not end in newline")
		start, end := l.viewPosition()
		assert.Equal(t, 0, start)
		assert.Equal(t, 4, end)
		for i := range 5 {
			assert.Equal(t, i, l.renderedItems[items[i].ID()].start)
			assert.Equal(t, i, l.renderedItems[items[i].ID()].end)
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
		assert.Equal(t, items[0].ID(), l.selectedItem)
		assert.Equal(t, 0, l.offset)
		require.Len(t, l.indexMap, 30)
		require.Len(t, l.items, 30)
		require.Len(t, l.renderedItems, 30)
		assert.Equal(t, 30, lipgloss.Height(l.rendered))
		assert.NotEqual(t, "\n", string(l.rendered[len(l.rendered)-1]), "should not end in newline")
		start, end := l.viewPosition()
		assert.Equal(t, 0, start)
		assert.Equal(t, 9, end)
		for i := range 30 {
			assert.Equal(t, i, l.renderedItems[items[i].ID()].start)
			assert.Equal(t, i, l.renderedItems[items[i].ID()].end)
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
		assert.Equal(t, items[29].ID(), l.selectedItem)
		assert.Equal(t, 0, l.offset)
		require.Len(t, l.indexMap, 30)
		require.Len(t, l.items, 30)
		require.Len(t, l.renderedItems, 30)
		assert.Equal(t, 30, lipgloss.Height(l.rendered))
		assert.NotEqual(t, "\n", string(l.rendered[len(l.rendered)-1]), "should not end in newline")
		start, end := l.viewPosition()
		assert.Equal(t, 20, start)
		assert.Equal(t, 29, end)
		for i := range 30 {
			assert.Equal(t, i, l.renderedItems[items[i].ID()].start)
			assert.Equal(t, i, l.renderedItems[items[i].ID()].end)
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
		assert.Equal(t, items[0].ID(), l.selectedItem)
		assert.Equal(t, 0, l.offset)
		require.Len(t, l.indexMap, 30)
		require.Len(t, l.items, 30)
		require.Len(t, l.renderedItems, 30)
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
			rItem := l.renderedItems[items[i].ID()]
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
		assert.Equal(t, items[29].ID(), l.selectedItem)
		assert.Equal(t, 0, l.offset)
		require.Len(t, l.indexMap, 30)
		require.Len(t, l.items, 30)
		require.Len(t, l.renderedItems, 30)
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
			rItem := l.renderedItems[items[i].ID()]
			assert.Equal(t, currentPosition, rItem.start)
			assert.Equal(t, currentPosition+i, rItem.end)
			currentPosition += i + 1
		}

		golden.RequireEqual(t, []byte(l.View()))
	})

	t.Run("should go to selected item and center", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 30 {
			content := strings.Repeat(fmt.Sprintf("Item %d\n", i), i+1)
			content = strings.TrimSuffix(content, "\n")
			item := NewSelectableItem(content)
			items = append(items, item)
		}
		l := New(items, WithDirectionForward(), WithSize(10, 10), WithSelectedItem(items[4].ID())).(*list[Item])
		execCmd(l, l.Init())

		// should select the last item
		assert.Equal(t, items[4].ID(), l.selectedItem)

		golden.RequireEqual(t, []byte(l.View()))
	})

	t.Run("should go to selected item and center backwards", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 30 {
			content := strings.Repeat(fmt.Sprintf("Item %d\n", i), i+1)
			content = strings.TrimSuffix(content, "\n")
			item := NewSelectableItem(content)
			items = append(items, item)
		}
		l := New(items, WithDirectionBackward(), WithSize(10, 10), WithSelectedItem(items[4].ID())).(*list[Item])
		execCmd(l, l.Init())

		// should select the last item
		assert.Equal(t, items[4].ID(), l.selectedItem)

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
		assert.Equal(t, items[10].ID(), l.selectedItem)

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
		assert.Equal(t, items[10].ID(), l.selectedItem)

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

func (s *simpleItem) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func execCmd(m tea.Model, cmd tea.Cmd) {
	for cmd != nil {
		msg := cmd()
		m, cmd = m.Update(msg)
	}
}
