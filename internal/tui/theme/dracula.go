package theme

import (
	"github.com/charmbracelet/lipgloss/v2"
)

// DraculaTheme implements the Theme interface with Dracula colors.
// It provides both dark and light variants, though Dracula is primarily a dark theme.
type DraculaTheme struct {
	BaseTheme
}

// NewDraculaTheme creates a new instance of the Dracula theme.
func NewDraculaTheme() *DraculaTheme {
	// Dracula color palette
	// Official colors from https://draculatheme.com/
	darkBackground := "#282a36"
	darkCurrentLine := "#44475a"
	darkSelection := "#44475a"
	darkForeground := "#f8f8f2"
	darkComment := "#6272a4"
	darkCyan := "#8be9fd"
	darkGreen := "#50fa7b"
	darkOrange := "#ffb86c"
	darkPink := "#ff79c6"
	darkPurple := "#bd93f9"
	darkRed := "#ff5555"
	darkYellow := "#f1fa8c"
	darkBorder := "#44475a"

	theme := &DraculaTheme{}

	// Base colors
	theme.PrimaryColor = lipgloss.Color(darkPurple)
	theme.SecondaryColor = lipgloss.Color(darkPink)
	theme.AccentColor = lipgloss.Color(darkCyan)

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
	theme.BackgroundDarkerColor = lipgloss.Color("#21222c") // Slightly darker than background

	// Border colors
	theme.BorderNormalColor = lipgloss.Color(darkBorder)
	theme.BorderFocusedColor = lipgloss.Color(darkPurple)
	theme.BorderDimColor = lipgloss.Color(darkSelection)

	// Diff view colors
	theme.DiffAddedColor = lipgloss.Color(darkGreen)
	theme.DiffRemovedColor = lipgloss.Color(darkRed)
	theme.DiffContextColor = lipgloss.Color(darkComment)
	theme.DiffHunkHeaderColor = lipgloss.Color(darkPurple)
	theme.DiffHighlightAddedColor = lipgloss.Color("#50fa7b")
	theme.DiffHighlightRemovedColor = lipgloss.Color("#ff5555")
	theme.DiffAddedBgColor = lipgloss.Color("#2c3b2c")
	theme.DiffRemovedBgColor = lipgloss.Color("#3b2c2c")
	theme.DiffContextBgColor = lipgloss.Color(darkBackground)
	theme.DiffLineNumberColor = lipgloss.Color(darkComment)
	theme.DiffAddedLineNumberBgColor = lipgloss.Color("#253025")
	theme.DiffRemovedLineNumberBgColor = lipgloss.Color("#302525")

	// Markdown colors
	theme.MarkdownTextColor = lipgloss.Color(darkForeground)
	theme.MarkdownHeadingColor = lipgloss.Color(darkPink)
	theme.MarkdownLinkColor = lipgloss.Color(darkPurple)
	theme.MarkdownLinkTextColor = lipgloss.Color(darkCyan)
	theme.MarkdownCodeColor = lipgloss.Color(darkGreen)
	theme.MarkdownBlockQuoteColor = lipgloss.Color(darkYellow)
	theme.MarkdownEmphColor = lipgloss.Color(darkYellow)
	theme.MarkdownStrongColor = lipgloss.Color(darkOrange)
	theme.MarkdownHorizontalRuleColor = lipgloss.Color(darkComment)
	theme.MarkdownListItemColor = lipgloss.Color(darkPurple)
	theme.MarkdownListEnumerationColor = lipgloss.Color(darkCyan)
	theme.MarkdownImageColor = lipgloss.Color(darkPurple)
	theme.MarkdownImageTextColor = lipgloss.Color(darkCyan)
	theme.MarkdownCodeBlockColor = lipgloss.Color(darkForeground)

	// Syntax highlighting colors
	theme.SyntaxCommentColor = lipgloss.Color(darkComment)
	theme.SyntaxKeywordColor = lipgloss.Color(darkPink)
	theme.SyntaxFunctionColor = lipgloss.Color(darkGreen)
	theme.SyntaxVariableColor = lipgloss.Color(darkOrange)
	theme.SyntaxStringColor = lipgloss.Color(darkYellow)
	theme.SyntaxNumberColor = lipgloss.Color(darkPurple)
	theme.SyntaxTypeColor = lipgloss.Color(darkCyan)
	theme.SyntaxOperatorColor = lipgloss.Color(darkPink)
	theme.SyntaxPunctuationColor = lipgloss.Color(darkForeground)

	return theme
}

func init() {
	// Register the Dracula theme with the theme manager
	RegisterTheme("dracula", NewDraculaTheme())
}