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

// TestChoiceHeightCountsWrappedLabels verifies that a label wrapping
// onto two rows is counted as two, so drawing at the reported height
// shows the whole list with no scrollbar and the question on top.
func TestChoiceHeightCountsWrappedLabels(t *testing.T) {
	t.Parallel()
	s := styles.CharmtonePantera()
	req := question.Question{
		ID: "q", Text: "Which database should we use?",
		Choices: []question.Choice{
			{ID: "a", Label: "PostgreSQL with the pgvector extension enabled"},
			{ID: "b", Label: "SQLite"},
		},
	}
	single := req
	single.Type = question.TypeSingleChoice
	multi := req
	multi.Type = question.TypeMultiChoice
	for name, d := range map[string]questionResponder{
		"single": NewSingleChoice(&s, single),
		"multi":  NewMultiChoice(&s, multi),
	} {
		d.SetFocused(true)
		h := d.Height(40)
		scr := uv.NewScreenBuffer(40, h)
		d.Draw(scr, image.Rect(0, 0, 40, h))
		out := scr.String()
		lines := strings.Split(out, "\n")
		require.Containsf(t, lines[0], "Which database", "%s: question scrolled off:\n%s", name, out)
		require.NotContainsf(t, out, "│", "%s: spurious scrollbar:\n%s", name, out)
		require.Containsf(t, out, "Something else?", "%s:\n%s", name, out)
	}
}
