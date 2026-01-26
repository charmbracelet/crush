// Package anthropic provides the dialog for Anthropic OAuth authentication.
package anthropic

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/oauth/anthropic"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/pkg/browser"
)

// AuthMode represents whether we're using API key or OAuth.
type AuthMode int

const (
	AuthModeAPIKey AuthMode = iota
	AuthModeOAuth
)

// DeviceFlowState represents the current state of the OAuth flow.
type DeviceFlowState int

const (
	DeviceFlowStateReady DeviceFlowState = iota
	DeviceFlowStateWaitingForCode
	DeviceFlowStateExchanging
	DeviceFlowStateSuccess
	DeviceFlowStateError
)

// AuthInitiatedMsg is sent when the auth is initiated successfully.
type AuthInitiatedMsg struct {
	AuthParams *anthropic.AuthParams
}

// DeviceFlowCompletedMsg is sent when the OAuth flow completes successfully.
type DeviceFlowCompletedMsg struct {
	Token *oauth.Token
}

// DeviceFlowErrorMsg is sent when the OAuth flow encounters an error.
type DeviceFlowErrorMsg struct {
	Error error
}

// APIKeySubmittedMsg is sent when the user submits an API key.
type APIKeySubmittedMsg struct {
	APIKey string
}

// DeviceFlow handles both API key and OAuth authentication for Anthropic.
type DeviceFlow struct {
	// Current mode
	mode  AuthMode
	State DeviceFlowState

	// Dimensions
	width int

	// API key mode
	apiKeyInput textinput.Model

	// OAuth mode
	authParams   *anthropic.AuthParams
	codeInput    textinput.Model
	token        *oauth.Token
	errorMessage string

	// Shared
	spinner spinner.Model

	// Key bindings
	tabKey key.Binding
}

// NewDeviceFlow creates a new device flow component.
func NewDeviceFlow() *DeviceFlow {
	t := styles.CurrentTheme()

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(t.GreenLight)

	// API key input
	apiKeyInput := textinput.New()
	apiKeyInput.Placeholder = "Enter your API key..."
	apiKeyInput.SetVirtualCursor(false)
	apiKeyInput.Prompt = "> "
	apiKeyInput.SetStyles(t.S().TextInput)
	apiKeyInput.SetWidth(50)

	// OAuth code input
	codeInput := textinput.New()
	codeInput.Placeholder = "Paste code#state here..."
	codeInput.SetVirtualCursor(false)
	codeInput.Prompt = "> "
	codeInput.SetStyles(t.S().TextInput)
	codeInput.SetWidth(50)

	return &DeviceFlow{
		mode:        AuthModeAPIKey,
		State:       DeviceFlowStateReady,
		spinner:     s,
		apiKeyInput: apiKeyInput,
		codeInput:   codeInput,
		tabKey: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "switch mode"),
		),
	}
}

// Init initializes the component.
func (d *DeviceFlow) Init() tea.Cmd {
	return d.apiKeyInput.Focus()
}

// Update handles messages and state transitions.
func (d *DeviceFlow) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Update spinner
	var spinnerCmd tea.Cmd
	d.spinner, spinnerCmd = d.spinner.Update(msg)
	if spinnerCmd != nil {
		cmds = append(cmds, spinnerCmd)
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		// Handle tab to switch modes
		if key.Matches(msg, d.tabKey) && d.State == DeviceFlowStateReady {
			if d.mode == AuthModeAPIKey {
				d.mode = AuthModeOAuth
				d.apiKeyInput.Blur()
				return d, nil
			} else {
				d.mode = AuthModeAPIKey
				return d, d.apiKeyInput.Focus()
			}
		}

	case AuthInitiatedMsg:
		d.authParams = msg.AuthParams
		d.State = DeviceFlowStateWaitingForCode
		cmds = append(cmds, d.codeInput.Focus())
		return d, tea.Batch(cmds...)

	case DeviceFlowCompletedMsg:
		d.State = DeviceFlowStateSuccess
		d.token = msg.Token
		return d, nil

	case DeviceFlowErrorMsg:
		d.State = DeviceFlowStateError
		d.errorMessage = msg.Error.Error()
		return d, nil
	}

	// Update the appropriate input based on mode and state
	if d.mode == AuthModeAPIKey && d.State == DeviceFlowStateReady {
		var inputCmd tea.Cmd
		d.apiKeyInput, inputCmd = d.apiKeyInput.Update(msg)
		if inputCmd != nil {
			cmds = append(cmds, inputCmd)
		}
	} else if d.mode == AuthModeOAuth && d.State == DeviceFlowStateWaitingForCode {
		var inputCmd tea.Cmd
		d.codeInput, inputCmd = d.codeInput.Update(msg)
		if inputCmd != nil {
			cmds = append(cmds, inputCmd)
		}
	}

	return d, tea.Batch(cmds...)
}

