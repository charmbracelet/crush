package list

import (
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"
)

func TestMouseSelection(t *testing.T) {
	t.Parallel()

	// Create simple string items
	items := []Item{
		NewStringItem("1", "First item"),
		NewStringItem("2", "Second item"),
		NewStringItem("3", "Third item"),
	}

	l := New(items...)
	l.SetSize(20, 10)

	// Click on first item (y=0)
	handled := l.HandleMouseDown(0, 0)
	require.True(t, handled)
	require.Equal(t, 0, l.SelectedIndex())
	require.Equal(t, "1", l.mouseDownItem)

	// Release
	handled = l.HandleMouseUp(5, 0)
	require.True(t, handled)

	// Click on second item (y=1, since each item is 1 line)
	handled = l.HandleMouseDown(0, 1)
	require.True(t, handled)
	require.Equal(t, 1, l.SelectedIndex())
	require.Equal(t, "2", l.mouseDownItem)
}

func TestMouseHighlight(t *testing.T) {
	t.Parallel()

	item := NewStringItem("1", "Hello World")
	items := []Item{item}

	l := New(items...)
	l.SetSize(20, 10)

	// Mouse down at position 2
	handled := l.HandleMouseDown(2, 0)
	require.True(t, handled)

	// Drag to position 7
	handled = l.HandleMouseDrag(7, 0)
	require.True(t, handled)

	// Release
	handled = l.HandleMouseUp(7, 0)
	require.True(t, handled)

	// Check highlight was set (same line, cols 2-7)
	startLine, startCol, endLine, endCol := item.GetHighlight()
	require.Equal(t, 0, startLine)
	require.Equal(t, 2, startCol)
	require.Equal(t, 0, endLine)
	require.Equal(t, 7, endCol)

	// Render to verify highlighting works
	output := l.Render()
	require.NotEmpty(t, output)
}

func TestMouseDragAcrossMultipleLines(t *testing.T) {
	t.Parallel()

	item1 := NewStringItem("1", "First")
	item2 := NewStringItem("2", "Second")

	l := New(item1, item2)
	l.SetSize(20, 10)

	// Mouse down on first item
	handled := l.HandleMouseDown(2, 0)
	require.True(t, handled)
	require.Equal(t, "1", l.mouseDownItem)

	// Drag to second item
	handled = l.HandleMouseDrag(3, 1)
	require.True(t, handled)
	require.Equal(t, "2", l.mouseDragItem)

	// Release
	handled = l.HandleMouseUp(3, 1)
	require.True(t, handled)

	// Both items should now be highlighted
	startLine1, startCol1, endLine1, endCol1 := item1.GetHighlight()
	require.Equal(t, 0, startLine1) // First item: from mouse down Y
	require.Equal(t, 2, startCol1)  // From mouse down X
	require.Equal(t, 0, endLine1)   // To end of first item
	require.Equal(t, 9999, endCol1) // End of line marker

	startLine2, startCol2, endLine2, endCol2 := item2.GetHighlight()
	require.Equal(t, 0, startLine2) // Second item: from start
	require.Equal(t, 0, startCol2)
	require.Equal(t, 0, endLine2) // To mouse drag Y (relative to item2)
	require.Equal(t, 3, endCol2)  // To mouse drag X
}

func TestMouseDragAcrossThreeItems(t *testing.T) {
	t.Parallel()

	item1 := NewStringItem("1", "First")
	item2 := NewStringItem("2", "Second")
	item3 := NewStringItem("3", "Third")

	l := New(item1, item2, item3)
	l.SetSize(20, 10)

	// Mouse down on first item
	l.HandleMouseDown(1, 0)

	// Drag to third item
	l.HandleMouseDrag(4, 2)
	l.HandleMouseUp(4, 2)

	// First item: partial from mouse down to end
	startLine1, _, endLine1, endCol1 := item1.GetHighlight()
	require.Equal(t, 0, startLine1)
	require.Equal(t, 0, endLine1)
	require.Equal(t, 9999, endCol1) // End of line

	// Middle item: fully highlighted
	startLine2, startCol2, endLine2, endCol2 := item2.GetHighlight()
	require.Equal(t, 0, startLine2)
	require.Equal(t, 0, startCol2)
	require.Equal(t, 0, endLine2)
	require.Equal(t, 9999, endCol2) // Full line

	// Last item: from start to mouse position
	startLine3, startCol3, endLine3, endCol3 := item3.GetHighlight()
	require.Equal(t, 0, startLine3)
	require.Equal(t, 0, startCol3)
	require.Equal(t, 0, endLine3)
	require.Equal(t, 4, endCol3)
}

