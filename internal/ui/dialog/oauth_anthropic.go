package dialog

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/oauth/anthropic"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/uiutil"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/pkg/browser"
)

// OAuthAnthropicState represents the current state of the authorization code flow.
type OAuthAnthropicState int

const (
	OAuthAnthropicStateInitializing OAuthAnthropicState = iota
	OAuthAnthropicStateWaitingForCode
	OAuthAnthropicStateExchanging
	OAuthAnthropicStateSuccess
	OAuthAnthropicStateError
)

// OAuthAnthropicID is the identifier for the Anthropic OAuth dialog.
const OAuthAnthropicID = "oauth-anthropic"

// OAuthAnthropic handles the Anthropic authorization code OAuth flow.
type OAuthAnthropic struct {
	com          *common.Common
	isOnboarding bool

	provider  catwalk.Provider
	model     config.SelectedModel
	modelType config.SelectedModelType

	State OAuthAnthropicState

	spinner   spinner.Model
	codeInput textinput.Model
	help      help.Model
	keyMap    struct {
		Submit key.Binding
		Close  key.Binding
	}

	width        int
	authParams   *anthropic.AuthParams
	token        *oauth.Token
	errorMessage string
}

var _ Dialog = (*OAuthAnthropic)(nil)

// ActionAnthropicAuthInitialized is sent when auth params are ready.
type ActionAnthropicAuthInitialized struct {
	AuthParams *anthropic.AuthParams
	Error      error
}

// ActionAnthropicTokenExchanged is sent when token exchange completes.
type ActionAnthropicTokenExchanged struct {
	Token *oauth.Token
	Error error
}

// NewOAuthAnthropic creates a new Anthropic OAuth component.
func NewOAuthAnthropic(
	com *common.Common,
	isOnboarding bool,
	provider catwalk.Provider,
	model config.SelectedModel,
	modelType config.SelectedModelType,
) (*OAuthAnthropic, tea.Cmd) {
	t := com.Styles

	m := OAuthAnthropic{}
	m.com = com
	m.isOnboarding = isOnboarding
	m.provider = provider
	m.model = model
	m.modelType = modelType
	m.width = 60
	m.State = OAuthAnthropicStateInitializing

	m.spinner = spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(t.Base.Foreground(t.GreenLight)),
	)

	m.codeInput = textinput.New()
	m.codeInput.Placeholder = "code#state"
	m.codeInput.SetVirtualCursor(false)
	m.codeInput.Prompt = "> "
	m.codeInput.SetStyles(t.TextInput)
	m.codeInput.SetWidth(50)

	m.help = help.New()
	m.help.Styles = t.DialogHelpStyles()

	m.keyMap.Submit = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "submit"),
	)
	m.keyMap.Close = CloseKey

	return &m, tea.Batch(m.spinner.Tick, m.initiateAuth)
}

// ID implements Dialog.
func (m *OAuthAnthropic) ID() string {
	return OAuthAnthropicID
}

func (m *OAuthAnthropic) initiateAuth() tea.Msg {
	minimumWait := 500 * time.Millisecond
	startTime := time.Now()

	authParams, err := anthropic.InitiateAuth()

	elapsed := time.Since(startTime)
	if elapsed < minimumWait {
		time.Sleep(minimumWait - elapsed)
	}

	return ActionAnthropicAuthInitialized{
		AuthParams: authParams,
		Error:      err,
	}
}

func (m *OAuthAnthropic) exchangeCode(code, state string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		token, err := anthropic.ExchangeCode(ctx, code, m.authParams.CodeVerifier, state)
		return ActionAnthropicTokenExchanged{
			Token: token,
			Error: err,
		}
	}
}

