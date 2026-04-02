package dialog

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
)

const HandoffID = "handoff"

type Handoff struct {
	com       *common.Common
	sessionID string
	input     textinput.Model
	help      help.Model
	spinner   spinner.Model
	loading   bool
	keyMap    struct {
		Confirm key.Binding
		Close   key.Binding
	}
}

var (
	_ Dialog        = (*Handoff)(nil)
	_ LoadingDialog = (*Handoff)(nil)
)

func NewHandoff(com *common.Common, sessionID string) *Handoff {
	input := textinput.New()
	input.SetVirtualCursor(false)
	input.Placeholder = "Describe what the next session should accomplish"
	input.SetStyles(com.Styles.TextInput)
	input.Focus()

	h := help.New()
	h.Styles = com.Styles.DialogHelpStyles()

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = com.Styles.Dialog.Spinner

	return &Handoff{
		com:       com,
		sessionID: sessionID,
		input:     input,
		help:      h,
		spinner:   s,
		keyMap: struct {
			Confirm key.Binding
			Close   key.Binding
		}{
			Confirm: key.NewBinding(key.WithKeys("enter", "ctrl+y"), key.WithHelp("enter", "generate")),
			Close:   CloseKey,
		},
	}
}

func (*Handoff) ID() string {
	return HandoffID
}

func (h *Handoff) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if h.loading {
			var cmd tea.Cmd
			h.spinner, cmd = h.spinner.Update(msg)
			return ActionCmd{Cmd: cmd}
		}
	case tea.KeyPressMsg:
		if h.loading {
			if key.Matches(msg, h.keyMap.Close) {
				return ActionClose{}
			}
			return nil
		}

		switch {
		case key.Matches(msg, h.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, h.keyMap.Confirm):
			goal := strings.TrimSpace(h.input.Value())
			if goal == "" {
				return nil
			}
			return ActionGenerateHandoff{
				SessionID: h.sessionID,
				Goal:      goal,
			}
		default:
			var cmd tea.Cmd
			h.input, cmd = h.input.Update(msg)
			return ActionCmd{Cmd: cmd}
		}
	}
	return nil
}

func (h *Handoff) Cursor() *tea.Cursor {
	if h.loading {
		return nil
	}
	cur := InputCursor(h.com.Styles, h.input.Cursor())
	if cur == nil {
		return nil
	}
	intro := "Describe the goal for the next top-level session. Crush will draft the prompt and carry over the most relevant files."
	cur.Y += titleContentHeight + lipgloss.Height(intro)
	return cur
}

func (h *Handoff) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := h.com.Styles
	width := max(0, min(defaultDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()

	h.input.SetWidth(max(0, innerWidth-t.Dialog.InputPrompt.GetHorizontalFrameSize()-1))
	h.help.SetWidth(innerWidth)

	rc := NewRenderContext(t, width)
	rc.Title = "Create Handoff"
	rc.AddPart("Describe the goal for the next top-level session. Crush will draft the prompt and carry over the most relevant files.")
	rc.AddPart(t.Dialog.InputPrompt.Render(h.input.View()))
	if h.loading {
		rc.Help = h.spinner.View() + " Generating handoff..."
	} else {
		rc.Help = h.help.View(h)
	}

	view := rc.Render()
	cur := h.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

func (h *Handoff) ShortHelp() []key.Binding {
	return []key.Binding{h.keyMap.Confirm, h.keyMap.Close}
}

func (h *Handoff) FullHelp() [][]key.Binding {
	return [][]key.Binding{{h.keyMap.Confirm, h.keyMap.Close}}
}

func (h *Handoff) StartLoading() tea.Cmd {
	if h.loading {
		return nil
	}
	h.loading = true
	return h.spinner.Tick
}

func (h *Handoff) StopLoading() {
	h.loading = false
}
