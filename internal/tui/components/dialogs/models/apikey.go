package models

import (
	"fmt"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/home"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
)

type APIKeyInputState int

const (
	APIKeyInputStateInitial APIKeyInputState = iota
	APIKeyInputStateVerifying
	APIKeyInputStateVerified
	APIKeyInputStateError
)

type APIKeyStateChangeMsg struct {
	State APIKeyInputState
}

type APIKeyInput struct {
	input              textinput.Model
	credentialNameInput textinput.Model
	width              int
	spinner            spinner.Model
	providerName       string
	state              APIKeyInputState
	title              string
	showTitle          bool
	showCredentialName bool
	focusedField       int // 0 = API key, 1 = credential name
}

func NewAPIKeyInput() *APIKeyInput {
	t := styles.CurrentTheme()

	ti := textinput.New()
	ti.Placeholder = "Enter your API key..."
	ti.SetVirtualCursor(false)
	ti.Prompt = "> "
	ti.SetStyles(t.S().TextInput)
	ti.Focus()

	credentialNameInput := textinput.New()
	credentialNameInput.Placeholder = "default"
	credentialNameInput.SetVirtualCursor(false)
	credentialNameInput.Prompt = "> "
	credentialNameInput.SetStyles(t.S().TextInput)

	return &APIKeyInput{
		input:               ti,
		credentialNameInput: credentialNameInput,
		state:               APIKeyInputStateInitial,
		spinner: spinner.New(
			spinner.WithSpinner(spinner.Dot),
			spinner.WithStyle(t.S().Base.Foreground(t.Green)),
		),
		providerName:       "Provider",
		showTitle:          true,
		showCredentialName: true,
		focusedField:       0,
	}
}

func (a *APIKeyInput) SetProviderName(name string) {
	a.providerName = name
	a.updateStatePresentation()
}

func (a *APIKeyInput) SetShowTitle(show bool) {
	a.showTitle = show
}

func (a *APIKeyInput) GetTitle() string {
	return a.title
}

func (a *APIKeyInput) Init() tea.Cmd {
	a.updateStatePresentation()
	return a.spinner.Tick
}

func (a *APIKeyInput) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if a.state == APIKeyInputStateVerifying {
			var cmd tea.Cmd
			a.spinner, cmd = a.spinner.Update(msg)
			a.updateStatePresentation()
			return a, cmd
		}
		return a, nil
	case APIKeyStateChangeMsg:
		a.state = msg.State
		var cmd tea.Cmd
		if msg.State == APIKeyInputStateVerifying {
			cmd = a.spinner.Tick
		}
		a.updateStatePresentation()
		return a, cmd
	case tea.KeyMsg:
		// Tab to switch between fields
		if msg.String() == "tab" {
			a.focusedField = 1 - a.focusedField
			if a.focusedField == 0 {
				a.input.Focus()
				a.credentialNameInput.Blur()
			} else {
				a.input.Blur()
				a.credentialNameInput.Focus()
			}
			return a, nil
		}
	}

	var cmd tea.Cmd
	if a.focusedField == 0 {
		a.input, cmd = a.input.Update(msg)
	} else {
		a.credentialNameInput, cmd = a.credentialNameInput.Update(msg)
	}
	return a, cmd
}

