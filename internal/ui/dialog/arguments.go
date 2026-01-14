package dialog

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/charmbracelet/crush/internal/commands"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/uiutil"
	uv "github.com/charmbracelet/ultraviolet"
)

// ArgumentsID is the identifier for the arguments dialog.
const ArgumentsID = "arguments"

// Dialog sizing for arguments.
const (
	maxInputWidth        = 120
	minInputWidth        = 30
	argumentsFieldHeight = 3 // label + input + spacing per field
)

// Arguments represents a dialog for collecting command arguments.
type Arguments struct {
	com       *common.Common
	title     string
	arguments []commands.Argument
	inputs    []textinput.Model
	focused   int
	spinner   spinner.Model
	loading   bool

	description  string
	resultAction Action

	help   help.Model
	keyMap struct {
		Confirm,
		Next,
		Previous,
		Close key.Binding
	}
}

var _ Dialog = (*Arguments)(nil)

// NewArguments creates a new arguments dialog.
func NewArguments(com *common.Common, title, description string, arguments []commands.Argument, resultAction Action) *Arguments {
	a := &Arguments{
		com:          com,
		title:        title,
		description:  description,
		arguments:    arguments,
		resultAction: resultAction,
	}

	a.help = help.New()
	a.help.Styles = com.Styles.DialogHelpStyles()

	a.keyMap.Confirm = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "confirm"),
	)
	a.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "tab"),
		key.WithHelp("↓/tab", "next"),
	)
	a.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "shift+tab"),
		key.WithHelp("↑/shift+tab", "previous"),
	)
	a.keyMap.Close = CloseKey

	// Create input fields for each argument.
	a.inputs = make([]textinput.Model, len(arguments))
	for i, arg := range arguments {
		input := textinput.New()
		input.SetVirtualCursor(false)
		input.SetStyles(com.Styles.TextInput)
		input.Prompt = "> "
		// Use description as placeholder if available, otherwise title
		if arg.Description != "" {
			input.Placeholder = arg.Description
		} else {
			input.Placeholder = arg.Title
		}

		if i == 0 {
			input.Focus()
		} else {
			input.Blur()
		}

		a.inputs[i] = input
	}
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(com.Styles.Secondary)
	a.spinner = s

	return a
}

// ID implements Dialog.
func (a *Arguments) ID() string {
	return ArgumentsID
}

// focusInput changes focus to a new input by index with wrap-around.
func (a *Arguments) focusInput(newIndex int) {
	a.inputs[a.focused].Blur()

	// Wrap around: Go's modulo can return negative, so add len first.
	n := len(a.inputs)
	a.focused = ((newIndex % n) + n) % n

	a.inputs[a.focused].Focus()
}

// HandleMsg implements Dialog.
func (a *Arguments) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if a.loading {
			var cmd tea.Cmd
			a.spinner, cmd = a.spinner.Update(msg)
			return ActionCmd{Cmd: cmd}
		}
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, a.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, a.keyMap.Confirm):
			// If we're on the last input or there's only one input, submit.
			if a.focused == len(a.inputs)-1 || len(a.inputs) == 1 {
				args := make(map[string]string)
				var warning tea.Cmd
				for i, arg := range a.arguments {
					args[arg.ID] = a.inputs[i].Value()
					if arg.Required && strings.TrimSpace(a.inputs[i].Value()) == "" {
						warning = uiutil.ReportWarn("Required argument '" + arg.Title + "' is missing.")
						break
					}
				}
				if warning != nil {
					return ActionCmd{Cmd: warning}
				}

				switch action := a.resultAction.(type) {
				case ActionRunCustomCommand:
					action.Args = args
					return action
				case ActionRunMCPPrompt:
					action.Args = args
					return action
				}
			}
			a.focusInput(a.focused + 1)
		case key.Matches(msg, a.keyMap.Next):
			a.focusInput(a.focused + 1)
		case key.Matches(msg, a.keyMap.Previous):
			a.focusInput(a.focused - 1)
		default:
			var cmd tea.Cmd
			a.inputs[a.focused], cmd = a.inputs[a.focused].Update(msg)
			return ActionCmd{Cmd: cmd}
		}
	case tea.PasteMsg:
		var cmd tea.Cmd
		a.inputs[a.focused], cmd = a.inputs[a.focused].Update(msg)
		return ActionCmd{Cmd: cmd}
	}
	return nil
}

