package dialog

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/dustin/go-humanize"
)

// MCPServersID is the identifier for the MCP servers dialog.
const MCPServersID = "mcp_servers"

// MCPServers represents a dialog that shows configured MCP servers.
type MCPServers struct {
	com    *common.Common
	keyMap struct {
		Select,
		UpDown,
		Next,
		Previous,
		Close key.Binding
	}

	help  help.Model
	input textinput.Model
	list  *list.FilterableList

	servers []mcp.ClientInfo

	// serverTools maps server name to its tools.
	serverTools map[string][]*mcp.Tool
	// serverPrompts maps server name to its prompts.
	serverPrompts map[string][]*mcp.Prompt

	// showingDetail indicates if we're showing server details.
	showingDetail bool
	// selectedServer is the server whose details are being shown.
	selectedServer *mcp.ClientInfo
}

var _ Dialog = (*MCPServers)(nil)

// NewMCPServers creates a new MCP servers dialog.
func NewMCPServers(com *common.Common) (*MCPServers, error) {
	m := &MCPServers{
		com: com,
	}

	help := help.New()
	help.Styles = com.Styles.DialogHelpStyles()

	m.help = help

	m.list = list.NewFilterableList()
	m.list.Focus()
	m.list.SetSelected(0)

	m.input = textinput.New()
	m.input.SetVirtualCursor(false)
	m.input.Placeholder = "Type to filter"
	m.input.SetStyles(com.Styles.TextInput)
	m.input.Focus()

	m.keyMap.Select = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "select"),
	)
	m.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑/↓", "choose"),
	)
	m.keyMap.Next = key.NewBinding(
		key.WithKeys("down"),
		key.WithHelp("↓", "next item"),
	)
	m.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑", "previous item"),
	)
	closeKey := CloseKey
	closeKey.SetHelp("esc", "back/cancel")
	m.keyMap.Close = closeKey

	// Load MCP server states.
	m.loadServers()

	return m, nil
}

// loadServers loads the current MCP server states.
func (m *MCPServers) loadServers() {
	states := mcp.GetStates()
	m.servers = make([]mcp.ClientInfo, 0, len(states))

	// Get servers in sorted order from config.
	for _, mcpCfg := range m.com.Config().MCP.Sorted() {
		if state, ok := states[mcpCfg.Name]; ok {
			m.servers = append(m.servers, state)
		}
	}

	// Load tools and prompts for each server.
	m.serverTools = make(map[string][]*mcp.Tool)
	m.serverPrompts = make(map[string][]*mcp.Prompt)

	for name, tools := range mcp.Tools() {
		m.serverTools[name] = tools
	}

	for name, prompts := range mcp.Prompts() {
		m.serverPrompts[name] = prompts
	}

	m.setServerItems()
}

// setServerItems sets the list items from loaded servers.
func (m *MCPServers) setServerItems() {
	items := make([]list.FilterableItem, len(m.servers))
	for i := range m.servers {
		items[i] = &MCPServerItem{
			Server: &m.servers[i],
			t:      m.com.Styles,
		}
	}
	m.list.SetItems(items...)
	m.list.SetFilter("")
	m.list.ScrollToTop()
	m.list.SetSelected(0)
	m.input.SetValue("")
}

// ID implements Dialog.
func (m *MCPServers) ID() string {
	return MCPServersID
}

