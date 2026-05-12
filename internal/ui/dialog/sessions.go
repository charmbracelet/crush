package dialog

import (
	"context"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/util"
	uv "github.com/charmbracelet/ultraviolet"
)

// SessionsID is the identifier for the session selector dialog.
const SessionsID = "session"

type sessionsMode uint8

// Possible modes a session item can be in
const (
	sessionsModeNormal sessionsMode = iota
	sessionsModeDeleting
	sessionsModeUpdating
	sessionsModeArchiving
	sessionsModeUnarchiving
)

// Session is a session selector dialog.
type Session struct {
	com                 *common.Common
	help                help.Model
	list                *list.FilterableList
	input               textinput.Model
	selectedSessionInx  int
	sessions            []session.Session
	archivedSessions    []session.Session
	archivedStartIndex  int // index where archived section starts (-1 if no archived)

	sessionsMode sessionsMode

	keyMap struct {
		Select          key.Binding
		Next            key.Binding
		Previous        key.Binding
		UpDown          key.Binding
		Delete          key.Binding
		Rename          key.Binding
		Archive         key.Binding
		Unarchive       key.Binding
		ConfirmRename   key.Binding
		CancelRename    key.Binding
		ConfirmDelete   key.Binding
		CancelDelete    key.Binding
		ConfirmArchive  key.Binding
		CancelArchive   key.Binding
		ConfirmUnarchive key.Binding
		CancelUnarchive  key.Binding
		Close           key.Binding
	}
}

var _ Dialog = (*Session)(nil)

// NewSessions creates a new Session dialog.
func NewSessions(com *common.Common, selectedSessionID string) (*Session, error) {
	s := new(Session)
	s.sessionsMode = sessionsModeNormal
	s.com = com
	s.archivedStartIndex = -1

	sessions, err := com.Workspace.ListSessions(context.TODO())
	if err != nil {
		return nil, err
	}
	s.sessions = sessions

	archivedSessions, err := com.Workspace.ListArchivedSessions(context.TODO())
	if err != nil {
		// Non-fatal: just show active sessions
		archivedSessions = nil
	}
	s.archivedSessions = archivedSessions

	for i, sess := range sessions {
		if sess.ID == selectedSessionID {
			s.selectedSessionInx = i
			break
		}
	}

	help := help.New()
	help.Styles = com.Styles.DialogHelpStyles()

	s.help = help
	s.list = list.NewFilterableList(s.buildListItems(sessionsModeNormal)...)
	s.list.Focus()
	s.list.SetSelected(s.selectedSessionInx)

	s.input = textinput.New()
	s.input.SetVirtualCursor(false)
	s.input.Placeholder = "Enter session name"
	s.input.SetStyles(com.Styles.TextInput)
	s.input.Focus()

	s.keyMap.Select = key.NewBinding(
		key.WithKeys("enter", "tab", "ctrl+y"),
		key.WithHelp("enter", "choose"),
	)
	s.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("↓", "next item"),
	)
	s.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑", "previous item"),
	)
	s.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑↓", "choose"),
	)
	s.keyMap.Delete = key.NewBinding(
		key.WithKeys("ctrl+x"),
		key.WithHelp("ctrl+x", "delete"),
	)
	s.keyMap.Rename = key.NewBinding(
		key.WithKeys("ctrl+r"),
		key.WithHelp("ctrl+r", "rename"),
	)
	s.keyMap.Archive = key.NewBinding(
		key.WithKeys("ctrl+a"),
		key.WithHelp("ctrl+a", "archive"),
	)
	s.keyMap.Unarchive = key.NewBinding(
		key.WithKeys("ctrl+u"),
		key.WithHelp("ctrl+u", "unarchive"),
	)
	s.keyMap.ConfirmRename = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "confirm"),
	)
	s.keyMap.CancelRename = key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "cancel"),
	)
	s.keyMap.ConfirmDelete = key.NewBinding(
		key.WithKeys("y"),
		key.WithHelp("y", "delete"),
	)
	s.keyMap.CancelDelete = key.NewBinding(
		key.WithKeys("n", "esc"),
		key.WithHelp("n", "cancel"),
	)
	s.keyMap.ConfirmArchive = key.NewBinding(
		key.WithKeys("y"),
		key.WithHelp("y", "archive"),
	)
	s.keyMap.CancelArchive = key.NewBinding(
		key.WithKeys("n", "esc"),
		key.WithHelp("n", "cancel"),
	)
	s.keyMap.ConfirmUnarchive = key.NewBinding(
		key.WithKeys("y"),
		key.WithHelp("y", "unarchive"),
	)
	s.keyMap.CancelUnarchive = key.NewBinding(
		key.WithKeys("n", "esc"),
		key.WithHelp("n", "cancel"),
	)
	s.keyMap.Close = CloseKey

	return s, nil
}

