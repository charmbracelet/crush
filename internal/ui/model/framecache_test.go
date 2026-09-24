package model

import (
	"strconv"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/attachments"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/list"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"
)

func newTestFrameCache(ttl time.Duration, maxEntries int) (*frameCache, *time.Time) {
	now := time.Unix(1_000_000, 0)
	c := newFrameCache(ttl, maxEntries)
	c.now = func() time.Time { return now }
	return c, &now
}

func TestFrameCache_PutGet(t *testing.T) {
	t.Parallel()
	c, _ := newTestFrameCache(time.Second, 4)
	key := frameKey{width: 80, height: 24, chat: RenderState{OffsetIdx: 1}}

	_, _, ok := c.get(key)
	require.False(t, ok)

	c.put(key, "frame", &tea.Cursor{Position: tea.Position{X: 1, Y: 2}})
	content, cursor, ok := c.get(key)
	require.True(t, ok)
	require.Equal(t, "frame", content)
	require.NotNil(t, cursor)
	require.Equal(t, 1, cursor.X)
	require.Equal(t, 2, cursor.Y)
	require.Equal(t, 1, c.hits)
	require.Equal(t, 1, c.misses)
}

func TestFrameCache_CursorIsCopied(t *testing.T) {
	t.Parallel()
	c, _ := newTestFrameCache(time.Second, 4)
	key := frameKey{width: 80, height: 24}
	orig := &tea.Cursor{Position: tea.Position{X: 1}}
	c.put(key, "frame", orig)
	orig.X = 99

	_, cursor, ok := c.get(key)
	require.True(t, ok)
	require.Equal(t, 1, cursor.X)
	cursor.X = 42

	_, again, _ := c.get(key)
	require.Equal(t, 1, again.X)
}

func TestFrameCache_TTLExpiry(t *testing.T) {
	t.Parallel()
	c, now := newTestFrameCache(time.Second, 4)
	key := frameKey{width: 80, height: 24}
	c.put(key, "frame", nil)

	*now = now.Add(time.Second)
	_, _, ok := c.get(key)
	require.True(t, ok, "entry at exactly TTL should still be served")

	*now = now.Add(time.Millisecond)
	_, _, ok = c.get(key)
	require.False(t, ok, "entry past TTL must not be served")
	require.Equal(t, 0, c.Len(), "expired entry must be dropped on read")
}

func TestFrameCache_GCSweepsExpired(t *testing.T) {
	t.Parallel()
	c, now := newTestFrameCache(time.Second, 8)
	for i := range 4 {
		c.put(frameKey{chat: RenderState{OffsetIdx: i}}, "old", nil)
	}
	*now = now.Add(2 * time.Second)
	for i := range 2 {
		c.put(frameKey{chat: RenderState{OffsetIdx: 10 + i}}, "new", nil)
	}
	require.Equal(t, 2, c.Len(), "put must sweep expired entries")

	*now = now.Add(2 * time.Second)
	c.gc(*now)
	require.Equal(t, 0, c.Len())
}

func TestFrameCache_EvictsLeastRecentlyUsedWhenFull(t *testing.T) {
	t.Parallel()
	c, now := newTestFrameCache(time.Minute, 3)
	for i := range 3 {
		c.put(frameKey{chat: RenderState{OffsetIdx: i}}, "frame", nil)
		*now = now.Add(time.Millisecond)
	}
	require.Equal(t, 3, c.Len())

	// Touch the oldest entry so it becomes the most recently used.
	_, _, ok := c.get(frameKey{chat: RenderState{OffsetIdx: 0}})
	require.True(t, ok)
	*now = now.Add(time.Millisecond)

	c.put(frameKey{chat: RenderState{OffsetIdx: 3}}, "frame", nil)
	require.Equal(t, 3, c.Len())
	_, _, ok = c.get(frameKey{chat: RenderState{OffsetIdx: 1}})
	require.False(t, ok, "least recently used entry must be evicted")
	for _, idx := range []int{0, 2, 3} {
		_, _, ok := c.get(frameKey{chat: RenderState{OffsetIdx: idx}})
		require.True(t, ok, "entry %d must survive", idx)
	}
}

func TestFrameCache_HitDoesNotExtendTTL(t *testing.T) {
	t.Parallel()
	c, now := newTestFrameCache(time.Second, 4)
	key := frameKey{chat: RenderState{OffsetIdx: 0}}
	c.put(key, "frame", nil)
	for range 5 {
		*now = now.Add(150 * time.Millisecond)
		_, _, ok := c.get(key)
		require.True(t, ok)
	}
	*now = now.Add(300 * time.Millisecond)
	_, _, ok := c.get(key)
	require.False(t, ok, "repeated hits must not keep a frame alive past its TTL")
}

