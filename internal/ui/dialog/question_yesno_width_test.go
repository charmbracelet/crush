package dialog

import (
	"image"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// TestYesNoDrawPreservesSuffixUnderWcWidth verifies question text wrapped
// under GraphemeWidth but drawn on a WcWidth screen still renders every
// printable suffix the layout admitted. The canonical case is halfwidth
// katakana with a voicing mark followed by ASCII (ｶﾞX) in a narrow area.
func TestYesNoDrawPreservesSuffixUnderWcWidth(t *testing.T) {
	t.Parallel()

	const text = "ｶﾞX"
	const width = 6

	s := styles.CharmtonePantera()
	d := NewYesNo(&s, question.Question{
		ID:   "q1",
		Type: question.TypeYesNo,
		Text: text,
	})

	iconPrompt := questionIconPrompt(&s, false)
	contentWidth := width - lipgloss.Width(iconPrompt)
	require.Greater(t, contentWidth, 0)

	wrapped := wrapAt(text, contentWidth, "", ansi.GraphemeWidth)
	require.Contains(t, wrapped, "X", "grapheme layout should retain the suffix")

	scr := uv.ScreenBuffer{
		RenderBuffer: uv.NewRenderBuffer(width, d.Height(width)),
		Method:       ansi.WcWidth,
	}
	d.Draw(scr, image.Rect(0, 0, width, scr.Bounds().Dy()))

	rendered := strings.TrimSpace(scr.Render())
	require.Contains(t, rendered, "X",
		"WcWidth draw must not drop a suffix the layout wrapped in; got %q", rendered)

	heightAfterDraw := d.Height(width)
	require.GreaterOrEqual(t, heightAfterDraw, 5,
		"Height after WcWidth draw should account for wrapped question lines")
}

// TestSingleChoiceDrawPreservesSuffixUnderWcWidth mirrors the YesNo case for
// choice dialogs, which share the choiceList layout path.
func TestSingleChoiceDrawPreservesSuffixUnderWcWidth(t *testing.T) {
	t.Parallel()

	const text = "ｶﾞX"
	const width = 12 // choice dialogs reserve horizontal padding vs YesNo

	s := styles.CharmtonePantera()
	d := NewSingleChoice(&s, question.Question{
		ID:   "q1",
		Type: question.TypeSingleChoice,
		Text: text,
		Choices: []question.Choice{
			{ID: "a", Label: "Yes"},
			{ID: "b", Label: "No"},
		},
	})

	scr := uv.ScreenBuffer{
		RenderBuffer: uv.NewRenderBuffer(width, 20),
		Method:       ansi.WcWidth,
	}
	d.Draw(scr, image.Rect(0, 0, width, 20))

	rendered := strings.TrimSpace(scr.Render())
	require.Contains(t, rendered, "X",
		"choice dialog WcWidth draw must not drop wrapped suffix; got %q", rendered)
}
