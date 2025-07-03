package lsp

import (
	"github.com/charmbracelet/bubbles/v2/help"
	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/lsp/autolsp"
	"github.com/charmbracelet/crush/internal/tui/components/completions"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/core/list"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/commands"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
)

const (
	LSPDialogID  dialogs.DialogID = "language_server_protocols"
	dialogTitle                   = "Language Server Protocols"
	defaultWidth int              = 60
)

type Item struct {
	Server    autolsp.Server
	Installed bool
}

type lspDetectedMsg struct {
	items []Item
}

type LSPSelectedMsg struct {
	Item Item
}

type LSPInstallationMsg struct {
	ServerName string
	Success    bool
	Message    string
}

// LSPDialog interface for the model selection dialog
type LSPDialog interface {
	dialogs.DialogModel
}

type Model struct {
	width, wWidth, wHeight int
	loading                bool
	items                  []Item
	selected               int
	keyMap                 KeyMap
	list                   list.ListModel
	help                   help.Model
}

func New() LSPDialog {
	t := styles.CurrentTheme()
	help := help.New()
	help.Styles = t.S().Help

	listKeyMap := list.DefaultKeyMap()
	keyMap := DefaultKeyMap()

	listKeyMap.Down.SetEnabled(false)
	listKeyMap.Up.SetEnabled(false)
	listKeyMap.HalfPageDown.SetEnabled(false)
	listKeyMap.HalfPageUp.SetEnabled(false)
	listKeyMap.Home.SetEnabled(false)
	listKeyMap.End.SetEnabled(false)

	listKeyMap.DownOneItem = keyMap.Next
	listKeyMap.UpOneItem = keyMap.Previous

	inputStyle := t.S().Base.Padding(0, 1, 0, 1)

	return &Model{
		loading: true,
		width:   defaultWidth,
		keyMap:  keyMap,
		help:    help,
		list: list.New(
			list.WithFilterable(true),
			list.WithKeyMap(listKeyMap),
			list.WithInputStyle(inputStyle),
			list.WithWrapNavigation(true),
		),
	}
}

func (m *Model) ID() dialogs.DialogID {
	return LSPDialogID
}

func (m *Model) Position() (int, int) {
	row := m.wHeight/4 - 2
	col := m.wWidth / 2
	col -= m.width / 2
	return row, col
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		m.list.Init(),
		m.detectLSPs(),
	)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.wWidth = msg.Width
		m.wHeight = msg.Height
		return m, m.list.SetSize(m.listWidth(), m.listHeight())
	case lspDetectedMsg:
		m.items = msg.items
		m.loading = false

		return m, m.setListItems(msg.items)
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.Close):
			return m, util.CmdHandler(dialogs.CloseDialogMsg{})
		case key.Matches(msg, m.keyMap.Select):
			selectedIdx := m.list.SelectedIndex()
			if selectedIdx == list.NoSelection {
				return m, nil
			}
			items := m.list.Items()
			selectedItem := items[selectedIdx].(completions.CompletionItem).Value().(Item)

			return m, tea.Sequence(
				util.CmdHandler(dialogs.CloseDialogMsg{}),
				util.CmdHandler(LSPSelectedMsg{Item: selectedItem}),
			)
		default:
			var cmd tea.Cmd
			u, cmd := m.list.Update(msg)
			m.list = u.(list.ListModel)
			return m, cmd
		}
	default:
		var cmd tea.Cmd
		u, cmd := m.list.Update(msg)
		m.list = u.(list.ListModel)
		return m, cmd
	}
}

