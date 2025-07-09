package models

import (
	"github.com/charmbracelet/bubbles/v2/help"
	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/fur/provider"
	"github.com/charmbracelet/crush/internal/tui/components/completions"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/core/list"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/lipgloss/v2"
)

const (
	ModelsDialogID dialogs.DialogID = "models"

	defaultWidth = 60
)

const (
	LargeModelType int = iota
	SmallModelType

	largeModelInputPlaceholder = "Choose a model for large, complex tasks"
	smallModelInputPlaceholder = "Choose a model for small, simple tasks"
)

// ModelSelectedMsg is sent when a model is selected
type ModelSelectedMsg struct {
	Model     config.SelectedModel
	ModelType config.SelectedModelType
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
	wWidth  int
	wHeight int

	modelList *ModelListComponent
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
	modelList := NewModelListComponent(listKeyMap, inputStyle, "Choose a model for large, complex tasks")
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
	return m.modelList.Init()
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
				return m, nil
			}
			items := m.modelList.Items()
			selectedItem := items[selectedItemInx].(completions.CompletionItem).Value().(ModelOption)

			var modelType config.SelectedModelType
			if m.modelList.GetModelType() == LargeModelType {
				modelType = config.SelectedModelTypeLarge
			} else {
				modelType = config.SelectedModelTypeSmall
			}

			return m, tea.Sequence(
				util.CmdHandler(dialogs.CloseDialogMsg{}),
				util.CmdHandler(ModelSelectedMsg{
					Model: config.SelectedModel{
						Model:    selectedItem.Model.ID,
						Provider: string(selectedItem.Provider.ID),
					},
					ModelType: modelType,
				}),
			)
		case key.Matches(msg, m.keyMap.Tab):
			if m.modelList.GetModelType() == LargeModelType {
				m.modelList.SetInputPlaceholder(smallModelInputPlaceholder)
				return m, m.modelList.SetModelType(SmallModelType)
			} else {
				m.modelList.SetInputPlaceholder(largeModelInputPlaceholder)
				return m, m.modelList.SetModelType(LargeModelType)
			}
		case key.Matches(msg, m.keyMap.Close):
			return m, util.CmdHandler(dialogs.CloseDialogMsg{})
		default:
			u, cmd := m.modelList.Update(msg)
			m.modelList = u
			return m, cmd
		}
	}
	return m, nil
}

func (m *modelDialogCmp) View() string {
	t := styles.CurrentTheme()
	listView := m.modelList.View()
	radio := m.modelTypeRadio()
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		t.S().Base.Padding(0, 1, 1, 1).Render(core.Title("Switch Model", m.width-lipgloss.Width(radio)-5)+" "+radio),
		listView,
		"",
		t.S().Base.Width(m.width-2).PaddingLeft(1).AlignHorizontal(lipgloss.Left).Render(m.help.View(m.keyMap)),
	)
	return m.style().Render(content)
}

func (m *modelDialogCmp) Cursor() *tea.Cursor {
	cursor := m.modelList.Cursor()
	if cursor != nil {
		cursor = m.moveCursor(cursor)
		return cursor
	}
	return nil
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
	items := m.modelList.Items()
	listHeigh := len(items) + 2 + 4
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

func (m *modelDialogCmp) modelTypeRadio() string {
	t := styles.CurrentTheme()
	choices := []string{"Large Task", "Small Task"}
	iconSelected := "◉"
	iconUnselected := "○"
	if m.modelList.GetModelType() == LargeModelType {
		return t.S().Base.Foreground(t.FgHalfMuted).Render(iconSelected + " " + choices[0] + "  " + iconUnselected + " " + choices[1])
	}
	return t.S().Base.Foreground(t.FgHalfMuted).Render(iconUnselected + " " + choices[0] + "  " + iconSelected + " " + choices[1])
}
