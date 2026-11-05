package dialog

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

// printableKey builds a key press for a printable character.
func printableKey(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

// newTestForm builds a question form from the given questions.
func newTestForm(t *testing.T, questions ...question.Question) *QuestionForm {
	t.Helper()
	s := styles.CharmtonePantera()
	return NewQuestionForm(&s, question.Request{ID: "batch", Questions: questions})
}

func singleChoiceQuestion(id string) question.Question {
	return question.Question{
		ID:   id,
		Type: question.TypeSingleChoice,
		Text: "Pick one",
		Choices: []question.Choice{
			{ID: "a", Label: "Alpha"},
			{ID: "b", Label: "Beta"},
		},
	}
}

func multiChoiceQuestion(id string) question.Question {
	q := singleChoiceQuestion(id)
	q.Type = question.TypeMultiChoice
	return q
}

func freeTextQuestion(id string) question.Question {
	return question.Question{
		ID:   id,
		Type: question.TypeFreeText,
		Text: "Explain",
	}
}

// choiceListOf returns the choice list of the active tab, failing the
// test if the active tab is not a choice question.
func choiceListOf(t *testing.T, f *QuestionForm) *choiceList {
	t.Helper()
	switch c := f.questions[f.activeIdx].(type) {
	case *SingleChoice:
		return &c.choiceList
	case *MultiChoice:
		return &c.choiceList
	default:
		t.Fatalf("active tab is not a choice question: %T", f.questions[f.activeIdx])
		return nil
	}
}

// moveOntoFillIn navigates the active tab's cursor onto the fill-in row.
func moveOntoFillIn(t *testing.T, f *QuestionForm) {
	t.Helper()
	cl := choiceListOf(t, f)
	for !cl.isFillIn() {
		f.HandleKey(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	require.True(t, cl.isFillIn(), "cursor should be on the fill-in row")
}

// TestFormTabKeysSwitchTabsWhenNotEditing verifies that [ and ] keep
// switching tabs while no text input is being edited.
func TestFormTabKeysSwitchTabsWhenNotEditing(t *testing.T) {
	t.Parallel()

	f := newTestForm(t, singleChoiceQuestion("q1"), singleChoiceQuestion("q2"))
	require.Equal(t, 0, f.activeIdx)

	f.HandleKey(printableKey(']'))
	require.Equal(t, 1, f.activeIdx, "']' should move to the next tab")

	f.HandleKey(printableKey('['))
	require.Equal(t, 0, f.activeIdx, "'[' should move to the previous tab")
}

// TestFillInRequiresEnterToActivate verifies that moving onto the
// fill-in row only highlights it, and that enter starts text entry.
func TestFillInRequiresEnterToActivate(t *testing.T) {
	t.Parallel()

	f := newTestForm(t, singleChoiceQuestion("q1"), singleChoiceQuestion("q2"))
	cl := choiceListOf(t, f)

	moveOntoFillIn(t, f)
	require.False(t, cl.fillIn.Focused(), "navigation must not start text entry")
	require.False(t, f.activeEditing())

	done, _ := f.HandleKey(enterKey)
	require.False(t, done, "activating the fill-in should not submit")
	require.True(t, cl.fillIn.Focused(), "enter should activate text entry")
	require.True(t, f.activeEditing())
}

// TestBracketsReachFillInWhileEditing verifies the reported bug: while
// the fill-in is being edited, [ and ] are typed instead of switching
// tabs, and the form stops advertising the tab bindings.
func TestBracketsReachFillInWhileEditing(t *testing.T) {
	t.Parallel()

	f := newTestForm(t, singleChoiceQuestion("q1"), singleChoiceQuestion("q2"))
	cl := choiceListOf(t, f)

	moveOntoFillIn(t, f)
	f.HandleKey(enterKey)
	require.True(t, cl.fillIn.Focused())

	f.HandleKey(printableKey('['))
	f.HandleKey(printableKey(']'))

	require.Equal(t, 0, f.activeIdx, "brackets must not switch tabs while editing")
	require.Equal(t, "[]", cl.fillIn.Value(), "brackets must reach the fill-in")

	for _, b := range f.ShortHelp() {
		require.NotContains(t, b.Keys(), "[", "tab bindings must be unbound while editing")
		require.NotContains(t, b.Keys(), "]", "tab bindings must be unbound while editing")
	}
}

// TestEnterAcceptsFillInAndRestoresTabKeys verifies that enter accepts
// the fill-in input and that tab keys work again afterwards.
func TestEnterAcceptsFillInAndRestoresTabKeys(t *testing.T) {
	t.Parallel()

	f := newTestForm(t, singleChoiceQuestion("q1"), singleChoiceQuestion("q2"))
	cl := choiceListOf(t, f)

	moveOntoFillIn(t, f)
	f.HandleKey(enterKey)
	f.HandleKey(printableKey('x'))

	done, _ := f.HandleKey(enterKey)
	require.False(t, done, "a multi-question batch waits for confirmation")
	require.False(t, cl.fillIn.Focused(), "enter must leave text entry")
	require.Equal(t, 1, f.activeIdx, "the answer should advance to the next tab")

	f.HandleKey(printableKey('['))
	require.Equal(t, 0, f.activeIdx, "'[' should switch tabs once editing stops")
}

// TestEscReturnsToSelectionWithoutSubmitting verifies that escape
// leaves text entry, keeps the typed text, and stays on the same tab.
func TestEscReturnsToSelectionWithoutSubmitting(t *testing.T) {
	t.Parallel()

	f := newTestForm(t, singleChoiceQuestion("q1"), singleChoiceQuestion("q2"))
	cl := choiceListOf(t, f)

	moveOntoFillIn(t, f)
	f.HandleKey(enterKey)
	f.HandleKey(printableKey('x'))

	done, _ := f.HandleKey(tea.KeyPressMsg{Code: tea.KeyEsc})
	require.False(t, done, "esc must not submit")
	require.Equal(t, 0, f.activeIdx, "esc must stay on the same tab")
	require.False(t, cl.fillIn.Focused(), "esc must leave text entry")
	require.Equal(t, "x", cl.fillIn.Value(), "esc must keep the typed text")

	f.HandleKey(printableKey(']'))
	require.Equal(t, 1, f.activeIdx, "']' should switch tabs once editing stops")
}

// TestMultiFillInEnterAcceptsWithoutSubmitting verifies that in a
// multi-choice question, enter accepts the fill-in text but leaves the
// question active so choices can still be toggled.
func TestMultiFillInEnterAcceptsWithoutSubmitting(t *testing.T) {
	t.Parallel()

	f := newTestForm(t, multiChoiceQuestion("q1"), singleChoiceQuestion("q2"))
	cl := choiceListOf(t, f)

	moveOntoFillIn(t, f)
	f.HandleKey(enterKey)
	require.True(t, cl.fillIn.Focused(), "enter should activate text entry")

	f.HandleKey(printableKey('x'))
	done, _ := f.HandleKey(enterKey)
	require.False(t, done, "multi-choice waits for the confirming tab")
	require.Equal(t, 0, f.activeIdx, "the multi-choice question stays active")
	require.False(t, cl.fillIn.Focused(), "enter must leave text entry")
	require.Equal(t, "x", cl.fillIn.Value())
	require.Equal(t, "x", f.questions[0].Response().FillInText)
}

// TestFreeTextBracketsReachEditor verifies that a free-text answer
// editor absorbs brackets while editing, and that esc returns to
// selection so [ and ] switch tabs again.
func TestFreeTextBracketsReachEditor(t *testing.T) {
	t.Parallel()

	f := newTestForm(t, freeTextQuestion("q1"), singleChoiceQuestion("q2"))
	ft, ok := f.questions[0].(*FreeText)
	require.True(t, ok)
	require.True(t, ft.editor.Focused(), "free text should start ready for input")

	f.HandleKey(printableKey('['))
	require.Equal(t, 0, f.activeIdx, "the bracket must not switch tabs")
	require.Equal(t, "[", ft.editor.Value(), "the bracket must reach the editor")

	done, _ := f.HandleKey(tea.KeyPressMsg{Code: tea.KeyEsc})
	require.False(t, done, "esc must not submit")
	require.False(t, ft.editor.Focused(), "esc should leave text entry")
	require.Equal(t, "[", ft.editor.Value(), "esc should keep the typed text")

	f.HandleKey(enterKey)
	require.True(t, ft.editor.Focused(), "enter should reactivate text entry")
	require.Equal(t, 0, f.activeIdx)

	f.HandleKey(tea.KeyPressMsg{Code: tea.KeyEsc})
	f.HandleKey(printableKey(']'))
	require.Equal(t, 1, f.activeIdx, "']' should switch tabs once editing stops")
}
