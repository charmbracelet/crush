package theme

import (
	"github.com/charmbracelet/lipgloss/v2"
)

// TronTheme implements the Theme interface with Tron-inspired colors.
// It provides both dark and light variants, though Tron is primarily a dark theme.
type TronTheme struct {
	BaseTheme
}

// NewTronTheme creates a new instance of the Tron theme.
func NewTronTheme() *TronTheme {
	// Tron color palette
	// Inspired by the Tron movie's neon aesthetic
	darkBackground := "#0c141f"
	darkCurrentLine := "#1a2633"
	darkSelection := "#1a2633"
	darkForeground := "#caf0ff"
	darkComment := "#4d6b87"
	darkCyan := "#00d9ff"
	darkBlue := "#007fff"
	darkOrange := "#ff9000"
	darkPink := "#ff00a0"
	darkPurple := "#b73fff"
	darkRed := "#ff3333"
	darkYellow := "#ffcc00"
	darkGreen := "#00ff8f"
	darkBorder := "#1a2633"

	theme := &TronTheme{}

	// Base colors
	theme.PrimaryColor = lipgloss.Color(darkCyan)
	theme.SecondaryColor = lipgloss.Color(darkBlue)
	theme.AccentColor = lipgloss.Color(darkOrange)

	// Status colors
	theme.ErrorColor = lipgloss.Color(darkRed)
	theme.WarningColor = lipgloss.Color(darkOrange)
	theme.SuccessColor = lipgloss.Color(darkGreen)
	theme.InfoColor = lipgloss.Color(darkCyan)

	// Text colors
	theme.TextColor = lipgloss.Color(darkForeground)
	theme.TextMutedColor = lipgloss.Color(darkComment)
	theme.TextEmphasizedColor = lipgloss.Color(darkYellow)

	// Background colors
	theme.BackgroundColor = lipgloss.Color(darkBackground)
	theme.BackgroundSecondaryColor = lipgloss.Color(darkCurrentLine)
	theme.BackgroundDarkerColor = lipgloss.Color("#070d14") // Slightly darker than background

	// Border colors
	theme.BorderNormalColor = lipgloss.Color(darkBorder)
	theme.BorderFocusedColor = lipgloss.Color(darkCyan)
	theme.BorderDimColor = lipgloss.Color(darkSelection)

	// Diff view colors
	theme.DiffAddedColor = lipgloss.Color(darkGreen)
	theme.DiffRemovedColor = lipgloss.Color(darkRed)
	theme.DiffContextColor = lipgloss.Color(darkComment)
	theme.DiffHunkHeaderColor = lipgloss.Color(darkBlue)
	theme.DiffHighlightAddedColor = lipgloss.Color("#00ff8f")
	theme.DiffHighlightRemovedColor = lipgloss.Color("#ff3333")
	theme.DiffAddedBgColor = lipgloss.Color("#0a2a1a")
	theme.DiffRemovedBgColor = lipgloss.Color("#2a0a0a")
	theme.DiffContextBgColor = lipgloss.Color(darkBackground)
	theme.DiffLineNumberColor = lipgloss.Color(darkComment)
	theme.DiffAddedLineNumberBgColor = lipgloss.Color("#082015")
	theme.DiffRemovedLineNumberBgColor = lipgloss.Color("#200808")

	// Markdown colors
	theme.MarkdownTextColor = lipgloss.Color(darkForeground)
	theme.MarkdownHeadingColor = lipgloss.Color(darkCyan)
	theme.MarkdownLinkColor = lipgloss.Color(darkBlue)
	theme.MarkdownLinkTextColor = lipgloss.Color(darkCyan)
	theme.MarkdownCodeColor = lipgloss.Color(darkGreen)
	theme.MarkdownBlockQuoteColor = lipgloss.Color(darkYellow)
	theme.MarkdownEmphColor = lipgloss.Color(darkYellow)
	theme.MarkdownStrongColor = lipgloss.Color(darkOrange)
	theme.MarkdownHorizontalRuleColor = lipgloss.Color(darkComment)
	theme.MarkdownListItemColor = lipgloss.Color(darkBlue)
	theme.MarkdownListEnumerationColor = lipgloss.Color(darkCyan)
	theme.MarkdownImageColor = lipgloss.Color(darkBlue)
	theme.MarkdownImageTextColor = lipgloss.Color(darkCyan)
	theme.MarkdownCodeBlockColor = lipgloss.Color(darkForeground)

	// Syntax highlighting colors
	theme.SyntaxCommentColor = lipgloss.Color(darkComment)
	theme.SyntaxKeywordColor = lipgloss.Color(darkCyan)
	theme.SyntaxFunctionColor = lipgloss.Color(darkGreen)
	theme.SyntaxVariableColor = lipgloss.Color(darkOrange)
	theme.SyntaxStringColor = lipgloss.Color(darkYellow)
	theme.SyntaxNumberColor = lipgloss.Color(darkBlue)
	theme.SyntaxTypeColor = lipgloss.Color(darkPurple)
	theme.SyntaxOperatorColor = lipgloss.Color(darkPink)
	theme.SyntaxPunctuationColor = lipgloss.Color(darkForeground)

	return theme
}