// View renders the component.
func (d *DeviceFlow) View() string {
	t := styles.CurrentTheme()

	primaryStyle := lipgloss.NewStyle().Foreground(t.Primary)
	greenStyle := lipgloss.NewStyle().Foreground(t.GreenLight)
	errorStyle := lipgloss.NewStyle().Foreground(t.Error)
	mutedStyle := lipgloss.NewStyle().Foreground(t.FgMuted)
	whiteStyle := lipgloss.NewStyle().Foreground(t.White)

	switch d.State {
	case DeviceFlowStateReady:
		if d.mode == AuthModeAPIKey {
			inputView := lipgloss.NewStyle().
				Margin(0, 1).
				Render(d.apiKeyInput.View())

			return inputView
		} else {
			// OAuth mode - ready to open browser
			instructions := lipgloss.NewStyle().
				Margin(0, 1).
				Width(d.width - 2).
				Render(
					whiteStyle.Render("Press ") +
						primaryStyle.Render("enter") +
						whiteStyle.Render(" to open your browser and sign in with\nyour Claude account."),
				)

			return instructions
		}

	case DeviceFlowStateWaitingForCode:
		instructions := lipgloss.NewStyle().
			Margin(0, 1).
			Width(d.width - 2).
			Render(
				whiteStyle.Render("After authorizing, copy the ") +
					primaryStyle.Render("code#state") +
					whiteStyle.Render(" shown and\npaste it below."),
			)

		var urlSection string
		if d.authParams != nil {
			uri := d.authParams.AuthURL
			displayURI := uri
			if len(displayURI) > 45 {
				displayURI = displayURI[:45] + "..."
			}
			link := lipgloss.NewStyle().
				Foreground(t.GreenDark).
				Underline(true).
				Hyperlink(uri, "id=anthropic-auth").
				Render(displayURI)
			urlSection = mutedStyle.
				Margin(1, 1, 0, 1).
				Width(d.width - 2).
				Render("Browser not opening? Go to:\n" + link)
		}

		inputView := lipgloss.NewStyle().
			Margin(1, 1, 0, 1).
			Render(d.codeInput.View())

		return lipgloss.JoinVertical(
			lipgloss.Left,
			instructions,
			urlSection,
			inputView,
		)

	case DeviceFlowStateExchanging:
		return lipgloss.NewStyle().
			Margin(0, 1).
			Render(
				greenStyle.Render(d.spinner.View()) +
					mutedStyle.Render("Exchanging code for tokens..."),
			)

	case DeviceFlowStateSuccess:
		return greenStyle.Margin(0, 1).Render("Authentication successful!")

	case DeviceFlowStateError:
		errMsg := "Authentication failed."
		if d.errorMessage != "" {
			errMsg = fmt.Sprintf("Authentication failed: %s", d.errorMessage)
		}
		return lipgloss.NewStyle().
			Margin(0, 1).
			Width(d.width - 2).
			Render(errorStyle.Render(errMsg))

	default:
		return ""
	}
}

// RadioView returns the radio toggle view for the title bar.
func (d *DeviceFlow) RadioView() string {
	t := styles.CurrentTheme()
	iconSelected := "◉"
	iconUnselected := "○"

	style := lipgloss.NewStyle().Foreground(t.FgHalfMuted)

	var content string
	if d.mode == AuthModeAPIKey {
		content = iconSelected + " API Key  " + iconUnselected + " Login with Claude"
	} else {
		content = iconUnselected + " API Key  " + iconSelected + " Login with Claude"
	}

	return style.Render(content)
}