func (a *APIKeyInput) updateStatePresentation() {
	t := styles.CurrentTheme()

	prefixStyle := t.S().Base.
		Foreground(t.Primary)
	accentStyle := t.S().Base.Foreground(t.Green).Bold(true)
	errorStyle := t.S().Base.Foreground(t.Cherry)

	switch a.state {
	case APIKeyInputStateInitial:
		titlePrefix := prefixStyle.Render("Enter your ")
		a.title = titlePrefix + accentStyle.Render(a.providerName+" API Key") + prefixStyle.Render(".")
		a.input.SetStyles(t.S().TextInput)
		a.input.Prompt = "> "
	case APIKeyInputStateVerifying:
		titlePrefix := prefixStyle.Render("Verifying your ")
		a.title = titlePrefix + accentStyle.Render(a.providerName+" API Key") + prefixStyle.Render("...")
		ts := t.S().TextInput
		// make the blurred state be the same
		ts.Blurred.Prompt = ts.Focused.Prompt
		a.input.Prompt = a.spinner.View()
		a.input.Blur()
	case APIKeyInputStateVerified:
		a.title = accentStyle.Render(a.providerName+" API Key") + prefixStyle.Render(" validated.")
		ts := t.S().TextInput
		// make the blurred state be the same
		ts.Blurred.Prompt = ts.Focused.Prompt
		a.input.SetStyles(ts)
		a.input.Prompt = styles.CheckIcon + " "
		a.input.Blur()
	case APIKeyInputStateError:
		a.title = errorStyle.Render("Invalid ") + accentStyle.Render(a.providerName+" API Key") + errorStyle.Render(". Try again?")
		ts := t.S().TextInput
		ts.Focused.Prompt = ts.Focused.Prompt.Foreground(t.Cherry)
		a.input.Focus()
		a.input.SetStyles(ts)
		a.input.Prompt = styles.ErrorIcon + " "
	}
}

func (a *APIKeyInput) View() string {
	inputView := a.input.View()

	t := styles.CurrentTheme()
	var content string

	if a.showCredentialName && a.state == APIKeyInputStateInitial {
		credentialNameView := a.credentialNameInput.View()
		credentialLabel := t.S().Subtle.Render("Credential name (optional)")

		dataPath := config.GlobalConfigData()
		dataPath = home.Short(dataPath)
		helpText := styles.CurrentTheme().S().Muted.
			Render(fmt.Sprintf("This will be written to the global configuration: %s", dataPath))

		if a.showTitle && a.title != "" {
			content = lipgloss.JoinVertical(
				lipgloss.Left,
				a.title,
				"",
				inputView,
				"",
				credentialLabel,
				credentialNameView,
				"",
				helpText,
			)
		} else {
			content = lipgloss.JoinVertical(
				lipgloss.Left,
				inputView,
				"",
				credentialLabel,
				credentialNameView,
				"",
				helpText,
			)
		}
	} else {
		dataPath := config.GlobalConfigData()
		dataPath = home.Short(dataPath)
		helpText := styles.CurrentTheme().S().Muted.
			Render(fmt.Sprintf("This will be written to the global configuration: %s", dataPath))

		if a.showTitle && a.title != "" {
			content = lipgloss.JoinVertical(
				lipgloss.Left,
				a.title,
				"",
				inputView,
				"",
				helpText,
			)
		} else {
			content = lipgloss.JoinVertical(
				lipgloss.Left,
				inputView,
				"",
				helpText,
			)
		}
	}

	return content
}

func (a *APIKeyInput) Cursor() *tea.Cursor {
	cursor := a.input.Cursor()
	if cursor != nil && a.showTitle {
		cursor.Y += 2 // Adjust for title and spacing
	}
	return cursor
}

func (a *APIKeyInput) Value() string {
	return a.input.Value()
}

func (a *APIKeyInput) CredentialName() string {
	name := a.credentialNameInput.Value()
	if name == "" {
		return "default"
	}
	return name
}

func (a *APIKeyInput) Tick() tea.Cmd {
	if a.state == APIKeyInputStateVerifying {
		return a.spinner.Tick
	}
	return nil
}

func (a *APIKeyInput) SetWidth(width int) {
	a.width = width
	a.input.SetWidth(width - 4)
}

func (a *APIKeyInput) Reset() {
	a.state = APIKeyInputStateInitial
	a.input.SetValue("")
	a.credentialNameInput.SetValue("")
	a.input.Focus()
	a.focusedField = 0
	a.updateStatePresentation()
}
