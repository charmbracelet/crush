package openai

import (
	"context"
	"fmt"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/oauth"
	openaioauth "github.com/charmbracelet/crush/internal/oauth/openai"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/pkg/browser"
)

// AuthFlowState represents the current state of the OAuth flow.
type AuthFlowState int

const (
	AuthFlowStateDisplay AuthFlowState = iota
	AuthFlowStateSuccess
	AuthFlowStateError
)

// AuthInitiatedMsg is sent when the OAuth flow is initialized.
type AuthInitiatedMsg struct {
	Flow   openaioauth.AuthFlow
	Server *openaioauth.LocalServer
}

// AuthCompletedMsg is sent when OAuth completes successfully.
type AuthCompletedMsg struct {
	Token *oauth.Token
}

// AuthErrorMsg is sent when OAuth encounters an error.
type AuthErrorMsg struct {
	Error error
}

// AuthFlow handles the OpenAI Codex OAuth authentication.
type AuthFlow struct {
	State      AuthFlowState
	width      int
	flow       openaioauth.AuthFlow
	server     *openaioauth.LocalServer
	token      *oauth.Token
	lastError  error
	cancelFunc context.CancelFunc
	spinner    spinner.Model
}

// NewAuthFlow creates a new OAuth flow component.
func NewAuthFlow() *AuthFlow {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(styles.CurrentTheme().GreenLight)
	return &AuthFlow{
		State:   AuthFlowStateDisplay,
		spinner: s,
	}
}

// Init initializes the OAuth flow.
func (d *AuthFlow) Init() tea.Cmd {
	return tea.Batch(d.spinner.Tick, d.initiateAuth)
}

// Update handles messages and state transitions.
func (d *AuthFlow) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	var cmd tea.Cmd
	d.spinner, cmd = d.spinner.Update(msg)

	switch msg := msg.(type) {
	case AuthInitiatedMsg:
		d.flow = msg.Flow
		d.server = msg.Server
		return d, tea.Batch(cmd, d.OpenURL(), d.startPolling())
	case AuthCompletedMsg:
		d.State = AuthFlowStateSuccess
		d.token = msg.Token
		d.stopPolling()
		return d, nil
	case AuthErrorMsg:
		d.State = AuthFlowStateError
		d.lastError = msg.Error
		d.stopPolling()
		return d, nil
	}

	return d, cmd
}

// View renders the OAuth dialog.
func (d *AuthFlow) View() string {
	t := styles.CurrentTheme()

	primary := lipgloss.NewStyle().Foreground(t.Primary)
	white := lipgloss.NewStyle().Foreground(t.White)
	muted := lipgloss.NewStyle().Foreground(t.FgMuted)
	green := lipgloss.NewStyle().Foreground(t.GreenLight)
	errorStyle := lipgloss.NewStyle().Foreground(t.Error)

	switch d.State {
	case AuthFlowStateDisplay:
		if d.flow.URL == "" {
			return lipgloss.NewStyle().
				Margin(0, 1).
				Render(green.Render(d.spinner.View()) + muted.Render("Initializing..."))
		}
		instructions := lipgloss.NewStyle().
			Margin(1, 1, 0, 1).
			Width(d.width - 2).
			Render(
				white.Render("Press ") +
					primary.Render("enter") +
					white.Render(" to open the browser and log in."),
			)

		url := muted.
			Margin(0, 1).
			Width(d.width - 2).
			Render("Browser not opening? Refer to\n" + lipgloss.NewStyle().Hyperlink(d.flow.URL, "id=openai-codex-auth").Render(d.flow.URL))

		waiting := green.
			Width(d.width-2).
			Margin(1, 1, 0, 1).
			Render(d.spinner.View() + "Waiting for authorization...")

		return lipgloss.JoinVertical(
			lipgloss.Left,
			instructions,
			url,
			waiting,
		)

	case AuthFlowStateSuccess:
		return green.Margin(0, 1).Render("Authentication successful!")

	case AuthFlowStateError:
		message := "Authentication failed. Try \"crush login openai-codex\"."
		if d.lastError != nil {
			message = fmt.Sprintf("Authentication failed: %v", d.lastError)
		}
		return errorStyle.Margin(0, 1).Render(message)
	default:
		return ""
	}
}

// SetWidth sets the width of the dialog.
func (d *AuthFlow) SetWidth(w int) {
	d.width = w
}

// Cursor hides the cursor.
func (d *AuthFlow) Cursor() *tea.Cursor { return nil }

// CopyURL copies the auth URL to the clipboard.
func (d *AuthFlow) CopyURL() tea.Cmd {
	if d.flow.URL == "" {
		return nil
	}
	return tea.Sequence(
		tea.SetClipboard(d.flow.URL),
		util.ReportInfo("URL copied to clipboard"),
	)
}

// OpenURL opens the auth URL in the browser.
func (d *AuthFlow) OpenURL() tea.Cmd {
	if d.flow.URL == "" {
		return nil
	}
	return tea.Sequence(
		func() tea.Msg {
			if err := browser.OpenURL(d.flow.URL); err != nil {
				return AuthErrorMsg{Error: fmt.Errorf("failed to open browser: %w", err)}
			}
			return nil
		},
	)
}

// Cancel stops the OAuth flow.
func (d *AuthFlow) Cancel() {
	d.stopPolling()
}

func (d *AuthFlow) initiateAuth() tea.Msg {
	flow, err := openaioauth.CreateAuthorizationFlow()
	if err != nil {
		return AuthErrorMsg{Error: err}
	}
	server, err := openaioauth.StartLocalServer(flow.State)
	if err != nil {
		return AuthErrorMsg{Error: err}
	}
	return AuthInitiatedMsg{Flow: flow, Server: server}
}

func (d *AuthFlow) startPolling() tea.Cmd {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	d.cancelFunc = cancel
	return func() tea.Msg {
		if d.server == nil {
			return AuthErrorMsg{Error: fmt.Errorf("OAuth callback server not available")}
		}
		defer d.server.Close()
		code, err := d.server.WaitForCode(ctx)
		if err != nil {
			return AuthErrorMsg{Error: err}
		}
		token, err := openaioauth.ExchangeAuthorizationCode(ctx, code, d.flow.Verifier)
		if err != nil {
			return AuthErrorMsg{Error: err}
		}
		return AuthCompletedMsg{Token: token}
	}
}

func (d *AuthFlow) stopPolling() {
	if d.cancelFunc != nil {
		d.cancelFunc()
		d.cancelFunc = nil
	}
	if d.server != nil {
		_ = d.server.Close()
		d.server = nil
	}
}
