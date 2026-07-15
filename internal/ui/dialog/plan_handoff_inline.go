package dialog

import (
	"image"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// PlanHandoffInline is a small inline prompt rendered at the bottom of the
// editor area when the plan agent signals it is ready for execution.
// It replaces the textarea temporarily, asking the user to switch to code
// mode or keep editing the plan.
type PlanHandoffInline struct {
	com        *common.Common
	selectedNo bool // false = "Implement" selected (the default)
	editing    bool
	focused    bool
	compositor *lipgloss.Compositor
	hoverX     int
	hoverY     int
	editor     textarea.Model

	heightChanged bool

	// OnConfirm is called when the user confirms switching to code mode.
	// The returned tea.Cmd is queued by the UI to perform the switch and
	// start the coder agent.
	OnConfirm func() tea.Cmd
	// OnKeepEditing is called with the user's feedback when they submit it.
	OnKeepEditing func(string) tea.Cmd

	pendingCmd tea.Cmd // set during mouse confirmation; retrieved via PendingCmd

	keyLeftRight key.Binding
	keyEnter     key.Binding
	keyYes       key.Binding
	keyNo        key.Binding
	keyClose     key.Binding
}

var _ InlineEditor = (*PlanHandoffInline)(nil)

// NewPlanHandoffInline creates an inline plan handoff prompt. Wire its
// callbacks before setting it as the active inline editor.
func NewPlanHandoffInline(com *common.Common) *PlanHandoffInline {
	editor := newQuestionTextarea(com.Styles, "Describe the changes...", 1000)
	editor.MinHeight = 3
	editor.MaxHeight = 8
	editor.SetHeight(3)

	return &PlanHandoffInline{
		com:        com,
		selectedNo: false, // default: "Implement" is highlighted
		editor:     editor,
		keyLeftRight: key.NewBinding(
			key.WithKeys("left", "right"),
			key.WithHelp("←/→", "switch"),
		),
		keyEnter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "confirm"),
		),
		keyYes: key.NewBinding(
			key.WithKeys("y", "Y"),
			key.WithHelp("y", "switch to code"),
		),
		keyNo: key.NewBinding(
			key.WithKeys("n", "N"),
			key.WithHelp("n", "keep editing"),
		),
		keyClose: CloseKey,
	}
}

// HandleKey processes a key press. Returns done=true when the user has
// made a choice. Returns a tea.Cmd to perform the mode switch when the
// user confirms.
func (p *PlanHandoffInline) HandleKey(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	if p.editing {
		switch {
		case key.Matches(msg, p.keyClose):
			p.editing = false
			p.editor.Blur()
			p.heightChanged = true
			return false, nil
		case key.Matches(msg, p.keyEnter):
			feedback := strings.TrimSpace(p.editor.Value())
			if feedback == "" {
				return false, nil
			}
			if p.OnKeepEditing != nil {
				return true, p.OnKeepEditing(feedback)
			}
			return true, nil
		default:
			previousHeight := p.editor.Height()
			var cmd tea.Cmd
			p.editor, cmd = p.editor.Update(msg)
			p.heightChanged = p.heightChanged || previousHeight != p.editor.Height()
			return false, cmd
		}
	}

	switch {
	case key.Matches(msg, p.keyClose):
		return true, nil
	case key.Matches(msg, p.keyNo):
		return false, p.startEditing()
	case key.Matches(msg, p.keyLeftRight):
		p.selectedNo = !p.selectedNo
		return false, nil
	case key.Matches(msg, p.keyEnter):
		if p.selectedNo {
			return false, p.startEditing()
		}
		return true, p.runConfirm()
	case key.Matches(msg, p.keyYes):
		return true, p.runConfirm()
	}
	return false, nil
}

func (p *PlanHandoffInline) startEditing() tea.Cmd {
	p.editing = true
	p.selectedNo = true
	p.heightChanged = true
	if p.focused {
		return p.editor.Focus()
	}
	return nil
}

func (p *PlanHandoffInline) runConfirm() tea.Cmd {
	if p.OnConfirm != nil {
		cmd := p.OnConfirm()
		p.pendingCmd = cmd
		return cmd
	}
	return nil
}

// PendingCmd returns a cmd queued during mouse confirmation.
// The UI checks this via the CmdOnDone interface after a mouse-click dismissal.
func (p *PlanHandoffInline) PendingCmd() tea.Cmd { return p.pendingCmd }

// Height returns the number of lines needed by the current handoff state.
func (p *PlanHandoffInline) Height(width int) int {
	if !p.editing {
		return 3
	}
	iconPrompt := questionIconPrompt(p.com.Styles, p.focused)
	return sectionHeight("What should be changed in the plan?", width-lipgloss.Width(iconPrompt)) +
		1 + p.editor.Height() + 1
}

