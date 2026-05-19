package chat

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/attachments"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

// finishedAssistantMessage builds an assistant message with text content and a
// finish part so AssistantMessageItem.isSpinning returns false and the
// prefix cache is exercised.
func finishedAssistantMessage(id, text string) *message.Message {
	return &message.Message{
		ID:   id,
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: text},
			message.Finish{Reason: message.FinishReasonEndTurn, Time: time.Now().Unix()},
		},
	}
}

// TestAssistantMessageItemRender_PrefixCacheFocusBlur covers the F3 invariant
// that focus → blur → focus produces the correct prefix every time and never
// leaks the previous focus state out of the cache.
func TestAssistantMessageItemRender_PrefixCacheFocusBlur(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	msg := finishedAssistantMessage("m1", "Hello world from the cache test.")
	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)

	const width = 60

	item.SetFocused(true)
	focused1 := item.Render(width)
	focused2 := item.Render(width)
	require.Equal(t, focused1, focused2, "second render must hit the cache and match the first")

	item.SetFocused(false)
	blurred1 := item.Render(width)
	require.NotEqual(t, focused1, blurred1, "blur must produce a different prefixed render than focus")

	item.SetFocused(true)
	focused3 := item.Render(width)
	require.Equal(t, focused1, focused3, "re-focus must produce identical output to the original focused render")
}

// TestAssistantMessageItemRender_PrefixCacheWidthInvalidates asserts that a
// width change does not return the previous width's cached output.
func TestAssistantMessageItemRender_PrefixCacheWidthInvalidates(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	msg := finishedAssistantMessage("m2", "Some content that wraps differently at different widths so the rendered output diverges.")
	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)
	item.SetFocused(true)

	narrow := item.Render(40)
	wide := item.Render(100)
	require.NotEqual(t, narrow, wide, "different widths must produce different rendered output")

	narrowAgain := item.Render(40)
	require.Equal(t, narrow, narrowAgain, "returning to the original width must hit (or repopulate) the cache with the same output")
}

// TestAssistantMessageItemRender_PrefixCacheHighlightOnTop guarantees that
// activating a highlight range bypasses the prefix cache so selection drags
// reflect immediately, and that clearing the highlight returns to the cached
// prefixed output unchanged.
func TestAssistantMessageItemRender_PrefixCacheHighlightOnTop(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	msg := finishedAssistantMessage("m3", "Hello world from the highlight test.")
	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)
	item.SetFocused(true)

	const width = 60
	plain := item.Render(width)

	// Activating a highlight must change the rendered output (selection
	// painted on top) without poisoning the cache for the un-highlighted
	// state that follows.
	item.SetHighlight(0, 0, 0, 5)
	highlighted := item.Render(width)
	require.NotEqual(t, plain, highlighted, "active highlight must change Render output")

	// Clear the highlight; the cached un-highlighted prefix render must
	// be returned unchanged.
	item.SetHighlight(-1, -1, -1, -1)
	plainAfter := item.Render(width)
	require.Equal(t, plain, plainAfter, "clearing the highlight must restore the cached prefixed output exactly")
}

// TestUserMessageItemRender_PrefixCacheFocusBlur is the user-message
// counterpart of the assistant focus/blur cache test.
func TestUserMessageItemRender_PrefixCacheFocusBlur(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	msg := &message.Message{
		ID:   "u1",
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: "Hello from the user."},
		},
	}
	r := attachments.NewRenderer(
		sty.Attachments.Normal,
		sty.Attachments.Deleting,
		sty.Attachments.Image,
		sty.Attachments.Text,
	)
	item := NewUserMessageItem(&sty, msg, r).(*UserMessageItem)

	const width = 60

	item.SetFocused(true)
	focused1 := item.Render(width)
	focused2 := item.Render(width)
	require.Equal(t, focused1, focused2)

	item.SetFocused(false)
	blurred := item.Render(width)
	require.NotEqual(t, focused1, blurred)

	item.SetFocused(true)
	focused3 := item.Render(width)
	require.Equal(t, focused1, focused3)
}

// TestCachedMessageItem_PrefixCacheSemantics covers the constant-prefix
// path used by AssistantInfoItem and the (width, key) keying used by every
// item, against the underlying cachedMessageItem helper directly. This
// avoids constructing a full *config.Config with an initialized provider
// map just to exercise cache plumbing that is identical for all callers.
func TestCachedMessageItem_PrefixCacheSemantics(t *testing.T) {
	t.Parallel()

	c := &cachedMessageItem{}

	// Empty cache: miss.
	_, ok := c.getCachedPrefixedRender(80, 0)
	require.False(t, ok)

	// Set then hit at the same (width, key).
	c.setCachedPrefixedRender("hello", 80, 0)
	got, ok := c.getCachedPrefixedRender(80, 0)
	require.True(t, ok)
	require.Equal(t, "hello", got)

	// Different width: miss.
	_, ok = c.getCachedPrefixedRender(120, 0)
	require.False(t, ok)

	// Different key (focused vs blurred): miss.
	_, ok = c.getCachedPrefixedRender(80, 1)
	require.False(t, ok)

	// clearCache drops the prefixed cache too.
	c.setCachedRender("raw", 80, 1)
	c.setCachedPrefixedRender("hello", 80, 0)
	c.clearCache()
	_, ok = c.getCachedPrefixedRender(80, 0)
	require.False(t, ok, "clearCache must drop the prefixed render cache")
	_, _, ok = c.getCachedRender(80)
	require.False(t, ok, "clearCache must also drop the raw render cache")
}

// TestAssistantMessageItemRender_PrefixCacheNoCacheLeak guards against a
// regression where the cache returned the prefixed output of the previous
// width. We verify that the cached output for width=W contains the W-sized
// prefix and not a stale wider one by checking that line lengths are
// consistent on cache hit.
func TestAssistantMessageItemRender_PrefixCacheNoCacheLeak(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	msg := finishedAssistantMessage("m4", strings.Repeat("word ", 40))
	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)
	item.SetFocused(true)

	out80 := item.Render(80)
	out120 := item.Render(120)
	require.NotEqual(t, out80, out120)

	// Hit each cached entry again and confirm stability.
	require.Equal(t, out80, item.Render(80))
	require.Equal(t, out120, item.Render(120))
}
