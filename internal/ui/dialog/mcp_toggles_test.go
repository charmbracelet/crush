package dialog

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func newMCPTogglesForTest(items []MCPToggleItem) *MCPToggles {
	s := styles.CharmtonePantera()
	com := &common.Common{Styles: &s}
	return NewMCPToggles(com, items)
}

func TestMCPToggles_Toggle(t *testing.T) {
	t.Parallel()

	m := newMCPTogglesForTest([]MCPToggleItem{
		{Name: "docker", Status: "connected"},
		{Name: "serena", Disabled: true, Status: "offline"},
		{Name: "frozen", ConfigDisabled: true, Status: "disabled"},
	})

	action := m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	toggled, ok := action.(ActionToggleMCP)
	require.True(t, ok)
	require.Equal(t, "docker", toggled.Name)
	require.True(t, toggled.Disabled, "enter should disable an enabled server")
	require.True(t, m.Items()[0].Disabled)

	action = m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Nil(t, action)

	action = m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	toggled, ok = action.(ActionToggleMCP)
	require.True(t, ok)
	require.Equal(t, "serena", toggled.Name)
	require.False(t, toggled.Disabled, "enter should re-enable a disabled server")
	require.False(t, m.Items()[1].Disabled)
}

func TestMCPToggles_ConfigDisabledCanBeEnabled(t *testing.T) {
	t.Parallel()

	m := newMCPTogglesForTest([]MCPToggleItem{
		{Name: "docker", Status: "connected"},
		{Name: "frozen", ConfigDisabled: true, Status: "disabled"},
	})

	m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	action := m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	toggled, ok := action.(ActionToggleMCP)
	require.True(t, ok)
	require.Equal(t, "frozen", toggled.Name)
	require.False(t, toggled.Disabled, "config-disabled servers must be enable-able")
	require.Equal(t, "starting...", m.Items()[1].Status, "enabling a config-disabled server must show immediate feedback")
}

func TestMCPToggles_NavigationClamps(t *testing.T) {
	t.Parallel()

	m := newMCPTogglesForTest([]MCPToggleItem{
		{Name: "docker", Status: "connected"},
		{Name: "serena", Status: "offline"},
	})

	m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyUp})
	action := m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	toggled, ok := action.(ActionToggleMCP)
	require.True(t, ok)
	require.Equal(t, "docker", toggled.Name, "cursor must clamp at the top")

	m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	action = m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	toggled, ok = action.(ActionToggleMCP)
	require.True(t, ok)
	require.Equal(t, "serena", toggled.Name, "cursor must clamp at the bottom")

	require.IsType(t, ActionClose{}, m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEscape}))
}
