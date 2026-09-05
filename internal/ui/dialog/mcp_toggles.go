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
	// ConfigDisabled is true when the server is disabled in the config
	// (global or local) without a repository-scoped enabled override.
	// Toggling it on starts the server at runtime and persists an enabled
	// override.
	ConfigDisabled bool
	// Status is the human-readable connection status.
	Status string
}

// ActionToggleMCP is sent when the user toggles an MCP server for the
// current repository. The model persists the new state.
type ActionToggleMCP struct {
	Name     string
	Disabled bool
}

// MCPToggles lets the user enable and disable MCP servers for the current
// repository. The overrides are repository-scoped and never written to
// config.
type MCPToggles struct {
	com    *common.Common
	width  int
	items  []MCPToggleItem
	cursor int
	help   help.Model
	keyMap struct {
		Up     key.Binding
		Down   key.Binding
		Toggle key.Binding
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

// HandleMsg implements Dialog.
func (m *MCPToggles) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.Up):
			m.cursor = max(0, m.cursor-1)
		case key.Matches(msg, m.keyMap.Down):
			m.cursor = min(len(m.items)-1, m.cursor+1)
		case key.Matches(msg, m.keyMap.Toggle):
			if m.cursor < 0 || m.cursor >= len(m.items) {
				return nil
			}
			item := m.items[m.cursor]
			// Toggle based on the effective state: a config-disabled
			// server starts disabled even without a repository override.
			if item.ConfigDisabled || item.Disabled {
				m.items[m.cursor].Disabled = false
			} else {
				m.items[m.cursor].Disabled = true
			}
			// A config-disabled server must be started at runtime when
			// enabled; surface that immediately instead of waiting for
			// the connection state event.
			if item.ConfigDisabled && !m.items[m.cursor].Disabled {
				m.items[m.cursor].Status = "starting..."
			}
			return ActionToggleMCP{
				Name:     item.Name,
				Disabled: m.items[m.cursor].Disabled,
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
	dialogStyle := t.Dialog.View.Width(m.width)
	view := dialogStyle.Render(m.dialogContent())
	DrawCenter(scr, area, view)
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
	elements := []string{
		m.headerContent(),
		m.innerContent(),
		renderDialogHelp(t, &m.help, m, innerWidth),
	}
	return strings.Join(elements, "\n")
}

func (m *MCPToggles) headerContent() string {
	t := m.com.Styles
	titleStyle := t.Dialog.Title
	dialogStyle := t.Dialog.View.Width(m.width)
	headerOffset := titleStyle.GetHorizontalFrameSize() + dialogStyle.GetHorizontalFrameSize()

	title := "Toggle MCPs"
	return common.DialogTitle(t, titleStyle.Render(title), m.width-headerOffset, t.Dialog.TitleGradFromColor, t.Dialog.TitleGradToColor)
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
// "disabled" via its connection state. Only a repository override forces
// the "disabled" label over a live connection.
func (m *MCPToggles) itemStatus(item MCPToggleItem) string {
	if item.Disabled {
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
	return []key.Binding{m.keyMap.Up, m.keyMap.Down, m.keyMap.Toggle, m.keyMap.Close}
}
