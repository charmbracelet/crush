// Package copilot provides the dialog for Copilot device flow authentication.
package copilot

import (
	"context"
	"fmt"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/oauth/copilot"
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
	DeviceFlowStateUnavailable
)

// DeviceAuthInitiatedMsg is sent when the device auth is initiated
// successfully.
type DeviceAuthInitiatedMsg struct {
	deviceCode *copilot.DeviceCode
}

// DeviceFlowCompletedMsg is sent when the device flow completes successfully.
type DeviceFlowCompletedMsg struct {
	Token *oauth.Token
}

// DeviceFlowErrorMsg is sent when the device flow encounters an error.
type DeviceFlowErrorMsg struct {
	Error error
}

// DeviceFlow handles the Copilot device flow authentication.
type DeviceFlow struct {
	State      DeviceFlowState
	width      int
	deviceCode *copilot.DeviceCode
	token      *oauth.Token
	cancelFunc context.CancelFunc
	spinner    spinner.Model
}

// NewDeviceFlow creates a new device flow component.
func NewDeviceFlow() *DeviceFlow {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(styles.CurrentTheme().GreenLight)
	return &DeviceFlow{
		State:   DeviceFlowStateDisplay,
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
		return d, tea.Batch(cmd, d.startPolling(msg.deviceCode))
	case DeviceFlowCompletedMsg:
		d.State = DeviceFlowStateSuccess
		d.token = msg.Token
		return d, nil
	case DeviceFlowErrorMsg:
		switch msg.Error {
		case copilot.ErrNotAvailable:
			d.State = DeviceFlowStateUnavailable
		default:
			d.State = DeviceFlowStateError
		}
		return d, nil
	}

	return d, cmd
}

// View renders the device flow dialog.
func (d *DeviceFlow) View() string {
	t := styles.CurrentTheme()

	whiteStyle := lipgloss.NewStyle().Foreground(t.White)
	primaryStyle := lipgloss.NewStyle().Foreground(t.Primary)
	greenStyle := lipgloss.NewStyle().Foreground(t.GreenLight)
	linkStyle := lipgloss.NewStyle().Foreground(t.GreenDark).Underline(true)
	errorStyle := lipgloss.NewStyle().Foreground(t.Error)
	mutedStyle := lipgloss.NewStyle().Foreground(t.FgMuted)

	switch d.State {
	case DeviceFlowStateDisplay:
		if d.deviceCode == nil {
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
					Render(d.deviceCode.UserCode),
			)

		uri := d.deviceCode.VerificationURI
		link := lipgloss.NewStyle().Hyperlink(uri, "id=copilot-verify").Render(uri)
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
			Width(d.width - 2).
			Render(errorStyle.Render("Authentication failed."))

	case DeviceFlowStateUnavailable:
		message := lipgloss.NewStyle().
			Margin(0, 1).
			Width(d.width - 2).
			Render("GitHub Copilot is unavailable for this account. To signup, go to the following page:")
		freeMessage := lipgloss.NewStyle().
			Margin(0, 1).
			Width(d.width - 2).
			Render("You may be able to request free access if eligible. For more information, see:")
		return lipgloss.JoinVertical(
			lipgloss.Left,
			message,
			"",
			linkStyle.Margin(0, 1).Width(d.width-2).Hyperlink(copilot.SignupURL, "id=copilot-signup").Render(copilot.SignupURL),
			"",
			freeMessage,
			"",
			linkStyle.Margin(0, 1).Width(d.width-2).Hyperlink(copilot.FreeURL, "id=copilot-free").Render(copilot.FreeURL),
		)

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
	switch d.State {
	case DeviceFlowStateDisplay:
		return tea.Sequence(
			tea.SetClipboard(d.deviceCode.UserCode),
			func() tea.Msg {
				if err := browser.OpenURL(d.deviceCode.VerificationURI); err != nil {
					return DeviceFlowErrorMsg{Error: fmt.Errorf("failed to open browser: %w", err)}
				}
				return nil
			},
			util.ReportInfo("Code copied and URL opened"),
		)
	case DeviceFlowStateUnavailable:
		return tea.Sequence(
			func() tea.Msg {
				if err := browser.OpenURL(copilot.SignupURL); err != nil {
					return DeviceFlowErrorMsg{Error: fmt.Errorf("failed to open browser: %w", err)}
				}
				return nil
			},
			util.ReportInfo("Code copied and URL opened"),
		)
	default:
		return nil
	}
}

// CopyCode copies just the user code to the clipboard.
func (d *DeviceFlow) CopyCode() tea.Cmd {
	if d.State != DeviceFlowStateDisplay {
		return nil
	}
	return tea.Sequence(
		tea.SetClipboard(d.deviceCode.UserCode),
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

	deviceCode, err := copilot.RequestDeviceCode(ctx)
	if err != nil {
		return DeviceFlowErrorMsg{Error: fmt.Errorf("failed to initiate device auth: %w", err)}
	}

	d.deviceCode = deviceCode

	return DeviceAuthInitiatedMsg{
		deviceCode: d.deviceCode,
	}
}

// startPolling starts polling for the device token.
func (d *DeviceFlow) startPolling(deviceCode *copilot.DeviceCode) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())
		d.cancelFunc = cancel

		token, err := copilot.PollForToken(ctx, deviceCode)
		if err != nil {
			if ctx.Err() != nil {
				return nil // cancelled, don't report error.
			}
			return DeviceFlowErrorMsg{Error: err}
		}

		return DeviceFlowCompletedMsg{Token: token}
	}
}
