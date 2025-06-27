package init

import (
	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	configv2 "github.com/charmbracelet/crush/internal/config"
	cmpChat "github.com/charmbracelet/crush/internal/tui/components/chat"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
)

const InitDialogID dialogs.DialogID = "init"

// InitDialogCmp is a component that asks the user if they want to initialize the project.
type InitDialogCmp interface {
	dialogs.DialogModel
}

type initDialogCmp struct {
	wWidth, wHeight int
	width, height   int
	selected        int
	keyMap          KeyMap
}

// NewInitDialogCmp creates a new InitDialogCmp.
func NewInitDialogCmp() InitDialogCmp {
	return &initDialogCmp{
		selected: 0,
		keyMap:   DefaultKeyMap(),
	}
}

// Init implements tea.Model.
func (m *initDialogCmp) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m *initDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.wWidth = msg.Width
		m.wHeight = msg.Height
		cmd := m.SetSize()
		return m, cmd
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.Close):
			return m, tea.Batch(
				util.CmdHandler(dialogs.CloseDialogMsg{}),
				m.handleInitialization(false),
			)
		case key.Matches(msg, m.keyMap.ChangeSelection):
			m.selected = (m.selected + 1) % 2
			return m, nil
		case key.Matches(msg, m.keyMap.Select):
			return m, tea.Batch(
				util.CmdHandler(dialogs.CloseDialogMsg{}),
				m.handleInitialization(m.selected == 0),
			)
		case key.Matches(msg, m.keyMap.Y):
			return m, tea.Batch(
				util.CmdHandler(dialogs.CloseDialogMsg{}),
				m.handleInitialization(true),
			)
		case key.Matches(msg, m.keyMap.N):
			return m, tea.Batch(
				util.CmdHandler(dialogs.CloseDialogMsg{}),
				m.handleInitialization(false),
			)
		}
	}
	return m, nil
}

func (m *initDialogCmp) renderButtons() string {
	t := styles.CurrentTheme()
	baseStyle := t.S().Base

	buttons := []core.ButtonOpts{
		{
			Text:           "Yes",
			UnderlineIndex: 0, // "Y"
			Selected:       m.selected == 0,
		},
		{
			Text:           "No",
			UnderlineIndex: 0, // "N"
			Selected:       m.selected == 1,
		},
	}

	content := core.SelectableButtons(buttons, "  ")

	return baseStyle.AlignHorizontal(lipgloss.Right).Width(m.width - 4).Render(content)
}

func (m *initDialogCmp) renderContent() string {
	t := styles.CurrentTheme()
	baseStyle := t.S().Base

	explanation := t.S().Text.
		Width(m.width - 4).
		Render("Initialization generates a new CRUSH.md file that contains information about your codebase, this file serves as memory for each project, you can freely add to it to help the agents be better at their job.")

	question := t.S().Text.
		Width(m.width - 4).
		Render("Would you like to initialize this project?")

	return baseStyle.Render(lipgloss.JoinVertical(
		lipgloss.Left,
		explanation,
		"",
		question,
	))
}

func (m *initDialogCmp) render() string {
	t := styles.CurrentTheme()
	baseStyle := t.S().Base
	title := core.Title("Initialize Project", m.width-4)

	content := m.renderContent()
	buttons := m.renderButtons()

	dialogContent := lipgloss.JoinVertical(
		lipgloss.Top,
		title,
		"",
		content,
		"",
		buttons,
		"",
	)

	return baseStyle.
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus).
		Width(m.width).
		Render(dialogContent)
}

// View implements tea.Model.
func (m *initDialogCmp) View() tea.View {
	return tea.NewView(m.render())
}

// SetSize sets the size of the component.
func (m *initDialogCmp) SetSize() tea.Cmd {
	m.width = min(90, m.wWidth)
	m.height = min(15, m.wHeight)
	return nil
}

// ID implements DialogModel.
func (m *initDialogCmp) ID() dialogs.DialogID {
	return InitDialogID
}

// Position implements DialogModel.
func (m *initDialogCmp) Position() (int, int) {
	row := (m.wHeight / 2) - (m.height / 2)
	col := (m.wWidth / 2) - (m.width / 2)
	return row, col
}

// handleInitialization handles the initialization logic when the dialog is closed.
func (m *initDialogCmp) handleInitialization(initialize bool) tea.Cmd {
	if initialize {
		// Run the initialization command
		prompt := `Please analyze this codebase and create a CRUSH.md file containing:
1. Build/lint/test commands - especially for running a single test
2. Code style guidelines including imports, formatting, types, naming conventions, error handling, etc.

The file you create will be given to agentic coding agents (such as yourself) that operate in this repository. Make it about 20 lines long.
If there's already a CRUSH.md, improve it.
If there are Cursor rules (in .cursor/rules/ or .cursorrules) or Copilot rules (in .github/copilot-instructions.md), make sure to include them.
Add the .crush directory to the .gitignore file if it's not already there.`

		// Mark the project as initialized
		if err := configv2.MarkProjectInitialized(); err != nil {
			return util.ReportError(err)
		}

		return tea.Sequence(
			util.CmdHandler(cmpChat.SessionClearedMsg{}),
			util.CmdHandler(cmpChat.SendMsg{
				Text: prompt,
			}),
		)
	} else {
		// Mark the project as initialized without running the command
		if err := configv2.MarkProjectInitialized(); err != nil {
			return util.ReportError(err)
		}
	}
	return nil
}

// CloseInitDialogMsg is a message that is sent when the init dialog is closed.
type CloseInitDialogMsg struct {
	Initialize bool
}

// ShowInitDialogMsg is a message that is sent to show the init dialog.
type ShowInitDialogMsg struct {
	Show bool
}
