package commands

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/v2/help"
	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/textinput"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/opencode-ai/opencode/internal/tui/components/dialogs"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

const (
	argumentsDialogID dialogs.DialogID = "arguments"
)

// ShowArgumentsDialogMsg is a message that is sent to show the arguments dialog.
type ShowArgumentsDialogMsg struct {
	CommandID string
	Content   string
	ArgNames  []string
}

// CloseArgumentsDialogMsg is a message that is sent when the arguments dialog is closed.
type CloseArgumentsDialogMsg struct {
	Submit    bool
	CommandID string
	Content   string
	Args      map[string]string
}

// CommandArgumentsDialog represents the commands dialog.
type CommandArgumentsDialog interface {
	dialogs.DialogModel
}

type commandArgumentsDialogCmp struct {
	width   int
	wWidth  int // Width of the terminal window
	wHeight int // Height of the terminal window

	inputs     []textinput.Model
	focusIndex int
	keys       ArgumentsDialogKeyMap
	commandID  string
	content    string
	argNames   []string
	help       help.Model
}

func NewCommandArgumentsDialog(commandID, content string, argNames []string) CommandArgumentsDialog {
	t := theme.CurrentTheme()
	inputs := make([]textinput.Model, len(argNames))

	for i, name := range argNames {
		ti := textinput.New()
		ti.Placeholder = fmt.Sprintf("Enter value for %s...", name)
		ti.SetWidth(40)
		ti.SetVirtualCursor(false)
		ti.Prompt = ""
		ds := ti.Styles()

		ds.Blurred.Placeholder = ds.Blurred.Placeholder.Background(t.Background()).Foreground(t.TextMuted())
		ds.Blurred.Prompt = ds.Blurred.Prompt.Background(t.Background()).Foreground(t.TextMuted())
		ds.Blurred.Text = ds.Blurred.Text.Background(t.Background()).Foreground(t.TextMuted())
		ds.Focused.Placeholder = ds.Blurred.Placeholder.Background(t.Background()).Foreground(t.TextMuted())
		ds.Focused.Prompt = ds.Blurred.Prompt.Background(t.Background()).Foreground(t.Text())
		ds.Focused.Text = ds.Blurred.Text.Background(t.Background()).Foreground(t.Text())
		ti.SetStyles(ds)
		// Only focus the first input initially
		if i == 0 {
			ti.Focus()
		} else {
			ti.Blur()
		}

		inputs[i] = ti
	}

	return &commandArgumentsDialogCmp{
		inputs:     inputs,
		keys:       DefaultArgumentsDialogKeyMap(),
		commandID:  commandID,
		content:    content,
		argNames:   argNames,
		focusIndex: 0,
		width:      60,
		help:       help.New(),
	}
}

// Init implements CommandArgumentsDialog.
func (c *commandArgumentsDialogCmp) Init() tea.Cmd {
	return nil
}

// Update implements CommandArgumentsDialog.
func (c *commandArgumentsDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		c.wWidth = msg.Width
		c.wHeight = msg.Height
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, c.keys.Confirm):
			if c.focusIndex == len(c.inputs)-1 {
				content := c.content
				for i, name := range c.argNames {
					value := c.inputs[i].Value()
					placeholder := "$" + name
					content = strings.ReplaceAll(content, placeholder, value)
				}
				return c, tea.Sequence(
					util.CmdHandler(dialogs.CloseDialogMsg{}),
					util.CmdHandler(CommandRunCustomMsg{
						Content: content,
					}),
				)
			}
			// Otherwise, move to the next input
			c.inputs[c.focusIndex].Blur()
			c.focusIndex++
			c.inputs[c.focusIndex].Focus()
		case key.Matches(msg, c.keys.Next):
			// Move to the next input
			c.inputs[c.focusIndex].Blur()
			c.focusIndex = (c.focusIndex + 1) % len(c.inputs)
			c.inputs[c.focusIndex].Focus()
		case key.Matches(msg, c.keys.Previous):
			// Move to the previous input
			c.inputs[c.focusIndex].Blur()
			c.focusIndex = (c.focusIndex - 1 + len(c.inputs)) % len(c.inputs)
			c.inputs[c.focusIndex].Focus()

		default:
			var cmd tea.Cmd
			c.inputs[c.focusIndex], cmd = c.inputs[c.focusIndex].Update(msg)
			return c, cmd
		}
	}
	return c, nil
}

// View implements CommandArgumentsDialog.
func (c *commandArgumentsDialogCmp) View() tea.View {
	t := theme.CurrentTheme()
	baseStyle := styles.BaseStyle()

	title := lipgloss.NewStyle().
		Foreground(t.Primary()).
		Bold(true).
		Padding(0, 1).
		Background(t.Background()).
		Render("Command Arguments")

	explanation := lipgloss.NewStyle().
		Foreground(t.Text()).
		Padding(0, 1).
		Background(t.Background()).
		Render("This command requires arguments.")

	// Create input fields for each argument
	inputFields := make([]string, len(c.inputs))
	for i, input := range c.inputs {
		// Highlight the label of the focused input
		labelStyle := lipgloss.NewStyle().
			Padding(1, 1, 0, 1).
			Background(t.Background())

		if i == c.focusIndex {
			labelStyle = labelStyle.Foreground(t.Text()).Bold(true)
		} else {
			labelStyle = labelStyle.Foreground(t.TextMuted())
		}

		label := labelStyle.Render(c.argNames[i] + ":")

		field := lipgloss.NewStyle().
			Foreground(t.Text()).
			Padding(0, 1).
			Background(t.Background()).
			Render(input.View())

		inputFields[i] = lipgloss.JoinVertical(lipgloss.Left, label, field)
	}

	// Join all elements vertically
	elements := []string{title, explanation}
	elements = append(elements, inputFields...)

	c.help.ShowAll = false
	helpText := baseStyle.Padding(0, 1).Render(c.help.View(c.keys))
	elements = append(elements, "", helpText)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		elements...,
	)

	view := tea.NewView(
		baseStyle.Padding(1, 1, 0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderBackground(t.Background()).
			BorderForeground(t.TextMuted()).
			Background(t.Background()).
			Width(c.width).
			Render(content),
	)
	cursor := c.inputs[c.focusIndex].Cursor()
	if cursor != nil {
		cursor = c.moveCursor(cursor)
	}
	view.SetCursor(cursor)
	return view
}

func (c *commandArgumentsDialogCmp) moveCursor(cursor *tea.Cursor) *tea.Cursor {
	offset := 13 + (1+c.focusIndex)*3
	cursor.Y += offset
	_, col := c.Position()
	cursor.X = cursor.X + col + 3
	return cursor
}

func (c *commandArgumentsDialogCmp) style() lipgloss.Style {
	t := theme.CurrentTheme()
	return styles.BaseStyle().
		Width(c.width).
		Padding(1).
		Border(lipgloss.RoundedBorder()).
		BorderBackground(t.Background()).
		BorderForeground(t.TextMuted())
}

func (q *commandArgumentsDialogCmp) Position() (int, int) {
	row := 10
	col := q.wWidth / 2
	col -= q.width / 2
	return row, col
}

// ID implements CommandArgumentsDialog.
func (c *commandArgumentsDialogCmp) ID() dialogs.DialogID {
	return argumentsDialogID
}
