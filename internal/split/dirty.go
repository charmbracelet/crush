package split

import "sync"

// DirtyTracker tracks which panes have changed since the last render.
// This enables selective re-rendering — only dirty panes need to be redrawn.
type DirtyTracker struct {
	mu      sync.Mutex
	dirty   map[string]bool
	renders map[string]string // cached render output per pane
}

// NewDirtyTracker creates a new dirty tracker.
func NewDirtyTracker() *DirtyTracker {
	return &DirtyTracker{
		dirty:   make(map[string]bool),
		renders: make(map[string]string),
	}
}

// MarkDirty marks a pane as needing re-render.
func (d *DirtyTracker) MarkDirty(paneID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.dirty[paneID] = true
}

// MarkAllDirty marks all tracked panes as dirty (e.g., on resize).
func (d *DirtyTracker) MarkAllDirty() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for id := range d.dirty {
		d.dirty[id] = true
	}
}

// IsDirty returns true if the pane needs re-rendering.
func (d *DirtyTracker) IsDirty(paneID string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.dirty[paneID]
}

// ClearDirty marks a pane as clean after rendering.
func (d *DirtyTracker) ClearDirty(paneID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.dirty[paneID] = false
}

// DirtyPanes returns all pane IDs that need re-rendering.
func (d *DirtyTracker) DirtyPanes() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	var result []string
	for id, isDirty := range d.dirty {
		if isDirty {
			result = append(result, id)
		}
	}
	return result
}

// DirtyCount returns the number of dirty panes.
func (d *DirtyTracker) DirtyCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	count := 0
	for _, isDirty := range d.dirty {
		if isDirty {
			count++
		}
	}
	return count
}

// SetCachedRender stores the rendered output for a pane.
func (d *DirtyTracker) SetCachedRender(paneID, rendered string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.renders[paneID] = rendered
	d.dirty[paneID] = false
}

// GetCachedRender returns the cached render for a pane, or empty string if none.
func (d *DirtyTracker) GetCachedRender(paneID string) (string, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	r, ok := d.renders[paneID]
	return r, ok
}

// Remove removes a pane from tracking.
func (d *DirtyTracker) Remove(paneID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.dirty, paneID)
	delete(d.renders, paneID)
}

// Track begins tracking a new pane (starts dirty).
func (d *DirtyTracker) Track(paneID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.dirty[paneID] = true
}

// RenderStats returns rendering performance stats.
type RenderStats struct {
	TotalPanes  int
	DirtyPanes  int
	CachedPanes int
}

// Stats returns current rendering stats.
func (d *DirtyTracker) Stats() RenderStats {
	d.mu.Lock()
	defer d.mu.Unlock()

	stats := RenderStats{
		TotalPanes: len(d.dirty),
	}
	for _, isDirty := range d.dirty {
		if isDirty {
			stats.DirtyPanes++
		} else {
			stats.CachedPanes++
		}
	}
	return stats
}
