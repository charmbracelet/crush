package theme

import (
	"github.com/charmbracelet/lipgloss/v2"
)

// OpenCodeTheme implements the Theme interface with OpenCode brand colors.
// It provides both dark and light variants.
type OpenCodeTheme struct {
	BaseTheme
}

// NewOpenCodeDarkTheme creates a new instance of the OpenCode Dark theme.
func NewOpenCodeDarkTheme() *OpenCodeTheme {
	// OpenCode color palette
	// Dark mode colors
	darkBackground := "#212121"
	darkCurrentLine := "#252525"
	darkSelection := "#303030"
	darkForeground := "#e0e0e0"
	darkComment := "#6a6a6a"
	darkPrimary := "#fab283"   // Primary orange/gold
	darkSecondary := "#5c9cf5" // Secondary blue
	darkAccent := "#9d7cd8"    // Accent purple
	darkRed := "#e06c75"       // Error red
	darkOrange := "#f5a742"    // Warning orange
	darkGreen := "#7fd88f"     // Success green
	darkCyan := "#56b6c2"      // Info cyan
	darkYellow := "#e5c07b"    // Emphasized text
	darkBorder := "#4b4c5c"    // Border color

	theme := &OpenCodeTheme{}

	// Base colors
	theme.PrimaryColor = lipgloss.Color(darkPrimary)
	theme.SecondaryColor = lipgloss.Color(darkSecondary)
	theme.AccentColor = lipgloss.Color(darkAccent)

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
	theme.BackgroundDarkerColor = lipgloss.Color("#121212") // Slightly darker than background

	// Border colors
	theme.BorderNormalColor = lipgloss.Color(darkBorder)
	theme.BorderFocusedColor = lipgloss.Color(darkPrimary)
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
	theme.MarkdownHeadingColor = lipgloss.Color(darkSecondary)
	theme.MarkdownLinkColor = lipgloss.Color(darkPrimary)
	theme.MarkdownLinkTextColor = lipgloss.Color(darkCyan)
	theme.MarkdownCodeColor = lipgloss.Color(darkGreen)
	theme.MarkdownBlockQuoteColor = lipgloss.Color(darkYellow)
	theme.MarkdownEmphColor = lipgloss.Color(darkYellow)
	theme.MarkdownStrongColor = lipgloss.Color(darkAccent)
	theme.MarkdownHorizontalRuleColor = lipgloss.Color(darkComment)
	theme.MarkdownListItemColor = lipgloss.Color(darkPrimary)
	theme.MarkdownListEnumerationColor = lipgloss.Color(darkCyan)
	theme.MarkdownImageColor = lipgloss.Color(darkPrimary)
	theme.MarkdownImageTextColor = lipgloss.Color(darkCyan)
	theme.MarkdownCodeBlockColor = lipgloss.Color(darkForeground)

	// Syntax highlighting colors
	theme.SyntaxCommentColor = lipgloss.Color(darkComment)
	theme.SyntaxKeywordColor = lipgloss.Color(darkSecondary)
	theme.SyntaxFunctionColor = lipgloss.Color(darkPrimary)
	theme.SyntaxVariableColor = lipgloss.Color(darkRed)
	theme.SyntaxStringColor = lipgloss.Color(darkGreen)
	theme.SyntaxNumberColor = lipgloss.Color(darkAccent)
	theme.SyntaxTypeColor = lipgloss.Color(darkYellow)
	theme.SyntaxOperatorColor = lipgloss.Color(darkCyan)
	theme.SyntaxPunctuationColor = lipgloss.Color(darkForeground)

	return theme
}

// NewOpenCodeLightTheme creates a new instance of the OpenCode Light theme.
func NewOpenCodeLightTheme() *OpenCodeTheme {
	// Light mode colors
	lightBackground := "#f8f8f8"
	lightCurrentLine := "#f0f0f0"
	lightSelection := "#e5e5e6"
	lightForeground := "#2a2a2a"
	lightComment := "#8a8a8a"
	lightPrimary := "#3b7dd8"   // Primary blue
	lightSecondary := "#7b5bb6" // Secondary purple
	lightAccent := "#d68c27"    // Accent orange/gold
	lightRed := "#d1383d"       // Error red
	lightOrange := "#d68c27"    // Warning orange
	lightGreen := "#3d9a57"     // Success green
	lightCyan := "#318795"      // Info cyan
	lightYellow := "#b0851f"    // Emphasized text
	lightBorder := "#d3d3d3"    // Border color

	theme := &OpenCodeTheme{}

	// Base colors
	theme.PrimaryColor = lipgloss.Color(lightPrimary)
	theme.SecondaryColor = lipgloss.Color(lightSecondary)
	theme.AccentColor = lipgloss.Color(lightAccent)

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
	theme.BorderFocusedColor = lipgloss.Color(lightPrimary)
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
	theme.MarkdownHeadingColor = lipgloss.Color(lightSecondary)
	theme.MarkdownLinkColor = lipgloss.Color(lightPrimary)
	theme.MarkdownLinkTextColor = lipgloss.Color(lightCyan)
	theme.MarkdownCodeColor = lipgloss.Color(lightGreen)
	theme.MarkdownBlockQuoteColor = lipgloss.Color(lightYellow)
	theme.MarkdownEmphColor = lipgloss.Color(lightYellow)
	theme.MarkdownStrongColor = lipgloss.Color(lightAccent)
	theme.MarkdownHorizontalRuleColor = lipgloss.Color(lightComment)
	theme.MarkdownListItemColor = lipgloss.Color(lightPrimary)
	theme.MarkdownListEnumerationColor = lipgloss.Color(lightCyan)
	theme.MarkdownImageColor = lipgloss.Color(lightPrimary)
	theme.MarkdownImageTextColor = lipgloss.Color(lightCyan)
	theme.MarkdownCodeBlockColor = lipgloss.Color(lightForeground)

	// Syntax highlighting colors
	theme.SyntaxCommentColor = lipgloss.Color(lightComment)
	theme.SyntaxKeywordColor = lipgloss.Color(lightSecondary)
	theme.SyntaxFunctionColor = lipgloss.Color(lightPrimary)
	theme.SyntaxVariableColor = lipgloss.Color(lightRed)
	theme.SyntaxStringColor = lipgloss.Color(lightGreen)
	theme.SyntaxNumberColor = lipgloss.Color(lightAccent)
	theme.SyntaxTypeColor = lipgloss.Color(lightYellow)
	theme.SyntaxOperatorColor = lipgloss.Color(lightCyan)
	theme.SyntaxPunctuationColor = lipgloss.Color(lightForeground)

	return theme
}

func init() {
	// Register the OpenCode themes with the theme manager
	RegisterTheme("opencode-dark", NewOpenCodeDarkTheme())
	RegisterTheme("opencode-light", NewOpenCodeLightTheme())
}