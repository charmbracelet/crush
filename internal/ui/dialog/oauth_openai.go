package dialog

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/oauth"
	openaioauth "github.com/charmbracelet/crush/internal/oauth/openai"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/uiutil"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/pkg/browser"
)

type openAICodexAuthInitMsg struct {
	flow   openaioauth.AuthFlow
	server *openaioauth.LocalServer
	err    error
}

type openAICodexAuthTokenMsg struct {
	token *oauth.Token
}

type openAICodexAuthErrorMsg struct {
	err error
}

// OAuthOpenAI handles the OpenAI Codex OAuth flow authentication.
type OAuthOpenAI struct {
	com          *common.Common
	isOnboarding bool

	provider  catwalk.Provider
	model     config.SelectedModel
	modelType config.SelectedModelType

	State OAuthState

	spinner spinner.Model
	help    help.Model
	keyMap  struct {
		Copy   key.Binding
		Submit key.Binding
		Close  key.Binding
	}

	width int

	flow      openaioauth.AuthFlow
	server    *openaioauth.LocalServer
	token     *oauth.Token
	lastError error
	cancel    context.CancelFunc
}

var _ Dialog = (*OAuthOpenAI)(nil)

// NewOAuthOpenAI creates a new OpenAI OAuth dialog.
func NewOAuthOpenAI(
	com *common.Common,
	isOnboarding bool,
	provider catwalk.Provider,
	model config.SelectedModel,
	modelType config.SelectedModelType,
) (*OAuthOpenAI, tea.Cmd) {
	t := com.Styles

	m := OAuthOpenAI{}
	m.com = com
	m.isOnboarding = isOnboarding
	m.provider = provider
	m.model = model
	m.modelType = modelType
	m.width = 60
	m.State = OAuthStateInitializing

	m.spinner = spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(t.Base.Foreground(t.GreenLight)),
	)

	m.help = help.New()
	m.help.Styles = t.DialogHelpStyles()

	m.keyMap.Copy = key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "copy url"),
	)
	m.keyMap.Submit = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "open browser"),
	)
	m.keyMap.Close = CloseKey

	return &m, tea.Batch(m.spinner.Tick, m.initiateAuth)
}

// ID implements Dialog.
func (m *OAuthOpenAI) ID() string {
	return OAuthID
}

// HandleMsg handles messages and state transitions.
func (m *OAuthOpenAI) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		switch m.State {
		case OAuthStateInitializing, OAuthStateDisplay:
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			if cmd != nil {
				return ActionCmd{cmd}
			}
		}

	case openAICodexAuthInitMsg:
		if msg.err != nil {
			m.State = OAuthStateError
			m.lastError = msg.err
			return nil
		}
		m.flow = msg.flow
		m.server = msg.server
		m.State = OAuthStateDisplay
		return ActionCmd{tea.Batch(m.openURL(), m.startPolling())}

	case openAICodexAuthTokenMsg:
		m.State = OAuthStateSuccess
		m.token = msg.token
		m.stopPolling()
		return nil

	case openAICodexAuthErrorMsg:
		m.State = OAuthStateError
		m.lastError = msg.err
		m.stopPolling()
		return ActionCmd{uiutil.ReportError(msg.err)}

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.Copy):
			cmd := m.copyURL()
			return ActionCmd{cmd}
		case key.Matches(msg, m.keyMap.Submit):
			switch m.State {
			case OAuthStateSuccess:
				return m.saveKeyAndContinue()
			default:
				return ActionCmd{m.openURL()}
			}
		case key.Matches(msg, m.keyMap.Close):
			switch m.State {
			case OAuthStateSuccess:
				return m.saveKeyAndContinue()
			default:
				m.stopPolling()
				return ActionClose{}
			}
		}
	}
	return nil
}

// Draw implements Dialog.
func (m *OAuthOpenAI) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
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
	return nil
}

