package dialog

import (
	"image"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

// clickQuestionText draws f and clicks where text is painted.
func clickQuestionText(t *testing.T, f *QuestionForm, area image.Rectangle, text string) (done, handled bool) {
	t.Helper()
	point := paintedTextPoint(t, drawScreen(f, area), text)
	return f.HandleMouseClick(point.X, point.Y)
}

// Clicking another choice closes an open note under the choice it was
// opened for, so later input reaches neither. A click inside the note keeps
// it open.
func TestQuestionClickSavesNoteUnderOriginalChoice(t *testing.T) {
	t.Parallel()
	for _, kind := range []question.Type{question.TypeSingleChoice, question.TypeMultiChoice} {
		for _, test := range []struct {
			name, click string
			selected    []string
			note        string
		}{
			{"other choice", "Beta", []string{"b"}, "Alpha note"},
			{"own note", "Alpha note", []string{"a"}, "Alpha note see 3"},
		} {
			t.Run(string(kind)+"/"+test.name, func(t *testing.T) {
				t.Parallel()
				sty := styles.CharmtonePantera()
				f := NewQuestionForm(&sty, question.Request{Questions: []question.Question{{
					ID: "q", Type: kind, Text: "Pick",
					Choices: []question.Choice{{ID: "a", Label: "Alpha"}, {ID: "b", Label: "Beta"}},
				}}})
				var answers []question.Answer
				f.OnAnswer = func(value []question.Answer) { answers = value }
				area := image.Rect(5, 3, 65, 18)
				f.HandleKey(tea.KeyPressMsg{Code: 'n', Mod: tea.ModAlt})
				f.HandlePaste(tea.PasteMsg{Content: "Alpha note"})
				_, handled := clickQuestionText(t, f, area, test.click)
				require.True(t, handled)
				f.HandlePaste(tea.PasteMsg{Content: " see 3"})
				for range 2 {
					if done, _ := f.HandleKey(enterKey); done {
						break
					}
				}
				require.Len(t, answers, 1)
				require.Equal(t, test.selected, answers[0].SelectedIDs)
				require.Equal(t, map[string]string{"a": test.note}, answers[0].Notes)
			})
		}
	}
}

// Clicking a yes or no button answers with the open note.
func TestQuestionYesNoClickKeepsNote(t *testing.T) {
	t.Parallel()
	sty := styles.CharmtonePantera()
	f := NewQuestionForm(&sty, question.Request{Questions: []question.Question{{
		ID: "q", Type: question.TypeYesNo, Text: "Proceed?",
	}}})
	var answers []question.Answer
	f.OnAnswer = func(value []question.Answer) { answers = value }
	f.HandleKey(tea.KeyPressMsg{Code: 'n', Mod: tea.ModAlt})
	f.HandlePaste(tea.PasteMsg{Content: "draft"})
	done, handled := clickQuestionText(t, f, image.Rect(5, 3, 65, 18), "Yes")
	require.True(t, done && handled)
	require.Len(t, answers, 1)
	require.Equal(t, "draft", answers[0].Notes[question.QuestionNoteKey])
}

// Leaving a question with its note open keeps the draft in the answer,
// also after coming back to the question.
func TestQuestionTabSnapshotsActiveNote(t *testing.T) {
	t.Parallel()
	for _, tabs := range []string{"]]", "][]]"} {
		sty := styles.CharmtonePantera()
		f := NewQuestionForm(&sty, question.Request{Questions: []question.Question{
			{ID: "a", Type: question.TypeYesNo, Text: "First?"},
			{ID: "b", Type: question.TypeYesNo, Text: "Second?"},
		}})
		var answers []question.Answer
		f.OnAnswer = func(value []question.Answer) { answers = value }
		f.HandleKey(tea.KeyPressMsg{Code: 'n', Mod: tea.ModAlt})
		f.HandlePaste(tea.PasteMsg{Content: "Keep this draft"})
		for _, tab := range tabs {
			f.HandleKey(keyMsg(tab))
		}
		done, _ := f.HandleKey(enterKey)
		require.True(t, done, tabs)
		require.Equal(t, "Keep this draft", answers[0].Notes[question.QuestionNoteKey], tabs)
	}
}
