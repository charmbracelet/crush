package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSidebarScrollbarLayout verifies the scrollbar gutter is reserved whenever
// content overflows the viewport (independent of focus), keeping the content
// width stable, and that no gutter is reserved when it fits.
func TestSidebarScrollbarLayout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		width         int
		contentHeight int
		viewport      int
		wantWidth     int
		wantScrollbar bool
	}{
		{"fits exactly, no gutter", 30, 10, 10, 30, false},
		{"fits under, no gutter", 30, 5, 10, 30, false},
		{"overflows, reserve gutter", 30, 40, 10, 29, true},
		{"overflows by one, reserve gutter", 30, 11, 10, 29, true},
		{"overflow with zero width clamps", 0, 40, 10, 0, true},
		{"overflow with width one clamps to zero", 1, 40, 10, 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotWidth, gotScrollbar := sidebarScrollbarLayout(tc.width, tc.contentHeight, tc.viewport)
			require.Equal(t, tc.wantWidth, gotWidth, "content width")
			require.Equal(t, tc.wantScrollbar, gotScrollbar, "scrollbar needed")
		})
	}
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
