package dialog

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/pinentry"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
)

// PinentryID is the identifier for the integrated pinentry dialog.
const PinentryID = "pinentry"

// Pinentry is the dialog that collects a GPG passphrase or security key
// PIN when the integrated pinentry needs a credential. Input is masked;
// the secret only ever leaves via [ActionPinentrySubmit].
type Pinentry struct {
	com   *common.Common
	req   pinentry.PromptRequest
	width int

	keyMap struct {
		Submit key.Binding
		Close  key.Binding
	}
	input textinput.Model
	help  help.Model
}

var _ Dialog = (*Pinentry)(nil)

// NewPinentry creates a dialog for the given credential request.
func NewPinentry(com *common.Common, req pinentry.PromptRequest) *Pinentry {
	m := Pinentry{}
	m.com = com
	m.req = req
	m.width = 0 // Set dynamically in Draw().

	m.input = textinput.New()
	m.input.SetVirtualCursor(false)
	m.input.Prompt = "> "
	m.input.Placeholder = "Type your passphrase..."
	if req.Kind == pinentry.KindPIN {
		m.input.Placeholder = "Type your PIN..."
	}
	m.input.EchoMode = textinput.EchoPassword
	m.input.EchoCharacter = '•'
	m.input.SetStyles(com.Styles.TextInput)
	m.input.Focus()

	m.help = help.New()
	m.help.Styles = com.Styles.DialogHelpStyles()

	m.keyMap.Submit = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "submit"),
	)
	m.keyMap.Close = CloseKey

	return &m
}

// ID implements Dialog.
func (m *Pinentry) ID() string {
	return PinentryID
}

// RequestID returns the ID of the prompt this dialog answers.
func (m *Pinentry) RequestID() string {
	return m.req.ID
}

// HandleMsg implements [Dialog].
func (m *Pinentry) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.Close):
			return ActionPinentryCancel{RequestID: m.req.ID}
		case key.Matches(msg, m.keyMap.Submit):
			if m.input.Value() == "" {
				return nil
			}
			return ActionPinentrySubmit{
				RequestID: m.req.ID,
				Secret:    m.input.Value(),
			}
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			if cmd != nil {
				return ActionCmd{cmd}
			}
		}
	case tea.PasteMsg:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		if cmd != nil {
			return ActionCmd{cmd}
		}
	}
	return nil
}

// Draw implements [Dialog].
func (m *Pinentry) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := m.com.Styles

	m.width = max(0, min(60, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	contentWidth := m.width - t.Dialog.View.GetHorizontalFrameSize()
	m.input.SetWidth(dialogInputTextWidth(t, m.input, contentWidth))

	// What is being unlocked comes before the input, the way a terminal
	// pinentry lays out its description.
	description := m.descriptionView(contentWidth)
	content := strings.Join([]string{
		m.headerView(),
		"",
		description,
		t.Dialog.InputPrompt.Render(m.input.View()),
		renderDialogHelp(t, &m.help, m, contentWidth),
	}, "\n")

	view := t.Dialog.View.Width(m.width).Render(content)
	cur := InputCursor(t, m.input.Cursor())
	if cur != nil {
		// InputCursor assumes the input sits right under the title.
		cur.Y += 1 + lipgloss.Height(description)
	}
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

func (m *Pinentry) headerView() string {
	var (
		t           = m.com.Styles
		titleStyle  = t.Dialog.Title
		textStyle   = t.Dialog.TitleText
		accentStyle = t.Dialog.TitleAccent
	)
	headerOffset := titleStyle.GetHorizontalFrameSize() + t.Dialog.View.GetHorizontalFrameSize()
	title := textStyle.Render("Enter your ") + accentStyle.Render(m.noun()) + textStyle.Render(".")
	return common.DialogTitle(t, titleStyle.Render(title), m.width-headerOffset, t.Dialog.TitleGradFromColor, t.Dialog.TitleGradToColor)
}

// descriptionView renders the prompt message and the key rows, wrapped
// to width so continuation lines keep their padding.
func (m *Pinentry) descriptionView(width int) string {
	t := m.com.Styles
	lines := []string{t.Dialog.PromptMessage.Width(width).Render(m.req.Prompt)}
	for _, line := range m.req.Key.Lines() {
		lines = append(lines, t.Dialog.PromptKey.Width(width).Render(line))
	}
	return strings.Join(lines, "\n")
}

// noun returns the credential noun for titles.
func (m *Pinentry) noun() string {
	if m.req.Kind == pinentry.KindPIN {
		return "Security Key PIN"
	}
	return "GPG Passphrase"
}

// FullHelp returns the full help view.
func (m *Pinentry) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{
			m.keyMap.Submit,
			m.keyMap.Close,
		},
	}
}

// ShortHelp returns the short help view.
func (m *Pinentry) ShortHelp() []key.Binding {
	return []key.Binding{
		m.keyMap.Submit,
		m.keyMap.Close,
	}
}
