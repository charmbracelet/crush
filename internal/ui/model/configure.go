package model

import (
	"math/rand"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
)

// ConfigureKeyMap defines key bindings for the configure model.
type ConfigureKeyMap struct{}

// DefaultConfigureKeyMap returns the default key bindings for the configure model.
func DefaultConfigureKeyMap() ConfigureKeyMap {
	return ConfigureKeyMap{}
}

// ConfigureModel represents the configure UI model.
type ConfigureModel struct {
	width  int
	height int
	com    *common.Common

	keyMap ConfigureKeyMap
}

// NewConfigureModel creates a new instance of ConfigureModel.
func NewConfigureModel(com *common.Common) *ConfigureModel {
	return &ConfigureModel{
		com:    com,
		keyMap: DefaultConfigureKeyMap(),
	}
}

// Configure initializes the configure model.
func (m *ConfigureModel) Configure() tea.Cmd {
	return nil
}

// Update handles incoming messages and updates the configure model state.
func (m *ConfigureModel) Update(msg tea.Msg) (*ConfigureModel, tea.Cmd) {
	// Handle messages here
	return m, nil
}

// Configured returns true if the app has been configured.
func (m *ConfigureModel) Configured() bool {
	return false
}

// View renders the configure model's view.
func (m *ConfigureModel) View() string {
	return lipgloss.NewStyle().Width(m.width).
		Height(m.height).
		Background(lipgloss.ANSIColor(rand.Intn(256))).
		Render(" Configure ")
}

// ShortHelp returns a brief help view for the configure model.
func (m *ConfigureModel) ShortHelp() []key.Binding {
	return nil
}

// FullHelp returns a detailed help view for the configure model.
func (m *ConfigureModel) FullHelp() [][]key.Binding {
	return nil
}

// SetWidth sets the width of the configure model.
func (m *ConfigureModel) SetWidth(width int) {
	m.width = width
}

// SetHeight sets the height of the configure model.
func (m *ConfigureModel) SetHeight(height int) {
	m.height = height
}
