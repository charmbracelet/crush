package dialog

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
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
	authBase

	state   awsSSOState
	command string

	keyMap struct {
		Open    key.Binding
		CopyURL key.Binding
	}

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
		authBase: newAuthBase(com, t.Dialog.OAuth.Spinner),
		command:  command,
		state:    awsSSOStateWaiting,
	}

	m.keyMap.Open = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "open in browser"),
	)
	// Same keys as the MCP dialog, which also has no code to copy.
	m.keyMap.CopyURL = key.NewBinding(
		key.WithKeys("c", "u"),
		key.WithHelp("c", "copy url"),
	)

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
			return m.tickSpinner(msg)
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

		case key.Matches(msg, m.keyMap.CopyURL):
			if m.state == awsSSOStateWaiting && m.url != "" {
				return ActionCmd{common.CopyToClipboard(m.url, "URL copied to clipboard")}
			}

		case key.Matches(msg, CloseKey):
			return ActionClose{}
		}
	}

	return nil
}

// Draw renders the AWS SSO auth dialog.
func (m *AWSSSO) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := m.com.Styles
	width, innerWidth := authWidths(t, area, authDialogMaxWidth)
	help := renderDialogHelp(t, &m.help, m, innerWidth)
	view, _ := authFrame(t, width, "AWS SSO Authentication", m.body(innerWidth), help, nil, false)
	DrawCenter(scr, area, view)
	return nil
}

// body renders the dialog's content for the current state, innerWidth
// cells wide.
func (m *AWSSSO) body(innerWidth int) string {
	t := m.com.Styles

	switch m.state {
	case awsSSOStateWaiting:
		if m.url != "" {
			// URL found; show it and wait for auth to complete.
			// Show the whole URL, broken at fixed widths: the user code
			// sits at its end. Each piece keeps the hyperlink.
			urlBox := authLinkBlock(t, innerWidth, m.url, "aws-sso-verify", "")

			waiting := authSpinnerLine(t, m.spinner, innerWidth, "Waiting for authentication...")

			return lipgloss.JoinVertical(
				lipgloss.Left,
				"",
				urlBox,
				"",
				waiting,
				"",
			)
		}

		// No URL yet; still waiting for command output.
		spinnerLine := authSpinnerLine(t, m.spinner, innerWidth, "Starting "+m.command+"...")
		return lipgloss.JoinVertical(lipgloss.Left, "", spinnerLine, "")

	case awsSSOStateSuccess:
		return authSuccessBlock(t, innerWidth)

	case awsSSOStateError:
		return authErrorBlock(t, innerWidth, m.errMsg)

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
		return []key.Binding{authEscKey(true)}
	case awsSSOStateSuccess:
		return []key.Binding{finishKey("close")}
	default:
		if m.url != "" {
			return []key.Binding{m.keyMap.Open, authEscKey(false), m.keyMap.CopyURL}
		}
		return []key.Binding{authEscKey(false)}
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
