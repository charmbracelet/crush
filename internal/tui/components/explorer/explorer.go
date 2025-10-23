package explorer

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/explorer"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/lipgloss/v2"
)

// Model represents the file explorer component
type Model struct {
	tree      *explorer.Tree
	watcher   *explorer.Watcher
	width     int
	height    int
	scroll    int // Scroll position for navigating the tree
	selected  *explorer.Node
	eventChan chan explorer.Event
	config    *config.Config
}

// KeyMap defines keyboard bindings for the explorer
type KeyMap struct {
	Up           key.Binding
	Down         key.Binding
	Left         key.Binding
	Right        key.Binding
	Enter        key.Binding
	Backspace    key.Binding
	Home         key.Binding
	End          key.Binding
	PageUp       key.Binding
	PageDown     key.Binding
	ToggleHidden key.Binding
	Refresh      key.Binding
}

// DefaultKeyMap returns the default key bindings for the explorer
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:           key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:         key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Left:         key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "left")),
		Right:        key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "right")),
		Enter:        key.NewBinding(key.WithKeys("enter", " "), key.WithHelp("Enter", "select")),
		Backspace:    key.NewBinding(key.WithKeys("backspace", "delete"), key.WithHelp("Backspace", "back")),
		Home:         key.NewBinding(key.WithKeys("home", "g"), key.WithHelp("Home", "top")),
		End:          key.NewBinding(key.WithKeys("end", "G"), key.WithHelp("End", "bottom")),
		PageUp:       key.NewBinding(key.WithKeys("pgup", "u"), key.WithHelp("PgUp", "page up")),
		PageDown:     key.NewBinding(key.WithKeys("pgdown", "d"), key.WithHelp("PgDown", "page down")),
		ToggleHidden: key.NewBinding(key.WithKeys("ctrl+h", "H"), key.WithHelp("Ctrl+H", "toggle hidden")),
		Refresh:      key.NewBinding(key.WithKeys("F5", "r"), key.WithHelp("F5", "refresh")),
	}
}

// New creates a new file explorer model
func New(cfg *config.Config, rootPath string, width, height int) (*Model, error) {
	// Get show hidden preference from config
	showHidden := true // Default to true, can be made configurable

	// Create the file tree
	tree, err := explorer.NewTree(rootPath, showHidden, 10) // Max depth of 10
	if err != nil {
		return nil, fmt.Errorf("failed to create file tree: %w", err)
	}

	// Create the file system watcher
	watcher, err := explorer.NewWatcher(tree, 250*time.Millisecond) // 250ms debounce
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	model := &Model{
		tree:      tree,
		watcher:   watcher,
		width:     width,
		height:    height,
		scroll:    0,
		selected:  tree.Root,
		eventChan: make(chan explorer.Event, 100),
		config:    cfg,
	}

	// Start the watcher
	if err := watcher.Start(); err != nil {
		return nil, fmt.Errorf("failed to start file watcher: %w", err)
	}

	// Subscribe to file system events
	go model.handleFileEvents()

	return model, nil
}

// Init initializes the explorer model
func (m *Model) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the model state
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	case tea.MouseMsg:
		return m.handleMouseMsg(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case explorer.Event:
		return m, m.handleFileEvent(msg)
	}
	return m, nil
}