// ID implements Dialog.
func (s *Session) ID() string {
	return SessionsID
}

// HandleMsg implements Dialog.
func (s *Session) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch s.sessionsMode {
		case sessionsModeDeleting:
			switch {
			case key.Matches(msg, s.keyMap.ConfirmDelete):
				action := s.confirmDeleteSession()
				s.list.SetItems(s.buildListItems(sessionsModeNormal)...)
				s.list.SelectFirst()
				s.list.ScrollToSelected()
				return action
			case key.Matches(msg, s.keyMap.CancelDelete):
				s.sessionsMode = sessionsModeNormal
				s.list.SetItems(s.buildListItems(sessionsModeNormal)...)
			}
		case sessionsModeUpdating:
			switch {
			case key.Matches(msg, s.keyMap.ConfirmRename):
				action := s.confirmRenameSession()
				s.list.SetItems(s.buildListItems(sessionsModeNormal)...)
				return action
			case key.Matches(msg, s.keyMap.CancelRename):
				s.sessionsMode = sessionsModeNormal
				s.list.SetItems(s.buildListItems(sessionsModeNormal)...)
			default:
				item := s.list.SelectedItem()
				if item == nil {
					return nil
				}
				if sessionItem, ok := item.(*SessionItem); ok {
					return sessionItem.HandleInput(msg)
				}
			}
		case sessionsModeArchiving:
			switch {
			case key.Matches(msg, s.keyMap.ConfirmArchive):
				action := s.confirmArchiveSession()
				s.list.SetItems(s.buildListItems(sessionsModeNormal)...)
				s.list.SelectFirst()
				s.list.ScrollToSelected()
				return action
			case key.Matches(msg, s.keyMap.CancelArchive):
				s.sessionsMode = sessionsModeNormal
				s.list.SetItems(s.buildListItems(sessionsModeNormal)...)
			}
		case sessionsModeUnarchiving:
			switch {
			case key.Matches(msg, s.keyMap.ConfirmUnarchive):
				action := s.confirmUnarchiveSession()
				s.list.SetItems(s.buildListItems(sessionsModeNormal)...)
				s.list.SelectFirst()
				s.list.ScrollToSelected()
				return action
			case key.Matches(msg, s.keyMap.CancelUnarchive):
				s.sessionsMode = sessionsModeNormal
				s.list.SetItems(s.buildListItems(sessionsModeNormal)...)
			}
		default:
			switch {
			case key.Matches(msg, s.keyMap.Close):
				return ActionClose{}
			case key.Matches(msg, s.keyMap.Rename):
				if s.isSelectedSeparator() || s.isSelectedArchived() {
					return nil
				}
				s.sessionsMode = sessionsModeUpdating
				s.list.SetItems(s.buildListItems(sessionsModeUpdating)...)
			case key.Matches(msg, s.keyMap.Delete):
				if s.isSelectedSeparator() {
					return nil
				}
				if s.isCurrentSessionBusy() {
					return ActionCmd{util.ReportWarn("Agent is busy, please wait...")}
				}
				s.sessionsMode = sessionsModeDeleting
				s.list.SetItems(s.buildListItems(sessionsModeDeleting)...)
			case key.Matches(msg, s.keyMap.Archive):
				if s.isSelectedSeparator() || s.isSelectedArchived() {
					return nil
				}
				if s.isCurrentSessionBusy() {
					return ActionCmd{util.ReportWarn("Agent is busy, please wait...")}
				}
				s.sessionsMode = sessionsModeArchiving
				s.list.SetItems(s.buildListItems(sessionsModeArchiving)...)
			case key.Matches(msg, s.keyMap.Unarchive):
				if !s.isSelectedArchived() {
					return nil
				}
				s.sessionsMode = sessionsModeUnarchiving
				s.list.SetItems(s.buildListItems(sessionsModeUnarchiving)...)
			case key.Matches(msg, s.keyMap.Previous):
				s.list.Focus()
				if s.list.IsSelectedFirst() {
					s.list.SelectLast()
				} else {
					s.list.SelectPrev()
				}
				// Skip separator
				if s.isSelectedSeparator() {
					s.list.SelectPrev()
				}
				s.list.ScrollToSelected()
			case key.Matches(msg, s.keyMap.Next):
				s.list.Focus()
				if s.list.IsSelectedLast() {
					s.list.SelectFirst()
				} else {
					s.list.SelectNext()
				}
				// Skip separator
				if s.isSelectedSeparator() {
					s.list.SelectNext()
				}
				s.list.ScrollToSelected()
			case key.Matches(msg, s.keyMap.Select):
				if s.isSelectedSeparator() {
					return nil
				}
				if item := s.list.SelectedItem(); item != nil {
					if sessionItem, ok := item.(*SessionItem); ok {
						return ActionSelectSession{sessionItem.Session}
					}
				}
			default:
				var cmd tea.Cmd
				s.input, cmd = s.input.Update(msg)
				value := s.input.Value()
				s.list.SetFilter(value)
				s.list.ScrollToTop()
				s.list.SetSelected(0)
				return ActionCmd{cmd}
			}
		}
	}
	return nil
}

