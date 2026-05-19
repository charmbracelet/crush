package model

import (
	"strconv"
	"testing"

	"github.com/charmbracelet/crush/internal/ui/chat"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// drawTestArea is a fixed area used by the cache tests; it must be at least
// as large as the test items so we can compare every cell.
func drawTestArea(width, height int) uv.Rectangle {
	return uv.Rect(0, 0, width, height)
}

// renderToBuffer mirrors what Chat.Draw does at the screen layer so tests
// can assert byte-equivalence with a fresh uv.NewStyledString render.
func renderToBuffer(t *testing.T, c *Chat, w, h int) string {
	t.Helper()
	scr := uv.NewScreenBuffer(w, h)
	c.Draw(scr, drawTestArea(w, h))
	return scr.Render()
}

// TestChatDrawCache_HitOnIdenticalRender asserts that two consecutive Draws
// against the same list output reuse the same chatDrawCache rather than
// allocating a fresh one. Both the wrapper pointer and the embedded
// RenderBuffer pointer must be stable across the second Draw — that's what
// proves no re-decoding happened.
func TestChatDrawCache_HitOnIdenticalRender(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.chat.SetMessages(
		testMessageItem{id: "a", text: "alpha"},
		testMessageItem{id: "b", text: "beta"},
	)
	u.updateLayoutAndSize()

	w, h := 80, 20
	_ = renderToBuffer(t, u.chat, w, h)
	require.NotNil(t, u.chat.drawCache, "first draw should populate cache")
	firstCache := u.chat.drawCache
	firstBuf := u.chat.drawCache.buf.RenderBuffer

	_ = renderToBuffer(t, u.chat, w, h)
	require.Same(t, firstCache, u.chat.drawCache,
		"identical rendered string must reuse the same cache pointer")
	require.Same(t, firstBuf, u.chat.drawCache.buf.RenderBuffer,
		"identical rendered string must reuse the same RenderBuffer pointer")
}

// TestChatDrawCache_MissOnDifferentRender asserts that when the list output
// changes, the cache is rebuilt and the new draw output matches a fresh
// uv.NewStyledString render byte-for-byte.
func TestChatDrawCache_MissOnDifferentRender(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.chat.SetMessages(
		testMessageItem{id: "a", text: "alpha"},
	)
	u.updateLayoutAndSize()

	w, h := 80, 20
	_ = renderToBuffer(t, u.chat, w, h)
	require.NotNil(t, u.chat.drawCache)
	firstCache := u.chat.drawCache

	// Replace the items so the rendered string differs.
	u.chat.SetMessages(
		testMessageItem{id: "c", text: "gamma delta"},
	)
	u.updateLayoutAndSize()

	got := renderToBuffer(t, u.chat, w, h)
	require.NotSame(t, firstCache, u.chat.drawCache,
		"changed rendered string must replace the cache entry")

	// Output must match a fresh uv.NewStyledString render of the current
	// list output for the same area. This is the byte-equivalence guard
	// that protects against blit drift from StyledString.Draw.
	want := freshStyledRender(u.chat.list.Render(), w, h)
	require.Equal(t, want, got)
}

// TestChatDrawCache_ReusedAcrossDifferentArea asserts that the cached
// decoded lines do not depend on the screen / area: the same string drawn
// into a smaller area must hit the cache, and the output must match a fresh
// uv.NewStyledString render against the new area.
func TestChatDrawCache_ReusedAcrossDifferentArea(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.chat.SetMessages(
		testMessageItem{id: "a", text: "alpha"},
		testMessageItem{id: "b", text: "beta"},
	)
	u.updateLayoutAndSize()

	w, h := 80, 20
	_ = renderToBuffer(t, u.chat, w, h)
	require.NotNil(t, u.chat.drawCache)
	firstCache := u.chat.drawCache
	firstBuf := u.chat.drawCache.buf.RenderBuffer

	// Draw the same content into a smaller buffer. The list output is
	// width-sensitive in production (list.Render depends on width), so we
	// hold the list width fixed via updateLayoutAndSize and only shrink
	// the destination area. The cache key is the rendered string + width
	// method, both unchanged here.
	smallScr := uv.NewScreenBuffer(40, 10)
	u.chat.Draw(smallScr, uv.Rect(0, 0, 40, 10))

	require.Same(t, firstCache, u.chat.drawCache,
		"smaller area must still hit the cache")
	require.Same(t, firstBuf, u.chat.drawCache.buf.RenderBuffer,
		"cached RenderBuffer must be reused across area changes")

	want := freshStyledRender(u.chat.list.Render(), 40, 10)
	require.Equal(t, want, smallScr.Render())
}

// TestChatDrawCache_BoundedSize asserts the draw cache holds at most one
// entry. This is structural — any future change that introduces an LRU or
// ring would have to update this test.
func TestChatDrawCache_BoundedSize(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	w, h := 80, 20

	// Cycle through several distinct list outputs and confirm that
	// drawCache is still a single *chatDrawCache pointing at the most
	// recent rendered string each time.
	for i := range 5 {
		u.chat.SetMessages(
			testMessageItem{id: "x", text: "tick " + strconv.Itoa(i)},
		)
		u.updateLayoutAndSize()
		_ = renderToBuffer(t, u.chat, w, h)
		require.NotNil(t, u.chat.drawCache)
		require.Equal(t, u.chat.list.Render(), u.chat.drawCache.rendered,
			"cache.rendered must always match the most recent list output")
	}

	// Sanity: the type of drawCache is a single pointer, not a slice/map.
	// This is enforced at compile time but checking it explicitly here
	// keeps the bounded-size invariant visible to future readers.
	require.IsType(t, (*chatDrawCache)(nil), u.chat.drawCache)
}

// freshStyledRender renders the same string through uv.NewStyledString into
// a fresh ScreenBuffer of the given size. It's the reference implementation
// used by the cache tests to detect blit drift.
func freshStyledRender(s string, w, h int) string {
	scr := uv.NewScreenBuffer(w, h)
	uv.NewStyledString(s).Draw(scr, uv.Rect(0, 0, w, h))
	return scr.Render()
}

// TestChatDrawCache_InvalidatedByWidthMethodSwap asserts the cache rebuilds
// when the destination screen's width method changes between frames.
// GraphemeWidth and WcWidth disagree on emoji ZWJ sequences and other
// modern unicode, so the decoded buffer is only valid for the method it
// was decoded with. Reusing across methods would corrupt cell widths.
func TestChatDrawCache_InvalidatedByWidthMethodSwap(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	// Use content where the two methods disagree so any accidental
	// reuse would surface as a different rendered cell layout. The
	// woman-technologist ZWJ sequence is one cell under GraphemeWidth
	// but two under WcWidth (the components combine).
	u.chat.SetMessages(
		testMessageItem{id: "a", text: "hi 👩\u200d💻 there"},
	)
	u.updateLayoutAndSize()

	w, h := 80, 5

	// First draw under GraphemeWidth.
	scrA := uv.ScreenBuffer{
		RenderBuffer: uv.NewRenderBuffer(w, h),
		Method:       ansi.GraphemeWidth,
	}
	u.chat.Draw(scrA, uv.Rect(0, 0, w, h))
	require.NotNil(t, u.chat.drawCache)
	require.Equal(t, ansi.GraphemeWidth, u.chat.drawCache.method)
	firstCache := u.chat.drawCache

	// Same string, swap the method. The cache key includes method, so
	// this must rebuild even though `rendered` is byte-identical.
	scrB := uv.ScreenBuffer{
		RenderBuffer: uv.NewRenderBuffer(w, h),
		Method:       ansi.WcWidth,
	}
	u.chat.Draw(scrB, uv.Rect(0, 0, w, h))
	require.NotSame(t, firstCache, u.chat.drawCache,
		"width method change must invalidate the cache")
	require.Equal(t, ansi.WcWidth, u.chat.drawCache.method)
}

// fixedMethodScreen wraps a uv.ScreenBuffer but reports a custom (non
// ansi.Method) WidthMethod implementation. Chat.Draw is expected to
// detect the type-assertion miss and fall through to the uncached path.
type fixedMethodScreen struct {
	uv.ScreenBuffer
	method uv.WidthMethod
}

func (s fixedMethodScreen) WidthMethod() uv.WidthMethod { return s.method }

// customWidth is a WidthMethod whose concrete type is intentionally NOT
// ansi.Method, so the type assertion in Chat.Draw fails. The actual
// width math just delegates to ansi.GraphemeWidth so the rendered
// output is well-defined and comparable.
type customWidth struct{}

func (customWidth) StringWidth(s string) int {
	return ansi.GraphemeWidth.StringWidth(s)
}

// TestChatDrawCache_FallbackOnNonAnsiMethod asserts that when the screen
// reports a WidthMethod whose concrete type is not ansi.Method, Chat.Draw
// skips the cache (it has no comparable key for an arbitrary interface)
// and falls through to a direct uv.NewStyledString.Draw, producing the
// same output as the upstream uncached path.
func TestChatDrawCache_FallbackOnNonAnsiMethod(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.chat.SetMessages(
		testMessageItem{id: "a", text: "alpha"},
		testMessageItem{id: "b", text: "beta"},
	)
	u.updateLayoutAndSize()

	w, h := 40, 10
	scr := fixedMethodScreen{
		ScreenBuffer: uv.ScreenBuffer{
			RenderBuffer: uv.NewRenderBuffer(w, h),
			Method:       ansi.GraphemeWidth,
		},
		method: customWidth{},
	}
	// Sanity: the wrapper actually returns a non-ansi.Method type so
	// the fallback path is exercised end-to-end.
	_, isAnsiMethod := scr.WidthMethod().(ansi.Method)
	require.False(t, isAnsiMethod,
		"test setup must hand Draw a non-ansi.Method WidthMethod")

	u.chat.Draw(scr, uv.Rect(0, 0, w, h))

	// The fallback path uses uv.NewStyledString(rendered).Draw — same
	// thing freshStyledRender does — so output must match the
	// reference render byte-for-byte.
	want := freshStyledRender(u.chat.list.Render(), w, h)
	require.Equal(t, want, scr.Render())

	// And no cache was populated (or if it was, it wasn't used for
	// this draw). Either way: drawing through the fallback must never
	// leave a stale cache that future draws would reuse incorrectly.
	require.Nil(t, u.chat.drawCache,
		"fallback path must not populate the cache")
}

// TestRenderedBounds_MatchesPrintStringTallyForZWJ pins the invariant
// that renderedBounds (used to size the cache buffer) returns the same
// width as the cell tally StyledString.Draw will write into a buffer
// using the same WidthMethod. We compare the computed width to a
// reference render: lay the string into an oversized buffer, then count
// non-empty cells per row. A divergent-width sample (woman-technologist
// ZWJ sequence) is the canonical case that would expose a Bounds()
// vs printString mismatch — under GraphemeWidth the sequence is one
// cell, under WcWidth it's two — so the same string must produce two
// different widths under the two methods, both matching the tally.
func TestRenderedBounds_MatchesPrintStringTallyForZWJ(t *testing.T) {
	t.Parallel()

	const sample = "x 👩\u200d💻 y"

	for _, m := range []ansi.Method{ansi.GraphemeWidth, ansi.WcWidth} {
		w, h := renderedBounds(sample, m)
		// Tally what printString would actually write.
		// Use an oversized buffer so nothing gets clipped.
		scr := uv.ScreenBuffer{
			RenderBuffer: uv.NewRenderBuffer(64, 4),
			Method:       m,
		}
		uv.NewStyledString(sample).Draw(scr, uv.Rect(0, 0, 64, 4))

		// Tally width: rightmost non-empty cell + its width, per row.
		gotW := 0
		gotH := 0
		for y := 0; y < 4; y++ {
			rowW := 0
			rowHasContent := false
			for x := 0; x < 64; x++ {
				cell := scr.CellAt(x, y)
				if cell == nil || cell.IsZero() ||
					cell.Content == " " && cell.Width == 1 &&
						cell.Style == (uv.Style{}) {
					continue
				}
				rowHasContent = true
				if x+cell.Width > rowW {
					rowW = x + cell.Width
				}
			}
			if rowHasContent {
				gotH = y + 1
				if rowW > gotW {
					gotW = rowW
				}
			}
		}

		require.Equal(t, gotW, w,
			"renderedBounds width must match printString cell tally for method %v", m)
		// Height is just '\n' count + 1 — no special-casing needed
		// here, but pin it so future regressions on multi-line
		// inputs don't sneak by.
		require.Equal(t, gotH, h,
			"renderedBounds height must match printString row tally for method %v", m)
	}
}

// Compile-time guard: testMessageItem is reused from layout_test.go and
// satisfies chat.MessageItem there. We rely on the same shape here.
var _ chat.MessageItem = testMessageItem{}
