package dialog

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/v3/pkg/catwalk"
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
	// supportsCodeEntry reports whether the sign-in can finish with a
	// code pasted back from the authorization page instead of the
	// loopback redirect — for when the user declines the browser
	// permission or the browser cannot reach the callback.
	supportsCodeEntry() bool
	// submitCode exchanges a pasted code (or full callback URL) for a
	// token. Only called when supportsCodeEntry is true.
	submitCode(input string) tea.Cmd
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
		Copy      key.Binding
		CopyURL   key.Binding
		Submit    key.Binding
		FocusCode key.Binding
		Close     key.Binding
	}

	width           int
	deviceCode      string
	userCode        string
	verificationURL string
	expiresIn       int
	interval        int
	token           *oauth.Token
	cancelFunc      context.CancelFunc
	// codeInput is the paste field for providers whose browser flow
	// can finish with a code entered back in the terminal.
	codeInput textinput.Model
	// errMsg is the reason the sign-in failed, shown in the error state.
	errMsg string
	// codeInputRow is the paste field's row within the inner content,
	// recorded on render so the cursor lands on it.
	codeInputRow int
}

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
	m.keyMap.FocusCode = key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "enter code"),
	)
	// Esc abandons a sign-in in progress; once it has failed there is
	// nothing left to cancel, so the error state shows plain exit.
	m.keyMap.Close = cancelKey

	if m.oAuthProvider.supportsCodeEntry() {
		m.codeInput = textinput.New()
		m.codeInput.SetVirtualCursor(false)
		m.codeInput.Prompt = "> "
		m.codeInput.Placeholder = "Paste the code from the browser..."
		m.codeInput.SetStyles(t.TextInput)
		m.codeInput.Blur()
	}

	return &m, tea.Batch(m.spinner.Tick, m.oAuthProvider.initiateAuth)
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
		// The paste field swallows keys while focused so typing lands in
		// it: enter submits the pasted code, esc steps back to the
		// buttons.
		if m.codeEntryActive() && m.codeInput.Focused() {
			switch {
			case key.Matches(msg, m.keyMap.Submit):
				if value := m.codeInput.Value(); value != "" {
					m.codeInput.Blur()
					return ActionCmd{m.oAuthProvider.submitCode(value)}
				}
				cmd := m.copyCodeAndOpenURL()
				return ActionCmd{cmd}

			case key.Matches(msg, m.keyMap.Close):
				m.codeInput.Blur()
				return nil

			default:
				var cmd tea.Cmd
				m.codeInput, cmd = m.codeInput.Update(msg)
				if cmd != nil {
					return ActionCmd{cmd}
				}
				return nil
			}
		}

		switch {
		case m.codeEntryActive() && key.Matches(msg, m.keyMap.FocusCode):
			m.codeInput.Focus()
			return nil

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
				// Cancel the in-flight authorization so a dismissed dialog
				// does not leave a poller or loopback listener behind.
				return ActionCloseOAuth{Cmd: m.oAuthProvider.stopPolling}
			}
		}

	case ActionInitiateOAuth:
		m.deviceCode = msg.DeviceCode
		m.userCode = msg.UserCode
		m.expiresIn = msg.ExpiresIn
		m.verificationURL = msg.VerificationURL
		m.interval = msg.Interval
		m.State = OAuthStateDisplay
		return ActionCmd{m.oAuthProvider.startPolling(msg.DeviceCode, msg.ExpiresIn)}

	case ActionCompleteOAuth:
		// The device flow finished and we have a token. Immediately
		// persist it and fetch models in the background (this triggers a
		// config reload that can take a few seconds), showing a spinner.
		// The success screen is presented only once that work completes,
		// so it truthfully means "ready to use" rather than gating the
		// work behind a keypress.
		m.State = OAuthStateSaving
		m.token = msg.Token
		return ActionCmd{tea.Batch(
			m.oAuthProvider.stopPolling,
			m.spinner.Tick,
			m.saveCredential(),
		)}

	case ActionOAuthErrored:
		m.State = OAuthStateError
		m.errMsg = errText(msg.Error)
		cmd := tea.Batch(m.oAuthProvider.stopPolling, util.ReportError(msg.Error))
		return ActionCmd{cmd}

	case tea.PasteMsg:
		// Pasting into the dialog goes straight into the paste field,
		// no focus step needed.
		if m.codeEntryActive() {
			m.codeInput.Focus()
			var cmd tea.Cmd
			m.codeInput, cmd = m.codeInput.Update(msg)
			if cmd != nil {
				return ActionCmd{cmd}
			}
		}

	case oauthSaveDoneMsg:
		// Credential saved and models fetched. Present the confirmation
		// screen; the actual model selection happens when the user
		// acknowledges it (fast, since the work is already done).
		m.State = OAuthStateSuccess
		return nil

	case oauthSaveErrMsg:
		// Save failed; surface the error and move to the error state so
		// the user can dismiss and retry the flow.
		m.State = OAuthStateError
		m.errMsg = errText(msg.err)
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
		cur := m.cursor()
		if cur != nil {
			cur = adjustOnboardingInputCursor(t, cur)
			DrawOnboardingCursor(scr, area, view, cur)
			return cur
		}
		DrawOnboarding(scr, area, view)
	} else {
		view := dialogStyle.Render(m.dialogContent())
		cur := m.cursor()
		DrawCenterCursor(scr, area, view, cur)
		return cur
	}
	return nil
}