func TestFrameCache_OverwriteDoesNotEvict(t *testing.T) {
	t.Parallel()
	c, _ := newTestFrameCache(time.Minute, 2)
	c.put(frameKey{chat: RenderState{OffsetIdx: 0}}, "a", nil)
	c.put(frameKey{chat: RenderState{OffsetIdx: 1}}, "b", nil)
	c.put(frameKey{chat: RenderState{OffsetIdx: 1}}, "b2", nil)
	require.Equal(t, 2, c.Len())
	content, _, ok := c.get(frameKey{chat: RenderState{OffsetIdx: 0}})
	require.True(t, ok)
	require.Equal(t, "a", content)
}

func TestFrameCache_Reset(t *testing.T) {
	t.Parallel()
	c, _ := newTestFrameCache(time.Minute, 4)
	c.put(frameKey{chat: RenderState{OffsetIdx: 0}}, "a", nil)
	c.reset()
	require.Equal(t, 0, c.Len())
	c.reset()
}

// stubDialog is an empty dialog used to exercise the dialog-open path.
type stubDialog struct{}

func (stubDialog) ID() string                               { return "stub" }
func (stubDialog) HandleMsg(tea.Msg) dialog.Action          { return nil }
func (stubDialog) Draw(uv.Screen, uv.Rectangle) *tea.Cursor { return nil }

// focusableTestItem is a testMessageItem that participates in selection so
// the chat scroll handlers behave as they do with real message items.
type focusableTestItem struct {
	testMessageItem
	focused bool
}

func (m *focusableTestItem) SetFocused(focused bool) { m.focused = focused }

var _ list.Focusable = (*focusableTestItem)(nil)

// newFrameTestUI returns a chat UI with a frame cache and enough items to
// scroll, sized so layout is stable across Views.
func newFrameTestUI(t testing.TB) *UI {
	t.Helper()
	u := newTestUI()
	u.com.Workspace = &testWorkspace{cfg: &config.Config{}}
	u.frames = newFrameCache(time.Minute, 8)
	u.dialog = dialog.NewOverlay()
	u.keyMap = DefaultKeyMap()
	u.focus = uiFocusMain
	u.status = NewStatus(u.com, u)
	u.attachments = attachments.New(nil, attachments.Keymap{})
	u.chat.scrollbarMode = config.ScrollbarAlways
	u.chat.SetMessages(frameTestItems("line")...)
	u.updateLayoutAndSize()
	return u
}

func frameTestItems(prefix string) []chat.MessageItem {
	items := make([]chat.MessageItem, 0, 200)
	for i := range 200 {
		items = append(items, &focusableTestItem{
			testMessageItem: testMessageItem{id: strconv.Itoa(i), text: prefix + " " + strconv.Itoa(i)},
		})
	}
	return items
}

// scrollOnlyUpdate simulates a scroll-only Update followed by View.
func scrollOnlyUpdate(u *UI, lines int) tea.View {
	u.beginFrameUpdate()
	u.markScrollOnly()
	_ = u.chat.ScrollBy(lines)
	u.endFrameUpdate()
	return u.View()
}

func TestView_ScrollOnlyUpdateHitsCache(t *testing.T) {
	t.Parallel()
	u := newFrameTestUI(t)

	first := u.View()
	require.Equal(t, 0, u.frames.hits)
	require.Equal(t, 1, u.frames.Len(), "fresh frame must be memoized")

	// A non-scroll update resets the cache and renders again.
	second := u.View()
	require.Equal(t, 0, u.frames.hits)
	require.Equal(t, 2, u.frames.misses)
	require.Equal(t, first.Content, second.Content)

	// A scroll-only update at the same position is served from cache.
	u.markScrollOnly()
	third := u.View()
	require.Equal(t, 1, u.frames.hits)
	require.Equal(t, first.Content, third.Content)
	require.False(t, u.scrollOnlyUpdate, "flag must be consumed by View")
}

