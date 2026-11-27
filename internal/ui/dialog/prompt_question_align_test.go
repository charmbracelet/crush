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

// TestQuestionAlignment verifies every question type wraps its text
// with a hanging indent under the first word and indents descriptions
// to the same gutter as the choices, at the reported height.
func TestQuestionAlignment(t *testing.T) {
	t.Parallel()
	s := styles.CharmtonePantera()
	text := "Should we migrate the analytics store to Postgres this sprint?"
	desc := "This affects the nightly jobs and the dashboards team."
	comps := map[string]questionResponder{
		"single": NewSingleChoice(&s, question.Question{
			ID: "q", Type: question.TypeSingleChoice, Text: text, Description: desc,
			Choices: []question.Choice{{ID: "a", Label: "Yes"}},
		}),
		"yesno":    NewYesNo(&s, question.Question{ID: "q", Type: question.TypeYesNo, Text: text, Description: desc}),
		"freetext": NewFreeText(&s, question.Question{ID: "q", Type: question.TypeFreeText, Text: text, Description: desc}),
		"confirm": NewConfirmComponent(&s, text, desc, []string{"One"},
			[]question.Question{{ID: "q"}}, []*question.Answer{nil}),
	}
	for name, q := range comps {
		q.SetFocused(true)
		h := q.Height(40)
		scr := uv.NewScreenBuffer(40, h)
		q.Draw(scr, image.Rect(0, 0, 40, h))
		lines := strings.Split(scr.String(), "\n")
		textCol := strings.Index(lines[0], "Should")
		require.Positive(t, textCol, name)
		require.Equalf(t, strings.Repeat(" ", textCol), lines[1][:textCol], "%s: wrapped question not hung:\n%s", name, scr.String())
		require.NotEqualf(t, ' ', rune(lines[1][textCol]), "%s: continuation misaligned:\n%s", name, scr.String())
		for _, l := range lines {
			if strings.Contains(l, "nightly jobs") {
				require.Truef(t, strings.HasPrefix(l, questionGutter) && l[len(questionGutter)] != ' ',
					"%s: description not on the gutter: %q", name, l)
			}
		}
	}
}
