package diffview

import (
	"image/color"

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
// These use charmtone colors as a fallback; prefer constructing styles via
// the themed palette in styles.NewStyles() for TUI use.
func DefaultLightStyle() Style {
	dividerFg := charmtone.Iron
	dividerBg := charmtone.Thunder
	dividerCodeFg := charmtone.Oyster
	dividerCodeBg := charmtone.Anchovy
	missingBg := charmtone.Ash
	equalFg := charmtone.Charcoal
	equalBg := charmtone.Ash
	equalCodeFg := charmtone.Pepper
	equalCodeBg := charmtone.Salt
	insertFg := charmtone.Turtle
	insertBg := lipgloss.Color("#c8e6c9")
	insertCodeBg := lipgloss.Color("#e8f5e9")
	deleteFg := charmtone.Cherry
	deleteBg := lipgloss.Color("#ffcdd2")
	deleteCodeBg := lipgloss.Color("#ffebee")
	codeFg := charmtone.Pepper

	return buildDiffStyle(
		dividerFg, dividerBg, dividerCodeFg, dividerCodeBg,
		missingBg,
		equalFg, equalBg, equalCodeFg, equalCodeBg,
		insertFg, insertBg, insertCodeBg,
		deleteFg, deleteBg, deleteCodeBg,
		codeFg,
	)
}

// DefaultDarkStyle provides a default dark theme style for the diff view.
// These use charmtone colors as a fallback; prefer constructing styles via
// the themed palette in styles.NewStyles() for TUI use.
func DefaultDarkStyle() Style {
	dividerFg := charmtone.Smoke
	dividerBg := charmtone.Sapphire
	dividerCodeFg := charmtone.Smoke
	dividerCodeBg := charmtone.Ox
	missingBg := charmtone.Charcoal
	equalFg := charmtone.Ash
	equalBg := charmtone.Charcoal
	equalCodeFg := charmtone.Salt
	equalCodeBg := charmtone.Pepper
	insertFg := charmtone.Turtle
	insertBg := lipgloss.Color("#293229")
	insertCodeBg := lipgloss.Color("#303a30")
	deleteFg := charmtone.Cherry
	deleteBg := lipgloss.Color("#332929")
	deleteCodeBg := lipgloss.Color("#3a3030")
	codeFg := charmtone.Salt

	return buildDiffStyle(
		dividerFg, dividerBg, dividerCodeFg, dividerCodeBg,
		missingBg,
		equalFg, equalBg, equalCodeFg, equalCodeBg,
		insertFg, insertBg, insertCodeBg,
		deleteFg, deleteBg, deleteCodeBg,
		codeFg,
	)
}

// buildDiffStyle constructs a Style from individual color values.
func buildDiffStyle(
	dividerFg, dividerBg, dividerCodeFg, dividerCodeBg,
	missingBg,
	equalFg, equalBg, equalCodeFg, equalCodeBg color.Color,
	insertFg color.Color, insertBg, insertCodeBg color.Color,
	deleteFg color.Color, deleteBg, deleteCodeBg color.Color,
	codeFg color.Color,
) Style {
	return Style{
		DividerLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(dividerFg).
				Background(dividerBg),
			Code: lipgloss.NewStyle().
				Foreground(dividerCodeFg).
				Background(dividerCodeBg),
		},
		MissingLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Background(missingBg),
			Code: lipgloss.NewStyle().
				Background(missingBg),
		},
		EqualLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(equalFg).
				Background(equalBg),
			Code: lipgloss.NewStyle().
				Foreground(equalCodeFg).
				Background(equalCodeBg),
		},
		InsertLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(insertFg).
				Background(insertBg),
			Symbol: lipgloss.NewStyle().
				Foreground(insertFg).
				Background(insertCodeBg),
			Code: lipgloss.NewStyle().
				Foreground(codeFg).
				Background(insertCodeBg),
		},
		DeleteLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(deleteFg).
				Background(deleteBg),
			Symbol: lipgloss.NewStyle().
				Foreground(deleteFg).
				Background(deleteCodeBg),
			Code: lipgloss.NewStyle().
				Foreground(codeFg).
				Background(deleteCodeBg),
		},
	}
}
