package dialogs

import (
	"fmt"

	"github.com/charmbracelet/bubbles/v2/help"
	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/llm/agent"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/exp/list"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
)

const (
	MCPRestartDialogID DialogID = "mcp_restart"
	LSPRestartDialogID DialogID = "lsp_restart"

	defaultRestartWidth int = 60
)

type RestartMCPMsg struct {
	Name string
}

type RestartLSPMsg struct {
	Name string
}

type restartDialogCmp struct {
	width   int
	wWidth  int
	wHeight int

	list   list.FilterableList[list.CompletionItem[RestartItem]]
	keyMap RestartDialogKeyMap
	help   help.Model

	dialogType string // "mcp" or "lsp"
}

type RestartItem struct {
	Name        string
	Type        string
	State       string
	Description string
	Disabled    bool
}

type RestartDialogKeyMap struct {
	Select key.Binding
	Close  key.Binding
}

func DefaultRestartDialogKeyMap() RestartDialogKeyMap {
	return RestartDialogKeyMap{
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Close: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "close"),
		),
	}
}

func (k RestartDialogKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Select, k.Close}
}

func (k RestartDialogKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Select, k.Close},
	}
}

func NewMCPRestartDialog() DialogModel {
	return newRestartDialog("mcp")
}

func NewLSPRestartDialog() DialogModel {
	return newRestartDialog("lsp")
}

func newRestartDialog(dialogType string) DialogModel {
	keyMap := DefaultRestartDialogKeyMap()
	listKeyMap := list.DefaultKeyMap()
	listKeyMap.Down.SetEnabled(false)
	listKeyMap.Up.SetEnabled(false)
	listKeyMap.DownOneItem = key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	)
	listKeyMap.UpOneItem = key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	)

	t := styles.CurrentTheme()
	inputStyle := t.S().Base.PaddingLeft(1).PaddingBottom(1)
	restartList := list.NewFilterableList(
		[]list.CompletionItem[RestartItem]{},
		list.WithFilterInputStyle(inputStyle),
		list.WithFilterListOptions(
			list.WithKeyMap(listKeyMap),
			list.WithWrapNavigation(),
			list.WithResizeByList(),
		),
	)

	help := help.New()
	help.Styles = t.S().Help

	return &restartDialogCmp{
		list:       restartList,
		width:      defaultRestartWidth,
		keyMap:     keyMap,
		help:       help,
		dialogType: dialogType,
	}
}

func (r *restartDialogCmp) Init() tea.Cmd {
	return r.loadItems()
}

func (r *restartDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		r.wWidth = msg.Width
		r.wHeight = msg.Height
		return r, tea.Batch(
			r.loadItems(),
			r.list.SetSize(r.listWidth(), r.listHeight()),
		)
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, r.keyMap.Select):
			selectedItem := r.list.SelectedItem()
			if selectedItem == nil {
				return r, nil
			}
			item := (*selectedItem).Value()
			if item.Disabled {
				return r, nil // Can't restart disabled items
			}

			if r.dialogType == "mcp" {
				return r, tea.Sequence(
					util.CmdHandler(CloseDialogMsg{}),
					util.CmdHandler(RestartMCPMsg{Name: item.Name}),
				)
			} else {
				return r, tea.Sequence(
					util.CmdHandler(CloseDialogMsg{}),
					util.CmdHandler(RestartLSPMsg{Name: item.Name}),
				)
			}
		case key.Matches(msg, r.keyMap.Close):
			return r, util.CmdHandler(CloseDialogMsg{})
		default:
			u, cmd := r.list.Update(msg)
			r.list = u.(list.FilterableList[list.CompletionItem[RestartItem]])
			return r, cmd
		}
	}
	return r, nil
}

