package dialog

import (
	"context"
	"image"
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

const sessionDoubleClickThreshold = 400 * time.Millisecond

type sessionsMode uint8

// Possible modes a session item can be in
const (
	sessionsModeNormal sessionsMode = iota
	sessionsModeDeleting
	sessionsModeUpdating
)

// Session is a session selector dialog.
type Session struct {
	com                *common.Common
	help               help.Model
	list               *list.FilterableList
	input              textinput.Model
	selectedSessionInx int
	sessions           []session.Session

	sessionsMode  sessionsMode
	bodyArea      image.Rectangle
	mouseScrolled bool
	lastClickTime time.Time
	lastClickID   string

	keyMap struct {
		Select        key.Binding
		Next          key.Binding
		Previous      key.Binding
		UpDown        key.Binding
		Delete        key.Binding
		Rename        key.Binding
		ConfirmRename key.Binding
		CancelRename  key.Binding
		ConfirmDelete key.Binding
		CancelDelete  key.Binding
		Close         key.Binding
	}
}

var _ Dialog = (*Session)(nil)

// NewSessions creates a new Session dialog.
func NewSessions(com *common.Common, selectedSessionID string) (*Session, error) {
	s := new(Session)
	s.sessionsMode = sessionsModeNormal
	s.com = com
	sessions, err := com.Workspace.ListSessions(context.TODO())
	if err != nil {
		return nil, err
	}

	s.sessions = sessions
	for i, sess := range sessions {
		if sess.ID == selectedSessionID {
			s.selectedSessionInx = i
			break
		}
	}

	help := help.New()
	help.Styles = com.Styles.DialogHelpStyles()

	s.help = help
	s.list = list.NewFilterableList(sessionItems(com.Styles, sessionsModeNormal, sessions...)...)
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
				s.list.SetItems(sessionItems(s.com.Styles, sessionsModeNormal, s.sessions...)...)
				s.list.SelectFirst()
				s.list.ScrollToSelected()
				return action
			case key.Matches(msg, s.keyMap.CancelDelete):
				s.sessionsMode = sessionsModeNormal
				s.list.SetItems(sessionItems(s.com.Styles, sessionsModeNormal, s.sessions...)...)
			}
		case sessionsModeUpdating:
			switch {
			case key.Matches(msg, s.keyMap.ConfirmRename):
				action := s.confirmRenameSession()
				s.list.SetItems(sessionItems(s.com.Styles, sessionsModeNormal, s.sessions...)...)
				return action
			case key.Matches(msg, s.keyMap.CancelRename):
				s.sessionsMode = sessionsModeNormal
				s.list.SetItems(sessionItems(s.com.Styles, sessionsModeNormal, s.sessions...)...)
			default:
				item := s.list.SelectedItem()
				if item == nil {
					return nil
				}
				if sessionItem, ok := item.(*SessionItem); ok {
					return sessionItem.HandleInput(msg)
				}
			}
		default:
			switch {
			case key.Matches(msg, s.keyMap.Close):
				return ActionClose{}
			case key.Matches(msg, s.keyMap.Rename):
				s.sessionsMode = sessionsModeUpdating
				s.list.SetItems(sessionItems(s.com.Styles, sessionsModeUpdating, s.sessions...)...)
			case key.Matches(msg, s.keyMap.Delete):
				if s.isCurrentSessionBusy() {
					return ActionCmd{util.ReportWarn("Agent is busy, please wait...")}
				}
				s.sessionsMode = sessionsModeDeleting
				s.list.SetItems(sessionItems(s.com.Styles, sessionsModeDeleting, s.sessions...)...)
			case key.Matches(msg, s.keyMap.Previous):
				s.list.Focus()
				if s.list.IsSelectedFirst() {
					s.list.SelectLast()
				} else {
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
				s.list.ScrollToSelected()
			case key.Matches(msg, s.keyMap.Select):
				if item := s.list.SelectedItem(); item != nil {
					sessionItem := item.(*SessionItem)
					return ActionSelectSession{sessionItem.Session}
				}
			default:
				prevValue := s.input.Value()
				var cmd tea.Cmd
				s.input, cmd = s.input.Update(msg)
				value := s.input.Value()
				if value != prevValue {
					s.list.SetFilter(value)
					s.list.ScrollToTop()
					s.list.SetSelected(0)
				}
				return ActionCmd{cmd}
			}
		}
	case common.CoalescedWheelMsg:
		if image.Pt(msg.Mouse.X, msg.Mouse.Y).In(s.sessionListArea()) {
			s.list.ScrollBy(int(msg.DeltaY))
			s.mouseScrolled = true
		}
	case tea.MouseClickMsg:
		return s.handleMouseClick(msg)
	}
	return nil
}

