package list

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"
)

func TestNewList(t *testing.T) {
	items := []Item{
		NewStringItem("Item 1"),
		NewStringItem("Item 2"),
		NewStringItem("Item 3"),
	}

	l := New(items...)
	l.SetSize(80, 24)

	if len(l.items) != 3 {
		t.Errorf("expected 3 items, got %d", len(l.items))
	}

	if l.width != 80 || l.height != 24 {
		t.Errorf("expected size 80x24, got %dx%d", l.width, l.height)
	}
}

func TestListDraw(t *testing.T) {
	items := []Item{
		NewStringItem("Item 1"),
		NewStringItem("Item 2"),
		NewStringItem("Item 3"),
	}

	l := New(items...)
	l.SetSize(80, 10)

	// Create a screen buffer to draw into
	screen := uv.NewScreenBuffer(80, 10)
	area := uv.Rect(0, 0, 80, 10)

	// Draw the list
	l.Draw(&screen, area)

	// Verify the buffer has content
	output := screen.Render()
	if len(output) == 0 {
		t.Error("expected non-empty output")
	}
}

func TestListAppendItem(t *testing.T) {
	items := []Item{
		NewStringItem("Item 1"),
	}

	l := New(items...)
	l.AppendItem(NewStringItem("Item 2"))

	if len(l.items) != 2 {
		t.Errorf("expected 2 items after append, got %d", len(l.items))
	}
}

func TestListDeleteItem(t *testing.T) {
	items := []Item{
		NewStringItem("Item 1"),
		NewStringItem("Item 2"),
		NewStringItem("Item 3"),
	}

	l := New(items...)
	l.DeleteItem(2)

	if len(l.items) != 2 {
		t.Errorf("expected 2 items after delete, got %d", len(l.items))
	}
}

func TestListUpdateItem(t *testing.T) {
	items := []Item{
		NewStringItem("Item 1"),
		NewStringItem("Item 2"),
	}

	l := New(items...)
	l.SetSize(80, 10)

	// Update item
	newItem := NewStringItem("Updated Item 2")
	l.UpdateItem(1, newItem)

	if l.items[1].(*StringItem).content != "Updated Item 2" {
		t.Errorf("expected updated content, got '%s'", l.items[1].(*StringItem).content)
	}
}

func TestListSelection(t *testing.T) {
	items := []Item{
		NewStringItem("Item 1"),
		NewStringItem("Item 2"),
		NewStringItem("Item 3"),
	}

	l := New(items...)
	l.SetSelected(0)

	if l.SelectedIndex() != 0 {
		t.Errorf("expected selected index 0, got %d", l.SelectedIndex())
	}

	l.SelectNext()
	if l.SelectedIndex() != 1 {
		t.Errorf("expected selected index 1 after SelectNext, got %d", l.SelectedIndex())
	}

	l.SelectPrev()
	if l.SelectedIndex() != 0 {
		t.Errorf("expected selected index 0 after SelectPrev, got %d", l.SelectedIndex())
	}
}

func TestListScrolling(t *testing.T) {
	items := []Item{
		NewStringItem("Item 1"),
		NewStringItem("Item 2"),
		NewStringItem("Item 3"),
		NewStringItem("Item 4"),
		NewStringItem("Item 5"),
	}

	l := New(items...)
	l.SetSize(80, 2) // Small viewport

	// Draw to initialize the master buffer
	screen := uv.NewScreenBuffer(80, 2)
	area := uv.Rect(0, 0, 80, 2)
	l.Draw(&screen, area)

	if l.Offset() != 0 {
		t.Errorf("expected initial offset 0, got %d", l.Offset())
	}

	l.ScrollBy(2)
	if l.Offset() != 2 {
		t.Errorf("expected offset 2 after ScrollBy(2), got %d", l.Offset())
	}

	l.ScrollToTop()
	if l.Offset() != 0 {
		t.Errorf("expected offset 0 after ScrollToTop, got %d", l.Offset())
	}
}

