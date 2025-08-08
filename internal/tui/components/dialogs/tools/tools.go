package tools

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/v2/help"
	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/llm/agent"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/lipgloss/v2"
)

const (
	ToolsDialogID dialogs.DialogID = "tools"
	dialogWidth   = 80
	dialogHeight  = 30
)

// ToolsDialog interface for the tools management dialog
type ToolsDialog interface {
	dialogs.DialogModel
}

// ToolItem represents a tool (MCP or LSP) in the list
type ToolItem struct {
	Name     string
	Type     string // "MCP" or "LSP"
	Enabled  bool
	State    string // For status display
}

// ToolsUpdatedMsg is sent when tools configuration changes
type ToolsUpdatedMsg struct {
	MCPs map[string]bool
	LSPs map[string]bool
}

type toolsDialogCmp struct {
	width   int
	height  int
	wWidth  int
	wHeight int

	items       []ToolItem
	cursor      int
	scrollOffset int
	viewHeight  int

	app    *app.App
	keyMap KeyMap
	help   help.Model
}

// NewToolsDialog creates a new tools management dialog
func NewToolsDialog(app *app.App) ToolsDialog {
	t := styles.CurrentTheme()
	help := help.New()
	help.Styles = t.S().Help

	d := &toolsDialogCmp{
		width:  dialogWidth,
		height: dialogHeight,
		app:    app,
		keyMap: DefaultKeyMap(),
		help:   help,
	}

	d.loadTools()
	return d
}

func (d *toolsDialogCmp) loadTools() {
	d.items = []ToolItem{}
	cfg := config.Get()

	// Load MCPs
	mcpStates := agent.GetMCPStates()
	for _, mcp := range cfg.MCP.Sorted() {
		state := "disabled"
		if mcpInfo, exists := mcpStates[mcp.Name]; exists {
			state = mcpInfo.State.String()
		}
		d.items = append(d.items, ToolItem{
			Name:    mcp.Name,
			Type:    "MCP",
			Enabled: !mcp.MCP.Disabled,
			State:   state,
		})
	}

	// Load LSPs
	for _, lspItem := range cfg.LSP.Sorted() {
		state := "disabled"
		if client, exists := d.app.LSPClients[lspItem.Name]; exists && client != nil {
			serverState := client.GetServerState()
			switch serverState {
			case lsp.StateReady:
				state = "ready"
			case lsp.StateError:
				state = "error"
			case lsp.StateStarting:
				state = "starting"
			default:
				state = "unknown"
			}
		}
		d.items = append(d.items, ToolItem{
			Name:    lspItem.Name,
			Type:    "LSP",
			Enabled: lspItem.LSP.Enabled,
			State:   state,
		})
	}

	// Calculate view height (accounting for borders and help)
	d.viewHeight = d.height - 6 // title, borders, help
}

func (d *toolsDialogCmp) Init() tea.Cmd {
	return nil
}

func (d *toolsDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		d.wWidth = msg.Width
		d.wHeight = msg.Height
		d.help.Width = d.width - 2
		return d, nil

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, d.keyMap.Close):
			return d, util.CmdHandler(dialogs.CloseDialogMsg{})

		case key.Matches(msg, d.keyMap.Toggle):
			if d.cursor < len(d.items) {
				item := &d.items[d.cursor]
				item.Enabled = !item.Enabled
				
				// Update configuration
				cfg := config.Get()
				if item.Type == "MCP" {
					if mcp, exists := cfg.MCP[item.Name]; exists {
						mcp.Disabled = !item.Enabled
						cfg.MCP[item.Name] = mcp
						// Save to config file
						if err := cfg.SetConfigField(fmt.Sprintf("mcp.%s.disabled", item.Name), !item.Enabled); err != nil {
							return d, util.ReportError(err)
						}
					}
				} else if item.Type == "LSP" {
					if lsp, exists := cfg.LSP[item.Name]; exists {
						lsp.Enabled = item.Enabled
						cfg.LSP[item.Name] = lsp
						// Save to config file
						if err := cfg.SetConfigField(fmt.Sprintf("lsp.%s.enabled", item.Name), item.Enabled); err != nil {
							return d, util.ReportError(err)
						}
					}
				}

				// Send update message
				return d, d.sendUpdateMsg()
			}

		case key.Matches(msg, d.keyMap.Up):
			if d.cursor > 0 {
				d.cursor--
				if d.cursor < d.scrollOffset {
					d.scrollOffset = d.cursor
				}
			}

		case key.Matches(msg, d.keyMap.Down):
			if d.cursor < len(d.items)-1 {
				d.cursor++
				if d.cursor >= d.scrollOffset+d.viewHeight {
					d.scrollOffset = d.cursor - d.viewHeight + 1
				}
			}
		}
	}

	return d, nil
}

