package explorer

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Node represents a file or directory in the explorer tree
type Node struct {
	Name     string  // File/directory name
	Path     string  // Full path
	IsDir    bool    // Is this a directory?
	IsHidden bool    // Is this a hidden file?
	Children []*Node // Child nodes (for directories)
	Expanded bool    // Is this directory expanded?
	Level    int     // Depth in tree
	Parent   *Node   // Parent node
	Size     int64   // File size in bytes
	ModTime  string  // Modification time string
}

// Tree represents the file explorer tree structure
type Tree struct {
	Root        *Node  // Root directory node
	Selected    *Node  // Currently selected node
	ShowHidden  bool   // Show hidden files
	MaxDepth    int    // Maximum depth to explore
	CurrentPath string // Current working directory
}

// NewTree creates a new file explorer tree
func NewTree(rootPath string, showHidden bool, maxDepth int) (*Tree, error) {
	// Clean and validate root path
	rootPath = filepath.Clean(rootPath)

	// Check if root exists
	info, err := os.Stat(rootPath)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		return nil, os.ErrNotExist
	}

	// Create root node
	root := &Node{
		Name:     filepath.Base(rootPath),
		Path:     rootPath,
		IsDir:    true,
		IsHidden: strings.HasPrefix(filepath.Base(rootPath), "."),
		Level:    0,
		Children: []*Node{},
		Expanded: true, // Root is always expanded
	}

	tree := &Tree{
		Root:        root,
		ShowHidden:  showHidden,
		MaxDepth:    maxDepth,
		CurrentPath: rootPath,
	}

	// Build initial tree structure
	if err := tree.buildChildren(root, 1); err != nil {
		return nil, err
	}

	return tree, nil
}

// buildChildren recursively builds the tree structure
func (t *Tree) buildChildren(parent *Node, level int) error {
	if level > t.MaxDepth {
		return nil
	}

	entries, err := os.ReadDir(parent.Path)
	if err != nil {
		return err
	}

	// Sort entries: directories first, then files, both alphabetically
	sort.Slice(entries, func(i, j int) bool {
		iDir := entries[i].IsDir()
		jDir := entries[j].IsDir()

		if iDir != jDir {
			return iDir // directories come first
		}

		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})

	for _, entry := range entries {
		// Skip hidden files unless explicitly shown
		if !t.ShowHidden && strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		fullPath := filepath.Join(parent.Path, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue // Skip files we can't stat
		}

		node := &Node{
			Name:     entry.Name(),
			Path:     fullPath,
			IsDir:    entry.IsDir(),
			IsHidden: strings.HasPrefix(entry.Name(), "."),
			Level:    level,
			Parent:   parent,
			Size:     info.Size(),
			ModTime:  info.ModTime().Format("2006-01-02 15:04"),
			Expanded: false,
		}

		// If it's a directory, build its children
		if node.IsDir {
			if err := t.buildChildren(node, level+1); err != nil {
				// Continue even if we can't read subdirectory
				continue
			}
		}

		parent.Children = append(parent.Children, node)
	}

	return nil
}

// Expand expands a directory node to show its children
func (t *Tree) Expand(node *Node) error {
	if !node.IsDir {
		return nil
	}

	if node.Expanded {
		return nil // Already expanded
	}

	node.Expanded = true

	// If children haven't been loaded yet, load them
	if len(node.Children) == 0 {
		return t.buildChildren(node, node.Level+1)
	}

	return nil
}

// Collapse collapses a directory node to hide its children
func (t *Tree) Collapse(node *Node) {
	if !node.IsDir {
		return
	}

	node.Expanded = false
}

// ToggleExpansion toggles a directory's expanded state
func (t *Tree) ToggleExpansion(node *Node) error {
	if !node.IsDir {
		return nil
	}

	if node.Expanded {
		t.Collapse(node)
	} else {
		return t.Expand(node)
	}

	return nil
}

// GetVisibleNodes returns all visible nodes in tree order
func (t *Tree) GetVisibleNodes() []*Node {
	return t.getVisibleNodes(t.Root, true)
}

