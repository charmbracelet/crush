package split

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDirtyTrackerBasic(t *testing.T) {
	t.Parallel()
	d := NewDirtyTracker()

	d.Track("p1")
	d.Track("p2")
	d.Track("p3")

	// All start dirty.
	require.True(t, d.IsDirty("p1"))
	require.True(t, d.IsDirty("p2"))
	require.True(t, d.IsDirty("p3"))
	require.Equal(t, 3, d.DirtyCount())

	// Clear one.
	d.ClearDirty("p1")
	require.False(t, d.IsDirty("p1"))
	require.Equal(t, 2, d.DirtyCount())

	// Mark dirty again.
	d.MarkDirty("p1")
	require.True(t, d.IsDirty("p1"))
}

func TestDirtyTrackerMarkAllDirty(t *testing.T) {
	t.Parallel()
	d := NewDirtyTracker()

	d.Track("p1")
	d.Track("p2")
	d.ClearDirty("p1")
	d.ClearDirty("p2")
	require.Equal(t, 0, d.DirtyCount())

	d.MarkAllDirty()
	require.Equal(t, 2, d.DirtyCount())
}

func TestDirtyTrackerCachedRender(t *testing.T) {
	t.Parallel()
	d := NewDirtyTracker()

	d.Track("p1")
	require.True(t, d.IsDirty("p1"))

	d.SetCachedRender("p1", "rendered content")
	require.False(t, d.IsDirty("p1")) // SetCachedRender clears dirty

	cached, ok := d.GetCachedRender("p1")
	require.True(t, ok)
	require.Equal(t, "rendered content", cached)

	// Non-existent pane.
	_, ok = d.GetCachedRender("nonexistent")
	require.False(t, ok)
}

func TestDirtyTrackerRemove(t *testing.T) {
	t.Parallel()
	d := NewDirtyTracker()

	d.Track("p1")
	d.SetCachedRender("p1", "content")

	d.Remove("p1")
	require.False(t, d.IsDirty("p1"))
	_, ok := d.GetCachedRender("p1")
	require.False(t, ok)
}

func TestDirtyTrackerDirtyPanes(t *testing.T) {
	t.Parallel()
	d := NewDirtyTracker()

	d.Track("p1")
	d.Track("p2")
	d.Track("p3")
	d.ClearDirty("p2")

	dirty := d.DirtyPanes()
	require.Len(t, dirty, 2)
	require.Contains(t, dirty, "p1")
	require.Contains(t, dirty, "p3")
}

func TestDirtyTrackerStats(t *testing.T) {
	t.Parallel()
	d := NewDirtyTracker()

	d.Track("p1")
	d.Track("p2")
	d.Track("p3")
	d.ClearDirty("p1")

	stats := d.Stats()
	require.Equal(t, 3, stats.TotalPanes)
	require.Equal(t, 2, stats.DirtyPanes)
	require.Equal(t, 1, stats.CachedPanes)
}

func TestDirtyTrackerConcurrent(t *testing.T) {
	t.Parallel()
	d := NewDirtyTracker()

	// Concurrent writes shouldn't race.
	done := make(chan struct{})
	for i := range 20 {
		go func(id int) {
			pane := "p" + string(rune('0'+id%10))
			d.Track(pane)
			d.MarkDirty(pane)
			d.IsDirty(pane)
			d.ClearDirty(pane)
			d.SetCachedRender(pane, "x")
			d.GetCachedRender(pane)
			d.DirtyPanes()
			d.Stats()
			done <- struct{}{}
		}(i)
	}
	for range 20 {
		<-done
	}
}
