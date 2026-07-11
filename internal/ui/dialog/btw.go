package dialog

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
)

// BtwID is the identifier for the side-question dialog.
const BtwID = "btw"

const (
	btwMaxWidth          = 80
	btwMinWidth          = 40
	btwMaxViewportHeight = 16
)

type btwState int

const (
	btwStateInput btwState = iota
	btwStateLoading
	btwStateAnswer
)

// SideQuestionResultMsg is returned after an async SideQuestion call.
type SideQuestionResultMsg struct {
	Answer           string
	Err              error
	Model            string
	Provider         string
	PromptTokens     int64
	CompletionTokens int64
	Question         string
}

// Btw is an ephemeral side-question dialog.
type Btw struct {
	com       *common.Common
	sessionID string
	state     btwState
	question  textinput.Model
	viewport  viewport.Model
	spinner   spinner.Model
	help      help.Model
	exchanges []proto.SideQuestionExchange

	answer           string
	errText          string
	model            string
	provider         string
	promptTokens     int64
	completionTokens int64
	pendingQuestion  string
	viewportDirty    bool

	keyMap struct {
		Confirm,
		ScrollUp,
		ScrollDown,
		Close key.Binding
	}
}

var (
	_ Dialog        = (*Btw)(nil)
	_ LoadingDialog = (*Btw)(nil)
)

// NewBtw creates a new side-question dialog for the given session.
func NewBtw(com *common.Common, sessionID string) *Btw {
	d := &Btw{
		com:       com,
		sessionID: sessionID,
		state:     btwStateInput,
	}

	d.help = help.New()
	d.help.Styles = com.Styles.DialogHelpStyles()

	d.keyMap.Confirm = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "ask"),
	)
	d.keyMap.ScrollUp = key.NewBinding(
		key.WithKeys("up", "k", "pgup"),
		key.WithHelp("↑/↓", "scroll"),
	)
	d.keyMap.ScrollDown = key.NewBinding(
		key.WithKeys("down", "j", "pgdown"),
		key.WithHelp("↑/↓", "scroll"),
	)
	d.keyMap.Close = CloseKey

	input := textinput.New()
	input.SetVirtualCursor(false)
	input.SetStyles(com.Styles.TextInput)
	input.Prompt = "> "
	input.Placeholder = "Ask about this session..."
	input.Focus()
	d.question = input

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = com.Styles.Dialog.Spinner
	d.spinner = s

	d.viewport = viewport.New()
	return d
}

// ID implements Dialog.
func (d *Btw) ID() string {
	return BtwID
}

// StartLoading implements LoadingDialog.
func (d *Btw) StartLoading() tea.Cmd {
	if d.state == btwStateLoading {
		return nil
	}
	d.state = btwStateLoading
	return d.spinner.Tick
}

// StopLoading implements LoadingDialog.
func (d *Btw) StopLoading() {
	if d.state == btwStateLoading {
		d.state = btwStateInput
	}
}

// HandleMsg implements Dialog.
func (d *Btw) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case SideQuestionResultMsg:
		if d.state != btwStateLoading {
			return nil
		}
		if msg.Err != nil {
			d.errText = msg.Err.Error()
			d.answer = ""
		} else {
			d.errText = ""
			d.answer = msg.Answer
			d.model = msg.Model
			d.provider = msg.Provider
			d.promptTokens = msg.PromptTokens
			d.completionTokens = msg.CompletionTokens
			if msg.Question != "" {
				d.exchanges = append(d.exchanges, proto.SideQuestionExchange{
					Question: msg.Question,
					Answer:   msg.Answer,
				})
			}
		}
		d.state = btwStateAnswer
		d.viewportDirty = true
		d.question.SetValue("")
		d.question.Blur()
		return nil

	case spinner.TickMsg:
		if d.state == btwStateLoading {
			var cmd tea.Cmd
			d.spinner, cmd = d.spinner.Update(msg)
			return ActionCmd{Cmd: cmd}
		}

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, d.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, d.keyMap.Confirm):
			switch d.state {
			case btwStateInput:
				return d.submit()
			case btwStateAnswer:
				d.state = btwStateInput
				d.answer = ""
				d.errText = ""
				d.question.SetValue("")
				d.question.Focus()
				return nil
			}
		case d.state == btwStateAnswer && key.Matches(msg, d.keyMap.ScrollUp):
			d.viewport.ScrollUp(1)
			return nil
		case d.state == btwStateAnswer && key.Matches(msg, d.keyMap.ScrollDown):
			d.viewport.ScrollDown(1)
			return nil
		case d.state == btwStateInput:
			var cmd tea.Cmd
			d.question, cmd = d.question.Update(msg)
			return ActionCmd{Cmd: cmd}
		}

	case common.CoalescedWheelMsg:
		if d.state == btwStateAnswer {
			d.viewport, _ = d.viewport.Update(tea.MouseWheelMsg(msg.Mouse))
		}

	case tea.PasteMsg:
		if d.state == btwStateInput {
			var cmd tea.Cmd
			d.question, cmd = d.question.Update(msg)
			return ActionCmd{Cmd: cmd}
		}
	}
	return nil
}

