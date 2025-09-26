package sessions

import (
	"context"
	"github.com/charmbracelet/bubbles/v2/help"
	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/event"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/tui/components/chat"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/exp/list"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/lipgloss/v2"
)

const SessionsDialogID dialogs.DialogID = "sessions"

// SessionDeletedMsg is sent when a session is deleted from the sessions dialog
type SessionDeletedMsg struct {
	SessionID string
}

// SessionDialog interface for the session switching dialog
type SessionDialog interface {
	dialogs.DialogModel
}

type SessionsList = list.FilterableList[list.CompletionItem[session.Session]]

type sessionDialogCmp struct {
	selectedInx       int
	wWidth            int
	wHeight           int
	width             int
	selectedSessionID string
	keyMap            KeyMap
	sessionsList      SessionsList
	help              help.Model
	app               *app.App
	showingConfirm    bool
	sessionToDelete   string
	confirmSelected   int // 0 for No (default), 1 for Yes
}

// NewSessionDialogCmp creates a new session switching dialog
func NewSessionDialogCmp(sessions []session.Session, selectedID string, app *app.App) SessionDialog {
	t := styles.CurrentTheme()
	listKeyMap := list.DefaultKeyMap()
	keyMap := DefaultKeyMap()
	listKeyMap.Down.SetEnabled(false)
	listKeyMap.Up.SetEnabled(false)
	listKeyMap.DownOneItem = keyMap.Next
	listKeyMap.UpOneItem = keyMap.Previous

	items := make([]list.CompletionItem[session.Session], len(sessions))
	if len(sessions) > 0 {
		for i, session := range sessions {
			items[i] = list.NewCompletionItem(session.Title, session, list.WithCompletionID(session.ID))
		}
	}

	inputStyle := t.S().Base.PaddingLeft(1).PaddingBottom(1)
	sessionsList := list.NewFilterableList(
		items,
		list.WithFilterPlaceholder("Enter a session name"),
		list.WithFilterInputStyle(inputStyle),
		list.WithFilterListOptions(
			list.WithKeyMap(listKeyMap),
			list.WithWrapNavigation(),
		),
	)
	help := help.New()
	help.Styles = t.S().Help
	s := &sessionDialogCmp{
		selectedSessionID: selectedID,
		keyMap:            DefaultKeyMap(),
		sessionsList:      sessionsList,
		help:              help,
		app:               app,
	}

	return s
}

func (s *sessionDialogCmp) Init() tea.Cmd {
	var cmds []tea.Cmd
	cmds = append(cmds, s.sessionsList.Init())
	cmds = append(cmds, s.sessionsList.Focus())
	return tea.Sequence(cmds...)
}

