package diffview

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/exp/charmtone"
)

// createStyle builds a diff view style with the specified colors
func createStyle(
	dividerLineFg, dividerLineBg, dividerCodeFg, dividerCodeBg charmtone.Key,
	missingLineBg, missingCodeBg charmtone.Key,
	equalLineFg, equalLineBg, equalCodeFg, equalCodeBg charmtone.Key,
	insertLineFg charmtone.Key, insertLineBg, insertSymbolBg, insertCodeBg string,
	deleteLineFg charmtone.Key, deleteLineBg, deleteSymbolBg, deleteCodeBg string,
) Style {
	return Style{
		DividerLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(dividerLineFg).
				Background(dividerLineBg),
			Code: lipgloss.NewStyle().
				Foreground(dividerCodeFg).
				Background(dividerCodeBg),
		},
		MissingLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Background(missingLineBg),
			Code: lipgloss.NewStyle().
				Background(missingCodeBg),
		},
		EqualLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(equalLineFg).
				Background(equalLineBg),
			Code: lipgloss.NewStyle().
				Foreground(equalCodeFg).
				Background(equalCodeBg),
		},
		InsertLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(insertLineFg).
				Background(lipgloss.Color(insertLineBg)),
			Symbol: lipgloss.NewStyle().
				Foreground(insertLineFg).
				Background(lipgloss.Color(insertSymbolBg)),
			Code: lipgloss.NewStyle().
				Foreground(charmtone.Salt).
				Background(lipgloss.Color(insertCodeBg)),
		},
		DeleteLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(deleteLineFg).
				Background(lipgloss.Color(deleteLineBg)),
			Symbol: lipgloss.NewStyle().
				Foreground(deleteLineFg).
				Background(lipgloss.Color(deleteSymbolBg)),
			Code: lipgloss.NewStyle().
				Foreground(charmtone.Salt).
				Background(lipgloss.Color(deleteCodeBg)),
		},
	}
}