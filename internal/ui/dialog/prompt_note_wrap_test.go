package dialog

import (
	"image"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/question"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"
)

const longNote = "remember to keep the migration reversible and tell ops first"

// TestSavedNotesAndAnswersWrap verifies that saved notes and a typed
// "Something else?" answer wrap onto extra rows instead of being cut
// off at the edge, and that the reported height makes room for them.
func TestSavedNotesAndAnswersWrap(t *testing.T) {
	t.Parallel()
	s := styles.CharmtonePantera()

	draw := func(d questionResponder) string {
		h := d.Height(40)
		scr := uv.NewScreenBuffer(40, h)
		d.Draw(scr, image.Rect(0, 0, 40, h))
		return strings.Join(strings.Fields(scr.String()), " ")
	}

	sc := NewSingleChoice(&s, question.Question{
		ID: "q", Type: question.TypeSingleChoice, Text: "Pick",
		Choices: []question.Choice{{ID: "a", Label: "Alpha"}},
	})
	sc.SetFocused(true)
	sc.notes["a"] = longNote
	sc.fillIn.SetValue(longNote)
	out := draw(sc)
	require.Equal(t, 2, strings.Count(out, "ops first"), "note and answer must both be shown whole: %s", out)

	yn := NewYesNo(&s, question.Question{ID: "q", Type: question.TypeYesNo, Text: "Sure?"})
	yn.SetFocused(true)
	yn.notes[question.QuestionNoteKey] = longNote
	out = draw(yn)
	require.Contains(t, out, "ops first", "yes/no note cut off: %s", out)
}
