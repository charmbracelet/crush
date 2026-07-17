package attachments

import (
	"strings"
	"testing"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func newTestRenderer() *Renderer {
	sty := styles.CharmtonePantera()
	return NewRenderer(
		sty.Attachments.Normal,
		sty.Attachments.Deleting,
		sty.Attachments.Image,
		sty.Attachments.Text,
		sty.Attachments.Skill,
		sty.Attachments.Remove,
	)
}

func TestRender_IncludesRemoveButton(t *testing.T) {
	t.Parallel()

	r := newTestRenderer()
	atts := []message.Attachment{
		{FileName: "test.txt"},
	}
	out := r.Render(atts, false, true, 80)
	require.Contains(t, out, styles.RemoveIcon)
}

func TestRender_DeletingModeNoRemoveButton(t *testing.T) {
	t.Parallel()

	r := newTestRenderer()
	atts := []message.Attachment{
		{FileName: "test.txt"},
	}
	out := r.Render(atts, true, true, 80)
	require.NotContains(t, out, styles.RemoveIcon)
}

func TestRender_ShowRemoveFalseOmitsRemoveButton(t *testing.T) {
	t.Parallel()

	r := newTestRenderer()
	atts := []message.Attachment{
		{FileName: "no-change.png"},
	}
	out := r.Render(atts, false, false, 80)
	require.NotContains(t, out, styles.RemoveIcon,
		"posted-message attachments must not show a remove button")
	require.Empty(t, r.bounds,
		"no remove bounds should be recorded when the button is hidden")
	require.Equal(t, -1, r.HitTestRemove(atts, 0))
}

func TestRender_ShowRemoveFalseKeepsGapBetweenChips(t *testing.T) {
	t.Parallel()

	// Regression for the #134 + #135 interaction: #134 moved the trailing
	// margin onto the remove button, and #135 hides that button on posted
	// messages. Together, posted messages with multiple attachments lost the
	// margin that separated adjacent chips, so their backgrounds touched. The
	// filename must carry the margin when the remove button is hidden.
	//
	// White-box width check: the visible width of the two chips without any
	// separator is icon+filename per chip. With the fix each posted chip adds
	// a 1-column trailing margin, so the rendered row is exactly two columns
	// wider. Stripping ANSI can't detect this (a margin space and a
	// background-colored padding space are both just spaces), so we measure
	// width instead.
	r := newTestRenderer()
	atts := []message.Attachment{
		{FileName: "alpha.txt"},
		{FileName: "beta.txt"},
	}
	bare := lipgloss.Width(r.textStyle.String()+r.normalStyle.Render("alpha.txt")) +
		lipgloss.Width(r.textStyle.String()+r.normalStyle.Render("beta.txt"))

	got := lipgloss.Width(r.Render(atts, false, false, 200))
	require.Equal(t, bare+2, got,
		"each posted chip must carry a 1-col trailing margin so adjacent chip backgrounds don't touch")
}

func TestRender_DeletingModeKeepsChipLayout(t *testing.T) {
	t.Parallel()

	// Regression for review feedback on #3338: entering delete-mode used
	// to replace the leading icon with the numeral and drop the remove
	// button, shifting every chip. The numeral must instead take over the
	// remove button's slot, leaving the left side of the chip as-is.
	r := newTestRenderer()
	atts := []message.Attachment{
		{FileName: "main.go"},
		{FileName: "models.go"},
	}
	idle := r.Render(atts, false, true, 200)
	deleting := r.Render(atts, true, true, 200)

	require.Equal(t, lipgloss.Width(idle), lipgloss.Width(deleting),
		"entering delete-mode must not shift the chips")
	require.Contains(t, deleting, styles.TextIcon,
		"delete-mode must keep the chip's icon")
	require.Contains(t, deleting, "0")
	require.Contains(t, deleting, "1")
}

func TestRender_RemoveButtonHasRightPadding(t *testing.T) {
	t.Parallel()

	// Regression for review feedback on #3338: the ✕ must not sit flush
	// against the right edge of its colored box. The cell to the right of the
	// glyph has to be padding — part of the button's background — rather than
	// a transparent margin, so the glyph has breathing room on its right.
	//
	// A plain-width or ANSI-stripped check can't catch this: a margin space
	// and a background-colored padding space are both one blank column. So we
	// inspect the per-cell background and assert the button's background
	// extends one cell past the ✕.
	r := newTestRenderer()
	atts := []message.Attachment{{FileName: "main.go"}}
	out := r.Render(atts, false, true, 200)

	cells := parseCells(out)
	xi := -1
	for i, c := range cells {
		if c.r == styles.RemoveIcon {
			xi = i
			break
		}
	}
	require.GreaterOrEqual(t, xi, 0, "rendered output must contain the ✕ glyph")
	require.NotEmpty(t, cells[xi].bg, "the ✕ cell must have the button's background")
	require.Less(t, xi+1, len(cells),
		"the ✕ must be followed by a trailing padding cell, not be the box's last cell")
	require.Equal(t, cells[xi].bg, cells[xi+1].bg,
		"the cell to the right of ✕ must share the button's background (padding), not be a transparent margin")
}

// cell is one rendered terminal cell: its rune and the truecolor background
// in effect ("r;g;b", or "" for none).
type cell struct {
	r  string
	bg string
}

// parseCells walks a lipgloss-rendered string and returns its visible cells
// with the background color active at each. It understands the SGR sequences
// lipgloss emits (truecolor 48;2;r;g;b backgrounds, 38;2;r;g;b foregrounds,
// and resets); other escapes are ignored.
func parseCells(s string) []cell {
	var cells []cell
	bg := ""
	for i := 0; i < len(s); {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && s[j] != 'm' {
				j++
			}
			if j < len(s) {
				bg = applyBG(s[i+2:j], bg)
				i = j + 1
				continue
			}
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		cells = append(cells, cell{r: s[i : i+size], bg: bg})
		i += size
	}
	return cells
}

// applyBG updates the current background given one SGR parameter string.
func applyBG(params, cur string) string {
	if params == "" || params == "0" {
		return ""
	}
	toks := strings.Split(params, ";")
	for k := 0; k < len(toks); k++ {
		switch toks[k] {
		case "0":
			cur = ""
		case "38": // foreground — skip its arguments
			if k+1 < len(toks) && toks[k+1] == "2" {
				k += 4
			} else if k+1 < len(toks) && toks[k+1] == "5" {
				k += 2
			}
		case "48": // background
			if k+4 < len(toks) && toks[k+1] == "2" {
				cur = toks[k+2] + ";" + toks[k+3] + ";" + toks[k+4]
				k += 4
			} else if k+2 < len(toks) && toks[k+1] == "5" {
				cur = toks[k+2]
				k += 2
			}
		}
	}
	return cur
}

func TestRender_MultipleChipsEachHaveRemoveButton(t *testing.T) {
	t.Parallel()

	r := newTestRenderer()
	atts := []message.Attachment{
		{FileName: "a.txt"},
		{FileName: "b.txt"},
		{FileName: "c.txt"},
	}
	out := r.Render(atts, false, true, 120)
	// Count occurrences of the remove glyph.
	count := 0
	for _, c := range out {
		if string(c) == styles.RemoveIcon {
			count++
		}
	}
	require.Equal(t, 3, count, "each chip should have a remove button")
}

func TestHitTestRemove_ClickOnFirstChipRemove(t *testing.T) {
	t.Parallel()

	r := newTestRenderer()
	atts := []message.Attachment{
		{FileName: "first.txt"},
		{FileName: "second.txt"},
	}
	_ = r.Render(atts, false, true, 120)

	// The remove button of the first chip should be hit-testable.
	// Click at various X positions to verify we hit the right chip.
	idx := r.HitTestRemove(atts, 0)
	// At x=0 we're on the icon, not the remove button.
	require.Equal(t, -1, idx)
}

func TestHitTestRemove_ReturnsCorrectIndex(t *testing.T) {
	t.Parallel()

	r := newTestRenderer()
	atts := []message.Attachment{
		{FileName: "first.txt"},
		{FileName: "second.txt"},
	}
	_ = r.Render(atts, false, true, 120)

	// Each chip bounds are stored after render. Verify there are two.
	require.Len(t, r.bounds, 2)

	// Click on the first chip's remove button.
	b0 := r.bounds[0]
	idx := r.HitTestRemove(atts, b0.startX)
	require.Equal(t, 0, idx)

	// Click on the second chip's remove button.
	b1 := r.bounds[1]
	idx = r.HitTestRemove(atts, b1.startX)
	require.Equal(t, 1, idx)
}

func TestHitTestRemove_TrailingMarginNotClickable(t *testing.T) {
	t.Parallel()

	r := newTestRenderer()
	atts := []message.Attachment{
		{FileName: "first.txt"},
		{FileName: "second.txt"},
	}
	_ = r.Render(atts, false, true, 120)

	// The cell just past a button's hit region belongs to the next chip, not
	// to this button — a click there must not remove this attachment.
	b0 := r.bounds[0]
	require.Equal(t, -1, r.HitTestRemove(atts, b0.removeEnd))
}

func TestHitTestRemove_OutsideAnyRemoveReturnsMinusOne(t *testing.T) {
	t.Parallel()

	r := newTestRenderer()
	atts := []message.Attachment{
		{FileName: "test.txt"},
	}
	_ = r.Render(atts, false, true, 80)

	// Click far past the remove button.
	idx := r.HitTestRemove(atts, 999)
	require.Equal(t, -1, idx)
}

func TestHandleClick_RemovesAttachment(t *testing.T) {
	t.Parallel()

	r := newTestRenderer()
	km := Keymap{}
	m := New(r, km)
	m.list = []message.Attachment{
		{FileName: "first.txt"},
		{FileName: "second.txt"},
	}

	// Render so bounds are populated.
	_ = m.Render(120)

	// Click the first chip's remove button.
	b0 := r.bounds[0]
	handled := m.HandleClick(b0.startX)
	require.True(t, handled)
	require.Len(t, m.list, 1)
	require.Equal(t, "second.txt", m.list[0].FileName)
}

func TestHandleClick_ClickOutsideRemoveDoesNothing(t *testing.T) {
	t.Parallel()

	r := newTestRenderer()
	km := Keymap{}
	m := New(r, km)
	m.list = []message.Attachment{
		{FileName: "test.txt"},
	}

	_ = m.Render(80)

	// Click at x=0 (the icon area, not the remove button).
	handled := m.HandleClick(0)
	require.False(t, handled)
	require.Len(t, m.list, 1)
}

func TestHandleClick_DeletingModeIgnored(t *testing.T) {
	t.Parallel()

	r := newTestRenderer()
	km := Keymap{}
	m := New(r, km)
	m.list = []message.Attachment{
		{FileName: "test.txt"},
	}
	m.deleting = true

	_ = m.Render(80)

	// bounds are empty in deleting mode since remove buttons aren't rendered.
	require.Empty(t, r.bounds)
	// Click anywhere — should be ignored.
	handled := m.HandleClick(10)
	require.False(t, handled)
}

func TestHandleClick_EmptyListIgnored(t *testing.T) {
	t.Parallel()

	r := newTestRenderer()
	km := Keymap{}
	m := New(r, km)

	handled := m.HandleClick(5)
	require.False(t, handled)
}
