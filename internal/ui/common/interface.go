package common

import (
	tea "charm.land/bubbletea/v2"
)

// Model represents a common interface for UI components.
type Model[T any] interface {
	Update(msg tea.Msg) (T, tea.Cmd)
	View() string
}