// FocusableTestItem is a test item that implements Focusable.
type FocusableTestItem struct {
	id      string
	content string
	focused bool
}

func (f *FocusableTestItem) ID() string {
	return f.id
}

func (f *FocusableTestItem) Height(width int) int {
	return 1
}

func (f *FocusableTestItem) Draw(scr uv.Screen, area uv.Rectangle) {
	prefix := "[ ]"
	if f.focused {
		prefix = "[X]"
	}
	content := prefix + " " + f.content
	styled := uv.NewStyledString(content)
	styled.Draw(scr, area)
}

func (f *FocusableTestItem) Focus() {
	f.focused = true
}

func (f *FocusableTestItem) Blur() {
	f.focused = false
}

func (f *FocusableTestItem) IsFocused() bool {
	return f.focused
}

func TestListFocus(t *testing.T) {
	items := []Item{
		&FocusableTestItem{id: "1", content: "Item 1"},
		&FocusableTestItem{id: "2", content: "Item 2"},
	}

	l := New(items...)
	l.SetSize(80, 10)
	l.SetSelected(0)

	// Focus the list
	l.Focus()

	if !l.Focused() {
		t.Error("expected list to be focused")
	}

	// Check if selected item is focused
	selectedItem := l.SelectedItem().(*FocusableTestItem)
	if !selectedItem.IsFocused() {
		t.Error("expected selected item to be focused")
	}

	// Select next and check focus changes
	l.SelectNext()
	if selectedItem.IsFocused() {
		t.Error("expected previous item to be blurred")
	}

	newSelectedItem := l.SelectedItem().(*FocusableTestItem)
	if !newSelectedItem.IsFocused() {
		t.Error("expected new selected item to be focused")
	}

	// Blur the list
	l.Blur()
	if l.Focused() {
		t.Error("expected list to be blurred")
	}
}

// TestFocusNavigationAfterAppendingToViewportHeight reproduces the bug:
// Append items until viewport is full, select last, then navigate backwards.
func TestFocusNavigationAfterAppendingToViewportHeight(t *testing.T) {
	t.Parallel()

	focusStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("86"))

	blurStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240"))

	// Start with one item
	items := []Item{
		NewStringItem("Item 1").WithFocusStyles(&focusStyle, &blurStyle),
	}

	l := New(items...)
	l.SetSize(20, 15) // 15 lines viewport height
	l.SetSelected(0)
	l.Focus()

	// Initial draw to build buffer
	screen := uv.NewScreenBuffer(20, 15)
	l.Draw(&screen, uv.Rect(0, 0, 20, 15))

	// Append items until we exceed viewport height
	// Each focusable item with border is 5 lines tall
	for i := 2; i <= 4; i++ {
		item := NewStringItem("Item "+string(rune('0'+i))).WithFocusStyles(&focusStyle, &blurStyle)
		l.AppendItem(item)
	}

	// Select the last item
	l.SetSelected(3)

	// Draw
	screen = uv.NewScreenBuffer(20, 15)
	l.Draw(&screen, uv.Rect(0, 0, 20, 15))
	output := screen.Render()

	t.Logf("After selecting last item:\n%s", output)
	require.Contains(t, output, "38;5;86", "expected focus color on last item")

	// Now navigate backwards
	l.SelectPrev()

	screen = uv.NewScreenBuffer(20, 15)
	l.Draw(&screen, uv.Rect(0, 0, 20, 15))
	output = screen.Render()

	t.Logf("After SelectPrev:\n%s", output)
	require.Contains(t, output, "38;5;86", "expected focus color after SelectPrev")

	// Navigate backwards again
	l.SelectPrev()

	screen = uv.NewScreenBuffer(20, 15)
	l.Draw(&screen, uv.Rect(0, 0, 20, 15))
	output = screen.Render()

	t.Logf("After second SelectPrev:\n%s", output)
	require.Contains(t, output, "38;5;86", "expected focus color after second SelectPrev")
}

