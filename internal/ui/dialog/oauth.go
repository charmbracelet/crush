package dialog

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/util"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/pkg/browser"
)

type OAuthProvider interface {
	name() string
	initiateAuth() tea.Msg
	startPolling(deviceCode string, expiresIn int) tea.Cmd
	stopPolling() tea.Msg
}

// OAuthState represents the current state of the device flow.
type OAuthState int

const (
	OAuthStateInitializing OAuthState = iota
	OAuthStateDisplay
	OAuthStateSuccess
	OAuthStateSaving
	OAuthStateError
)

// OAuthID is the identifier for the model selection dialog.
const OAuthID = "oauth"

// OAuth handles the OAuth flow authentication.
type OAuth struct {
	com          *common.Common
	isOnboarding bool

	provider      catwalk.Provider
	model         config.SelectedModel
	modelType     config.SelectedModelType
	oAuthProvider OAuthProvider

	State OAuthState

	spinner spinner.Model
	help    help.Model
	keyMap  struct {
		Copy    key.Binding
		CopyURL key.Binding
		Submit  key.Binding
		Close   key.Binding
	}

	width           int
	deviceCode      string
	userCode        string
	verificationURL string
	expiresIn       int
	interval        int
	token           *oauth.Token
	apiKey          string
	grantedScope    string
	attemptID       uint64
	cancelFunc      context.CancelFunc
}

var oauthAttemptID atomic.Uint64

var _ Dialog = (*OAuth)(nil)

// newOAuth creates a new device flow component.
func newOAuth(
	com *common.Common,
	isOnboarding bool,
	provider catwalk.Provider,
	model config.SelectedModel,
	modelType config.SelectedModelType,
	oAuthProvider OAuthProvider,
) (*OAuth, tea.Cmd) {
	t := com.Styles

	m := OAuth{}
	m.com = com
	m.isOnboarding = isOnboarding
	m.provider = provider
	m.model = model
	m.modelType = modelType
	m.oAuthProvider = oAuthProvider
	m.attemptID = oauthAttemptID.Add(1)
	m.width = 0 // Set dynamically in Draw().
	m.State = OAuthStateInitializing

	m.spinner = spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(t.Dialog.OAuth.Spinner),
	)

	m.help = help.New()
	m.help.Styles = t.DialogHelpStyles()

	m.keyMap.Copy = key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "copy code"),
	)
	m.keyMap.CopyURL = key.NewBinding(
		key.WithKeys("u"),
		key.WithHelp("u", "copy url"),
	)
	m.keyMap.Submit = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "copy & open"),
	)
	m.keyMap.Close = CloseKey

	return &m, tea.Batch(m.spinner.Tick, m.wrapOAuthCmd(m.oAuthProvider.initiateAuth))
}

func (m *OAuth) wrapOAuthCmd(cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		msg := cmd()
		switch value := msg.(type) {
		case ActionInitiateOAuth:
			value.AttemptID = m.attemptID
			return value
		case ActionCompleteOAuth:
			value.AttemptID = m.attemptID
			return value
		case ActionOAuthErrored:
			value.AttemptID = m.attemptID
			return value
		default:
			return msg
		}
	}
}

// ID implements Dialog.
func (m *OAuth) ID() string {
	return OAuthID
}

