package model

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
)

// InitKeyMap defines key bindings for the chat model.
type InitKeyMap struct{}

// DefaultInitKeyMap returns the default key bindings for the chat model.
func DefaultInitKeyMap() ChatKeyMap {
	return ChatKeyMap{}
}

// InitModel represents the chat UI model.
type InitModel struct {
	com *common.Common

	keyMap ChatKeyMap
}

// NewInitModel creates a new instance of ChatModel.
func NewInitModel(com *common.Common) *InitModel {
	return &InitModel{
		com:    com,
		keyMap: DefaultInitKeyMap(),
	}
}

// Init initializes the chat model.
func (m *InitModel) Init() tea.Cmd {
	return nil
}

// Update handles incoming messages and updates the chat model state.
func (m *InitModel) Update(msg tea.Msg) (*InitModel, tea.Cmd) {
	// Handle messages here
	return m, nil
}

// View renders the chat model's view.
func (m *InitModel) View() string {
	return "Init Model View"
}

// ShortHelp returns a brief help view for the chat model.
func (m *InitModel) ShortHelp() []key.Binding {
	return nil
}

// FullHelp returns a detailed help view for the chat model.
func (m *InitModel) FullHelp() [][]key.Binding {
	return nil
}
