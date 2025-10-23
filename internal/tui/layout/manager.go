package layout

import (
	"strings"

	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/lipgloss/v2"
)

// Pane represents a UI pane in the layout
type Pane int

const (
	LeftPane Pane = iota
	CenterPane
	RightPane
	BottomPane
)

// LayoutConfig represents the layout configuration
type LayoutConfig struct {
	LeftWidth    int
	RightWidth   int
	BottomHeight int
	MinPaneSize  int
	ShowLeft     bool
	ShowRight    bool
	ShowBottom   bool
	FocusedPane  Pane
}

// Model represents the layout manager
type Model struct {
	width     int
	height    int
	config    LayoutConfig
	panes     map[Pane]tea.Model
	keyMap    KeyMap
	focusPane Pane
}

// KeyMap defines keyboard bindings for layout management
type KeyMap struct {
	SwitchPane   key.Binding
	ResizeLeft   key.Binding
	ResizeRight  key.Binding
	ResizeUp     key.Binding
	ResizeDown   key.Binding
	ToggleLeft   key.Binding
	ToggleRight  key.Binding
	ToggleBottom key.Binding
}

// DefaultKeyMap returns the default key bindings for layout management
func DefaultKeyMap() KeyMap {
	return KeyMap{
		SwitchPane:   key.NewBinding(key.WithKeys("tab"), key.WithHelp("Tab", "Switch pane")),
		ResizeLeft:   key.NewBinding(key.WithKeys("ctrl+left", "ctrl+h"), key.WithHelp("Ctrl+←", "Resize left")),
		ResizeRight:  key.NewBinding(key.WithKeys("ctrl+right", "ctrl+l"), key.WithHelp("Ctrl+→", "Resize right")),
		ResizeUp:     key.NewBinding(key.WithKeys("ctrl+up", "ctrl+k"), key.WithHelp("Ctrl+↑", "Resize up")),
		ResizeDown:   key.NewBinding(key.WithKeys("ctrl+down", "ctrl+j"), key.WithHelp("Ctrl+↓", "Resize down")),
		ToggleLeft:   key.NewBinding(key.WithKeys("ctrl+1"), key.WithHelp("Ctrl+1", "Toggle left pane")),
		ToggleRight:  key.NewBinding(key.WithKeys("ctrl+2"), key.WithHelp("Ctrl+2", "Toggle right pane")),
		ToggleBottom: key.NewBinding(key.WithKeys("ctrl+3"), key.WithHelp("Ctrl+3", "Toggle bottom pane")),
	}
}

// New creates a new layout manager
func New(width, height int) *Model {
	config := LayoutConfig{
		LeftWidth:    30, // 30% of width
		RightWidth:   30, // 30% of width
		BottomHeight: 20, // 20% of height
		MinPaneSize:  10, // Minimum size for panes
		ShowLeft:     true,
		ShowRight:    false,
		ShowBottom:   true,
		FocusedPane:  CenterPane,
	}

	return &Model{
		width:     width,
		height:    height,
		config:    config,
		panes:     make(map[Pane]tea.Model),
		keyMap:    DefaultKeyMap(),
		focusPane: CenterPane,
	}
}

// Init initializes the layout manager
func (m *Model) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the layout state
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	}
	return m, nil
}

// View renders the complete layout
func (m *Model) View() string {
	theme := styles.CurrentTheme()

	// Calculate pane dimensions
	leftWidth := m.width * m.config.LeftWidth / 100
	rightWidth := m.width * m.config.RightWidth / 100
	bottomHeight := m.height * m.config.BottomHeight / 100
	centerWidth := m.width - leftWidth - rightWidth
	centerHeight := m.height - bottomHeight

	// Adjust for hidden panes
	if !m.config.ShowLeft {
		centerWidth += leftWidth
		leftWidth = 0
	}
	if !m.config.ShowRight {
		centerWidth += rightWidth
		rightWidth = 0
	}
	if !m.config.ShowBottom {
		centerHeight += bottomHeight
		bottomHeight = 0
	}

	// Build the layout
	var layout strings.Builder

	// Top section (left + center + right)
	var topSection strings.Builder
	if m.config.ShowLeft {
		leftPane := m.renderPane(LeftPane, leftWidth, centerHeight)
		topSection.WriteString(leftPane)
	}
	centerPane := m.renderPane(CenterPane, centerWidth, centerHeight)
	topSection.WriteString(centerPane)
	if m.config.ShowRight {
		rightPane := m.renderPane(RightPane, rightWidth, centerHeight)
		topSection.WriteString(rightPane)
	}

	layout.WriteString(topSection.String())

	// Bottom section
	if m.config.ShowBottom {
		bottomPane := m.renderPane(BottomPane, m.width, bottomHeight)
		layout.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, bottomPane))
	}

	// Apply border and style
	return theme.S().Base.
		Width(m.width).
		Height(m.height).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.BorderFocus).
		Render(layout.String())
}