// HandleMsg handles messages and state transitions.
func (m *OAuthAnthropic) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		switch m.State {
		case OAuthAnthropicStateInitializing, OAuthAnthropicStateExchanging:
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			if cmd != nil {
				return ActionCmd{cmd}
			}
		}

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.Submit):
			switch m.State {
			case OAuthAnthropicStateWaitingForCode:
				return m.submitCode()
			case OAuthAnthropicStateSuccess:
				return m.saveKeyAndContinue()
			}

		case key.Matches(msg, m.keyMap.Close):
			switch m.State {
			case OAuthAnthropicStateSuccess:
				return m.saveKeyAndContinue()
			default:
				return ActionClose{}
			}
		}

	case ActionAnthropicAuthInitialized:
		if msg.Error != nil {
			m.State = OAuthAnthropicStateError
			m.errorMessage = msg.Error.Error()
			return nil
		}
		m.authParams = msg.AuthParams
		m.State = OAuthAnthropicStateWaitingForCode
		// Open browser automatically (non-fatal, user can still manually open)
		_ = browser.OpenURL(m.authParams.AuthURL)
		return ActionCmd{m.codeInput.Focus()}

	case ActionAnthropicTokenExchanged:
		if msg.Error != nil {
			m.State = OAuthAnthropicStateError
			m.errorMessage = msg.Error.Error()
			return ActionCmd{uiutil.ReportError(msg.Error)}
		}
		m.token = msg.Token
		m.State = OAuthAnthropicStateSuccess
		return nil
	}

	// Update text input
	if m.State == OAuthAnthropicStateWaitingForCode {
		var cmd tea.Cmd
		m.codeInput, cmd = m.codeInput.Update(msg)
		if cmd != nil {
			return ActionCmd{cmd}
		}
	}

	return nil
}

func (m *OAuthAnthropic) submitCode() Action {
	input := strings.TrimSpace(m.codeInput.Value())
	if input == "" {
		return ActionCmd{uiutil.ReportError(fmt.Errorf("please enter the code"))}
	}

	// Parse code#state format
	if !strings.Contains(input, "#") {
		return ActionCmd{uiutil.ReportError(fmt.Errorf("invalid format: expected code#state"))}
	}

	parts := strings.SplitN(input, "#", 2)
	code := parts[0]
	state := parts[1]

	// Validate state
	if state != m.authParams.State {
		return ActionCmd{uiutil.ReportError(fmt.Errorf("state mismatch - please try again"))}
	}

	m.State = OAuthAnthropicStateExchanging
	m.codeInput.Blur()
	return ActionCmd{tea.Batch(m.spinner.Tick, m.exchangeCode(code, state))}
}

// Draw renders the dialog.
func (m *OAuthAnthropic) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	var (
		t           = m.com.Styles
		dialogStyle = t.Dialog.View.Width(m.width)
	)
	if m.isOnboarding {
		view := m.dialogContent()
		DrawOnboarding(scr, area, view)
	} else {
		view := dialogStyle.Render(m.dialogContent())
		DrawCenter(scr, area, view)
	}

	// Return cursor position for text input
	if m.State == OAuthAnthropicStateWaitingForCode {
		cur := m.codeInput.Cursor()
		return InputCursor(t, cur)
	}
	return nil
}

func (m *OAuthAnthropic) dialogContent() string {
	var (
		t         = m.com.Styles
		helpStyle = t.Dialog.HelpView
	)

	switch m.State {
	case OAuthAnthropicStateInitializing:
		return m.innerDialogContent()

	default:
		elements := []string{
			m.headerContent(),
			m.innerDialogContent(),
			helpStyle.Render(m.help.View(m)),
		}
		return strings.Join(elements, "\n")
	}
}

func (m *OAuthAnthropic) headerContent() string {
	var (
		t            = m.com.Styles
		titleStyle   = t.Dialog.Title
		textStyle    = t.Dialog.PrimaryText
		dialogStyle  = t.Dialog.View.Width(m.width)
		headerOffset = titleStyle.GetHorizontalFrameSize() + dialogStyle.GetHorizontalFrameSize()
		dialogTitle  = "Authenticate with Anthropic"
	)
	if m.isOnboarding {
		return textStyle.Render(dialogTitle)
	}
	return common.DialogTitle(t, titleStyle.Render(dialogTitle), m.width-headerOffset)
}

