package claude

import (
	"context"
	"fmt"
	"net/url"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/oauth/claude"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/pkg/browser"
	"github.com/zeebo/xxh3"
)

type OAuthState int

const (
	OAuthStateURL OAuthState = iota
	OAuthStateCode
)

type OAuthValidationState int

const (
	OAuthValidationStateNone OAuthValidationState = iota
	OAuthValidationStateVerifying
	OAuthValidationStateValid
	OAuthValidationStateError
)

type ValidationCompletedMsg struct {
	State OAuthValidationState
	Token *oauth.Token
}

type AuthenticationCompleteMsg struct{}

type OAuth2 struct {
	State           OAuthState
	ValidationState OAuthValidationState
	width           int
	isOnboarding    bool

	// URL page
	err       error
	verifier  string
	challenge string
	URL       string
	urlId     string
	token     *oauth.Token

	// Code input page
	CodeInput textinput.Model
	spinner   spinner.Model
}

func NewOAuth2() *OAuth2 {
	return &OAuth2{
		State: OAuthStateURL,
	}
}

func (o *OAuth2) Init() tea.Cmd {
	t := styles.CurrentTheme()

	verifier, challenge, err := claude.GetChallenge()
	if err != nil {
		o.err = err
		return nil
	}

	url, err := claude.AuthorizeURL(verifier, challenge)
	if err != nil {
		o.err = err
		return nil
	}

	o.verifier = verifier
	o.challenge = challenge
	o.URL = url

	h := xxh3.New()
	_, _ = h.WriteString(o.URL)
	o.urlId = fmt.Sprintf("id=%x", h.Sum(nil))

	o.CodeInput = textinput.New()
	o.CodeInput.Placeholder = "Paste or type"
	o.CodeInput.SetVirtualCursor(false)
	o.CodeInput.Prompt = "> "
	o.CodeInput.SetStyles(t.S().TextInput)
	o.CodeInput.SetWidth(50)

	o.spinner = spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(t.S().Base.Foreground(t.Green)),
	)

	return nil
}

func (o *OAuth2) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case ValidationCompletedMsg:
		o.ValidationState = msg.State
		o.token = msg.Token
		switch o.ValidationState {
		case OAuthValidationStateError:
			o.CodeInput.Focus()
		}
		o.updatePrompt()
	}

	if o.ValidationState == OAuthValidationStateVerifying {
		var cmd tea.Cmd
		o.spinner, cmd = o.spinner.Update(msg)
		cmds = append(cmds, cmd)
		o.updatePrompt()
	}
	{
		var cmd tea.Cmd
		o.CodeInput, cmd = o.CodeInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	return o, tea.Batch(cmds...)
}

func (o *OAuth2) ValidationConfirm() (util.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch {
	case o.State == OAuthStateURL:
		_ = browser.OpenURL(o.URL)
		o.State = OAuthStateCode
		cmds = append(cmds, o.CodeInput.Focus())
	case o.ValidationState == OAuthValidationStateNone || o.ValidationState == OAuthValidationStateError:
		o.CodeInput.Blur()
		o.ValidationState = OAuthValidationStateVerifying
		cmds = append(cmds, o.spinner.Tick, o.validateCode)
	case o.ValidationState == OAuthValidationStateValid:
		cmds = append(cmds, func() tea.Msg { return AuthenticationCompleteMsg{} })
	}

	o.updatePrompt()
	return o, tea.Batch(cmds...)
}

func (o *OAuth2) View() string {
	t := styles.CurrentTheme()

	whiteStyle := lipgloss.NewStyle().Foreground(t.White)
	primaryStyle := lipgloss.NewStyle().Foreground(t.Primary)
	successStyle := lipgloss.NewStyle().Foreground(t.Success)
	errorStyle := lipgloss.NewStyle().Foreground(t.Error)

	titleStyle := whiteStyle
	if o.isOnboarding {
		titleStyle = primaryStyle
	}

	switch {
	case o.err != nil:
		return lipgloss.NewStyle().
			Margin(0, 1).
			Foreground(t.Error).
			Render(o.err.Error())
	case o.State == OAuthStateURL:
		heading := lipgloss.
			NewStyle().
			Margin(0, 1).
			Render(titleStyle.Render("Press enter key to open the following ") + successStyle.Render("URL") + titleStyle.Render(":"))

		return lipgloss.JoinVertical(
			lipgloss.Left,
			heading,
			"",
			lipgloss.NewStyle().
				Margin(0, 1).
				Foreground(t.FgMuted).
				Hyperlink(o.URL, o.urlId).
				Render(o.displayUrl()),
		)
	case o.State == OAuthStateCode:
		var heading string

		switch o.ValidationState {
		case OAuthValidationStateNone:
			st := lipgloss.NewStyle().Margin(0, 1)
			heading = st.Render(titleStyle.Render("Enter the ") + successStyle.Render("code") + titleStyle.Render(" you received."))
		case OAuthValidationStateVerifying:
			heading = titleStyle.Margin(0, 1).Render("Verifying...")
		case OAuthValidationStateValid:
			heading = successStyle.Margin(0, 1).Render("Validated.")
		case OAuthValidationStateError:
			heading = errorStyle.Margin(0, 1).Render("Invalid. Try again?")
		}

		return lipgloss.JoinVertical(
			lipgloss.Left,
			heading,
			"",
			" "+o.CodeInput.View(),
		)
	default:
		panic("claude oauth2: invalid state")
	}
}

func (o *OAuth2) SetDefaults() {
	o.State = OAuthStateURL
	o.ValidationState = OAuthValidationStateNone
	o.CodeInput.SetValue("")
	o.err = nil
}

func (o *OAuth2) SetWidth(w int) {
	o.width = w
	o.CodeInput.SetWidth(w - 4)
}

func (o *OAuth2) SetError(err error) {
	o.err = err
}

func (o *OAuth2) validateCode() tea.Msg {
	token, err := claude.ExchangeToken(context.Background(), o.CodeInput.Value(), o.verifier)
	if err != nil || token == nil {
		return ValidationCompletedMsg{State: OAuthValidationStateError}
	}
	return ValidationCompletedMsg{State: OAuthValidationStateValid, Token: token}
}

func (o *OAuth2) updatePrompt() {
	switch o.ValidationState {
	case OAuthValidationStateNone:
		o.CodeInput.Prompt = "> "
	case OAuthValidationStateVerifying:
		o.CodeInput.Prompt = o.spinner.View() + " "
	case OAuthValidationStateValid:
		o.CodeInput.Prompt = styles.CheckIcon + " "
	case OAuthValidationStateError:
		o.CodeInput.Prompt = styles.ErrorIcon + " "
	}
}

// Remove query params for display
// e.g., "https://claude.ai/oauth/authorize?..." -> "https://claude.ai/oauth/authorize..."
func (o *OAuth2) displayUrl() string {
	parsed, err := url.Parse(o.URL)
	if err != nil {
		return o.URL
	}

	if parsed.RawQuery != "" {
		parsed.RawQuery = ""
		return parsed.String() + "..."
	}

	return o.URL
}
