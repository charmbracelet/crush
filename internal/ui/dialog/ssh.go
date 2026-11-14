package dialog

import (
	"cmp"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
	m := SSH{com: com, req: req}
	m.width = 0 // Set dynamically in Draw().
	m.confirm = req.Kind == sshaskpass.KindConfirm

	m.input = textinput.New()
	m.input.SetVirtualCursor(false)
	m.input.Placeholder = "Type your password..."
	switch req.Title {
	case "SSH Passphrase":
		m.input.Placeholder = "Type your passphrase..."
	case "Security Key PIN":
		m.input.Placeholder = "Type your PIN..."
	}
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

// RequestID returns the ID of the prompt this dialog answers.
func (m *SSH) RequestID() string {
	return m.req.ID
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
	contentWidth := m.width - t.Dialog.View.GetHorizontalFrameSize()
	m.input.Prompt = "> "
	m.input.SetWidth(dialogInputTextWidth(t, m.input, contentWidth))

	// What OpenSSH is asking for comes before the input.
	description := m.descriptionView(contentWidth)
	lines := []string{m.headerView(), "", description}
	if m.confirm {
		lines = append(lines, "", m.hintView())
	} else {
		lines = append(lines, t.Dialog.InputPrompt.Render(m.input.View()))
	}
	lines = append(lines, renderDialogHelp(t, &m.help, m, contentWidth))

	view := t.Dialog.View.Width(m.width).Render(strings.Join(lines, "\n"))
	cur := InputCursor(t, m.input.Cursor())
	if cur != nil {
		// InputCursor assumes the input sits right under the title.
		cur.Y += 1 + lipgloss.Height(description)
	}
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
		title = textStyle.Render(cmp.Or(m.req.Title, "Confirm") + ".")
	} else {
		title = textStyle.Render("Enter your ") + accentStyle.Render(m.noun()) + textStyle.Render(".")
	}
	return common.DialogTitle(t, titleStyle.Render(title), m.width-headerOffset, t.Dialog.TitleGradFromColor, t.Dialog.TitleGradToColor)
}

// descriptionView renders the message and key row, wrapped to width so
// continuation lines keep their padding.
func (m *SSH) descriptionView(width int) string {
	t := m.com.Styles
	lines := []string{t.Dialog.SSHMessage.Width(width).Render(m.messageView())}
	if m.req.KeyInfo != "" {
		label := sshaskpass.KeyInfoLabel(m.req.Prompt, m.req.Kind)
		lines = append(lines, t.Dialog.SSHKey.Width(width).Render(label+": "+m.req.KeyInfo))
	}
	return strings.Join(lines, "\n")
}

// messageView returns the friendly description of what OpenSSH is
// asking for, falling back to the raw prompt text when the request
// carries no parsed copy.
func (m *SSH) messageView() string {
	if msg := strings.TrimSpace(m.req.Message); msg != "" {
		return msg
	}
	return strings.TrimRight(strings.TrimSpace(m.req.Prompt), ": ")
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
	if m.req.Title != "" {
		return m.req.Title
	}
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
