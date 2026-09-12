// Package tabmgr manages terminal tabs, each owning a split-pane tree
// and metadata (git branch, working directory, pane type).
package tabmgr

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/charmbracelet/crush/internal/split"
)

// PaneType identifies what kind of content a pane holds.
type PaneType int

const (
	PaneSession PaneType = iota // Chat session
	PaneShell                   // Raw terminal shell (bash/PowerShell)
)

func (p PaneType) String() string {
	switch p {
	case PaneSession:
		return "session"
	case PaneShell:
		return "shell"
	default:
		return "unknown"
	}
}

var tabIDCounter atomic.Int64

func nextTabID() string {
	n := tabIDCounter.Add(1)
	return fmt.Sprintf("tab-%d", n)
}

var paneIDCounter atomic.Int64

func nextPaneID() string {
	n := paneIDCounter.Add(1)
	return fmt.Sprintf("pane-%d", n)
}

// Tab represents a single tab in the multiplexer.
type Tab struct {
	// mu protects GitBranch and GitDirty which are written by the background
	// git watcher goroutine and read by the UI render goroutine.
	mu sync.RWMutex

	// ID uniquely identifies this tab.
	ID string

	// Name is the display name for this tab.
	Name string

	// CWD is the working directory for this tab.
	CWD string

	// GitBranch is the current git branch name (empty if not a git repo).
	// Protected by mu.
	GitBranch string

	// GitDirty is true if the working tree has uncommitted changes.
	// Protected by mu.
	GitDirty bool

	// SplitRoot is the root of this tab's pane split tree.
	SplitRoot *split.Node

	// FocusedPane is the ID of the currently focused pane within this tab.
	FocusedPane string

	// Panes maps pane IDs to their metadata.
	Panes map[string]*PaneMeta

	// cachedLeaves caches the depth-first pane order. Invalidated on split/close.
	cachedLeaves []string
}

// SetGitStatus updates the git status fields atomically.
func (t *Tab) SetGitStatus(branch string, dirty bool) {
	t.mu.Lock()
	t.GitBranch = branch
	t.GitDirty = dirty
	t.mu.Unlock()
}

// GetGitStatus reads the git status fields atomically.
func (t *Tab) GetGitStatus() (branch string, dirty bool) {
	t.mu.RLock()
	branch = t.GitBranch
	dirty = t.GitDirty
	t.mu.RUnlock()
	return
}

// PaneMeta holds metadata about a specific pane within a tab.
type PaneMeta struct {
	ID       string
	Type     PaneType
	Title    string // optional display title
	CWD      string // pane-specific working directory (may differ from tab CWD)

	// SessionID is the database session ID for chat panes (empty if no session).
	SessionID string
	// ModelProvider is the per-pane model provider override (empty = global).
	ModelProvider string
	// ModelID is the per-pane model ID override (empty = global).
	ModelID string
}

// NewTab creates a new tab with a single pane.
func NewTab(name, cwd string, paneType PaneType) *Tab {
	tabID := nextTabID()
	paneID := nextPaneID()

	tab := &Tab{
		ID:          tabID,
		Name:        name,
		CWD:         cwd,
		SplitRoot:   split.NewLeaf(paneID),
		FocusedPane: paneID,
		Panes: map[string]*PaneMeta{
			paneID: {
				ID:   paneID,
				Type: paneType,
				CWD:  cwd,
			},
		},
	}
	return tab
}

// leaves returns the cached leaf order, recomputing if invalidated.
func (t *Tab) leaves() []string {
	if t.cachedLeaves == nil {
		t.cachedLeaves = split.AllLeaves(t.SplitRoot)
	}
	return t.cachedLeaves
}

// invalidateLeafCache clears the cached leaf order.
func (t *Tab) invalidateLeafCache() {
	t.cachedLeaves = nil
}

// SplitFocused splits the currently focused pane in the given direction,
// creating a new pane of the specified type.
func (t *Tab) SplitFocused(dir split.Direction, paneType PaneType) (string, error) {
	if t.FocusedPane == "" {
		return "", fmt.Errorf("no focused pane")
	}
	return t.SplitPane(t.FocusedPane, dir, paneType)
}

// SplitPane splits the pane with the given ID, creating a new pane.
func (t *Tab) SplitPane(paneID string, dir split.Direction, paneType PaneType) (string, error) {
	newPaneID := nextPaneID()
	// SplitLeaf returns an error if paneID is not found — no pre-check needed.
	if err := split.SplitLeaf(t.SplitRoot, paneID, dir, newPaneID); err != nil {
		return "", err
	}
	t.invalidateLeafCache()

	cwd := t.CWD
	if existing, ok := t.Panes[paneID]; ok {
		cwd = existing.CWD
	}

	t.Panes[newPaneID] = &PaneMeta{
		ID:   newPaneID,
		Type: paneType,
		CWD:  cwd,
	}

	return newPaneID, nil
}

// ClosePane removes a pane from the tab. If it was the focused pane,
// focus moves to the first remaining pane. Returns an error if it's the
// only pane (use CloseTab instead).
func (t *Tab) ClosePane(paneID string) error {
	if err := split.RemoveLeaf(t.SplitRoot, paneID); err != nil {
		return err
	}
	delete(t.Panes, paneID)
	t.invalidateLeafCache()

	if t.FocusedPane == paneID {
		leaves := split.AllLeaves(t.SplitRoot)
		if len(leaves) > 0 {
			t.FocusedPane = leaves[0]
		} else {
			t.FocusedPane = ""
		}
	}
	return nil
}

// PaneCount returns the number of panes in this tab.
func (t *Tab) PaneCount() int {
	return split.LeafCount(t.SplitRoot)
}

// Layout computes pane rectangles for the given dimensions.
func (t *Tab) Layout(width, height int) []split.PaneLayout {
	return split.LayoutWithDividers(t.SplitRoot, width, height)
}

// Dividers computes divider positions for the given dimensions.
func (t *Tab) Dividers(width, height int) []split.Rect {
	return split.Dividers(t.SplitRoot, width, height)
}

// FocusNext moves focus to the next pane in depth-first order (wraps around).
func (t *Tab) FocusNext() string {
	return t.focusCycle(1)
}

// FocusPrev moves focus to the previous pane in depth-first order (wraps around).
func (t *Tab) FocusPrev() string {
	return t.focusCycle(-1)
}

func (t *Tab) focusCycle(delta int) string {
	leaves := t.leaves()
	if len(leaves) == 0 {
		return ""
	}

	current := -1
	for i, id := range leaves {
		if id == t.FocusedPane {
			current = i
			break
		}
	}

	next := (current + delta + len(leaves)) % len(leaves)
	t.FocusedPane = leaves[next]
	return t.FocusedPane
}
