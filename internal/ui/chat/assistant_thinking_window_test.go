package chat

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// thinkingMessageWithLines builds a still-thinking assistant message
// whose reasoning content is `count` short paragraphs separated by
// blank lines. The blank-line separation is what matters: glamour
// renders paragraph blocks one-per-line in the output (with a
// trailing blank line between paragraphs) instead of reflowing the
// entire input into one big wrapped paragraph. That gives us a
// post-glamour line count we can drive past the tail-window
// threshold deterministically. Each paragraph is tagged with its
// (1-based) index so the test can identify head vs tail in the
// rendered output.
//
// The message has no text content and no Finish part, so
// IsThinking() returns true and the render path skips the
// "Thought for" footer — keeping the rendered height computation
// simple.
func thinkingMessageWithLines(id string, count int) *message.Message {
	var b strings.Builder
	for i := 1; i <= count; i++ {
		b.WriteString("ln")
		b.WriteString(itoa(i))
		if i < count {
			// Blank line between paragraphs: glamour preserves the
			// per-paragraph structure rather than reflowing into one
			// wrapped block, so totalLines tracks count predictably.
			b.WriteString("\n\n")
		}
	}
	return &message.Message{
		ID:   id,
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.ReasoningContent{
				Thinking:  b.String(),
				StartedAt: testStartedAt,
			},
		},
	}
}

// itoa is a local stdlib-free integer formatter used only by these
// tests; pulling fmt in just for %d would be wasteful when the test
// fixtures already churn 5000+ short strings.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// renderedThinkingHeight returns the line count of the cached
// thinking section render only (not the full RawRender, which also
// includes the content and error sections). Drives a render at
// `width` first to populate the cache.
func renderedThinkingHeight(t *testing.T, item *AssistantMessageItem, width int) int {
	t.Helper()
	_ = item.RawRender(width)
	require.NotEmpty(t, item.thinkingSec.out,
		"thinking section must be populated after RawRender")
	return lipgloss.Height(item.thinkingSec.out)
}

// TestThinkingWindow_CollapsedCapPreserved guards that F5 did not
// regress the existing collapsed-mode behaviour: a 5000-line
// thinking block in the default (collapsed) state still renders at
// most a small bounded height — the last `maxCollapsedThinkingHeight`
// lines plus the truncation hint. The thinking message keeps
// IsThinking() == true, so the optional "Thought for" footer is
// suppressed and the section height equals the box height.
func TestThinkingWindow_CollapsedCapPreserved(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	msg := thinkingMessageWithLines("collapsed", 5000)
	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)

	// Default state must be collapsed.
	require.Equal(t, thinkingCollapsed, item.thinkingViewMode)

	// Unique odd width avoids sharing the glamour renderer cache with
	// any other parallel test (the renderer instance is memoized per
	// width and is not safe for concurrent Render calls).
	const width = 91
	height := renderedThinkingHeight(t, item, width)

	// Collapsed mode keeps the existing cap: last 10 lines + a
	// 2-line hint prefix (hint + blank). Allow a small slack for
	// any future style-driven padding so the test is robust to
	// cosmetic tweaks while still being orders of magnitude below
	// the 5000-line source.
	const collapsedUpperBound = maxCollapsedThinkingHeight + 5
	require.LessOrEqual(t, height, collapsedUpperBound,
		"collapsed mode must remain bounded by the small cap; got %d", height)
}

// TestThinkingWindow_ExpandedShortSkipsTailWindow guards that a
// short thinking block (well under the tail-window cap) still
// toggles directly to full expansion without an intermediate
// tail-window step and shows no affordance footer. The cycle is
// collapsed -> full -> collapsed for short blocks; tail-window is
// only inserted when it would actually elide content.
func TestThinkingWindow_ExpandedShortSkipsTailWindow(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	const lines = 50
	require.Less(t, lines, maxExpandedThinkingTailLines,
		"this test relies on the source being well under the tail cap")
	msg := thinkingMessageWithLines("short", lines)
	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)

	require.True(t, item.ToggleExpanded(),
		"first toggle should report expanded")
	require.Equal(t, thinkingFullExpanded, item.thinkingViewMode,
		"short blocks must skip tail-window and go straight to full expansion")

	const width = 93
	_ = item.RawRender(width)
	out := item.thinkingSec.out
	plain := ansi.Strip(out)

	require.NotContains(t, plain, "earlier lines hidden",
		"short blocks must not show the tail-window affordance")
	require.NotContains(t, plain, "lines hidden",
		"short expanded blocks must not show any truncation hint")
	require.Contains(t, plain, "ln1 ",
		"a fully expanded short block must include the very first source paragraph")
	require.Contains(t, plain, "ln50 ",
		"a fully expanded short block must include the last source paragraph")
}

