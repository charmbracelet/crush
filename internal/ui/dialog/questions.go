package dialog

import (
	"log/slog"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ask_question"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
)

const QuestionsID = "questions"

type Questions struct {
	com *common.Common

	// Input
	req ask_question.QuestionsRequest

	// State
	currQuestion           int
	selectedOptsByQuestion map[int]map[int]bool // map[questionIdx]map[optionIdx]bool

	// Keyboard
	keyMap struct {
		UpDown   key.Binding
		Next     key.Binding
		Previous key.Binding
		Select   key.Binding
		Submit   key.Binding
		Close    key.Binding
	}

	// UI
	list *questionOptionsList
	help help.Model
}

func NewAskQuestionDialog(com *common.Common, req ask_question.QuestionsRequest) *Questions {
	d := &Questions{
		com:                    com,
		req:                    req,
		currQuestion:           0,
		selectedOptsByQuestion: make(map[int]map[int]bool),
		list:                   newQuestionOptionsList(com.Styles),
		help:                   help.New(),
	}

	d.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑/↓", "choose"),
	)
	d.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("↓", "next option"),
	)
	d.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑", "previous option"),
	)
	d.keyMap.Select = key.NewBinding(
		key.WithKeys("space"),
		key.WithHelp("space", "select"),
	)
	d.keyMap.Submit = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "confirm"),
	)
	d.keyMap.Close = CloseKey

	d.list.Focus()
	d.initList()
	d.list.SetSelected(0)

	d.help.Styles = com.Styles.DialogHelpStyles()

	return d
}

func (q *Questions) ID() string {
	return QuestionsID
}

func (q *Questions) initList() {
	// Return early if there are no questions (it should never happen)
	if len(q.req.Questions) == 0 {
		return
	}

	// Initialize map of selected options for current question
	if q.selectedOptsByQuestion[q.currQuestion] == nil {
		q.selectedOptsByQuestion[q.currQuestion] = make(map[int]bool)
	}

	q.refreshList()
	q.list.SelectFirst()
}

func (q *Questions) refreshList() {
	q.list.SetQuestion(
		q.req.Questions[q.currQuestion],
		q.selectedOptsByQuestion[q.currQuestion],
	)
}

func (q *Questions) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, q.keyMap.Close):
			return ActionAskQuestionResponse{Response: ask_question.AnswersResponse{}}
		case key.Matches(msg, q.keyMap.Previous):
			q.list.Focus()
			if q.list.IsSelectedFirst() {
				q.list.SelectLast()
			} else {
				q.list.SelectPrev()
			}
			q.list.ScrollToSelected()
		case key.Matches(msg, q.keyMap.Next):
			q.list.Focus()
			if q.list.IsSelectedLast() {
				q.list.SelectFirst()
			} else {
				q.list.SelectNext()
			}
			q.list.ScrollToSelected()
		case key.Matches(msg, q.keyMap.Select):
			currQ := q.req.Questions[q.currQuestion]
			idx := q.list.Selected()
			if idx < 0 {
				break
			}
			if !currQ.MultiSelect {
				q.selectedOptsByQuestion[q.currQuestion] = make(map[int]bool)
				q.selectedOptsByQuestion[q.currQuestion][idx] = true
			} else {
				q.selectedOptsByQuestion[q.currQuestion][idx] = !q.selectedOptsByQuestion[q.currQuestion][idx]
			}
			q.refreshList()
		case key.Matches(msg, q.keyMap.Submit):
			if q.currQuestion < len(q.req.Questions)-1 {
				q.currQuestion++
				q.initList()
			} else {
				// Loop over all the Questions to assemble the Answers response
				res := ask_question.NewAnswersResponse(&q.req)
				for questIdx, quest := range q.req.Questions {
					// Create an Answer for each Question
					ans := ask_question.NewAnswer(quest)
					for optIdx, optSelected := range q.selectedOptsByQuestion[questIdx] {
						// If the option is selected, select it on the Answer too
						if optSelected {
							ans.Select(quest.Options[optIdx].Label)
						}
					}
					res.SetAnswerAt(questIdx, ans)
				}

				// Check if the response is complete: this should always be true
				if !res.IsComplete() {
					slog.Warn("AskQuestionDialog: incomplete response, missing answers", "want", cap(res.Answers), "got", len(res.Answers))
				}

				return ActionAskQuestionResponse{Response: res}
			}
		}
	}
	return nil
}

func (q *Questions) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	// Return early if there are no questions (it should never happen)
	if len(q.req.Questions) == 0 {
		return nil
	}
	// Determine current question
	currQ := q.req.Questions[q.currQuestion]

	// Styles shorthand
	t := q.com.Styles

	// Figure out dimensions
	width := max(0, min(defaultDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	height := max(0, min(defaultDialogHeight, area.Dy()-t.Dialog.View.GetVerticalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.InputPrompt.GetVerticalFrameSize() + inputContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()

	// Set dimensions for List and Help bar
	q.list.SetSize(innerWidth, height-heightOffset)
	q.help.SetWidth(innerWidth)

	rc := NewRenderContext(t, width)

	// Render: Title and Question's Header
	rc.Title = "Question"
	rc.TitleInfo = t.HalfMuted.Padding(0, 1).Render(currQ.Header)

	// Render: Question
	questionText := t.Dialog.TitleAccent.Italic(true).Padding(1, 2).Render(currQ.Question)
	rc.AddPart(questionText)

	// Render: Question's Options
	listView := t.Dialog.List.Height(q.list.Height()).Render(q.list.Render())
	rc.AddPart(listView)

	// Render: Help
	rc.Help = q.help.View(q)

	view := rc.Render()
	DrawCenterCursor(scr, area, view, nil)

	return nil
}

// ShortHelp returns the short help view.
func (q *Questions) ShortHelp() []key.Binding {
	h := []key.Binding{
		q.keyMap.UpDown,
		q.keyMap.Select,
		q.keyMap.Submit,
	}
	h = append(h, q.keyMap.Close)
	return h
}

// FullHelp returns the full help view.
func (q *Questions) FullHelp() [][]key.Binding {
	return [][]key.Binding{q.ShortHelp()}
}
