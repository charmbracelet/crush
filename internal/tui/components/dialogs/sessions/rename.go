package sessions

import (
	"context"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/v2/help"
	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/textinput"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/lipgloss/v2"
)

const SessionRenameDialogID dialogs.DialogID = "session_rename"

// SessionRenameDialog interface for the session rename dialog
type SessionRenameDialog interface {
	dialogs.DialogModel
}

type sessionRenameDialogCmp struct {
	wWidth          int
	wHeight         int
	width           int
	keyMap          SessionRenameKeys
	sessions        session.Service
	selectedSession session.Session
	input           textinput.Model
	help            help.Model
}

// NewSessionRenameDialogCmp creates a new session rename dialog
func NewSessionRenameDialogCmp(sessions session.Service, selectedSession session.Session) SessionRenameDialog {
	t := styles.CurrentTheme()

	inputValue := selectedSession.Title

	ti := textinput.New()
	ti.SetValue(inputValue)
	ti.SetVirtualCursor(false)
	ti.SetCursor(utf8.RuneCountInString(inputValue))
	ti.SetStyles(t.S().TextInput)
	ti.Focus()

	help := help.New()
	help.Styles = t.S().Help

	s := &sessionRenameDialogCmp{
		keyMap:          SessionRenameKeyMap(),
		sessions:        sessions,
		selectedSession: selectedSession,
		input:           ti,
		help:            help,
	}

	return s
}

// ID implements SessionDialog.
func (s *sessionRenameDialogCmp) ID() dialogs.DialogID {
	return SessionRenameDialogID
}

func (s *sessionRenameDialogCmp) Init() tea.Cmd {
	return s.input.Focus()
}

func (s *sessionRenameDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.wWidth = msg.Width
		s.wHeight = msg.Height
		s.width = min(120, s.wWidth-8)
		s.input.SetWidth(s.inputWidth())
		return s, nil
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, s.keyMap.Confirm):
			newTitle := s.input.Value()
			if newTitle == "" {
				return s, util.ReportError(errors.New("session name cannot be empty"))
			}
			s.selectedSession.Title = newTitle
			updated, err := s.sessions.Save(context.Background(), s.selectedSession)
			if err != nil {
				return s, util.ReportError(fmt.Errorf("cannot save session: %w", err))
			}
			s.selectedSession = updated
			return s, tea.Sequence(
				util.CmdHandler(dialogs.CloseDialogMsg{}),
				util.CmdHandler(dialogs.OpenDialogMsg{
					Model: NewSessionDialogCmp(s.sessions, s.selectedSession.ID),
				}),
			)
		case key.Matches(msg, s.keyMap.Close):
			return s, tea.Sequence(
				util.CmdHandler(dialogs.CloseDialogMsg{}),
				util.CmdHandler(dialogs.OpenDialogMsg{
					Model: NewSessionDialogCmp(s.sessions, s.selectedSession.ID),
				}),
			)
		default:
			u, cmd := s.input.Update(msg)
			s.input = u
			return s, cmd
		}
	}
	return s, nil
}

func (s *sessionRenameDialogCmp) View() string {
	t := styles.CurrentTheme()
	inputStyle := t.S().Base.PaddingLeft(1).PaddingBottom(1)
	inputView := inputStyle.Render(s.input.View())
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		t.S().Base.Padding(0, 1, 1, 1).Render(core.Title("Rename Session", s.width-4)),
		inputView,
		"",
		t.S().Base.Width(s.width-2).PaddingLeft(1).AlignHorizontal(lipgloss.Left).Render(s.help.View(s.keyMap)),
	)

	return s.style().Render(content)
}

func (s *sessionRenameDialogCmp) Position() (int, int) {
	row := s.wHeight/4 - 2 // just a bit above the center
	col := s.wWidth / 2
	col -= s.width / 2
	return row, col
}

func (s *sessionRenameDialogCmp) style() lipgloss.Style {
	t := styles.CurrentTheme()
	return t.S().Base.
		Width(s.width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus)
}

func (s *sessionRenameDialogCmp) inputWidth() int {
	return s.width - 2 // 2 for the border
}
