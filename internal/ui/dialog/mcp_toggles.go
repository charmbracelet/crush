package dialog

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
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
	com    *common.Common
	width  int
	items  []MCPToggleItem
	cursor int
	scope  MCPToggleScope
	help   help.Model
	keyMap struct {
		Up     key.Binding
		Down   key.Binding
		Toggle key.Binding
		Scope  key.Binding
		Close  key.Binding
	}
}

var _ Dialog = (*MCPToggles)(nil)

// NewMCPToggles creates a new MCP toggles dialog.
func NewMCPToggles(com *common.Common, items []MCPToggleItem) *MCPToggles {
	t := com.Styles
	m := &MCPToggles{
		com:   com,
		width: 0, // Set dynamically in Draw().
		items: items,
	}

	m.help = help.New()
	m.help.Styles = t.DialogHelpStyles()

	m.keyMap.Up = key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	)
	m.keyMap.Down = key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	)
	m.keyMap.Toggle = key.NewBinding(
		key.WithKeys("enter", " ", "space"),
		key.WithHelp("enter", "toggle"),
	)
	m.keyMap.Scope = key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch scope"),
	)
	m.keyMap.Close = CloseKey

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

// Scope returns the selected toggle scope.
func (m *MCPToggles) Scope() MCPToggleScope {
	return m.scope
}

// HandleMsg implements Dialog.
func (m *MCPToggles) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.Up):
			m.cursor = max(0, m.cursor-1)
		case key.Matches(msg, m.keyMap.Down):
			m.cursor = min(len(m.items)-1, m.cursor+1)
		case key.Matches(msg, m.keyMap.Scope):
			if m.scope == MCPToggleScopeLocal {
				m.scope = MCPToggleScopeGlobal
			} else {
				m.scope = MCPToggleScopeLocal
			}
		case key.Matches(msg, m.keyMap.Toggle):
			if m.cursor < 0 || m.cursor >= len(m.items) {
				return nil
			}
			item := m.items[m.cursor]
			// Toggle based on the effective state for the active scope:
			// local considers the repository overrides, global reads the
			// config's raw disabled flag.
			currentlyDisabled := item.localDisabled()
			if m.scope == MCPToggleScopeGlobal {
				currentlyDisabled = item.ConfigDisabled
			}
			newState := !currentlyDisabled
			if m.scope == MCPToggleScopeGlobal {
				m.items[m.cursor].ConfigDisabled = newState
			} else {
				m.items[m.cursor].Disabled = newState
				// A config-disabled server enabled locally must be started
				// at runtime; surface that immediately instead of waiting
				// for the connection state event.
				if item.ConfigDisabled && !newState {
					m.items[m.cursor].EnabledOverride = true
					m.items[m.cursor].Status = "starting..."
				}
			}
			return ActionToggleMCP{
				Name:     item.Name,
				Disabled: newState,
				Global:   m.scope == MCPToggleScopeGlobal,
			}
		case key.Matches(msg, m.keyMap.Close):
			return ActionClose{}
		}
	}
	return nil
}

