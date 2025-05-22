package theme

import (
	"github.com/charmbracelet/lipgloss/v2"
)

// Flexoki color palette constants
const (
	// Base colors
	flexokiPaper   = "#FFFCF0" // Paper (lightest)
	flexokiBase50  = "#F2F0E5" // bg-2 (light)
	flexokiBase100 = "#E6E4D9" // ui (light)
	flexokiBase150 = "#DAD8CE" // ui-2 (light)
	flexokiBase200 = "#CECDC3" // ui-3 (light)
	flexokiBase300 = "#B7B5AC" // tx-3 (light)
	flexokiBase500 = "#878580" // tx-2 (light)
	flexokiBase600 = "#6F6E69" // tx (light)
	flexokiBase700 = "#575653" // tx-3 (dark)
	flexokiBase800 = "#403E3C" // ui-3 (dark)
	flexokiBase850 = "#343331" // ui-2 (dark)
	flexokiBase900 = "#282726" // ui (dark)
	flexokiBase950 = "#1C1B1A" // bg-2 (dark)
	flexokiBlack   = "#100F0F" // bg (darkest)

	// Accent colors - Light theme (600)
	flexokiRed600     = "#AF3029"
	flexokiOrange600  = "#BC5215"
	flexokiYellow600  = "#AD8301"
	flexokiGreen600   = "#66800B"
	flexokiCyan600    = "#24837B"
	flexokiBlue600    = "#205EA6"
	flexokiPurple600  = "#5E409D"
	flexokiMagenta600 = "#A02F6F"

	// Accent colors - Dark theme (400)
	flexokiRed400     = "#D14D41"
	flexokiOrange400  = "#DA702C"
	flexokiYellow400  = "#D0A215"
	flexokiGreen400   = "#879A39"
	flexokiCyan400    = "#3AA99F"
	flexokiBlue400    = "#4385BE"
	flexokiPurple400  = "#8B7EC8"
	flexokiMagenta400 = "#CE5D97"
)

// FlexokiTheme implements the Theme interface with Flexoki colors.
// It provides both dark and light variants.
type FlexokiTheme struct {
	BaseTheme
}

