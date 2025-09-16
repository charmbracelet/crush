package playground

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/nom-nom-hub/blush/internal/tui/styles"
)

// outputModel represents the output display component
type outputModel struct {
	width   int
	height  int
	content []string
	scroll  int
}

// newOutputModel creates a new output model
func newOutputModel() outputModel {
	return outputModel{
		content: []string{},
		scroll:  0,
	}
}

// Init implements tea.Model
func (m outputModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m outputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up":
			if m.scroll > 0 {
				m.scroll--
			}
		case "down":
			if m.scroll < len(m.content)-1 {
				m.scroll++
			}
		case "pgup":
			m.scroll -= m.height
			if m.scroll < 0 {
				m.scroll = 0
			}
		case "pgdown":
			m.scroll += m.height
			maxScroll := len(m.content) - 1
			if m.scroll > maxScroll {
				m.scroll = maxScroll
			}
		}
	}

	return m, nil
}

// View implements tea.Model
func (m outputModel) View() string {
	t := styles.CurrentTheme()
	
	// Display the output content
	var displayLines []string
	
	// If we have content, display it
	if len(m.content) > 0 {
		// Determine which lines to display based on scroll position and height
		start := m.scroll
		end := start + m.height - 2 // Account for border
		if end > len(m.content) {
			end = len(m.content)
		}
		
		displayLines = m.content[start:end]
	} else {
		// Show placeholder text
		displayLines = []string{"No output yet. Run some code to see results here."}
	}
	
	// Join lines
	content := strings.Join(displayLines, "\n")
	
	// Apply border styling
	style := t.S().Base.
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Width(m.width).
		Height(m.height)
	
	return style.Render(content)
}

// SetSize sets the size of the output model
func (m *outputModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Focus gives focus to the output model
func (m *outputModel) Focus() {
	// For now, this is a no-op since we don't have focus-specific behavior
}

// Blur removes focus from the output model
func (m *outputModel) Blur() {
	// For now, this is a no-op since we don't have focus-specific behavior
}