// Draw implements Dialog.
func (m *MCPToggles) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := m.com.Styles
	m.width = max(0, min(m.requiredWidth(t), area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	DrawCenter(scr, area, m.dialogContent())
	return nil
}

// requiredWidth returns the width needed to fit the widest row (name, at
// least one space, and status) on a single line, plus row padding and the
// dialog frame. A fixed 64-column cap word-wraps long server names onto a
// second line.
func (m *MCPToggles) requiredWidth(t *styles.Styles) int {
	widest := 48 // Comfortable minimum so short names don't shrink the dialog.
	for _, item := range m.items {
		row := lipgloss.Width(item.Name) + 1 + lipgloss.Width(m.itemStatus(item))
		widest = max(widest, row)
	}
	return widest + 2 /* row padding */ + t.Dialog.View.GetHorizontalFrameSize()
}

func (m *MCPToggles) dialogContent() string {
	t := m.com.Styles
	innerWidth := m.width - t.Dialog.View.GetHorizontalFrameSize()
	rc := NewRenderContext(t, m.width)
	rc.Title = "Toggle MCPs"
	rc.TitleInfo = m.scopeRadioView(t)
	rc.AddPart(m.innerContent())
	rc.Help = renderDialogHelp(t, &m.help, m, innerWidth)
	return rc.Render()
}

// scopeRadioView renders the Local/Global radio selector, mirroring the
// command palette's System/User switch on the title line.
func (m *MCPToggles) scopeRadioView(t *styles.Styles) string {
	radio := func(s MCPToggleScope) string {
		bullet := t.Radio.Off
		if s == m.scope {
			bullet = t.Radio.On
		}
		return bullet.Render() + t.Radio.Label.Padding(0, 1).Render(s.String())
	}
	return " " + radio(MCPToggleScopeLocal) + " " + radio(MCPToggleScopeGlobal)
}

func (m *MCPToggles) innerContent() string {
	t := m.com.Styles
	innerWidth := m.width - t.Dialog.View.GetHorizontalFrameSize()

	if len(m.items) == 0 {
		return t.Dialog.SecondaryText.
			Width(innerWidth).
			Padding(0, 1).
			Render("No MCP servers configured.")
	}

	// The row style adds Padding(0, 1), so the text area is two columns
	// narrower than the dialog's inner width.
	rowWidth := max(0, innerWidth-2)
	rows := make([]string, 0, len(m.items))
	for i, item := range m.items {
		status := m.itemStatus(item)
		gap := max(1, rowWidth-lipgloss.Width(item.Name)-lipgloss.Width(status))

		if i == m.cursor {
			// Render the full row (name + gap + status) through the
			// selection style so the highlight covers both columns.
			rows = append(rows, t.Dialog.SelectedItem.Render(
				item.Name+strings.Repeat(" ", gap)+status,
			))
			continue
		}

		// Enabled servers share the green used by the home-page MCP list.
		// OnlineText (not OnlineIcon) carries no "●" prefix: rendering
		// through the icon style would prepend the icon and push the row
		// over the dialog width.
		statusStyle := t.Resource.OnlineText
		if status == "disabled" {
			// UnsetPadding: SecondaryText carries its own Padding(0, 1),
			// which would widen the row one column past every other row.
			statusStyle = t.Dialog.SecondaryText.UnsetPadding()
		}
		row := t.Dialog.NormalItem.UnsetPadding().Render(item.Name) +
			strings.Repeat(" ", gap) +
			statusStyle.Render(status)
		// Match the selected branch: a single Padding(0, 1) around the row.
		// NormalItem already carries its own padding, so using it to render
		// the name plus an outer padding doubled the insets and pushed the
		// row over the dialog width.
		rows = append(rows, lipgloss.NewStyle().Padding(0, 1).Render(row))
	}

	return lipgloss.JoinVertical(lipgloss.Left, "", strings.Join(rows, "\n"), "")
}

// itemStatus returns the right-hand status label for an item. The live
// connection state speaks for itself: a config-disabled server that was
// runtime-enabled shows "starting..."/"connected", an untouched one shows
// "disabled" via its connection state. Only a repository override (local
// scope) or the config flag (global scope) forces the "disabled" label
// over a live connection.
func (m *MCPToggles) itemStatus(item MCPToggleItem) string {
	if m.scope == MCPToggleScopeGlobal {
		if item.ConfigDisabled {
			return "disabled"
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

// FullHelp implements help.KeyMap.
func (m *MCPToggles) FullHelp() [][]key.Binding {
	return [][]key.Binding{m.ShortHelp()}
}

// ShortHelp implements help.KeyMap.
func (m *MCPToggles) ShortHelp() []key.Binding {
	return []key.Binding{m.keyMap.Up, m.keyMap.Down, m.keyMap.Toggle, m.keyMap.Scope, m.keyMap.Close}
}
