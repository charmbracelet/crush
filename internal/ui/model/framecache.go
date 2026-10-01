package model

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

const (
	// frameCacheTTL bounds how long a memoized frame may be served after it
	// was rendered. It is a safety valve: even if a state change slips past
	// invalidation, a stale frame is replaced within one TTL of the next
	// update, and by the idle GC within two TTLs when there is none.
	frameCacheTTL = 3 * time.Second

	// frameCacheMaxEntries caps the number of memoized frames. A frame is
	// the full styled screen as text; at a large terminal with heavily
	// styled chat content this can reach several hundred KB each, so the
	// cap bounds worst-case memory to roughly 10-20 MB.
	frameCacheMaxEntries = 32
)

// frameGCMsg is sent on a timer to sweep expired frames when the UI is
// otherwise idle, so a burst of scrolling does not pin memory indefinitely.
type frameGCMsg struct{}

// frameKey identifies a rendered frame for a given chat scroll position.
// Everything else that affects the frame (layout, dialogs, items, sidebar
// content, textarea) invalidates the whole cache via reset, so the key only
// needs to capture what a scroll-only update is allowed to change. State
// that is read live at render time rather than delivered by a message
// (e.g. the agent busy flag) can lag for at most one TTL within a scroll
// burst; the TTL is the safety valve for that.
type frameKey struct {
	width, height int
	chat          RenderState
}

type frameEntry struct {
	content string
	cursor  *tea.Cursor
	// at is the render time and drives TTL expiry; lastUsed is the last hit
	// and drives eviction, so a hot frame (e.g. the clamped bottom during
	// overscroll) is kept while still expiring on schedule.
	at       time.Time
	lastUsed time.Time
}

// frameCache memoizes fully rendered frames keyed by chat scroll position.
//
// Bubble Tea calls View after every Update, so a flood of wheel events pays
// for a full redraw per event even when the scroll offset did not move
// (clamped at the top or bottom) or moved back to a position rendered a
// moment ago. Serving those frames from memory keeps the event loop
// draining instead of blocking input behind redraw work.
type frameCache struct {
	entries map[frameKey]*frameEntry
	ttl     time.Duration
	max     int
	now     func() time.Time

	hits, misses int
	// putSkips counts frames dropped because the state moved while Draw
	// was rendering them.
	putSkips int
}

func newFrameCache(ttl time.Duration, maxEntries int) *frameCache {
	return &frameCache{
		entries: make(map[frameKey]*frameEntry, maxEntries),
		ttl:     ttl,
		max:     maxEntries,
		now:     time.Now,
	}
}

// Len returns the number of memoized frames, including expired ones that
// have not yet been swept.
func (c *frameCache) Len() int {
	return len(c.entries)
}

func (c *frameCache) expired(e *frameEntry, now time.Time) bool {
	return now.Sub(e.at) > c.ttl
}

// get returns the memoized frame for key if present and within TTL.
func (c *frameCache) get(key frameKey) (string, *tea.Cursor, bool) {
	e, ok := c.entries[key]
	if !ok {
		c.misses++
		return "", nil, false
	}
	now := c.now()
	if c.expired(e, now) {
		delete(c.entries, key)
		c.misses++
		return "", nil, false
	}
	c.hits++
	e.lastUsed = now
	if e.cursor == nil {
		return e.content, nil, true
	}
	cur := *e.cursor
	return e.content, &cur, true
}

// put memoizes a frame, sweeping expired entries first and evicting the
// least recently used entry when the cache is full.
func (c *frameCache) put(key frameKey, content string, cursor *tea.Cursor) {
	now := c.now()
	c.gc(now)
	if _, ok := c.entries[key]; !ok && len(c.entries) >= c.max {
		c.evictLRU()
	}
	var cur *tea.Cursor
	if cursor != nil {
		copied := *cursor
		cur = &copied
	}
	c.entries[key] = &frameEntry{content: content, cursor: cur, at: now, lastUsed: now}
}

// gc removes every entry older than the TTL.
func (c *frameCache) gc(now time.Time) {
	for key, e := range c.entries {
		if c.expired(e, now) {
			delete(c.entries, key)
		}
	}
}

func (c *frameCache) evictLRU() {
	var (
		victimKey frameKey
		victim    *frameEntry
	)
	for key, e := range c.entries {
		if victim == nil || e.lastUsed.Before(victim.lastUsed) {
			victimKey, victim = key, e
		}
	}
	if victim != nil {
		delete(c.entries, victimKey)
	}
}

