package login

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/charmtone"
	"github.com/pkg/browser"

	"github.com/charmbracelet/crush/internal/oauth"
)

// authState represents the state of the OAuth TUI flow.
type authState int

const (
	authStateIntro authState = iota
	authStatePreparing
	authStateWaiting
	authStateExchanging
	authStateError
)

// authKeyMap represents the key bindings for the OAuth TUI.
type authKeyMap struct {
	Continue key.Binding
	Cancel   key.Binding
}

// defaultAuthKeybinds returns the default key bindings for the OAuth TUI.
func defaultAuthKeybinds() authKeyMap {
	return authKeyMap{
		Continue: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "continue"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("ctrl+c", "esc"),
			key.WithHelp("ctrl+c", "cancel"),
		),
	}
}

// ShortHelp returns the key bindings for the short help screen.
func (k authKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Continue, k.Cancel}
}

// FullHelp returns the key bindings for the full help screen.
func (k authKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Continue, k.Cancel}}
}

// updateAuthKeymap enables/disables key bindings based on the current state.
func (m *authModel) updateAuthKeymap() {
	m.keymap.Continue.SetEnabled(m.state == authStateIntro)
}

// authModel is the Bubble Tea model for the OAuth authorization flow.
type authModel struct {
	platform string
	newFlow  func() flow
	flow     flow

	state authState

	spinner spinner.Model

	verificationURL string
	userCode        string
	browserFailed   bool

	help   help.Model
	keymap authKeyMap

	token    *oauth.Token
	err      error
	canceled bool
	quitting bool
	// blankLines is the height of the last rendered frame, used to blank it
	// out in the final render on quit.
	blankLines int
	width      int
}

type authReadyMsg struct {
	flow     flow
	url      string
	userCode string
}

type authTokenMsg struct {
	token *oauth.Token
	err   error
}

type authErrMsg struct {
	err error
}

func newAuthModel(platform string, newFlow func() flow) authModel {
	s := spinner.New(spinner.WithSpinner(spinner.Dot))
	s.Style = lipgloss.NewStyle().Foreground(charmtone.Julep)
	m := authModel{
		platform: platform,
		newFlow:  newFlow,
		state:    authStateIntro,
		spinner:  s,
		help:     help.New(),
		keymap:   defaultAuthKeybinds(),
	}
	m.updateAuthKeymap()
	return m
}

// runTUI runs the OAuth authorization flow with a small TUI that guides
// the user through opening a browser and waiting for the callback.
func runTUI(platform string, newFlow func() flow) (*oauth.Token, error) {
	p := tea.NewProgram(newAuthModel(platform, newFlow))
	final, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("running auth program: %w", err)
	}
	m, ok := final.(authModel)
	if !ok {
		return nil, errors.New("unexpected auth program model")
	}
	if m.flow != nil {
		m.flow.Close()
	}
	if m.err != nil {
		return nil, m.err
	}
	if m.canceled {
		return nil, errors.New("authentication canceled")
	}
	return m.token, nil
}

// Init initializes the auth TUI.
func (m authModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the auth TUI.
func (m authModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case authReadyMsg:
		m.flow = msg.flow
		m.verificationURL = msg.url
		m.userCode = msg.userCode
		if !m.browserFailed {
			m.browserFailed = browser.OpenURL(msg.url) != nil
		}
		m.state = authStateWaiting
		m.updateAuthKeymap()
		cmds := []tea.Cmd{waitCmd(m.flow), m.spinner.Tick}
		if msg.userCode != "" {
			cmds = append(cmds, tea.SetClipboard(msg.userCode))
		}
		return m, tea.Batch(cmds...)

	case authTokenMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.token = msg.token
		}
		m.blankLines = m.contentHeight()
		m.quitting = true
		return m, tea.Quit

	case authErrMsg:
		m.err = msg.err
		m.blankLines = m.contentHeight()
		m.quitting = true
		return m, tea.Quit

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		switch m.state {
		case authStatePreparing, authStateWaiting, authStateExchanging:
			return m, cmd
		default:
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.help.SetWidth(msg.Width)
		return m, nil

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keymap.Cancel):
			m.canceled = true
			m.blankLines = m.contentHeight()
			m.quitting = true
			return m, tea.Quit
		case key.Matches(msg, m.keymap.Continue):
			if m.state == authStateIntro {
				m.state = authStatePreparing
				m.updateAuthKeymap()
				return m, tea.Batch(startFlowCmd(m.newFlow), m.spinner.Tick)
			}
		}
		var cmd tea.Cmd
		m.help, cmd = m.help.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m authModel) authHeader() string {
	title := "Let’s authenticate with " + titles[m.platform]
	w := m.authWidth()
	if lipgloss.Width(title) > w {
		title = ansi.Truncate(title, w, "…")
		return "\n  " + headerStyle.Render(title)
	}
	header := headerStyle.Render(title)
	if remaining := w - lipgloss.Width(title) - 1; remaining > 0 {
		header += " " + m.slashes(min(remaining, maxHeaderSlashes))
	}
	return "\n  " + header
}

