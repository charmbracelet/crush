package dialog

import (
	"image"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// clickQuestionText draws f and clicks where text is painted.
func clickQuestionText(t *testing.T, f *QuestionForm, area image.Rectangle, text string) (done, handled bool) {
	t.Helper()
	point := paintedTextPoint(t, drawScreen(f, area), text)
	return f.HandleMouseClick(point.X, point.Y)
}

func TestQuestionButtonsRejectClippedCells(t *testing.T) {
	t.Parallel()
	sty := styles.CharmtonePantera()
	d := NewYesNo(&sty, question.Question{ID: "yes", Text: "Continue?"})
	// A narrow viewport clips the Yes button, so a click on its row just
	// right of the viewport misses.
	narrow := image.Rect(5, 3, 12, 10)
	yes := paintedTextPoint(t, drawScreen(d, narrow), "Yes")
	done, handled := d.HandleMouseClick(narrow.Max.X, yes.Y)
	require.False(t, done)
	require.False(t, handled)

	// A viewport that ends a row above the buttons hides them, so a click
	// where they would be misses. Starting a rectangle below its bottom
	// swaps its corners, which once painted the buttons below it.
	d.Request.Description = strings.Repeat("Context\n", 30)
	yes = paintedTextPoint(t, drawScreen(d, image.Rect(5, 3, 65, 60)), "Yes")
	area := image.Rect(5, 3, 65, yes.Y-1)
	screen := uv.NewScreenBuffer(80, 60)
	d.Draw(screen, area)
	for _, line := range strings.Split(ansi.Strip(screen.Render()), "\n")[area.Max.Y:] {
		require.Empty(t, strings.TrimSpace(line), "offscreen buttons cannot paint outside the question viewport")
	}
	done, handled = d.HandleMouseClick(yes.X, yes.Y)
	require.False(t, done)
	require.False(t, handled)

	d.Request.Description = "Context"
	d.notes[question.QuestionNoteKey] = "Saved note"
	screen = uv.NewScreenBuffer(80, 40)
	d.Draw(screen, narrow)
	require.NotContains(t, ansi.Strip(screen.Render()), "Saved", "a note below the viewport stays hidden")
}

// A click where Yup! was painted before the summary scrolled it away
// does not submit.
func TestQuestionConfirmHiddenButtonsCannotSubmit(t *testing.T) {
	t.Parallel()
	sty := styles.CharmtonePantera()
	c := NewConfirmComponent(&sty, "Confirm", strings.Repeat("Context\n", 30), nil, nil, nil)
	submitted := false
	c.OnConfirm = func() { submitted = true }
	area := image.Rect(5, 3, 65, 12)
	c.scrollOffset = 1000
	point := paintedTextPoint(t, drawScreen(c, area), "Yup!")
	c.scrollOffset = 0
	drawScreen(c, area)
	done, handled := c.HandleMouseClick(point.X, point.Y)
	require.False(t, done)
	require.False(t, handled)
	require.False(t, submitted)
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