func TestView_ScrollAwayAndBackHitsCache(t *testing.T) {
	t.Parallel()
	u := newFrameTestUI(t)
	u.chat.ScrollToTop()

	top := u.View()
	down := scrollOnlyUpdate(u, 4)
	require.NotEqual(t, top.Content, down.Content, "scrolling must change the frame")
	require.Equal(t, 0, u.frames.hits)
	require.Equal(t, 2, u.frames.Len())

	back := scrollOnlyUpdate(u, -4)
	require.Equal(t, 1, u.frames.hits, "returning to a rendered position must hit")
	require.Equal(t, top.Content, back.Content)

	again := scrollOnlyUpdate(u, 4)
	require.Equal(t, 2, u.frames.hits)
	require.Equal(t, down.Content, again.Content)
}

func TestView_ClampedOverscrollHitsCache(t *testing.T) {
	t.Parallel()
	u := newFrameTestUI(t)
	u.chat.ScrollToBottom()
	bottom := u.View()

	for range 5 {
		v := scrollOnlyUpdate(u, 5)
		require.Equal(t, bottom.Content, v.Content)
	}
	require.Equal(t, 5, u.frames.hits, "overscroll at the bottom must be served from cache")
	require.Equal(t, 1, u.frames.Len())
}

func TestView_ContentChangeBetweenScrollsIsNotStale(t *testing.T) {
	t.Parallel()
	u := newFrameTestUI(t)
	u.chat.ScrollToTop()
	before := u.View()

	// A message update is not scroll-only: the cache must reset.
	u.beginFrameUpdate()
	u.chat.SetMessages(frameTestItems("changed")...)
	u.chat.ScrollToTop()
	u.endFrameUpdate()
	after := u.View()
	require.NotEqual(t, before.Content, after.Content)
	require.Equal(t, 0, u.frames.hits)

	// A following scroll-only View at the same key serves the new content.
	u.markScrollOnly()
	v := u.View()
	require.Equal(t, 1, u.frames.hits)
	require.Equal(t, after.Content, v.Content)
}

func TestView_LayoutChangeOverridesScrollOnly(t *testing.T) {
	t.Parallel()
	u := newFrameTestUI(t)

	u.View()
	require.Equal(t, 1, u.frames.Len())

	u.markScrollOnly()
	u.updateSize()
	u.View()
	require.Equal(t, 0, u.frames.hits, "layout change must not serve a memoized frame")
	require.False(t, u.frameDirty)
}

func TestView_LayoutChangeDuringDrawIsNotStoredUnderStaleKey(t *testing.T) {
	t.Parallel()
	u := newFrameTestUI(t)
	u.View()
	require.Equal(t, 1, u.frames.Len())

	// Resize without recomputing layout: Draw detects the drift, calls
	// updateSize, and sets frameDirty after the key was computed.
	u.markScrollOnly()
	u.width += 10
	u.View()
	require.Equal(t, 0, u.frames.hits)
	require.Equal(t, 0, u.frames.Len(), "frame drawn under a changed layout must not be memoized")
	require.False(t, u.frameDirty, "dirty flag must not leak into the next update")

	u.View()
	require.Equal(t, 1, u.frames.Len())
}

func TestView_DialogDisablesCache(t *testing.T) {
	t.Parallel()
	u := newFrameTestUI(t)
	u.View()
	require.Equal(t, 1, u.frames.Len())

	u.dialog.OpenDialog(stubDialog{})
	u.View()
	require.Equal(t, 0, u.frames.Len(), "opening a dialog must drop memoized frames")

	u.markScrollOnly()
	u.View()
	require.Equal(t, 0, u.frames.hits)
	require.Equal(t, 0, u.frames.Len(), "frames must not be memoized while a dialog is open")
}

func TestFrameGC_ArmsOnceAndDrainsWhenIdle(t *testing.T) {
	t.Parallel()
	u := newFrameTestUI(t)

	u.beginFrameUpdate()
	require.Nil(t, u.endFrameUpdate(), "non-scroll update must not arm the GC timer")

	u.markScrollOnly()
	require.NotNil(t, u.endFrameUpdate(), "first scroll-only update must arm the GC timer")
	require.True(t, u.frameGCArmed)
	require.Nil(t, u.endFrameUpdate(), "must not double-arm")
	u.View()
	require.Equal(t, 1, u.frames.Len())

	// GC while frames are still fresh: nothing swept, frame served, re-armed.
	u.beginFrameUpdate()
	u.handleFrameGC()
	require.True(t, u.scrollOnlyUpdate, "GC must not invalidate frames")
	require.NotNil(t, u.endFrameUpdate(), "GC must re-arm while frames remain")
	u.View()
	require.Equal(t, 1, u.frames.hits)

	// GC after everything expired: cache drains and the render is not stored.
	now := time.Now()
	u.frames.now = func() time.Time { return now.Add(2 * time.Minute) }
	u.beginFrameUpdate()
	u.handleFrameGC()
	require.False(t, u.scrollOnlyUpdate)
	require.True(t, u.frameSkipPut)
	require.Nil(t, u.endFrameUpdate(), "GC must stop once the cache is empty")
	u.View()
	require.Equal(t, 0, u.frames.Len(), "post-drain render must not be memoized")
	require.False(t, u.frameSkipPut)
	require.False(t, u.frameGCArmed)
}

