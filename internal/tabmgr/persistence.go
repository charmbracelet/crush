package tabmgr

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/split"
)

const (
	sessionFileName    = "sessions.json"
	saveDebounceDelay  = 2 * time.Second
)

// SessionLayout is the serializable representation of a tab layout.
type SessionLayout struct {
	Version   int              `json:"version"`
	ActiveTab int              `json:"active_tab"`
	Tabs      []SessionTab     `json:"tabs"`
	SavedAt   string           `json:"saved_at"`
}

// SessionTab is the serializable representation of a single tab.
type SessionTab struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	CWD       string           `json:"cwd"`
	SplitTree *SessionNode     `json:"split_tree"`
}

// SessionNode is the serializable representation of a split tree node.
type SessionNode struct {
	// Leaf fields (non-nil if this is a pane).
	PaneID   string `json:"pane_id,omitempty"`
	PaneType string `json:"pane_type,omitempty"` // "session" or "shell"
	PaneCWD  string `json:"pane_cwd,omitempty"`  // pane working directory

	// Per-pane session and model state (leaf nodes only).
	SessionID     string `json:"session_id,omitempty"`
	ModelProvider string `json:"model_provider,omitempty"`
	ModelID       string `json:"model_id,omitempty"`

	// Split fields (non-nil if this is a split).
	Dir   string       `json:"dir,omitempty"` // "H" or "V"
	Ratio float64      `json:"ratio,omitempty"`
	A     *SessionNode `json:"a,omitempty"`
	B     *SessionNode `json:"b,omitempty"`
}

// Persistence handles saving and loading tab layouts.
type Persistence struct {
	dataDir string
	mu      sync.Mutex
	timer   *time.Timer
}

// NewPersistence creates a new persistence handler for the given data directory.
func NewPersistence(dataDir string) *Persistence {
	return &Persistence{dataDir: dataDir}
}

// SaveLayout saves the current tab manager state to disk.
func (p *Persistence) SaveLayout(mgr *TabManager) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	layout := SessionLayout{
		Version:   1,
		ActiveTab: mgr.ActiveIndex(),
		SavedAt:   time.Now().UTC().Format(time.RFC3339),
	}

	for _, tab := range mgr.Tabs() {
		st := SessionTab{
			ID:   tab.ID,
			Name: tab.Name,
			CWD:  tab.CWD,
		}
		st.SplitTree = serializeNode(tab.SplitRoot, tab.Panes)
		layout.Tabs = append(layout.Tabs, st)
	}

	data, err := json.MarshalIndent(layout, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session layout: %w", err)
	}

	path := filepath.Join(p.dataDir, sessionFileName)
	if err := os.MkdirAll(p.dataDir, 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write session file: %w", err)
	}

	return nil
}

// SaveLayoutDebounced saves the layout with a debounce delay.
// Multiple rapid calls within the delay window result in a single save.
func (p *Persistence) SaveLayoutDebounced(mgr *TabManager) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.timer != nil {
		p.timer.Stop()
	}
	p.timer = time.AfterFunc(saveDebounceDelay, func() {
		if err := p.SaveLayout(mgr); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save session layout: %v\n", err)
		}
	})
}

// LoadLayout loads a previously saved tab layout and reconstructs tabs.
// Returns nil and no error if no saved layout exists.
func (p *Persistence) LoadLayout() (*SessionLayout, error) {
	path := filepath.Join(p.dataDir, sessionFileName)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read session file: %w", err)
	}

	var layout SessionLayout
	if err := json.Unmarshal(data, &layout); err != nil {
		return nil, fmt.Errorf("unmarshal session layout: %w", err)
	}

	if layout.Version != 1 {
		return nil, fmt.Errorf("unsupported session layout version: %d", layout.Version)
	}

	return &layout, nil
}

// RestoreTabs reconstructs a TabManager from a saved layout.
func (p *Persistence) RestoreTabs(layout *SessionLayout) *TabManager {
	mgr := New()

	for _, st := range layout.Tabs {
		// Create the tab with a default pane first.
		tab := mgr.AddTab(st.Name, st.CWD, PaneSession)

		// If we have a saved split tree, reconstruct it.
		if st.SplitTree != nil {
			root, panes := deserializeNode(st.SplitTree)
			if root != nil {
				tab.SplitRoot = root
				tab.Panes = panes
				// Set focus to the first pane.
				leaves := split.AllLeaves(root)
				if len(leaves) > 0 {
					tab.FocusedPane = leaves[0]
				}
			}
		}
	}

	// Restore active tab.
	if layout.ActiveTab >= 0 && layout.ActiveTab < mgr.Len() {
		mgr.SelectTab(layout.ActiveTab)
	}

	return mgr
}

func serializeNode(node *split.Node, panes map[string]*PaneMeta) *SessionNode {
	if node == nil {
		return nil
	}

	if node.IsLeaf() {
		sn := &SessionNode{
			PaneID:   node.Leaf.PaneID,
			PaneType: "session",
		}
		if meta, ok := panes[node.Leaf.PaneID]; ok {
			sn.PaneType = meta.Type.String()
			sn.PaneCWD = meta.CWD
			sn.SessionID = meta.SessionID
			sn.ModelProvider = meta.ModelProvider
			sn.ModelID = meta.ModelID
		}
		return sn
	}

	if node.IsSplit() {
		return &SessionNode{
			Dir:   node.Split.Dir.String(),
			Ratio: node.Split.Ratio,
			A:     serializeNode(node.Split.A, panes),
			B:     serializeNode(node.Split.B, panes),
		}
	}

	return nil
}

func deserializeNode(sn *SessionNode) (*split.Node, map[string]*PaneMeta) {
	panes := make(map[string]*PaneMeta)
	node := deserializeNodeRecursive(sn, panes)
	return node, panes
}

func deserializeNodeRecursive(sn *SessionNode, panes map[string]*PaneMeta) *split.Node {
	if sn == nil {
		return nil
	}

	// Leaf node.
	if sn.PaneID != "" {
		paneType := PaneSession
		if sn.PaneType == "shell" {
			paneType = PaneShell
		}
		panes[sn.PaneID] = &PaneMeta{
			ID:            sn.PaneID,
			Type:          paneType,
			CWD:           sn.PaneCWD,
			SessionID:     sn.SessionID,
			ModelProvider: sn.ModelProvider,
			ModelID:       sn.ModelID,
		}
		return split.NewLeaf(sn.PaneID)
	}

	// Split node.
	if sn.A != nil && sn.B != nil {
		dir := split.Horizontal
		if sn.Dir == "V" {
			dir = split.Vertical
		}
		a := deserializeNodeRecursive(sn.A, panes)
		b := deserializeNodeRecursive(sn.B, panes)
		return split.NewSplit(dir, sn.Ratio, a, b)
	}

	return nil
}