// Cursor returns the cursor position relative to the dialog.
// we pass the description height to offset the cursor correctly.
func (a *Arguments) Cursor(descriptionHeight int) *tea.Cursor {
	cursor := InputCursor(a.com.Styles, a.inputs[a.focused].Cursor())
	if cursor == nil {
		return nil
	}
	cursor.Y += descriptionHeight + a.focused*argumentsFieldHeight + 1
	return cursor
}

// Draw implements Dialog.
func (a *Arguments) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	s := a.com.Styles

	dialogContentStyle := s.Base.Padding(1)

	possibleWidth := area.Dx() - s.Dialog.View.GetHorizontalFrameSize() - dialogContentStyle.GetHorizontalFrameSize()
	// Build fields with label and input.
	caser := cases.Title(language.English)

	var fields []string
	for i, arg := range a.arguments {
		isFocused := i == a.focused

		// Try to pretty up the title for the label.
		title := strings.ReplaceAll(arg.Title, "_", " ")
		title = strings.ReplaceAll(title, "-", " ")
		titleParts := strings.Fields(title)
		for i, part := range titleParts {
			titleParts[i] = caser.String(strings.ToLower(part))
		}
		labelText := strings.Join(titleParts, " ")
		if arg.Required {
			labelText += s.Base.Foreground(s.Primary).Render("*")
		}

		labelStyle := s.Base.Foreground(s.FgMuted)
		if isFocused {
			labelStyle = s.Base.Foreground(s.FgBase).Bold(true)
		}
		labelWidth := lipgloss.Width(labelText)
		placeholderWidth := lipgloss.Width(a.inputs[i].Placeholder)

		inputWidth := max(placeholderWidth, labelWidth, minInputWidth)
		inputWidth = min(inputWidth, min(possibleWidth, maxInputWidth))
		a.inputs[i].SetWidth(inputWidth + lipgloss.Width(a.inputs[i].Prompt))

		label := labelStyle.Render(labelText)
		inputLine := a.inputs[i].View()

		field := lipgloss.JoinVertical(lipgloss.Left, label, inputLine, "")
		fields = append(fields, field)
	}

	renderedFields := lipgloss.JoinVertical(lipgloss.Left, fields...)

	// We ancor the width to the fields so the field with the longest description
	// determines the dialog width. The width is capped at maxInputWidth or the
	// available area width.
	width := lipgloss.Width(renderedFields) + dialogContentStyle.GetHorizontalFrameSize() + s.Dialog.View.GetHorizontalFrameSize()

	// Use standard header
	titleStyle := s.Dialog.Title

	titleText := a.title
	if titleText == "" {
		titleText = "Arguments"
	}

	header := common.DialogTitle(s, titleText, width)

	// Add description if available.
	var description string
	if a.description != "" {
		descStyle := s.Base.Width(width).MarginBottom(1)
		description = descStyle.Render(a.description)
	}

	helpView := s.Dialog.HelpView.Width(width).Render(a.help.View(a))

	contentParts := []string{}
	if description != "" {
		contentParts = append(contentParts, description)
	}
	contentParts = append(contentParts, renderedFields)

	if a.loading {
		helpView = s.Dialog.HelpView.Width(width).Render(a.spinner.View() + " Generating Prompt...")
	}

	view := lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render(header),
		dialogContentStyle.Render(lipgloss.JoinVertical(lipgloss.Left, contentParts...)),
		helpView,
	)

	dialog := s.Dialog.View.Render(view)

	descriptionHeight := 0
	if a.description != "" {
		descriptionHeight = lipgloss.Height(description)
	}
	cur := a.Cursor(descriptionHeight)

	DrawCenterCursor(scr, area, dialog, cur)
	return cur
}

// StartLoading implements [LoadingDialog].
func (a *Arguments) StartLoading() tea.Cmd {
	if a.loading {
		return nil
	}
	a.loading = true
	return a.spinner.Tick
}

// StopLoading implements [LoadingDialog].
func (a *Arguments) StopLoading() {
	a.loading = false
}

// ShortHelp implements help.KeyMap.
func (a *Arguments) ShortHelp() []key.Binding {
	return []key.Binding{
		a.keyMap.Confirm,
		a.keyMap.Next,
		a.keyMap.Close,
	}
}

// FullHelp implements help.KeyMap.
func (a *Arguments) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{a.keyMap.Confirm, a.keyMap.Next, a.keyMap.Previous},
		{a.keyMap.Close},
	}
}
