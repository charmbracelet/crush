package dialog

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func newTestBtw(t *testing.T) *Btw {
	t.Helper()
	s := styles.CharmtonePantera()
	com := &common.Common{Styles: &s}
	return NewBtw(com, "session-test")
}

// typeBtw feeds text through the real key path so the input's internal
// position and horizontal scroll match what a user would produce.
func typeBtw(d *Btw, text string) {
	for _, r := range text {
		d.HandleMsg(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

func drawBtw(d *Btw, w, h int) (uv.ScreenBuffer, *tea.Cursor) {
	scr := uv.NewScreenBuffer(w, h)
	cur := d.Draw(scr, uv.Rect(0, 0, w, h))
	return scr, cur
}

// cellsAt reads n cells starting at (x, y) as a plain string.
func cellsAt(scr uv.ScreenBuffer, x, y, n int) string {
	var b strings.Builder
	for i := range n {
		c := scr.CellAt(x+i, y)
		if c == nil || c.Content == "" {
			b.WriteString(" ")
			continue
		}
		b.WriteString(c.Content)
	}
	return b.String()
}

// TestBtw_CursorSitsAfterTypedText asserts the real terminal cursor lands on
// the cell immediately after the last typed character, at every screen width
// including ones narrower than btwMinWidth. Sizing the input without
// accounting for the "> " prompt used to push the reported cursor past the
// text it belongs to.
func TestBtw_CursorSitsAfterTypedText(t *testing.T) {
	t.Parallel()

	const typed = "hello"
	for _, screenW := range []int{20, 30, 46, 80, 120, 200} {
		d := newTestBtw(t)
		typeBtw(d, typed)

		scr, cur := drawBtw(d, screenW, 24)
		require.NotNilf(t, cur, "width %d: input state must report a cursor", screenW)

		require.GreaterOrEqualf(t, cur.X, len(typed),
			"width %d: cursor at X=%d cannot sit after %q", screenW, cur.X, typed)
		require.Lessf(t, cur.X, screenW,
			"width %d: cursor at X=%d is off screen", screenW, cur.X)
		require.Lessf(t, cur.Y, 24, "width %d: cursor at Y=%d is off screen", screenW, cur.Y)

		got := cellsAt(scr, cur.X-len(typed), cur.Y, len(typed))
		require.Equalf(t, typed, got,
			"width %d: expected the %d cells before the cursor to spell %q, got %q\n%s",
			screenW, len(typed), typed, got, ansi.Strip(scr.Render()))

		require.Equalf(t, " ", cellsAt(scr, cur.X, cur.Y, 1),
			"width %d: cursor should rest on a blank cell, not on text or a border", screenW)
	}
}

// TestBtw_LongInputKeepsCursorInsideDialog asserts that text long enough to
// scroll the input horizontally still leaves the cursor inside the dialog
// rather than on (or past) its right border.
func TestBtw_LongInputKeepsCursorInsideDialog(t *testing.T) {
	t.Parallel()

	for _, screenW := range []int{30, 60, 120} {
		d := newTestBtw(t)
		// Draw once first: the dialog is on screen before the user types,
		// and that first Draw is what sizes the input.
		drawBtw(d, screenW, 24)
		typeBtw(d, strings.Repeat("abcdefghij", 30))

		scr, cur := drawBtw(d, screenW, 24)
		require.NotNilf(t, cur, "width %d: input state must report a cursor", screenW)
		require.Lessf(t, cur.X, screenW, "width %d: cursor at X=%d is off screen", screenW, cur.X)

		// The cursor trails the scrolled text, so the cell under it must
		// still be blank and the one before it must hold a character.
		require.Equalf(t, " ", cellsAt(scr, cur.X, cur.Y, 1),
			"width %d: cursor landed on %q instead of a blank cell\n%s",
			screenW, cellsAt(scr, cur.X, cur.Y, 1), ansi.Strip(scr.Render()))
		require.NotEqualf(t, " ", cellsAt(scr, cur.X-1, cur.Y, 1),
			"width %d: cursor is not adjacent to the input text", screenW)
	}
}

// TestBtw_DialogWidthStableAcrossStates asserts the dialog box does not
// change width between the input and answer states. An oversized input line
// used to widen the box, so the centered dialog shifted under the user on
// every state transition.
func TestBtw_DialogWidthStableAcrossStates(t *testing.T) {
	t.Parallel()

	const screenW, screenH = 100, 24

	measure := func(scr uv.ScreenBuffer) (left, right int) {
		left, right = -1, -1
		for y := range screenH {
			for x := range screenW {
				c := scr.CellAt(x, y)
				if c == nil || strings.TrimSpace(c.Content) == "" {
					continue
				}
				if left == -1 || x < left {
					left = x
				}
				if x > right {
					right = x
				}
			}
		}
		return left, right
	}

	d := newTestBtw(t)
	drawBtw(d, screenW, screenH)
	typeBtw(d, strings.Repeat("q", 200))
	inputScr, _ := drawBtw(d, screenW, screenH)
	inputLeft, inputRight := measure(inputScr)

	// The result is only accepted from the loading state.
	require.NotNil(t, d.submit())
	d.HandleMsg(SideQuestionResultMsg{Answer: "an answer", Question: "q"})
	require.Equal(t, btwStateAnswer, d.state)
	answerScr, _ := drawBtw(d, screenW, screenH)
	answerLeft, answerRight := measure(answerScr)

	require.Equal(t, answerRight-answerLeft, inputRight-inputLeft,
		"dialog width must not change between input and answer states")
	require.Equal(t, answerLeft, inputLeft,
		"dialog must not shift horizontally between input and answer states")
}

// TestBtw_LoadingShowsSingleSpinner asserts only one spinner is on screen
// while a side question is in flight. The body and the help line each used
// to render the same spinner model.
func TestBtw_LoadingShowsSingleSpinner(t *testing.T) {
	t.Parallel()

	d := newTestBtw(t)
	typeBtw(d, "why?")
	require.NotNil(t, d.submit())
	require.Equal(t, btwStateLoading, d.state)

	scr, cur := drawBtw(d, 80, 24)
	require.Nil(t, cur, "loading state must not report a cursor")

	rendered := ansi.Strip(scr.Render())
	total := 0
	for _, frame := range spinner.Dot.Frames {
		total += strings.Count(rendered, frame)
	}
	require.Equalf(t, 1, total,
		"expected exactly one spinner frame on screen, found %d\n%s", total, rendered)
}
