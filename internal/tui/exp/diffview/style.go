package diffview

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/exp/charmtone"
)

// LineStyle defines the styles for a given line type in the diff view.
type LineStyle struct {
	LineNumber lipgloss.Style
	Symbol     lipgloss.Style
	Code       lipgloss.Style
}

// Style defines the overall style for the diff view, including styles for
// different line types such as divider, missing, equal, insert, and delete
// lines.
type Style struct {
	DividerLine LineStyle
	MissingLine LineStyle
	EqualLine   LineStyle
	InsertLine  LineStyle
	DeleteLine  LineStyle
}

// DefaultLightStyle provides a default light theme style for the diff view.
func DefaultLightStyle() Style {
	return createStyle(
		charmtone.Iron, charmtone.Thunder, charmtone.Oyster, charmtone.Anchovy, // divider
		charmtone.Ash, charmtone.Ash, // missing
		charmtone.Charcoal, charmtone.Ash, charmtone.Pepper, charmtone.Salt, // equal
		charmtone.Turtle, "#c8e6c9", "#e8f5e9", "#e8f5e9", // insert
		charmtone.Cherry, "#ffcdd2", "#ffebee", "#ffebee", // delete
	)
}

// DefaultDarkStyle provides a default dark theme style for the diff view.
func DefaultDarkStyle() Style {
	return createStyle(
		charmtone.Smoke, charmtone.Sapphire, charmtone.Smoke, charmtone.Ox, // divider
		charmtone.Charcoal, charmtone.Charcoal, // missing
		charmtone.Ash, charmtone.Charcoal, charmtone.Salt, charmtone.Pepper, // equal
		charmtone.Turtle, "#293229", "#303a30", "#303a30", // insert
		charmtone.Cherry, "#332929", "#3a3030", "#3a3030", // delete
	)
}
