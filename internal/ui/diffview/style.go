package diffview

import (
	"charm.land/lipgloss/v2"
	"charm.land/tone"
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
	Filename    LineStyle
}

// DefaultLightStyle provides a default light theme style for the diff view.
func DefaultLightStyle() Style {
	return Style{
		DividerLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(tone.Iron).
				Background(tone.Thunder),
			Code: lipgloss.NewStyle().
				Foreground(tone.Oyster).
				Background(tone.Anchovy),
		},
		MissingLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Background(tone.Sash),
			Code: lipgloss.NewStyle().
				Background(tone.Sash),
		},
		EqualLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(tone.Char).
				Background(tone.Sash),
			Code: lipgloss.NewStyle().
				Foreground(tone.Pepper).
				Background(tone.Salt),
		},
		InsertLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(tone.Turtle).
				Background(lipgloss.Color("#c8e6c9")),
			Symbol: lipgloss.NewStyle().
				Foreground(tone.Turtle).
				Background(lipgloss.Color("#e8f5e9")),
			Code: lipgloss.NewStyle().
				Foreground(tone.Pepper).
				Background(lipgloss.Color("#e8f5e9")),
		},
		DeleteLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(tone.Cherry).
				Background(lipgloss.Color("#ffcdd2")),
			Symbol: lipgloss.NewStyle().
				Foreground(tone.Cherry).
				Background(lipgloss.Color("#ffebee")),
			Code: lipgloss.NewStyle().
				Foreground(tone.Pepper).
				Background(lipgloss.Color("#ffebee")),
		},
		Filename: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(tone.Iron).
				Background(tone.Thunder),
			Code: lipgloss.NewStyle().
				Foreground(tone.Iron).
				Background(tone.Thunder),
		},
	}
}

// DefaultDarkStyle provides a default dark theme style for the diff view.
func DefaultDarkStyle() Style {
	return Style{
		DividerLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(tone.Smoke).
				Background(tone.Sapphire),
			Code: lipgloss.NewStyle().
				Foreground(tone.Smoke).
				Background(tone.Ox),
		},
		MissingLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Background(tone.Char),
			Code: lipgloss.NewStyle().
				Background(tone.Char),
		},
		EqualLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(tone.Sash).
				Background(tone.Char),
			Code: lipgloss.NewStyle().
				Foreground(tone.Salt).
				Background(tone.Pepper),
		},
		InsertLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(tone.Turtle).
				Background(lipgloss.Color("#293229")),
			Symbol: lipgloss.NewStyle().
				Foreground(tone.Turtle).
				Background(lipgloss.Color("#303a30")),
			Code: lipgloss.NewStyle().
				Foreground(tone.Salt).
				Background(lipgloss.Color("#303a30")),
		},
		DeleteLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(tone.Cherry).
				Background(lipgloss.Color("#332929")),
			Symbol: lipgloss.NewStyle().
				Foreground(tone.Cherry).
				Background(lipgloss.Color("#3a3030")),
			Code: lipgloss.NewStyle().
				Foreground(tone.Salt).
				Background(lipgloss.Color("#3a3030")),
		},
		Filename: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(tone.Smoke).
				Background(tone.Sapphire),
			Code: lipgloss.NewStyle().
				Foreground(tone.Smoke).
				Background(tone.Sapphire),
		},
	}
}