// Draw renders the inline prompt at the given screen area.
func (p *PlanHandoffInline) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	if p.editing {
		return p.drawEditor(scr, area)
	}

	y := area.Min.Y

	iconPrompt := questionIconPrompt(p.com.Styles, p.focused)
	qText := iconPrompt + p.com.Styles.Editor.QuestionUnselected.Render("Plan is ready. Switch to code mode?")
	y += drawStyledText(scr, image.Rect(area.Min.X, y, area.Max.X, area.Max.Y), qText)
	y++ // blank

	buttonOptsList := []common.ButtonOpts{
		{Text: "Implement", Selected: !p.selectedNo, Padding: 3, UnderlineIndex: -1},
		{Text: "Keep editing", Selected: p.selectedNo, Padding: 3, UnderlineIndex: -1},
	}
	hoveredBtn := common.HitButtonIndex(p.compositor, p.hoverX, p.hoverY)
	buttonOptsList[0].Hovered = hoveredBtn == 0
	buttonOptsList[1].Hovered = hoveredBtn == 1
	p.compositor = common.ButtonHitCompositor(p.com.Styles, buttonOptsList, " ", area.Min.X, y)
	buttons := common.ButtonGroup(p.com.Styles, buttonOptsList, " ")
	drawStyledText(scr, image.Rect(area.Min.X, y, area.Max.X, area.Max.Y), buttons)

	return nil
}

func (p *PlanHandoffInline) drawEditor(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	y := area.Min.Y
	iconPrompt := questionIconPrompt(p.com.Styles, p.focused)
	iconWidth := lipgloss.Width(iconPrompt)
	questionText := p.com.Styles.Editor.QuestionUnselected.Render(
		ansi.Wrap("What should be changed in the plan?", max(1, area.Dx()-iconWidth), ""),
	)
	y += drawStyledText(scr, image.Rect(area.Min.X, y, area.Max.X, area.Max.Y), iconPrompt+questionText)
	y++

	promptPrefix := p.com.Styles.Editor.QuestionBody.Render("> ")
	prefixWidth := lipgloss.Width(promptPrefix)
	p.editor.SetWidth(max(1, area.Dx()-2-prefixWidth))
	editorCursor := p.editor.Cursor()
	var cursor *tea.Cursor
	for row, line := range strings.Split(p.editor.View(), "\n") {
		text := promptPrefix + line
		if row > 0 {
			text = strings.Repeat(" ", prefixWidth) + line
		}
		drawStyledText(scr, image.Rect(area.Min.X, y+row, area.Max.X, y+row+1), text)
		if editorCursor != nil && editorCursor.Y == row {
			current := *editorCursor
			current.X += prefixWidth
			current.Y += y - area.Min.Y
			cursor = &current
		}
	}
	return cursor
}

// HeightChanged reports whether the current state changed its layout height.
func (p *PlanHandoffInline) HeightChanged() bool {
	changed := p.heightChanged
	p.heightChanged = false
	return changed
}

// SetFocused updates the focus state (affects icon styling).
func (p *PlanHandoffInline) SetFocused(focused bool) {
	p.focused = focused
	if focused && p.editing {
		p.editor.Focus()
	} else {
		p.editor.Blur()
	}
}

// ShortHelp returns key bindings for the status bar.
func (p *PlanHandoffInline) ShortHelp() []key.Binding {
	if p.editing {
		return []key.Binding{p.keyEnter, p.keyClose}
	}
	return []key.Binding{p.keyLeftRight, p.keyEnter, p.keyYes, p.keyNo}
}

// SetHover implements MouseClickableEditor.
func (p *PlanHandoffInline) SetHover(x, y int) { p.hoverX = x; p.hoverY = y }

// HandleMouseClick implements MouseClickableEditor. Clicking "Implement"
// stores the confirm cmd via PendingCmd; clicking "Keep editing" opens the
// feedback editor.
func (p *PlanHandoffInline) HandleMouseClick(x, y int) (bool, bool) {
	if p.editing {
		return false, false
	}
	switch common.HitButtonIndex(p.compositor, x, y) {
	case 0: // Implement
		p.selectedNo = false
		p.runConfirm()
		return true, true
	case 1: // Keep editing
		p.startEditing()
		return false, true
	}
	return false, false
}

// HandlePaste forwards paste events to the feedback editor.
func (p *PlanHandoffInline) HandlePaste(msg tea.PasteMsg) tea.Cmd {
	if !p.editing {
		return nil
	}
	previousHeight := p.editor.Height()
	var cmd tea.Cmd
	p.editor, cmd = p.editor.Update(msg)
	p.heightChanged = p.heightChanged || previousHeight != p.editor.Height()
	return cmd
}