func (d *Btw) submit() Action {
	q := strings.TrimSpace(d.question.Value())
	if q == "" {
		return nil
	}
	d.pendingQuestion = q
	d.state = btwStateLoading
	d.errText = ""
	d.answer = ""

	sessionID := d.sessionID
	exchanges := append([]proto.SideQuestionExchange(nil), d.exchanges...)
	com := d.com
	return ActionCmd{Cmd: tea.Batch(
		d.spinner.Tick,
		func() tea.Msg {
			resp, err := com.Workspace.SideQuestion(context.Background(), sessionID, q, exchanges)
			if err != nil {
				return SideQuestionResultMsg{Err: err, Question: q}
			}
			return SideQuestionResultMsg{
				Answer:           resp.Answer,
				Model:            resp.Model,
				Provider:         resp.Provider,
				PromptTokens:     resp.PromptTokens,
				CompletionTokens: resp.CompletionTokens,
				Question:         q,
			}
		},
	)}
}

// Draw implements Dialog.
func (d *Btw) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	s := d.com.Styles
	frameW := s.Dialog.View.GetHorizontalFrameSize()
	width := min(btwMaxWidth, max(btwMinWidth, area.Dx()-4-frameW))

	title := common.DialogTitle(s, "btw", width, s.Dialog.TitleGradFromColor, s.Dialog.TitleGradToColor)
	header := s.Dialog.Title.Render(title)

	var body string
	var cur *tea.Cursor
	switch d.state {
	case btwStateInput:
		d.question.SetWidth(max(10, width-2))
		body = d.question.View()
		cur = InputCursor(s, d.question.Cursor())
		if cur != nil {
			cur.Y += lipgloss.Height(header) + 1
		}
	case btwStateLoading:
		body = d.spinner.View() + " Thinking..."
	case btwStateAnswer:
		body = d.renderAnswer(width)
	}

	helpText := d.helpLine()
	helpView := s.Dialog.HelpView.Width(width).Render(helpText)

	view := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		body,
		"",
		helpView,
	)
	dialogView := s.Dialog.View.Width(width + frameW).Render(view)
	DrawCenterCursor(scr, area, dialogView, cur)
	return cur
}

func (d *Btw) renderAnswer(width int) string {
	s := d.com.Styles
	contentWidth := max(10, width-1)

	var content string
	if d.errText != "" {
		content = s.Dialog.SecondaryText.Width(contentWidth).Render(d.errText)
	} else {
		r := common.MarkdownRenderer(s, contentWidth)
		mu := common.LockMarkdownRenderer(r)
		mu.Lock()
		rendered, err := r.Render(d.answer)
		mu.Unlock()
		if err != nil {
			content = d.answer
		} else {
			content = strings.TrimSpace(rendered)
		}
	}

	contentHeight := lipgloss.Height(content)
	availableHeight := min(btwMaxViewportHeight, max(3, contentHeight))
	needsScrollbar := contentHeight > availableHeight
	viewportWidth := contentWidth
	if needsScrollbar {
		viewportWidth = contentWidth - 1
	}

	if d.viewport.Width() != viewportWidth || d.viewportDirty {
		d.viewportDirty = true
	}
	d.viewport.SetWidth(viewportWidth)
	d.viewport.SetHeight(availableHeight)
	if d.viewportDirty {
		d.viewport.SetContent(content)
		d.viewportDirty = false
	}

	view := d.viewport.View()
	if needsScrollbar {
		sb := common.Scrollbar(s, availableHeight, d.viewport.TotalLineCount(), availableHeight, d.viewport.YOffset())
		view = lipgloss.JoinHorizontal(lipgloss.Top, view, sb)
	}
	return view
}

func (d *Btw) helpLine() string {
	switch d.state {
	case btwStateLoading:
		return d.spinner.View() + " answering · esc close"
	case btwStateAnswer:
		usage := ""
		if d.promptTokens+d.completionTokens > 0 {
			usage = fmt.Sprintf(" · %d+%d tok", d.promptTokens, d.completionTokens)
		}
		model := ""
		if d.model != "" {
			model = " · " + d.model
		}
		if d.errText != "" {
			return "enter ask follow-up · esc close"
		}
		return "↑/↓ scroll · enter ask follow-up · esc close" + model + usage
	default:
		return "enter ask · esc close"
	}
}

// ShortHelp implements help.KeyMap.
func (d *Btw) ShortHelp() []key.Binding {
	switch d.state {
	case btwStateAnswer:
		return []key.Binding{d.keyMap.ScrollUp, d.keyMap.Confirm, d.keyMap.Close}
	default:
		return []key.Binding{d.keyMap.Confirm, d.keyMap.Close}
	}
}

// FullHelp implements help.KeyMap.
func (d *Btw) FullHelp() [][]key.Binding {
	return [][]key.Binding{d.ShortHelp()}
}
