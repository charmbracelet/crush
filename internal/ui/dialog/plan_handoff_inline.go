package dialog

import (
	"image"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
)

// PlanHandoffInline is a small inline prompt rendered at the bottom of the
// editor area when the plan agent signals it is ready for execution.
// It replaces the textarea temporarily, asking the user to switch to code
// mode or keep editing the plan.
type PlanHandoffInline struct {
	com        *common.Common
	selectedNo bool // false = "Switch to code" selected (the default)
	focused    bool
	compositor *lipgloss.Compositor
	hoverX     int
	hoverY     int

	// OnConfirm is called when the user confirms switching to code mode.
	// The returned tea.Cmd is queued by the UI to perform the switch and
	// start the coder agent.
	OnConfirm func() tea.Cmd

	pendingCmd tea.Cmd // set during mouse confirmation; retrieved via PendingCmd

	keyLeftRight key.Binding
	keyEnter     key.Binding
	keyYes       key.Binding
	keyNo        key.Binding
	keyClose     key.Binding
}

var _ InlineEditor = (*PlanHandoffInline)(nil)

// NewPlanHandoffInline creates an inline plan handoff prompt. Wire
// OnConfirm before setting it as the active inline editor.
func NewPlanHandoffInline(com *common.Common) *PlanHandoffInline {
	return &PlanHandoffInline{
		com:        com,
		selectedNo: false, // default: "Switch to code" is highlighted
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
			key.WithKeys("n", "N", "esc"),
			key.WithHelp("n/esc", "keep editing"),
		),
		keyClose: CloseKey,
	}
}

// HandleKey processes a key press. Returns done=true when the user has
// made a choice. Returns a tea.Cmd to perform the mode switch when the
// user confirms.
func (p *PlanHandoffInline) HandleKey(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	switch {
	case key.Matches(msg, p.keyNo), key.Matches(msg, p.keyClose):
		return true, nil
	case key.Matches(msg, p.keyLeftRight):
		p.selectedNo = !p.selectedNo
		return false, nil
	case key.Matches(msg, p.keyEnter):
		if !p.selectedNo {
			return true, p.runConfirm()
		}
		return true, nil
	case key.Matches(msg, p.keyYes):
		return true, p.runConfirm()
	}
	return false, nil
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

// Height returns 3: question line + blank line + button line.
func (p *PlanHandoffInline) Height(width int) int { return 3 }

// Draw renders the inline prompt at the given screen area.
func (p *PlanHandoffInline) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	y := area.Min.Y

	iconPrompt := questionIconPrompt(p.com.Styles, p.focused)
	qText := iconPrompt + p.com.Styles.Editor.QuestionUnselected.Render("Plan is ready. Switch to code mode?")
	y += drawStyledText(scr, image.Rect(area.Min.X, y, area.Max.X, area.Max.Y), qText)
	y++ // blank

	buttonOptsList := []common.ButtonOpts{
		{Text: "Switch to code", Selected: !p.selectedNo, Padding: 3, UnderlineIndex: -1},
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

// HeightChanged always returns false — Height is constant.
func (p *PlanHandoffInline) HeightChanged() bool { return false }

// SetFocused updates the focus state (affects icon styling).
func (p *PlanHandoffInline) SetFocused(focused bool) { p.focused = focused }

// ShortHelp returns key bindings for the status bar.
func (p *PlanHandoffInline) ShortHelp() []key.Binding {
	return []key.Binding{p.keyLeftRight, p.keyEnter, p.keyYes, p.keyNo}
}

// SetHover implements MouseClickableEditor.
func (p *PlanHandoffInline) SetHover(x, y int) { p.hoverX = x; p.hoverY = y }

// HandleMouseClick implements MouseClickableEditor. Clicking "Switch to code"
// stores the confirm cmd via PendingCmd; clicking "Keep editing" cancels.
func (p *PlanHandoffInline) HandleMouseClick(x, y int) (bool, bool) {
	switch common.HitButtonIndex(p.compositor, x, y) {
	case 0: // Switch to code
		p.selectedNo = false
		p.runConfirm()
		return true, true
	case 1: // Keep editing
		p.selectedNo = true
		return true, true
	}
	return false, false
}
