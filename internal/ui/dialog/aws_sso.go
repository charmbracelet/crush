package dialog

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/pkg/browser"
)

// AWSSSOID is the identifier for the AWS SSO auth dialog.
const AWSSSOID = "aws_sso"

// awsSSOState represents the current state of the AWS SSO flow.
type awsSSOState int

const (
	awsSSOStateWaiting awsSSOState = iota
	awsSSOStateSuccess
	awsSSOStateError
)

// AWSSSO displays the progress of an AWS SSO refresh. The command itself runs
// in the coordinator (so the refreshed credentials land where the model calls
// are made); this dialog is a pure view driven by agent notifications: it
// shows a spinner while the command runs, the verification URL once it
// appears, and the final success or error. The only local action it performs
// is opening the URL in the user's browser.
type AWSSSO struct {
	com *common.Common

	state   awsSSOState
	command string

	spinner spinner.Model
	help    help.Model
	keyMap  struct {
		Open  key.Binding
		Close key.Binding
	}

	width  int
	url    string
	errMsg string
}

var _ Dialog = (*AWSSSO)(nil)

// NewAWSSSO creates a new AWS SSO authentication dialog for the given refresh
// command. The dialog starts in the waiting state; SetURL and Finish drive it
// as the coordinator reports progress.
func NewAWSSSO(com *common.Common, command string) (*AWSSSO, tea.Cmd) {
	t := com.Styles

	m := &AWSSSO{
		com:     com,
		command: command,
		width:   0, // Set dynamically in Draw().
		state:   awsSSOStateWaiting,
	}

	m.spinner = spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(t.Dialog.OAuth.Spinner),
	)

	m.help = help.New()
	m.help.Styles = t.DialogHelpStyles()

	m.keyMap.Open = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "open in browser"),
	)
	m.keyMap.Close = CloseKey

	return m, m.spinner.Tick
}

// SetURL records the SSO verification URL so the dialog can display it and
// offer to open it in the browser.
func (m *AWSSSO) SetURL(url string) {
	if m.state == awsSSOStateWaiting {
		m.url = url
	}
}

// Finish transitions the dialog to its terminal state. A nil error means the
// refresh succeeded; a non-empty message means it failed.
func (m *AWSSSO) Finish(errMsg string) {
	if errMsg != "" {
		m.state = awsSSOStateError
		m.errMsg = errMsg
		return
	}
	m.state = awsSSOStateSuccess
}

// ID implements Dialog.
func (m *AWSSSO) ID() string {
	return AWSSSOID
}

// HandleMsg handles messages and state transitions.
func (m *AWSSSO) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if m.state == awsSSOStateWaiting {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			if cmd != nil {
				return ActionCmd{cmd}
			}
		}

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.Open):
			if m.state == awsSSOStateWaiting && m.url != "" {
				return ActionCmd{m.openURLCmd()}
			}
			if m.state == awsSSOStateSuccess {
				return ActionClose{}
			}

		case key.Matches(msg, m.keyMap.Close):
			return ActionClose{}
		}
	}

	return nil
}

