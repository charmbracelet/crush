package dialog

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
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

// TestMCPToggles_FitsSmallTerminal verifies that long server names are
// truncated instead of wrapping inside the frame, and that the visible rows
// shrink to the screen height while keeping the cursor in view. Previously
// each long row wrapped onto two lines and the 12-row window ran past the
// bottom of a short terminal.
func TestMCPToggles_FitsSmallTerminal(t *testing.T) {
	t.Parallel()

	var items []MCPToggleItem
	for i := range 20 {
		items = append(items, MCPToggleItem{Name: fmt.Sprintf("server-with-a-really-long-name-number-%02d", i), Status: "connected"})
	}
	m := newMCPTogglesForTest(items)
	drawFits(t, m, 40, 12)

	for range 15 {
		m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	drawFits(t, m, 40, 12)
	require.Less(t, m.maxRows, maxVisibleToggleRows)
	require.GreaterOrEqual(t, m.cursor, m.offset, "the cursor row stays visible")
	require.Less(t, m.cursor, m.offset+m.maxRows, "the cursor row stays visible")
}

// TestMCPTogglesLongStatusKeepsName verifies a long error status gives
// way before the server name, so the row still says which server failed.
func TestMCPTogglesLongStatusKeepsName(t *testing.T) {
	t.Parallel()

	m := newMCPTogglesForTest([]MCPToggleItem{
		{Name: "linear", Status: "connected"},
		{Name: "context7", Status: "error: connection refused by the remote host after three attempts"},
	})
	scr := uv.NewScreenBuffer(40, 20)
	m.Draw(scr, scr.Bounds())
	out := scr.String()
	require.Contains(t, out, "context7")
	require.Contains(t, out, "error: conn")
}

func TestFitToggleRow(t *testing.T) {
	t.Parallel()

	name, status := fitToggleRow("linear", "connected", 30)
	require.Equal(t, "linear", name)
	require.Equal(t, "connected", status)

	name, status = fitToggleRow("a-very-long-server-name-indeed", "connected", 30)
	require.Equal(t, "connected", status, "a short status stays whole")
	require.Equal(t, 30-2-1, ansi.StringWidth(name+status))

	name, status = fitToggleRow("context7", strings.Repeat("x", 80), 30)
	require.Equal(t, "context7", name, "a short name stays whole")
	require.Equal(t, 30-2-1, ansi.StringWidth(name+status))

	name, status = fitToggleRow(strings.Repeat("n", 80), strings.Repeat("s", 80), 30)
	require.Equal(t, 13, ansi.StringWidth(status), "a long status keeps only half the row")
	require.Equal(t, 30-2-1, ansi.StringWidth(name+status))
}

// TestMCPTogglesStatusColors verifies each status label shares its dot's
// meaning: errors are not drawn in the connected green.
func TestMCPTogglesStatusColors(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	online := statusTextStyle(&s, "connected").GetForeground()
	require.NotEqual(t, online, statusTextStyle(&s, "error: boom").GetForeground())
	require.Equal(t, s.Resource.ErrorIcon.GetForeground(), statusTextStyle(&s, "error: boom").GetForeground())
	require.Equal(t, s.Resource.NeedsAuthIcon.GetForeground(), statusTextStyle(&s, "needs authentication").GetForeground())
	require.NotEqual(t, online, statusTextStyle(&s, "offline").GetForeground())
	require.Equal(t, "error", ansi.Strip(statusTextStyle(&s, "error").Render("error")), "no glyph leaks into the label")
}