// TestThinkingWindow_TailWindowed asserts the central F5 behaviour:
// expanding a long thinking block produces a tail window of size
// `maxExpandedThinkingTailLines` plus the affordance footer, with
// the LAST source line present (i.e. we tailed, not headed) and
// earlier lines elided.
//
// Beyond presence/absence of sentinels, this test verifies a true
// `tail -K` relationship between the tail-windowed render and the
// fully-expanded render of the same source at the same width: the
// last K plain-ANSI lines of the windowed render must byte-equal
// the last K lines of the unwindowed render.
//
// K is sized below the cap to absorb the affordance prefix (hint +
// blank line) and any small framing differences introduced by the
// bordered ThinkingBox. The cap minus 5 leaves a comfortable margin
// for padding/footer rows while still asserting that the bulk of
// the rendered tail is identical.
func TestThinkingWindow_TailWindowed(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	const total = 5000
	const width = 95

	// Tail-windowed render.
	tailMsg := thinkingMessageWithLines("tail", total)
	tailItem := NewAssistantMessageItem(&sty, tailMsg).(*AssistantMessageItem)
	require.True(t, tailItem.ToggleExpanded(), "first toggle should report expanded")
	require.Equal(t, thinkingTailWindow, tailItem.thinkingViewMode,
		"a long block must enter tail-window after the first toggle")

	height := renderedThinkingHeight(t, tailItem, width)

	// The visible window is N tail lines plus an affordance line
	// and a blank-line spacer (matching the existing collapsed-mode
	// hint structure). Allow a small slack for style-driven
	// padding.
	const expectedFloor = maxExpandedThinkingTailLines + 1
	const expectedCeil = maxExpandedThinkingTailLines + 5
	require.GreaterOrEqual(t, height, expectedFloor,
		"tail-window must include at least N + affordance lines; got %d", height)
	require.LessOrEqual(t, height, expectedCeil,
		"tail-window must not exceed N + a small padding budget; got %d", height)

	tailPlain := ansi.Strip(tailItem.thinkingSec.out)

	require.Contains(t, tailPlain, "earlier lines hidden",
		"tail-windowed render must include the affordance footer")
	require.Contains(t, tailPlain, "ln5000",
		"tail-windowed render must include the LAST source paragraph — we tailed, not headed")
	require.NotContains(t, tailPlain, "ln1 ",
		"tail-windowed render must elide early source paragraphs")

	// Independent reference render: same source, same width, full
	// expansion (no tail slice). The tail-windowed output's last K
	// lines must byte-equal the unwindowed output's last K lines.
	fullMsg := thinkingMessageWithLines("tail-full-ref", total)
	fullItem := NewAssistantMessageItem(&sty, fullMsg).(*AssistantMessageItem)
	fullItem.thinkingViewMode = thinkingFullExpanded
	_ = fullItem.RawRender(width)
	fullPlain := ansi.Strip(fullItem.thinkingSec.out)

	tailLines := strings.Split(tailPlain, "\n")
	fullLines := strings.Split(fullPlain, "\n")

	// K is the cap minus a small budget that covers the affordance
	// prefix (hint line + blank line) and any framing differences
	// the bordered ThinkingBox style may introduce around the
	// edges. Documented inline because going much larger lets the
	// affordance row leak into the comparison; going much smaller
	// dilutes the assertion.
	const K = maxExpandedThinkingTailLines - 5
	require.GreaterOrEqual(t, len(tailLines), K,
		"tail render must contain at least K lines; got %d", len(tailLines))
	require.GreaterOrEqual(t, len(fullLines), K,
		"full render must contain at least K lines; got %d", len(fullLines))

	tailTail := tailLines[len(tailLines)-K:]
	fullTail := fullLines[len(fullLines)-K:]
	require.Equal(t, fullTail, tailTail,
		"tail-windowed render's last %d lines must byte-equal the unwindowed render's last %d lines (true tail -K relationship)",
		K, K)
}

