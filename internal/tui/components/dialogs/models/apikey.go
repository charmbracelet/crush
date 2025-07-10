package models

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/v2/textinput"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/lipgloss/v2"
)

type APIKeyInput struct {
	input        textinput.Model
	width        int
	height       int
	providerName string
}

func NewAPIKeyInput() *APIKeyInput {
	t := styles.CurrentTheme()

	ti := textinput.New()
	ti.Placeholder = "Enter your API key..."
	ti.SetWidth(50)
	ti.SetVirtualCursor(false)
	ti.Prompt = "> "
	ti.SetStyles(t.S().TextInput)
	ti.Focus()

	return &APIKeyInput{
		input:        ti,
		width:        60,
		providerName: "Provider",
	}
}

func (a *APIKeyInput) SetProviderName(name string) {
	a.providerName = name
}

func (a *APIKeyInput) Init() tea.Cmd {
	return textinput.Blink
}

func (a *APIKeyInput) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
	}

	var cmd tea.Cmd
	a.input, cmd = a.input.Update(msg)
	return a, cmd
}

func (a *APIKeyInput) View() string {
	t := styles.CurrentTheme()

	title := t.S().Base.
		Foreground(t.Primary).
		Bold(true).
		Render(fmt.Sprintf("Enter your %s API Key", a.providerName))

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