func (s *Session) handleMouseClick(msg tea.MouseClickMsg) Action {
	if msg.Button != tea.MouseLeft || s.sessionsMode != sessionsModeNormal {
		s.resetMouseClick()
		return nil
	}
	area := s.sessionListArea()
	area.Max.X = min(area.Max.X, area.Min.X+s.list.Width())
	point := image.Pt(msg.X, msg.Y)
	if !point.In(area) {
		s.resetMouseClick()
		return nil
	}
	index, _ := s.list.ItemIndexAtPosition(point.X-area.Min.X, point.Y-area.Min.Y)
	if index < 0 {
		s.resetMouseClick()
		return nil
	}
	sessionItem := s.list.ItemAt(index).(*SessionItem)
	now := time.Now()
	if s.lastClickID == sessionItem.ID() && now.Sub(s.lastClickTime) <= sessionDoubleClickThreshold {
		s.resetMouseClick()
		return ActionSelectSession{sessionItem.Session}
	}
	s.lastClickTime = now
	s.lastClickID = sessionItem.ID()
	s.list.SetSelected(index)
	return nil
}

func (s *Session) resetMouseClick() {
	s.lastClickTime = time.Time{}
	s.lastClickID = ""
}

// Cursor returns the cursor position relative to the dialog.
func (s *Session) Cursor() *tea.Cursor {
	return InputCursor(s.com.Styles, s.input.Cursor())
}

// Draw implements [Dialog].
func (s *Session) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := s.com.Styles
	s.bodyArea = image.Rectangle{}
	width := max(0, min(defaultDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	height := max(0, min(defaultDialogHeight, area.Dy()-t.Dialog.View.GetVerticalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	s.input.SetWidth(dialogInputTextWidth(t, s.input, innerWidth))
	listHeight, listTotalHeight, listWidth := sizeDialogList(t, s.list, innerWidth, height)

	// Hide the timestamps uniformly when the widest would crowd the title.
	applyInfoColumnVisibility(s.list.FilteredItems(), listWidth, sessionInfoMaxPercent)

	// This makes it so we do not scroll the list if we don't have to
	start, end := s.list.VisibleItemIndices()

	// if selected index is outside visible range, scroll to it
	if !s.mouseScrolled && (s.selectedSessionInx < start || s.selectedSessionInx > end) {
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
	bodyView := t.Dialog.List.Height(s.list.Height()).Render(s.list.Render())
	bodyView = joinScrollbar(t, bodyView, listHeight, listTotalHeight, listHeight, s.list.Offset())
	rc.AddPart(bodyView)
	rc.Help = renderDialogHelp(t, &s.help, s, innerWidth)

	view := rc.Render()
	s.updateSessionListArea(area, view, bodyView, rc.Help, rc.ViewStyle, t.Dialog.List, innerWidth, listHeight)

	DrawCenterCursor(scr, area, view, cur)
	return cur
}

func (s *Session) updateSessionListArea(
	area uv.Rectangle,
	view string,
	bodyView string,
	helpView string,
	viewStyle lipgloss.Style,
	bodyStyle lipgloss.Style,
	bodyWidth int,
	bodyHeight int,
) {
	viewWidth, viewHeight := lipgloss.Size(view)
	dialogArea := common.CenterRect(area, min(viewWidth, area.Dx()), min(viewHeight, area.Dy()))
	bodyViewTop := dialogArea.Max.Y -
		viewStyle.GetMarginBottom() -
		viewStyle.GetBorderBottomSize() -
		viewStyle.GetPaddingBottom() -
		lipgloss.Height(helpView) -
		lipgloss.Height(bodyView)
	bodyMin := image.Pt(
		dialogArea.Min.X+
			viewStyle.GetMarginLeft()+
			viewStyle.GetBorderLeftSize()+
			viewStyle.GetPaddingLeft()+
			bodyStyle.GetMarginLeft()+
			bodyStyle.GetBorderLeftSize()+
			bodyStyle.GetPaddingLeft(),
		bodyViewTop+
			bodyStyle.GetMarginTop()+
			bodyStyle.GetBorderTopSize()+
			bodyStyle.GetPaddingTop(),
	)
	s.bodyArea = image.Rect(
		bodyMin.X,
		bodyMin.Y,
		bodyMin.X+bodyWidth,
		bodyMin.Y+bodyHeight,
	).Intersect(dialogArea).Intersect(area)
}

func (s *Session) sessionListArea() image.Rectangle {
	return s.bodyArea
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
	default:
		return []key.Binding{
			s.keyMap.UpDown,
			s.keyMap.Rename,
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
	}
	for i := 0; i < len(slice); i += 4 {
		end := min(i+4, len(slice))
		m = append(m, slice[i:end])
	}
	return m
}
