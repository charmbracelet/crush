package list

import (
	"strings"
	"testing"
)

func TestListDoesNotEatLastLine(t *testing.T) {
	// Create items that exactly fill the viewport
	items := []Item{
		NewStringItem("1", "Line 1"),
		NewStringItem("2", "Line 2"),
		NewStringItem("3", "Line 3"),
		NewStringItem("4", "Line 4"),
		NewStringItem("5", "Line 5"),
	}

	// Create list with height exactly matching content (5 lines, no gaps)
	l := New(items...)
	l.SetSize(20, 5)

	// Render the list
	output := l.Render()

	// Count actual lines in output
	lines := strings.Split(strings.TrimRight(output, "\r\n"), "\r\n")
	actualLineCount := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			actualLineCount++
		}
	}

	// All 5 items should be visible
	if !strings.Contains(output, "Line 1") {
		t.Error("expected output to contain 'Line 1'")
	}
	if !strings.Contains(output, "Line 2") {
		t.Error("expected output to contain 'Line 2'")
	}
	if !strings.Contains(output, "Line 3") {
		t.Error("expected output to contain 'Line 3'")
	}
	if !strings.Contains(output, "Line 4") {
		t.Error("expected output to contain 'Line 4'")
	}
	if !strings.Contains(output, "Line 5") {
		t.Error("expected output to contain 'Line 5'")
	}

	if actualLineCount != 5 {
		t.Errorf("expected 5 lines with content, got %d", actualLineCount)
	}
}

func TestListWithScrollDoesNotEatLastLine(t *testing.T) {
	// Create more items than viewport height
	items := []Item{
		NewStringItem("1", "Item 1"),
		NewStringItem("2", "Item 2"),
		NewStringItem("3", "Item 3"),
		NewStringItem("4", "Item 4"),
		NewStringItem("5", "Item 5"),
		NewStringItem("6", "Item 6"),
		NewStringItem("7", "Item 7"),
	}

	// Viewport shows 3 items at a time
	l := New(items...)
	l.SetSize(20, 3)

	// Need to render first to build the buffer and calculate total height
	_ = l.Render()

	// Now scroll to bottom
	l.ScrollToBottom()

	output := l.Render()

	t.Logf("Output:\n%s", output)
	t.Logf("Offset: %d, Total height: %d", l.offset, l.TotalHeight())

	// Should show last 3 items: 5, 6, 7
	if !strings.Contains(output, "Item 5") {
		t.Error("expected output to contain 'Item 5'")
	}
	if !strings.Contains(output, "Item 6") {
		t.Error("expected output to contain 'Item 6'")
	}
	if !strings.Contains(output, "Item 7") {
		t.Error("expected output to contain 'Item 7'")
	}

	// Should not show earlier items
	if strings.Contains(output, "Item 1") {
		t.Error("expected output to NOT contain 'Item 1' when scrolled to bottom")
	}
}