func (m *OAuthAnthropic) innerDialogContent() string {
	var (
		t            = m.com.Styles
		mutedStyle   = lipgloss.NewStyle().Foreground(t.FgMuted)
		greenStyle   = lipgloss.NewStyle().Foreground(t.GreenLight)
		errorStyle   = lipgloss.NewStyle().Foreground(t.Error)
		whiteStyle   = lipgloss.NewStyle().Foreground(t.White)
		primaryStyle = lipgloss.NewStyle().Foreground(t.Primary)
		linkStyle    = lipgloss.NewStyle().Foreground(t.GreenDark).Underline(true)
	)

	switch m.State {
	case OAuthAnthropicStateInitializing:
		return lipgloss.NewStyle().
			Margin(1, 1).
			Width(m.width - 2).
			Align(lipgloss.Center).
			Render(
				greenStyle.Render(m.spinner.View()) +
					mutedStyle.Render("Initializing..."),
			)

	case OAuthAnthropicStateWaitingForCode:
		instructions := lipgloss.NewStyle().
			Margin(0, 1).
			Width(m.width - 2).
			Render(
				whiteStyle.Render("A browser window should have opened.\nAfter authorizing, copy the ") +
					primaryStyle.Render("code#state") +
					whiteStyle.Render(" shown and paste it below."),
			)

		var url string
		if m.authParams != nil {
			link := linkStyle.Hyperlink(m.authParams.AuthURL, "id=anthropic-auth").Render("claude.ai/oauth/authorize...")
			url = mutedStyle.
				Margin(0, 1).
				Width(m.width - 2).
				Render("Browser not opening? Go to:\n" + link)
		}

		inputBox := lipgloss.NewStyle().
			Margin(0, 1).
			Render(m.codeInput.View())

		return lipgloss.JoinVertical(
			lipgloss.Left,
			"",
			instructions,
			"",
			url,
			"",
			inputBox,
			"",
		)

	case OAuthAnthropicStateExchanging:
		return lipgloss.NewStyle().
			Margin(1, 1).
			Width(m.width - 2).
			Align(lipgloss.Center).
			Render(
				greenStyle.Render(m.spinner.View()) +
					mutedStyle.Render("Exchanging code for tokens..."),
			)

	case OAuthAnthropicStateSuccess:
		return greenStyle.
			Margin(1).
			Width(m.width - 2).
			Render("Authentication successful!")

	case OAuthAnthropicStateError:
		errMsg := "Authentication failed."
		if m.errorMessage != "" {
			errMsg = fmt.Sprintf("Authentication failed: %s", m.errorMessage)
		}
		return lipgloss.NewStyle().
			Margin(1).
			Width(m.width - 2).
			Render(errorStyle.Render(errMsg))

	default:
		return ""
	}
}

// FullHelp returns the full help view.
func (m *OAuthAnthropic) FullHelp() [][]key.Binding {
	return [][]key.Binding{m.ShortHelp()}
}

// ShortHelp returns the short help view.
func (m *OAuthAnthropic) ShortHelp() []key.Binding {
	switch m.State {
	case OAuthAnthropicStateError:
		return []key.Binding{m.keyMap.Close}

	case OAuthAnthropicStateSuccess:
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("enter", "ctrl+y", "esc"),
				key.WithHelp("enter", "finish"),
			),
		}

	case OAuthAnthropicStateWaitingForCode:
		return []key.Binding{
			m.keyMap.Submit,
			m.keyMap.Close,
		}

	default:
		return []key.Binding{m.keyMap.Close}
	}
}

func (m *OAuthAnthropic) saveKeyAndContinue() Action {
	cfg := m.com.Config()

	// Prefix with "Bearer " so coordinator uses Authorization header
	bearerToken := "Bearer " + m.token.AccessToken
	m.token.AccessToken = bearerToken

	err := cfg.SetProviderAPIKey(string(m.provider.ID), m.token)
	if err != nil {
		return ActionCmd{uiutil.ReportError(fmt.Errorf("failed to save API key: %w", err))}
	}

	return ActionSelectModel{
		Provider:  m.provider,
		Model:     m.model,
		ModelType: m.modelType,
	}
}