// Draw renders the AWS SSO auth dialog.
func (m *AWSSSO) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	var (
		t           = m.com.Styles
		dialogWidth = max(0, min(60, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
		dialogStyle = t.Dialog.View.Width(dialogWidth)
	)
	m.width = dialogWidth
	view := dialogStyle.Render(m.dialogContent())
	DrawCenter(scr, area, view)
	return nil
}

func (m *AWSSSO) dialogContent() string {
	t := m.com.Styles
	innerWidth := m.width - t.Dialog.View.GetHorizontalFrameSize()

	elements := []string{
		m.headerContent(),
		m.innerDialogContent(),
		renderDialogHelp(t, &m.help, m, innerWidth),
	}
	return strings.Join(elements, "\n")
}

func (m *AWSSSO) headerContent() string {
	var (
		t            = m.com.Styles
		titleStyle   = t.Dialog.Title
		dialogStyle  = t.Dialog.View.Width(m.width)
		headerOffset = titleStyle.GetHorizontalFrameSize() + dialogStyle.GetHorizontalFrameSize()
		dialogTitle  = "AWS SSO Authentication"
	)
	return common.DialogTitle(t, titleStyle.Render(dialogTitle), m.width-headerOffset, t.Dialog.TitleGradFromColor, t.Dialog.TitleGradToColor)
}

func (m *AWSSSO) innerDialogContent() string {
	var (
		t                = m.com.Styles
		instructionStyle = t.Dialog.OAuth.Instructions
		enterKeyStyle    = t.Dialog.OAuth.Enter
		successStyle     = t.Dialog.OAuth.Success
		linkStyle        = t.Dialog.OAuth.Link
		errorStyle       = t.Dialog.OAuth.ErrorText
		statusTextStyle  = t.Dialog.OAuth.StatusText
	)

	// innerWidth is the dialog's content area (total minus the View
	// border). Every block sizes to this and uses padding for the inset, so
	// nothing is re-wrapped when the dialog frame renders it.
	innerWidth := m.width - t.Dialog.View.GetHorizontalFrameSize()

	switch m.state {
	case awsSSOStateWaiting:
		if m.url != "" {
			// URL found; show it and wait for auth to complete. Render each
			// text segment with its own style: wrapping the whole string in
			// one style would drop the text color after enterKeyStyle's
			// reset code.
			instructionText := instructionStyle.Render("Press ") +
				enterKeyStyle.Render("enter") +
				instructionStyle.Render(" to open the authorization page.")
			instructions := lipgloss.NewStyle().
				Width(innerWidth).
				Padding(0, 1).
				Render(instructionText)

			displayURL := ansi.Truncate(m.url, max(0, innerWidth-2), "…") // -2 for padding
			link := linkStyle.Hyperlink(m.url, "id=aws-sso-verify").Render(displayURL)
			urlBox := lipgloss.NewStyle().
				Width(innerWidth).
				Padding(0, 1).
				Align(lipgloss.Center).
				Render(link)

			waiting := statusTextStyle.
				Width(innerWidth).
				Padding(0, 1).
				Render(
					successStyle.Render(m.spinner.View()) +
						statusTextStyle.Render("Waiting for authentication..."),
				)

			return lipgloss.JoinVertical(
				lipgloss.Left,
				"",
				instructions,
				"",
				urlBox,
				"",
				waiting,
				"",
			)
		}

		// No URL yet; still waiting for command output.
		spinnerLine := statusTextStyle.
			Width(innerWidth).
			Padding(0, 1).
			Render(
				successStyle.Render(m.spinner.View()) +
					statusTextStyle.Render("Starting "+m.command+"..."),
			)
		return lipgloss.JoinVertical(lipgloss.Left, "", spinnerLine, "")

	case awsSSOStateSuccess:
		return successStyle.
			Width(innerWidth).
			Align(lipgloss.Center).
			Render("✓ Authentication successful!")

	case awsSSOStateError:
		header := errorStyle.
			Width(innerWidth).
			Padding(0, 1).
			Render("Authentication failed.")

		if m.errMsg == "" {
			return header
		}

		flattened := strings.Join(strings.Fields(strings.TrimSpace(m.errMsg)), " ")
		detail := statusTextStyle.
			Width(innerWidth).
			Padding(0, 1).
			Render(flattened)

		return lipgloss.JoinVertical(lipgloss.Left, "", header, "", detail, "")

	default:
		return ""
	}
}

// FullHelp returns the full help view.
func (m *AWSSSO) FullHelp() [][]key.Binding {
	return [][]key.Binding{m.ShortHelp()}
}

// ShortHelp returns the short help view.
func (m *AWSSSO) ShortHelp() []key.Binding {
	switch m.state {
	case awsSSOStateError:
		return []key.Binding{m.keyMap.Close}
	case awsSSOStateSuccess:
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("enter", "ctrl+y", "esc"),
				key.WithHelp("enter", "close"),
			),
		}
	default:
		if m.url != "" {
			return []key.Binding{m.keyMap.Open, m.keyMap.Close}
		}
		return []key.Binding{m.keyMap.Close}
	}
}

func (m *AWSSSO) openURLCmd() tea.Cmd {
	return func() tea.Msg {
		if m.url == "" {
			return nil
		}
		_ = browser.OpenURL(m.url)
		return nil
	}
}
