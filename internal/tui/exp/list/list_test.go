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

		// should select item 10
		assert.Equal(t, items[0].ID(), l.SelectedItemID())
		assert.Equal(t, 0, l.offset)
		require.Equal(t, 5, l.indexMap.Len())
		require.Equal(t, 5, l.items.Len())
		require.Equal(t, 5, len(l.itemPositions))
		assert.Equal(t, 5, lipgloss.Height(l.rendered))
		assert.NotEqual(t, "\n", string(l.rendered[len(l.rendered)-1]), "should not end in newline")
		start, end := l.viewPosition()
		assert.Equal(t, 0, start)
		assert.Equal(t, 4, end)
		for i := range 5 {
			item := l.itemPositions[i]
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

		// should select item 10
		assert.Equal(t, items[4].ID(), l.SelectedItemID())
		assert.Equal(t, 0, l.offset)
		require.Equal(t, 5, l.indexMap.Len())
		require.Equal(t, 5, l.items.Len())
		require.Equal(t, 5, len(l.itemPositions))
		assert.Equal(t, 5, lipgloss.Height(l.rendered))
		assert.NotEqual(t, "\n", string(l.rendered[len(l.rendered)-1]), "should not end in newline")
		start, end := l.viewPosition()
		assert.Equal(t, 0, start)
		assert.Equal(t, 4, end)
		for i := range 5 {
			item := l.itemPositions[i]
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

		// should select item 10
		assert.Equal(t, items[0].ID(), l.SelectedItemID())
		assert.Equal(t, 0, l.offset)
		require.Equal(t, 30, l.indexMap.Len())
		require.Equal(t, 30, l.items.Len())
		require.Equal(t, 30, len(l.itemPositions))
		// With virtual scrolling, rendered height should be viewport height (10)
		assert.Equal(t, 10, lipgloss.Height(l.rendered))
		assert.NotEqual(t, "\n", string(l.rendered[len(l.rendered)-1]), "should not end in newline")
		start, end := l.viewPosition()
		assert.Equal(t, 0, start)
		assert.Equal(t, 9, end)
		for i := range 30 {
			item := l.itemPositions[i]
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

		// should select item 10
		assert.Equal(t, items[29].ID(), l.SelectedItemID())
		assert.Equal(t, 0, l.offset)
		require.Equal(t, 30, l.indexMap.Len())
		require.Equal(t, 30, l.items.Len())
		require.Equal(t, 30, len(l.itemPositions))
		// With virtual scrolling, rendered height should be viewport height (10)
		assert.Equal(t, 10, lipgloss.Height(l.rendered))
		assert.NotEqual(t, "\n", string(l.rendered[len(l.rendered)-1]), "should not end in newline")
		start, end := l.viewPosition()
		assert.Equal(t, 20, start)
		assert.Equal(t, 29, end)
		for i := range 30 {
			item := l.itemPositions[i]
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

		// should select item 10
		assert.Equal(t, items[0].ID(), l.SelectedItemID())
		assert.Equal(t, 0, l.offset)
		require.Equal(t, 30, l.indexMap.Len())
		require.Equal(t, 30, l.items.Len())
		require.Equal(t, 30, len(l.itemPositions))
		expectedLines := 0
		for i := range 30 {
			expectedLines += (i + 1) * 1
		}
		// With virtual scrolling, rendered height should be viewport height (10)
		assert.Equal(t, 10, lipgloss.Height(l.rendered))
		if len(l.rendered) > 0 {
			assert.NotEqual(t, "\n", string(l.rendered[len(l.rendered)-1]), "should not end in newline")
		}
		start, end := l.viewPosition()
		assert.Equal(t, 0, start)
		assert.Equal(t, 9, end)
		currentPosition := 0
		for i := range 30 {
			rItem := l.itemPositions[i]
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

		// should select item 10
		assert.Equal(t, items[29].ID(), l.SelectedItemID())
		assert.Equal(t, 0, l.offset)
		require.Equal(t, 30, l.indexMap.Len())
		require.Equal(t, 30, l.items.Len())
		require.Equal(t, 30, len(l.itemPositions))
		expectedLines := 0
		for i := range 30 {
			expectedLines += (i + 1) * 1
		}
		// With virtual scrolling, rendered height should be viewport height (10)
		assert.Equal(t, 10, lipgloss.Height(l.rendered))
		if len(l.rendered) > 0 {
			assert.NotEqual(t, "\n", string(l.rendered[len(l.rendered)-1]), "should not end in newline")
		}
		start, end := l.viewPosition()
		assert.Equal(t, expectedLines-10, start)
		assert.Equal(t, expectedLines-1, end)
		currentPosition := 0
		for i := range 30 {
			rItem := l.itemPositions[i]
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

		// should select item 10
		assert.Equal(t, items[10].ID(), l.SelectedItemID())

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

		// should select item 10
		assert.Equal(t, items[10].ID(), l.SelectedItemID())

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
		// With virtual scrolling, rendered height should be viewport height (10)
		assert.Equal(t, 10, lipgloss.Height(l.rendered))
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
		// With virtual scrolling, rendered height should be viewport height (10)
		assert.Equal(t, 10, lipgloss.Height(l.rendered))
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
		// With virtual scrolling, rendered height should be viewport height (10)
		assert.Equal(t, 10, lipgloss.Height(l.rendered))
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
		// With virtual scrolling, rendered height should be viewport height (10)
		assert.Equal(t, 10, lipgloss.Height(l.rendered))
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
		// With virtual scrolling, rendered height should be viewport height (10)
		assert.Equal(t, 10, lipgloss.Height(l.rendered))
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
		// With virtual scrolling, rendered height should be viewport height (10)
		assert.Equal(t, 10, lipgloss.Height(l.rendered))
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
		// With virtual scrolling, rendered height should be viewport height (10)
		assert.Equal(t, 10, lipgloss.Height(l.rendered))
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
		// With virtual scrolling, rendered height should be viewport height (10)
		assert.Equal(t, 10, lipgloss.Height(l.rendered))
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
		// With virtual scrolling, rendered height should be viewport height (10)
		assert.Equal(t, 10, lipgloss.Height(l.rendered))
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
		// With virtual scrolling, rendered height should be viewport height (10)
		assert.Equal(t, 10, lipgloss.Height(l.rendered))
		golden.RequireEqual(t, []byte(l.View()))
	})

	t.Run("should scroll to top with SelectItemAbove and render 5 lines", func(t *testing.T) {
		t.Parallel()
		// Create 10 items
		items := []Item{}
		for i := range 10 {
			item := NewSelectableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		
		// Create list with viewport of 5 lines height and 20 width, starting at the bottom (index 9)
		l := New(items, WithDirectionForward(), WithSize(20, 5), WithSelectedIndex(9)).(*list[Item])
		execCmd(l, l.Init())
		
		// Verify we start at the bottom (item 9 selected)
		assert.Equal(t, items[9].ID(), l.SelectedItemID())
		assert.Equal(t, 9, l.SelectedItemIndex())
		
		// Scroll to top one by one using SelectItemAbove
		for i := 8; i >= 0; i-- {
			execCmd(l, l.SelectItemAbove())
			assert.Equal(t, items[i].ID(), l.SelectedItemID())
			assert.Equal(t, i, l.SelectedItemIndex())
		}
		
		// Now we should be at the first item
		assert.Equal(t, items[0].ID(), l.SelectedItemID())
		assert.Equal(t, 0, l.SelectedItemIndex())
		
		// Verify the viewport is rendering exactly 5 lines
		rendered := l.View()
		
		// Check the height using lipgloss
		assert.Equal(t, 5, lipgloss.Height(rendered), "Should render exactly 5 lines")
		
		// Verify offset is at the top
		assert.Equal(t, 0, l.offset)
		
		// Verify the viewport position
		start, end := l.viewPosition()
		assert.Equal(t, 0, start, "View should start at position 0")
		assert.Equal(t, 4, end, "View should end at position 4")
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