func (m *OAuthOpenAI) dialogContent() string {
	var (
		t         = m.com.Styles
		helpStyle = t.Dialog.HelpView
	)

	switch m.State {
	case OAuthStateInitializing:
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

func (m *OAuthOpenAI) headerContent() string {
	var (
		t            = m.com.Styles
		titleStyle   = t.Dialog.Title
		textStyle    = t.Dialog.PrimaryText
		dialogStyle  = t.Dialog.View.Width(m.width)
		headerOffset = titleStyle.GetHorizontalFrameSize() + dialogStyle.GetHorizontalFrameSize()
	)
	if m.isOnboarding {
		return textStyle.Render(m.dialogTitle())
	}
	return common.DialogTitle(t, titleStyle.Render(m.dialogTitle()), m.width-headerOffset, m.com.Styles.Primary, m.com.Styles.Secondary)
}

func (m *OAuthOpenAI) innerDialogContent() string {
	var (
		t         = m.com.Styles
		textStyle = t.Dialog.PrimaryText
		muted     = t.Dialog.SecondaryText
	)

	switch m.State {
	case OAuthStateInitializing:
		return muted.Render(m.spinner.View() + "Starting OpenAI Codex authentication...")
	case OAuthStateDisplay:
		link := t.Dialog.SecondaryText.Hyperlink(m.flow.URL, "id=openai-codex-auth").Render(m.flow.URL)
		return strings.Join([]string{
			textStyle.Render("Press enter to open the browser and complete authentication."),
			"",
			muted.Render("Browser not opening? Open this URL manually:"),
			link,
			"",
			muted.Render(m.spinner.View() + "Waiting for authorization..."),
		}, "\n")
	case OAuthStateSuccess:
		return textStyle.Render("Authentication successful! Press enter to continue.")
	case OAuthStateError:
		errMsg := "Authentication failed. Try \"crush login openai-codex\" from the CLI."
		if m.lastError != nil {
			errMsg = fmt.Sprintf("Authentication failed: %v", m.lastError)
		}
		return t.Dialog.TitleError.Render(errMsg)
	default:
		return ""
	}
}

func (m *OAuthOpenAI) dialogTitle() string {
	var (
		t           = m.com.Styles
		textStyle   = t.Dialog.TitleText
		accentStyle = t.Dialog.TitleAccent
	)
	return textStyle.Render("Authenticate with ") + accentStyle.Render("OpenAI Codex") + textStyle.Render(".")
}

// FullHelp returns the full help view.
func (m *OAuthOpenAI) FullHelp() [][]key.Binding {
	return [][]key.Binding{{m.keyMap.Copy, m.keyMap.Submit, m.keyMap.Close}}
}

// ShortHelp returns the full help view.
func (m *OAuthOpenAI) ShortHelp() []key.Binding {
	return []key.Binding{m.keyMap.Copy, m.keyMap.Submit, m.keyMap.Close}
}

func (m *OAuthOpenAI) initiateAuth() tea.Msg {
	flow, err := openaioauth.CreateAuthorizationFlow()
	if err != nil {
		return openAICodexAuthInitMsg{err: err}
	}
	server, err := openaioauth.StartLocalServer(flow.State)
	if err != nil {
		return openAICodexAuthInitMsg{flow: flow, err: err}
	}
	return openAICodexAuthInitMsg{flow: flow, server: server}
}

func (m *OAuthOpenAI) startPolling() tea.Cmd {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	m.cancel = cancel
	return func() tea.Msg {
		if m.server == nil {
			return openAICodexAuthErrorMsg{err: fmt.Errorf("OAuth callback server not available")}
		}
		defer m.server.Close()
		code, err := m.server.WaitForCode(ctx)
		if err != nil {
			return openAICodexAuthErrorMsg{err: err}
		}
		token, err := openaioauth.ExchangeAuthorizationCode(ctx, code, m.flow.Verifier)
		if err != nil {
			return openAICodexAuthErrorMsg{err: err}
		}
		return openAICodexAuthTokenMsg{token: token}
	}
}

func (m *OAuthOpenAI) stopPolling() {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	if m.server != nil {
		_ = m.server.Close()
		m.server = nil
	}
}

func (m *OAuthOpenAI) openURL() tea.Cmd {
	return func() tea.Msg {
		if m.flow.URL == "" {
			return nil
		}
		if err := browser.OpenURL(m.flow.URL); err != nil {
			return uiutil.ReportError(fmt.Errorf("failed to open browser: %w", err))()
		}
		return nil
	}
}

func (m *OAuthOpenAI) copyURL() tea.Cmd {
	if m.flow.URL == "" {
		return nil
	}
	return tea.Sequence(
		tea.SetClipboard(m.flow.URL),
		uiutil.ReportInfo("URL copied to clipboard"),
	)
}

func (m *OAuthOpenAI) saveKeyAndContinue() Action {
	cfg := m.com.Config()

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