// TestThinkingWindow_PromoteToFull verifies the cycle continues from
// tail-window to full expansion: the second toggle drops the
// affordance, removes the tail slice, and produces a render that
// matches a fresh item rendered directly in the full-expanded
// state.
func TestThinkingWindow_PromoteToFull(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	const total = 1500
	msg := thinkingMessageWithLines("promote", total)
	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)

	const width = 97

	require.True(t, item.ToggleExpanded())
	require.Equal(t, thinkingTailWindow, item.thinkingViewMode)
	_ = item.RawRender(width)
	tailOut := item.thinkingSec.out
	require.Contains(t, ansi.Strip(tailOut), "earlier lines hidden")

	require.True(t, item.ToggleExpanded(), "second toggle stays expanded (full)")
	require.Equal(t, thinkingFullExpanded, item.thinkingViewMode)
	_ = item.RawRender(width)
	fullOut := item.thinkingSec.out
	fullPlain := ansi.Strip(fullOut)

	require.NotContains(t, fullPlain, "earlier lines hidden",
		"full expansion must drop the tail-window affordance")
	require.Contains(t, fullPlain, "ln1 ",
		"full expansion must include the first source paragraph")
	require.Contains(t, fullPlain, "ln1500 ",
		"full expansion must include the last source paragraph")

	// Independent reference: a fresh item, rendered straight into
	// the full-expanded state, must produce byte-equal output.
	freshMsg := thinkingMessageWithLines("promote-fresh", total)
	fresh := NewAssistantMessageItem(&sty, freshMsg).(*AssistantMessageItem)
	fresh.thinkingViewMode = thinkingFullExpanded
	_ = fresh.RawRender(width)
	require.Equal(t, fresh.thinkingSec.out, fullOut,
		"cached full-expanded output must match a fresh full-expanded render")

	// And the cycle closes back to collapsed.
	require.False(t, item.ToggleExpanded(), "third toggle must report collapsed")
	require.Equal(t, thinkingCollapsed, item.thinkingViewMode)
}

// sectionKey is the tuple that defines a cache-hit identity for an
// assistantSection: (width, srcHash, extra). Comparing this tuple
// across mutations is a stronger invariant than byte-equality of
// rendered output: byte-equality could in principle hold even if
// the cache invalidated and re-rendered identical bytes, while
// tuple-equality proves the lookup key never moved.
type sectionKey struct {
	width   int
	srcHash uint64
	extra   uint64
}

func keyOf(s assistantSection) sectionKey {
	return sectionKey{width: s.width, srcHash: s.srcHash, extra: s.extra}
}

// TestThinkingWindow_ContentChangeKeepsThinkingCacheInTailWindow
// guards the F4/F5 boundary: streaming the main content while the
// thinking block sits in tail-window mode must NOT invalidate the
// thinking section cache. Tail-window state is folded into
// thinkingKey()'s extra hash, so changing only the content text
// keeps thinking's (srcHash, extra) tuple identical and the cache
// hits.
//
// The assertion is on the cache key tuple, not just rendered bytes:
// equal output could in principle survive a re-render with
// identical inputs, but identical (width, srcHash, extra) tuples
// across the SetMessage cycle prove the thinking cache was never
// invalidated to begin with. The mirror tuple on the content
// section MUST move (the source text changed), or the test isn't
// exercising what it claims to.
func TestThinkingWindow_ContentChangeKeepsThinkingCacheInTailWindow(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	const total = 1000

	build := func(content string) *message.Message {
		var b strings.Builder
		for i := 1; i <= total; i++ {
			b.WriteString("ln")
			b.WriteString(itoa(i))
			if i < total {
				b.WriteString("\n\n")
			}
		}
		parts := []message.ContentPart{
			message.ReasoningContent{
				Thinking:   b.String(),
				StartedAt:  testStartedAt,
				FinishedAt: testFinishedAt,
			},
		}
		if content != "" {
			parts = append(parts, message.TextContent{Text: content})
		}
		return &message.Message{ID: "tail-stream", Role: message.Assistant, Parts: parts}
	}

	item := NewAssistantMessageItem(&sty, build("first answer")).(*AssistantMessageItem)
	item.thinkingViewMode = thinkingTailWindow

	const width = 99
	_ = item.RawRender(width)
	first := snapshot(item)
	firstThinkingKey := keyOf(item.thinkingSec)
	firstContentKey := keyOf(item.contentSec)
	require.NotEmpty(t, first.thinking)

	item.SetMessage(build("first answer with more streaming text"))
	_ = item.RawRender(width)
	second := snapshot(item)
	secondThinkingKey := keyOf(item.thinkingSec)
	secondContentKey := keyOf(item.contentSec)

	require.Equal(t, firstThinkingKey, secondThinkingKey,
		"thinking section's (width, srcHash, extra) tuple must not move "+
			"across a content-only update — proves the cache key never invalidated")
	require.Equal(t, first.thinking, second.thinking,
		"content streaming must not invalidate the tail-windowed thinking cache")
	require.NotEqual(t, firstContentKey, secondContentKey,
		"content section's tuple MUST move; otherwise this test isn't exercising a real content change")
	require.NotEqual(t, first.content, second.content,
		"content section must have re-rendered")
}

