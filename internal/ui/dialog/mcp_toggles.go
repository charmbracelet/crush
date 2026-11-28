package dialog

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// MCPTogglesID is the identifier for the MCP toggles dialog.
const MCPTogglesID = "mcp_toggles"

// MCPToggleItem describes one configured MCP server in the toggles dialog.
type MCPToggleItem struct {
	Name string
	// Disabled is the repository-scoped override: when true the server's
	// tools are hidden from every session in this repository.
	Disabled bool
	// ConfigDisabled is the server's disabled flag in the config, before
	// any repository-scoped override. The Global scope reads and writes
	// this flag.
	ConfigDisabled bool
	// EnabledOverride is the repository-scoped enabled override: a config
	// server enabled locally for this repository. Only the Local scope
	// considers it.
	EnabledOverride bool
	// Status is the human-readable connection status.
	Status string
}

// localDisabled returns the effective local state: a config-disabled
// server stays disabled locally unless the repository enabled override
// turned it on.
func (i MCPToggleItem) localDisabled() bool {
	return i.Disabled || (i.ConfigDisabled && !i.EnabledOverride)
}

// ActionToggleMCP is sent when the user toggles an MCP server. Local
// toggles persist a repository-scoped override; global toggles write the
// disabled flag to the global config.
type ActionToggleMCP struct {
	Name     string
	Disabled bool
	Global   bool
}

// MCPToggleScope selects which store a toggle affects.
type MCPToggleScope int

const (
	// MCPToggleScopeLocal persists repository-scoped overrides.
	MCPToggleScopeLocal MCPToggleScope = iota
	// MCPToggleScopeGlobal writes the disabled flag to the config.
	MCPToggleScopeGlobal
)

// String returns the radio label for the scope.
func (s MCPToggleScope) String() string {
	if s == MCPToggleScopeGlobal {
		return "Global"
	}
	return "Local"
}

// MCPToggles lets the user enable and disable MCP servers, either for
// the current repository (Local, the default) or in the config (Global).
type MCPToggles struct {
	toggleDialog
	items []MCPToggleItem
}

var _ Dialog = (*MCPToggles)(nil)

// NewMCPToggles creates a new MCP toggles dialog.
func NewMCPToggles(com *common.Common, items []MCPToggleItem) *MCPToggles {
	m := &MCPToggles{items: items}
	m.toggleDialog = newToggleDialog(com, m, "Toggle MCPs", "No MCP servers configured.")
	return m
}

// ID implements Dialog.
func (m *MCPToggles) ID() string {
	return MCPTogglesID
}

// Items returns the current items.
func (m *MCPToggles) Items() []MCPToggleItem {
	return m.items
}

func (m *MCPToggles) toggleRows(scope MCPToggleScope) []toggleRow {
	t := m.com.Styles
	rows := make([]toggleRow, len(m.items))
	for i, item := range m.items {
		status := m.itemStatus(item)
		// The dot mirrors the sidebar's status icons.
		rows[i] = toggleRow{
			mark:        statusDot(t, status).Render(),
			plainMark:   "●",
			name:        item.Name,
			status:      status,
			statusStyle: statusTextStyle(t, status),
		}
	}
	return rows
}

func (m *MCPToggles) toggleAt(i int, scope MCPToggleScope) Action {
	item := m.items[i]
	// Local reads the repository overrides, global the config flag.
	currentlyDisabled := item.localDisabled()
	if scope == MCPToggleScopeGlobal {
		currentlyDisabled = item.ConfigDisabled
	}
	newState := !currentlyDisabled
	if scope == MCPToggleScopeGlobal {
		m.items[i].ConfigDisabled = newState
	} else {
		m.items[i].Disabled = newState
		// A server enabled locally starts at runtime; show it right away.
		if !newState {
			m.items[i].EnabledOverride = true
			m.items[i].Status = "starting"
		}
	}
	return ActionToggleMCP{
		Name:     item.Name,
		Disabled: newState,
		Global:   scope == MCPToggleScopeGlobal,
	}
}

// statusDot maps a status label to the sidebar's status icon style.
func statusDot(t *styles.Styles, status string) lipgloss.Style {
	switch {
	case status == "connected" || status == "active":
		return t.Resource.OnlineIcon
	case status == "starting":
		return t.Resource.BusyIcon
	case status == "error" || strings.HasPrefix(status, "error:"):
		return t.Resource.ErrorIcon
	case status == "needs authentication":
		return t.Resource.NeedsAuthIcon
	default:
		// disabled, offline
		return t.Resource.DisabledIcon
	}
}

// statusTextStyle colors a status label to match its dot, so an error
// reads as an error rather than in the connected green.
func statusTextStyle(t *styles.Styles, status string) lipgloss.Style {
	switch {
	case status == "error" || strings.HasPrefix(status, "error:"):
		return t.Resource.ErrorIcon.UnsetString()
	case status == "needs authentication":
		return t.Resource.NeedsAuthIcon.UnsetString()
	case status == "disabled" || status == "starting" || status == "offline":
		// UnsetPadding: SecondaryText carries its own Padding(0, 1),
		// which would widen the row one column past every other row.
		return t.Dialog.SecondaryText.UnsetPadding()
	default:
		return t.Resource.OnlineText
	}
}

// itemStatus returns the right-hand status label for an item. The live
// connection state speaks for itself: a config-disabled server that was
// runtime-enabled shows "starting"/"connected", an untouched one shows
// "disabled" via its connection state. Only a repository override (local
// scope) or the config flag (global scope) forces the "disabled" label
// over a live connection.
func (m *MCPToggles) itemStatus(item MCPToggleItem) string {
	if m.scope == MCPToggleScopeGlobal {
		if item.ConfigDisabled {
			return "disabled"
		}
		// The live connection reflects the local override, which takes
		// precedence; the global setting itself is still enabled.
		if item.localDisabled() {
			return "enabled"
		}
		return item.Status
	}
	if item.localDisabled() {
		return "disabled"
	}
	return item.Status
}

// SetItemStatus refreshes one item's live connection status without
// touching its Disabled override, so an open dialog updates as servers
// finish connecting.
func (m *MCPToggles) SetItemStatus(name, status string) {
	for i, item := range m.items {
		if item.Name == name {
			m.items[i].Status = status
			return
		}
	}
}
