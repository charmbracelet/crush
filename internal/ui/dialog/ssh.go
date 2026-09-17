package dialog

import (
	"cmp"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/sshaskpass"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
)

// SSHID is the identifier for the integrated SSH askpass dialog.
const SSHID = "ssh"

// SSH is the dialog that collects an SSH credential or confirmation
// when the integrated askpass needs one: an authentication password, a
// private key passphrase, or a confirmation (security key touch, host
// key acceptance). Secret input is masked; the value only ever leaves
// via [ActionSSHSubmit].
type SSH struct {
	com   *common.Common
	req   sshaskpass.PromptRequest
	width int

	// confirm reports that this prompt is a confirmation question
	// (security key touch, host key acceptance) rather than a secret;
	// enter submits "yes" without any input.
	confirm bool

	keyMap struct {
		Submit key.Binding
		Close  key.Binding
	}
	input textinput.Model
	help  help.Model
}

var _ Dialog = (*SSH)(nil)

// NewSSH creates a dialog for the given credential request.
func NewSSH(com *common.Common, req sshaskpass.PromptRequest) *SSH {
	m := SSH{}
	m.com = com
	m.req = req
	m.width = 0 // Set dynamically in Draw().
	m.confirm = req.Kind == sshaskpass.KindConfirm

	m.input = textinput.New()
	m.input.SetVirtualCursor(false)
	m.input.Placeholder = "Type your " + string(req.Kind) + "..."
	m.input.EchoMode = textinput.EchoPassword
	m.input.EchoCharacter = '•'
	m.input.SetStyles(com.Styles.TextInput)
	if !m.confirm {
		m.input.Focus()
	}

	m.help = help.New()
	m.help.Styles = com.Styles.DialogHelpStyles()

	m.keyMap.Submit = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", cmp.Or(submitLabel(req.Kind), "submit")),
	)
	m.keyMap.Close = CloseKey

	return &m
}

// ID implements Dialog.
func (m *SSH) ID() string {
	return SSHID
}

// HandleMsg implements [Dialog].
func (m *SSH) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.Close):
			return ActionSSHCancel{RequestID: m.req.ID}
		case key.Matches(msg, m.keyMap.Submit):
			if m.confirm {
				// Confirmations answer "yes"; OpenSSH treats any reply
				// that does not start with y as a decline.
				return ActionSSHSubmit{
					RequestID: m.req.ID,
					Secret:    "yes",
				}
			}
			if m.input.Value() == "" {
				return nil
			}
			return ActionSSHSubmit{
				RequestID: m.req.ID,
				Secret:    m.input.Value(),
			}
		default:
			if m.confirm {
				break
			}
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			if cmd != nil {
				return ActionCmd{cmd}
			}
		}
	case tea.PasteMsg:
		if m.confirm {
			break
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		if cmd != nil {
			return ActionCmd{cmd}
		}
	}
	return nil
}

// Draw implements [Dialog].
func (m *SSH) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := m.com.Styles

	m.width = max(0, min(60, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	innerWidth := m.width - t.Dialog.View.GetHorizontalFrameSize() - 2
	m.input.SetWidth(max(0, innerWidth-t.Dialog.InputPrompt.GetHorizontalFrameSize()-1)) // (1) cursor padding

	dialogStyle := t.Dialog.View.Width(m.width)
	inputStyle := t.Dialog.InputPrompt
	helpView := renderDialogHelp(t, &m.help, m, m.width-dialogStyle.GetHorizontalFrameSize())

	m.input.Prompt = "> "

	lines := []string{
		m.headerView(),
	}
	if m.confirm {
		lines = append(lines, m.descriptionView(), "", m.hintView())
	} else {
		lines = append(lines, inputStyle.Render(m.input.View()), m.descriptionView())
	}
	lines = append(lines, "", helpView)

	view := dialogStyle.Render(strings.Join(lines, "\n"))
	cur := InputCursor(t, m.input.Cursor())
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

func (m *SSH) headerView() string {
	var (
		t           = m.com.Styles
		titleStyle  = t.Dialog.Title
		textStyle   = t.Dialog.TitleText
		accentStyle = t.Dialog.TitleAccent
	)
	headerOffset := titleStyle.GetHorizontalFrameSize() + t.Dialog.View.GetHorizontalFrameSize()
	var title string
	if m.confirm {
		title = textStyle.Render("Confirm.")
	} else {
		title = textStyle.Render("Enter your ") + accentStyle.Render(m.noun()) + textStyle.Render(".")
	}
	return common.DialogTitle(t, titleStyle.Render(title), m.width-headerOffset, t.Dialog.TitleGradFromColor, t.Dialog.TitleGradToColor)
}

func (m *SSH) descriptionView() string {
	lines := []string{m.req.Prompt}
	if m.req.KeyInfo != "" {
		if m.req.Kind == sshaskpass.KindPassword {
			lines = append(lines, "Account: "+m.req.KeyInfo)
		} else {
			lines = append(lines, "Key: "+m.req.KeyInfo)
		}
	}
	return strings.Join(lines, "\n")
}

// hintView tells the user what a confirmation prompt is waiting for.
func (m *SSH) hintView() string {
	lower := strings.ToLower(m.req.Prompt)
	if strings.Contains(lower, "presence") || strings.Contains(lower, "touch") {
		return "Touch your security key, then press enter to continue."
	}
	return "Press enter to confirm, esc to cancel."
}

// noun returns the credential noun for titles.
func (m *SSH) noun() string {
	if m.req.Kind == sshaskpass.KindConfirm {
		return "Confirmation"
	}
	return "SSH Password"
}

// submitLabel returns the submit key help label for a credential kind.
func submitLabel(kind sshaskpass.Kind) string {
	if kind == sshaskpass.KindConfirm {
		return "confirm"
	}
	return ""
}

// FullHelp returns the full help view.
func (m *SSH) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{
			m.keyMap.Submit,
			m.keyMap.Close,
		},
	}
}

// ShortHelp returns the short help view.
func (m *SSH) ShortHelp() []key.Binding {
	return []key.Binding{
		m.keyMap.Submit,
		m.keyMap.Close,
	}
}