// getVisibleNodes recursively collects visible nodes
func (t *Tree) getVisibleNodes(node *Node, includeSelf bool) []*Node {
	var nodes []*Node

	if includeSelf {
		nodes = append(nodes, node)
	}

	// If directory is not expanded, don't include children
	if node.IsDir && !node.Expanded {
		return nodes
	}

	// Add children
	for _, child := range node.Children {
		nodes = append(nodes, t.getVisibleNodes(child, true)...)
	}

	return nodes
}

// GetNodeByPath finds a node by its full path
func (t *Tree) GetNodeByPath(path string) *Node {
	return t.getNodeByPath(t.Root, path)
}

// getNodeByPath recursively searches for a node by path
func (t *Tree) getNodeByPath(node *Node, path string) *Node {
	if node.Path == path {
		return node
	}

	if !node.IsDir || !node.Expanded {
		return nil
	}

	for _, child := range node.Children {
		if found := t.getNodeByPath(child, path); found != nil {
			return found
		}
	}

	return nil
}

// SetSelected sets the currently selected node
func (t *Tree) SetSelected(node *Node) {
	t.Selected = node
}

// GetSelected returns the currently selected node
func (t *Tree) GetSelected() *Node {
	return t.Selected
}

// Refresh reloads the tree structure from the filesystem
func (t *Tree) Refresh() error {
	// Rebuild the entire tree
	newRoot := &Node{
		Name:     t.Root.Name,
		Path:     t.Root.Path,
		IsDir:    true,
		IsHidden: t.Root.IsHidden,
		Level:    0,
		Children: []*Node{},
		Expanded: true,
		Parent:   nil,
	}

	// Preserve expanded state for existing directories
	if err := t.buildChildrenWithState(newRoot, 1, t.Root); err != nil {
		return err
	}

	t.Root = newRoot

	// Re-select the node if it still exists
	if t.Selected != nil {
		if found := t.GetNodeByPath(t.Selected.Path); found != nil {
			t.Selected = found
		} else {
			t.Selected = t.Root
		}
	}

	return nil
}

// buildChildrenWithState builds children while preserving expansion state
func (t *Tree) buildChildrenWithState(parent *Node, level int, oldNode *Node) error {
	if level > t.MaxDepth {
		return nil
	}

	entries, err := os.ReadDir(parent.Path)
	if err != nil {
		return err
	}

	// Create a map of old children for quick lookup
	oldChildren := make(map[string]*Node)
	for _, child := range oldNode.Children {
		oldChildren[child.Name] = child
	}

	// Sort entries
	sort.Slice(entries, func(i, j int) bool {
		iDir := entries[i].IsDir()
		jDir := entries[j].IsDir()

		if iDir != jDir {
			return iDir
		}

		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})

	for _, entry := range entries {
		// Skip hidden files unless explicitly shown
		if !t.ShowHidden && strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		fullPath := filepath.Join(parent.Path, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		node := &Node{
			Name:     entry.Name(),
			Path:     fullPath,
			IsDir:    entry.IsDir(),
			IsHidden: strings.HasPrefix(entry.Name(), "."),
			Level:    level,
			Parent:   parent,
			Size:     info.Size(),
			ModTime:  info.ModTime().Format("2006-01-02 15:04"),
			Expanded: false,
		}

		// Preserve expansion state for directories
		if oldChild, exists := oldChildren[entry.Name()]; exists && oldChild.IsDir {
			node.Expanded = oldChild.Expanded
		}

		// If it's a directory and expanded, build its children
		if node.IsDir && node.Expanded {
			if err := t.buildChildrenWithState(node, level+1, oldChildren[entry.Name()]); err != nil {
				continue
			}
		}

		parent.Children = append(parent.Children, node)
	}

	return nil
}

// SetShowHidden toggles whether to show hidden files
func (t *Tree) SetShowHidden(show bool) {
	t.ShowHidden = show
	t.Refresh() // Refresh to apply new setting
}

// GetShowHidden returns whether hidden files are shown
func (t *Tree) GetShowHidden() bool {
	return t.ShowHidden
}