// TestThinkingWindow_ToggleInvalidatesOnlyThinking verifies that
// cycling thinkingViewMode invalidates the thinking section cache
// alone — content and error caches survive across the toggle.
//
// Like TestThinkingWindow_ContentChangeKeepsThinkingCacheInTailWindow,
// the assertion is on the cache key tuple (width, srcHash, extra)
// at each section, not just on rendered bytes:
//   - thinking's tuple MUST move (extra folds in thinkingViewMode)
//   - content's and error's tuples MUST NOT move (their keys depend
//     only on their own source text, untouched by the toggle).
func TestThinkingWindow_ToggleInvalidatesOnlyThinking(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	const total = 1500
	build := func() *message.Message {
		var b strings.Builder
		for i := 1; i <= total; i++ {
			b.WriteString("ln")
			b.WriteString(itoa(i))
			if i < total {
				b.WriteString("\n\n")
			}
		}
		return &message.Message{
			ID:   "toggle-iso",
			Role: message.Assistant,
			Parts: []message.ContentPart{
				message.ReasoningContent{
					Thinking:   b.String(),
					StartedAt:  testStartedAt,
					FinishedAt: testFinishedAt,
				},
				message.TextContent{Text: "answer text"},
				message.Finish{
					Reason:  message.FinishReasonError,
					Message: "boom",
					Details: "details",
					Time:    testFinishTime,
				},
			},
		}
	}

	item := NewAssistantMessageItem(&sty, build()).(*AssistantMessageItem)

	const width = 101
	_ = item.RawRender(width)
	first := snapshot(item)
	firstThink := keyOf(item.thinkingSec)
	firstContent := keyOf(item.contentSec)
	firstErr := keyOf(item.errorSec)
	require.NotEmpty(t, first.thinking)
	require.NotEmpty(t, first.content)
	require.NotEmpty(t, first.errSec)

	// Cycle: collapsed -> tail-window. Only thinking should change.
	require.True(t, item.ToggleExpanded())
	require.Equal(t, thinkingTailWindow, item.thinkingViewMode)
	_ = item.RawRender(width)
	second := snapshot(item)
	secondThink := keyOf(item.thinkingSec)
	secondContent := keyOf(item.contentSec)
	secondErr := keyOf(item.errorSec)

	require.NotEqual(t, firstThink, secondThink,
		"thinking section's tuple MUST move on toggle (extra folds in thinkingViewMode)")
	require.Equal(t, firstContent, secondContent,
		"content section's tuple must not move on a thinking toggle")
	require.Equal(t, firstErr, secondErr,
		"error section's tuple must not move on a thinking toggle")
	require.NotEqual(t, first.thinking, second.thinking,
		"toggling into tail-window must re-render the thinking section")
	require.Equal(t, first.content, second.content,
		"toggling thinking view-mode must not invalidate the content section")
	require.Equal(t, first.errSec, second.errSec,
		"toggling thinking view-mode must not invalidate the error section")

	// Cycle: tail-window -> full. Same expectation.
	require.True(t, item.ToggleExpanded())
	require.Equal(t, thinkingFullExpanded, item.thinkingViewMode)
	_ = item.RawRender(width)
	third := snapshot(item)
	thirdThink := keyOf(item.thinkingSec)
	thirdContent := keyOf(item.contentSec)
	thirdErr := keyOf(item.errorSec)

	require.NotEqual(t, secondThink, thirdThink,
		"thinking section's tuple MUST move on the second toggle as well")
	require.Equal(t, secondContent, thirdContent,
		"content section's tuple must remain stable across the second toggle")
	require.Equal(t, secondErr, thirdErr,
		"error section's tuple must remain stable across the second toggle")
	require.NotEqual(t, second.thinking, third.thinking,
		"toggling into full expansion must re-render the thinking section")
	require.Equal(t, second.content, third.content)
	require.Equal(t, second.errSec, third.errSec)
}

// TestThinkingWindow_BoxHeightTracksWindow asserts that
// thinkingBoxHeight reflects the WINDOWED render's height in
// tail-window mode, not the (much larger) full thinking height.
// This is what HandleMouseClick uses to detect whether a click
// landed on the thinking box, so getting it wrong would make
// click detection extend off the bottom of the visible box.
func TestThinkingWindow_BoxHeightTracksWindow(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	const total = 5000
	msg := thinkingMessageWithLines("box-height", total)
	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)

	const width = 103

	// Tail-window: height should be roughly the cap.
	item.thinkingViewMode = thinkingTailWindow
	_ = item.RawRender(width)
	tailHeight := item.thinkingBoxHeight
	require.Greater(t, tailHeight, 0)
	require.LessOrEqual(t, tailHeight, maxExpandedThinkingTailLines+5,
		"tail-window box height must reflect the windowed render, not the full thinking height; got %d",
		tailHeight)

	// Full expansion: height should grow well past the tail cap.
	item.thinkingViewMode = thinkingFullExpanded
	_ = item.RawRender(width)
	fullHeight := item.thinkingBoxHeight
	require.Greater(t, fullHeight, maxExpandedThinkingTailLines*2,
		"full expansion box height must reflect the full thinking render; got %d",
		fullHeight)
}