func (d *toolsDialogCmp) sendUpdateMsg() tea.Cmd {
	mcps := make(map[string]bool)
	lsps := make(map[string]bool)

	for _, item := range d.items {
		if item.Type == "MCP" {
			mcps[item.Name] = item.Enabled
		} else if item.Type == "LSP" {
			lsps[item.Name] = item.Enabled
		}
	}

	return util.CmdHandler(ToolsUpdatedMsg{
		MCPs: mcps,
		LSPs: lsps,
	})
}

func (d *toolsDialogCmp) View() string {
	if d.wWidth == 0 || d.wHeight == 0 {
		return ""
	}

	t := styles.CurrentTheme()
	
	// Title
	title := t.S().Title.Render("Tools Management")
	
	// Build list view
	var listView strings.Builder
	visibleEnd := d.scrollOffset + d.viewHeight
	if visibleEnd > len(d.items) {
		visibleEnd = len(d.items)
	}

	for i := d.scrollOffset; i < visibleEnd; i++ {
		item := d.items[i]
		
		// Checkbox
		checkbox := "[ ]"
		if item.Enabled {
			checkbox = "[✓]"
		}
		
		// Type badge
		typeBadge := t.S().Base.Background(t.Border).Padding(0, 1).Render(item.Type)
		if item.Type == "MCP" {
			typeBadge = t.S().Base.Background(t.Primary).Foreground(t.BgBase).Padding(0, 1).Render(item.Type)
		}
		
		// State indicator
		stateStr := ""
		if item.Enabled {
			switch item.State {
			case "connected", "ready":
				stateStr = t.S().Success.Render(" ● " + item.State)
			case "starting":
				stateStr = t.S().Warning.Render(" ● " + item.State)
			case "error":
				stateStr = t.S().Error.Render(" ● " + item.State)
			default:
				stateStr = t.S().Muted.Render(" ● " + item.State)
			}
		}
		
		// Build line
		line := fmt.Sprintf("%s %s %s%s", checkbox, typeBadge, item.Name, stateStr)
		
		// Apply cursor highlighting
		if i == d.cursor {
			line = t.S().Base.Background(t.Primary).Foreground(t.BgBase).Render(line)
		} else {
			line = t.S().Base.Render(line)
		}
		
		listView.WriteString(line + "\n")
	}
	
	// Scroll indicator
	scrollInfo := ""
	if len(d.items) > d.viewHeight {
		scrollInfo = fmt.Sprintf(" (%d/%d)", d.cursor+1, len(d.items))
	}
	
	// Help
	helpView := d.help.View(d.keyMap)
	
	// Assemble dialog
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		listView.String(),
	)
	
	if scrollInfo != "" {
		content += t.S().Muted.Render(scrollInfo)
	}
	
	content = lipgloss.JoinVertical(
		lipgloss.Left,
		content,
		"",
		helpView,
	)
	
	// Apply dialog style
	dialogStyle := t.S().Base.
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus).
		Width(d.width).
		Height(d.height)
	
	return dialogStyle.Render(content)
}

func (d *toolsDialogCmp) Position() (int, int) {
	row := (d.wHeight - d.height) / 2
	col := (d.wWidth - d.width) / 2
	return row, col
}

func (d *toolsDialogCmp) ID() dialogs.DialogID {
	return ToolsDialogID
}