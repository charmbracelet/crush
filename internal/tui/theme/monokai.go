package theme

import (
	"github.com/charmbracelet/lipgloss/v2"
)

// MonokaiProTheme implements the Theme interface with Monokai Pro colors.
// It provides both dark and light variants.
type MonokaiProTheme struct {
	BaseTheme
}

// NewMonokaiProTheme creates a new instance of the Monokai Pro theme.
func NewMonokaiProTheme() *MonokaiProTheme {
	// Monokai Pro color palette (dark mode)
	darkBackground := "#2d2a2e"
	darkCurrentLine := "#403e41"
	darkSelection := "#5b595c"
	darkForeground := "#fcfcfa"
	darkComment := "#727072"
	darkRed := "#ff6188"
	darkOrange := "#fc9867"
	darkYellow := "#ffd866"
	darkGreen := "#a9dc76"
	darkCyan := "#78dce8"
	darkBlue := "#ab9df2"
	darkPurple := "#ab9df2"
	darkBorder := "#403e41"

	theme := &MonokaiProTheme{}

	// Base colors
	theme.PrimaryColor = lipgloss.Color(darkCyan)
	theme.SecondaryColor = lipgloss.Color(darkPurple)
	theme.AccentColor = lipgloss.Color(darkOrange)

	// Status colors
	theme.ErrorColor = lipgloss.Color(darkRed)
	theme.WarningColor = lipgloss.Color(darkOrange)
	theme.SuccessColor = lipgloss.Color(darkGreen)
	theme.InfoColor = lipgloss.Color(darkBlue)

	// Text colors
	theme.TextColor = lipgloss.Color(darkForeground)
	theme.TextMutedColor = lipgloss.Color(darkComment)
	theme.TextEmphasizedColor = lipgloss.Color(darkYellow)

	// Background colors
	theme.BackgroundColor = lipgloss.Color(darkBackground)
	theme.BackgroundSecondaryColor = lipgloss.Color(darkCurrentLine)
	theme.BackgroundDarkerColor = lipgloss.Color("#221f22") // Slightly darker than background

	// Border colors
	theme.BorderNormalColor = lipgloss.Color(darkBorder)
	theme.BorderFocusedColor = lipgloss.Color(darkCyan)
	theme.BorderDimColor = lipgloss.Color(darkSelection)

	// Diff view colors
	theme.DiffAddedColor = lipgloss.Color("#a9dc76")
	theme.DiffRemovedColor = lipgloss.Color("#ff6188")
	theme.DiffContextColor = lipgloss.Color("#a0a0a0")
	theme.DiffHunkHeaderColor = lipgloss.Color("#a0a0a0")
	theme.DiffHighlightAddedColor = lipgloss.Color("#c2e7a9")
	theme.DiffHighlightRemovedColor = lipgloss.Color("#ff8ca6")
	theme.DiffAddedBgColor = lipgloss.Color("#3a4a35")
	theme.DiffRemovedBgColor = lipgloss.Color("#4a3439")
	theme.DiffContextBgColor = lipgloss.Color(darkBackground)
	theme.DiffLineNumberColor = lipgloss.Color("#888888")
	theme.DiffAddedLineNumberBgColor = lipgloss.Color("#2d3a28")
	theme.DiffRemovedLineNumberBgColor = lipgloss.Color("#3d2a2e")

	// Markdown colors
	theme.MarkdownTextColor = lipgloss.Color(darkForeground)
	theme.MarkdownHeadingColor = lipgloss.Color(darkPurple)
	theme.MarkdownLinkColor = lipgloss.Color(darkCyan)
	theme.MarkdownLinkTextColor = lipgloss.Color(darkBlue)
	theme.MarkdownCodeColor = lipgloss.Color(darkGreen)
	theme.MarkdownBlockQuoteColor = lipgloss.Color(darkYellow)
	theme.MarkdownEmphColor = lipgloss.Color(darkYellow)
	theme.MarkdownStrongColor = lipgloss.Color(darkOrange)
	theme.MarkdownHorizontalRuleColor = lipgloss.Color(darkComment)
	theme.MarkdownListItemColor = lipgloss.Color(darkCyan)
	theme.MarkdownListEnumerationColor = lipgloss.Color(darkBlue)
	theme.MarkdownImageColor = lipgloss.Color(darkCyan)
	theme.MarkdownImageTextColor = lipgloss.Color(darkBlue)
	theme.MarkdownCodeBlockColor = lipgloss.Color(darkForeground)

	// Syntax highlighting colors
	theme.SyntaxCommentColor = lipgloss.Color(darkComment)
	theme.SyntaxKeywordColor = lipgloss.Color(darkRed)
	theme.SyntaxFunctionColor = lipgloss.Color(darkGreen)
	theme.SyntaxVariableColor = lipgloss.Color(darkForeground)
	theme.SyntaxStringColor = lipgloss.Color(darkYellow)
	theme.SyntaxNumberColor = lipgloss.Color(darkPurple)
	theme.SyntaxTypeColor = lipgloss.Color(darkBlue)
	theme.SyntaxOperatorColor = lipgloss.Color(darkCyan)
	theme.SyntaxPunctuationColor = lipgloss.Color(darkForeground)

	return theme
}

