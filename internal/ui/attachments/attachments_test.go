package attachments

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func newTestRenderer() *Renderer {
	sty := styles.CharmtonePantera()
	return NewRenderer(sty.Attachments)
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
	bare := lipgloss.Width(r.styles.Text.String()+r.styles.Normal.Render("alpha.txt")) +
		lipgloss.Width(r.styles.Text.String()+r.styles.Normal.Render("beta.txt"))

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

func TestRender_RemoveButtonKeepsGapBetweenChips(t *testing.T) {
	t.Parallel()

	r := newTestRenderer()
	atts := []message.Attachment{
		{FileName: "first.txt"},
		{FileName: "second.txt"},
	}
	cells := parseCells(r.Render(atts, false, true, 200))

	xi := -1
	for i, c := range cells {
		if c.r == styles.RemoveIcon {
			xi = i
			break
		}
	}
	require.GreaterOrEqual(t, xi, 0)
	require.Less(t, xi+2, len(cells))
	require.Empty(t, cells[xi+2].bg, "adjacent attachment chips must have a transparent one-cell gap")
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

// overflowFixture is a set of attachments wide enough to overflow any
// reasonable editor, so tests can exercise the hint path.
func overflowFixture() []message.Attachment {
	atts := make([]message.Attachment, 8)
	for i := range atts {
		atts[i] = message.Attachment{FileName: fmt.Sprintf("file-%d.txt", i)}
	}
	return atts
}

// chipWidth is the rendered width of a lone editor chip for the given
// filename. Below this a row cannot fit even one chip, so it is the
// natural floor for any "the row fits" assertion.
func chipWidth(r *Renderer, name string) int {
	return lipgloss.Width(r.Render([]message.Attachment{{FileName: name}}, false, true, 1000))
}

// hintCount returns the number the "N more…" hint claims, or 0 when the
// row carries no hint. The trailing trim matters: a hint padded out to a
// fixed width would otherwise read as no hint at all.
func hintCount(out string) int {
	plain := strings.TrimRight(ansi.Strip(out), " ")
	if !strings.HasSuffix(plain, "more…") {
		return 0
	}
	fields := strings.Fields(plain)
	n, err := strconv.Atoi(fields[len(fields)-2])
	if err != nil {
		return 0
	}
	return n
}

func TestRender_SingleVisibleChipHasNoMoreHint(t *testing.T) {
	t.Parallel()

	// Regression: the row used to budget every chip at the maximum filename
	// width, so on a narrow editor a single short-named attachment was
	// counted as overflowing its own row. The chip rendered fine and then
	// "1 more…" was tacked on beside it, promising a file that was already
	// on screen. Widths are now measured as chips are laid out.
	r := newTestRenderer()
	atts := []message.Attachment{{FileName: "paste_1.png"}}
	for width := range 121 {
		require.Zero(t, hintCount(r.Render(atts, false, true, width)),
			"a lone attachment must never claim there are others (width %d)", width)
	}
}

func TestRender_HintOnlyAppearsWhenSomethingIsHidden(t *testing.T) {
	t.Parallel()

	// The stronger form of the regression above: for any number of
	// attachments at any width, a hint may appear only when chips were
	// actually left out, and it must name exactly how many. A single-
	// attachment test cannot catch a hint that is off by one for n >= 2.
	r := newTestRenderer()
	all := overflowFixture()
	for n := 1; n <= len(all); n++ {
		for width := range 161 {
			out := r.Render(all[:n], false, true, width)
			drawn := strings.Count(out, styles.TextIcon)
			require.Positive(t, drawn, "some chip must always be drawn (n=%d width=%d)", n, width)
			if got := hintCount(out); got != 0 {
				require.Equal(t, n-drawn, got,
					"hint must name exactly the chips left out (n=%d width=%d drew=%d)", n, width, drawn)
			} else {
				require.True(t, drawn == n || drawn == 1,
					"a row may go without the hint only when everything fit, or when "+
						"a single chip crowded the hint out (n=%d width=%d drew=%d)", n, width, drawn)
			}
		}
	}
}

func TestRender_DrawnChipsAreTheLeadingRun(t *testing.T) {
	t.Parallel()

	// Overflow drops chips off the end, never the middle. Click handling
	// indexes straight into the attachment slice with the chip's position,
	// so a gap would remove the wrong file.
	r := newTestRenderer()
	atts := overflowFixture()
	for width := range 161 {
		out := ansi.Strip(r.Render(atts, false, true, width))
		drawn := strings.Count(out, styles.TextIcon)
		for i := range drawn {
			require.Contains(t, out, fmt.Sprintf("file-%d.txt", i),
				"chip %d must be present when %d chips were drawn (width %d)", i, drawn, width)
		}
		for i := drawn; i < len(atts); i++ {
			require.NotContains(t, out, fmt.Sprintf("file-%d.txt", i),
				"chip %d must be absent when only %d chips were drawn (width %d)", i, drawn, width)
		}
	}
}

func TestRender_MoreHintUsesTheThemeStyle(t *testing.T) {
	t.Parallel()

	// The hint used to be rendered with a bare lipgloss.Style, so it came
	// out in the terminal's default foreground next to the themed chips.
	// The expectation is built from the theme rather than from the
	// renderer, so this cannot pass by agreeing with itself.
	r := newTestRenderer()
	atts := overflowFixture()
	out := r.Render(atts, false, true, 60)

	n := hintCount(out)
	require.Positive(t, n, "width 60 must overflow for this to mean anything")
	want := styles.CharmtonePantera().Attachments.More.Render(fmt.Sprintf("%d more…", n))
	require.Contains(t, want, "\x1b[", "the theme's hint style must actually set a color")
	require.Contains(t, out, want, "the row must render the hint through the theme style")
}

func TestRender_RowNeverSpillsOnceOneChipFits(t *testing.T) {
	t.Parallel()

	// Room is reserved for the hint before a chip is committed, so a row
	// wide enough for a single chip never runs past the width it was
	// given. Narrower than that, the first chip is drawn anyway and the
	// row is exactly that one chip wide.
	r := newTestRenderer()
	atts := overflowFixture()
	floor := chipWidth(r, "file-0.txt")
	for width := range 161 {
		got := lipgloss.Width(r.Render(atts, false, true, width))
		if width < floor {
			require.Equal(t, floor, got,
				"too narrow for a chip, so the row is exactly one chip (width %d)", width)
			continue
		}
		require.LessOrEqual(t, got, width,
			"the attachment row must not spill past the editor (width %d)", width)
	}
}

func TestRender_OverflowHoldsInEveryMode(t *testing.T) {
	t.Parallel()

	// Delete-mode and posted messages share this layout code, so the hint
	// and the width budget have to hold for them too.
	for _, tc := range []struct {
		name                 string
		deleting, showRemove bool
	}{
		{"editor", false, true},
		{"delete mode", true, true},
		{"posted message", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Render mutates the renderer's hit-test bounds, so each
			// parallel subtest needs its own.
			r := newTestRenderer()
			atts := overflowFixture()
			out := r.Render(atts, tc.deleting, tc.showRemove, 60)
			drawn := strings.Count(out, styles.TextIcon)
			require.Less(t, drawn, len(atts), "width 60 must overflow for this to mean anything")
			require.Equal(t, len(atts)-drawn, hintCount(out))
			require.LessOrEqual(t, lipgloss.Width(out), 60)
		})
	}
}

func TestHitTestRemove_OverflowBoundsOnlyCoverDrawnChips(t *testing.T) {
	t.Parallel()

	// Callers index straight into the attachment slice with whatever
	// HitTestRemove returns, so a bound recorded for a chip that was never
	// drawn would delete the wrong file.
	r := newTestRenderer()
	atts := overflowFixture()
	out := r.Render(atts, false, true, 60)
	drawn := strings.Count(out, styles.TextIcon)
	require.Less(t, drawn, len(atts), "width 60 must overflow for this to mean anything")
	require.Len(t, r.bounds, drawn, "only drawn chips may carry a clickable remove button")

	for x := range lipgloss.Width(out) {
		require.Less(t, r.HitTestRemove(atts, x), drawn,
			"column %d hit-tests to a chip that was never drawn", x)
	}
}

func TestRender_NonPositiveWidthDrawsOneChip(t *testing.T) {
	t.Parallel()

	// Layout can hand down a zero or negative width mid-resize. That must
	// not panic or silently render an empty row. Nothing fits beside the
	// first chip at these widths, so the hint gives way to the filename.
	r := newTestRenderer()
	atts := []message.Attachment{{FileName: "a.txt"}, {FileName: "b.txt"}}
	for _, width := range []int{-5, 0, 1} {
		out := r.Render(atts, false, true, width)
		require.Equal(t, 1, strings.Count(out, styles.TextIcon),
			"width %d should still show the first chip", width)
		require.Zero(t, hintCount(out),
			"width %d has no room for a hint beside the chip", width)
	}
}

func TestRender_NoAttachmentsRendersNothing(t *testing.T) {
	t.Parallel()

	r := newTestRenderer()
	require.Empty(t, r.Render(nil, false, true, 80))
	require.Empty(t, r.bounds)
}