func TestMouseDragBackward(t *testing.T) {
	t.Parallel()

	item1 := NewStringItem("1", "First")
	item2 := NewStringItem("2", "Second")
	item3 := NewStringItem("3", "Third")

	l := New(item1, item2, item3)
	l.SetSize(20, 10)

	// Mouse down on third item
	l.HandleMouseDown(4, 2)

	// Drag backward to first item
	l.HandleMouseDrag(1, 0)
	l.HandleMouseUp(1, 0)

	// Should highlight in same order as forward drag
	// First item: from drag position to end
	startLine1, startCol1, endLine1, endCol1 := item1.GetHighlight()
	require.Equal(t, 0, startLine1)
	require.Equal(t, 1, startCol1) // From drag X
	require.Equal(t, 0, endLine1)
	require.Equal(t, 9999, endCol1)

	// Middle item: fully highlighted
	startLine2, startCol2, endLine2, endCol2 := item2.GetHighlight()
	require.Equal(t, 0, startLine2)
	require.Equal(t, 0, startCol2)
	require.Equal(t, 0, endLine2)
	require.Equal(t, 9999, endCol2)

	// Last item: from start to mouse down position
	startLine3, startCol3, endLine3, endCol3 := item3.GetHighlight()
	require.Equal(t, 0, startLine3)
	require.Equal(t, 0, startCol3)
	require.Equal(t, 0, endLine3)
	require.Equal(t, 4, endCol3) // To down X
}

func TestMouseDragBackwardSingleItem(t *testing.T) {
	t.Parallel()

	item := NewStringItem("1", "Hello World")
	l := New(item)
	l.SetSize(20, 10)

	// Mouse down at col 7
	l.HandleMouseDown(7, 0)

	// Drag backward to col 2
	l.HandleMouseDrag(2, 0)
	l.HandleMouseUp(2, 0)

	// Should highlight cols 2-7 (start to end)
	startLine, startCol, endLine, endCol := item.GetHighlight()
	require.Equal(t, 0, startLine)
	require.Equal(t, 2, startCol) // Drag position
	require.Equal(t, 0, endLine)
	require.Equal(t, 7, endCol) // Down position
}

func TestClearHighlight(t *testing.T) {
	t.Parallel()

	item := NewStringItem("1", "Hello World")
	items := []Item{item}

	l := New(items...)
	l.SetSize(20, 10)

	// Set highlight via mouse
	l.HandleMouseDown(2, 0)
	l.HandleMouseDrag(7, 0)
	l.HandleMouseUp(7, 0)

	// Verify highlight is set
	startLine, _, endLine, _ := item.GetHighlight()
	require.Equal(t, 0, startLine)
	require.Equal(t, 0, endLine)

	// Clear highlight
	l.ClearHighlight()

	// Verify highlight is cleared
	startLine, _, _, _ = item.GetHighlight()
	require.Equal(t, -1, startLine)
}

func TestMouseClickOutsideItems(t *testing.T) {
	t.Parallel()

	items := []Item{
		NewStringItem("1", "Only item"),
	}

	l := New(items...)
	l.SetSize(20, 10)

	// Click way below the items
	handled := l.HandleMouseDown(0, 9)
	require.False(t, handled)

	// Mouse state should not be set
	require.False(t, l.mouseDown)
	require.Equal(t, "", l.mouseDownItem)
}

func TestHighlightableInterface(t *testing.T) {
	t.Parallel()

	item := NewStringItem("1", "Test")

	// Verify StringItem implements Highlightable
	var _ Highlightable = item

	// Set highlight
	item.SetHighlight(0, 1, 0, 3) // Line 0, cols 1-3
	startLine, startCol, endLine, endCol := item.GetHighlight()
	require.Equal(t, 0, startLine)
	require.Equal(t, 1, startCol)
	require.Equal(t, 0, endLine)
	require.Equal(t, 3, endCol)

	// Clear highlight
	item.SetHighlight(-1, -1, -1, -1)
	startLine, _, _, _ = item.GetHighlight()
	require.Equal(t, -1, startLine)
}

