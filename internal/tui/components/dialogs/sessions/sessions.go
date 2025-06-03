package sessions

import (
	"github.com/charmbracelet/bubbles/v2/help"
	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/opencode-ai/opencode/internal/session"
	"github.com/opencode-ai/opencode/internal/tui/components/chat"
	"github.com/opencode-ai/opencode/internal/tui/components/completions"
	"github.com/opencode-ai/opencode/internal/tui/components/core"
	"github.com/opencode-ai/opencode/internal/tui/components/core/list"
	"github.com/opencode-ai/opencode/internal/tui/components/dialogs"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

const id dialogs.DialogID = "sessions"

// SessionDialog interface for the session switching dialog
type SessionDialog interface {
	dialogs.DialogModel
}

type sessionDialogCmp struct {
	selectedInx       int
	wWidth            int
	wHeight           int
	width             int
	selectedSessionID string
	keyMap            KeyMap
	sessionsList      list.ListModel
	renderedSelected  bool
	help              help.Model
}

// NewSessionDialogCmp creates a new session switching dialog
func NewSessionDialogCmp(sessions []session.Session, selectedID string) SessionDialog {
	t := styles.CurrentTheme()
	listKeyMap := list.DefaultKeyMap()
	keyMap := DefaultKeyMap()

	listKeyMap.Down.SetEnabled(false)
	listKeyMap.Up.SetEnabled(false)
	listKeyMap.NDown.SetEnabled(false)
	listKeyMap.NUp.SetEnabled(false)
	listKeyMap.HalfPageDown.SetEnabled(false)
	listKeyMap.HalfPageUp.SetEnabled(false)
	listKeyMap.Home.SetEnabled(false)
	listKeyMap.End.SetEnabled(false)

	listKeyMap.DownOneItem = keyMap.Next
	listKeyMap.UpOneItem = keyMap.Previous

	selectedInx := 0
	items := make([]util.Model, len(sessions))
	if len(sessions) > 0 {
		for i, session := range sessions {
			items[i] = completions.NewCompletionItem(session.Title, session)
			if session.ID == selectedID {
				selectedInx = i
			}
		}
	}

	sessionsList := list.New(
		list.WithFilterable(true),
		list.WithFilterPlaceholder("Enter a session name"),
		list.WithKeyMap(listKeyMap),
		list.WithItems(items),
		list.WithWrapNavigation(true),
	)
	help := help.New()
	help.Styles = t.S().Help
	s := &sessionDialogCmp{
		selectedInx:       selectedInx,
		selectedSessionID: selectedID,
		keyMap:            DefaultKeyMap(),
		sessionsList:      sessionsList,
		help:              help,
	}

	return s
}

func (s *sessionDialogCmp) Init() tea.Cmd {
	return s.sessionsList.Init()
}

func (s *sessionDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.wWidth = msg.Width
		s.wHeight = msg.Height
		s.width = s.wWidth / 2
		var cmds []tea.Cmd
		cmds = append(cmds, s.sessionsList.SetSize(s.listWidth(), s.listHeight()))
		if !s.renderedSelected {
			cmds = append(cmds, s.sessionsList.SetSelected(s.selectedInx))
			s.renderedSelected = true
		}
		return s, tea.Sequence(cmds...)
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, s.keyMap.Select):
			if len(s.sessionsList.Items()) > 0 {
				items := s.sessionsList.Items()
				selectedItemInx := s.sessionsList.SelectedIndex()
				return s, tea.Sequence(
					util.CmdHandler(dialogs.CloseDialogMsg{}),
					util.CmdHandler(
						chat.SessionSelectedMsg(items[selectedItemInx].(completions.CompletionItem).Value().(session.Session)),
					),
				)
			}
		default:
			u, cmd := s.sessionsList.Update(msg)
			s.sessionsList = u.(list.ListModel)
			return s, cmd
		}
	}
	return s, nil
}

func (s *sessionDialogCmp) View() tea.View {
	t := styles.CurrentTheme()
	listView := s.sessionsList.View()
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		t.S().Base.Padding(0, 1, 1, 1).Render(core.Title("Switch Session", s.width-4)),
		listView.String(),
		"",
		t.S().Base.Width(s.width-2).PaddingLeft(1).AlignHorizontal(lipgloss.Left).Render(s.help.View(s.keyMap)),
	)

	v := tea.NewView(s.style().Render(content))
	if listView.Cursor() != nil {
		c := s.moveCursor(listView.Cursor())
		v.SetCursor(c)
	}
	return v
}

func (s *sessionDialogCmp) style() lipgloss.Style {
	t := styles.CurrentTheme()
	return t.S().Base.
		Width(s.width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus)
}

func (s *sessionDialogCmp) listHeight() int {
	return s.wHeight/2 - 6 // 5 for the border, title and help
}

func (s *sessionDialogCmp) listWidth() int {
	return s.width - 2 // 2 for the border
}

func (s *sessionDialogCmp) Position() (int, int) {
	row := s.wHeight/4 - 2 // just a bit above the center
	col := s.wWidth / 2
	col -= s.width / 2
	return row, col
}

func (s *sessionDialogCmp) moveCursor(cursor *tea.Cursor) *tea.Cursor {
	row, col := s.Position()
	offset := row + 3 // Border + title
	cursor.Y += offset
	cursor.X = cursor.X + col + 2
	return cursor
}

// ID implements SessionDialog.
func (s *sessionDialogCmp) ID() dialogs.DialogID {
	return id
}
