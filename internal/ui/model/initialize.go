package model

import (
	"math/rand"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
)

// InitKeyMap defines key bindings for the init model.
type InitKeyMap struct{}

// DefaultInitKeyMap returns the default key bindings for the init model.
func DefaultInitKeyMap() InitKeyMap {
	return InitKeyMap{}
}

// InitializeModel represents the init UI model.
type InitializeModel struct {
	width, height int
	com           *common.Common

	keyMap InitKeyMap
}

// NewInitModel creates a new instance of InitModel.
func NewInitModel(com *common.Common) *InitializeModel {
	return &InitializeModel{
		com:    com,
		keyMap: DefaultInitKeyMap(),
	}
}

// Init initializes the init model.
func (m *InitializeModel) Init() tea.Cmd {
	return nil
}

// Update handles incoming messages and updates the init model state.
func (m *InitializeModel) Update(msg tea.Msg) (*InitializeModel, tea.Cmd) {
	// Handle messages here
	return m, nil
}

// View renders the init model's view.
func (m *InitializeModel) View() string {
	return lipgloss.NewStyle().Width(m.width).
		Height(m.height).
		Background(lipgloss.ANSIColor(rand.Intn(256))).
		Render(" Configure ")
}

// ShortHelp returns a brief help view for the init model.
func (m *InitializeModel) ShortHelp() []key.Binding {
	return nil
}

// FullHelp returns a detailed help view for the init model.
func (m *InitializeModel) FullHelp() [][]key.Binding {
	return nil
}

// SetWidth sets the width of the init model.
func (m *InitializeModel) SetWidth(width int) {
	m.width = width
}

// SetHeight sets the height of the init model.
func (m *InitializeModel) SetHeight(height int) {
	m.height = height
}
