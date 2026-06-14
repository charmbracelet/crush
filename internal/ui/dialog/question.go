package dialog

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
)

// QuestionID is the identifier for the question dialog.
const QuestionID = "question"

// Question is the dialog shown when the model asks the user a clarifying question.
type Question struct {
	com     *common.Common
	request question.QuestionRequest

	// selectedOption is the index of the currently highlighted option (-1 = none).
	selectedOption int
	// selectedOptions tracks selected options in multi-select mode.
	selectedOptions []bool
	// inputFocused is true when the text input has keyboard focus.
	inputFocused bool
	input        textinput.Model

	help   help.Model
	keyMap questionKeyMap
}

type questionKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Select key.Binding
	Num1   key.Binding
	Num2   key.Binding
	Num3   key.Binding
	Num4   key.Binding
	Num5   key.Binding
	Num6   key.Binding
	Num7   key.Binding
	Num8   key.Binding
	Num9   key.Binding
	Toggle key.Binding
	Close  key.Binding
}

func defaultQuestionKeyMap() questionKeyMap {
	return questionKeyMap{
		Up:     key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "up")),
		Down:   key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "down")),
		Select: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
		Num1:   key.NewBinding(key.WithKeys("1"), key.WithHelp("1-9", "quick select")),
		Num2:   key.NewBinding(key.WithKeys("2")),
		Num3:   key.NewBinding(key.WithKeys("3")),
		Num4:   key.NewBinding(key.WithKeys("4")),
		Num5:   key.NewBinding(key.WithKeys("5")),
		Num6:   key.NewBinding(key.WithKeys("6")),
		Num7:   key.NewBinding(key.WithKeys("7")),
		Num8:   key.NewBinding(key.WithKeys("8")),
		Num9:   key.NewBinding(key.WithKeys("9")),
		Toggle: key.NewBinding(key.WithKeys("space", " "), key.WithHelp("space", "toggle")),
		Close:  CloseKey,
	}
}

var _ Dialog = (*Question)(nil)

// NewQuestion creates a new question dialog.
func NewQuestion(com *common.Common, req question.QuestionRequest) *Question {
	input := textinput.New()
	input.SetVirtualCursor(false)
	input.SetStyles(com.Styles.TextInput)
	input.Placeholder = "Type your answer..."
	input.Prompt = "> "

	h := help.New()
	h.Styles = com.Styles.DialogHelpStyles()

	startInInputMode := len(req.Options) == 0
	if startInInputMode {
		input.Focus()
	} else {
		input.Blur()
	}

	return &Question{
		com:             com,
		request:         req,
		selectedOption:  0,
		selectedOptions: make([]bool, len(req.Options)),
		inputFocused:    startInInputMode,
		input:           input,
		help:            h,
		keyMap:          defaultQuestionKeyMap(),
	}
}

// ID implements Dialog.
func (q *Question) ID() string { return QuestionID }

// HandleMsg implements Dialog.
func (q *Question) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		// Number shortcuts always work — immediately select and submit.
		numBindings := []key.Binding{q.keyMap.Num1, q.keyMap.Num2, q.keyMap.Num3, q.keyMap.Num4, q.keyMap.Num5, q.keyMap.Num6, q.keyMap.Num7, q.keyMap.Num8, q.keyMap.Num9}
		for i, kb := range numBindings {
			if key.Matches(msg, kb) && i < len(q.request.Options) {
				if q.request.AllowMultiple {
					q.toggleOption(i)
					return nil
				}
				return q.respond(q.request.Options[i])
			}
		}

		if q.inputFocused && key.Matches(msg, q.keyMap.Toggle) {
			var cmd tea.Cmd
			q.input, cmd = q.input.Update(tea.KeyPressMsg(tea.Key{Text: " ", Code: ' '}))
			return ActionCmd{Cmd: cmd}
		}

		switch {
		case key.Matches(msg, q.keyMap.Close):
			return q.respond("")
		case key.Matches(msg, q.keyMap.Up):
			if q.inputFocused {
				// From text input → back to last option.
				q.inputFocused = false
				q.input.Blur()
				if len(q.request.Options) > 0 {
					q.selectedOption = len(q.request.Options) - 1
				}
			} else if q.selectedOption > 0 {
				q.selectedOption--
			}
		case key.Matches(msg, q.keyMap.Down):
			if !q.inputFocused {
				if q.selectedOption < len(q.request.Options)-1 {
					q.selectedOption++
				} else {
					// Past last option → switch to text input.
					q.inputFocused = true
					q.input.Focus()
				}
			}
		case key.Matches(msg, q.keyMap.Toggle) && q.request.AllowMultiple && !q.inputFocused:
			q.toggleOption(q.selectedOption)
		case key.Matches(msg, q.keyMap.Select):
			if q.request.AllowMultiple {
				answers := q.selectedAnswers()
				if val := strings.TrimSpace(q.input.Value()); val != "" {
					answers = append(answers, val)
				}
				if len(answers) > 0 {
					return q.respond(answers...)
				}
			} else if q.inputFocused {
				val := strings.TrimSpace(q.input.Value())
				if val != "" {
					return q.respond(val)
				}
			} else if q.selectedOption >= 0 && q.selectedOption < len(q.request.Options) {
				return q.respond(q.request.Options[q.selectedOption])
			}
		default:
			if q.inputFocused {
				var cmd tea.Cmd
				q.input, cmd = q.input.Update(msg)
				return ActionCmd{Cmd: cmd}
			}
		}
	case tea.PasteMsg:
		if q.inputFocused {
			var cmd tea.Cmd
			q.input, cmd = q.input.Update(msg)
			return ActionCmd{Cmd: cmd}
		}
	}
	return nil
}