func TestFocusableItemUpdate(t *testing.T) {
	// Create styles with borders
	focusStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("86"))

	blurStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240"))

	// Create a focusable item
	item := NewStringItem("Test Item").WithFocusStyles(&focusStyle, &blurStyle)

	// Initially not focused - render with blur style
	screen1 := uv.NewScreenBuffer(20, 5)
	area := uv.Rect(0, 0, 20, 5)
	item.Draw(&screen1, area)
	output1 := screen1.Render()

	// Focus the item
	item.Focus()

	// Render again - should show focus style
	screen2 := uv.NewScreenBuffer(20, 5)
	item.Draw(&screen2, area)
	output2 := screen2.Render()

	// Outputs should be different (different border colors)
	if output1 == output2 {
		t.Error("expected different output after focusing, but got same output")
	}

	// Verify focus state
	if !item.IsFocused() {
		t.Error("expected item to be focused")
	}

	// Blur the item
	item.Blur()

	// Render again - should show blur style again
	screen3 := uv.NewScreenBuffer(20, 5)
	item.Draw(&screen3, area)
	output3 := screen3.Render()

	// Output should match original blur output
	if output1 != output3 {
		t.Error("expected same output after blurring as initial state")
	}

	// Verify blur state
	if item.IsFocused() {
		t.Error("expected item to be blurred")
	}
}

func TestFocusableItemHeightWithBorder(t *testing.T) {
	// Create a style with a border (adds 2 to vertical height)
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder())

	// Item without styles has height 1
	plainItem := NewStringItem("Test")
	plainHeight := plainItem.Height(20)
	if plainHeight != 1 {
		t.Errorf("expected plain height 1, got %d", plainHeight)
	}

	// Item with border should add border height (2 lines)
	item := NewStringItem("Test").WithFocusStyles(&borderStyle, &borderStyle)
	itemHeight := item.Height(20)
	expectedHeight := 1 + 2 // content + border
	if itemHeight != expectedHeight {
		t.Errorf("expected height %d (content 1 + border 2), got %d",
			expectedHeight, itemHeight)
	}
}

func TestFocusableItemInList(t *testing.T) {
	focusStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("86"))

	blurStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240"))

	// Create list with focusable items
	items := []Item{
		NewStringItem("Item 1").WithFocusStyles(&focusStyle, &blurStyle),
		NewStringItem("Item 2").WithFocusStyles(&focusStyle, &blurStyle),
		NewStringItem("Item 3").WithFocusStyles(&focusStyle, &blurStyle),
	}

	l := New(items...)
	l.SetSize(80, 20)
	l.SetSelected(0)

	// Focus the list
	l.Focus()

	// First item should be focused
	firstItem := items[0].(*StringItem)
	if !firstItem.IsFocused() {
		t.Error("expected first item to be focused after focusing list")
	}

	// Render to ensure changes are visible
	output1 := l.Render()
	if !strings.Contains(output1, "Item 1") {
		t.Error("expected output to contain first item")
	}

	// Select second item
	l.SetSelected(1)

	// First item should be blurred, second focused
	if firstItem.IsFocused() {
		t.Error("expected first item to be blurred after changing selection")
	}

	secondItem := items[1].(*StringItem)
	if !secondItem.IsFocused() {
		t.Error("expected second item to be focused after selection")
	}

	// Render again - should show updated focus
	output2 := l.Render()
	if !strings.Contains(output2, "Item 2") {
		t.Error("expected output to contain second item")
	}

	// Outputs should be different
	if output1 == output2 {
		t.Error("expected different output after selection change")
	}
}