// HandleMsg handles messages and state transitions.
func (m *OAuth) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		switch m.State {
		case OAuthStateInitializing, OAuthStateDisplay, OAuthStateSaving:
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			if cmd != nil {
				return ActionCmd{cmd}
			}
		}

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.Copy):
			cmd := m.copyCode()
			return ActionCmd{cmd}

		case key.Matches(msg, m.keyMap.CopyURL):
			cmd := m.copyURL()
			return ActionCmd{cmd}

		case key.Matches(msg, m.keyMap.Submit):
			switch m.State {
			case OAuthStateSuccess:
				return m.confirmAndSelectModel()

			case OAuthStateSaving:
				// Save in progress; ignore submits until it finishes.
				return nil

			default:
				cmd := m.copyCodeAndOpenURL()
				return ActionCmd{cmd}
			}

		case key.Matches(msg, m.keyMap.Close):
			switch m.State {
			case OAuthStateSuccess:
				return m.confirmAndSelectModel()

			case OAuthStateSaving:
				// Save in progress; ignore submits until it finishes.
				return nil

			default:
				return ActionClose{Cmd: m.oAuthProvider.stopPolling}
			}
		}

	case ActionInitiateOAuth:
		if msg.AttemptID != m.attemptID {
			return nil
		}
		m.deviceCode = msg.DeviceCode
		m.userCode = msg.UserCode
		m.expiresIn = msg.ExpiresIn
		m.verificationURL = msg.VerificationURL
		m.interval = msg.Interval
		m.State = OAuthStateDisplay
		return ActionCmd{m.wrapOAuthCmd(m.oAuthProvider.startPolling(msg.DeviceCode, msg.ExpiresIn))}

	case ActionCompleteOAuth:
		if msg.AttemptID != m.attemptID {
			return nil
		}
		// The device flow finished and we have a token. Immediately
		// persist it and fetch models in the background (this triggers a
		// config reload that can take a few seconds), showing a spinner.
		// The success screen is presented only once that work completes,
		// so it truthfully means "ready to use" rather than gating the
		// work behind a keypress.
		m.State = OAuthStateSaving
		m.token = msg.Token
		m.apiKey = msg.APIKey
		m.grantedScope = msg.Scope
		return ActionCmd{tea.Batch(
			m.oAuthProvider.stopPolling,
			m.spinner.Tick,
			m.saveCredential(),
		)}

	case ActionOAuthErrored:
		if msg.AttemptID != m.attemptID {
			return nil
		}
		m.State = OAuthStateError
		cmd := tea.Batch(m.oAuthProvider.stopPolling, util.ReportError(msg.Error))
		return ActionCmd{cmd}

	case oauthSaveDoneMsg:
		// Credential saved and models fetched. Present the confirmation
		// screen; the actual model selection happens when the user
		// acknowledges it (fast, since the work is already done).
		m.State = OAuthStateSuccess
		m.apiKey = ""
		m.token = nil
		return nil

	case oauthSaveErrMsg:
		// Save failed; surface the error and move to the error state so
		// the user can dismiss and retry the flow.
		m.State = OAuthStateError
		return ActionCmd{util.ReportError(msg.err)}
	}
	return nil
}

// oauthSaveDoneMsg is emitted by the background save command once the
// credential has been persisted and models fetched. The model-selection
// details are read from the dialog's own fields when the user confirms.
type oauthSaveDoneMsg struct{}

// oauthSaveErrMsg is emitted by the background save command when persisting
// the credential fails.
type oauthSaveErrMsg struct {
	err error
}

