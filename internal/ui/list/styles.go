package list

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/exp/charmtone"
)

// Styles holds the styles for the List component.
type Styles struct {
	NormalItem  lipgloss.Style
	FocusedItem lipgloss.Style
}

// DefaultStyles returns the default styles for the List component.
func DefaultStyles() (s Styles) {
	s.NormalItem = lipgloss.NewStyle().
		MarginLeft(1).
		PaddingLeft(1)
	s.FocusedItem = lipgloss.NewStyle().
		Border(lipgloss.ThickBorder(), false, false, false, true).
		PaddingLeft(1).
		BorderForeground(charmtone.Guac)

	return s
}