func TestMouseWithScrolling(t *testing.T) {
	t.Parallel()

	// Create items taller than viewport
	var items []Item
	for i := 0; i < 20; i++ {
		items = append(items, NewStringItem(string(rune('A'+i)), "Item"))
	}

	l := New(items...)
	l.SetSize(20, 5) // Only 5 lines visible
	l.ensureBuilt()  // Ensure buffer is built before scrolling

	// Scroll down
	l.ScrollBy(10)
	require.Equal(t, 10, l.Offset())

	// Click at viewport y=2 (should translate to buffer y=12)
	handled := l.HandleMouseDown(0, 2)
	require.True(t, handled)

	// Should select item at buffer position 12 (item 12)
	require.Equal(t, 12, l.SelectedIndex())
}

func TestFindItemAtPosition(t *testing.T) {
	t.Parallel()

	items := []Item{
		NewStringItem("1", "Line1"),
		NewStringItem("2", "Line2\nLine3"), // 2 lines tall
		NewStringItem("3", "Line4"),
	}

	l := New(items...)
	l.SetSize(20, 10)
	l.ensureBuilt()

	// Position 0 should be item 1
	id, itemY := l.findItemAtPosition(0)
	require.Equal(t, "1", id)
	require.Equal(t, 0, itemY)

	// Position 1 should be item 2, line 0
	id, itemY = l.findItemAtPosition(1)
	require.Equal(t, "2", id)
	require.Equal(t, 0, itemY)

	// Position 2 should be item 2, line 1
	id, itemY = l.findItemAtPosition(2)
	require.Equal(t, "2", id)
	require.Equal(t, 1, itemY)

	// Position 3 should be item 3
	id, itemY = l.findItemAtPosition(3)
	require.Equal(t, "3", id)
	require.Equal(t, 0, itemY)

	// Out of bounds
	id, itemY = l.findItemAtPosition(100)
	require.Equal(t, "", id)
	require.Equal(t, 0, itemY)
}

func TestHighlightRender(t *testing.T) {
	t.Parallel()

	item := NewStringItem("1", "Hello World")
	item.SetHighlight(0, 0, 0, 5) // Highlight "Hello" on line 0

	items := []Item{item}
	l := New(items...)
	l.SetSize(20, 10)

	// Render with highlight
	screen := uv.NewScreenBuffer(20, 10)
	area := uv.Rect(0, 0, 20, 10)
	l.Draw(&screen, area)

	output := screen.Render()
	require.Contains(t, output, "Hello")
	require.Contains(t, output, "World")
}

func TestGetHighlightedText(t *testing.T) {
	t.Parallel()

	// Test single item highlight
	item1 := NewStringItem("1", "Hello World")
	item1.SetHighlight(0, 0, 0, 5) // Highlight "Hello"

	l := New(item1)
	l.SetSize(20, 10)
	l.ensureBuilt()

	text := l.GetHighlightedText()
	require.Equal(t, "Hello", text)

	// Test multi-line highlight within single item
	item2 := NewStringItem("2", "Line 1\nLine 2\nLine 3")
	item2.SetHighlight(0, 0, 1, 6) // Highlight from start to end of line 2

	l2 := New(item2)
	l2.SetSize(20, 10)
	l2.ensureBuilt()

	text = l2.GetHighlightedText()
	require.Equal(t, "Line 1\nLine 2", text)

	// Test multiple items highlighted
	item3 := NewStringItem("3", "First")
	item4 := NewStringItem("4", "Second")
	item5 := NewStringItem("5", "Third")

	item3.SetHighlight(0, 0, 0, 5) // "First"
	item5.SetHighlight(0, 0, 0, 5) // "Third"

	l3 := New(item3, item4, item5)
	l3.SetSize(20, 10)
	l3.ensureBuilt()

	text = l3.GetHighlightedText()
	require.Equal(t, "First\nThird", text)

	// Test no highlights
	item6 := NewStringItem("6", "No highlight")
	l4 := New(item6)
	l4.SetSize(20, 10)
	l4.ensureBuilt()

	text = l4.GetHighlightedText()
	require.Equal(t, "", text)
}