// View renders the device flow dialog.
func (m *OAuth) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	var (
		t           = m.com.Styles
		dialogWidth = max(0, min(60, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
		dialogStyle = t.Dialog.View.Width(dialogWidth)
	)
	m.width = dialogWidth
	if m.isOnboarding {
		view := m.dialogContent()
		DrawOnboarding(scr, area, view)
	} else {
		view := dialogStyle.Render(m.dialogContent())
		DrawCenter(scr, area, view)
	}
	return nil
}

func (m *OAuth) dialogContent() string {
	t := m.com.Styles

	switch m.State {
	case OAuthStateInitializing, OAuthStateSaving:
		return m.innerDialogContent()

	default:
		innerWidth := m.width - t.Dialog.View.GetHorizontalFrameSize()
		elements := []string{
			m.headerContent(),
			m.innerDialogContent(),
			renderDialogHelp(t, &m.help, m, innerWidth),
		}
		return strings.Join(elements, "\n")
	}
}

func (m *OAuth) headerContent() string {
	var (
		t            = m.com.Styles
		titleStyle   = t.Dialog.Title
		textStyle    = t.Dialog.PrimaryText
		dialogStyle  = t.Dialog.View.Width(m.width)
		headerOffset = titleStyle.GetHorizontalFrameSize() + dialogStyle.GetHorizontalFrameSize()
		dialogTitle  = fmt.Sprintf("Let’s authenticate with %s", m.oAuthProvider.name())
	)
	if m.isOnboarding {
		return textStyle.Render(dialogTitle)
	}
	return common.DialogTitle(t, titleStyle.Render(dialogTitle), m.width-headerOffset, t.Dialog.TitleGradFromColor, t.Dialog.TitleGradToColor)
}

func (m *OAuth) innerDialogContent() string {
	var (
		t                = m.com.Styles
		instructionStyle = t.Dialog.OAuth.Instructions
		enterKeyStyle    = t.Dialog.OAuth.Enter
		successStyle     = t.Dialog.OAuth.Success
		linkStyle        = t.Dialog.OAuth.Link
		errorStyle       = t.Dialog.OAuth.ErrorText
		statusTextStyle  = t.Dialog.OAuth.StatusText
	)

	// innerWidth is the dialog's content area: total width minus the
	// View frame (border). Every block sizes to this so nothing gets
	// re-wrapped when the dialog frame renders it.
	innerWidth := m.width - t.Dialog.View.GetHorizontalFrameSize()

	switch m.State {
	case OAuthStateInitializing:
		return lipgloss.NewStyle().
			Width(innerWidth).
			Align(lipgloss.Center).
			Render(
				successStyle.Render(m.spinner.View()) +
					statusTextStyle.Render("Initializing..."),
			)

	case OAuthStateDisplay:
		if m.userCode == "" {
			instructionText := instructionStyle.Render("Press ") +
				enterKeyStyle.Render("enter") +
				instructionStyle.Render(" to open the browser and authorize Crush.")
			instructions := lipgloss.NewStyle().
				Width(innerWidth).
				Padding(0, 1).
				Render(instructionText)
			link := linkStyle.Hyperlink(m.verificationURL, "id=oauth-verify").Render(m.verificationURL)
			url := statusTextStyle.
				Width(innerWidth).
				Padding(0, 1).
				Render("Browser not opening? Copy this URL:\n" + link)
			waiting := statusTextStyle.
				Width(innerWidth).
				Padding(0, 1).
				Render(successStyle.Render(m.spinner.View()) + statusTextStyle.Render("Waiting for authorization..."))
			return lipgloss.JoinVertical(lipgloss.Left, "", instructions, "", url, "", waiting, "")
		}

		// Render each text segment with its own style. Wrapping the
		// whole concatenation in a single style would lose the text
		// color after enterKeyStyle's reset code.
		instructionText := instructionStyle.Render("Press ") +
			enterKeyStyle.Render("enter") +
			instructionStyle.Render(" to copy the code below and open the browser.")
		instructions := lipgloss.NewStyle().
			Width(innerWidth).
			Padding(0, 1).
			Render(instructionText)

		codeBox := lipgloss.NewStyle().
			Width(innerWidth).
			Height(7).
			Align(lipgloss.Center, lipgloss.Center).
			Background(t.Dialog.OAuth.UserCodeBg).
			Render(
				t.Dialog.OAuth.UserCode.Render(m.userCode),
			)

		link := linkStyle.Hyperlink(m.verificationURL, "id=oauth-verify").Render(m.verificationURL)
		url := statusTextStyle.
			Width(innerWidth).
			Padding(0, 1).
			Render("Browser not opening? Pay a visit to:\n" + link)

		waiting := statusTextStyle.
			Width(innerWidth).
			Padding(0, 1).
			Render(
				successStyle.Render(m.spinner.View()) + statusTextStyle.Render("Verifying..."),
			)

		return lipgloss.JoinVertical(
			lipgloss.Left,
			"",
			instructions,
			"",
			codeBox,
			"",
			url,
			"",
			waiting,
			"",
		)

	case OAuthStateSuccess:
		message := "Authentication successful!"
		if m.grantedScope != "" {
			message = fmt.Sprintf("Authentication successful (scope: %s)!", m.grantedScope)
		}
		return successStyle.
			Width(innerWidth).
			Padding(1).
			Render(message)

	case OAuthStateSaving:
		return lipgloss.NewStyle().
			Width(innerWidth).
			Align(lipgloss.Center).
			Render(
				successStyle.Render(m.spinner.View()) +
					statusTextStyle.Render(" Fetching models..."),
			)

	case OAuthStateError:
		return errorStyle.
			Width(innerWidth).
			Padding(1).
			Render("Authentication failed.")

	default:
		return ""
	}
}

// FullHelp returns the full help view.
func (m *OAuth) FullHelp() [][]key.Binding {
	return [][]key.Binding{m.ShortHelp()}
}

// ShortHelp returns the full help view.
func (m *OAuth) ShortHelp() []key.Binding {
	switch m.State {
	case OAuthStateError:
		return []key.Binding{m.keyMap.Close}

	case OAuthStateSuccess:
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("enter", "ctrl+y", "esc"),
				key.WithHelp("enter", "finish"),
			),
		}

	case OAuthStateSaving:
		// No actionable keys while the save completes.
		return nil

	default:
		result := []key.Binding{m.keyMap.CopyURL, m.keyMap.Submit, m.keyMap.Close}
		if m.userCode != "" {
			result = append([]key.Binding{m.keyMap.Copy}, result...)
		}
		return result
	}
}