func (s *sessionDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		var cmds []tea.Cmd
		s.wWidth = msg.Width
		s.wHeight = msg.Height
		s.width = min(120, s.wWidth-8)
		s.sessionsList.SetInputWidth(s.listWidth() - 2)
		cmds = append(cmds, s.sessionsList.SetSize(s.listWidth(), s.listHeight()))
		if s.selectedSessionID != "" {
			cmds = append(cmds, s.sessionsList.SetSelected(s.selectedSessionID))
		}
		return s, tea.Batch(cmds...)
	case SessionDeletedMsg:
		// Remove the deleted session from the list and refresh
		allSessions, _ := s.app.Sessions.List(context.Background())
		items := make([]list.CompletionItem[session.Session], len(allSessions))
		for i, session := range allSessions {
			items[i] = list.NewCompletionItem(session.Title, session, list.WithCompletionID(session.ID))
		}
		
		// If the deleted session was the currently selected one, clear the selection
		if s.selectedSessionID == msg.SessionID {
			s.selectedSessionID = ""
		}
		
		cmd := s.sessionsList.SetItems(items)
		return s, cmd
	case chat.SessionSelectedMsg:
		// Update the selected session when a new session is selected
		s.selectedSessionID = msg.ID
		var cmds []tea.Cmd
		// Also refresh the session list to include any newly created sessions
		allSessions, _ := s.app.Sessions.List(context.Background())
		items := make([]list.CompletionItem[session.Session], len(allSessions))
		for i, session := range allSessions {
			items[i] = list.NewCompletionItem(session.Title, session, list.WithCompletionID(session.ID))
		}
		cmds = append(cmds, s.sessionsList.SetItems(items))
		if s.selectedSessionID != "" {
			cmds = append(cmds, s.sessionsList.SetSelected(s.selectedSessionID))
		}
		return s, tea.Batch(cmds...)
	case tea.KeyPressMsg:
		// Handle confirmation dialog keys first
		if s.showingConfirm {
			switch {
			case key.Matches(msg, s.keyMap.Next) || key.Matches(msg, s.keyMap.Previous) || key.Matches(msg, s.keyMap.Tab):
				// Toggle between Yes/No buttons (arrow keys or tab navigation)
				s.confirmSelected = (s.confirmSelected + 1) % 2
				return s, nil
			case key.Matches(msg, s.keyMap.Select):
				// Execute selected action
				if s.confirmSelected == 1 { // Yes selected
					s.showingConfirm = false
					sessionID := s.sessionToDelete
					s.sessionToDelete = ""
					s.confirmSelected = 0 // Reset to No for next time
					return s, func() tea.Msg {
						if err := s.app.Sessions.Delete(context.Background(), sessionID); err != nil {
							return util.InfoMsg{
								Type: util.InfoTypeError,
								Msg:  err.Error(),
							}
						}
						return SessionDeletedMsg{SessionID: sessionID}
					}
				} else { // No selected
					s.showingConfirm = false
					s.sessionToDelete = ""
					s.confirmSelected = 0 // Reset to No for next time
					return s, nil
				}
			case msg.String() == "y" || msg.String() == "Y":
				// Direct Yes key
				s.showingConfirm = false
				sessionID := s.sessionToDelete
				s.sessionToDelete = ""
				s.confirmSelected = 0 // Reset to No for next time
				return s, func() tea.Msg {
					if err := s.app.Sessions.Delete(context.Background(), sessionID); err != nil {
						return util.InfoMsg{
							Type: util.InfoTypeError,
							Msg:  err.Error(),
						}
					}
					return SessionDeletedMsg{SessionID: sessionID}
				}
			case msg.String() == "n" || msg.String() == "N" || key.Matches(msg, s.keyMap.Close):
				// Cancel delete
				s.showingConfirm = false
				s.sessionToDelete = ""
				s.confirmSelected = 0 // Reset to No for next time
				return s, nil
			}
			return s, nil // Ignore other keys when showing confirm
		}
		
		switch {
		case key.Matches(msg, s.keyMap.Select):
			selectedItem := s.sessionsList.SelectedItem()
			if selectedItem != nil {
				selected := *selectedItem
				event.SessionSwitched()
				return s, tea.Sequence(
					util.CmdHandler(dialogs.CloseDialogMsg{}),
					util.CmdHandler(
						chat.SessionSelectedMsg(selected.Value()),
					),
				)
			}
		case key.Matches(msg, s.keyMap.Delete):
			if s.showingConfirm {
				return s, nil // Ignore delete key when showing confirm dialog
			}
			selectedItem := s.sessionsList.SelectedItem()
			if selectedItem != nil {
				selected := *selectedItem
				s.showingConfirm = true
				s.sessionToDelete = selected.Value().ID
			}
		case key.Matches(msg, s.keyMap.Close):
			return s, util.CmdHandler(dialogs.CloseDialogMsg{})
		default:
			u, cmd := s.sessionsList.Update(msg)
			s.sessionsList = u.(SessionsList)
			return s, cmd
		}
	}
	return s, nil
}

func (s *sessionDialogCmp) View() string {
	t := styles.CurrentTheme()
	
	if s.showingConfirm {
		return s.renderConfirmDialog()
	}
	
	listView := s.sessionsList.View()
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		t.S().Base.Padding(0, 1, 1, 1).Render(core.Title("Switch Session", s.width-4)),
		listView,
		"",
		t.S().Base.Width(s.width-2).PaddingLeft(1).AlignHorizontal(lipgloss.Left).Render(s.help.View(s.keyMap)),
	)

	return s.style().Render(content)
}

func (s *sessionDialogCmp) renderConfirmDialog() string {
	t := styles.CurrentTheme()
	baseStyle := t.S().Base
	
	// Get session title
	var sessionTitle string
	if selectedItem := s.sessionsList.SelectedItem(); selectedItem != nil {
		sessionTitle = (*selectedItem).FilterValue()
	}
	
	// Title
	titleView := core.Title("Delete Session", s.width-4)
	
	// Content
	explanation := t.S().Text.
		Width(s.width - 4).
		Render("Are you sure you want to delete this session? This action cannot be undone.")
	
	sessionText := t.S().Text.
		Width(s.width - 4).
		Foreground(t.FgMuted).
		Render("Session: " + sessionTitle)
	
	content := baseStyle.Render(lipgloss.JoinVertical(
		lipgloss.Left,
		explanation,
		"",
		sessionText,
	))
	
	// Buttons
	buttons := []core.ButtonOpts{
		{
			Text:           "No",
			UnderlineIndex: 0, // "N"
			Selected:       s.confirmSelected == 0,
		},
		{
			Text:           "Yes",
			UnderlineIndex: 0, // "Y"
			Selected:       s.confirmSelected == 1,
		},
	}
	
	buttonsView := core.SelectableButtons(buttons, "  ")
	buttonsContainer := baseStyle.AlignHorizontal(lipgloss.Right).Width(s.width - 4).Render(buttonsView)
	
	// Combine all parts
	dialogContent := lipgloss.JoinVertical(
		lipgloss.Top,
		titleView,
		"",
		content,
		"",
		buttonsContainer,
		"",
	)
	
	return s.style().Render(dialogContent)
}

func (s *sessionDialogCmp) Cursor() *tea.Cursor {
	if cursor, ok := s.sessionsList.(util.Cursor); ok {
		cursor := cursor.Cursor()
		if cursor != nil {
			cursor = s.moveCursor(cursor)
		}
		return cursor
	}
	return nil
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
	return SessionsDialogID
}
