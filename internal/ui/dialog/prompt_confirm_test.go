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

// TestConfirmSummaryWraps verifies that long answers on the confirm tab
// wrap under their bullet and the reported height fits them all, so
// the user can read every answer before confirming.
func TestConfirmSummaryWraps(t *testing.T) {
	t.Parallel()
	s := styles.CharmtonePantera()
	answers := []*question.Answer{
		{QuestionID: "q1", FillInText: "PostgreSQL with pgvector and nightly backups to s3"},
		{QuestionID: "q2", FillInText: "Yes"},
	}
	c := NewConfirmComponent(&s, "", "", []string{"Database", "Deploy"},
		[]question.Question{{ID: "q1"}, {ID: "q2"}}, answers)
	h := c.Height(40)
	scr := uv.NewScreenBuffer(40, h)
	c.Draw(scr, image.Rect(0, 0, 40, h))
	out := scr.String()
	require.Contains(t, out, "backups to s3", "answer cut off:\n%s", out)
	require.Contains(t, out, "Yup!", "buttons pushed out of view:\n%s", out)
	require.NotContains(t, out, "│", "spurious scrollbar:\n%s", out)
	// Continuation lines hang under the text, not the bullet.
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "backups") && !strings.Contains(l, "•") {
			require.True(t, strings.HasPrefix(l, "  "), "no hanging indent: %q", l)
		}
	}
}