func (q *Question) toggleOption(index int) {
	if index >= 0 && index < len(q.selectedOptions) {
		q.selectedOptions[index] = !q.selectedOptions[index]
	}
}

func (q *Question) selectedAnswers() []string {
	answers := make([]string, 0, len(q.selectedOptions))
	for i, selected := range q.selectedOptions {
		if selected && i < len(q.request.Options) {
			answers = append(answers, q.request.Options[i])
		}
	}
	return answers
}

func (q *Question) respond(answers ...string) Action {
	return ActionQuestionResponse{
		Request:  q.request,
		Response: question.QuestionResponse{Answers: answers},
	}
}

// Draw implements Dialog.
func (q *Question) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	s := q.com.Styles

	width := min(defaultDialogMaxWidth, area.Dx()-4)
	innerWidth := width - s.Dialog.View.GetHorizontalFrameSize()

	rc := NewRenderContext(s, width)
	rc.Title = "Question"
	rc.Gap = 1

	// Question text — wrap to inner width.
	questionText := lipgloss.NewStyle().Width(innerWidth).Render(q.request.Question)
	rc.AddPart(questionText)

	// Options list.
	if len(q.request.Options) > 0 {
		var lines []string
		for i, opt := range q.request.Options {
			var line string
			numStr := fmt.Sprintf("%d", i+1)
			marker := ""
			if q.request.AllowMultiple {
				marker = "[ ] "
				if i < len(q.selectedOptions) && q.selectedOptions[i] {
					marker = "[x] "
				}
			}
			if !q.inputFocused && i == q.selectedOption {
				line = s.Dialog.SelectedItem.Width(innerWidth).Render(" > " + numStr + ". " + marker + opt)
			} else {
				line = s.Dialog.NormalItem.Width(innerWidth).Render("   " + numStr + ". " + marker + opt)
			}
			lines = append(lines, line)
		}
		rc.AddPart(lipgloss.JoinVertical(lipgloss.Left, lines...))
	}

	// Free-text input.
	inputLabel := lipgloss.NewStyle().Faint(true).Render("Or type your answer:")
	q.input.SetWidth(innerWidth - s.Dialog.InputPrompt.GetHorizontalFrameSize())
	inputView := s.Dialog.InputPrompt.Render(q.input.View())
	rc.AddPart(lipgloss.JoinVertical(lipgloss.Left, inputLabel, inputView))

	// Help bar.
	rc.Help = q.help.View(q)

	view := rc.Render()

	// Cursor — only when input is focused.
	var cur *tea.Cursor
	if q.inputFocused {
		cur = InputCursor(s, q.input.Cursor())
		if cur != nil {
			// InputCursor adds the title frame + input style frame (top+bottom).
			// The input style bottom frame effectively covers the title content line,
			// so we only need to add the gap after the title, then question + options + label.
			linesAbove := rc.Gap // gap between title and question text
			linesAbove += lipgloss.Height(questionText) + rc.Gap
			if len(q.request.Options) > 0 {
				linesAbove += len(q.request.Options) + rc.Gap
			}
			linesAbove++ // "Or type your answer:" label line
			cur.Y += linesAbove
		}
	}

	DrawCenterCursor(scr, area, view, cur)
	return cur
}

// ShortHelp implements help.KeyMap.
func (q *Question) ShortHelp() []key.Binding {
	if q.request.AllowMultiple {
		return []key.Binding{q.keyMap.Up, q.keyMap.Down, q.keyMap.Toggle, q.keyMap.Select, q.keyMap.Num1, q.keyMap.Close}
	}
	return []key.Binding{q.keyMap.Up, q.keyMap.Down, q.keyMap.Select, q.keyMap.Num1, q.keyMap.Close}
}

// FullHelp implements help.KeyMap.
func (q *Question) FullHelp() [][]key.Binding {
	return [][]key.Binding{q.ShortHelp()}
}
