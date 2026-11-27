package dialog

import (
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/stretchr/testify/require"
)

// TestSessionItemCachesRender verifies a session row keeps its rendered
// line so repeated renders at the same width are reused.
func TestSessionItemCachesRender(t *testing.T) {
	t.Parallel()
	com := newPickerTestCommon()
	item := &SessionItem{Versioned: &list.Versioned{}, Session: session.Session{ID: "a", Title: "alpha", UpdatedAt: time.Now().Unix()}, t: com.Styles}
	first := item.Render(40)
	require.Len(t, item.cache, 1)
	require.Equal(t, first, item.Render(40))
	require.Len(t, item.cache, 1, "a repeat render at the same width reuses the line")

	item.SetFocused(true)
	require.NotEqual(t, first, item.Render(40), "focus change must invalidate the cache")
}

// TestSessionItemRefreshesRelativeTime verifies the cached row follows its
// "ago" text: a session list left open must not freeze at "now".
func TestSessionItemRefreshesRelativeTime(t *testing.T) {
	t.Parallel()
	com := newPickerTestCommon()
	item := &SessionItem{Versioned: &list.Versioned{}, Session: session.Session{ID: "a", Title: "alpha", UpdatedAt: time.Now().Unix()}, t: com.Styles}
	require.Contains(t, item.Render(60), "now")

	// The same session, a few minutes later.
	item.UpdatedAt = time.Now().Add(-3 * time.Minute).Unix()
	require.Contains(t, item.Render(60), "3 minutes ago")
}

// TestNotificationItemCachesRender verifies a notification style row keeps
// its rendered line, and that focus changes still invalidate it.
func TestNotificationItemCachesRender(t *testing.T) {
	t.Parallel()
	com := newPickerTestCommon()
	item := &NotificationItem{Versioned: &list.Versioned{}, style: NotificationStyle{ID: "bell", Title: "Bell"}, t: com.Styles}
	first := item.Render(40)
	require.Len(t, item.cache, 1)
	require.Equal(t, first, item.Render(40))

	item.SetFocused(true)
	require.NotEqual(t, first, item.Render(40), "focus change must invalidate the cache")
}