// Cursor returns the cursor position relative to the dialog.
func (s *Session) Cursor() *tea.Cursor {
	return InputCursor(s.com.Styles, s.input.Cursor())
}

// Draw implements [Dialog].
func (s *Session) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := s.com.Styles
	width := max(0, min(defaultDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	height := max(0, min(defaultDialogHeight, area.Dy()-t.Dialog.View.GetVerticalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.InputPrompt.GetVerticalFrameSize() + inputContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()
	s.input.SetWidth(max(0, innerWidth-t.Dialog.InputPrompt.GetHorizontalFrameSize()-1)) // (1) cursor padding
	s.list.SetSize(innerWidth, height-heightOffset)
	s.help.SetWidth(innerWidth)

	// This makes it so we do not scroll the list if we don't have to
	start, end := s.list.VisibleItemIndices()

	// if selected index is outside visible range, scroll to it
	if s.selectedSessionInx < start || s.selectedSessionInx > end {
		s.list.ScrollToSelected()
	}

	var cur *tea.Cursor
	rc := NewRenderContext(t, width)
	rc.Title = "Sessions"
	switch s.sessionsMode {
	case sessionsModeDeleting:
		rc.TitleStyle = t.Dialog.Sessions.DeletingTitle
		rc.TitleGradientFromColor = t.Dialog.Sessions.DeletingTitleGradientFromColor
		rc.TitleGradientToColor = t.Dialog.Sessions.DeletingTitleGradientToColor
		rc.ViewStyle = t.Dialog.Sessions.DeletingView
		rc.AddPart(t.Dialog.Sessions.DeletingMessage.Render("Delete this session?"))
	case sessionsModeArchiving:
		rc.TitleStyle = t.Dialog.Sessions.ArchivingTitle
		rc.TitleGradientFromColor = t.Dialog.Sessions.ArchivingTitleGradientFromColor
		rc.TitleGradientToColor = t.Dialog.Sessions.ArchivingTitleGradientToColor
		rc.ViewStyle = t.Dialog.Sessions.ArchivingView
		rc.AddPart(t.Dialog.Sessions.ArchivingMessage.Render("Archive this session?"))
	case sessionsModeUpdating:
		rc.TitleStyle = t.Dialog.Sessions.RenamingingTitle
		rc.TitleGradientFromColor = t.Dialog.Sessions.RenamingTitleGradientFromColor
		rc.TitleGradientToColor = t.Dialog.Sessions.RenamingTitleGradientToColor
		rc.ViewStyle = t.Dialog.Sessions.RenamingView
		message := t.Dialog.Sessions.RenamingingMessage.Render("Rename this session?")
		rc.AddPart(message)
		item := s.selectedSessionItem()
		if item == nil {
			return nil
		}
		cur = item.Cursor()
		if cur == nil {
			break
		}

		start, end := s.list.VisibleItemIndices()
		selectedIndex := s.list.Selected()

		titleStyle := t.Dialog.Sessions.RenamingingTitle
		dialogStyle := t.Dialog.Sessions.RenamingView
		inputStyle := t.Dialog.InputPrompt

		// Adjust cursor position to account for dialog layout + message
		cur.X += inputStyle.GetBorderLeftSize() +
			inputStyle.GetMarginLeft() +
			inputStyle.GetPaddingLeft() +
			dialogStyle.GetBorderLeftSize() +
			dialogStyle.GetPaddingLeft() +
			dialogStyle.GetMarginLeft()
		cur.Y += titleStyle.GetVerticalFrameSize() +
			inputStyle.GetBorderTopSize() +
			inputStyle.GetMarginTop() +
			inputStyle.GetPaddingTop() +
			inputStyle.GetBorderBottomSize() +
			inputStyle.GetMarginBottom() +
			inputStyle.GetPaddingBottom() +
			dialogStyle.GetPaddingTop() +
			dialogStyle.GetBorderTopSize() +
			lipgloss.Height(message) - 1

		// move the cursor by one down until we see the selectedIndex
		for ; start <= end && start != selectedIndex && selectedIndex > -1; start++ {
			cur.Y += 1
		}
	default:
		inputView := t.Dialog.InputPrompt.Render(s.input.View())
		cur = s.Cursor()
		rc.AddPart(inputView)
	}
	listView := t.Dialog.List.Height(s.list.Height()).Render(s.list.Render())
	rc.AddPart(listView)
	rc.Help = s.help.View(s)

	view := rc.Render()

	DrawCenterCursor(scr, area, view, cur)
	return cur
}

func (s *Session) selectedSessionItem() *SessionItem {
	if item := s.list.SelectedItem(); item != nil {
		return item.(*SessionItem)
	}
	return nil
}

func (s *Session) confirmDeleteSession() Action {
	sessionItem := s.selectedSessionItem()
	s.sessionsMode = sessionsModeNormal
	if sessionItem == nil {
		return nil
	}

	s.removeSession(sessionItem.ID())
	return ActionCmd{s.deleteSessionCmd(sessionItem.ID())}
}

func (s *Session) removeSession(id string) {
	var newSessions []session.Session
	for _, sess := range s.sessions {
		if sess.ID == id {
			continue
		}
		newSessions = append(newSessions, sess)
	}
	s.sessions = newSessions
}

func (s *Session) deleteSessionCmd(id string) tea.Cmd {
	return func() tea.Msg {
		err := s.com.Workspace.DeleteSession(context.TODO(), id)
		if err != nil {
			return util.NewErrorMsg(err)
		}
		return nil
	}
}

func (s *Session) confirmArchiveSession() Action {
	sessionItem := s.selectedSessionItem()
	s.sessionsMode = sessionsModeNormal
	if sessionItem == nil {
		return nil
	}

	s.removeSession(sessionItem.ID())
	// Move to archived sessions
	s.archivedSessions = append([]session.Session{sessionItem.Session}, s.archivedSessions...)
	return ActionCmd{s.archiveSessionCmd(sessionItem.ID())}
}

func (s *Session) archiveSessionCmd(id string) tea.Cmd {
	return func() tea.Msg {
		err := s.com.Workspace.ArchiveSession(context.TODO(), id)
		if err != nil {
			return util.NewErrorMsg(err)
		}
		return nil
	}
}

func (s *Session) confirmUnarchiveSession() Action {
	sessionItem := s.selectedSessionItem()
	s.sessionsMode = sessionsModeNormal
	if sessionItem == nil {
		return nil
	}

	s.removeArchivedSession(sessionItem.ID())
	// Move to active sessions with updated timestamp
	sess := sessionItem.Session
	sess.UpdatedAt = time.Now().Unix()
	sess.ArchivedAt = 0
	s.sessions = append([]session.Session{sess}, s.sessions...)
	return ActionCmd{s.unarchiveSessionCmd(sessionItem.ID())}
}

func (s *Session) removeArchivedSession(id string) {
	var newSessions []session.Session
	for _, sess := range s.archivedSessions {
		if sess.ID == id {
			continue
		}
		newSessions = append(newSessions, sess)
	}
	s.archivedSessions = newSessions
}

func (s *Session) unarchiveSessionCmd(id string) tea.Cmd {
	return func() tea.Msg {
		err := s.com.Workspace.UnarchiveSession(context.TODO(), id)
		if err != nil {
			return util.NewErrorMsg(err)
		}
		return nil
	}
}

func (s *Session) confirmRenameSession() Action {
	sessionItem := s.selectedSessionItem()
	s.sessionsMode = sessionsModeNormal
	if sessionItem == nil {
		return nil
	}

	newTitle := strings.TrimSpace(sessionItem.InputValue())
	if newTitle == "" {
		return nil
	}
	session := sessionItem.Session
	session.Title = newTitle
	s.updateSession(session)
	return ActionCmd{s.updateSessionCmd(session)}
}

func (s *Session) updateSession(session session.Session) {
	for existingID, sess := range s.sessions {
		if sess.ID == session.ID {
			s.sessions[existingID] = session
			break
		}
	}
}

func (s *Session) updateSessionCmd(session session.Session) tea.Cmd {
	return func() tea.Msg {
		_, err := s.com.Workspace.SaveSession(context.TODO(), session)
		if err != nil {
			return util.NewErrorMsg(err)
		}
		return nil
	}
}

func (s *Session) isCurrentSessionBusy() bool {
	sessionItem := s.selectedSessionItem()
	if sessionItem == nil {
		return false
	}

	if !s.com.Workspace.AgentIsReady() {
		return false
	}

	return s.com.Workspace.AgentIsSessionBusy(sessionItem.ID())
}

// ShortHelp implements [help.KeyMap].
func (s *Session) ShortHelp() []key.Binding {
	switch s.sessionsMode {
	case sessionsModeDeleting:
		return []key.Binding{
			s.keyMap.ConfirmDelete,
			s.keyMap.CancelDelete,
		}
	case sessionsModeUpdating:
		return []key.Binding{
			s.keyMap.ConfirmRename,
			s.keyMap.CancelRename,
		}
	case sessionsModeArchiving:
		return []key.Binding{
			s.keyMap.ConfirmArchive,
			s.keyMap.CancelArchive,
		}
	case sessionsModeUnarchiving:
		return []key.Binding{
			s.keyMap.ConfirmUnarchive,
			s.keyMap.CancelUnarchive,
		}
	default:
		if s.isSelectedArchived() {
			return []key.Binding{
				s.keyMap.UpDown,
				s.keyMap.Unarchive,
				s.keyMap.Delete,
				s.keyMap.Select,
				s.keyMap.Close,
			}
		}
		return []key.Binding{
			s.keyMap.UpDown,
			s.keyMap.Rename,
			s.keyMap.Archive,
			s.keyMap.Delete,
			s.keyMap.Select,
			s.keyMap.Close,
		}
	}
}

// FullHelp implements [help.KeyMap].
func (s *Session) FullHelp() [][]key.Binding {
	m := [][]key.Binding{}
	slice := []key.Binding{
		s.keyMap.UpDown,
		s.keyMap.Rename,
		s.keyMap.Archive,
		s.keyMap.Delete,
		s.keyMap.Select,
		s.keyMap.Close,
	}

	switch s.sessionsMode {
	case sessionsModeDeleting:
		slice = []key.Binding{
			s.keyMap.ConfirmDelete,
			s.keyMap.CancelDelete,
		}
	case sessionsModeUpdating:
		slice = []key.Binding{
			s.keyMap.ConfirmRename,
			s.keyMap.CancelRename,
		}
	case sessionsModeArchiving:
		slice = []key.Binding{
			s.keyMap.ConfirmArchive,
			s.keyMap.CancelArchive,
		}
	case sessionsModeUnarchiving:
		slice = []key.Binding{
			s.keyMap.ConfirmUnarchive,
			s.keyMap.CancelUnarchive,
		}
	}
	for i := 0; i < len(slice); i += 4 {
		end := min(i+4, len(slice))
		m = append(m, slice[i:end])
	}
	return m
}

// buildListItems builds the list items including active sessions, separator, and archived sessions.
func (s *Session) buildListItems(mode sessionsMode) []list.FilterableItem {
	items := sessionItems(s.com.Styles, mode, s.sessions...)

	if len(s.archivedSessions) > 0 {
		// Add separator before archived sessions
		s.archivedStartIndex = len(items)
		items = append(items, NewSeparatorItem(s.com.Styles, "Archived"))

		// Add archived sessions
		archivedItems := sessionItems(s.com.Styles, mode, s.archivedSessions...)
		items = append(items, archivedItems...)
	} else {
		s.archivedStartIndex = -1
	}

	return items
}

// isSelectedArchived returns true if the currently selected item is an archived session.
func (s *Session) isSelectedArchived() bool {
	if s.archivedStartIndex < 0 {
		return false
	}
	selected := s.list.Selected()
	// Account for separator: archived items start at archivedStartIndex + 1
	return selected > s.archivedStartIndex
}

// isSelectedSeparator returns true if the currently selected item is the separator.
func (s *Session) isSelectedSeparator() bool {
	if s.archivedStartIndex < 0 {
		return false
	}
	return s.list.Selected() == s.archivedStartIndex
}
