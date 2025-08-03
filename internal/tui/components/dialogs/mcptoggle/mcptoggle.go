package mcptoggle

import (
	"fmt"

	"github.com/charmbracelet/bubbles/v2/help"
	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/exp/list"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
)

const (
	MCPToggleDialogID dialogs.DialogID = "mcptoggle"
	defaultWidth      int              = 60
)

type MCPServerDisabledMsg struct {
	ServerName string
}

type MCPToggleDialog interface {
	dialogs.DialogModel
}

type MCPServer struct {
	Name     string
	Config   config.MCPConfig
	Disabled bool
}

type mcpToggleDialogCmp struct {
	width   int
	wWidth  int
	wHeight int

	mcpList list.FilterableList[list.CompletionItem[MCPServer]]
	keyMap  KeyMap
	help    help.Model
	servers []MCPServer
}

type KeyMap struct {
	Up       key.Binding
	Down     key.Binding
	Select   key.Binding
	Close    key.Binding
	ToggleOn key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter", " "),
			key.WithHelp("enter/space", "toggle"),
		),
		Close: key.NewBinding(
			key.WithKeys("esc", "ctrl+c"),
			key.WithHelp("esc", "close"),
		),
		ToggleOn: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "toggle all"),
		),
	}
}

func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Select, k.ToggleOn, k.Close}
}

func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down},
		{k.Select, k.ToggleOn},
		{k.Close},
	}
}

func NewMCPToggleDialog() MCPToggleDialog {
	keyMap := DefaultKeyMap()
	listKeyMap := list.DefaultKeyMap()
	listKeyMap.Down.SetEnabled(false)
	listKeyMap.Up.SetEnabled(false)
	listKeyMap.DownOneItem = keyMap.Down
	listKeyMap.UpOneItem = keyMap.Up

	t := styles.CurrentTheme()
	inputStyle := t.S().Base.PaddingLeft(1).PaddingBottom(1)
	mcpList := list.NewFilterableList(
		[]list.CompletionItem[MCPServer]{},
		list.WithFilterInputStyle(inputStyle),
		list.WithFilterListOptions(
			list.WithKeyMap(listKeyMap),
			list.WithWrapNavigation(),
			list.WithResizeByList(),
		),
	)

	help := help.New()
	help.Styles = t.S().Help

	return &mcpToggleDialogCmp{
		mcpList: mcpList,
		width:   defaultWidth,
		keyMap:  keyMap,
		help:    help,
	}
}

func (m *mcpToggleDialogCmp) Init() tea.Cmd {
	cfg := config.Get()
	servers := []MCPServer{}

	for name, mcpConfig := range cfg.MCP {
		servers = append(servers, MCPServer{
			Name:     name,
			Config:   mcpConfig,
			Disabled: mcpConfig.Disabled,
		})
	}
	m.servers = servers

	mcpItems := []list.CompletionItem[MCPServer]{}
	for _, server := range servers {
		status := "Enabled"
		if server.Disabled {
			status = "Disabled"
		}
		title := fmt.Sprintf("%s (%s)", server.Name, status)
		mcpItems = append(mcpItems, list.NewCompletionItem(title, server, list.WithCompletionID(server.Name)))
	}

	return m.mcpList.SetItems(mcpItems)
}

func (m *mcpToggleDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.wWidth = msg.Width
		m.wHeight = msg.Height
		return m, m.mcpList.SetSize(m.listWidth(), m.listHeight())
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.Select):
			selectedItem := m.mcpList.SelectedItem()
			if selectedItem == nil {
				return m, nil
			}
			server := (*selectedItem).Value()
			return m, m.toggleMCPServer(server.Name)
		case key.Matches(msg, m.keyMap.ToggleOn):
			return m, m.toggleAllMCPServers()
		case key.Matches(msg, m.keyMap.Close):
			return m, util.CmdHandler(dialogs.CloseDialogMsg{})
		default:
			u, cmd := m.mcpList.Update(msg)
			m.mcpList = u.(list.FilterableList[list.CompletionItem[MCPServer]])
			return m, cmd
		}
	}
	return m, nil
}

func (m *mcpToggleDialogCmp) toggleMCPServer(serverName string) tea.Cmd {
	cfg := config.Get()
	if mcpConfig, exists := cfg.MCP[serverName]; exists {
		// Get current selected item ID to preserve selection
		var selectedID string
		if selectedItem := m.mcpList.SelectedItem(); selectedItem != nil {
			selectedID = (*selectedItem).ID()
		}

		newDisabled := !mcpConfig.Disabled

		// Update config
		err := cfg.SetConfigField(fmt.Sprintf("mcp.%s.disabled", serverName), newDisabled)
		if err != nil {
			return util.ReportError(fmt.Errorf("failed to toggle MCP server %s: %w", serverName, err))
		}

		// Update in-memory config
		mcpConfig.Disabled = newDisabled
		cfg.MCP[serverName] = mcpConfig

		// Update display
		for i, server := range m.servers {
			if server.Name == serverName {
				m.servers[i].Disabled = newDisabled
				break
			}
		}

		// Refresh the list with preserved selection
		mcpItems := []list.CompletionItem[MCPServer]{}
		for _, server := range m.servers {
			status := "Enabled"
			if server.Disabled {
				status = "Disabled"
			}
			title := fmt.Sprintf("%s (%s)", server.Name, status)
			mcpItems = append(mcpItems, list.NewCompletionItem(title, server, list.WithCompletionID(server.Name)))
		}

		statusText := "enabled"
		if newDisabled {
			statusText = "disabled"
		}

		var cmds []tea.Cmd
		cmds = append(cmds, m.mcpList.SetItems(mcpItems))
		if selectedID != "" {
			cmds = append(cmds, m.mcpList.SetSelected(selectedID))
		}

		// If we're disabling the server, notify the main app to close the client
		if newDisabled {
			cmds = append(cmds, util.CmdHandler(MCPServerDisabledMsg{ServerName: serverName}))
			cmds = append(cmds, util.ReportInfo(fmt.Sprintf("MCP server '%s' disabled and disconnected", serverName)))
		} else {
			cmds = append(cmds, util.ReportInfo(fmt.Sprintf("MCP server '%s' %s", serverName, statusText)))
		}

		return tea.Sequence(cmds...)
	}
	return util.ReportError(fmt.Errorf("MCP server '%s' not found", serverName))
}

