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
	require.Equal(t, "starting", m.Items()[1].Status, "a locally re-enabled server is restarted")
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
	require.Equal(t, "starting", m.Items()[1].Status, "enabling a config-disabled server must show immediate feedback")
}

func TestMCPToggles_ScopeSwitchAndGlobalToggle(t *testing.T) {
	t.Parallel()

	m := newMCPTogglesForTest([]MCPToggleItem{
		{Name: "docker", Status: "connected"},
	})
	require.Equal(t, MCPToggleScopeLocal, m.Scope(), "local must be the default scope")

	// Tab cycles to global.
	require.Nil(t, m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyTab}))
	require.Equal(t, MCPToggleScopeGlobal, m.Scope())

	action := m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	toggled, ok := action.(ActionToggleMCP)
	require.True(t, ok)
	require.Equal(t, "docker", toggled.Name)
	require.True(t, toggled.Disabled, "global toggle should disable an enabled server")
	require.True(t, toggled.Global, "toggle must be flagged global when the global scope is selected")
	require.True(t, m.Items()[0].ConfigDisabled, "the config flag must flip optimistically")

	// Tab again returns to local.
	require.Nil(t, m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyTab}))
	require.Equal(t, MCPToggleScopeLocal, m.Scope())
}

func TestMCPToggles_LocalEnableDoesNotAffectGlobalScope(t *testing.T) {
	t.Parallel()

	// A config-disabled server enabled locally for this repository.
	m := newMCPTogglesForTest([]MCPToggleItem{
		{Name: "docker", ConfigDisabled: true, EnabledOverride: true, Status: "connected"},
	})

	// Local scope: the override wins, the server shows enabled.
	require.Equal(t, "connected", m.itemStatus(m.Items()[0]))

	// Global scope: the config still disables it, so it must show
	// disabled even though it is running for this repository.
	m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyTab})
	require.Equal(t, MCPToggleScopeGlobal, m.Scope())
	require.Equal(t, "disabled", m.itemStatus(m.Items()[0]))

	// A global enable flips the config flag and clears the need for a
	// local override display.
	action := m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	toggled, ok := action.(ActionToggleMCP)
	require.True(t, ok)
	require.False(t, toggled.Disabled)
	require.True(t, toggled.Global)
	require.False(t, m.Items()[0].ConfigDisabled)
}

func TestMCPToggles_LocalDisableDoesNotAffectGlobalScope(t *testing.T) {
	t.Parallel()

	m := newMCPTogglesForTest([]MCPToggleItem{
		{Name: "docker", Status: "connected"},
	})

	m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	m.SetItemStatus("docker", "disabled")
	require.Equal(t, "disabled", m.itemStatus(m.Items()[0]))

	m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyTab})
	require.Equal(t, "enabled", m.itemStatus(m.Items()[0]))
	require.False(t, m.Items()[0].ConfigDisabled)
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

func TestMCPToggles_LazyToggle(t *testing.T) {
	t.Parallel()

	m := newMCPTogglesForTest([]MCPToggleItem{
		{Name: "docker", Status: "connected", Lazy: true},
		{Name: "serena", Status: "connected"},
	})

	// Laziness is the default, so the first row starts lazy and the second
	// is pinned; "l" flips either way.
	action := m.HandleMsg(tea.KeyPressMsg{Code: 'l'})
	pinned, ok := action.(ActionToggleMCPLazy)
	require.True(t, ok, "l must emit a lazy toggle")
	require.Equal(t, "docker", pinned.Name)
	require.False(t, pinned.Lazy, "l should pin a lazy server")
	require.False(t, m.Items()[0].Lazy)

	m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	action = m.HandleMsg(tea.KeyPressMsg{Code: 'l'})
	pinned, ok = action.(ActionToggleMCPLazy)
	require.True(t, ok)
	require.Equal(t, "serena", pinned.Name)
	require.True(t, pinned.Lazy, "l should unpin a pinned server")
	require.True(t, m.Items()[1].Lazy)

	// The lazy key must stay distinct from the enable/disable toggle.
	action = m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.IsType(t, ActionToggleMCP{}, action)
}

func TestMCPToggles_LazyLabel(t *testing.T) {
	t.Parallel()

	require.Equal(t, "lazy", MCPToggleItem{Status: "connected", Lazy: true}.lazyLabel())
	require.Equal(t, "pinned", MCPToggleItem{Status: "connected"}.lazyLabel())
	// A disabled server has no tools to hide, so it carries no marker.
	require.Empty(t, MCPToggleItem{Status: "disabled", ConfigDisabled: true, Lazy: true}.lazyLabel())
}

func TestMCPToggles_SetItemLazy(t *testing.T) {
	t.Parallel()

	m := newMCPTogglesForTest([]MCPToggleItem{
		{Name: "docker", Status: "connected", Lazy: true},
	})

	m.SetItemLazy("docker", false)
	require.False(t, m.Items()[0].Lazy)
	require.Equal(t, "connected", m.Items()[0].Status, "status must survive")

	// Unknown names are ignored rather than panicking.
	m.SetItemLazy("nope", true)
	require.Len(t, m.Items(), 1)
}
