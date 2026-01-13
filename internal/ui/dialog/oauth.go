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
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/oauth/hyper"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/uiutil"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/pkg/browser"
)

// OAuthState represents the current state of the device flow.
type OAuthState int

const (
	OAuthStateInitializing OAuthState = iota
	OAuthStateDisplay
	OAuthStateSuccess
	OAuthStateError
)

// OAuthID is the identifier for the model selection dialog.
const OAuthID = "oauth"

// OAuth handles the OAuth flow authentication.
type OAuth struct {
	com *common.Common

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

	width           int
	deviceCode      string
	userCode        string
	verificationURL string
	expiresIn       int
	token           *oauth.Token
	cancelFunc      context.CancelFunc
}

var _ Dialog = (*OAuth)(nil)

// NewOAuth creates a new device flow component.
func NewOAuth(com *common.Common, provider catwalk.Provider, model config.SelectedModel, modelType config.SelectedModelType) (*OAuth, error) {
	t := com.Styles

	m := OAuth{}
	m.com = com
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
		key.WithHelp("c", "copy code"),
	)
	m.keyMap.Submit = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "copy & open"),
	)
	m.keyMap.Close = CloseKey

	return &m, nil
}

// ID implements Dialog.
func (m *OAuth) ID() string {
	return OAuthID
}

// Init implements Dialog.
func (m *OAuth) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.initiateDeviceAuth)
}

// HandleMsg handles messages and state transitions.
func (m *OAuth) HandleMsg(msg tea.Msg) Action {
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

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.Copy):
			cmd := m.copyCode()
			return ActionCmd{cmd}

		case key.Matches(msg, m.keyMap.Submit):
			switch m.State {
			case OAuthStateSuccess:
				return m.saveKeyAndContinue()

			default:
				cmd := m.copyCodeAndOpenURL()
				return ActionCmd{cmd}
			}

		case key.Matches(msg, m.keyMap.Close):
			switch m.State {
			case OAuthStateSuccess:
				return m.saveKeyAndContinue()

			default:
				return ActionClose{}
			}
		}

	case ActionInitiateOAuth:
		m.deviceCode = msg.DeviceCode
		m.userCode = msg.UserCode
		m.expiresIn = msg.ExpiresIn
		m.verificationURL = msg.VerificationURL
		m.State = OAuthStateDisplay
		return ActionCmd{m.startPolling(msg.DeviceCode)}

	case ActionCompleteOAuth:
		m.State = OAuthStateSuccess
		m.token = msg.Token
		return ActionCmd{m.stopPolling}

	case ActionOAuthErrored:
		m.State = OAuthStateError
		cmd := tea.Batch(m.stopPolling, uiutil.ReportError(msg.Error))
		return ActionCmd{cmd}
	}
	return nil
}

// View renders the device flow dialog.
func (m *OAuth) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	var (
		t           = m.com.Styles
		dialogStyle = t.Dialog.View.Width(m.width)
		view        = dialogStyle.Render(m.dialogContent())
	)
	DrawCenterCursor(scr, area, view, nil)
	return nil
}