// cursor reports the paste field's cursor when it is focused, so the
// terminal caret sits in the input. It must run after dialogContent,
// which records where the field lands.
func (m *OAuth) cursor() *tea.Cursor {
	if !m.codeEntryActive() || !m.codeInput.Focused() {
		return nil
	}
	cur := m.codeInput.Cursor()
	if cur == nil {
		return nil
	}
	t := m.com.Styles
	frame := t.Dialog.View
	input := t.Dialog.InputPrompt
	cur.X += frame.GetMarginLeft() + frame.GetBorderLeftSize() + frame.GetPaddingLeft() +
		input.GetMarginLeft() + input.GetBorderLeftSize() + input.GetPaddingLeft()
	cur.Y += frame.GetMarginTop() + frame.GetBorderTopSize() + frame.GetPaddingTop() +
		lipgloss.Height(m.headerContent()) + m.codeInputRow
	return cur
}

func (m *OAuth) dialogContent() string {
	t := m.com.Styles

	innerWidth := m.width - t.Dialog.View.GetHorizontalFrameSize()
	elements := []string{
		m.headerContent(),
		m.innerDialogContent(),
		renderDialogHelp(t, &m.help, m, innerWidth),
	}
	return strings.Join(elements, "\n")
}

func (m *OAuth) headerContent() string {
	title := fmt.Sprintf("Authenticate with %s", m.oAuthProvider.name())
	return authTitle(m.com.Styles, m.width, title, m.isOnboarding)
}