// NewFlexokiDarkTheme creates a new instance of the Flexoki Dark theme.
func NewFlexokiDarkTheme() *FlexokiTheme {
	theme := &FlexokiTheme{}

	// Base colors
	theme.PrimaryColor = lipgloss.Color(flexokiBlue400)
	theme.SecondaryColor = lipgloss.Color(flexokiPurple400)
	theme.AccentColor = lipgloss.Color(flexokiOrange400)

	// Status colors
	theme.ErrorColor = lipgloss.Color(flexokiRed400)
	theme.WarningColor = lipgloss.Color(flexokiYellow400)
	theme.SuccessColor = lipgloss.Color(flexokiGreen400)
	theme.InfoColor = lipgloss.Color(flexokiCyan400)

	// Text colors
	theme.TextColor = lipgloss.Color(flexokiBase300)
	theme.TextMutedColor = lipgloss.Color(flexokiBase700)
	theme.TextEmphasizedColor = lipgloss.Color(flexokiYellow400)

	// Background colors
	theme.BackgroundColor = lipgloss.Color(flexokiBlack)
	theme.BackgroundSecondaryColor = lipgloss.Color(flexokiBase950)
	theme.BackgroundDarkerColor = lipgloss.Color(flexokiBase900)

	// Border colors
	theme.BorderNormalColor = lipgloss.Color(flexokiBase900)
	theme.BorderFocusedColor = lipgloss.Color(flexokiBlue400)
	theme.BorderDimColor = lipgloss.Color(flexokiBase850)

	// Diff view colors
	theme.DiffAddedColor = lipgloss.Color(flexokiGreen400)
	theme.DiffRemovedColor = lipgloss.Color(flexokiRed400)
	theme.DiffContextColor = lipgloss.Color(flexokiBase700)
	theme.DiffHunkHeaderColor = lipgloss.Color(flexokiBase700)
	theme.DiffHighlightAddedColor = lipgloss.Color(flexokiGreen400)
	theme.DiffHighlightRemovedColor = lipgloss.Color(flexokiRed400)
	theme.DiffAddedBgColor = lipgloss.Color("#1D2419")   // Darker green background
	theme.DiffRemovedBgColor = lipgloss.Color("#241919") // Darker red background
	theme.DiffContextBgColor = lipgloss.Color(flexokiBlack)
	theme.DiffLineNumberColor = lipgloss.Color(flexokiBase700)
	theme.DiffAddedLineNumberBgColor = lipgloss.Color("#1A2017")   // Slightly darker green
	theme.DiffRemovedLineNumberBgColor = lipgloss.Color("#201717") // Slightly darker red

	// Markdown colors
	theme.MarkdownTextColor = lipgloss.Color(flexokiBase300)
	theme.MarkdownHeadingColor = lipgloss.Color(flexokiYellow400)
	theme.MarkdownLinkColor = lipgloss.Color(flexokiCyan400)
	theme.MarkdownLinkTextColor = lipgloss.Color(flexokiMagenta400)
	theme.MarkdownCodeColor = lipgloss.Color(flexokiGreen400)
	theme.MarkdownBlockQuoteColor = lipgloss.Color(flexokiCyan400)
	theme.MarkdownEmphColor = lipgloss.Color(flexokiYellow400)
	theme.MarkdownStrongColor = lipgloss.Color(flexokiOrange400)
	theme.MarkdownHorizontalRuleColor = lipgloss.Color(flexokiBase800)
	theme.MarkdownListItemColor = lipgloss.Color(flexokiBlue400)
	theme.MarkdownListEnumerationColor = lipgloss.Color(flexokiBlue400)
	theme.MarkdownImageColor = lipgloss.Color(flexokiPurple400)
	theme.MarkdownImageTextColor = lipgloss.Color(flexokiMagenta400)
	theme.MarkdownCodeBlockColor = lipgloss.Color(flexokiBase300)

	// Syntax highlighting colors (based on Flexoki's mappings)
	theme.SyntaxCommentColor = lipgloss.Color(flexokiBase700)     // tx-3
	theme.SyntaxKeywordColor = lipgloss.Color(flexokiGreen400)    // gr
	theme.SyntaxFunctionColor = lipgloss.Color(flexokiOrange400)  // or
	theme.SyntaxVariableColor = lipgloss.Color(flexokiBlue400)    // bl
	theme.SyntaxStringColor = lipgloss.Color(flexokiCyan400)      // cy
	theme.SyntaxNumberColor = lipgloss.Color(flexokiPurple400)    // pu
	theme.SyntaxTypeColor = lipgloss.Color(flexokiYellow400)      // ye
	theme.SyntaxOperatorColor = lipgloss.Color(flexokiBase500)    // tx-2
	theme.SyntaxPunctuationColor = lipgloss.Color(flexokiBase500) // tx-2

	return theme
}

