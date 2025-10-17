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

func TestViewPosition(t *testing.T) {
	t.Parallel()
	
	t.Run("forward direction - normal scrolling", func(t *testing.T) {
		t.Parallel()
		items := []Item{createItem("test", 1)}
		l := New(items, WithDirectionForward(), WithSize(20, 10)).(*list[Item])
		l.virtualHeight = 50
		
		// At the top
		l.offset = 0
		start, end := l.viewPosition()
		assert.Equal(t, 0, start)
		assert.Equal(t, 9, end)
		
		// In the middle
		l.offset = 20
		start, end = l.viewPosition()
		assert.Equal(t, 20, start)
		assert.Equal(t, 29, end)
		
		// Near the bottom
		l.offset = 40
		start, end = l.viewPosition()
		assert.Equal(t, 40, start)
		assert.Equal(t, 49, end)
		
		// Past the maximum valid offset (should be clamped)
		l.offset = 45
		start, end = l.viewPosition()
		assert.Equal(t, 40, start) // Clamped to max valid offset
		assert.Equal(t, 49, end)
		
		// Way past the end (should be clamped)
		l.offset = 100
		start, end = l.viewPosition()
		assert.Equal(t, 40, start) // Clamped to max valid offset
		assert.Equal(t, 49, end)
	})
	
	t.Run("forward direction - edge case with exact fit", func(t *testing.T) {
		t.Parallel()
		items := []Item{createItem("test", 1)}
		l := New(items, WithDirectionForward(), WithSize(20, 10)).(*list[Item])
		l.virtualHeight = 10
		
		l.offset = 0
		start, end := l.viewPosition()
		assert.Equal(t, 0, start)
		assert.Equal(t, 9, end)
		
		// Offset beyond valid range should be clamped
		l.offset = 5
		start, end = l.viewPosition()
		assert.Equal(t, 0, start)
		assert.Equal(t, 9, end)
	})
	
	t.Run("forward direction - content smaller than viewport", func(t *testing.T) {
		t.Parallel()
		items := []Item{createItem("test", 1)}
		l := New(items, WithDirectionForward(), WithSize(20, 10)).(*list[Item])
		l.virtualHeight = 5
		
		l.offset = 0
		start, end := l.viewPosition()
		assert.Equal(t, 0, start)
		assert.Equal(t, 4, end)
		
		// Any offset should be clamped to 0
		l.offset = 10
		start, end = l.viewPosition()
		assert.Equal(t, 0, start)
		assert.Equal(t, 4, end)
	})
	
	t.Run("backward direction - normal scrolling", func(t *testing.T) {
		t.Parallel()
		items := []Item{createItem("test", 1)}
		l := New(items, WithDirectionBackward(), WithSize(20, 10)).(*list[Item])
		l.virtualHeight = 50
		
		// At the bottom (offset 0 in backward mode)
		l.offset = 0
		start, end := l.viewPosition()
		assert.Equal(t, 40, start)
		assert.Equal(t, 49, end)
		
		// In the middle
		l.offset = 20
		start, end = l.viewPosition()
		assert.Equal(t, 20, start)
		assert.Equal(t, 29, end)
		
		// Near the top
		l.offset = 40
		start, end = l.viewPosition()
		assert.Equal(t, 0, start)
		assert.Equal(t, 9, end)
		
		// Past the maximum valid offset (should be clamped)
		l.offset = 45
		start, end = l.viewPosition()
		assert.Equal(t, 0, start)
		assert.Equal(t, 9, end)
	})
	
	t.Run("backward direction - edge cases", func(t *testing.T) {
		t.Parallel()
		items := []Item{createItem("test", 1)}
		l := New(items, WithDirectionBackward(), WithSize(20, 10)).(*list[Item])
		l.virtualHeight = 5
		
		// Content smaller than viewport
		l.offset = 0
		start, end := l.viewPosition()
		assert.Equal(t, 0, start)
		assert.Equal(t, 4, end)
		
		// Any offset should show all content
		l.offset = 10
		start, end = l.viewPosition()
		assert.Equal(t, 0, start)
		assert.Equal(t, 4, end)
	})
}

// Helper to create a test item with specific height
func createItem(id string, height int) Item {
		content := strings.Repeat(id+"\n", height)
		if height > 0 {
			content = strings.TrimSuffix(content, "\n")
		}
		item := &testItem{
			id:      id,
			content: content,
		}
	return item
}

