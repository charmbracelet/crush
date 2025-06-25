package update

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/crush/internal/updater"
	"github.com/charmbracelet/lipgloss/v2"
)

const DialogID dialogs.DialogID = "update"

type State int

const (
	StatePrompt State = iota
	StateUpdating
	StateSuccess
	StateError
)

// UpdateAvailableMsg is sent when an update is available
type UpdateAvailableMsg struct {
	UpdateInfo *updater.UpdateInfo
}

// UpdateStartMsg is sent when update process starts
type UpdateStartMsg struct{}

// UpdateCompleteMsg is sent when update completes successfully
type UpdateCompleteMsg struct{}

// UpdateErrorMsg is sent when update fails
type UpdateErrorMsg struct {
	Error error
}

// UpdateDeclinedMsg is sent when user declines the update
type UpdateDeclinedMsg struct{}

type Model struct {
	wWidth, wHeight int
	width, height   int
	updateInfo      *updater.UpdateInfo
	state           State
	error           error
	ctx             context.Context
	selected        int
	keyMap          KeyMap
}

func New(ctx context.Context, updateInfo *updater.UpdateInfo) *Model {
	return &Model{
		updateInfo: updateInfo,
		state:      StatePrompt,
		ctx:        ctx,
		selected:   0,
		keyMap:     DefaultKeyMap(),
	}
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.wWidth = msg.Width
		m.wHeight = msg.Height
		cmd := m.SetSize()
		return m, cmd

	case tea.KeyPressMsg:
		switch m.state {
		case StatePrompt:
			switch {
			case key.Matches(msg, m.keyMap.Close):
				return m, tea.Batch(
					util.CmdHandler(dialogs.CloseDialogMsg{}),
					util.CmdHandler(UpdateDeclinedMsg{}),
				)
			case key.Matches(msg, m.keyMap.ChangeSelection):
				m.selected = (m.selected + 1) % 2
				return m, nil
			case key.Matches(msg, m.keyMap.Select):
				if m.selected == 0 {
					m.state = StateUpdating
					return m, m.performUpdate()
				} else {
					return m, tea.Batch(
						util.CmdHandler(dialogs.CloseDialogMsg{}),
						util.CmdHandler(UpdateDeclinedMsg{}),
					)
				}
			case key.Matches(msg, m.keyMap.Y):
				m.state = StateUpdating
				return m, m.performUpdate()
			case key.Matches(msg, m.keyMap.N):
				return m, tea.Batch(
					util.CmdHandler(dialogs.CloseDialogMsg{}),
					util.CmdHandler(UpdateDeclinedMsg{}),
				)
			}
		case StateSuccess, StateError:
			switch {
			case key.Matches(msg, m.keyMap.Close), key.Matches(msg, m.keyMap.Select):
				return m, util.CmdHandler(dialogs.CloseDialogMsg{})
			}
		}

	case UpdateCompleteMsg:
		m.state = StateSuccess
		return m, nil

	case UpdateErrorMsg:
		m.state = StateError
		m.error = msg.Error
		return m, nil
	}

	return m, nil
}

func (m *Model) performUpdate() tea.Cmd {
	return func() tea.Msg {
		if err := updater.PerformUpdate(m.ctx, m.updateInfo); err != nil {
			return UpdateErrorMsg{Error: err}
		}
		return UpdateCompleteMsg{}
	}
}

func (m *Model) renderButtons() string {
	if m.state != StatePrompt {
		return ""
	}

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

func (m *Model) renderContent() string {
	t := styles.CurrentTheme()
	baseStyle := t.S().Base

	var content string

	switch m.state {
	case StatePrompt:
		info := fmt.Sprintf("A new version of Crush is available!\n\n"+
			"Current version: %s\n"+
			"Latest version:  %s\n\n",
			m.updateInfo.CurrentVersion,
			m.updateInfo.LatestVersion)

		// Show release notes if available (truncated)
		if m.updateInfo.ReleaseNotes != "" {
			notes := m.updateInfo.ReleaseNotes
			if len(notes) > 200 {
				notes = notes[:200] + "..."
			}
			info += fmt.Sprintf("Release Notes:\n%s\n\n", notes)
		}

		info += "Would you like to update now?"
		content = info

	case StateUpdating:
		content = "Downloading and installing the update...\n\n" +
			"Please wait, this may take a moment."

	case StateSuccess:
		content = fmt.Sprintf("Crush has been successfully updated to version %s!\n\n"+
			"Please restart the application to use the new version.",
			m.updateInfo.LatestVersion)

	case StateError:
		errorMsg := m.error.Error()
		if len(errorMsg) > 300 {
			errorMsg = errorMsg[:300] + "..."
		}
		content = fmt.Sprintf("Failed to update Crush:\n\n%s\n\n"+
			"You can try updating manually by visiting:\n"+
			"https://github.com/charmbracelet/crush-internal/releases",
			errorMsg)
	}

	return baseStyle.
		Width(m.width - 4).
		Render(content)
}

func (m *Model) render() string {
	t := styles.CurrentTheme()
	baseStyle := t.S().Base

	var title string
	switch m.state {
	case StatePrompt:
		title = "Update Available"
	case StateUpdating:
		title = "Updating..."
	case StateSuccess:
		title = "Update Complete"
	case StateError:
		title = "Update Failed"
	}

	titleRendered := core.Title(title, m.width-4)
	content := m.renderContent()
	buttons := m.renderButtons()

	var dialogContent string
	if buttons != "" {
		dialogContent = lipgloss.JoinVertical(
			lipgloss.Top,
			titleRendered,
			"",
			content,
			"",
			buttons,
			"",
		)
	} else {
		dialogContent = lipgloss.JoinVertical(
			lipgloss.Top,
			titleRendered,
			"",
			content,
			"",
		)
	}

	return baseStyle.
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus).
		Width(m.width).
		Render(dialogContent)
}

func (m *Model) View() tea.View {
	return tea.NewView(m.render())
}

func (m *Model) SetSize() tea.Cmd {
	m.width = min(80, m.wWidth)
	m.height = min(20, m.wHeight)
	return nil
}

func (m *Model) Position() (int, int) {
	row := (m.wHeight / 2) - (m.height / 2)
	col := (m.wWidth / 2) - (m.width / 2)
	return row, col
}

func (m *Model) ID() dialogs.DialogID {
	return DialogID
}

func (m *Model) Close() tea.Cmd {
	return nil
}