func (m *OAuth) dialogContent() string {
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

func (m *OAuth) headerContent() string {
	var (
		t            = m.com.Styles
		titleStyle   = t.Dialog.Title
		dialogStyle  = t.Dialog.View.Width(m.width)
		headerOffset = titleStyle.GetHorizontalFrameSize() + dialogStyle.GetHorizontalFrameSize()
	)
	return common.DialogTitle(t, titleStyle.Render("Authenticate with Hyper"), m.width-headerOffset)
}

func (m *OAuth) innerDialogContent() string {
	var (
		t            = m.com.Styles
		whiteStyle   = lipgloss.NewStyle().Foreground(t.White)
		primaryStyle = lipgloss.NewStyle().Foreground(t.Primary)
		greenStyle   = lipgloss.NewStyle().Foreground(t.GreenLight)
		linkStyle    = lipgloss.NewStyle().Foreground(t.GreenDark).Underline(true)
		errorStyle   = lipgloss.NewStyle().Foreground(t.Error)
		mutedStyle   = lipgloss.NewStyle().Foreground(t.FgMuted)
	)

	switch m.State {
	case OAuthStateInitializing:
		return lipgloss.NewStyle().
			Margin(1, 1).
			Width(m.width - 2).
			Align(lipgloss.Center).
			Render(
				greenStyle.Render(m.spinner.View()) +
					mutedStyle.Render("Initializing..."),
			)

	case OAuthStateDisplay:
		instructions := lipgloss.NewStyle().
			Margin(1).
			Width(m.width - 2).
			Render(
				whiteStyle.Render("Press ") +
					primaryStyle.Render("enter") +
					whiteStyle.Render(" to copy the code below and open the browser."),
			)

		codeBox := lipgloss.NewStyle().
			Width(m.width-2).
			Height(7).
			Align(lipgloss.Center, lipgloss.Center).
			Background(t.BgBaseLighter).
			Margin(1).
			Render(
				lipgloss.NewStyle().
					Bold(true).
					Foreground(t.White).
					Render(m.userCode),
			)

		link := linkStyle.Hyperlink(m.verificationURL, "id=oauth-verify").Render(m.verificationURL)
		url := mutedStyle.
			Margin(0, 1).
			Width(m.width - 2).
			Render("Browser not opening? Refer to\n" + link)

		waiting := greenStyle.
			Width(m.width - 2).
			Margin(1).
			Render(m.spinner.View() + "Verifying...")

		return lipgloss.JoinVertical(
			lipgloss.Left,
			instructions,
			codeBox,
			url,
			waiting,
		)

	case OAuthStateSuccess:
		return greenStyle.
			Margin(1).
			Width(m.width - 2).
			Align(lipgloss.Center).
			Render("Authentication successful!")

	case OAuthStateError:
		return lipgloss.NewStyle().
			Margin(1).
			Width(m.width - 2).
			Render(errorStyle.Render("Authentication failed."))

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
				key.WithKeys("finish", "ctrl+y", "esc"),
				key.WithHelp("enter", "finish"),
			),
		}

	default:
		return []key.Binding{
			m.keyMap.Copy,
			m.keyMap.Submit,
			m.keyMap.Close,
		}
	}
}

func (d *OAuth) copyCode() tea.Cmd {
	if d.State != OAuthStateDisplay {
		return nil
	}
	return tea.Sequence(
		tea.SetClipboard(d.userCode),
		uiutil.ReportInfo("Code copied to clipboard"),
	)
}

func (d *OAuth) copyCodeAndOpenURL() tea.Cmd {
	if d.State != OAuthStateDisplay {
		return nil
	}
	return tea.Sequence(
		tea.SetClipboard(d.userCode),
		func() tea.Msg {
			if err := browser.OpenURL(d.verificationURL); err != nil {
				return ActionOAuthErrored{fmt.Errorf("failed to open browser: %w", err)}
			}
			return nil
		},
		uiutil.ReportInfo("Code copied and URL opened"),
	)
}

func (m *OAuth) saveKeyAndContinue() Action {
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

func (m *OAuth) initiateDeviceAuth() tea.Msg {
	minimumWait := 750 * time.Millisecond
	startTime := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	authResp, err := hyper.InitiateDeviceAuth(ctx)

	ellapsed := time.Since(startTime)
	if ellapsed < minimumWait {
		time.Sleep(minimumWait - ellapsed)
	}

	if err != nil {
		return ActionOAuthErrored{fmt.Errorf("failed to initiate device auth: %w", err)}
	}

	return ActionInitiateOAuth{
		DeviceCode:      authResp.DeviceCode,
		UserCode:        authResp.UserCode,
		ExpiresIn:       authResp.ExpiresIn,
		VerificationURL: authResp.VerificationURL,
	}
}

// startPolling starts polling for the device token.
func (m *OAuth) startPolling(deviceCode string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())
		m.cancelFunc = cancel

		refreshToken, err := hyper.PollForToken(ctx, deviceCode, m.expiresIn)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return ActionOAuthErrored{err}
		}

		token, err := hyper.ExchangeToken(ctx, refreshToken)
		if err != nil {
			return ActionOAuthErrored{fmt.Errorf("token exchange failed: %w", err)}
		}

		introspect, err := hyper.IntrospectToken(ctx, token.AccessToken)
		if err != nil {
			return ActionOAuthErrored{fmt.Errorf("token introspection failed: %w", err)}
		}
		if !introspect.Active {
			return ActionOAuthErrored{fmt.Errorf("access token is not active")}
		}

		return ActionCompleteOAuth{token}
	}
}

func (m *OAuth) stopPolling() tea.Msg {
	if m.cancelFunc != nil {
		m.cancelFunc()
	}
	return nil
}
