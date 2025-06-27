package models

import (
	"github.com/charmbracelet/bubbles/v2/help"
	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	configv2 "github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/fur/provider"
	"github.com/charmbracelet/crush/internal/tui/components/completions"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/core/list"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/commands"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/lipgloss/v2"
)

const (
	ModelsDialogID dialogs.DialogID = "models"

	defaultWidth = 60
)

// ModelSelectedMsg is sent when a model is selected
type ModelSelectedMsg struct {
	Model configv2.PreferredModel
}

// CloseModelDialogMsg is sent when a model is selected
type CloseModelDialogMsg struct{}

// ModelDialog interface for the model selection dialog
type ModelDialog interface {
	dialogs.DialogModel
}

type ModelOption struct {
	Provider provider.Provider
	Model    provider.Model
}

type modelDialogCmp struct {
	width   int
	wWidth  int // Width of the terminal window
	wHeight int // Height of the terminal window

	modelList list.ListModel
	keyMap    KeyMap
	help      help.Model
}

func NewModelDialogCmp() ModelDialog {
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

	t := styles.CurrentTheme()
	inputStyle := t.S().Base.Padding(0, 1, 0, 1)
	modelList := list.New(
		list.WithFilterable(true),
		list.WithKeyMap(listKeyMap),
		list.WithInputStyle(inputStyle),
		list.WithWrapNavigation(true),
	)
	help := help.New()
	help.Styles = t.S().Help

	return &modelDialogCmp{
		modelList: modelList,
		width:     defaultWidth,
		keyMap:    DefaultKeyMap(),
		help:      help,
	}
}

func (m *modelDialogCmp) Init() tea.Cmd {
	providers := configv2.Providers()
	cfg := configv2.Get()

	coderAgent := cfg.Agents[configv2.AgentCoder]
	modelItems := []util.Model{}
	selectIndex := 0
	for _, provider := range providers {
		name := provider.Name
		if name == "" {
			name = string(provider.ID)
		}
		modelItems = append(modelItems, commands.NewItemSection(name))
		for _, model := range provider.Models {
			if model.ID == coderAgent.Model && provider.ID == coderAgent.Provider {
				selectIndex = len(modelItems) // Set the selected index to the current model
			}
			modelItems = append(modelItems, completions.NewCompletionItem(model.Name, ModelOption{
				Provider: provider,
				Model:    model,
			}))
		}
	}

	return tea.Sequence(m.modelList.Init(), m.modelList.SetItems(modelItems), m.modelList.SetSelected(selectIndex))
}

func (m *modelDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.wWidth = msg.Width
		m.wHeight = msg.Height
		return m, m.modelList.SetSize(m.listWidth(), m.listHeight())
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.Select):
			selectedItemInx := m.modelList.SelectedIndex()
			if selectedItemInx == list.NoSelection {
				return m, nil // No item selected, do nothing
			}
			items := m.modelList.Items()
			selectedItem := items[selectedItemInx].(completions.CompletionItem).Value().(ModelOption)

			return m, tea.Sequence(
				util.CmdHandler(dialogs.CloseDialogMsg{}),
				util.CmdHandler(ModelSelectedMsg{Model: configv2.PreferredModel{
					ModelID:  selectedItem.Model.ID,
					Provider: selectedItem.Provider.ID,
				}}),
			)
		case key.Matches(msg, m.keyMap.Close):
			return m, util.CmdHandler(dialogs.CloseDialogMsg{})
		default:
			u, cmd := m.modelList.Update(msg)
			m.modelList = u.(list.ListModel)
			return m, cmd
		}
	}
	return m, nil
}

func (m *modelDialogCmp) View() tea.View {
	t := styles.CurrentTheme()
	listView := m.modelList.View()
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		t.S().Base.Padding(0, 1, 1, 1).Render(core.Title("Switch Model", m.width-4)),
		listView.String(),
		"",
		t.S().Base.Width(m.width-2).PaddingLeft(1).AlignHorizontal(lipgloss.Left).Render(m.help.View(m.keyMap)),
	)
	v := tea.NewView(m.style().Render(content))
	if listView.Cursor() != nil {
		c := m.moveCursor(listView.Cursor())
		v.SetCursor(c)
	}
	return v
}

func (m *modelDialogCmp) style() lipgloss.Style {
	t := styles.CurrentTheme()
	return t.S().Base.
		Width(m.width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus)
}

func (m *modelDialogCmp) listWidth() int {
	return defaultWidth - 2 // 4 for padding
}

func (m *modelDialogCmp) listHeight() int {
	listHeigh := len(m.modelList.Items()) + 2 + 4 // height based on items + 2 for the input + 4 for the sections
	return min(listHeigh, m.wHeight/2)
}

func (m *modelDialogCmp) Position() (int, int) {
	row := m.wHeight/4 - 2 // just a bit above the center
	col := m.wWidth / 2
	col -= m.width / 2
	return row, col
}

func (m *modelDialogCmp) moveCursor(cursor *tea.Cursor) *tea.Cursor {
	row, col := m.Position()
	offset := row + 3 // Border + title
	cursor.Y += offset
	cursor.X = cursor.X + col + 2
	return cursor
}

func (m *modelDialogCmp) ID() dialogs.DialogID {
	return ModelsDialogID
}