func TestRenderVirtualScrolling(t *testing.T) {
	t.Parallel()
	
	t.Run("should handle partially visible items at top", func(t *testing.T) {
		t.Parallel()
		items := []Item{
			createItem("A", 1),
			createItem("B", 5),
			createItem("C", 1),
			createItem("D", 3),
		}
		
		l := New(items, WithDirectionForward(), WithSize(20, 3)).(*list[Item])
		execCmd(l, l.Init())
		
		// Position B partially visible at top
		l.offset = 2 // Start viewing from line 2 (middle of B)
		l.calculateItemPositions()
		
		// Item positions: A(0-0), B(1-5), C(6-6), D(7-9)
		// Viewport: lines 2-4 (height=3)
		// Should show: lines 2-4 of B (3 lines from B)
		
		rendered := l.renderVirtualScrolling()
		lines := strings.Split(rendered, "\n")
		assert.Equal(t, 3, len(lines))
		assert.Equal(t, "B", lines[0])
		assert.Equal(t, "B", lines[1])
		assert.Equal(t, "B", lines[2])
	})
	
	t.Run("should handle gaps between items correctly", func(t *testing.T) {
		t.Parallel()
		items := []Item{
			createItem("A", 1),
			createItem("B", 1),
			createItem("C", 1),
		}
		
		l := New(items, WithDirectionForward(), WithSize(20, 5), WithGap(1)).(*list[Item])
		execCmd(l, l.Init())
		
		// Item positions: A(0-0), gap(1), B(2-2), gap(3), C(4-4)
		// Viewport: lines 0-4 (height=5)
		// Should show all items with gaps
		
		rendered := l.renderVirtualScrolling()
		lines := strings.Split(rendered, "\n")
		assert.Equal(t, 5, len(lines))
		assert.Equal(t, "A", lines[0])
		assert.Equal(t, "", lines[1]) // gap
		assert.Equal(t, "B", lines[2])
		assert.Equal(t, "", lines[3]) // gap
		assert.Equal(t, "C", lines[4])
	})
	
	t.Run("should not show empty lines when scrolled to bottom", func(t *testing.T) {
		t.Parallel()
		items := []Item{
			createItem("A", 2),
			createItem("B", 2),
			createItem("C", 2),
			createItem("D", 2),
			createItem("E", 2),
		}
		
		l := New(items, WithDirectionForward(), WithSize(20, 4)).(*list[Item])
		execCmd(l, l.Init())
		l.calculateItemPositions()
		
		// Total height: 10 lines (5 items * 2 lines each)
		// Scroll to show last 4 lines
		l.offset = 6
		
		rendered := l.renderVirtualScrolling()
		lines := strings.Split(rendered, "\n")
		assert.Equal(t, 4, len(lines))
		// Should show last 2 items completely
		assert.Equal(t, "D", lines[0])
		assert.Equal(t, "D", lines[1])
		assert.Equal(t, "E", lines[2])
		assert.Equal(t, "E", lines[3])
	})
	
	t.Run("should handle offset at maximum boundary", func(t *testing.T) {
		t.Parallel()
		items := []Item{
			createItem("A", 3),
			createItem("B", 3),
			createItem("C", 3),
			createItem("D", 3),
		}
		
		l := New(items, WithDirectionForward(), WithSize(20, 5)).(*list[Item])
		execCmd(l, l.Init())
		l.calculateItemPositions()
		
		// Total height: 12 lines
		// Max valid offset: 12 - 5 = 7
		l.offset = 7
		
		rendered := l.renderVirtualScrolling()
		lines := strings.Split(rendered, "\n")
		assert.Equal(t, 5, len(lines))
		// Should show from line 7 to 11
		assert.Contains(t, rendered, "C")
		assert.Contains(t, rendered, "D")
		
		// Try setting offset beyond max - should be clamped
		l.offset = 20
		rendered = l.renderVirtualScrolling()
		lines = strings.Split(rendered, "\n")
		assert.Equal(t, 5, len(lines))
		// Should still show the same content as offset=7
		assert.Contains(t, rendered, "C")
		assert.Contains(t, rendered, "D")
	})
}

// testItem is a simple implementation of Item for testing
type testItem struct {
	id      string
	content string
}

func (t *testItem) ID() string {
	return t.id
}

func (t *testItem) View() string {
	return t.content
}

func (t *testItem) Selectable() bool {
	return true
}

func (t *testItem) Height() int {
	return lipgloss.Height(t.content)
}

func (t *testItem) Init() tea.Cmd {
	return nil
}

func (t *testItem) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return t, nil
}

func (t *testItem) SetSize(width, height int) tea.Cmd {
	return nil
}

func (t *testItem) GetSize() (int, int) {
	return 0, lipgloss.Height(t.content)
}

func (t *testItem) SetFocused(focused bool) tea.Cmd {
	return nil
}

func (t *testItem) Focused() bool {
	return false
}

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
