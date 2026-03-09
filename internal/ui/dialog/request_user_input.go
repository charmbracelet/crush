package dialog

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/userinput"
	uv "github.com/charmbracelet/ultraviolet"
)

const (
	RequestUserInputID       = "request_user_input"
	requestUserInputMaxWidth = 84
	customAnswerLabel        = "Other"
	customAnswerDescription  = "Provide a custom answer"
)

type RequestUserInput struct {
	com *common.Common

	request     userinput.Request
	current     int
	selected    int
	customMode  bool
	customInput textinput.Model
	answers     map[string]userinput.Answer
	help        help.Model
	keyMap      requestUserInputKeyMap
}

type requestUserInputKeyMap struct {
	Select   key.Binding
	Next     key.Binding
	Previous key.Binding
	Close    key.Binding
}

func (k requestUserInputKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Select, k.Close}
}

func (k requestUserInputKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Select, k.Close}, {k.Previous, k.Next}}
}

func NewRequestUserInput(com *common.Common, request userinput.Request) *RequestUserInput {
	input := textinput.New()
	input.SetVirtualCursor(false)
	input.Placeholder = "Type your answer"
	input.SetStyles(com.Styles.TextInput)

	helpModel := help.New()
	helpModel.Styles = com.Styles.DialogHelpStyles()

	return &RequestUserInput{
		com:         com,
		request:     request,
		answers:     make(map[string]userinput.Answer, len(request.Questions)),
		customInput: input,
		help:        helpModel,
		keyMap: requestUserInputKeyMap{
			Select:   key.NewBinding(key.WithKeys("enter", "ctrl+y"), key.WithHelp("enter", "confirm")),
			Next:     key.NewBinding(key.WithKeys("down", "ctrl+n"), key.WithHelp("↓", "next")),
			Previous: key.NewBinding(key.WithKeys("up", "ctrl+p"), key.WithHelp("↑", "previous")),
			Close:    CloseKey,
		},
	}
}

func (*RequestUserInput) ID() string {
	return RequestUserInputID
}

func (r *RequestUserInput) Cursor() *tea.Cursor {
	if !r.customMode {
		return nil
	}
	question := r.currentQuestion()
	helper := r.com.Styles.Dialog.SecondaryText.Render("Provide a custom answer and press Enter to continue.")
	cur := InputCursor(r.com.Styles, r.customInput.Cursor())
	if cur == nil {
		return nil
	}
	cur.Y += titleContentHeight + 3 + lipgloss.Height(question.Question) + lipgloss.Height(helper)
	return cur
}

func (r *RequestUserInput) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, r.keyMap.Close):
			if r.customMode {
				r.customMode = false
				r.customInput.Blur()
				return nil
			}
			return ActionResolveUserInput{Response: userinput.Response{
				RequestID:  r.request.ID,
				SessionID:  r.request.SessionID,
				ToolCallID: r.request.ToolCallID,
				Status:     userinput.ResponseStatusCanceled,
				Answers:    r.collectedAnswers(),
			}}
		case key.Matches(msg, r.keyMap.Previous):
			if !r.customMode {
				options := optionsWithOther(r.currentQuestion())
				r.selected = (r.selected + len(options) - 1) % len(options)
			}
			return nil
		case key.Matches(msg, r.keyMap.Next):
			if !r.customMode {
				options := optionsWithOther(r.currentQuestion())
				r.selected = (r.selected + 1) % len(options)
			}
			return nil
		case key.Matches(msg, r.keyMap.Select):
			if r.customMode {
				value := strings.TrimSpace(r.customInput.Value())
				if value == "" {
					return nil
				}
				r.answers[r.currentQuestion().ID] = userinput.Answer{
					QuestionID:  r.currentQuestion().ID,
					CustomInput: value,
				}
				return r.advance()
			}
			question := r.currentQuestion()
			if r.selected == len(question.Options) {
				r.customMode = true
				r.customInput.SetValue("")
				r.customInput.Focus()
				return nil
			}
			selected := question.Options[r.selected]
			r.answers[question.ID] = userinput.Answer{
				QuestionID:     question.ID,
				SelectedOption: selected.Label,
			}
			return r.advance()
		}
	}

	if r.customMode {
		var cmd tea.Cmd
		r.customInput, cmd = r.customInput.Update(msg)
		return ActionCmd{Cmd: cmd}
	}

	return nil
}

func (r *RequestUserInput) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	question := r.currentQuestion()
	title := fmt.Sprintf("%s (%d/%d)", question.Header, r.current+1, len(r.request.Questions))
	if title == fmt.Sprintf(" (%d/%d)", r.current+1, len(r.request.Questions)) {
		title = fmt.Sprintf("Question %d/%d", r.current+1, len(r.request.Questions))
	}

	var parts []string
	parts = append(parts, r.com.Styles.Dialog.Title.Render(title))
	parts = append(parts, "")
	parts = append(parts, question.Question)
	parts = append(parts, "")

	if r.customMode {
		parts = append(parts, r.com.Styles.Dialog.SecondaryText.Render("Provide a custom answer and press Enter to continue."))
		parts = append(parts, "")
		parts = append(parts, r.com.Styles.Dialog.InputPrompt.Render(r.customInput.View()))
	} else {
		for idx, option := range optionsWithOther(question) {
			prefix := "○"
			if idx == r.selected {
				prefix = "●"
			}
			parts = append(parts, fmt.Sprintf("%s %s", prefix, option.Label))
			parts = append(parts, r.com.Styles.Dialog.SecondaryText.Render("  "+option.Description))
			parts = append(parts, "")
		}
		parts = append(parts, r.help.View(r.keyMap))
	}

	content := strings.Join(parts, "\n")
	dialogWidth := min(area.Dx(), requestUserInputMaxWidth)
	r.customInput.SetWidth(max(0, dialogWidth-r.com.Styles.Dialog.View.GetHorizontalFrameSize()-r.com.Styles.Dialog.InputPrompt.GetHorizontalFrameSize()-1))
	rendered := r.com.Styles.Dialog.View.Width(dialogWidth).Render(content)
	cur := r.Cursor()
	DrawCenterCursor(scr, area, rendered, cur)
	return cur
}

func (r *RequestUserInput) advance() Action {
	r.customMode = false
	r.customInput.Blur()
	r.selected = 0
	if r.current == len(r.request.Questions)-1 {
		return ActionResolveUserInput{Response: userinput.Response{
			RequestID:  r.request.ID,
			SessionID:  r.request.SessionID,
			ToolCallID: r.request.ToolCallID,
			Status:     userinput.ResponseStatusSubmitted,
			Answers:    r.collectedAnswers(),
		}}
	}
	r.current++
	return nil
}

func (r *RequestUserInput) collectedAnswers() []userinput.Answer {
	answers := make([]userinput.Answer, 0, len(r.request.Questions))
	for _, question := range r.request.Questions {
		answer, ok := r.answers[question.ID]
		if !ok {
			continue
		}
		answers = append(answers, answer)
	}
	return answers
}

func (r *RequestUserInput) currentQuestion() requestQuestion {
	return requestQuestion(r.request.Questions[r.current])
}

type requestQuestion userinput.Question

func optionsWithOther(question requestQuestion) []userinput.Option {
	q := userinput.Question(question)
	options := make([]userinput.Option, 0, len(question.Options)+1)
	options = append(options, q.Options...)
	options = append(options, userinput.Option{Label: customAnswerLabel, Description: customAnswerDescription})
	return options
}
