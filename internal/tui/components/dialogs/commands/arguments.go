package commands

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/v2/help"
	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/textinput"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/lipgloss/v2"
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
	t := styles.CurrentTheme()
	inputs := make([]textinput.Model, len(argNames))

	for i, name := range argNames {
		ti := textinput.New()
		ti.Placeholder = fmt.Sprintf("Enter value for %s...", name)
		ti.SetWidth(40)
		ti.SetVirtualCursor(false)
		ti.Prompt = ""

		ti.SetStyles(t.S().TextInput)
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
func (c *commandArgumentsDialogCmp) View() string {
	t := styles.CurrentTheme()
	baseStyle := t.S().Base

	title := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true).
		Padding(0, 1).
		Render("Command Arguments")

	explanation := t.S().Text.
		Padding(0, 1).
		Render("This command requires arguments.")

	// Create input fields for each argument
	inputFields := make([]string, len(c.inputs))
	for i, input := range c.inputs {
		// Highlight the label of the focused input
		labelStyle := baseStyle.
			Padding(1, 1, 0, 1)

		if i == c.focusIndex {
			labelStyle = labelStyle.Foreground(t.FgBase).Bold(true)
		} else {
			labelStyle = labelStyle.Foreground(t.FgMuted)
		}

		label := labelStyle.Render(c.argNames[i] + ":")

		field := t.S().Text.
			Padding(0, 1).
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

	return baseStyle.Padding(1, 1, 0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus).
		Width(c.width).
		Render(content)
}

func (c *commandArgumentsDialogCmp) Cursor() *tea.Cursor {
	cursor := c.inputs[c.focusIndex].Cursor()
	if cursor != nil {
		cursor = c.moveCursor(cursor)
	}
	return cursor
}

func (c *commandArgumentsDialogCmp) moveCursor(cursor *tea.Cursor) *tea.Cursor {
	row, col := c.Position()
	offset := row + 3 + (1+c.focusIndex)*3
	cursor.Y += offset
	cursor.X = cursor.X + col + 3
	return cursor
}

func (c *commandArgumentsDialogCmp) Position() (int, int) {
	row := c.wHeight / 2
	row -= c.wHeight / 2
	col := c.wWidth / 2
	col -= c.width / 2
	return row, col
}

// ID implements CommandArgumentsDialog.
func (c *commandArgumentsDialogCmp) ID() dialogs.DialogID {
	return argumentsDialogID
}
