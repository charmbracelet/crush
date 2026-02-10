package dialog

import (
	"context"
	"fmt"
	"net/url"
	"strings"

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
	"github.com/charmbracelet/x/ansi"
	"github.com/pkg/browser"
)

type OAuthProvider interface {
	name() string
	initiateAuth() tea.Msg
	waitForAuthorization(ctx context.Context) tea.Cmd
	stopAuthorization() tea.Msg
}

type OAuthStep interface {
	instructions() string
	verificationURL() string
	hyperlinkID() string
}

type OAuthStepCode interface {
	OAuthStep
	userCode() string
	copyValue() string
}

// OAuthState represents the current state of the OAuth flow.
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
		Copy   key.Binding
		Submit key.Binding
		Close  key.Binding
	}

	width      int
	step       OAuthStep
	token      *oauth.Token
	cancelFunc context.CancelFunc
}

var _ Dialog = (*OAuth)(nil)

// newOAuth creates a new OAuth authentication component.
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
		key.WithHelp("c", "copy"),
	)
	m.keyMap.Submit = key.NewBinding(
		key.WithKeys("enter", "ctrl+y", "o"),
		key.WithHelp("enter/o", "open"),
	)
	m.keyMap.Close = CloseKey

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
				return ActionCmd{
					Cmd: tea.Sequence(
						m.stopAuthorization(),
						func() tea.Msg { return ActionClose{} },
					),
				}
			}
		}

	case ActionInitiateOAuth:
		if msg.Step == nil {
			m.State = OAuthStateError
			return ActionCmd{util.ReportError(fmt.Errorf("oauth initiation returned no flow step"))}
		}

		ctx, cancel := context.WithCancel(context.Background())
		m.cancelFunc = cancel
		m.step = msg.Step
		m.State = OAuthStateDisplay
		return ActionCmd{m.oAuthProvider.waitForAuthorization(ctx)}

	case ActionCompleteOAuth:
		m.State = OAuthStateSuccess
		m.token = msg.Token
		return ActionCmd{m.stopAuthorization()}

	case ActionOAuthErrored:
		m.State = OAuthStateError
		cmd := tea.Batch(m.stopAuthorization(), util.ReportError(msg.Error))
		return ActionCmd{cmd}
	}
	return nil
}

// View renders the device flow dialog.
func (m *OAuth) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
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
		textStyle    = t.Dialog.PrimaryText
		dialogStyle  = t.Dialog.View.Width(m.width)
		headerOffset = titleStyle.GetHorizontalFrameSize() + dialogStyle.GetHorizontalFrameSize()
		dialogTitle  = fmt.Sprintf("Authenticate with %s", m.oAuthProvider.name())
	)
	if m.isOnboarding {
		return textStyle.Render(dialogTitle)
	}
	return common.DialogTitle(t, titleStyle.Render(dialogTitle), m.width-headerOffset, t.Primary, t.Secondary)
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
		if m.step == nil {
			return lipgloss.NewStyle().
				Margin(1).
				Width(m.width - 2).
				Render(errorStyle.Render("Authentication flow is not available."))
		}

		instructions := lipgloss.NewStyle().
			Margin(0, 1).
			Width(m.width - 2).
			Render(
				whiteStyle.Render("Press ") +
					primaryStyle.Render("enter") +
					whiteStyle.Render(" to "+m.step.instructions()) + "\n" +
					whiteStyle.Render("Press ") +
					primaryStyle.Render("c") +
					whiteStyle.Render(" to copy "+m.copyTargetLabel()+"."),
			)

		elements := []string{"", instructions}
		if step, ok := m.stepCode(); ok {
			codeBox := lipgloss.NewStyle().
				Width(m.width-2).
				Height(7).
				Align(lipgloss.Center, lipgloss.Center).
				Background(t.BgBaseLighter).
				Margin(0, 1).
				Render(
					lipgloss.NewStyle().
						Bold(true).
						Foreground(t.White).
						Render(step.userCode()),
				)
			elements = append(elements, "", codeBox)
		}

		rawURL := m.step.verificationURL()
		displayURL := m.displayVerificationURL(rawURL, m.width-4)
		link := linkStyle.Hyperlink(rawURL).Render(displayURL)
		if displayURL != rawURL {
			link += "\n" + mutedStyle.Render("(truncated for display)")
		}
		url := mutedStyle.
			Margin(0, 1).
			Width(m.width - 2).
			Render("Verification URL\n" + link)

		waiting := lipgloss.NewStyle().
			Margin(0, 1).
			Width(m.width - 2).
			Render(
				greenStyle.Render(m.spinner.View()) + mutedStyle.Render("Verifying..."),
			)

		elements = append(elements, "", url, "", waiting, "")
		return lipgloss.JoinVertical(lipgloss.Left, elements...)

	case OAuthStateSuccess:
		return greenStyle.
			Margin(1).
			Width(m.width - 2).
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
		bindings := []key.Binding{m.keyMap.Submit, m.keyMap.Close}
		if m.step != nil {
			bindings = append([]key.Binding{m.copyHelpBinding()}, bindings...)
		}
		return bindings
	}
}

