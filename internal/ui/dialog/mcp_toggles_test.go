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

// pressEnter advances the active row's setting by one step of the cycle.
func pressEnter(m *MCPToggles) ActionSetMCPServerSetting {
	action := m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	setting, ok := action.(ActionSetMCPServerSetting)
	if !ok {
		panic("enter must emit ActionSetMCPServerSetting")
	}
	return setting
}

func TestMCPToggles_SettingDerivation(t *testing.T) {
	t.Parallel()

	// Lazy is the default: a connected server with the lazy flag set.
	lazy := MCPToggleItem{Name: "docker", Status: "connected", Lazy: true}
	require.Equal(t, MCPServerSettingLazy, lazy.setting(MCPToggleScopeLocal))

	// Connected and not lazy reads as fully enabled.
	enabled := MCPToggleItem{Name: "docker", Status: "connected"}
	require.Equal(t, MCPServerSettingEnabled, enabled.setting(MCPToggleScopeLocal))

	// A repository override, or a config flag with no override, reads as disabled.
	off := MCPToggleItem{Name: "docker", Status: "offline", Disabled: true}
	require.Equal(t, MCPServerSettingDisabled, off.setting(MCPToggleScopeLocal))

	// A config-disabled server that was locally re-enabled is connected in
	// the local scope but still disabled in the global scope.
	frozen := MCPToggleItem{Name: "docker", ConfigDisabled: true, EnabledOverride: true, Status: "connected"}
	require.Equal(t, MCPServerSettingEnabled, frozen.setting(MCPToggleScopeLocal))
	require.Equal(t, MCPServerSettingDisabled, frozen.setting(MCPToggleScopeGlobal))
}

func TestMCPToggles_CycleOrder(t *testing.T) {
	t.Parallel()

	require.Equal(t, MCPServerSettingLazy, MCPServerSettingDisabled.next())
	require.Equal(t, MCPServerSettingEnabled, MCPServerSettingLazy.next())
	require.Equal(t, MCPServerSettingDisabled, MCPServerSettingEnabled.next())
}

func TestMCPToggles_CycleWalksAllThreeSettings(t *testing.T) {
	t.Parallel()

	// A server that starts lazy cycles lazy -> enabled -> disabled -> lazy,
	// and each step carries the matching connection and lazy writes.
	m := newMCPTogglesForTest([]MCPToggleItem{
		{Name: "docker", Status: "connected", Lazy: true},
	})

	setting := pressEnter(m)
	require.Equal(t, "docker", setting.Name)
	require.Equal(t, MCPServerSettingEnabled, setting.Setting)
	require.False(t, m.Items()[0].Lazy, "enabled clears the lazy flag")
	require.False(t, m.Items()[0].Disabled)

	setting = pressEnter(m)
	require.Equal(t, MCPServerSettingDisabled, setting.Setting)
	require.True(t, m.Items()[0].Disabled)
	require.False(t, setting.Global)

	setting = pressEnter(m)
	require.Equal(t, MCPServerSettingLazy, setting.Setting)
	require.False(t, m.Items()[0].Disabled, "lazy re-enables the connection")
	require.True(t, m.Items()[0].Lazy)
}

func TestMCPToggles_ConfigDisabledStartsOnReEnable(t *testing.T) {
	t.Parallel()

	m := newMCPTogglesForTest([]MCPToggleItem{
		{Name: "frozen", ConfigDisabled: true, Status: "disabled"},
	})

	// A config-disabled server is disabled; cycling re-enables it for the
	// repository and must show immediate starting feedback.
	setting := pressEnter(m)
	require.Equal(t, "frozen", setting.Name)
	require.Equal(t, MCPServerSettingLazy, setting.Setting)
	require.Equal(t, "starting", m.Items()[0].Status, "enabling a config-disabled server must show immediate feedback")
	require.True(t, m.Items()[0].EnabledOverride)
}

func TestMCPToggles_ScopeSwitchAndGlobalSetting(t *testing.T) {
	t.Parallel()

	m := newMCPTogglesForTest([]MCPToggleItem{
		{Name: "docker", Status: "connected", Lazy: true},
	})
	require.Equal(t, MCPToggleScopeLocal, m.Scope(), "local must be the default scope")

	// Tab cycles to global.
	require.Nil(t, m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyTab}))
	require.Equal(t, MCPToggleScopeGlobal, m.Scope())

	// From lazy, the cycle advances to enabled, and the row flips the
	// config flag optimistically when the global scope is active.
	setting := pressEnter(m)
	require.Equal(t, "docker", setting.Name)
	require.Equal(t, MCPServerSettingEnabled, setting.Setting)
	require.True(t, setting.Global, "the action must be flagged global when the global scope is selected")
	require.False(t, m.Items()[0].Lazy)

	// Advancing to disabled in the global scope writes the config flag.
	setting = pressEnter(m)
	require.Equal(t, MCPServerSettingDisabled, setting.Setting)
	require.True(t, setting.Global)
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

	// Local scope: the override wins, the server reads as connected.
	require.Equal(t, "connected", m.itemStatus(m.Items()[0]))

	// Global scope: the config still disables it, so it must read disabled
	// even though it is running for this repository.
	m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyTab})
	require.Equal(t, MCPToggleScopeGlobal, m.Scope())
	require.Equal(t, "disabled", m.itemStatus(m.Items()[0]))

	// Advancing the global setting from disabled goes to lazy and clears
	// the config flag; the write is flagged global.
	setting := pressEnter(m)
	require.Equal(t, MCPServerSettingLazy, setting.Setting)
	require.True(t, setting.Global)
	require.False(t, m.Items()[0].ConfigDisabled)
}

func TestMCPToggles_NavigationClamps(t *testing.T) {
	t.Parallel()

	m := newMCPTogglesForTest([]MCPToggleItem{
		{Name: "docker", Status: "connected"},
		{Name: "serena", Status: "offline"},
	})

	m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyUp})
	setting := pressEnter(m)
	require.Equal(t, "docker", setting.Name, "cursor must clamp at the top")

	m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	setting = pressEnter(m)
	require.Equal(t, "serena", setting.Name, "cursor must clamp at the bottom")

	require.IsType(t, ActionClose{}, m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEscape}))
}

func TestMCPToggles_SettingLabel(t *testing.T) {
	t.Parallel()

	connected := MCPToggleScopeLocal
	require.Equal(t, "lazy", MCPToggleItem{Status: "connected", Lazy: true}.settingLabel(connected))
	require.Equal(t, "enabled", MCPToggleItem{Status: "connected"}.settingLabel(connected))
	// A disabled server has no tools to speak of, so it carries no label.
	require.Empty(t, MCPToggleItem{Status: "disabled", ConfigDisabled: true, Lazy: true}.settingLabel(MCPToggleScopeGlobal))
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