func (m *Model) setListItems(items []Item) tea.Cmd {
	listItems := make([]util.Model, 0)

	// Group items by installation status
	installedItems := []Item{}
	notInstalledItems := []Item{}

	for _, lsp := range items {
		if lsp.Installed {
			installedItems = append(installedItems, lsp)
		} else {
			notInstalledItems = append(notInstalledItems, lsp)
		}
	}

	// Add installed section if there are installed items
	if len(installedItems) > 0 {
		listItems = append(listItems, commands.NewItemSection("Installed"))
		for _, lsp := range installedItems {
			// Check if LSP is enabled in config
			cfg := config.Get()
			statusIcon := "🔴" // Red circle for disabled/not configured
			if cfg.HasLSP(string(lsp.Server.Name)) {
				statusIcon = "🟢" // Green circle for enabled and configured
			}

			text := statusIcon + " " + string(lsp.Server.Name)
			listItems = append(listItems, completions.NewCompletionItem(text, lsp))
		}
	}

	// Add not installed section if there are not installed items
	if len(notInstalledItems) > 0 {
		listItems = append(listItems, commands.NewItemSection("Not Installed"))
		for _, lsp := range notInstalledItems {
			// Not installed LSPs show gray circle
			text := "⚪ " + string(lsp.Server.Name)
			listItems = append(listItems, completions.NewCompletionItem(text, lsp))
		}
	}

	return m.list.SetItems(listItems)
}

func (c *Model) View() tea.View {
	t := styles.CurrentTheme()
	if c.loading {
		content := lipgloss.JoinVertical(
			lipgloss.Left,
			t.S().Base.Padding(0, 1, 1, 1).Render(core.Title(dialogTitle, c.width-4)),
			"Loading...",
			"",
			t.S().Base.Width(c.width-2).PaddingLeft(1).AlignHorizontal(lipgloss.Left).Render(c.help.View(c.keyMap)),
		)
		return tea.NewView(c.style().Render(content))
	}
	if len(c.items) == 0 {
		content := lipgloss.JoinVertical(
			lipgloss.Left,
			t.S().Base.Padding(0, 1, 1, 1).Render(core.Title(dialogTitle, c.width-4)),
			"Crush was unable to detect any known programming languages in this workspace.",
			"",
			t.S().Base.Width(c.width-2).PaddingLeft(1).AlignHorizontal(lipgloss.Left).Render(c.help.View(c.keyMap)),
		)
		return tea.NewView(c.style().Render(content))
	}

	listView := c.list.View()
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		t.S().Base.Padding(0, 1, 1, 1).Render(core.Title(dialogTitle, c.width-4)),
		listView.String(),
		"",
		t.S().Base.Width(c.width-2).PaddingLeft(1).AlignHorizontal(lipgloss.Left).Render(c.help.View(c.keyMap)),
	)

	v := tea.NewView(c.style().Render(content))
	if listView.Cursor() != nil {
		cursor := c.moveCursor(listView.Cursor())
		v.SetCursor(cursor)
	}
	return v
}

func (m *Model) detectLSPs() tea.Cmd {
	return func() tea.Msg {
		langs := autolsp.NewLangDetector().Detect()

		detector := autolsp.NewServerDetector(
			autolsp.ServerDetectorWithLangs(langs...),
		)
		installed, notInstalled := detector.Detect()

		items := make([]Item, 0, len(installed)+len(notInstalled))

		for _, server := range installed {
			items = append(items, Item{Server: server, Installed: true})
		}
		for _, server := range notInstalled {
			items = append(items, Item{Server: server, Installed: false})
		}

		return lspDetectedMsg{items: items}
	}
}

func (m *Model) style() lipgloss.Style {
	t := styles.CurrentTheme()
	return t.S().Base.
		Width(m.width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus)
}

func (m *Model) listWidth() int {
	return m.width - 2
}

func (m *Model) listHeight() int {
	listHeight := len(m.items) + 15
	return min(listHeight, m.wHeight/2)
}

func (m *Model) moveCursor(cursor *tea.Cursor) *tea.Cursor {
	row, col := m.Position()
	offset := row + 3
	cursor.Y += offset
	cursor.X = cursor.X + col + 2
	return cursor
}
