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
	// Lazy is the effective lazy state: the server's tool schemas stay out
	// of the model context until mcp_search loads them.
	Lazy bool
}

// settingLabel names the setting for the row. A disabled server has no
// tools to speak of, and its "disabled" status already reads off the
// connection state, so only the two connected settings carry a label.
func (i MCPToggleItem) settingLabel(scope MCPToggleScope) string {
	if i.disabledIn(scope) {
		return ""
	}
	if i.Lazy {
		return "lazy"
	}
	return "enabled"
}

// disabledIn reports the effective disabled state for a scope: the local
// scope honors the repository overrides, the global scope reads the raw
// config flag.
func (i MCPToggleItem) disabledIn(scope MCPToggleScope) bool {
	if scope == MCPToggleScopeGlobal {
		return i.ConfigDisabled
	}
	return i.localDisabled()
}

// setting derives the current tri-state setting for a scope from the
// effective disabled flag and the lazy policy.
func (i MCPToggleItem) setting(scope MCPToggleScope) MCPServerSetting {
	if i.disabledIn(scope) {
		return MCPServerSettingDisabled
	}
	if i.Lazy {
		return MCPServerSettingLazy
	}
	return MCPServerSettingEnabled
}

// localDisabled returns the effective local state: a config-disabled
// server stays disabled locally unless the repository enabled override
// turned it on.
func (i MCPToggleItem) localDisabled() bool {
	return i.Disabled || (i.ConfigDisabled && !i.EnabledOverride)
}

// MCPServerSetting is one of the three per-server settings offered by the
// dialog. It unifies the connection flag and the lazy-context flag into a
// single choice: Enabled keeps the server connected with its tool schemas
// always in the model context, Lazy Load keeps it connected but serves the
// schemas on demand through mcp_search (the default), and Disabled takes it
// offline entirely.
type MCPServerSetting int

const (
	MCPServerSettingEnabled MCPServerSetting = iota
	MCPServerSettingDisabled
	MCPServerSettingLazy
)

// String returns the setting label shown in dialog rows.
func (s MCPServerSetting) String() string {
	switch s {
	case MCPServerSettingDisabled:
		return "disabled"
	case MCPServerSettingLazy:
		return "lazy"
	default:
		return "enabled"
	}
}

// next returns the setting the enter/space cycle advances to: disabled ->
// lazy -> enabled -> disabled, an escalation of how much of the server the
// model sees.
func (s MCPServerSetting) next() MCPServerSetting {
	switch s {
	case MCPServerSettingDisabled:
		return MCPServerSettingLazy
	case MCPServerSettingLazy:
		return MCPServerSettingEnabled
	default:
		return MCPServerSettingDisabled
	}
}

// ActionSetMCPServerSetting persists a tri-state choice for one server. The
// connection half honors Global (repository override vs config), while the
// lazy half always writes the global config: it only decides what the model
// is shown, never which servers connect.
type ActionSetMCPServerSetting struct {
	Name    string
	Setting MCPServerSetting
	Global  bool
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

// MCPToggles lets the user choose between the three per-server settings
// (Enabled, Disabled, and Lazy Load) either for the current repository
// (Local, the default) or in the config (Global). Enter cycles the active
// row through the settings.
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
		key.WithHelp("enter", "cycle setting"),
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
			next := item.setting(m.scope).next()
			// Apply optimistically so the row flips immediately; the UI
			// model persists the choice and surfaces failures as toasts.
			switch next {
			case MCPServerSettingDisabled:
				if m.scope == MCPToggleScopeGlobal {
					m.items[m.cursor].ConfigDisabled = true
				} else {
					m.items[m.cursor].Disabled = true
					m.items[m.cursor].EnabledOverride = false
				}
			default:
				if m.scope == MCPToggleScopeGlobal {
					m.items[m.cursor].ConfigDisabled = false
				} else {
					m.items[m.cursor].Disabled = false
					// A config-disabled server enabled locally must be
					// started at runtime; surface that immediately instead
					// of waiting for the connection event.
					if item.ConfigDisabled {
						m.items[m.cursor].EnabledOverride = true
						m.items[m.cursor].Status = "starting"
					}
				}
				m.items[m.cursor].Lazy = next == MCPServerSettingLazy
			}
			return ActionSetMCPServerSetting{
				Name:    item.Name,
				Setting: next,
				Global:  m.scope == MCPToggleScopeGlobal,
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

// requiredWidth returns the width needed to fit the widest row (status
// dot, name, at least one space, and status) on a single line, plus row
// padding and the dialog frame. A fixed 64-column cap word-wraps long
// server names onto a second line.
func (m *MCPToggles) requiredWidth(t *styles.Styles) int {
	widest := 48 // Comfortable minimum so short names don't shrink the dialog.
	for _, item := range m.items {
		status := m.itemStatus(item)
		if label := item.settingLabel(m.scope); label != "" {
			status += " · " + label
		}
		row := 2 /* dot + space */ + lipgloss.Width(item.Name) + 1 + lipgloss.Width(status)
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
		// The status dot mirrors the sidebar: green connected, yellow
		// starting, red error, gray disabled/offline. Icon styles carry
		// their own "●" via SetString, so Render() yields just the dot.
		// It sits left of the name, like the sidebar rows.
		dot := statusDot(t, status)
		if label := item.settingLabel(m.scope); label != "" {
			status += " · " + label
		}
		gap := max(1, rowWidth-2 /* dot + space */ -lipgloss.Width(item.Name)-lipgloss.Width(status))

		if i == m.cursor {
			// The full row goes through the selection style in plain
			// text: a styled dot or status inside the content would emit
			// ANSI resets that clear the selection background for the
			// rest of the line, leaving the status unhighlighted.
			rows = append(rows, t.Dialog.SelectedItem.Render(
				"● "+item.Name+strings.Repeat(" ", gap)+status,
			))
			continue
		}

		// OnlineText (not OnlineIcon) is the text style; the dot is
		// rendered separately so no "●" prefix sneaks into the text
		// and pushes the row over the dialog width.
		statusStyle := t.Resource.OnlineText
		if status == "disabled" || status == "starting" {
			// UnsetPadding: SecondaryText carries its own Padding(0, 1),
			// which would widen the row one column past every other row.
			statusStyle = t.Dialog.SecondaryText.UnsetPadding()
		}
		row := dot.Render() + " " +
			t.Dialog.NormalItem.UnsetPadding().Render(item.Name) +
			strings.Repeat(" ", gap) +
			statusStyle.Render(status)
		// A single Padding(0, 1) around the row.
		rows = append(rows, lipgloss.NewStyle().Padding(0, 1).Render(row))
	}

	return lipgloss.JoinVertical(lipgloss.Left, "", strings.Join(rows, "\n"), "")
}

// statusDot maps a status label to the sidebar's status icon style.
func statusDot(t *styles.Styles, status string) lipgloss.Style {
	switch {
	case status == "connected":
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

// SetItemLazy refreshes one item's lazy state without disturbing anything
// else, so an open dialog reflects a config reload or a global flip.
func (m *MCPToggles) SetItemLazy(name string, lazy bool) {
	for i, item := range m.items {
		if item.Name == name {
			m.items[i].Lazy = lazy
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
