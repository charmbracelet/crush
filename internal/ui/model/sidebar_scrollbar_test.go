package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSidebarContentWidth verifies the sidebar always reserves exactly one
// column for the scrollbar gutter (so the cached full-width logo, rendered at
// this same width, is never clipped) and clamps at zero for tiny widths.
func TestSidebarContentWidth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		sidebarWidth int
		want         int
	}{
		{30, 29},
		{2, 1},
		{1, 0},
		{0, 0},
	}
	for _, tc := range tests {
		require.Equal(t, tc.want, sidebarContentWidth(tc.sidebarWidth),
			"sidebarContentWidth(%d)", tc.sidebarWidth)
	}
}

// TestBlankSidebarColumn verifies the gutter spacer is a single column, height
// rows tall, and empty for non-positive heights.
func TestBlankSidebarColumn(t *testing.T) {
	t.Parallel()

	require.Equal(t, "", blankSidebarColumn(0))
	require.Equal(t, "", blankSidebarColumn(-3))
	require.Equal(t, " ", blankSidebarColumn(1))

	got := blankSidebarColumn(3)
	require.Equal(t, " \n \n ", got, "3 rows of a single-space column")
	require.Equal(t, 3, strings.Count(got, " "), "one space per row")
}

// TestShortHelpTabStability verifies that the Tab key binding description
// in ShortHelp remains stable and focus-neutral, rather than fluctuating,
// and doesn't pollute the global/base key map definitions when focus changes.
func TestShortHelpTabStability(t *testing.T) {
	t.Parallel()
	m := newSidebarTestUI()

	// Capture initial tab help description
	m.focus = uiFocusEditor
	bindsEditor := m.ShortHelp()
	var editorTabHelp string
	for _, b := range bindsEditor {
		if b.Help().Key == "tab" {
			editorTabHelp = b.Help().Desc
			break
		}
	}

	// Change focus to Main
	m.focus = uiFocusMain
	bindsMain := m.ShortHelp()
	var mainTabHelp string
	for _, b := range bindsMain {
		if b.Help().Key == "tab" {
			mainTabHelp = b.Help().Desc
			break
		}
	}

	// Change focus to Sidebar
	m.focus = uiFocusSidebar
	bindsSidebar := m.ShortHelp()
	var sidebarTabHelp string
	for _, b := range bindsSidebar {
		if b.Help().Key == "tab" {
			sidebarTabHelp = b.Help().Desc
			break
		}
	}

	// Validate they are all stable and focus-neutral ("change focus")
	// rather than shifting to "focus chat", "focus sidebar", or "focus editor".
	require.Equal(t, "change focus", editorTabHelp)
	require.Equal(t, "change focus", mainTabHelp)
	require.Equal(t, "change focus", sidebarTabHelp)
}

// TestSidebarUpDownHelpNoStatePollution verifies that customizing UpDown help
// text during sidebar focus is scoped correctly and doesn't permanently pollute
// the base keymap configuration.
func TestSidebarUpDownHelpNoStatePollution(t *testing.T) {
	t.Parallel()
	m := newSidebarTestUI()

	// The base UpDown help ("↑↓", "scroll"); the sidebar footer overrides the
	// key label to "↑/↓". The Desc stays "scroll" either way, so assert on the
	// Key — that's what actually reveals in-place mutation of the shared keymap.
	defaultKey := m.keyMap.Chat.UpDown.Help().Key
	require.Equal(t, "↑↓", defaultKey)

	// Trigger ShortHelp when focused on sidebar (which sets the cloned UpDown's
	// help to "↑/↓" / "scroll").
	m.focus = uiFocusSidebar
	m.ShortHelp()

	// The base keymap must be unchanged — not mutated to the sidebar's "↑/↓".
	require.Equal(t, defaultKey, m.keyMap.Chat.UpDown.Help().Key, "base keymap UpDown key must not be polluted by ShortHelp")
	require.Equal(t, "scroll", m.keyMap.Chat.UpDown.Help().Desc)
}
