package theme

import (
	"github.com/charmbracelet/lipgloss/v2"
)

// OneDarkTheme implements the Theme interface with Atom's One Dark colors.
// It provides both dark and light variants.
type OneDarkTheme struct {
	BaseTheme
}

// NewOneDarkTheme creates a new instance of the One Dark theme.
func NewOneDarkTheme() *OneDarkTheme {
	// One Dark color palette
	// Dark mode colors from Atom One Dark
	darkBackground := "#282c34"
	darkCurrentLine := "#2c313c"
	darkSelection := "#3e4451"
	darkForeground := "#abb2bf"
	darkComment := "#5c6370"
	darkRed := "#e06c75"
	darkOrange := "#d19a66"
	darkYellow := "#e5c07b"
	darkGreen := "#98c379"
	darkCyan := "#56b6c2"
	darkBlue := "#61afef"
	darkPurple := "#c678dd"
	darkBorder := "#3b4048"

	theme := &OneDarkTheme{}

	// Base colors
	theme.PrimaryColor = lipgloss.Color(darkBlue)
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
	theme.BackgroundDarkerColor = lipgloss.Color("#21252b") // Slightly darker than background

	// Border colors
	theme.BorderNormalColor = lipgloss.Color(darkBorder)
	theme.BorderFocusedColor = lipgloss.Color(darkBlue)
	theme.BorderDimColor = lipgloss.Color(darkSelection)

	// Diff view colors
	theme.DiffAddedColor = lipgloss.Color("#478247")
	theme.DiffRemovedColor = lipgloss.Color("#7C4444")
	theme.DiffContextColor = lipgloss.Color("#a0a0a0")
	theme.DiffHunkHeaderColor = lipgloss.Color("#a0a0a0")
	theme.DiffHighlightAddedColor = lipgloss.Color("#DAFADA")
	theme.DiffHighlightRemovedColor = lipgloss.Color("#FADADD")
	theme.DiffAddedBgColor = lipgloss.Color("#303A30")
	theme.DiffRemovedBgColor = lipgloss.Color("#3A3030")
	theme.DiffContextBgColor = lipgloss.Color(darkBackground)
	theme.DiffLineNumberColor = lipgloss.Color("#888888")
	theme.DiffAddedLineNumberBgColor = lipgloss.Color("#293229")
	theme.DiffRemovedLineNumberBgColor = lipgloss.Color("#332929")

	// Markdown colors
	theme.MarkdownTextColor = lipgloss.Color(darkForeground)
	theme.MarkdownHeadingColor = lipgloss.Color(darkPurple)
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
	theme.SyntaxKeywordColor = lipgloss.Color(darkPurple)
	theme.SyntaxFunctionColor = lipgloss.Color(darkBlue)
	theme.SyntaxVariableColor = lipgloss.Color(darkRed)
	theme.SyntaxStringColor = lipgloss.Color(darkGreen)
	theme.SyntaxNumberColor = lipgloss.Color(darkOrange)
	theme.SyntaxTypeColor = lipgloss.Color(darkYellow)
	theme.SyntaxOperatorColor = lipgloss.Color(darkCyan)
	theme.SyntaxPunctuationColor = lipgloss.Color(darkForeground)

	return theme
}

// NewOneLightTheme creates a new instance of the One Light theme.
func NewOneLightTheme() *OneDarkTheme {
	// Light mode colors from Atom One Light
	lightBackground := "#fafafa"
	lightCurrentLine := "#f0f0f0"
	lightSelection := "#e5e5e6"
	lightForeground := "#383a42"
	lightComment := "#a0a1a7"
	lightRed := "#e45649"
	lightOrange := "#da8548"
	lightYellow := "#c18401"
	lightGreen := "#50a14f"
	lightCyan := "#0184bc"
	lightBlue := "#4078f2"
	lightPurple := "#a626a4"
	lightBorder := "#d3d3d3"

	theme := &OneDarkTheme{}

	// Base colors
	theme.PrimaryColor = lipgloss.Color(lightBlue)
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
	theme.BorderFocusedColor = lipgloss.Color(lightBlue)
	theme.BorderDimColor = lipgloss.Color(lightSelection)

	// Diff view colors
	theme.DiffAddedColor = lipgloss.Color("#2E7D32")
	theme.DiffRemovedColor = lipgloss.Color("#C62828")
	theme.DiffContextColor = lipgloss.Color("#757575")
	theme.DiffHunkHeaderColor = lipgloss.Color("#757575")
	theme.DiffHighlightAddedColor = lipgloss.Color("#A5D6A7")
	theme.DiffHighlightRemovedColor = lipgloss.Color("#EF9A9A")
	theme.DiffAddedBgColor = lipgloss.Color("#E8F5E9")
	theme.DiffRemovedBgColor = lipgloss.Color("#FFEBEE")
	theme.DiffContextBgColor = lipgloss.Color(lightBackground)
	theme.DiffLineNumberColor = lipgloss.Color("#9E9E9E")
	theme.DiffAddedLineNumberBgColor = lipgloss.Color("#C8E6C9")
	theme.DiffRemovedLineNumberBgColor = lipgloss.Color("#FFCDD2")

	// Markdown colors
	theme.MarkdownTextColor = lipgloss.Color(lightForeground)
	theme.MarkdownHeadingColor = lipgloss.Color(lightPurple)
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
	theme.SyntaxKeywordColor = lipgloss.Color(lightPurple)
	theme.SyntaxFunctionColor = lipgloss.Color(lightBlue)
	theme.SyntaxVariableColor = lipgloss.Color(lightRed)
	theme.SyntaxStringColor = lipgloss.Color(lightGreen)
	theme.SyntaxNumberColor = lipgloss.Color(lightOrange)
	theme.SyntaxTypeColor = lipgloss.Color(lightYellow)
	theme.SyntaxOperatorColor = lipgloss.Color(lightCyan)
	theme.SyntaxPunctuationColor = lipgloss.Color(lightForeground)

	return theme
}

func init() {
	// Register the One Dark and One Light themes with the theme manager
	RegisterTheme("onedark", NewOneDarkTheme())
	RegisterTheme("onelight", NewOneLightTheme())
}
