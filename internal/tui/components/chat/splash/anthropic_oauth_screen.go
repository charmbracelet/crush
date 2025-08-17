package splash

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/textinput"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/lacymorrow/lash/internal/config"
	"github.com/lacymorrow/lash/internal/tui/components/common"
	dialogmodels "github.com/lacymorrow/lash/internal/tui/components/dialogs/models"
	"github.com/lacymorrow/lash/internal/tui/styles"
	"github.com/lacymorrow/lash/internal/tui/util"
)

// oauthSuccessMsg is emitted when token exchange succeeded
type oauthSuccessMsg struct {
	providerID string
	modelID    string
	modelType  config.SelectedModelType
}

// oauthErrorMsg is emitted when token exchange fails and we want to show the error inline
// on the OAuth screen rather than relying solely on the global status bar.
type oauthErrorMsg struct{ err string }

type anthropicOAuthScreen struct {
	handler *common.AnthropicOAuthHandler

	url      string
	verifier string

	codeInput  textinput.Model
	status     string
	exchanging bool

	width int

	keyOpen   key.Binding
	keyCopy   key.Binding
	keySubmit key.Binding

	lastOpen time.Time
}

func newAnthropicOAuthScreen(option *dialogmodels.ModelOption, modelType config.SelectedModelType) *anthropicOAuthScreen {
	t := styles.CurrentTheme()
	ti := textinput.New()
	ti.Placeholder = "Paste code#state here"
	ti.SetVirtualCursor(false)
	ti.Prompt = "> "
	ti.SetStyles(t.S().TextInput)
	ti.Focus()

	return &anthropicOAuthScreen{
		handler: &common.AnthropicOAuthHandler{
			ProviderID: string(option.Provider.ID),
			ModelID:    option.Model.ID,
			ModelType:  modelType,
		},
		codeInput: ti,
		keyOpen:   key.NewBinding(key.WithKeys("o")),
		keyCopy:   key.NewBinding(key.WithKeys("c", "y")),
		keySubmit: key.NewBinding(key.WithKeys("enter")),
	}
}

func (s *anthropicOAuthScreen) Init() tea.Cmd {
	return tea.Sequence(
		func() tea.Msg {
			url, verifier, err := s.handler.StartOAuth()
			if err != nil {
				return util.InfoMsg{Type: util.InfoTypeError, Msg: err.Error()}
			}
			s.url = url
			s.verifier = verifier
			return nil
		},
		s.openBrowserCmd(),
	)
}

func (s *anthropicOAuthScreen) SetWidth(width int) {
	s.width = width
	// keep input comfortably wide within pane width
	s.codeInput.SetWidth(max(20, width-4))
}

func (s *anthropicOAuthScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case oauthErrorMsg:
		s.status = fmt.Sprintf("Error: %s", m.err)
		s.exchanging = false
		return s, nil
	case tea.KeyPressMsg:
		switch {
		case key.Matches(m, s.keyOpen):
			return s, s.openBrowserCmd()
		case key.Matches(m, s.keyCopy):
			if s.url != "" {
				_ = clipboard.WriteAll(s.url)
				return s, util.ReportInfo("Auth link copied to clipboard")
			}
			return s, nil
		case key.Matches(m, s.keySubmit):
			code := strings.TrimSpace(s.codeInput.Value())
			if code == "" {
				return s, nil
			}
			if s.exchanging {
				return s, nil
			}
			s.exchanging = true
			s.status = "Exchanging code..."
			return s, s.exchangeCmd(code)
		}
	}
	var cmd tea.Cmd
	s.codeInput, cmd = s.codeInput.Update(msg)
	return s, cmd
}

func (s *anthropicOAuthScreen) openBrowserCmd() tea.Cmd {
	if s.url == "" {
		return nil
	}
	return func() tea.Msg {
		if !s.lastOpen.IsZero() && time.Since(s.lastOpen) < 2*time.Second {
			return nil
		}
		s.lastOpen = time.Now()
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", s.url)
		case "linux":
			cmd = exec.Command("xdg-open", s.url)
		case "windows":
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", s.url)
		default:
			cmd = exec.Command("open", s.url)
		}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Start()
		return util.InfoMsg{Type: util.InfoTypeInfo, Msg: s.handler.GetBrowserMessage()}
	}
}

func (s *anthropicOAuthScreen) exchangeCmd(code string) tea.Cmd {
	return func() tea.Msg {
		err := s.handler.ExchangeCode(code, s.verifier)
		if err != nil {
			return oauthErrorMsg{err: err.Error()}
		}
		return util.InfoMsg{Type: util.InfoTypeInfo, Msg: "Authentication successful"}
	}
}

func (s *anthropicOAuthScreen) View() string {
	t := styles.CurrentTheme()
	title := t.S().Base.Foreground(t.Primary).Render(s.handler.GetTitleText())

	body := []string{
		t.S().Base.Render("1. A browser window was opened. Sign in and approve."),
		t.S().Base.Render("2. Press 'o' to open link, 'c/y' to copy link."),
		t.S().Muted.Render(s.url),
		t.S().Base.Render("3. Copy the code shown (it looks like code#state)."),
		t.S().Base.Render("4. Paste below and press Enter."),
		"",
		t.S().Muted.Render("If the browser didn't open: press 'o' to open again."),
	}

	input := s.codeInput.View()

	if s.status != "" {
		body = append(body, "", t.S().Base.Render(s.status))
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		strings.Join(body, "\n"),
		"",
		input,
	)

	// Note: Width is managed by parent
	return content
}