package list

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestFocusableItemLastLineNotEaten(t *testing.T) {
	// Create focusable items with borders
	focusStyle := lipgloss.NewStyle().
		Padding(1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("86"))

	blurStyle := lipgloss.NewStyle().
		BorderForeground(lipgloss.Color("240"))

	items := []Item{
		NewFocusableItem(NewStringItem("1", "Item 1"), &focusStyle, &blurStyle),
		Gap,
		NewFocusableItem(NewStringItem("2", "Item 2"), &focusStyle, &blurStyle),
		Gap,
		NewFocusableItem(NewStringItem("3", "Item 3"), &focusStyle, &blurStyle),
		Gap,
		NewFocusableItem(NewStringItem("4", "Item 4"), &focusStyle, &blurStyle),
		Gap,
		NewFocusableItem(NewStringItem("5", "Item 5"), &focusStyle, &blurStyle),
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
	l.SetSelectedIndex(len(items) - 1)

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