// HandleMsg implements [Dialog].
func (m *MCPServers) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.Close):
			if m.showingDetail {
				m.showingDetail = false
				m.selectedServer = nil
				return nil
			}
			return ActionClose{}
		case key.Matches(msg, m.keyMap.Previous):
			if m.showingDetail {
				return nil
			}
			m.list.Focus()
			if m.list.IsSelectedFirst() {
				m.list.SelectLast()
				m.list.ScrollToBottom()
				break
			}
			m.list.SelectPrev()
			m.list.ScrollToSelected()
		case key.Matches(msg, m.keyMap.Next):
			if m.showingDetail {
				return nil
			}
			m.list.Focus()
			if m.list.IsSelectedLast() {
				m.list.SelectFirst()
				m.list.ScrollToTop()
				break
			}
			m.list.SelectNext()
			m.list.ScrollToSelected()
		case key.Matches(msg, m.keyMap.Select):
			if m.showingDetail {
				return nil
			}
			if selectedItem := m.list.SelectedItem(); selectedItem != nil {
				if item, ok := selectedItem.(*MCPServerItem); ok && item != nil {
					m.selectedServer = item.Server
					m.showingDetail = true
				}
			}
		default:
			if m.showingDetail {
				return nil
			}
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			value := m.input.Value()
			m.list.SetFilter(value)
			m.list.ScrollToTop()
			m.list.SetSelected(0)
			return ActionCmd{cmd}
		}
	}
	return nil
}

// Cursor returns the cursor position relative to the dialog.
func (m *MCPServers) Cursor() *tea.Cursor {
	if m.showingDetail {
		return nil
	}
	return InputCursor(m.com.Styles, m.input.Cursor())
}

// Draw implements [Dialog].
func (m *MCPServers) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	if m.showingDetail {
		return m.drawDetail(scr, area)
	}
	return m.drawList(scr, area)
}

