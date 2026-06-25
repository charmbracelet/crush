package dialog

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os/exec"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/pkg/browser"
)

// AWSSSOID is the identifier for the AWS SSO auth dialog.
const AWSSSOID = "aws_sso"

// awsSSOState represents the current state of the AWS SSO flow.
type awsSSOState int

const (
	awsSSOStateWaiting awsSSOState = iota
	awsSSOStateSuccess
	awsSSOStateError
)

// awsSSOLineMsg carries a single line of output from the running command.
type awsSSOLineMsg struct {
	line string
}

// awsSSODoneMsg signals that the command has finished.
type awsSSODoneMsg struct {
	err    error
	stderr string
}

// streamResult carries either a line of output or a completion signal
// from the streaming command reader goroutine.
type streamResult struct {
	line   string
	done   bool
	err    error
	stderr string
}

// AWSSSO handles the AWS SSO authentication refresh flow.
type AWSSSO struct {
	com *common.Common

	state   awsSSOState
	command string
	cwd     string

	spinner spinner.Model
	help    help.Model
	keyMap  struct {
		Open  key.Binding
		Retry key.Binding
		Close key.Binding
	}

	width      int
	url        string
	stderr     string
	streamCh   chan streamResult
	cancelFunc context.CancelFunc
	onComplete func()
}

var _ Dialog = (*AWSSSO)(nil)

// NewAWSSSO creates a new AWS SSO authentication dialog.
// The command is the shell command to run (e.g. "aws sso login").
// onComplete is called when authentication succeeds so the caller can retry.
func NewAWSSSO(com *common.Common, command, cwd string, onComplete func()) (*AWSSSO, tea.Cmd) {
	t := com.Styles

	m := &AWSSSO{
		com:        com,
		command:    command,
		cwd:        cwd,
		width:      60,
		state:      awsSSOStateWaiting,
		onComplete: onComplete,
	}

	m.spinner = spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(t.Dialog.OAuth.Spinner),
	)

	m.help = help.New()
	m.help.Styles = t.DialogHelpStyles()

	m.keyMap.Open = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "open in browser"),
	)
	m.keyMap.Retry = key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "retry"),
	)
	m.keyMap.Close = CloseKey

	return m, tea.Batch(m.spinner.Tick, m.startCommandCmd())
}

// ID implements Dialog.
func (m *AWSSSO) ID() string {
	return AWSSSOID
}

// HandleMsg handles messages and state transitions.
func (m *AWSSSO) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if m.state == awsSSOStateWaiting {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			if cmd != nil {
				return ActionCmd{cmd}
			}
		}

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.Open):
			if m.state == awsSSOStateWaiting && m.url != "" {
				return ActionCmd{m.openURLCmd()}
			}
			if m.state == awsSSOStateSuccess {
				return ActionClose{}
			}

		case key.Matches(msg, m.keyMap.Retry):
			if m.state == awsSSOStateError {
				m.state = awsSSOStateWaiting
				m.url = ""
				m.stderr = ""
				return ActionCmd{tea.Batch(m.spinner.Tick, m.startCommandCmd())}
			}

		case key.Matches(msg, m.keyMap.Close):
			if m.cancelFunc != nil {
				m.cancelFunc()
			}
			return ActionClose{}
		}

	case awsSSOLineMsg:
		// Extract URL from streamed output as it appears.
		if m.url == "" {
			if url := ExtractAWSSSOURL(msg.line); url != "" {
				m.url = url
			}
		}
		// Continue reading the next line from the stream.
		if m.streamCh != nil {
			return ActionCmd{m.readNextLineCmd()}
		}
		return nil

	case awsSSODoneMsg:
		m.streamCh = nil
		if msg.err != nil {
			m.state = awsSSOStateError
			m.stderr = msg.stderr
			return nil
		}
		m.state = awsSSOStateSuccess
		if m.onComplete != nil {
			m.onComplete()
		}
		return nil
	}

	return nil
}

// Draw renders the AWS SSO auth dialog.
func (m *AWSSSO) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	var (
		t           = m.com.Styles
		dialogStyle = t.Dialog.View.Width(m.width)
	)
	view := dialogStyle.Render(m.dialogContent())
	DrawCenter(scr, area, view)
	return nil
}

func (m *AWSSSO) dialogContent() string {
	var (
		t         = m.com.Styles
		helpStyle = t.Dialog.HelpView
	)

	elements := []string{
		m.headerContent(),
		m.innerDialogContent(),
		helpStyle.Render(m.help.View(m)),
	}
	return strings.Join(elements, "\n")
}

func (m *AWSSSO) headerContent() string {
	var (
		t            = m.com.Styles
		titleStyle   = t.Dialog.Title
		dialogStyle  = t.Dialog.View.Width(m.width)
		headerOffset = titleStyle.GetHorizontalFrameSize() + dialogStyle.GetHorizontalFrameSize()
		dialogTitle  = "AWS SSO Authentication"
	)
	return common.DialogTitle(t, titleStyle.Render(dialogTitle), m.width-headerOffset, t.Dialog.TitleGradFromColor, t.Dialog.TitleGradToColor)
}