// NewFlexokiLightTheme creates a new instance of the Flexoki Light theme.
func NewFlexokiLightTheme() *FlexokiTheme {
	theme := &FlexokiTheme{}

	// Base colors
	theme.PrimaryColor = lipgloss.Color(flexokiBlue600)
	theme.SecondaryColor = lipgloss.Color(flexokiPurple600)
	theme.AccentColor = lipgloss.Color(flexokiOrange600)

	// Status colors
	theme.ErrorColor = lipgloss.Color(flexokiRed600)
	theme.WarningColor = lipgloss.Color(flexokiYellow600)
	theme.SuccessColor = lipgloss.Color(flexokiGreen600)
	theme.InfoColor = lipgloss.Color(flexokiCyan600)

	// Text colors
	theme.TextColor = lipgloss.Color(flexokiBase600)
	theme.TextMutedColor = lipgloss.Color(flexokiBase500)
	theme.TextEmphasizedColor = lipgloss.Color(flexokiYellow600)

	// Background colors
	theme.BackgroundColor = lipgloss.Color(flexokiPaper)
	theme.BackgroundSecondaryColor = lipgloss.Color(flexokiBase50)
	theme.BackgroundDarkerColor = lipgloss.Color(flexokiBase100)

	// Border colors
	theme.BorderNormalColor = lipgloss.Color(flexokiBase100)
	theme.BorderFocusedColor = lipgloss.Color(flexokiBlue600)
	theme.BorderDimColor = lipgloss.Color(flexokiBase150)

	// Diff view colors
	theme.DiffAddedColor = lipgloss.Color(flexokiGreen600)
	theme.DiffRemovedColor = lipgloss.Color(flexokiRed600)
	theme.DiffContextColor = lipgloss.Color(flexokiBase500)
	theme.DiffHunkHeaderColor = lipgloss.Color(flexokiBase500)
	theme.DiffHighlightAddedColor = lipgloss.Color(flexokiGreen600)
	theme.DiffHighlightRemovedColor = lipgloss.Color(flexokiRed600)
	theme.DiffAddedBgColor = lipgloss.Color("#EFF2E2")   // Light green background
	theme.DiffRemovedBgColor = lipgloss.Color("#F2E2E2") // Light red background
	theme.DiffContextBgColor = lipgloss.Color(flexokiPaper)
	theme.DiffLineNumberColor = lipgloss.Color(flexokiBase500)
	theme.DiffAddedLineNumberBgColor = lipgloss.Color("#E5EBD9")   // Light green
	theme.DiffRemovedLineNumberBgColor = lipgloss.Color("#EBD9D9") // Light red

	// Markdown colors
	theme.MarkdownTextColor = lipgloss.Color(flexokiBase600)
	theme.MarkdownHeadingColor = lipgloss.Color(flexokiYellow600)
	theme.MarkdownLinkColor = lipgloss.Color(flexokiCyan600)
	theme.MarkdownLinkTextColor = lipgloss.Color(flexokiMagenta600)
	theme.MarkdownCodeColor = lipgloss.Color(flexokiGreen600)
	theme.MarkdownBlockQuoteColor = lipgloss.Color(flexokiCyan600)
	theme.MarkdownEmphColor = lipgloss.Color(flexokiYellow600)
	theme.MarkdownStrongColor = lipgloss.Color(flexokiOrange600)
	theme.MarkdownHorizontalRuleColor = lipgloss.Color(flexokiBase200)
	theme.MarkdownListItemColor = lipgloss.Color(flexokiBlue600)
	theme.MarkdownListEnumerationColor = lipgloss.Color(flexokiBlue600)
	theme.MarkdownImageColor = lipgloss.Color(flexokiPurple600)
	theme.MarkdownImageTextColor = lipgloss.Color(flexokiMagenta600)
	theme.MarkdownCodeBlockColor = lipgloss.Color(flexokiBase600)

	// Syntax highlighting colors (based on Flexoki's mappings)
	theme.SyntaxCommentColor = lipgloss.Color(flexokiBase300)     // tx-3
	theme.SyntaxKeywordColor = lipgloss.Color(flexokiGreen600)    // gr
	theme.SyntaxFunctionColor = lipgloss.Color(flexokiOrange600)  // or
	theme.SyntaxVariableColor = lipgloss.Color(flexokiBlue600)    // bl
	theme.SyntaxStringColor = lipgloss.Color(flexokiCyan600)      // cy
	theme.SyntaxNumberColor = lipgloss.Color(flexokiPurple600)    // pu
	theme.SyntaxTypeColor = lipgloss.Color(flexokiYellow600)      // ye
	theme.SyntaxOperatorColor = lipgloss.Color(flexokiBase500)    // tx-2
	theme.SyntaxPunctuationColor = lipgloss.Color(flexokiBase500) // tx-2

	return theme
}

func init() {
	// Register the Flexoki themes with the theme manager
	RegisterTheme("flexoki-dark", NewFlexokiDarkTheme())
	RegisterTheme("flexoki-light", NewFlexokiLightTheme())
}
