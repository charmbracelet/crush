package list

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
)

func TestFocusableItemUpdate(t *testing.T) {
	// Create styles with borders
	focusStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("86"))

	blurStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240"))

	// Create a focusable item
	innerItem := NewStringItem("1", "Test Item")
	item := NewFocusableItem(innerItem, &focusStyle, &blurStyle)

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

	innerItem := NewStringItem("1", "Test")
	item := NewFocusableItem(innerItem, &borderStyle, &borderStyle)

	// Inner item has height 1
	innerHeight := innerItem.Height(20)
	if innerHeight != 1 {
		t.Errorf("expected inner height 1, got %d", innerHeight)
	}

	// Focusable item should add border height (2 lines)
	itemHeight := item.Height(20)
	expectedHeight := innerHeight + 2
	if itemHeight != expectedHeight {
		t.Errorf("expected height %d (inner %d + border 2), got %d",
			expectedHeight, innerHeight, itemHeight)
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
		NewFocusableItem(NewStringItem("1", "Item 1"), &focusStyle, &blurStyle),
		NewFocusableItem(NewStringItem("2", "Item 2"), &focusStyle, &blurStyle),
		NewFocusableItem(NewStringItem("3", "Item 3"), &focusStyle, &blurStyle),
	}

	l := New(items...)
	l.SetSize(80, 20)
	l.SetSelectedIndex(0)

	// Focus the list
	l.Focus()

	// First item should be focused
	firstItem := items[0].(*FocusableItem)
	if !firstItem.IsFocused() {
		t.Error("expected first item to be focused after focusing list")
	}

	// Render to ensure changes are visible
	output1 := l.Render()
	if !strings.Contains(output1, "Item 1") {
		t.Error("expected output to contain first item")
	}

	// Select second item
	l.SetSelectedIndex(1)

	// First item should be blurred, second focused
	if firstItem.IsFocused() {
		t.Error("expected first item to be blurred after changing selection")
	}

	secondItem := items[1].(*FocusableItem)
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
	innerItem := NewStringItem("1", "Plain Item")
	item := NewFocusableItem(innerItem, nil, nil)

	// Height should match inner item exactly (no border)
	innerHeight := innerItem.Height(20)
	itemHeight := item.Height(20)
	if itemHeight != innerHeight {
		t.Errorf("expected height %d (same as inner), got %d", innerHeight, itemHeight)
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

	innerItem := NewStringItem("1", "Test")
	item := NewFocusableItem(innerItem, &focusStyle, nil)

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
