package login

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/exp/charmtone"
)

var (
	// trailingSpace is an invisible character that prevents the Bubble Tea
	// renderer from stripping trailing blank lines. The renderer erases empty
	// lines after the last non-empty content, so a zero-width space keeps
	// the line alive without being visible.
	trailingSpace = "\u200b"

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(charmtone.Charple)
	slashStyle = lipgloss.NewStyle().
			Bold(true)
	errorStyle = lipgloss.NewStyle().
			Foreground(charmtone.Coral)
)