// maxHeaderSlashes caps how wide the decorative slash run can get.
const maxHeaderSlashes = 50

// slashes renders n diagonal slashes with a horizontal gradient that fades
// from the header purple to pink, echoing the dialog title treatment.
func (authModel) slashes(n int) string {
	ramp := lipgloss.Blend1D(n, charmtone.Charple, charmtone.Dolly)
	var b strings.Builder
	for _, c := range ramp {
		b.WriteString(slashStyle.Foreground(c).Render("╱"))
	}
	return b.String()
}

// authWidth returns the usable text width for the auth TUI, accounting for
// the 2-space indent. Falls back to 80 when the terminal width is unknown.
func (m authModel) authWidth() int {
	const indent = 2
	w := m.width - indent
	if w < 20 {
		w = 80 - indent
	}
	return w
}

// View renders the auth TUI.
func (m authModel) View() tea.View {
	if m.quitting {
		return tea.NewView("")
	}
	return tea.NewView(m.content())
}

// content renders the full auth view. The trailing zero-width space keeps
// the final blank line alive (the renderer erases trailing empty lines).
func (m authModel) content() string {
	var b strings.Builder
	wrap := lipgloss.NewStyle().MaxWidth(m.authWidth())

	b.WriteString(m.authHeader())
	b.WriteString("\n\n  ")

	switch m.state {
	case authStateIntro:
		b.WriteString(wrap.Render("To authenticate we're going to open the browser. Ready?"))
	case authStatePreparing:
		b.WriteString(m.spinner.View())
		b.WriteString(wrap.Render("Preparing..."))
	case authStateWaiting:
		if m.userCode != "" {
			b.WriteString(wrap.Render("Your code is "))
			b.WriteString(lipgloss.NewStyle().Foreground(charmtone.Julep).Render(m.userCode))
			b.WriteString(" ")
			b.WriteString(lipgloss.NewStyle().Foreground(charmtone.Oyster).Render("copied to clipboard"))
			b.WriteString("\n\n  ")
		}
		urlStyle := lipgloss.NewStyle().
			Foreground(charmtone.Guac).
			Underline(true).
			Hyperlink(m.verificationURL).
			MaxWidth(m.authWidth())
		b.WriteString(wrap.Render("Browser not opening? Visit:"))
		b.WriteString("\n  ")
		b.WriteString(urlStyle.Render(m.verificationURL))
		b.WriteString("\n\n  ")
		b.WriteString(m.spinner.View())
		b.WriteString(wrap.Render("Waiting for authorization..."))
	case authStateExchanging:
		b.WriteString(m.spinner.View())
		b.WriteString(wrap.Render("Exchanging token..."))
	case authStateError:
		b.WriteString(errorStyle.Render(wrap.Render(m.err.Error())))
	default:
		return ""
	}

	b.WriteString("\n\n  ")
	b.WriteString(m.help.View(m.keymap))
	b.WriteString("\n")
	b.WriteString(trailingSpace)
	return b.String()
}

// contentHeight returns the height of the current view in lines.
func (m authModel) contentHeight() int {
	return lipgloss.Height(m.content())
}

// startFlowCmd kicks off the platform's OAuth flow and returns the URL to
// open in the browser along with an optional user code.
func startFlowCmd(newFlow func() flow) tea.Cmd {
	return func() tea.Msg {
		f := newFlow()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		url, userCode, err := f.Start(ctx)
		if err != nil {
			return authErrMsg{err: err}
		}

		return authReadyMsg{
			flow:     f,
			url:      url,
			userCode: userCode,
		}
	}
}

// waitCmd blocks until the user completes authorization in the browser and
// returns the resulting token.
func waitCmd(f flow) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		token, err := f.Wait(ctx)
		return authTokenMsg{token: token, err: err}
	}
}
