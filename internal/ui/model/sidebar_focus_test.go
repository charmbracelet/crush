package model

import (
	"testing"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"
)

func newSidebarTestUI() *UI {
	s := styles.CharmtonePantera()
	com := &common.Common{Styles: &s}
	return &UI{
		com:      com,
		state:    uiChat,
		focus:    uiFocusEditor,
		width:    140,
		dialog:   dialog.NewOverlay(),
		chat:     NewChat(com, config.ScrollbarDefault),
		textarea: textarea.New(),
	}
}

// TestTabSidebarGoesToEditor exercises the real handleKeyPressMsg to verify
// that Tab from the sidebar returns focus to the editor.
func TestTabSidebarGoesToEditor(t *testing.T) {
	t.Parallel()
	m := newSidebarTestUI()
	m.focus = uiFocusSidebar
	m.keyMap.Tab = key.NewBinding(key.WithKeys("tab"))

	m.handleKeyPressMsg(tea.KeyPressMsg{Code: tea.KeyTab, Text: "tab"})
	require.Equal(t, uiFocusEditor, m.focus)
}

func TestSidebarScrollClampsAtZero(t *testing.T) {
	t.Parallel()
	m := newSidebarTestUI()
	m.focus = uiFocusSidebar
	m.keyMap.Chat.Up = key.NewBinding(key.WithKeys("up"))
	m.sidebarScroll = 0

	m.handleKeyPressMsg(tea.KeyPressMsg{Code: tea.KeyUp})
	require.Equal(t, 0, m.sidebarScroll, "scroll should not go below 0")
}

func TestSidebarScrollIncrements(t *testing.T) {
	t.Parallel()
	m := newSidebarTestUI()
	m.focus = uiFocusSidebar
	m.keyMap.Chat.Down = key.NewBinding(key.WithKeys("down"))

	m.handleKeyPressMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Greater(t, m.sidebarScroll, 0, "scroll should increment on down")
}

// TestSidebarMouseWheelScrollsInChatDirection verifies the mouse wheel over
// the sidebar scrolls in the same direction as the chat panel and the Down
// key: wheel-down (DeltaY>0) increases the scroll offset (shows lower content).
func TestSidebarMouseWheelScrollsInChatDirection(t *testing.T) {
	t.Parallel()
	m := newSidebarTestUI()
	m.layout.sidebar = uv.Rect(0, 0, 40, 50) // sidebar spans x in [0,40)
	m.sidebarScroll = 5

	// Wheel down over the sidebar → scroll down (offset increases).
	handled := m.scrollSidebarOnWheel(common.CoalescedWheelMsg{Mouse: tea.Mouse{X: 10}, DeltaY: 3})
	require.True(t, handled, "wheel over the sidebar should be handled by the sidebar")
	require.Equal(t, 8, m.sidebarScroll, "wheel down should scroll the sidebar down")

	// Wheel up over the sidebar → scroll up (offset decreases).
	m.scrollSidebarOnWheel(common.CoalescedWheelMsg{Mouse: tea.Mouse{X: 10}, DeltaY: -2})
	require.Equal(t, 6, m.sidebarScroll, "wheel up should scroll the sidebar up")

	// Wheel outside the sidebar's x-range is not handled here.
	handled = m.scrollSidebarOnWheel(common.CoalescedWheelMsg{Mouse: tea.Mouse{X: 100}, DeltaY: 3})
	require.False(t, handled, "wheel outside the sidebar should not be handled by it")
}

// TestSidebarFocusEnumExists documents the three-state non-compact cycle.
func TestSidebarFocusEnumExists(t *testing.T) {
	t.Parallel()
	require.NotEqual(t, uiFocusEditor, uiFocusSidebar)
	require.NotEqual(t, uiFocusMain, uiFocusSidebar)
	require.NotEqual(t, uiFocusNone, uiFocusSidebar)
}
