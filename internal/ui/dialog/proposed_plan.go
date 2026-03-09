package dialog

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
)

const ProposedPlanID = "proposed_plan"

type proposedPlanChoice int

const (
	proposedPlanChoiceExecute proposedPlanChoice = iota
	proposedPlanChoiceRevise
	proposedPlanChoiceCancel
)

type ProposedPlan struct {
	com       *common.Common
	sessionID string
	plan      string
	selected  proposedPlanChoice
	inputMode bool
	feedback  textinput.Model
	help      help.Model
	keyMap    struct {
		Select key.Binding
		Left   key.Binding
		Right  key.Binding
		Tab    key.Binding
		Close  key.Binding
	}
}

func NewProposedPlan(com *common.Common, sessionID, plan string) *ProposedPlan {
	input := textinput.New()
	input.SetVirtualCursor(false)
	input.Placeholder = "Describe what to change in the plan"
	input.SetStyles(com.Styles.TextInput)

	h := help.New()
	h.Styles = com.Styles.DialogHelpStyles()

	return &ProposedPlan{
		com:       com,
		sessionID: sessionID,
		plan:      strings.TrimSpace(plan),
		feedback:  input,
		help:      h,
		selected:  proposedPlanChoiceExecute,
		keyMap: struct {
			Select key.Binding
			Left   key.Binding
			Right  key.Binding
			Tab    key.Binding
			Close  key.Binding
		}{
			Select: key.NewBinding(key.WithKeys("enter", " "), key.WithHelp("enter", "confirm")),
			Left:   key.NewBinding(key.WithKeys("left"), key.WithHelp("←", "previous")),
			Right:  key.NewBinding(key.WithKeys("right"), key.WithHelp("→", "next")),
			Tab:    key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch")),
			Close:  CloseKey,
		},
	}
}

func (*ProposedPlan) ID() string { return ProposedPlanID }

func (p *ProposedPlan) Cursor() *tea.Cursor {
	if !p.inputMode {
		return nil
	}
	cur := InputCursor(p.com.Styles, p.feedback.Cursor())
	if cur == nil {
		return nil
	}
	cur.Y += titleContentHeight + 3
	return cur
}

func (p *ProposedPlan) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if p.inputMode {
			switch {
			case key.Matches(msg, p.keyMap.Close):
				p.inputMode = false
				p.feedback.Blur()
				return nil
			case key.Matches(msg, p.keyMap.Select):
				feedback := strings.TrimSpace(p.feedback.Value())
				if feedback == "" {
					return nil
				}
				return ActionSubmitPlanFeedback{SessionID: p.sessionID, Feedback: feedback}
			}
			var cmd tea.Cmd
			p.feedback, cmd = p.feedback.Update(msg)
			return ActionCmd{Cmd: cmd}
		}

		switch {
		case key.Matches(msg, p.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, p.keyMap.Left):
			p.selected = (p.selected + 2) % 3
			return nil
		case key.Matches(msg, p.keyMap.Right, p.keyMap.Tab):
			p.selected = (p.selected + 1) % 3
			return nil
		case key.Matches(msg, p.keyMap.Select):
			switch p.selected {
			case proposedPlanChoiceExecute:
				return ActionExecuteProposedPlan{SessionID: p.sessionID, Plan: p.plan}
			case proposedPlanChoiceRevise:
				p.inputMode = true
				p.feedback.SetValue("")
				p.feedback.Focus()
				return nil
			default:
				return ActionClose{}
			}
		}
	}
	return nil
}

func (p *ProposedPlan) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	width := min(defaultDialogMaxWidth+18, max(56, area.Dx()-4))
	rc := NewRenderContext(p.com.Styles, width)
	rc.Title = "Proposed Plan Ready"

	if p.inputMode {
		p.feedback.SetWidth(max(0, width-p.com.Styles.Dialog.View.GetHorizontalFrameSize()-p.com.Styles.Dialog.InputPrompt.GetHorizontalFrameSize()-1))
		rc.AddPart("Start implementation now, or describe what should change in the plan.")
		rc.AddPart(p.com.Styles.Dialog.InputPrompt.Render(p.feedback.View()))
		rc.Help = p.help.View(p)
		view := rc.Render()
		cur := p.Cursor()
		DrawCenterCursor(scr, area, view, cur)
		return cur
	}

	preview := p.plan
	if lipgloss.Height(preview) > 8 {
		lines := strings.Split(preview, "\n")
		preview = strings.Join(lines[:8], "\n") + "\n…"
	}
	rc.AddPart("The plan is ready. Choose what to do next.")
	rc.AddPart(p.com.Styles.Dialog.SecondaryText.Render(preview))
	rc.AddPart(common.ButtonGroup(p.com.Styles, []common.ButtonOpts{
		{Text: "Start Implementation", Selected: p.selected == proposedPlanChoiceExecute, Padding: 2},
		{Text: "Revise Plan", Selected: p.selected == proposedPlanChoiceRevise, Padding: 2},
		{Text: "Cancel", Selected: p.selected == proposedPlanChoiceCancel, Padding: 2},
	}, " "))
	rc.Help = p.help.View(p)
	view := rc.Render()
	DrawCenter(scr, area, view)
	return nil
}

func (p *ProposedPlan) ShortHelp() []key.Binding {
	if p.inputMode {
		return []key.Binding{p.keyMap.Select, p.keyMap.Close}
	}
	return []key.Binding{p.keyMap.Left, p.keyMap.Right, p.keyMap.Select, p.keyMap.Close}
}

func (p *ProposedPlan) FullHelp() [][]key.Binding {
	if p.inputMode {
		return [][]key.Binding{{p.keyMap.Select, p.keyMap.Close}}
	}
	return [][]key.Binding{{p.keyMap.Left, p.keyMap.Right, p.keyMap.Select}, {p.keyMap.Tab, p.keyMap.Close}}
}