// NewMonokaiProLightTheme creates a new instance of the Monokai Pro Light theme.
func NewMonokaiProLightTheme() *MonokaiProTheme {
	// Light mode colors (adapted from dark)
	lightBackground := "#fafafa"
	lightCurrentLine := "#f0f0f0"
	lightSelection := "#e5e5e6"
	lightForeground := "#2d2a2e"
	lightComment := "#939293"
	lightRed := "#f92672"
	lightOrange := "#fd971f"
	lightYellow := "#e6db74"
	lightGreen := "#9bca65"
	lightCyan := "#66d9ef"
	lightBlue := "#7e75db"
	lightPurple := "#ae81ff"
	lightBorder := "#d3d3d3"

	theme := &MonokaiProTheme{}

	// Base colors
	theme.PrimaryColor = lipgloss.Color(lightCyan)
	theme.SecondaryColor = lipgloss.Color(lightPurple)
	theme.AccentColor = lipgloss.Color(lightOrange)

	// Status colors
	theme.ErrorColor = lipgloss.Color(lightRed)
	theme.WarningColor = lipgloss.Color(lightOrange)
	theme.SuccessColor = lipgloss.Color(lightGreen)
	theme.InfoColor = lipgloss.Color(lightBlue)

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
	theme.DiffAddedColor = lipgloss.Color("#9bca65")
	theme.DiffRemovedColor = lipgloss.Color("#f92672")
	theme.DiffContextColor = lipgloss.Color("#757575")
	theme.DiffHunkHeaderColor = lipgloss.Color("#757575")
	theme.DiffHighlightAddedColor = lipgloss.Color("#c5e0b4")
	theme.DiffHighlightRemovedColor = lipgloss.Color("#ffb3c8")
	theme.DiffAddedBgColor = lipgloss.Color("#e8f5e9")
	theme.DiffRemovedBgColor = lipgloss.Color("#ffebee")
	theme.DiffContextBgColor = lipgloss.Color(lightBackground)
	theme.DiffLineNumberColor = lipgloss.Color("#9e9e9e")
	theme.DiffAddedLineNumberBgColor = lipgloss.Color("#c8e6c9")
	theme.DiffRemovedLineNumberBgColor = lipgloss.Color("#ffcdd2")

	// Markdown colors
	theme.MarkdownTextColor = lipgloss.Color(lightForeground)
	theme.MarkdownHeadingColor = lipgloss.Color(lightPurple)
	theme.MarkdownLinkColor = lipgloss.Color(lightCyan)
	theme.MarkdownLinkTextColor = lipgloss.Color(lightBlue)
	theme.MarkdownCodeColor = lipgloss.Color(lightGreen)
	theme.MarkdownBlockQuoteColor = lipgloss.Color(lightYellow)
	theme.MarkdownEmphColor = lipgloss.Color(lightYellow)
	theme.MarkdownStrongColor = lipgloss.Color(lightOrange)
	theme.MarkdownHorizontalRuleColor = lipgloss.Color(lightComment)
	theme.MarkdownListItemColor = lipgloss.Color(lightCyan)
	theme.MarkdownListEnumerationColor = lipgloss.Color(lightBlue)
	theme.MarkdownImageColor = lipgloss.Color(lightCyan)
	theme.MarkdownImageTextColor = lipgloss.Color(lightBlue)
	theme.MarkdownCodeBlockColor = lipgloss.Color(lightForeground)

	// Syntax highlighting colors
	theme.SyntaxCommentColor = lipgloss.Color(lightComment)
	theme.SyntaxKeywordColor = lipgloss.Color(lightRed)
	theme.SyntaxFunctionColor = lipgloss.Color(lightGreen)
	theme.SyntaxVariableColor = lipgloss.Color(lightForeground)
	theme.SyntaxStringColor = lipgloss.Color(lightYellow)
	theme.SyntaxNumberColor = lipgloss.Color(lightPurple)
	theme.SyntaxTypeColor = lipgloss.Color(lightBlue)
	theme.SyntaxOperatorColor = lipgloss.Color(lightCyan)
	theme.SyntaxPunctuationColor = lipgloss.Color(lightForeground)

	return theme
}

func init() {
	// Register the Monokai Pro themes with the theme manager
	RegisterTheme("monokai", NewMonokaiProTheme())
	RegisterTheme("monokai-light", NewMonokaiProLightTheme())
}
