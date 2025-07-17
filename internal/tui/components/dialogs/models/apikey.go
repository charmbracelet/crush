package models

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/v2/spinner"
	"github.com/charmbracelet/bubbles/v2/textinput"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/lipgloss/v2"
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
	input        textinput.Model
	width        int
	spinner      spinner.Model
	providerName string
	state        APIKeyInputState
}

func NewAPIKeyInput() *APIKeyInput {
	t := styles.CurrentTheme()

	ti := textinput.New()
	ti.Placeholder = "Enter your API key..."
	ti.SetVirtualCursor(false)
	ti.Prompt = "> "
	ti.SetStyles(t.S().TextInput)
	ti.Focus()

	return &APIKeyInput{
		input: ti,
		state: APIKeyInputStateInitial,
		spinner: spinner.New(
			spinner.WithSpinner(spinner.Dot),
			spinner.WithStyle(t.S().Base.Foreground(t.Green)),
		),
		providerName: "Provider",
	}
}

func (a *APIKeyInput) SetProviderName(name string) {
	a.providerName = name
}

func (a *APIKeyInput) Init() tea.Cmd {
	return a.spinner.Tick
}

func (a *APIKeyInput) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if a.state == APIKeyInputStateVerifying {
			var cmd tea.Cmd
			a.spinner, cmd = a.spinner.Update(msg)
			return a, cmd
		}
		return a, nil
	case APIKeyStateChangeMsg:
		a.state = msg.State
		if msg.State == APIKeyInputStateVerifying {
			return a, a.spinner.Tick
		}
	}

	var cmd tea.Cmd
	a.input, cmd = a.input.Update(msg)
	return a, cmd
}

func (a *APIKeyInput) View() string {
	t := styles.CurrentTheme()

	var title string

	prefixStyle := t.S().Base.
		Foreground(t.Primary).
		Bold(true)
	accentStyle := t.S().Base.Foreground(t.Green).Bold(true)
	errorStyle := t.S().Base.Foreground(t.Cherry).Bold(true)

	switch a.state {
	case APIKeyInputStateInitial:
		titlePrefix := prefixStyle.Render("Enter your ")
		title = titlePrefix + accentStyle.Render(a.providerName+" API Key") + prefixStyle.Render(".")
		a.input.SetStyles(t.S().TextInput)
		a.input.Prompt = "> "
	case APIKeyInputStateVerifying:
		titlePrefix := prefixStyle.Render("Verifying your ")
		title = titlePrefix + accentStyle.Render(a.providerName+" API Key") + prefixStyle.Render("...")
		ts := t.S().TextInput
		// make the blurred state be the same
		ts.Blurred.Prompt = ts.Focused.Prompt
		a.input.Prompt = a.spinner.View() + " "
		a.input.Blur()
	case APIKeyInputStateVerified:
		title = accentStyle.Render(a.providerName+" API Key") + prefixStyle.Render(" validated.")
		a.input.SetStyles(t.S().TextInput)
		a.input.Prompt = styles.CheckIcon + " "
		a.input.Blur()
	case APIKeyInputStateError:
		title = errorStyle.Render("Invalid ") + accentStyle.Render(a.providerName+" API Key") + errorStyle.Render(". Try again?")
		ts := t.S().TextInput
		ts.Focused.Prompt = ts.Focused.Prompt.Foreground(t.Cherry)
		a.input.Focus()
		a.input.SetStyles(ts)
		a.input.Prompt = styles.ErrorIcon + " "
	}

	inputView := a.input.View()

	dataPath := config.GlobalConfigData()
	dataPath = strings.Replace(dataPath, config.HomeDir(), "~", 1)
	helpText := t.S().Muted.
		Render(fmt.Sprintf("This will be written to the global configuration: %s", dataPath))

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		inputView,
		"",
		helpText,
	)

	return content
}

func (a *APIKeyInput) Cursor() *tea.Cursor {
	cursor := a.input.Cursor()
	if cursor != nil {
		cursor.Y += 2 // Adjust for title and spacing
	}
	return cursor
}

func (a *APIKeyInput) Value() string {
	return a.input.Value()
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
