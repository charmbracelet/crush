package dialog

import (
	"fmt"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

type argumentsDialogKeyMap struct {
	Enter  key.Binding
	Escape key.Binding
}

// ShortHelp implements key.Map.
func (k argumentsDialogKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "confirm"),
		),
		key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
	}
}

// FullHelp implements key.Map.
func (k argumentsDialogKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}

// ShowMultiArgumentsDialogMsg is a message that is sent to show the multi-arguments dialog.
type ShowMultiArgumentsDialogMsg struct {
	CommandID string
	Content   string
	ArgNames  []string
}

// CloseMultiArgumentsDialogMsg is a message that is sent when the multi-arguments dialog is closed.
type CloseMultiArgumentsDialogMsg struct {
	Submit    bool
	CommandID string
	Content   string
	Args      map[string]string
}

// MultiArgumentsDialogCmp is a component that asks the user for multiple command arguments.
type MultiArgumentsDialogCmp struct {
	width, height int
	inputs        []textinput.Model
	focusIndex    int
	keys          argumentsDialogKeyMap
	commandID     string
	content       string
	argNames      []string
}

// NewMultiArgumentsDialogCmp creates a new MultiArgumentsDialogCmp.
func NewMultiArgumentsDialogCmp(commandID, content string, argNames []string) MultiArgumentsDialogCmp {
	t := theme.CurrentTheme()
	inputs := make([]textinput.Model, len(argNames))

	for i, name := range argNames {
		ti := textinput.New()
		ti.Placeholder = fmt.Sprintf("Enter value for %s...", name)
		ti.Width = 40
		ti.Prompt = ""
		ti.PlaceholderStyle = ti.PlaceholderStyle.Background(t.Background())
		ti.PromptStyle = ti.PromptStyle.Background(t.Background())
		ti.TextStyle = ti.TextStyle.Background(t.Background())
		
		// Only focus the first input initially
		if i == 0 {
			ti.Focus()
			ti.PromptStyle = ti.PromptStyle.Foreground(t.Primary())
			ti.TextStyle = ti.TextStyle.Foreground(t.Primary())
		} else {
			ti.Blur()
		}

		inputs[i] = ti
	}

	return MultiArgumentsDialogCmp{
		inputs:    inputs,
		keys:      argumentsDialogKeyMap{},
		commandID: commandID,
		content:   content,
		argNames:  argNames,
		focusIndex: 0,
	}
}

// Init implements tea.Model.
func (m MultiArgumentsDialogCmp) Init() tea.Cmd {
	// Make sure only the first input is focused
	for i := range m.inputs {
		if i == 0 {
			m.inputs[i].Focus()
		} else {
			m.inputs[i].Blur()
		}
	}
	
	return textinput.Blink
}

// Update implements tea.Model.
func (m MultiArgumentsDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	t := theme.CurrentTheme()

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			return m, util.CmdHandler(CloseMultiArgumentsDialogMsg{
				Submit:    false,
				CommandID: m.commandID,
				Content:   m.content,
				Args:      nil,
			})
		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			// If we're on the last input, submit the form
			if m.focusIndex == len(m.inputs)-1 {
				args := make(map[string]string)
				for i, name := range m.argNames {
					args[name] = m.inputs[i].Value()
				}
				return m, util.CmdHandler(CloseMultiArgumentsDialogMsg{
					Submit:    true,
					CommandID: m.commandID,
					Content:   m.content,
					Args:      args,
				})
			}
			// Otherwise, move to the next input
			m.inputs[m.focusIndex].Blur()
			m.focusIndex++
			m.inputs[m.focusIndex].Focus()
			m.inputs[m.focusIndex].PromptStyle = m.inputs[m.focusIndex].PromptStyle.Foreground(t.Primary())
			m.inputs[m.focusIndex].TextStyle = m.inputs[m.focusIndex].TextStyle.Foreground(t.Primary())
		case key.Matches(msg, key.NewBinding(key.WithKeys("tab"))):
			// Move to the next input
			m.inputs[m.focusIndex].Blur()
			m.focusIndex = (m.focusIndex + 1) % len(m.inputs)
			m.inputs[m.focusIndex].Focus()
			m.inputs[m.focusIndex].PromptStyle = m.inputs[m.focusIndex].PromptStyle.Foreground(t.Primary())
			m.inputs[m.focusIndex].TextStyle = m.inputs[m.focusIndex].TextStyle.Foreground(t.Primary())
		case key.Matches(msg, key.NewBinding(key.WithKeys("shift+tab"))):
			// Move to the previous input
			m.inputs[m.focusIndex].Blur()
			m.focusIndex = (m.focusIndex - 1 + len(m.inputs)) % len(m.inputs)
			m.inputs[m.focusIndex].Focus()
			m.inputs[m.focusIndex].PromptStyle = m.inputs[m.focusIndex].PromptStyle.Foreground(t.Primary())
			m.inputs[m.focusIndex].TextStyle = m.inputs[m.focusIndex].TextStyle.Foreground(t.Primary())
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	// Update the focused input
	var cmd tea.Cmd
	m.inputs[m.focusIndex], cmd = m.inputs[m.focusIndex].Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View implements tea.Model.
func (m MultiArgumentsDialogCmp) View() string {
	t := theme.CurrentTheme()
	baseStyle := styles.BaseStyle()

	// Calculate width needed for content
	maxWidth := 60 // Width for explanation text

	title := lipgloss.NewStyle().
		Foreground(t.Primary()).
		Bold(true).
		Width(maxWidth).
		Padding(0, 1).
		Background(t.Background()).
		Render("Command Arguments")

	explanation := lipgloss.NewStyle().
		Foreground(t.Text()).
		Width(maxWidth).
		Padding(0, 1).
		Background(t.Background()).
		Render("This command requires multiple arguments. Please enter values for each:")

	// Create input fields for each argument
	inputFields := make([]string, len(m.inputs))
	for i, input := range m.inputs {
		// Highlight the label of the focused input
		labelStyle := lipgloss.NewStyle().
			Width(maxWidth).
			Padding(1, 1, 0, 1).
			Background(t.Background())
			
		if i == m.focusIndex {
			labelStyle = labelStyle.Foreground(t.Primary()).Bold(true)
		} else {
			labelStyle = labelStyle.Foreground(t.TextMuted())
		}
		
		label := labelStyle.Render(m.argNames[i] + ":")

		field := lipgloss.NewStyle().
			Foreground(t.Text()).
			Width(maxWidth).
			Padding(0, 1).
			Background(t.Background()).
			Render(input.View())

		inputFields[i] = lipgloss.JoinVertical(lipgloss.Left, label, field)
	}

	maxWidth = min(maxWidth, m.width-10)

	// Join all elements vertically
	elements := []string{title, explanation}
	elements = append(elements, inputFields...)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		elements...,
	)

	return baseStyle.Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderBackground(t.Background()).
		BorderForeground(t.TextMuted()).
		Background(t.Background()).
		Width(lipgloss.Width(content) + 4).
		Render(content)
}

// SetSize sets the size of the component.
func (m *MultiArgumentsDialogCmp) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Bindings implements layout.Bindings.
func (m MultiArgumentsDialogCmp) Bindings() []key.Binding {
	return m.keys.ShortHelp()
}