// View renders the file explorer
func (m *Model) View() string {
	theme := styles.CurrentTheme()

	// Get visible nodes
	visibleNodes := m.tree.GetVisibleNodes()

	// Calculate visible area
	visibleHeight := m.height - 2 // Leave space for header/footer

	// Determine which nodes to show based on scroll position
	startIdx := m.scroll
	endIdx := min(startIdx+visibleHeight, len(visibleNodes))
	if startIdx >= len(visibleNodes) {
		startIdx = max(0, len(visibleNodes)-visibleHeight)
		endIdx = len(visibleNodes)
	}

	if endIdx > len(visibleNodes) {
		endIdx = len(visibleNodes)
	}

	nodesToShow := visibleNodes[startIdx:endIdx]

	// Build the view
	var content strings.Builder

	// Header
	header := theme.S().Base.Padding(0, 1).Render("📁 Explorer")
	content.WriteString(header)
	content.WriteString("\n")

	// Render visible nodes
	for _, node := range nodesToShow {
		selected := m.tree.GetSelected()
		isSelected := selected != nil && node == selected
		line := m.renderNode(node, isSelected)
		content.WriteString(line)
	}

	// Footer with scroll info
	scrollInfo := fmt.Sprintf("%d/%d", startIdx+1, len(visibleNodes))
	footer := theme.S().Base.Padding(0, 1).AlignHorizontal(lipgloss.Right).Render(scrollInfo)
	content.WriteString("\n")
	content.WriteString(footer)

	// Apply border and style
	return theme.S().Base.
		Width(m.width).
		Height(m.height).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.BorderFocus).
		Render(content.String())
}

// renderNode renders a single tree node
func (m *Model) renderNode(node *explorer.Node, isSelected bool) string {
	theme := styles.CurrentTheme()

	// Build prefix (indentation and tree symbols)
	var prefix strings.Builder
	for i := 0; i < node.Level; i++ {
		prefix.WriteString("  ")
	}

	if node.IsDir {
		if node.Expanded {
			prefix.WriteString("📂 ")
		} else {
			prefix.WriteString("📁 ")
		}
	} else {
		prefix.WriteString("📄 ")
	}

	// Build the line
	var line strings.Builder
	line.WriteString(prefix.String())

	// Determine style based on selection and node type
	var style lipgloss.Style
	if isSelected {
		style = theme.S().Base.Background(theme.Primary).Foreground(theme.FgSelected).Bold(true)
	} else if node.IsHidden {
		style = theme.S().Base.Foreground(theme.FgHalfMuted)
	} else if node.IsDir {
		style = theme.S().Base.Foreground(theme.FgBase).Bold(true)
	} else {
		style = theme.S().Base.Foreground(theme.FgBase)
	}

	// Render the node name
	line.WriteString(style.Render(node.Name))

	return line.String()
}

// handleKeyMsg handles keyboard input
func (m *Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	keymap := DefaultKeyMap()

	switch {
	case key.Matches(msg, keymap.Up):
		m.moveUp()
	case key.Matches(msg, keymap.Down):
		m.moveDown()
	case key.Matches(msg, keymap.Left):
		m.collapseCurrent()
	case key.Matches(msg, keymap.Right):
		m.expandCurrent()
	case key.Matches(msg, keymap.Enter):
		m.toggleExpansion()
	case key.Matches(msg, keymap.Home):
		m.scrollToTop()
	case key.Matches(msg, keymap.End):
		m.scrollToBottom()
	case key.Matches(msg, keymap.PageUp):
		m.pageUp()
	case key.Matches(msg, keymap.PageDown):
		m.pageDown()
	case key.Matches(msg, keymap.ToggleHidden):
		m.toggleHiddenFiles()
	case key.Matches(msg, keymap.Refresh):
		m.refreshTree()
	}

	return m, nil
}

// handleMouseMsg handles mouse input
func (m *Model) handleMouseMsg(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// TODO: Implement mouse click handling for file selection
	return m, nil
}

// handleFileEvent handles file system events
func (m *Model) handleFileEvent(event explorer.Event) tea.Cmd {
	// TODO: Handle file system events (refresh tree, update selection, etc.)
	return nil
}

// handleFileEvents processes file system events from the watcher
func (m *Model) handleFileEvents() {
	for event := range m.eventChan {
		// Handle the event
		_ = m.handleFileEvent(event)
	}
}

