// Package hyper provides the dialog for Hyper device flow authentication.
package hyper

import (
	"context"
	"fmt"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	hyperp "github.com/charmbracelet/crush/internal/agent/hyper"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/oauth/hyper"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/pkg/browser"
)

// DeviceFlowState represents the current state of the device flow.
type DeviceFlowState int

const (
	DeviceFlowStateDisplay DeviceFlowState = iota
	DeviceFlowStateSuccess
	DeviceFlowStateError
)

// DeviceAuthInitiatedMsg is sent when the device auth is initiated
// successfully.
type DeviceAuthInitiatedMsg struct {
	deviceCode string
	expiresIn  int
}

// DeviceFlowCompletedMsg is sent when the device flow completes successfully.
type DeviceFlowCompletedMsg struct {
	Token *oauth.Token
}

// DeviceFlowErrorMsg is sent when the device flow encounters an error.
type DeviceFlowErrorMsg struct {
	Error error
}

// DeviceFlow handles the Hyper device flow authentication.
type DeviceFlow struct {
	State           DeviceFlowState
	width           int
	baseURL         string
	deviceCode      string
	userCode        string
	verificationURL string
	expiresIn       int
	token           *oauth.Token
	cancelFunc      context.CancelFunc
	spinner         spinner.Model
}

// NewDeviceFlow creates a new device flow component.
func NewDeviceFlow() *DeviceFlow {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(styles.CurrentTheme().GreenLight)
	return &DeviceFlow{
		State:   DeviceFlowStateDisplay,
		baseURL: hyperp.BaseURL(),
		spinner: s,
	}
}

// Init initializes the device flow by calling the device auth API and starting polling.
func (d *DeviceFlow) Init() tea.Cmd {
	return tea.Batch(d.spinner.Tick, d.initiateDeviceAuth)
}

// Update handles messages and state transitions.
func (d *DeviceFlow) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	var cmd tea.Cmd
	d.spinner, cmd = d.spinner.Update(msg)

	switch msg := msg.(type) {
	case DeviceAuthInitiatedMsg:
		// Start polling now that we have the device code.
		d.expiresIn = msg.expiresIn
		return d, tea.Batch(cmd, d.startPolling(msg.deviceCode))
	case DeviceFlowCompletedMsg:
		d.State = DeviceFlowStateSuccess
		d.token = msg.Token
		return d, nil
	case DeviceFlowErrorMsg:
		d.State = DeviceFlowStateError
		return d, util.ReportError(msg.Error)
	}

	return d, cmd
}

// View renders the device flow dialog.
func (d *DeviceFlow) View() string {
	t := styles.CurrentTheme()

	whiteStyle := lipgloss.NewStyle().Foreground(t.White)
	primaryStyle := lipgloss.NewStyle().Foreground(t.Primary)
	greenStyle := lipgloss.NewStyle().Foreground(t.GreenLight)
	errorStyle := lipgloss.NewStyle().Foreground(t.Error)
	mutedStyle := lipgloss.NewStyle().Foreground(t.FgMuted)

	switch d.State {
	case DeviceFlowStateDisplay:
		if d.userCode == "" {
			return lipgloss.NewStyle().
				Margin(0, 1).
				Render(
					greenStyle.Render(d.spinner.View()) +
						mutedStyle.Render("Initializing..."),
				)
		}

		instructions := lipgloss.NewStyle().
			Margin(1, 1, 0, 1).
			Width(d.width - 2).
			Render(

				whiteStyle.Render("Press ") +
					primaryStyle.Render("enter") +
					whiteStyle.Render(" to copy the code below and open the browser."),
			)

		codeBox := lipgloss.NewStyle().
			Width(d.width-2).
			Height(7).
			Align(lipgloss.Center, lipgloss.Center).
			Background(t.BgBaseLighter).
			Margin(1).
			Render(
				lipgloss.NewStyle().
					Bold(true).
					Foreground(t.White).
					Render(d.userCode),
			)

		link := lipgloss.NewStyle().Hyperlink(d.verificationURL, "id=hyper-verify").Render(d.verificationURL)
		url := mutedStyle.
			Margin(0, 1).
			Width(d.width - 2).
			Render("Browser not opening? Refer to\n" + link)

		waiting := greenStyle.
			Width(d.width-2).
			Margin(1, 1, 0, 1).
			Render(d.spinner.View() + "Verifying...")

		return lipgloss.JoinVertical(
			lipgloss.Left,
			instructions,
			codeBox,
			url,
			waiting,
		)

	case DeviceFlowStateSuccess:
		return greenStyle.Margin(0, 1).Render("Authentication successful!")

	case DeviceFlowStateError:
		return lipgloss.NewStyle().
			Margin(0, 1).
			Width(d.width).
			Render(errorStyle.Render("Authentication failed."))

	default:
		return ""
	}
}

// SetWidth sets the width of the dialog.
func (d *DeviceFlow) SetWidth(w int) {
	d.width = w
}

// Cursor hides the cursor.
func (d *DeviceFlow) Cursor() *tea.Cursor { return nil }

// CopyCodeAndOpenURL copies the user code to the clipboard and opens the URL.
func (d *DeviceFlow) CopyCodeAndOpenURL() tea.Cmd {
	return tea.Sequence(
		tea.SetClipboard(d.userCode),
		func() tea.Msg {
			if err := browser.OpenURL(d.verificationURL); err != nil {
				return DeviceFlowErrorMsg{Error: fmt.Errorf("failed to open browser: %w", err)}
			}
			return nil
		},
		util.ReportInfo("Code copied and URL opened"),
	)
}

// CopyCode copies just the user code to the clipboard.
func (d *DeviceFlow) CopyCode() tea.Cmd {
	return tea.Sequence(
		tea.SetClipboard(d.userCode),
		util.ReportInfo("Code copied to clipboard"),
	)
}

// Cancel cancels the device flow polling.
func (d *DeviceFlow) Cancel() {
	if d.cancelFunc != nil {
		d.cancelFunc()
	}
}

func (d *DeviceFlow) initiateDeviceAuth() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	authResp, err := hyper.InitiateDeviceAuth(ctx)
	if err != nil {
		return DeviceFlowErrorMsg{Error: fmt.Errorf("failed to initiate device auth: %w", err)}
	}

	d.deviceCode = authResp.DeviceCode
	d.userCode = authResp.UserCode
	d.verificationURL = authResp.VerificationURL

	return DeviceAuthInitiatedMsg{
		deviceCode: authResp.DeviceCode,
		expiresIn:  authResp.ExpiresIn,
	}
}

// startPolling starts polling for the device token.
func (d *DeviceFlow) startPolling(deviceCode string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())
		d.cancelFunc = cancel

		// Poll for refresh token.
		refreshToken, err := hyper.PollForToken(ctx, deviceCode, d.expiresIn)
		if err != nil {
			if ctx.Err() != nil {
				// Cancelled, don't report error.
				return nil
			}
			return DeviceFlowErrorMsg{Error: err}
		}

		// Exchange refresh token for access token.
		token, err := hyper.ExchangeToken(ctx, refreshToken)
		if err != nil {
			return DeviceFlowErrorMsg{Error: fmt.Errorf("token exchange failed: %w", err)}
		}

		// Verify the access token works.
		introspect, err := hyper.IntrospectToken(ctx, token.AccessToken)
		if err != nil {
			return DeviceFlowErrorMsg{Error: fmt.Errorf("token introspection failed: %w", err)}
		}
		if !introspect.Active {
			return DeviceFlowErrorMsg{Error: fmt.Errorf("access token is not active")}
		}

		return DeviceFlowCompletedMsg{Token: token}
	}
}
