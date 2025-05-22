package theme

import (
	"github.com/charmbracelet/lipgloss/v2"
)

// TokyoNightTheme implements the Theme interface with Tokyo Night colors.
// It provides both dark and light variants.
type TokyoNightTheme struct {
	BaseTheme
}

// NewTokyoNightTheme creates a new instance of the Tokyo Night theme.
func NewTokyoNightTheme() *TokyoNightTheme {
	// Tokyo Night color palette
	// Dark mode colors
	darkBackground := "#222436"
	darkCurrentLine := "#1e2030"
	darkSelection := "#2f334d"
	darkForeground := "#c8d3f5"
	darkComment := "#636da6"
	darkRed := "#ff757f"
	darkOrange := "#ff966c"
	darkYellow := "#ffc777"
	darkGreen := "#c3e88d"
	darkCyan := "#86e1fc"
	darkBlue := "#82aaff"
	darkPurple := "#c099ff"
	darkBorder := "#3b4261"

	theme := &TokyoNightTheme{}

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
	theme.BackgroundDarkerColor = lipgloss.Color("#191B29") // Darker background from palette

	// Border colors
	theme.BorderNormalColor = lipgloss.Color(darkBorder)
	theme.BorderFocusedColor = lipgloss.Color(darkBlue)
	theme.BorderDimColor = lipgloss.Color(darkSelection)

	// Diff view colors
	theme.DiffAddedColor = lipgloss.Color("#4fd6be")            // teal from palette
	theme.DiffRemovedColor = lipgloss.Color("#c53b53")          // red1 from palette
	theme.DiffContextColor = lipgloss.Color("#828bb8")          // fg_dark from palette
	theme.DiffHunkHeaderColor = lipgloss.Color("#828bb8")       // fg_dark from palette
	theme.DiffHighlightAddedColor = lipgloss.Color("#b8db87")   // git.add from palette
	theme.DiffHighlightRemovedColor = lipgloss.Color("#e26a75") // git.delete from palette
	theme.DiffAddedBgColor = lipgloss.Color("#20303b")
	theme.DiffRemovedBgColor = lipgloss.Color("#37222c")
	theme.DiffContextBgColor = lipgloss.Color(darkBackground)
	theme.DiffLineNumberColor = lipgloss.Color("#545c7e") // dark3 from palette
	theme.DiffAddedLineNumberBgColor = lipgloss.Color("#1b2b34")
	theme.DiffRemovedLineNumberBgColor = lipgloss.Color("#2d1f26")

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

// NewTokyoNightDayTheme creates a new instance of the Tokyo Night Day theme.
func NewTokyoNightDayTheme() *TokyoNightTheme {
	// Light mode colors (Tokyo Night Day)
	lightBackground := "#e1e2e7"
	lightCurrentLine := "#d5d6db"
	lightSelection := "#c8c9ce"
	lightForeground := "#3760bf"
	lightComment := "#848cb5"
	lightRed := "#f52a65"
	lightOrange := "#b15c00"
	lightYellow := "#8c6c3e"
	lightGreen := "#587539"
	lightCyan := "#007197"
	lightBlue := "#2e7de9"
	lightPurple := "#9854f1"
	lightBorder := "#a8aecb"

	theme := &TokyoNightTheme{}

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
	theme.BackgroundDarkerColor = lipgloss.Color("#f0f0f5") // Slightly lighter than background

	// Border colors
	theme.BorderNormalColor = lipgloss.Color(lightBorder)
	theme.BorderFocusedColor = lipgloss.Color(lightBlue)
	theme.BorderDimColor = lipgloss.Color(lightSelection)

	// Diff view colors
	theme.DiffAddedColor = lipgloss.Color("#1e725c")
	theme.DiffRemovedColor = lipgloss.Color("#c53b53")
	theme.DiffContextColor = lipgloss.Color("#7086b5")
	theme.DiffHunkHeaderColor = lipgloss.Color("#7086b5")
	theme.DiffHighlightAddedColor = lipgloss.Color("#4db380")
	theme.DiffHighlightRemovedColor = lipgloss.Color("#f52a65")
	theme.DiffAddedBgColor = lipgloss.Color("#d5e5d5")
	theme.DiffRemovedBgColor = lipgloss.Color("#f7d8db")
	theme.DiffContextBgColor = lipgloss.Color(lightBackground)
	theme.DiffLineNumberColor = lipgloss.Color("#848cb5")
	theme.DiffAddedLineNumberBgColor = lipgloss.Color("#c5d5c5")
	theme.DiffRemovedLineNumberBgColor = lipgloss.Color("#e7c8cb")

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
	// Register the Tokyo Night themes with the theme manager
	RegisterTheme("tokyonight", NewTokyoNightTheme())
	RegisterTheme("tokyonight-day", NewTokyoNightDayTheme())
}