func (m *mcpToggleDialogCmp) toggleAllMCPServers() tea.Cmd {
	cfg := config.Get()

	// Get current selected item ID to preserve selection
	var selectedID string
	if selectedItem := m.mcpList.SelectedItem(); selectedItem != nil {
		selectedID = (*selectedItem).ID()
	}

	// Determine if we should enable or disable all servers
	// If any server is enabled, disable all. Otherwise, enable all.
	anyEnabled := false
	for _, mcpConfig := range cfg.MCP {
		if !mcpConfig.Disabled {
			anyEnabled = true
			break
		}
	}

	newDisabled := anyEnabled // If any are enabled, disable all

	var cmds []tea.Cmd
	var disabledServers []string
	for serverName, mcpConfig := range cfg.MCP {
		if mcpConfig.Disabled != newDisabled {
			// Update config
			err := cfg.SetConfigField(fmt.Sprintf("mcp.%s.disabled", serverName), newDisabled)
			if err != nil {
				cmds = append(cmds, util.ReportError(fmt.Errorf("failed to toggle MCP server %s: %w", serverName, err)))
				continue
			}

			// Update in-memory config
			mcpConfig.Disabled = newDisabled
			cfg.MCP[serverName] = mcpConfig

			// Track disabled servers for cleanup
			if newDisabled {
				disabledServers = append(disabledServers, serverName)
			}

			// Update display
			for i, server := range m.servers {
				if server.Name == serverName {
					m.servers[i].Disabled = newDisabled
					break
				}
			}
		}
	}

	// Refresh the list with preserved selection
	mcpItems := []list.CompletionItem[MCPServer]{}
	for _, server := range m.servers {
		status := "Enabled"
		if server.Disabled {
			status = "Disabled"
		}
		title := fmt.Sprintf("%s (%s)", server.Name, status)
		mcpItems = append(mcpItems, list.NewCompletionItem(title, server, list.WithCompletionID(server.Name)))
	}

	statusText := "enabled"
	if newDisabled {
		statusText = "disabled"
	}

	cmds = append(cmds, m.mcpList.SetItems(mcpItems))
	if selectedID != "" {
		cmds = append(cmds, m.mcpList.SetSelected(selectedID))
	}

	// Send disabled messages for proper cleanup
	for _, serverName := range disabledServers {
		cmds = append(cmds, util.CmdHandler(MCPServerDisabledMsg{ServerName: serverName}))
	}

	if len(disabledServers) > 0 {
		cmds = append(cmds, util.ReportInfo(fmt.Sprintf("All MCP servers %s and disconnected", statusText)))
	} else {
		cmds = append(cmds, util.ReportInfo(fmt.Sprintf("All MCP servers %s", statusText)))
	}

	return tea.Sequence(cmds...)
}

func (m *mcpToggleDialogCmp) View() string {
	t := styles.CurrentTheme()

	header := t.S().Base.Padding(0, 1, 1, 1).Render(core.Title("MCP Servers", m.width-4))
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		m.mcpList.View(),
		"",
		t.S().Base.Width(m.width-2).PaddingLeft(1).AlignHorizontal(lipgloss.Left).Render(m.help.View(m.keyMap)),
	)
	return m.style().Render(content)
}

func (m *mcpToggleDialogCmp) Cursor() *tea.Cursor {
	if cursor, ok := m.mcpList.(util.Cursor); ok {
		cursor := cursor.Cursor()
		if cursor != nil {
			cursor = m.moveCursor(cursor)
		}
		return cursor
	}
	return nil
}

func (m *mcpToggleDialogCmp) listWidth() int {
	return defaultWidth - 2
}

func (m *mcpToggleDialogCmp) listHeight() int {
	listHeight := len(m.mcpList.Items()) + 2 + 4 // height based on items + 2 for the input + 4 for the sections
	return min(listHeight, m.wHeight/2)
}

func (m *mcpToggleDialogCmp) moveCursor(cursor *tea.Cursor) *tea.Cursor {
	row, col := m.Position()
	offset := row + 3
	cursor.Y += offset
	cursor.X = cursor.X + col + 2
	return cursor
}

func (m *mcpToggleDialogCmp) style() lipgloss.Style {
	t := styles.CurrentTheme()
	return t.S().Base.
		Width(m.width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus)
}

func (m *mcpToggleDialogCmp) Position() (int, int) {
	row := m.wHeight/4 - 2
	col := m.wWidth / 2
	col -= m.width / 2
	return row, col
}

func (m *mcpToggleDialogCmp) ID() dialogs.DialogID {
	return MCPToggleDialogID
}
