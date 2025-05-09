package dialog

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

// ArgumentsDialogCmp is a component that asks the user for command arguments.
type ArgumentsDialogCmp struct {
	width, height int
	textInput     textinput.Model
	keys          argumentsDialogKeyMap
	commandID     string
	content       string
}

// NewArgumentsDialogCmp creates a new ArgumentsDialogCmp.
func NewArgumentsDialogCmp(commandID, content string) ArgumentsDialogCmp {
	t := theme.CurrentTheme()
	ti := textinput.New()
	ti.Placeholder = "Enter arguments..."
	ti.Focus()
	ti.Width = 40
	ti.Prompt = ""
	ti.PlaceholderStyle = ti.PlaceholderStyle.Background(t.Background())
	ti.PromptStyle = ti.PromptStyle.Background(t.Background())
	ti.TextStyle = ti.TextStyle.Background(t.Background())

	return ArgumentsDialogCmp{
		textInput: ti,
		keys:      argumentsDialogKeyMap{},
		commandID: commandID,
		content:   content,
	}
}

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

// Init implements tea.Model.
func (m ArgumentsDialogCmp) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.textInput.Focus(),
	)
}

// Update implements tea.Model.
func (m ArgumentsDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			return m, util.CmdHandler(CloseArgumentsDialogMsg{})
		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			return m, util.CmdHandler(CloseArgumentsDialogMsg{
				Submit:    true,
				CommandID: m.commandID,
				Content:   m.content,
				Arguments: m.textInput.Value(),
			})
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	m.textInput, cmd = m.textInput.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View implements tea.Model.
func (m ArgumentsDialogCmp) View() string {
	t := theme.CurrentTheme()
	baseStyle := styles.BaseStyle()

	// Calculate width needed for content
	maxWidth := 60 // Width for explanation text

	title := baseStyle.
		Foreground(t.Primary()).
		Bold(true).
		Width(maxWidth).
		Padding(0, 1).
		Render("Command Arguments")

	explanation := baseStyle.
		Foreground(t.Text()).
		Width(maxWidth).
		Padding(0, 1).
		Render("This command requires arguments. Please enter the text to replace $ARGUMENTS with:")

	inputField := baseStyle.
		Foreground(t.Text()).
		Width(maxWidth).
		Padding(1, 1).
		Render(m.textInput.View())

	maxWidth = min(maxWidth, m.width-10)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		explanation,
		inputField,
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
func (m *ArgumentsDialogCmp) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Bindings implements layout.Bindings.
func (m ArgumentsDialogCmp) Bindings() []key.Binding {
	return m.keys.ShortHelp()
}

// CloseArgumentsDialogMsg is a message that is sent when the arguments dialog is closed.
type CloseArgumentsDialogMsg struct {
	Submit    bool
	CommandID string
	Content   string
	Arguments string
}

// ShowArgumentsDialogMsg is a message that is sent to show the arguments dialog.
type ShowArgumentsDialogMsg struct {
	CommandID string
	Content   string
}