func (r *restartDialogCmp) View() string {
	t := styles.CurrentTheme()
	title := "Restart MCP"
	if r.dialogType == "lsp" {
		title = "Restart LSP"
	}

	header := t.S().Base.Padding(0, 1, 1, 1).Render(core.Title(title, r.width-4))
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		r.list.View(),
		"",
		t.S().Base.Width(r.width-2).PaddingLeft(1).AlignHorizontal(lipgloss.Left).Render(r.help.View(r.keyMap)),
	)
	return r.style().Render(content)
}

func (r *restartDialogCmp) Cursor() *tea.Cursor {
	if cursor, ok := r.list.(util.Cursor); ok {
		cursor := cursor.Cursor()
		if cursor != nil {
			cursor = r.moveCursor(cursor)
		}
		return cursor
	}
	return nil
}

func (r *restartDialogCmp) loadItems() tea.Cmd {
	var items []RestartItem

	if r.dialogType == "mcp" {
		// Load MCP items
		cfg := config.Get()
		mcpStates := agent.GetMCPStates()

		for name, mcpConfig := range cfg.MCP {
			state := "unknown"
			disabled := false
			description := fmt.Sprintf("Type: %s", mcpConfig.Type)

			if mcpInfo, exists := mcpStates[name]; exists {
				state = mcpInfo.State.String()
				if mcpInfo.ToolCount > 0 {
					description += fmt.Sprintf(", Tools: %d", mcpInfo.ToolCount)
				}
				if mcpInfo.Error != nil {
					description += fmt.Sprintf(", Error: %s", mcpInfo.Error.Error())
				}
			}

			// SSE MCPs cannot be restarted
			if mcpConfig.Type == config.MCPSse {
				disabled = true
				description += " (cannot restart SSE)"
			}

			items = append(items, RestartItem{
				Name:        name,
				Type:        string(mcpConfig.Type),
				State:       state,
				Description: description,
				Disabled:    disabled,
			})
		}
	} else {
		// Load LSP items
		cfg := config.Get()
		// We need to get LSP states from the app, but we can't import app here
		// For now, let's create a basic list from configuration
		for name := range cfg.LSP {
			items = append(items, RestartItem{
				Name:        name,
				Type:        "lsp",
				State:       "unknown",
				Description: "LSP client",
				Disabled:    false,
			})
		}
	}

	if len(items) == 0 {
		noItemsText := "No MCPs available"
		if r.dialogType == "lsp" {
			noItemsText = "No LSPs available"
		}
		items = append(items, RestartItem{
			Name:        "none",
			Type:        "none",
			State:       "none",
			Description: noItemsText,
			Disabled:    true,
		})
	}

	// Convert to list items
	listItems := make([]list.CompletionItem[RestartItem], 0, len(items))
	for _, item := range items {
		title := item.Name
		if item.Disabled {
			title += " (disabled)"
		}
		title += fmt.Sprintf(" [%s]", item.State)

		opts := []list.CompletionItemOption{
			list.WithCompletionID(item.Name),
		}

		listItems = append(listItems, list.NewCompletionItem(title, item, opts...))
	}

	return r.list.SetItems(listItems)
}

func (r *restartDialogCmp) listWidth() int {
	return defaultRestartWidth - 2
}

func (r *restartDialogCmp) listHeight() int {
	listHeight := len(r.list.Items()) + 2 + 4 // height based on items + 2 for input + 4 for sections
	return min(listHeight, r.wHeight/2)
}

func (r *restartDialogCmp) moveCursor(cursor *tea.Cursor) *tea.Cursor {
	row, col := r.Position()
	offset := row + 3
	cursor.Y += offset
	cursor.X = cursor.X + col + 2
	return cursor
}

func (r *restartDialogCmp) style() lipgloss.Style {
	t := styles.CurrentTheme()
	return t.S().Base.
		Width(r.width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus)
}

func (r *restartDialogCmp) Position() (int, int) {
	row := r.wHeight/4 - 2
	col := r.wWidth / 2
	col -= r.width / 2
	return row, col
}

func (r *restartDialogCmp) ID() DialogID {
	if r.dialogType == "mcp" {
		return MCPRestartDialogID
	}
	return LSPRestartDialogID
}
