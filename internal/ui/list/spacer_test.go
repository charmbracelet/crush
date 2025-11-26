package list

import (
	"strings"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
)

func TestSpacerItem(t *testing.T) {
	spacer := NewSpacerItem("spacer1", 3)

	// Check ID
	if spacer.ID() != "spacer1" {
		t.Errorf("expected ID 'spacer1', got %q", spacer.ID())
	}

	// Check height
	if h := spacer.Height(80); h != 3 {
		t.Errorf("expected height 3, got %d", h)
	}

	// Height should be constant regardless of width
	if h := spacer.Height(20); h != 3 {
		t.Errorf("expected height 3 for width 20, got %d", h)
	}

	// Draw should not produce any visible content
	screen := uv.NewScreenBuffer(20, 3)
	area := uv.Rect(0, 0, 20, 3)
	spacer.Draw(&screen, area)

	output := screen.Render()
	// Should be empty (just spaces)
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			t.Errorf("expected empty spacer output, got: %q", line)
		}
	}
}

func TestSpacerItemInList(t *testing.T) {
	// Create a list with items separated by spacers
	items := []Item{
		NewStringItem("1", "Item 1"),
		NewSpacerItem("spacer1", 1),
		NewStringItem("2", "Item 2"),
		NewSpacerItem("spacer2", 2),
		NewStringItem("3", "Item 3"),
	}

	l := New(items...)
	l.SetSize(20, 10)

	output := l.Render()

	// Should contain all three items
	if !strings.Contains(output, "Item 1") {
		t.Error("expected output to contain 'Item 1'")
	}
	if !strings.Contains(output, "Item 2") {
		t.Error("expected output to contain 'Item 2'")
	}
	if !strings.Contains(output, "Item 3") {
		t.Error("expected output to contain 'Item 3'")
	}

	// Total height should be: 1 (item1) + 1 (spacer1) + 1 (item2) + 2 (spacer2) + 1 (item3) = 6
	expectedHeight := 6
	if l.TotalHeight() != expectedHeight {
		t.Errorf("expected total height %d, got %d", expectedHeight, l.TotalHeight())
	}
}

func TestSpacerItemNavigation(t *testing.T) {
	// Spacers should not be selectable (they're not focusable)
	items := []Item{
		NewStringItem("1", "Item 1"),
		NewSpacerItem("spacer1", 1),
		NewStringItem("2", "Item 2"),
	}

	l := New(items...)
	l.SetSize(20, 10)

	// Select first item
	l.SetSelectedIndex(0)
	if l.SelectedIndex() != 0 {
		t.Errorf("expected selected index 0, got %d", l.SelectedIndex())
	}

	// Can select the spacer (it's a valid item, just not focusable)
	l.SetSelectedIndex(1)
	if l.SelectedIndex() != 1 {
		t.Errorf("expected selected index 1, got %d", l.SelectedIndex())
	}

	// Can select item after spacer
	l.SetSelectedIndex(2)
	if l.SelectedIndex() != 2 {
		t.Errorf("expected selected index 2, got %d", l.SelectedIndex())
	}
}
