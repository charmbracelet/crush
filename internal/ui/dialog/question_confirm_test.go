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

// TestConfirmScrollHintOnlyWhenScrollable verifies the scroll hint is
// shown only when the summary overflows its area.
func TestConfirmScrollHintOnlyWhenScrollable(t *testing.T) {
	t.Parallel()
	s := styles.CharmtonePantera()
	answers := []*question.Answer{{QuestionID: "q1", FillInText: "a"}, {QuestionID: "q2", FillInText: "b"}}
	c := NewConfirmComponent(&s, "", "", []string{"One", "Two"},
		[]question.Question{{ID: "q1"}, {ID: "q2"}}, answers)
	hasScroll := func() bool {
		for _, b := range c.ShortHelp() {
			if b.Help().Desc == "scroll" {
				return true
			}
		}
		return false
	}
	h := c.Height(60)
	c.Draw(uv.NewScreenBuffer(60, h), image.Rect(0, 0, 60, h))
	require.False(t, hasScroll(), "nothing scrolls at full height")
	c.Draw(uv.NewScreenBuffer(60, 3), image.Rect(0, 0, 60, 3))
	require.True(t, hasScroll(), "scroll hint missing when content overflows")
}