func (m *OAuth) copyCode() tea.Cmd {
	if m.State != OAuthStateDisplay {
		return nil
	}

	copyValue, label := m.copyContent()
	if copyValue == "" {
		return nil
	}

	return tea.Sequence(
		tea.SetClipboard(copyValue),
		util.ReportInfo(fmt.Sprintf("%s copied to clipboard", label)),
	)
}

func (m *OAuth) copyCodeAndOpenURL() tea.Cmd {
	if m.State != OAuthStateDisplay || m.step == nil {
		return nil
	}

	openURL := func() tea.Msg {
		if err := browser.OpenURL(m.step.verificationURL()); err != nil {
			return ActionOAuthErrored{fmt.Errorf("failed to open browser: %w", err)}
		}
		return nil
	}

	copyValue, label := m.copyContent()
	if copyValue == "" {
		return tea.Sequence(
			openURL,
			util.ReportInfo("URL opened"),
		)
	}

	return tea.Sequence(
		tea.SetClipboard(copyValue),
		openURL,
		util.ReportInfo(fmt.Sprintf("%s copied and URL opened", label)),
	)
}

func (m *OAuth) saveKeyAndContinue() Action {
	cfg := m.com.Config()

	err := cfg.SetProviderAPIKey(string(m.provider.ID), m.token)
	if err != nil {
		return ActionCmd{util.ReportError(fmt.Errorf("failed to save API key: %w", err))}
	}

	return ActionSelectModel{
		Provider:  m.provider,
		Model:     m.model,
		ModelType: m.modelType,
	}
}

func (m *OAuth) stopAuthorization() tea.Cmd {
	if m.cancelFunc != nil {
		m.cancelFunc()
		m.cancelFunc = nil
	}
	return m.oAuthProvider.stopAuthorization
}

func (m *OAuth) stepCode() (OAuthStepCode, bool) {
	step, ok := m.step.(OAuthStepCode)
	return step, ok
}

func (m *OAuth) copyContent() (string, string) {
	if step, ok := m.stepCode(); ok {
		return step.copyValue(), "Code"
	}
	if m.step != nil {
		return m.step.verificationURL(), "URL"
	}
	return "", ""
}

func (m *OAuth) copyHelpBinding() key.Binding {
	label := "copy"
	if _, ok := m.stepCode(); ok {
		label = "copy code"
	} else if m.step != nil {
		label = "copy url"
	}
	return key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", label),
	)
}

func (m *OAuth) copyTargetLabel() string {
	if _, ok := m.stepCode(); ok {
		return "the code"
	}
	return "the full URL"
}

func (m *OAuth) displayVerificationURL(rawURL string, maxWidth int) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return rawURL
	}

	displayURL := rawURL
	if parsed, err := url.Parse(rawURL); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		displayURL = parsed.Scheme + "://" + parsed.Host + parsed.EscapedPath()
		if parsed.RawQuery != "" {
			displayURL += "?…"
		}
	}

	if maxWidth <= 0 {
		return displayURL
	}
	return ansi.Truncate(displayURL, maxWidth, "…")
}
