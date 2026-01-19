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

// colorScheme defines the colors for a theme
type colorScheme struct {
	dividerLineFg, dividerLineBg, dividerCodeFg, dividerCodeBg charmtone.Key
	missingLineBg, missingCodeBg                               charmtone.Key
	equalLineFg, equalLineBg, equalCodeFg, equalCodeBg         charmtone.Key
	insertLineFg                                               charmtone.Key
	insertLineBg, insertSymbolBg, insertCodeBg                 string
	deleteLineFg                                               charmtone.Key
	deleteLineBg, deleteSymbolBg, deleteCodeBg                 string
}

// buildStyle creates a Style from the given color scheme
func buildStyle(scheme colorScheme) Style {
	return Style{
		DividerLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(scheme.dividerLineFg).
				Background(scheme.dividerLineBg),
			Code: lipgloss.NewStyle().
				Foreground(scheme.dividerCodeFg).
				Background(scheme.dividerCodeBg),
		},
		MissingLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Background(scheme.missingLineBg),
			Code: lipgloss.NewStyle().
				Background(scheme.missingCodeBg),
		},
		EqualLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(scheme.equalLineFg).
				Background(scheme.equalLineBg),
			Code: lipgloss.NewStyle().
				Foreground(scheme.equalCodeFg).
				Background(scheme.equalCodeBg),
		},
		InsertLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(scheme.insertLineFg).
				Background(lipgloss.Color(scheme.insertLineBg)),
			Symbol: lipgloss.NewStyle().
				Foreground(scheme.insertLineFg).
				Background(lipgloss.Color(scheme.insertSymbolBg)),
			Code: lipgloss.NewStyle().
				Foreground(charmtone.Salt).
				Background(lipgloss.Color(scheme.insertCodeBg)),
		},
		DeleteLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(scheme.deleteLineFg).
				Background(lipgloss.Color(scheme.deleteLineBg)),
			Symbol: lipgloss.NewStyle().
				Foreground(scheme.deleteLineFg).
				Background(lipgloss.Color(scheme.deleteSymbolBg)),
			Code: lipgloss.NewStyle().
				Foreground(charmtone.Salt).
				Background(lipgloss.Color(scheme.deleteCodeBg)),
		},
	}
}

// DefaultLightStyle provides a default light theme style for the diff view.
func DefaultLightStyle() Style {
	return buildStyle(colorScheme{
		dividerLineFg: charmtone.Iron, dividerLineBg: charmtone.Thunder,
		dividerCodeFg: charmtone.Oyster, dividerCodeBg: charmtone.Anchovy,
		missingLineBg: charmtone.Ash, missingCodeBg: charmtone.Ash,
		equalLineFg: charmtone.Charcoal, equalLineBg: charmtone.Ash,
		equalCodeFg: charmtone.Pepper, equalCodeBg: charmtone.Salt,
		insertLineFg: charmtone.Turtle,
		insertLineBg: "#c8e6c9", insertSymbolBg: "#e8f5e9", insertCodeBg: "#e8f5e9",
		deleteLineFg: charmtone.Cherry,
		deleteLineBg: "#ffcdd2", deleteSymbolBg: "#ffebee", deleteCodeBg: "#ffebee",
	})
}

// DefaultDarkStyle provides a default dark theme style for the diff view.
func DefaultDarkStyle() Style {
	return Style{
		DividerLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(charmtone.Smoke).
				Background(charmtone.Sapphire),
			Code: lipgloss.NewStyle().
				Foreground(charmtone.Smoke).
				Background(charmtone.Ox),
		},
		MissingLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Background(charmtone.Charcoal),
			Code: lipgloss.NewStyle().
				Background(charmtone.Charcoal),
		},
		EqualLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(charmtone.Ash).
				Background(charmtone.Charcoal),
			Code: lipgloss.NewStyle().
				Foreground(charmtone.Salt).
				Background(charmtone.Pepper),
		},
		InsertLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(charmtone.Turtle).
				Background(lipgloss.Color("#293229")),
			Symbol: lipgloss.NewStyle().
				Foreground(charmtone.Turtle).
				Background(lipgloss.Color("#303a30")),
			Code: lipgloss.NewStyle().
				Foreground(charmtone.Salt).
				Background(lipgloss.Color("#303a30")),
		},
		DeleteLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(charmtone.Cherry).
				Background(lipgloss.Color("#332929")),
			Symbol: lipgloss.NewStyle().
				Foreground(charmtone.Cherry).
				Background(lipgloss.Color("#3a3030")),
			Code: lipgloss.NewStyle().
				Foreground(charmtone.Salt).
				Background(lipgloss.Color("#3a3030")),
		},
	}
}
