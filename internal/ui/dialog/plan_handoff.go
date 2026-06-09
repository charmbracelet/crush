package dialog

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
)

// PlanHandoffID is the identifier for the plan handoff dialog.
const PlanHandoffID = "plan_handoff"

// PlanHandoff is a confirmation dialog shown when the plan agent signals it
// is ready for execution. The user can confirm to switch to code mode or
// dismiss to keep editing the plan.
type PlanHandoff struct {
	com             *common.Common
	selectedConfirm bool // true if "Switch to code" is selected
	keyMap          struct {
		LeftRight,
		EnterSpace,
		Confirm,
		Cancel,
		Tab,
		Close key.Binding
	}
}

var _ Dialog = (*PlanHandoff)(nil)

// NewPlanHandoff creates a new plan handoff confirmation dialog.
func NewPlanHandoff(com *common.Common) *PlanHandoff {
	p := &PlanHandoff{
		com:             com,
		selectedConfirm: true,
	}
	p.keyMap.LeftRight = key.NewBinding(
		key.WithKeys("left", "right"),
		key.WithHelp("←/→", "switch options"),
	)
	p.keyMap.EnterSpace = key.NewBinding(
		key.WithKeys("enter", " "),
		key.WithHelp("enter/space", "confirm"),
	)
	p.keyMap.Confirm = key.NewBinding(
		key.WithKeys("y", "Y"),
		key.WithHelp("y/Y", "switch to code"),
	)
	p.keyMap.Cancel = key.NewBinding(
		key.WithKeys("n", "N"),
		key.WithHelp("n/N", "keep editing"),
	)
	p.keyMap.Tab = key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch options"),
	)
	p.keyMap.Close = CloseKey
	return p
}

// ID implements [Dialog].
func (*PlanHandoff) ID() string {
	return PlanHandoffID
}

// HandleMsg implements [Dialog].
func (p *PlanHandoff) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, p.keyMap.Close, p.keyMap.Cancel):
			return ActionClose{}
		case key.Matches(msg, p.keyMap.LeftRight, p.keyMap.Tab):
			p.selectedConfirm = !p.selectedConfirm
		case key.Matches(msg, p.keyMap.EnterSpace):
			if p.selectedConfirm {
				return ActionSwitchToCodeMode{}
			}
			return ActionClose{}
		case key.Matches(msg, p.keyMap.Confirm):
			return ActionSwitchToCodeMode{}
		}
	}
	return nil
}

// Draw implements [Dialog].
func (p *PlanHandoff) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	const question = "Plan is ready. Switch to code mode?"
	baseStyle := p.com.Styles.Dialog.Quit.Content
	buttonOpts := []common.ButtonOpts{
		{Text: "Switch to code", Selected: p.selectedConfirm, Padding: 3},
		{Text: "Keep editing", Selected: !p.selectedConfirm, Padding: 3},
	}
	buttons := common.ButtonGroup(p.com.Styles, buttonOpts, " ")
	content := baseStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Center,
			question,
			"",
			buttons,
		),
	)

	view := p.com.Styles.Dialog.Quit.Frame.Render(content)
	DrawCenter(scr, area, view)
	return nil
}

// ShortHelp implements [help.KeyMap].
func (p *PlanHandoff) ShortHelp() []key.Binding {
	return []key.Binding{
		p.keyMap.LeftRight,
		p.keyMap.EnterSpace,
	}
}

// FullHelp implements [help.KeyMap].
func (p *PlanHandoff) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{p.keyMap.LeftRight, p.keyMap.EnterSpace, p.keyMap.Confirm, p.keyMap.Cancel},
		{p.keyMap.Tab, p.keyMap.Close},
	}
}
