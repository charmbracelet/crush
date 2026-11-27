package dialog

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func newTestBatchForm(t *testing.T) (*QuestionForm, *bool) {
	t.Helper()
	s := styles.CharmtonePantera()
	form := NewQuestionForm(&s, question.Request{
		ID: "b1",
		Questions: []question.Question{
			{
				ID:   "q1",
				Type: question.TypeSingleChoice,
				Text: "Pick one",
				Choices: []question.Choice{
					{ID: "a", Label: "Alpha"},
					{ID: "b", Label: "Beta"},
				},
			},
			{ID: "q2", Type: question.TypeYesNo, Text: "Sure?"},
		},
	})
	form.SetFocused(true)
	cancelled := false
	form.OnCancel = func() { cancelled = true }
	return form, &cancelled
}

func helpFor(bindings []key.Binding, k string) string {
	for _, b := range bindings {
		if b.Help().Key == k {
			return b.Help().Desc
		}
	}
	return ""
}

func TestQuestionFormEscInFillInKeepsBatch(t *testing.T) {
	t.Parallel()
	form, cancelled := newTestBatchForm(t)

	// Move to the fill-in and type an answer.
	form.HandleKey(tea.KeyPressMsg{Code: tea.KeyUp})
	sc := form.questions[0].(*SingleChoice)
	require.True(t, sc.fillIn.Focused())
	form.HandleKey(tea.KeyPressMsg{Code: 'x', Text: "x"})
	require.Equal(t, "back", helpFor(form.ShortHelp(), "esc"))

	done, _ := form.HandleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.False(t, done)
	require.False(t, *cancelled, "esc in the fill-in must not cancel the batch")
	require.False(t, sc.fillIn.Focused())
	require.Equal(t, "x", sc.fillIn.Value())
	require.Equal(t, 0, form.activeIdx, "esc must not advance to the next question")

	// A second esc, with nothing focused, cancels.
	done, _ = form.HandleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.True(t, done)
	require.True(t, *cancelled)
}

func TestQuestionFormEscInNoteKeepsBatch(t *testing.T) {
	t.Parallel()
	form, cancelled := newTestBatchForm(t)

	form.switchTab(1)
	form.HandleKey(tea.KeyPressMsg{Code: 'n', Mod: tea.ModAlt})
	yn := form.questions[1].(*YesNo)
	require.True(t, yn.noteEditor.Focused())
	form.HandleKey(tea.KeyPressMsg{Code: 'h', Text: "h"})
	require.Equal(t, "back", helpFor(form.ShortHelp(), "esc"))

	done, _ := form.HandleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.False(t, done)
	require.False(t, *cancelled, "esc in a note must not cancel the batch")
	require.Equal(t, "h", yn.notes[question.QuestionNoteKey])
}

// TestInlineQuestionEscLabels verifies each inline question labels esc
// by what it does: it cancels the batch from a question, goes back from
// the confirm tab, and goes back from the plan feedback editor.
func TestInlineQuestionEscLabels(t *testing.T) {
	t.Parallel()
	form, _ := newTestBatchForm(t)
	require.Equal(t, "cancel", helpFor(form.ShortHelp(), "esc"), "choice question")
	form.switchTab(1)
	require.Equal(t, "cancel", helpFor(form.ShortHelp(), "esc"), "yes/no question")
	form.switchTab(2)
	require.Equal(t, "back", helpFor(form.ShortHelp(), "esc"), "confirm tab")

	p := newTestPlanHandoff()
	p.HandleKey(tea.KeyPressMsg{Code: 'n', Text: "n"})
	require.Equal(t, "back", helpFor(p.ShortHelp(), "esc"), "plan feedback editor")
}

// TestNewlineHintMatchesAcrossEditors verifies free-text answers and
// plan feedback advertise the same newline key.
func TestNewlineHintMatchesAcrossEditors(t *testing.T) {
	t.Parallel()
	s := styles.CharmtonePantera()
	ft := NewFreeText(&s, question.Question{ID: "q", Type: question.TypeFreeText, Text: "Why?"})
	p := newTestPlanHandoff()
	p.HandleKey(tea.KeyPressMsg{Code: 'n', Text: "n"})
	require.Equal(t, "newline", helpFor(ft.ShortHelp(), "ctrl+j"))
	require.Equal(t, "newline", helpFor(p.ShortHelp(), "ctrl+j"))
}

// TestQuestionLabelsTruncateByWidth verifies auto tab labels are cut by
// width with an ellipsis, and the confirm summary names the full
// question instead of the clipped tab label.
func TestQuestionLabelsTruncateByWidth(t *testing.T) {
	t.Parallel()
	require.Equal(t, "Use Postgres", shortLabel("Use Postgres"))
	long := shortLabel("Use Postgres for the analytics store?")
	require.True(t, strings.HasSuffix(long, "…"), long)
	require.Greater(t, len(long), len("Use Postgres for"))

	s := styles.CharmtonePantera()
	form := NewQuestionForm(&s, question.Request{
		ID: "b",
		Questions: []question.Question{
			{ID: "q1", Type: question.TypeYesNo, Text: "Use Postgres for the analytics store?"},
			{ID: "q2", Type: question.TypeYesNo, Text: "Deploy now?"},
		},
	})
	require.Equal(t, "Use Postgres for the analytics store?", form.confirmComp.QuestionLabels[0])
}
