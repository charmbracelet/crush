package common

import (
	tea "charm.land/bubbletea/v2"
)

// Model represents a common interface for UI components.
type Model[T any] interface {
	Update(msg tea.Msg) (T, tea.Cmd)
	View() string
}

// CoalescedWheelMsg is emitted by the input filter after collapsing many
// terminal wheel samples into one message. DeltaX and DeltaY carry the summed
// scroll magnitude so handlers can scroll proportionally.
type CoalescedWheelMsg struct {
	Mouse  tea.Mouse
	DeltaX float64
	DeltaY float64
}