// renderPane renders a single pane with border and focus
func (m *Model) renderPane(pane Pane, width, height int) string {
	theme := styles.CurrentTheme()

	// Get the pane model
	paneModel, exists := m.panes[pane]
	if !exists {
		// Create a placeholder pane
		paneModel = &placeholderModel{width: width, height: height, name: m.getPaneName(pane)}
	}

	// Determine border style based on focus
	var borderStyle lipgloss.Style
	if m.focusPane == pane {
		borderStyle = theme.S().Base.Border(lipgloss.NormalBorder()).BorderForeground(theme.Primary)
	} else {
		borderStyle = theme.S().Base.Border(lipgloss.NormalBorder()).BorderForeground(theme.Border)
	}

	// Render the pane content
	var paneContent string
	if viewableModel, ok := paneModel.(interface{ View() string }); ok {
		paneContent = viewableModel.View()
	}
	if sizeableModel, ok := paneModel.(sizeableModel); ok {
		sizeableModel.SetSize(width, height)
		if viewableModel, ok := sizeableModel.(interface{ View() string }); ok {
			paneContent = viewableModel.View()
		}
	}

	// Apply border and return
	return borderStyle.
		Width(width).
		Height(height).
		Render(paneContent)
}

// getPaneName returns the display name for a pane
func (m *Model) getPaneName(pane Pane) string {
	switch pane {
	case LeftPane:
		return "Explorer"
	case CenterPane:
		return "Content"
	case RightPane:
		return "Preview"
	case BottomPane:
		return "Status"
	default:
		return "Unknown"
	}
}

// handleKeyMsg handles keyboard input for layout management
func (m *Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch {
	case key.Matches(msg, m.keyMap.SwitchPane):
		m.switchFocus()
	case key.Matches(msg, m.keyMap.ToggleLeft):
		m.togglePane(LeftPane)
	case key.Matches(msg, m.keyMap.ToggleRight):
		m.togglePane(RightPane)
	case key.Matches(msg, m.keyMap.ToggleBottom):
		m.togglePane(BottomPane)
	case key.Matches(msg, m.keyMap.ResizeLeft):
		cmd = m.resizePane(LeftPane, -5, 0) // Decrease by 5%
	case key.Matches(msg, m.keyMap.ResizeRight):
		cmd = m.resizePane(RightPane, 5, 0) // Increase by 5%
	case key.Matches(msg, m.keyMap.ResizeUp):
		cmd = m.resizePane(m.focusPane, 0, 5) // Increase height by 5%
	case key.Matches(msg, m.keyMap.ResizeDown):
		cmd = m.resizePane(m.focusPane, 0, -5) // Decrease height by 5%
	}
	return m, cmd
}

// switchFocus switches focus to the next pane
func (m *Model) switchFocus() {
	allPanes := []Pane{LeftPane, CenterPane, RightPane, BottomPane}

	// Filter to only visible panes
	visiblePanes := []Pane{}
	for _, pane := range allPanes {
		if m.isPaneVisible(pane) {
			visiblePanes = append(visiblePanes, pane)
		}
	}

	if len(visiblePanes) == 0 {
		return
	}

	// Find current focus index
	currentIdx := -1
	for i, pane := range visiblePanes {
		if pane == m.focusPane {
			currentIdx = i
			break
		}
	}

	// Switch to next pane
	nextIdx := (currentIdx + 1) % len(visiblePanes)
	m.focusPane = visiblePanes[nextIdx]
}

// togglePane toggles the visibility of a pane
func (m *Model) togglePane(pane Pane) {
	switch pane {
	case LeftPane:
		m.config.ShowLeft = !m.config.ShowLeft
	case RightPane:
		m.config.ShowRight = !m.config.ShowRight
	case BottomPane:
		m.config.ShowBottom = !m.config.ShowBottom
	}
}

// resizePane resizes a pane by the given percentage
func (m *Model) resizePane(pane Pane, widthDelta, heightDelta int) tea.Cmd {
	return func() tea.Msg {
		return ResizePaneMsg{
			Pane:        pane,
			WidthDelta:  widthDelta,
			HeightDelta: heightDelta,
		}
	}
}

// isPaneVisible returns true if a pane is currently visible
func (m *Model) isPaneVisible(pane Pane) bool {
	switch pane {
	case LeftPane:
		return m.config.ShowLeft
	case RightPane:
		return m.config.ShowRight
	case BottomPane:
		return m.config.ShowBottom
	case CenterPane:
		return true // Center pane is always visible
	default:
		return false
	}
}

// SetPane sets a pane model
func (m *Model) SetPane(pane Pane, model tea.Model) {
	m.panes[pane] = model
}

// GetPane returns a pane model
func (m *Model) GetPane(pane Pane) tea.Model {
	return m.panes[pane]
}

// GetFocusedPane returns the currently focused pane
func (m *Model) GetFocusedPane() Pane {
	return m.focusPane
}

// SetFocusedPane sets the focused pane
func (m *Model) SetFocusedPane(pane Pane) {
	m.focusPane = pane
}

// GetConfig returns the current layout configuration
func (m *Model) GetConfig() LayoutConfig {
	return m.config
}

// SetConfig updates the layout configuration
func (m *Model) SetConfig(config LayoutConfig) {
	m.config = config
}

// ResizePaneMsg represents a pane resize request
type ResizePaneMsg struct {
	Pane        Pane
	WidthDelta  int
	HeightDelta int
}

// placeholderModel represents a placeholder pane
type placeholderModel struct {
	width  int
	height int
	name   string
}

func (m *placeholderModel) Init() tea.Cmd {
	return nil
}

func (m *placeholderModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m *placeholderModel) View() string {
	theme := styles.CurrentTheme()

	content := theme.S().Base.
		Width(m.width).
		Height(m.height).
		AlignHorizontal(lipgloss.Center).
		AlignVertical(lipgloss.Center).
		Foreground(theme.FgHalfMuted).
		Render(m.name)

	return content
}

func (m *placeholderModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// sizeableModel interface for panes that can be resized
type sizeableModel interface {
	tea.Model
	SetSize(width, height int)
}