// NewTronLightTheme creates a new instance of the Tron Light theme.
func NewTronLightTheme() *TronTheme {
	// Light mode approximation
	lightBackground := "#f0f8ff"
	lightCurrentLine := "#e0f0ff"
	lightSelection := "#d0e8ff"
	lightForeground := "#0c141f"
	lightComment := "#4d6b87"
	lightCyan := "#0097b3"
	lightBlue := "#0066cc"
	lightOrange := "#cc7300"
	lightPink := "#cc0080"
	lightPurple := "#9932cc"
	lightRed := "#cc2929"
	lightYellow := "#cc9900"
	lightGreen := "#00cc72"
	lightBorder := "#d0e8ff"

	theme := &TronTheme{}

	// Base colors
	theme.PrimaryColor = lipgloss.Color(lightCyan)
	theme.SecondaryColor = lipgloss.Color(lightBlue)
	theme.AccentColor = lipgloss.Color(lightOrange)

	// Status colors
	theme.ErrorColor = lipgloss.Color(lightRed)
	theme.WarningColor = lipgloss.Color(lightOrange)
	theme.SuccessColor = lipgloss.Color(lightGreen)
	theme.InfoColor = lipgloss.Color(lightCyan)

	// Text colors
	theme.TextColor = lipgloss.Color(lightForeground)
	theme.TextMutedColor = lipgloss.Color(lightComment)
	theme.TextEmphasizedColor = lipgloss.Color(lightYellow)

	// Background colors
	theme.BackgroundColor = lipgloss.Color(lightBackground)
	theme.BackgroundSecondaryColor = lipgloss.Color(lightCurrentLine)
	theme.BackgroundDarkerColor = lipgloss.Color("#ffffff") // Slightly lighter than background

	// Border colors
	theme.BorderNormalColor = lipgloss.Color(lightBorder)
	theme.BorderFocusedColor = lipgloss.Color(lightCyan)
	theme.BorderDimColor = lipgloss.Color(lightSelection)

	// Diff view colors
	theme.DiffAddedColor = lipgloss.Color(lightGreen)
	theme.DiffRemovedColor = lipgloss.Color(lightRed)
	theme.DiffContextColor = lipgloss.Color(lightComment)
	theme.DiffHunkHeaderColor = lipgloss.Color(lightBlue)
	theme.DiffHighlightAddedColor = lipgloss.Color("#a5d6a7")
	theme.DiffHighlightRemovedColor = lipgloss.Color("#ef9a9a")
	theme.DiffAddedBgColor = lipgloss.Color("#e8f5e9")
	theme.DiffRemovedBgColor = lipgloss.Color("#ffebee")
	theme.DiffContextBgColor = lipgloss.Color(lightBackground)
	theme.DiffLineNumberColor = lipgloss.Color(lightComment)
	theme.DiffAddedLineNumberBgColor = lipgloss.Color("#c8e6c9")
	theme.DiffRemovedLineNumberBgColor = lipgloss.Color("#ffcdd2")

	// Markdown colors
	theme.MarkdownTextColor = lipgloss.Color(lightForeground)
	theme.MarkdownHeadingColor = lipgloss.Color(lightCyan)
	theme.MarkdownLinkColor = lipgloss.Color(lightBlue)
	theme.MarkdownLinkTextColor = lipgloss.Color(lightCyan)
	theme.MarkdownCodeColor = lipgloss.Color(lightGreen)
	theme.MarkdownBlockQuoteColor = lipgloss.Color(lightYellow)
	theme.MarkdownEmphColor = lipgloss.Color(lightYellow)
	theme.MarkdownStrongColor = lipgloss.Color(lightOrange)
	theme.MarkdownHorizontalRuleColor = lipgloss.Color(lightComment)
	theme.MarkdownListItemColor = lipgloss.Color(lightBlue)
	theme.MarkdownListEnumerationColor = lipgloss.Color(lightCyan)
	theme.MarkdownImageColor = lipgloss.Color(lightBlue)
	theme.MarkdownImageTextColor = lipgloss.Color(lightCyan)
	theme.MarkdownCodeBlockColor = lipgloss.Color(lightForeground)

	// Syntax highlighting colors
	theme.SyntaxCommentColor = lipgloss.Color(lightComment)
	theme.SyntaxKeywordColor = lipgloss.Color(lightCyan)
	theme.SyntaxFunctionColor = lipgloss.Color(lightGreen)
	theme.SyntaxVariableColor = lipgloss.Color(lightOrange)
	theme.SyntaxStringColor = lipgloss.Color(lightYellow)
	theme.SyntaxNumberColor = lipgloss.Color(lightBlue)
	theme.SyntaxTypeColor = lipgloss.Color(lightPurple)
	theme.SyntaxOperatorColor = lipgloss.Color(lightPink)
	theme.SyntaxPunctuationColor = lipgloss.Color(lightForeground)

	return theme
}

func init() {
	// Register the Tron themes with the theme manager
	RegisterTheme("tron", NewTronTheme())
	RegisterTheme("tron-light", NewTronLightTheme())
}