func TestFocusableItemWithNilStyles(t *testing.T) {
	// Test with nil styles - should render inner item directly
	item := NewStringItem("Plain Item").WithFocusStyles(nil, nil)

	// Height should be based on content (no border since styles are nil)
	itemHeight := item.Height(20)
	if itemHeight != 1 {
		t.Errorf("expected height 1 (no border), got %d", itemHeight)
	}

	// Draw should work without styles
	screen := uv.NewScreenBuffer(20, 5)
	area := uv.Rect(0, 0, 20, 5)
	item.Draw(&screen, area)
	output := screen.Render()

	// Should contain the inner content
	if !strings.Contains(output, "Plain Item") {
		t.Error("expected output to contain inner item content")
	}

	// Focus/blur should still work but not change appearance
	item.Focus()
	screen2 := uv.NewScreenBuffer(20, 5)
	item.Draw(&screen2, area)
	output2 := screen2.Render()

	// Output should be identical since no styles
	if output != output2 {
		t.Error("expected same output with nil styles whether focused or not")
	}

	if !item.IsFocused() {
		t.Error("expected item to be focused")
	}
}

func TestFocusableItemWithOnlyFocusStyle(t *testing.T) {
	// Test with only focus style (blur is nil)
	focusStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("86"))

	item := NewStringItem("Test").WithFocusStyles(&focusStyle, nil)

	// When not focused, should use nil blur style (no border)
	screen1 := uv.NewScreenBuffer(20, 5)
	area := uv.Rect(0, 0, 20, 5)
	item.Draw(&screen1, area)
	output1 := screen1.Render()

	// Focus the item
	item.Focus()
	screen2 := uv.NewScreenBuffer(20, 5)
	item.Draw(&screen2, area)
	output2 := screen2.Render()

	// Outputs should be different (focused has border, blurred doesn't)
	if output1 == output2 {
		t.Error("expected different output when only focus style is set")
	}
}

func TestFocusableItemLastLineNotEaten(t *testing.T) {
	// Create focusable items with borders
	focusStyle := lipgloss.NewStyle().
		Padding(1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("86"))

	blurStyle := lipgloss.NewStyle().
		BorderForeground(lipgloss.Color("240"))

	items := []Item{
		NewStringItem("Item 1").WithFocusStyles(&focusStyle, &blurStyle),
		Gap,
		NewStringItem("Item 2").WithFocusStyles(&focusStyle, &blurStyle),
		Gap,
		NewStringItem("Item 3").WithFocusStyles(&focusStyle, &blurStyle),
		Gap,
		NewStringItem("Item 4").WithFocusStyles(&focusStyle, &blurStyle),
		Gap,
		NewStringItem("Item 5").WithFocusStyles(&focusStyle, &blurStyle),
	}

	// Items with padding(1) and border are 5 lines each
	// Viewport of 10 lines fits exactly 2 items
	l := New()
	l.SetSize(20, 10)

	for _, item := range items {
		l.AppendItem(item)
	}

	// Focus the list
	l.Focus()

	// Select last item
	l.SetSelected(len(items) - 1)

	// Scroll to bottom
	l.ScrollToBottom()

	output := l.Render()

	t.Logf("Output:\n%s", output)
	t.Logf("Offset: %d, Total height: %d", l.offset, l.TotalHeight())

	// Select previous - will skip gaps and go to Item 4
	l.SelectPrev()

	output = l.Render()

	t.Logf("Output:\n%s", output)
	t.Logf("Offset: %d, Total height: %d", l.offset, l.TotalHeight())

	// Should show items 3 (unfocused), 4 (focused), and part of 5 (unfocused)
	if !strings.Contains(output, "Item 3") {
		t.Error("expected output to contain 'Item 3'")
	}
	if !strings.Contains(output, "Item 4") {
		t.Error("expected output to contain 'Item 4'")
	}
	if !strings.Contains(output, "Item 5") {
		t.Error("expected output to contain 'Item 5'")
	}

	// Count bottom borders - should have 1 (focused item 4)
	bottomBorderCount := 0
	for _, line := range strings.Split(output, "\r\n") {
		if strings.Contains(line, "╰") || strings.Contains(line, "└") {
			bottomBorderCount++
		}
	}

	if bottomBorderCount != 1 {
		t.Errorf("expected 1 bottom border (focused item 4), got %d", bottomBorderCount)
	}
}