// Navigation methods
func (m *Model) moveUp() {
	visibleNodes := m.tree.GetVisibleNodes()
	if len(visibleNodes) == 0 {
		return
	}

	currentSelected := m.tree.GetSelected()
	if currentSelected == nil {
		m.tree.SetSelected(visibleNodes[0])
		return
	}

	// Find current selection index
	currentIdx := -1
	for i, node := range visibleNodes {
		if node == currentSelected {
			currentIdx = i
			break
		}
	}

	// Move up
	if currentIdx > 0 {
		m.tree.SetSelected(visibleNodes[currentIdx-1])
		m.ensureVisible()
	}
}

func (m *Model) moveDown() {
	visibleNodes := m.tree.GetVisibleNodes()
	if len(visibleNodes) == 0 {
		return
	}

	currentSelected := m.tree.GetSelected()
	if currentSelected == nil {
		m.tree.SetSelected(visibleNodes[0])
		return
	}

	// Find current selection index
	currentIdx := -1
	for i, node := range visibleNodes {
		if node == currentSelected {
			currentIdx = i
			break
		}
	}

	// Move down
	if currentIdx < len(visibleNodes)-1 {
		m.tree.SetSelected(visibleNodes[currentIdx+1])
		m.ensureVisible()
	}
}

func (m *Model) expandCurrent() {
	selected := m.tree.GetSelected()
	if selected != nil && selected.IsDir {
		_ = m.tree.Expand(selected)
	}
}

func (m *Model) collapseCurrent() {
	selected := m.tree.GetSelected()
	if selected != nil && selected.IsDir {
		m.tree.Collapse(selected)
	}
}

func (m *Model) toggleExpansion() {
	selected := m.tree.GetSelected()
	if selected != nil && selected.IsDir {
		_ = m.tree.ToggleExpansion(selected)
	}
}

func (m *Model) scrollToTop() {
	m.scroll = 0
}

func (m *Model) scrollToBottom() {
	visibleNodes := m.tree.GetVisibleNodes()
	visibleHeight := m.height - 2
	m.scroll = max(0, len(visibleNodes)-visibleHeight)
}

func (m *Model) pageUp() {
	visibleHeight := m.height - 2
	m.scroll = max(0, m.scroll-visibleHeight)
}

func (m *Model) pageDown() {
	visibleHeight := m.height - 2
	visibleNodes := m.tree.GetVisibleNodes()
	m.scroll = min(m.scroll+visibleHeight, max(0, len(visibleNodes)-visibleHeight))
}

func (m *Model) toggleHiddenFiles() {
	showHidden := !m.tree.GetShowHidden()
	m.tree.SetShowHidden(showHidden)
}

func (m *Model) refreshTree() {
	_ = m.tree.Refresh()
}

func (m *Model) ensureVisible() {
	visibleNodes := m.tree.GetVisibleNodes()
	visibleHeight := m.height - 2
	selected := m.tree.GetSelected()

	if selected == nil {
		return
	}

	// Find selected node index
	selectedIdx := -1
	for i, node := range visibleNodes {
		if node == selected {
			selectedIdx = i
			break
		}
	}

	// Adjust scroll if selection is not visible
	if selectedIdx < m.scroll {
		m.scroll = selectedIdx
	} else if selectedIdx >= m.scroll+visibleHeight {
		m.scroll = max(0, selectedIdx-visibleHeight+1)
	}
}

// GetSelectedPath returns the path of the currently selected file/directory
func (m *Model) GetSelectedPath() string {
	selected := m.tree.GetSelected()
	if selected != nil {
		return selected.Path
	}
	return ""
}

// IsSelectedDirectory returns true if the current selection is a directory
func (m *Model) IsSelectedDirectory() bool {
	selected := m.tree.GetSelected()
	return selected != nil && selected.IsDir
}

// Cleanup cleans up the explorer resources
func (m *Model) Cleanup() {
	if m.watcher != nil {
		m.watcher.Stop()
	}
	if m.eventChan != nil {
		close(m.eventChan)
	}
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
