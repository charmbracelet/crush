package dialog

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/util"
	uv "github.com/charmbracelet/ultraviolet"
)

// GoalInputID is the identifier for the goal input dialog.
const GoalInputID = "goal_input"

// GoalInput is a dedicated dialog for entering a goal objective.
// It uses textarea instead of textinput so that text wraps visually
// and cursor movement (including Up/Down) works normally.
type GoalInput struct {
	com          *common.Common
	ta           textarea.Model
	help         help.Model
	resultAction ActionSetGoal

	keyMap struct {
		Submit key.Binding
		Close  key.Binding
	}

	width int
}

var _ Dialog = (*GoalInput)(nil)

// NewGoalInput creates a new GoalInput dialog.
func NewGoalInput(com *common.Common, resultAction ActionSetGoal) *GoalInput {
	g := &GoalInput{
		com:          com,
		resultAction: resultAction,
	}

	g.keyMap.Submit = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "confirm"),
	)
	g.keyMap.Close = CloseKey

	ta := textarea.New()
	ta.SetStyles(com.Styles.Editor.Textarea)
	ta.ShowLineNumbers = false
	ta.CharLimit = -1
	ta.SetVirtualCursor(false)
	ta.DynamicHeight = true
	ta.MinHeight = 1
	ta.MaxHeight = 8
	ta.Placeholder = "What should the agent accomplish?"
	ta.Focus()
	g.ta = ta

	g.help = help.New()
	g.help.Styles = com.Styles.DialogHelpStyles()

	return g
}

// ID implements Dialog.
func (g *GoalInput) ID() string {
	return GoalInputID
}

// HandleMsg implements Dialog.
func (g *GoalInput) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, g.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, g.keyMap.Submit):
			objective := strings.TrimSpace(g.ta.Value())
			if objective == "" {
				return ActionCmd{Cmd: util.ReportWarn("Please provide an objective for the goal.")}
			}
			action := g.resultAction
			action.Args = map[string]string{"objective": objective}
			return action
		default:
			var cmd tea.Cmd
			g.ta, cmd = g.ta.Update(msg)
			return ActionCmd{Cmd: cmd}
		}
	case tea.PasteMsg:
		var cmd tea.Cmd
		g.ta, cmd = g.ta.Update(msg)
		return ActionCmd{Cmd: cmd}
	}
	return nil
}

// Draw implements Dialog.
func (g *GoalInput) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	s := g.com.Styles

	dialogContentStyle := s.Dialog.Arguments.Content
	possibleWidth := area.Dx() - s.Dialog.View.GetHorizontalFrameSize() - dialogContentStyle.GetHorizontalFrameSize()

	inputWidth := max(minInputWidth, min(possibleWidth, maxInputWidth))
	g.width = inputWidth
	g.ta.SetWidth(inputWidth - 2) // leave room for prompt prefix

	label := s.Dialog.Arguments.InputLabelFocused.Render("Objective") +
		s.Dialog.Arguments.InputRequiredMarkFocused.String()

	description := s.Dialog.Arguments.Description.Width(inputWidth).
		Render("Set an objective for the agent to accomplish autonomously.")

	helpView := s.Dialog.HelpView.Width(inputWidth).Render(g.help.View(g))

	header := common.DialogTitle(s, "Set Goal", inputWidth, s.Dialog.TitleGradFromColor, s.Dialog.TitleGradToColor)

	taView := g.ta.View()

	contentParts := lipgloss.JoinVertical(lipgloss.Left,
		description,
		label,
		taView,
	)

	view := lipgloss.JoinVertical(
		lipgloss.Left,
		s.Dialog.Title.Render(header),
		dialogContentStyle.Render(contentParts),
		helpView,
	)

	dialog := s.Dialog.View.Render(view)

	cur := g.cursor(
		lipgloss.Height(description),
		lipgloss.Height(label),
		lipgloss.Height(s.Dialog.Title.Render(header)),
	)

	DrawCenterCursor(scr, area, dialog, cur)
	return cur
}

// cursor computes the terminal cursor position relative to the dialog's top-left.
func (g *GoalInput) cursor(descriptionHeight, labelHeight, titleRenderedHeight int) *tea.Cursor {
	taCur := g.ta.Cursor()
	if taCur == nil {
		return nil
	}

	s := g.com.Styles
	dialogStyle := s.Dialog.View
	contentStyle := s.Dialog.Arguments.Content

	// Horizontal: dialog border+padding + content padding + textarea cursor X
	taCur.X += dialogStyle.GetBorderLeftSize() +
		dialogStyle.GetPaddingLeft() +
		dialogStyle.GetMarginLeft() +
		contentStyle.GetPaddingLeft() +
		contentStyle.GetBorderLeftSize()

	// Vertical: dialog border+padding + title + content padding + description + label + textarea cursor Y
	taCur.Y += dialogStyle.GetBorderTopSize() +
		dialogStyle.GetPaddingTop() +
		dialogStyle.GetMarginTop() +
		titleRenderedHeight +
		contentStyle.GetPaddingTop() +
		contentStyle.GetBorderTopSize() +
		descriptionHeight +
		labelHeight

	return taCur
}

// ShortHelp implements help.KeyMap.
func (g *GoalInput) ShortHelp() []key.Binding {
	return []key.Binding{g.keyMap.Submit, g.keyMap.Close}
}

// FullHelp implements help.KeyMap.
func (g *GoalInput) FullHelp() [][]key.Binding {
	return [][]key.Binding{{g.keyMap.Submit, g.keyMap.Close}}
}
