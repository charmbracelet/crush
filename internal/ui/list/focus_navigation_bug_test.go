package list

import (
	"testing"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"
)

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
		NewFocusableItem(NewStringItem("1", "Item 1"), &focusStyle, &blurStyle),
	}

	l := New(items...)
	l.SetSize(20, 15) // 15 lines viewport height
	l.SetSelectedIndex(0)
	l.Focus()

	// Initial draw to build buffer
	screen := uv.NewScreenBuffer(20, 15)
	l.Draw(&screen, uv.Rect(0, 0, 20, 15))

	// Append items until we exceed viewport height
	// Each focusable item with border is 5 lines tall
	for i := 2; i <= 4; i++ {
		inner := NewStringItem(string(rune('0'+i)), "Item "+string(rune('0'+i)))
		focusable := NewFocusableItem(inner, &focusStyle, &blurStyle)
		l.AppendItem(focusable)
	}

	// Select the last item
	l.SetSelectedIndex(3)

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