func (m *OAuth) copyCode() tea.Cmd {
	if m.State != OAuthStateDisplay || m.userCode == "" {
		return nil
	}
	return common.CopyToClipboard(m.userCode, "Code copied to clipboard")
}

func (m *OAuth) copyURL() tea.Cmd {
	if m.State != OAuthStateDisplay {
		return nil
	}
	return common.CopyToClipboard(m.verificationURL, "URL copied to clipboard")
}

func (m *OAuth) copyCodeAndOpenURL() tea.Cmd {
	if m.State != OAuthStateDisplay {
		return nil
	}
	if m.userCode == "" {
		return func() tea.Msg {
			if err := browser.OpenURL(m.verificationURL); err != nil {
				return ActionOAuthErrored{AttemptID: m.attemptID, Error: fmt.Errorf("failed to open browser: %w", err)}
			}
			return nil
		}
	}
	return common.CopyToClipboardWithCallback(
		m.userCode,
		"Code copied and URL opened",
		func() tea.Msg {
			if err := browser.OpenURL(m.verificationURL); err != nil {
				return ActionOAuthErrored{AttemptID: m.attemptID, Error: fmt.Errorf("failed to open browser: %w", err)}
			}
			return nil
		},
	)
}

// saveCredential returns a command that persists the OAuth token and
// triggers the config reload (including model discovery) off the UI update
// loop. It reports completion via oauthSaveDoneMsg or oauthSaveErrMsg.
func (m *OAuth) saveCredential() tea.Cmd {
	// Capture the fields the command needs so it does not race with
	// dialog state.
	var (
		com      = m.com
		provider = m.provider
		token    = m.token
		apiKey   = m.apiKey
	)
	return func() tea.Msg {
		credential := any(token)
		if apiKey != "" {
			credential = apiKey
		}
		if credential == nil {
			return oauthSaveErrMsg{err: fmt.Errorf("authentication returned no credential")}
		}
		if err := com.Workspace.SetProviderAPIKey(config.ScopeGlobal, string(provider.ID), credential); err != nil {
			return oauthSaveErrMsg{err: fmt.Errorf("failed to save API key: %w", err)}
		}
		return oauthSaveDoneMsg{}
	}
}

// confirmAndSelectModel is invoked when the user acknowledges the success
// screen. The credential is already saved, so this only resumes model
// selection, which closes the dialog.
func (m *OAuth) confirmAndSelectModel() Action {
	return ActionSelectModel{
		Provider:  m.provider,
		Model:     m.model,
		ModelType: m.modelType,
	}
}