func TestKeyScroll_DoesNotSequenceGCTick(t *testing.T) {
	t.Parallel()
	u := newFrameTestUI(t)
	u.chat.Focus()
	u.chat.ScrollToBottom()

	u.beginFrameUpdate()
	cmd := u.handleKeyPressMsg(tea.KeyPressMsg{Code: tea.KeyPgUp})
	require.True(t, u.scrollOnlyUpdate)
	require.False(t, u.frameGCArmed, "key handler must not arm the GC timer itself")
	require.Nil(t, cmd, "scroll key with no animations must produce no sequenced cmds")
	require.NotNil(t, u.endFrameUpdate())
}

func TestUpdate_WheelScrollThroughUpdateHitsCache(t *testing.T) {
	t.Parallel()
	u := newFrameTestUI(t)
	u.chat.ScrollToBottom()
	u.chat.SetSelected(u.chat.Len() - 1)
	bottom := u.View()

	wheel := common.CoalescedWheelMsg{
		Mouse:  tea.Mouse{X: u.layout.main.Min.X + 1, Y: u.layout.main.Min.Y + 1},
		DeltaY: 1,
	}
	_, cmd := u.Update(wheel)
	require.NotNil(t, cmd, "first scroll-only update must return the GC arm")
	require.True(t, u.frameGCArmed)
	v := u.View()
	require.Equal(t, 1, u.frames.hits, "clamped wheel scroll must be served from cache")
	require.Equal(t, bottom.Content, v.Content)

	_, _ = u.Update(wheel)
	u.View()
	require.Equal(t, 2, u.frames.hits)

	// Scrolling up moves to a new position (and may move the selection):
	// a miss. Scrolling back down and then overscrolling again must hit
	// the frame rendered on the way back.
	up := wheel
	up.DeltaY = -1
	_, _ = u.Update(up)
	moved := u.View()
	require.NotEqual(t, bottom.Content, moved.Content)
	require.Equal(t, 2, u.frames.hits)
	_, _ = u.Update(wheel)
	back := u.View()
	_, _ = u.Update(wheel)
	again := u.View()
	require.Equal(t, 3, u.frames.hits)
	require.Equal(t, back.Content, again.Content)
	require.True(t, u.chat.AtBottom())
}

func TestUpdate_FrameGCMsgReArmsThroughUpdate(t *testing.T) {
	t.Parallel()
	u := newFrameTestUI(t)
	u.View()

	u.markScrollOnly()
	require.NotNil(t, u.endFrameUpdate())
	u.View()
	require.Equal(t, 1, u.frames.Len())

	_, cmd := u.Update(frameGCMsg{})
	require.NotNil(t, cmd, "GC through Update must re-arm while frames remain")
	require.True(t, u.frameGCArmed)
	u.View()
	require.Equal(t, 2, u.frames.hits, "GC must not discard fresh frames")

	now := time.Now()
	u.frames.now = func() time.Time { return now.Add(2 * time.Minute) }
	_, cmd = u.Update(frameGCMsg{})
	require.Nil(t, cmd, "GC through Update must disarm once drained")
	require.False(t, u.frameGCArmed)
	u.View()
	require.Equal(t, 0, u.frames.Len())
}

func TestUpdate_DirtyFromNonCacheableViewResetsNextScrollOnlyView(t *testing.T) {
	t.Parallel()
	u := newFrameTestUI(t)
	u.View()
	require.Equal(t, 1, u.frames.Len())

	// A layout change inside a scroll-only update while a dialog is open
	// leaves frameDirty behind; the next scroll-only view after the dialog
	// closes must not serve the pre-change frame.
	u.dialog.OpenDialog(stubDialog{})
	u.markScrollOnly()
	u.updateSize()
	u.View()
	require.Equal(t, 0, u.frames.Len())

	u.dialog.CloseFrontDialog()
	u.markScrollOnly()
	u.View()
	require.Equal(t, 0, u.frames.hits)
	require.Equal(t, 1, u.frames.Len())
}
