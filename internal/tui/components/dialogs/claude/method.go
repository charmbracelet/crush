package claude

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
)

type AuthMethod int

const (
	AuthMethodAPIKey AuthMethod = iota
	AuthMethodOAuth2
)

type AuthMethodChooser struct {
	State        AuthMethod
	width        int
	isOnboarding bool
}

func NewAuthMethodChooser() *AuthMethodChooser {
	return &AuthMethodChooser{
		State: AuthMethodOAuth2,
	}
}

func (a *AuthMethodChooser) Init() tea.Cmd {
	return nil
}

func (a *AuthMethodChooser) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	return a, nil
}

func (a *AuthMethodChooser) View() string {
	t := styles.CurrentTheme()

	white := lipgloss.NewStyle().Foreground(t.White)
	primary := lipgloss.NewStyle().Foreground(t.Primary)
	success := lipgloss.NewStyle().Foreground(t.Success)

	titleStyle := white
	if a.isOnboarding {
		titleStyle = primary
	}

	question := lipgloss.
		NewStyle().
		Margin(0, 1).
		Render(titleStyle.Render("How would you like to authenticate with ") + success.Render("Anthropic") + titleStyle.Render("?"))

	squareWidth := (a.width - 2) / 2
	squareHeight := squareWidth / 3
	if isOdd(squareHeight) {
		squareHeight++
	}

	square := lipgloss.NewStyle().
		Width(squareWidth).
		Height(squareHeight).
		Margin(0, 0).
		Border(lipgloss.RoundedBorder())

	squareText := lipgloss.NewStyle().
		Width(squareWidth - 2).
		Height(squareHeight).
		Align(lipgloss.Center).
		AlignVertical(lipgloss.Center)

	oauthBorder := t.AuthBorderSelected
	oauthText := t.AuthTextSelected
	apiKeyBorder := t.AuthBorderUnselected
	apiKeyText := t.AuthTextUnselected

	if a.State == AuthMethodAPIKey {
		oauthBorder, apiKeyBorder = apiKeyBorder, oauthBorder
		oauthText, apiKeyText = apiKeyText, oauthText
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		question,
		"",
		lipgloss.JoinHorizontal(
			lipgloss.Center,
			square.MarginLeft(1).
				Inherit(oauthBorder).Render(squareText.Inherit(oauthText).Render("Claude Account\nwith Subscription")),
			square.MarginRight(1).
				Inherit(apiKeyBorder).Render(squareText.Inherit(apiKeyText).Render("API Key")),
		),
	)
}

func (a *AuthMethodChooser) SetDefaults() {
	a.State = AuthMethodOAuth2
}

func (a *AuthMethodChooser) SetWidth(w int) {
	a.width = w
}

func (a *AuthMethodChooser) ToggleChoice() {
	switch a.State {
	case AuthMethodAPIKey:
		a.State = AuthMethodOAuth2
	case AuthMethodOAuth2:
		a.State = AuthMethodAPIKey
	}
}

func isOdd(n int) bool {
	return n%2 != 0
}
