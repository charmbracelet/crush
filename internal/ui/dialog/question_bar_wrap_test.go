package dialog

import (
	"testing"

	"github.com/charmbracelet/crush/internal/question"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// The gutter bar marks which item the selection is on. A label or
// description long enough to wrap must carry it down every row, or the
// bar breaks into disconnected stubs and one item reads as several.
//
// The keyboard path always did this. Hover lit the bar on the first row
// of each block and dropped it on the rest, which is what the bug looked
// like on screen: a lit line, a dark line, a lit line.
func TestChoiceBarSurvivesWrapping(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	req := question.Question{
		ID:   "q1",
		Type: question.TypeSingleChoice,
		Text: "Pick one",
		Choices: []question.Choice{
			{
				ID:          "a",
				Label:       "Keep scene grouping, accept the rewinds",
				Description: "Scenes stay whole; 273 groups, avg 3.8 photos",
			},
			{ID: "b", Label: "Groups must be consecutive runs"},
		},
	}

	for _, tc := range []struct {
		name  string
		setUp func(d *SingleChoice)
	}{
		{"keyboard", func(d *SingleChoice) { d.mouseActive = false; d.cursorIdx = 0 }},
		{"hover", func(d *SingleChoice) { d.mouseActive = true; d.hoveredChoice = 0 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			d := NewSingleChoice(&s, req)
			tc.setUp(d)

			// A label of two rows, stated outright rather than by
			// restating how the component wraps one. The width is narrow
			// enough that the description wraps as well.
			lines := d.buildLines(36, "> ", func(int, question.Choice, bool, int) string {
				return "label row one\nlabel row two"
			})

			var rows int
			for _, ln := range lines {
				if ln.choiceIdx != 0 || ansi.Strip(ln.text) == "" {
					continue
				}
				rows++
				require.Contains(t, ansi.Strip(ln.text), "┃",
					"every row of the highlighted choice needs the bar, got %q", ansi.Strip(ln.text))
			}
			require.Greater(t, rows, 2, "the choice must span several rows for this to test anything")
		})
	}
}