// SetWidth sets the width of the dialog.
func (d *DeviceFlow) SetWidth(w int) {
	d.width = w
	d.apiKeyInput.SetWidth(w - 6)
	d.codeInput.SetWidth(w - 6)
}

// Cursor returns the cursor position for text input.
func (d *DeviceFlow) Cursor() *tea.Cursor {
	if d.mode == AuthModeAPIKey && d.State == DeviceFlowStateReady {
		cursor := d.apiKeyInput.Cursor()
		if cursor != nil {
			cursor.Y += 1
		}
		return cursor
	}
	if d.State == DeviceFlowStateWaitingForCode {
		cursor := d.codeInput.Cursor()
		if cursor != nil {
			// Adjust for instructions + url section + margins
			cursor.Y += 6
		}
		return cursor
	}
	return nil
}

// Submit handles the enter key based on current mode and state.
func (d *DeviceFlow) Submit() tea.Cmd {
	if d.mode == AuthModeAPIKey && d.State == DeviceFlowStateReady {
		// Submit API key
		apiKey := strings.TrimSpace(d.apiKeyInput.Value())
		if apiKey == "" {
			return util.ReportError(fmt.Errorf("please enter an API key"))
		}
		return func() tea.Msg {
			return APIKeySubmittedMsg{APIKey: apiKey}
		}
	}

	if d.mode == AuthModeOAuth && d.State == DeviceFlowStateReady {
		// Initiate OAuth flow
		return tea.Batch(d.spinner.Tick, d.initiateAuth)
	}

	if d.State == DeviceFlowStateWaitingForCode {
		// Submit OAuth code
		return d.submitCode()
	}

	return nil
}

// SubmitCode validates and submits the authorization code (for backwards compat).
func (d *DeviceFlow) SubmitCode() tea.Cmd {
	return d.Submit()
}

func (d *DeviceFlow) submitCode() tea.Cmd {
	input := strings.TrimSpace(d.codeInput.Value())
	if input == "" {
		return util.ReportError(fmt.Errorf("please enter the code"))
	}

	// Parse code#state format
	if !strings.Contains(input, "#") {
		return util.ReportError(fmt.Errorf("invalid format: expected code#state"))
	}

	parts := strings.SplitN(input, "#", 2)
	code := parts[0]
	state := parts[1]

	// Validate state
	if state != d.authParams.State {
		return util.ReportError(fmt.Errorf("state mismatch - please try again"))
	}

	d.State = DeviceFlowStateExchanging
	d.codeInput.Blur()
	return tea.Batch(d.spinner.Tick, d.exchangeCode(code, state))
}

// GetMode returns the current auth mode.
func (d *DeviceFlow) GetMode() AuthMode {
	return d.mode
}

// GetAPIKey returns the API key value.
func (d *DeviceFlow) GetAPIKey() string {
	return d.apiKeyInput.Value()
}

// GetToken returns the token after successful authentication.
func (d *DeviceFlow) GetToken() *oauth.Token {
	return d.token
}

// Cancel cancels the OAuth flow.
func (d *DeviceFlow) Cancel() {
	// Nothing to cancel for PKCE flow
}

// CopyCodeAndOpenURL opens the authorization URL in the browser.
func (d *DeviceFlow) CopyCodeAndOpenURL() tea.Cmd {
	return d.Submit()
}

// CopyCode is a no-op for Anthropic.
func (d *DeviceFlow) CopyCode() tea.Cmd {
	return nil
}

func (d *DeviceFlow) initiateAuth() tea.Msg {
	authParams, err := anthropic.InitiateAuth()
	if err != nil {
		return DeviceFlowErrorMsg{Error: fmt.Errorf("failed to initiate auth: %w", err)}
	}

	// Open browser
	_ = browser.OpenURL(authParams.AuthURL)

	return AuthInitiatedMsg{
		AuthParams: authParams,
	}
}

func (d *DeviceFlow) exchangeCode(code, state string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		token, err := anthropic.ExchangeCode(ctx, code, d.authParams.CodeVerifier, state)
		if err != nil {
			return DeviceFlowErrorMsg{Error: err}
		}

		return DeviceFlowCompletedMsg{Token: token}
	}
}
