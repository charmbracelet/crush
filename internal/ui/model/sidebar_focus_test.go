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

// TestSidebarFocusEnumExists documents the three-state non-compact cycle.
func TestSidebarFocusEnumExists(t *testing.T) {
	t.Parallel()
	require.NotEqual(t, uiFocusEditor, uiFocusSidebar)
	require.NotEqual(t, uiFocusMain, uiFocusSidebar)
	require.NotEqual(t, uiFocusNone, uiFocusSidebar)
}