func (m *AWSSSO) innerDialogContent() string {
	var (
		t                = m.com.Styles
		instructionStyle = t.Dialog.OAuth.Instructions
		enterKeyStyle    = t.Dialog.OAuth.Enter
		successStyle     = t.Dialog.OAuth.Success
		linkStyle        = t.Dialog.OAuth.Link
		errorStyle       = t.Dialog.OAuth.ErrorText
		statusTextStyle  = t.Dialog.OAuth.StatusText
	)

	switch m.state {
	case awsSSOStateWaiting:
		if m.url != "" {
			// URL found; show it and wait for auth to complete.
			instructions := lipgloss.NewStyle().
				Margin(0, 1).
				Width(m.width - 2).
				Render(
					instructionStyle.Render("Press ") +
						enterKeyStyle.Render("enter") +
						instructionStyle.Render(" to open the authorization page."),
				)

			contentWidth := m.width - 4 // subtract margin (1 each side) + dialog padding
			displayURL := ansi.Truncate(m.url, max(0, contentWidth), "…")
			link := linkStyle.Hyperlink(m.url, "id=aws-sso-verify").Render(displayURL)
			urlBox := lipgloss.NewStyle().
				Margin(0, 1).
				Width(contentWidth).
				Align(lipgloss.Center).
				Render(link)

			waiting := lipgloss.NewStyle().
				Margin(0, 1).
				Width(m.width - 2).
				Render(
					successStyle.Render(m.spinner.View()) +
						statusTextStyle.Render("Waiting for authentication..."),
				)

			return lipgloss.JoinVertical(
				lipgloss.Left,
				"",
				instructions,
				"",
				urlBox,
				"",
				waiting,
				"",
			)
		}

		// No URL yet; still waiting for command output.
		return lipgloss.NewStyle().
			Margin(1, 1).
			Width(m.width - 2).
			Align(lipgloss.Center).
			Render(
				successStyle.Render(m.spinner.View()) +
					statusTextStyle.Render("Starting "+m.command+"..."),
			)

	case awsSSOStateSuccess:
		return successStyle.
			Margin(1).
			Width(m.width - 2).
			Align(lipgloss.Center).
			Render("✓ Authentication successful!")

	case awsSSOStateError:
		header := lipgloss.NewStyle().
			Margin(0, 1).
			Width(m.width - 2).
			Align(lipgloss.Center).
			Render(errorStyle.Render("Authentication failed."))

		if m.stderr == "" {
			return header
		}

		contentWidth := m.width - 4
		flattened := strings.Join(strings.Fields(strings.TrimSpace(m.stderr)), " ")
		wrapped := ansi.Wordwrap(flattened, contentWidth, "")
		detail := statusTextStyle.
			Margin(0, 1).
			Render(wrapped)

		return lipgloss.JoinVertical(lipgloss.Left, "", header, "", detail, "")

	default:
		return ""
	}
}

// FullHelp returns the full help view.
func (m *AWSSSO) FullHelp() [][]key.Binding {
	return [][]key.Binding{m.ShortHelp()}
}

// ShortHelp returns the short help view.
func (m *AWSSSO) ShortHelp() []key.Binding {
	switch m.state {
	case awsSSOStateError:
		return []key.Binding{m.keyMap.Retry, m.keyMap.Close}
	case awsSSOStateSuccess:
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("enter", "ctrl+y", "esc"),
				key.WithHelp("enter", "close"),
			),
		}
	default:
		if m.url != "" {
			return []key.Binding{m.keyMap.Open, m.keyMap.Close}
		}
		return []key.Binding{m.keyMap.Close}
	}
}

// startCommandCmd returns a tea.Cmd that launches the auth refresh command
// and begins streaming its output line-by-line.
func (m *AWSSSO) startCommandCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())
		m.cancelFunc = cancel

		cmd := exec.CommandContext(ctx, "sh", "-c", m.command)
		cmd.Dir = m.cwd

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			cancel()
			return awsSSODoneMsg{err: err}
		}
		stderrPipe, err := cmd.StderrPipe()
		if err != nil {
			cancel()
			return awsSSODoneMsg{err: err}
		}

		if err := cmd.Start(); err != nil {
			cancel()
			return awsSSODoneMsg{err: err}
		}

		ch := make(chan streamResult)
		m.streamCh = ch

		go func() {
			defer close(ch)
			var stderrBuf bytes.Buffer
			// Merge stdout and stderr into one scanner so we catch
			// the URL regardless of which stream AWS writes it to.
			// Also capture stderr separately for error display.
			stderrTee := io.TeeReader(stderrPipe, &stderrBuf)
			combined := io.MultiReader(stdout, stderrTee)
			scanner := bufio.NewScanner(combined)
			for scanner.Scan() {
				ch <- streamResult{line: scanner.Text()}
			}
			runErr := cmd.Wait()
			ch <- streamResult{done: true, err: runErr, stderr: stderrBuf.String()}
		}()

		// Read the first line/result.
		r, ok := <-ch
		if !ok {
			return awsSSODoneMsg{}
		}
		if r.done {
			return awsSSODoneMsg{err: r.err, stderr: r.stderr}
		}
		return awsSSOLineMsg{line: r.line}
	}
}

// readNextLineCmd returns a tea.Cmd that reads the next line from the
// active stream channel.
func (m *AWSSSO) readNextLineCmd() tea.Cmd {
	return func() tea.Msg {
		if m.streamCh == nil {
			return nil
		}
		r, ok := <-m.streamCh
		if !ok {
			return awsSSODoneMsg{}
		}
		if r.done {
			return awsSSODoneMsg{err: r.err, stderr: r.stderr}
		}
		return awsSSOLineMsg{line: r.line}
	}
}

func (m *AWSSSO) openURLCmd() tea.Cmd {
	return func() tea.Msg {
		if m.url == "" {
			return nil
		}
		_ = browser.OpenURL(m.url)
		return nil
	}
}
