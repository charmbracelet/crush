package dialog

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
)

// SnippetID is the identifier for the snippet dialog.
const SnippetID = "snippet"

const (
	snippetMinWidth  = 40
	snippetMaxWidth  = 100
	snippetMinHeight = 8
	snippetMaxHeight = 20
)

// Snippet is a dialog that collects a multi-line code snippet from the user
// and inserts it into the chat editor wrapped in a fenced code block.
type Snippet struct {
	com    *common.Common
	editor textarea.Model
	keyMap struct {
		Submit  key.Binding
		Newline key.Binding
		Close   key.Binding
	}
	help help.Model
}

var _ Dialog = (*Snippet)(nil)

// NewSnippet creates a new snippet dialog.
func NewSnippet(com *common.Common) *Snippet {
	ta := textarea.New()
	ta.SetStyles(com.Styles.Editor.Textarea)
	ta.ShowLineNumbers = false
	ta.CharLimit = -1
	ta.SetVirtualCursor(false)
	ta.DynamicHeight = true
	ta.MinHeight = snippetMinHeight
	ta.MaxHeight = snippetMaxHeight
	ta.Placeholder = "Paste or type your code here…"
	ta.Focus()

	s := &Snippet{
		com:    com,
		editor: ta,
	}

	s.keyMap.Submit = key.NewBinding(
		key.WithKeys("ctrl+d"),
		key.WithHelp("ctrl+d", "insert"),
	)
	s.keyMap.Newline = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "newline"),
	)
	s.keyMap.Close = CloseKey

	s.help = help.New()
	s.help.Styles = com.Styles.DialogHelpStyles()

	return s
}

// ID implements Dialog.
func (s *Snippet) ID() string {
	return SnippetID
}

// HandleMsg implements Dialog.
func (s *Snippet) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, s.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, s.keyMap.Submit):
			content := s.editor.Value()
			if content == "" {
				return ActionClose{}
			}
			return ActionInsertSnippet{Content: content}
		default:
			var cmd tea.Cmd
			s.editor, cmd = s.editor.Update(msg)
			return ActionCmd{Cmd: cmd}
		}
	case tea.PasteMsg:
		var cmd tea.Cmd
		s.editor, cmd = s.editor.Update(msg)
		return ActionCmd{Cmd: cmd}
	}
	return nil
}

// Draw implements Dialog.
func (s *Snippet) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	st := s.com.Styles

	dialogW := min(snippetMaxWidth, max(snippetMinWidth, area.Dx()-st.Dialog.View.GetHorizontalFrameSize()-4))
	s.editor.SetWidth(dialogW - st.Dialog.View.GetHorizontalFrameSize())

	helpView := st.Dialog.HelpView.Width(dialogW - st.Dialog.View.GetHorizontalFrameSize()).Render(s.help.View(s))

	header := common.DialogTitle(st, "Paste Code Snippet", dialogW-st.Dialog.View.GetHorizontalFrameSize(), st.Dialog.TitleGradFromColor, st.Dialog.TitleGradToColor)

	editorView := s.editor.View()

	content := st.Dialog.Arguments.Content.Render(editorView)

	view := lipgloss.JoinVertical(
		lipgloss.Left,
		st.Dialog.Title.Render(header),
		content,
		helpView,
	)

	dialog := st.Dialog.View.Render(view)

	cur := s.cursor(lipgloss.Height(st.Dialog.Title.Render(header))+st.Dialog.View.GetVerticalPadding()/2, dialogW)
	DrawCenterCursor(scr, area, dialog, cur)
	return cur
}

// cursor returns the cursor position relative to the dialog content area.
func (s *Snippet) cursor(headerHeight, _ int) *tea.Cursor {
	cur := InputCursor(s.com.Styles, s.editor.Cursor())
	if cur == nil {
		return nil
	}
	cur.Y += headerHeight + 1
	return cur
}

// ShortHelp implements help.KeyMap.
func (s *Snippet) ShortHelp() []key.Binding {
	return []key.Binding{s.keyMap.Submit, s.keyMap.Close}
}

// FullHelp implements help.KeyMap.
func (s *Snippet) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{s.keyMap.Submit, s.keyMap.Newline, s.keyMap.Close},
	}
}