func (m *OAuth) innerDialogContent() string {
	var (
		t               = m.com.Styles
		linkStyle       = t.Dialog.OAuth.Link
		statusTextStyle = t.Dialog.OAuth.StatusText
	)

	// innerWidth is the dialog's content area: total width minus the
	// View frame (border). Every block sizes to this so nothing gets
	// re-wrapped when the dialog frame renders it.
	innerWidth := m.width - t.Dialog.View.GetHorizontalFrameSize()

	switch m.State {
	case OAuthStateInitializing:
		status := authSpinnerLine(t, m.spinner, innerWidth, "Initializing...")
		return lipgloss.JoinVertical(lipgloss.Left, "", status, "")

	case OAuthStateDisplay:
		elements := []string{""}
		if m.userCode != "" {
			codeBox := lipgloss.NewStyle().
				Width(innerWidth).
				Height(7).
				Align(lipgloss.Center, lipgloss.Center).
				Background(t.Dialog.OAuth.UserCodeBg).
				Render(
					t.Dialog.OAuth.UserCode.Render(m.userCode),
				)
			elements = append(elements, codeBox, "")
		}

		link := wrappedLink(linkStyle, m.verificationURL, "id=oauth-verify", innerWidth-2) // -2 for padding
		url := statusTextStyle.
			Width(innerWidth).
			Padding(0, 1).
			Render("Browser not opening? Pay a visit to:\n" + link)

		waitingMsg := "Waiting for browser authentication..."
		if m.userCode != "" {
			waitingMsg = "Verifying..."
		}
		waiting := authSpinnerLine(t, m.spinner, innerWidth, waitingMsg)

		elements = append(elements, url, "", waiting, "")

		if m.codeEntryActive() {
			// The manual fallback: the authorization page shows a code
			// to paste when the redirect cannot come back on its own.
			inputStyle := t.Dialog.InputPrompt
			sizeDialogInput(t, &m.codeInput, innerWidth)
			elements = append(elements,
				statusTextStyle.
					Width(innerWidth).
					Padding(0, 1).
					Render("Declined or browser didn't connect? Enter the code from the page:"),
			)
			m.codeInputRow = lipgloss.Height(strings.Join(elements, "\n")) +
				inputStyle.GetMarginTop() + inputStyle.GetBorderTopSize() + inputStyle.GetPaddingTop()
			elements = append(elements, inputStyle.Render(m.codeInput.View()))
		}

		return lipgloss.JoinVertical(lipgloss.Left, elements...)

	case OAuthStateSuccess:
		return authSuccessBlock(t, innerWidth)

	case OAuthStateSaving:
		status := authSpinnerLine(t, m.spinner, innerWidth, "Fetching models...")
		// No keys work while saving, so the empty help row below
		// stands in for the trailing blank line.
		return lipgloss.JoinVertical(lipgloss.Left, "", status)

	case OAuthStateError:
		return authErrorBlock(t, innerWidth, m.errMsg)

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
		return []key.Binding{CloseKey}

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

	case OAuthStateInitializing:
		// Only esc does anything yet: it abandons the sign-in.
		return []key.Binding{m.keyMap.Close}

	default:
		if m.codeEntryActive() && m.codeInput.Focused() {
			return []key.Binding{
				key.NewBinding(
					key.WithKeys("enter", "ctrl+y"),
					key.WithHelp("enter", "submit code"),
				),
				key.NewBinding(
					key.WithKeys("esc", "alt+esc"),
					key.WithHelp("esc", "back"),
				),
			}
		}
		submit := m.keyMap.Submit
		if m.userCode == "" {
			submit = key.NewBinding(
				key.WithKeys("enter", "ctrl+y"),
				key.WithHelp("enter", "open browser"),
			)
		}
		// The main action and esc come first so a narrow help bar
		// truncates the optional hints rather than the way out.
		h := []key.Binding{submit, m.keyMap.Close}
		if m.codeEntryActive() {
			h = append(h, m.keyMap.FocusCode)
		}
		if m.userCode != "" {
			h = append(h, m.keyMap.Copy)
		}
		return append(h, m.keyMap.CopyURL)
	}
}

// codeEntryActive reports whether the paste field is shown: a browser
// flow in its display state with no device code in play (the device
// flow's code is only ever copied out, never pasted in).
func (m *OAuth) codeEntryActive() bool {
	return m.oAuthProvider.supportsCodeEntry() &&
		m.State == OAuthStateDisplay && m.userCode == ""
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
		// Browser flows have no code to copy; just open the URL.
		return func() tea.Msg {
			if err := browser.OpenURL(m.verificationURL); err != nil {
				return ActionOAuthErrored{fmt.Errorf("failed to open browser: %w", err)}
			}
			return nil
		}
	}
	return common.CopyToClipboardWithCallback(
		m.userCode,
		"Code copied and URL opened",
		func() tea.Msg {
			if err := browser.OpenURL(m.verificationURL); err != nil {
				return ActionOAuthErrored{fmt.Errorf("failed to open browser: %w", err)}
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
	)
	return func() tea.Msg {
		if err := com.Workspace.SetProviderAPIKey(config.ScopeGlobal, string(provider.ID), token); err != nil {
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

// errText flattens an error into one line of text for display, or ""
// when there is no error.
func errText(err error) string {
	if err == nil {
		return ""
	}
	return strings.Join(strings.Fields(err.Error()), " ")
}