// reset drops every memoized frame. Called whenever an update may have
// changed anything other than the chat scroll position.
func (c *frameCache) reset() {
	if len(c.entries) == 0 {
		return
	}
	clear(c.entries)
}

// markScrollOnly flags the current update as having changed nothing but the
// chat scroll position or selection, which lets View serve a memoized frame
// for that position. Update arms the idle GC timer when the flag is set; it
// is not returned here so callers that sequence their commands do not put a
// blocking tick ahead of real work.
func (m *UI) markScrollOnly() {
	m.scrollOnlyUpdate = true
}

// invalidateFrames forces the next View to drop all memoized frames, even if
// the current update was marked scroll-only.
func (m *UI) invalidateFrames() {
	m.frameDirty = true
}

// beginFrameUpdate clears the per-update memoization flag so a flag left
// over from an update whose View never consumed it cannot leak into this
// one.
func (m *UI) beginFrameUpdate() {
	m.scrollOnlyUpdate = false
	m.frameSkipPut = false
}

// endFrameUpdate returns the command that arms the idle GC timer when this
// update was scroll-only, the coming View can actually memoize a frame, and
// no timer is pending.
func (m *UI) endFrameUpdate() tea.Cmd {
	if !m.scrollOnlyUpdate || !m.frameCacheable() {
		return nil
	}
	return m.armFrameGC()
}

// frameCacheable reports whether the current UI state renders frames that
// may be memoized.
func (m *UI) frameCacheable() bool {
	return m.frames != nil && m.state == uiChat && !m.dialog.HasDialogs()
}

func (m *UI) armFrameGC() tea.Cmd {
	if m.frames == nil || m.frameGCArmed {
		return nil
	}
	m.frameGCArmed = true
	return tea.Tick(m.frames.ttl, func(time.Time) tea.Msg { return frameGCMsg{} })
}

// handleFrameGC sweeps expired frames. The sweep itself changes nothing
// visible, so the update is marked scroll-only to avoid discarding
// still-valid frames; Update re-arms the timer through endFrameUpdate while
// frames remain. When the sweep empties the cache the UI has been idle for a
// full TTL: the frame rendered by the following View is not memoized so
// nothing stays pinned and the timer is left disarmed until the next scroll.
func (m *UI) handleFrameGC() {
	m.frameGCArmed = false
	if m.frames == nil {
		return
	}
	m.frames.gc(m.frames.now())
	if m.frames.Len() == 0 {
		m.frameSkipPut = true
		return
	}
	m.scrollOnlyUpdate = true
}

// frameKeyNow returns the key for the current state without touching the
// per-update flags.
func (m *UI) frameKeyNow() (frameKey, bool) {
	if m.frames == nil || !m.frameCacheable() {
		return frameKey{}, false
	}
	return frameKey{
		width:  m.width,
		height: m.height,
		chat:   m.chat.RenderState(),
	}, true
}

// currentFrameKey consumes the per-update memoization flags, resets the cache
// when the update was not scroll-only, and returns the key for the frame
// about to be rendered. ok is false when the current state is not cacheable.
func (m *UI) currentFrameKey() (key frameKey, ok bool) {
	scrollOnly := m.scrollOnlyUpdate && !m.frameDirty
	m.scrollOnlyUpdate = false
	m.frameDirty = false
	if m.frames == nil {
		return frameKey{}, false
	}
	if !scrollOnly {
		m.frames.reset()
	}
	if !m.frameCacheable() {
		return frameKey{}, false
	}
	return m.frameKeyNow()
}

// storeFrame memoizes a freshly rendered frame under key. If Draw changed
// the layout while rendering (frameDirty set after currentFrameKey consumed
// it) the frame no longer matches the key and every earlier frame is stale,
// so the cache is reset instead. GC-triggered renders are not stored so the
// cache drains fully when idle.
func (m *UI) storeFrame(key frameKey, content string, cursor *tea.Cursor) {
	if m.frameDirty {
		m.frameDirty = false
		m.frames.reset()
		return
	}
	if m.frameSkipPut {
		m.frameSkipPut = false
		return
	}
	// Re-read the key after Draw. Rendering an item may bump its version
	// (the list does the same re-read for the same reason), and Draw may
	// re-anchor the list in follow mode. Either way the frame no longer
	// depicts the state key describes, so storing it would serve a frame
	// under a position it does not match.
	if now, ok := m.frameKeyNow(); !ok || now != key {
		m.frames.putSkips++
		return
	}
	m.frames.put(key, content, cursor)
}
