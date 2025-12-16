package commands

import (
	"cmp"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/crush/internal/uicmd"
)

const (
	argumentsDialogID dialogs.DialogID = "arguments"
)

// ShowArgumentsDialogMsg is a message that is sent to show the arguments dialog.
type ShowArgumentsDialogMsg = uicmd.ShowArgumentsDialogMsg

// CloseArgumentsDialogMsg is a message that is sent when the arguments dialog is closed.
type CloseArgumentsDialogMsg = uicmd.CloseArgumentsDialogMsg

// CommandArgumentsDialog represents the commands dialog.
type CommandArgumentsDialog interface {
	dialogs.DialogModel
}

type commandArgumentsDialogCmp struct {
	wWidth, wHeight int
	width, height   int

	inputs    []textinput.Model
	focused   int
	keys      ArgumentsDialogKeyMap
	arguments []Argument
	help      help.Model

	id          string
	title       string
	name        string
	description string

	onSubmit func(args map[string]string) tea.Cmd
}

type Argument struct {
	Name, Title, Description string
	Required                 bool
}

func NewCommandArgumentsDialog(
	id, title, name, description string,
	arguments []Argument,
	onSubmit func(args map[string]string) tea.Cmd,
) CommandArgumentsDialog {
	t := styles.CurrentTheme()
	inputs := make([]textinput.Model, len(arguments))

	for i, arg := range arguments {
		ti := textinput.New()
		ti.Placeholder = cmp.Or(arg.Description, "Enter value for "+arg.Title)
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
		inputs:      inputs,
		keys:        DefaultArgumentsDialogKeyMap(),
		id:          id,
		name:        name,
		title:       title,
		description: description,
		arguments:   arguments,
		width:       60,
		help:        help.New(),
		onSubmit:    onSubmit,
	}
}

// Init implements CommandArgumentsDialog.
func (c *commandArgumentsDialogCmp) Init() tea.Cmd {
	return nil
}

// Update implements CommandArgumentsDialog.
func (c *commandArgumentsDialogCmp) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		c.wWidth = msg.Width
		c.wHeight = msg.Height
		c.width = min(90, c.wWidth)
		c.height = min(15, c.wHeight)
		for i := range c.inputs {
			c.inputs[i].SetWidth(c.width - (paddingHorizontal * 2))
		}
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, c.keys.Close):
			return c, util.CmdHandler(dialogs.CloseDialogMsg{})
		case key.Matches(msg, c.keys.Confirm):
			if c.focused == len(c.inputs)-1 {
				args := make(map[string]string)
				for i, arg := range c.arguments {
					value := c.inputs[i].Value()
					args[arg.Name] = value
				}
				return c, tea.Sequence(
					util.CmdHandler(dialogs.CloseDialogMsg{}),
					c.onSubmit(args),
				)
			}
			// Otherwise, move to the next input
			c.inputs[c.focused].Blur()
			c.focused++
			c.inputs[c.focused].Focus()
		case key.Matches(msg, c.keys.Next):
			// Move to the next input
			c.inputs[c.focused].Blur()
			c.focused = (c.focused + 1) % len(c.inputs)
			c.inputs[c.focused].Focus()
		case key.Matches(msg, c.keys.Previous):
			// Move to the previous input
			c.inputs[c.focused].Blur()
			c.focused = (c.focused - 1 + len(c.inputs)) % len(c.inputs)
			c.inputs[c.focused].Focus()
		case key.Matches(msg, c.keys.Close):
			return c, util.CmdHandler(dialogs.CloseDialogMsg{})
		default:
			var cmd tea.Cmd
			c.inputs[c.focused], cmd = c.inputs[c.focused].Update(msg)
			return c, cmd
		}
	case tea.PasteMsg:
		var cmd tea.Cmd
		c.inputs[c.focused], cmd = c.inputs[c.focused].Update(msg)
		return c, cmd
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
		Render(cmp.Or(c.title, c.name))

	promptName := t.S().Text.
		Padding(0, 1).
		Render(c.description)

	inputFields := make([]string, len(c.inputs))
	for i, input := range c.inputs {
		labelStyle := baseStyle.Padding(1, 1, 0, 1)

		if i == c.focused {
			labelStyle = labelStyle.Foreground(t.FgBase).Bold(true)
		} else {
			labelStyle = labelStyle.Foreground(t.FgMuted)
		}

		arg := c.arguments[i]
		argName := cmp.Or(arg.Title, arg.Name)
		if arg.Required {
			argName += "*"
		}
		label := labelStyle.Render(argName + ":")

		field := t.S().Text.
			Padding(0, 1).
			Render(input.View())

		inputFields[i] = lipgloss.JoinVertical(lipgloss.Left, label, field)
	}

	elements := []string{title, promptName}
	elements = append(elements, inputFields...)

	c.help.ShowAll = false
	helpText := baseStyle.Padding(0, 1).Render(c.help.View(c.keys))
	elements = append(elements, "", helpText)

	content := lipgloss.JoinVertical(lipgloss.Left, elements...)

	return baseStyle.Padding(1, 1, 0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus).
		Width(c.width).
		Render(content)
}

func (c *commandArgumentsDialogCmp) Cursor() *tea.Cursor {
	if len(c.inputs) == 0 {
		return nil
	}
	cursor := c.inputs[c.focused].Cursor()
	if cursor != nil {
		cursor = c.moveCursor(cursor)
	}
	return cursor
}

const (
	headerHeight      = 3
	itemHeight        = 3
	paddingHorizontal = 3
)

func (c *commandArgumentsDialogCmp) moveCursor(cursor *tea.Cursor) *tea.Cursor {
	row, col := c.Position()
	offset := row + headerHeight + (1+c.focused)*itemHeight
	cursor.Y += offset
	cursor.X = cursor.X + col + paddingHorizontal
	return cursor
}

func (c *commandArgumentsDialogCmp) Position() (int, int) {
	row := (c.wHeight / 2) - (c.height / 2)
	col := (c.wWidth / 2) - (c.width / 2)
	return row, col
}

// ID implements CommandArgumentsDialog.
func (c *commandArgumentsDialogCmp) ID() dialogs.DialogID {
	return argumentsDialogID
}
