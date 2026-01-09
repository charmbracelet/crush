package dialog

import (
	"context"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
)

// SessionsID is the identifier for the session selector dialog.
const SessionsID = "session"

// Session is a session selector dialog.
type Session struct {
	width, height      int
	com                *common.Common
	help               help.Model
	list               *list.FilterableList
	input              textinput.Model
	selectedSessionInx int

	keyMap struct {
		Select   key.Binding
		Next     key.Binding
		Previous key.Binding
		UpDown   key.Binding
		Close    key.Binding
	}
}

var _ Dialog = (*Session)(nil)

// NewSessions creates a new Session dialog.
func NewSessions(com *common.Common, selectedSessionID string) (*Session, error) {
	s := new(Session)
	s.com = com
	sessions, err := com.App.Sessions.List(context.TODO())
	if err != nil {
		return nil, err
	}

	for i, sess := range sessions {
		if sess.ID == selectedSessionID {
			s.selectedSessionInx = i
			break
		}
	}

	help := help.New()
	help.Styles = com.Styles.DialogHelpStyles()

	s.help = help
	s.list = list.NewFilterableList(sessionItems(com.Styles, sessions...)...)
	s.list.Focus()

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
	s.keyMap.Close = CloseKey

	return s, nil
}

// SetSize sets the size of the dialog.
func (s *Session) SetSize(width, height int) {
	t := s.com.Styles
	s.width = width
	s.height = height
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + 1 + // (1) title content
		t.Dialog.InputPrompt.GetVerticalFrameSize() + 1 + // (1) input content
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()
	s.input.SetWidth(innerWidth - t.Dialog.InputPrompt.GetHorizontalFrameSize() - 1) // (1) cursor padding
	s.list.SetSize(innerWidth, height-heightOffset)
	s.help.SetWidth(width)

	// Now that we know the height we can select the selected session and scroll to it.
	s.list.SetSelected(s.selectedSessionInx)
	s.list.ScrollToSelected()
}

// ID implements Dialog.
func (s *Session) ID() string {
	return SessionsID
}

// Update implements Dialog.
func (s *Session) Update(msg tea.Msg) tea.Msg {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, s.keyMap.Close):
			return CloseMsg{}
		case key.Matches(msg, s.keyMap.Previous):
			s.list.Focus()
			if s.list.IsSelectedFirst() {
				s.list.SelectLast()
				s.list.ScrollToBottom()
				break
			}
			s.list.SelectPrev()
			s.list.ScrollToSelected()
		case key.Matches(msg, s.keyMap.Next):
			s.list.Focus()
			if s.list.IsSelectedLast() {
				s.list.SelectFirst()
				s.list.ScrollToTop()
				break
			}
			s.list.SelectNext()
			s.list.ScrollToSelected()
		case key.Matches(msg, s.keyMap.Select):
			if item := s.list.SelectedItem(); item != nil {
				sessionItem := item.(*SessionItem)
				return SessionSelectedMsg{sessionItem.Session}
			}
		default:
			var cmd tea.Cmd
			s.input, cmd = s.input.Update(msg)
			value := s.input.Value()
			s.list.SetFilter(value)
			s.list.ScrollToTop()
			s.list.SetSelected(0)
			if cmd != nil {
				return cmd()
			}
		}
	}
	return nil
}

// Cursor returns the cursor position relative to the dialog.
func (s *Session) Cursor() *tea.Cursor {
	return InputCursor(s.com.Styles, s.input.Cursor())
}

// View implements [Dialog].
func (s *Session) View() string {
	titleStyle := s.com.Styles.Dialog.Title
	dialogStyle := s.com.Styles.Dialog.View.Width(s.width)
	header := common.DialogTitle(s.com.Styles, "Switch Session",
		max(0, s.width-dialogStyle.GetHorizontalFrameSize()-
			titleStyle.GetHorizontalFrameSize()))

	return HeaderInputListHelpView(s.com.Styles, s.width, s.list.Height(), header,
		s.input.View(), s.list.Render(), s.help.View(s))
}

// ShortHelp implements [help.KeyMap].
func (s *Session) ShortHelp() []key.Binding {
	return []key.Binding{
		s.keyMap.UpDown,
		s.keyMap.Select,
		s.keyMap.Close,
	}
}

// FullHelp implements [help.KeyMap].
func (s *Session) FullHelp() [][]key.Binding {
	m := [][]key.Binding{}
	slice := []key.Binding{
		s.keyMap.Select,
		s.keyMap.Next,
		s.keyMap.Previous,
		s.keyMap.Close,
	}
	for i := 0; i < len(slice); i += 4 {
		end := min(i+4, len(slice))
		m = append(m, slice[i:end])
	}
	return m
}