// drawList draws the server list view.
func (m *MCPServers) drawList(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := m.com.Styles
	width := max(0, min(defaultDialogMaxWidth, area.Dx()))
	height := max(0, min(defaultDialogHeight, area.Dy()))

	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.InputPrompt.GetVerticalFrameSize() + inputContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()

	m.input.SetWidth(innerWidth - t.Dialog.InputPrompt.GetHorizontalFrameSize() - 1)

	listHeight := min(height-heightOffset, m.list.Len())
	m.list.SetSize(innerWidth, listHeight)
	m.help.SetWidth(innerWidth)

	rc := NewRenderContext(t, width)
	rc.Title = "MCP Servers"

	if len(m.servers) == 0 {
		rc.AddPart(t.Dialog.NormalItem.Render("No MCP servers configured"))
		rc.AddPart(t.Subtle.Render("Configure MCP servers in crush.json"))
	} else {
		inputView := t.Dialog.InputPrompt.Render(m.input.View())
		rc.AddPart(inputView)
		listView := t.Dialog.List.Height(m.list.Height()).Render(m.list.Render())
		rc.AddPart(listView)
	}
	rc.Help = m.help.View(m)

	view := rc.Render()

	cur := m.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

// drawDetail draws the server detail view.
func (m *MCPServers) drawDetail(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := m.com.Styles
	width := max(0, min(defaultDialogMaxWidth, area.Dx()))

	rc := NewRenderContext(t, width)
	rc.Title = m.selectedServer.Name
	rc.Gap = 1

	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()

	// Use TitleAccent for bold labels.
	labelStyle := t.Dialog.TitleAccent

	// Status.
	rc.AddPart(labelStyle.Render("Status"))
	statusText := m.selectedServer.State.String()
	statusStyle := t.Dialog.NormalItem
	switch m.selectedServer.State {
	case mcp.StateConnected:
		statusStyle = t.Dialog.NormalItem
		statusText = "Connected"
	case mcp.StateStarting:
		statusText = "Starting..."
	case mcp.StateError:
		statusText = "Error"
		if m.selectedServer.Error != nil {
			statusText = fmt.Sprintf("Error: %s", m.selectedServer.Error.Error())
		}
	case mcp.StateDisabled:
		statusText = "Disabled"
	}
	rc.AddPart(statusStyle.Render(statusText))

	// Connected time (if connected).
	if m.selectedServer.State == mcp.StateConnected && !m.selectedServer.ConnectedAt.IsZero() {
		rc.AddPart(labelStyle.Render("Connected"))
		rc.AddPart(t.Dialog.NormalItem.Render(humanize.Time(m.selectedServer.ConnectedAt)))
	}

	// Tools - show names.
	if tools := m.serverTools[m.selectedServer.Name]; len(tools) > 0 {
		rc.AddPart(labelStyle.Render(fmt.Sprintf("Tools (%d)", len(tools))))
		var toolNames []string
		for _, tool := range tools {
			toolNames = append(toolNames, tool.Name)
		}
		toolsStr := strings.Join(toolNames, ", ")
		if len(toolsStr) > innerWidth-4 {
			toolsStr = toolsStr[:innerWidth-7] + "..."
		}
		rc.AddPart(t.Dialog.NormalItem.Render(toolsStr))
	}

	// Prompts - show names.
	if prompts := m.serverPrompts[m.selectedServer.Name]; len(prompts) > 0 {
		rc.AddPart(labelStyle.Render(fmt.Sprintf("Prompts (%d)", len(prompts))))
		var promptNames []string
		for _, prompt := range prompts {
			promptNames = append(promptNames, prompt.Name)
		}
		promptsStr := strings.Join(promptNames, ", ")
		if len(promptsStr) > innerWidth-4 {
			promptsStr = promptsStr[:innerWidth-7] + "..."
		}
		rc.AddPart(t.Dialog.NormalItem.Render(promptsStr))
	}

	// Server configuration from config.
	if mcpCfg, ok := m.com.Config().MCP[m.selectedServer.Name]; ok {
		// Type.
		rc.AddPart(labelStyle.Render("Type"))
		rc.AddPart(t.Dialog.NormalItem.Render(string(mcpCfg.Type)))

		// Command/URL.
		if mcpCfg.Command != "" {
			rc.AddPart(labelStyle.Render("Command"))
			cmd := mcpCfg.Command
			if len(mcpCfg.Args) > 0 {
				cmd = cmd + " " + strings.Join(mcpCfg.Args, " ")
			}
			if len(cmd) > innerWidth-4 {
				cmd = cmd[:innerWidth-7] + "..."
			}
			rc.AddPart(t.Dialog.NormalItem.Render(cmd))
		}

		if mcpCfg.URL != "" {
			rc.AddPart(labelStyle.Render("URL"))
			url := mcpCfg.URL
			if len(url) > innerWidth-4 {
				url = url[:innerWidth-7] + "..."
			}
			rc.AddPart(t.Dialog.NormalItem.Render(url))
		}

		// Timeout.
		if mcpCfg.Timeout > 0 {
			rc.AddPart(labelStyle.Render("Timeout"))
			rc.AddPart(t.Dialog.NormalItem.Render(fmt.Sprintf("%s", time.Duration(mcpCfg.Timeout)*time.Second)))
		}

		// Disabled tools.
		if len(mcpCfg.DisabledTools) > 0 {
			rc.AddPart(labelStyle.Render("Disabled Tools"))
			tools := strings.Join(mcpCfg.DisabledTools, ", ")
			if len(tools) > innerWidth-4 {
				tools = tools[:innerWidth-7] + "..."
			}
			rc.AddPart(t.Dialog.NormalItem.Render(tools))
		}
	}

	m.help.SetWidth(innerWidth)
	rc.Help = m.help.View(m)

	view := rc.Render()

	DrawCenter(scr, area, view)
	return nil
}

// ShortHelp implements [help.KeyMap].
func (m *MCPServers) ShortHelp() []key.Binding {
	if m.showingDetail {
		return []key.Binding{
			m.keyMap.Close,
		}
	}
	return []key.Binding{
		m.keyMap.UpDown,
		m.keyMap.Select,
		m.keyMap.Close,
	}
}

// FullHelp implements [help.KeyMap].
func (m *MCPServers) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{m.keyMap.Select, m.keyMap.Next, m.keyMap.Previous},
		{m.keyMap.Close},
	}
}
