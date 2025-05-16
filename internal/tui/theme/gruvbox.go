package theme

import (
	"github.com/charmbracelet/lipgloss/v2"
)

// Gruvbox color palette constants
const (
	// Dark theme colors
	gruvboxDarkBg0          = "#282828"
	gruvboxDarkBg0Soft      = "#32302f"
	gruvboxDarkBg1          = "#3c3836"
	gruvboxDarkBg2          = "#504945"
	gruvboxDarkBg3          = "#665c54"
	gruvboxDarkBg4          = "#7c6f64"
	gruvboxDarkFg0          = "#fbf1c7"
	gruvboxDarkFg1          = "#ebdbb2"
	gruvboxDarkFg2          = "#d5c4a1"
	gruvboxDarkFg3          = "#bdae93"
	gruvboxDarkFg4          = "#a89984"
	gruvboxDarkGray         = "#928374"
	gruvboxDarkRed          = "#cc241d"
	gruvboxDarkRedBright    = "#fb4934"
	gruvboxDarkGreen        = "#98971a"
	gruvboxDarkGreenBright  = "#b8bb26"
	gruvboxDarkYellow       = "#d79921"
	gruvboxDarkYellowBright = "#fabd2f"
	gruvboxDarkBlue         = "#458588"
	gruvboxDarkBlueBright   = "#83a598"
	gruvboxDarkPurple       = "#b16286"
	gruvboxDarkPurpleBright = "#d3869b"
	gruvboxDarkAqua         = "#689d6a"
	gruvboxDarkAquaBright   = "#8ec07c"
	gruvboxDarkOrange       = "#d65d0e"
	gruvboxDarkOrangeBright = "#fe8019"

	// Light theme colors
	gruvboxLightBg0          = "#fbf1c7"
	gruvboxLightBg0Soft      = "#f2e5bc"
	gruvboxLightBg1          = "#ebdbb2"
	gruvboxLightBg2          = "#d5c4a1"
	gruvboxLightBg3          = "#bdae93"
	gruvboxLightBg4          = "#a89984"
	gruvboxLightFg0          = "#282828"
	gruvboxLightFg1          = "#3c3836"
	gruvboxLightFg2          = "#504945"
	gruvboxLightFg3          = "#665c54"
	gruvboxLightFg4          = "#7c6f64"
	gruvboxLightGray         = "#928374"
	gruvboxLightRed          = "#9d0006"
	gruvboxLightRedBright    = "#cc241d"
	gruvboxLightGreen        = "#79740e"
	gruvboxLightGreenBright  = "#98971a"
	gruvboxLightYellow       = "#b57614"
	gruvboxLightYellowBright = "#d79921"
	gruvboxLightBlue         = "#076678"
	gruvboxLightBlueBright   = "#458588"
	gruvboxLightPurple       = "#8f3f71"
	gruvboxLightPurpleBright = "#b16286"
	gruvboxLightAqua         = "#427b58"
	gruvboxLightAquaBright   = "#689d6a"
	gruvboxLightOrange       = "#af3a03"
	gruvboxLightOrangeBright = "#d65d0e"
)

// GruvboxTheme implements the Theme interface with Gruvbox colors.
// It provides both dark and light variants.
type GruvboxTheme struct {
	BaseTheme
}

// NewGruvboxTheme creates a new instance of the Gruvbox theme.
func NewGruvboxTheme() *GruvboxTheme {
	theme := &GruvboxTheme{}

	// Base colors
	theme.PrimaryColor = lipgloss.Color(gruvboxDarkBlueBright)
	theme.SecondaryColor = lipgloss.Color(gruvboxDarkPurpleBright)
	theme.AccentColor = lipgloss.Color(gruvboxDarkOrangeBright)

	// Status colors
	theme.ErrorColor = lipgloss.Color(gruvboxDarkRedBright)
	theme.WarningColor = lipgloss.Color(gruvboxDarkYellowBright)
	theme.SuccessColor = lipgloss.Color(gruvboxDarkGreenBright)
	theme.InfoColor = lipgloss.Color(gruvboxDarkBlueBright)

	// Text colors
	theme.TextColor = lipgloss.Color(gruvboxDarkFg1)
	theme.TextMutedColor = lipgloss.Color(gruvboxDarkFg4)
	theme.TextEmphasizedColor = lipgloss.Color(gruvboxDarkYellowBright)

	// Background colors
	theme.BackgroundColor = lipgloss.Color(gruvboxDarkBg0)
	theme.BackgroundSecondaryColor = lipgloss.Color(gruvboxDarkBg1)
	theme.BackgroundDarkerColor = lipgloss.Color(gruvboxDarkBg0Soft)

	// Border colors
	theme.BorderNormalColor = lipgloss.Color(gruvboxDarkBg2)
	theme.BorderFocusedColor = lipgloss.Color(gruvboxDarkBlueBright)
	theme.BorderDimColor = lipgloss.Color(gruvboxDarkBg1)

	// Diff view colors
	theme.DiffAddedColor = lipgloss.Color(gruvboxDarkGreenBright)
	theme.DiffRemovedColor = lipgloss.Color(gruvboxDarkRedBright)
	theme.DiffContextColor = lipgloss.Color(gruvboxDarkFg4)
	theme.DiffHunkHeaderColor = lipgloss.Color(gruvboxDarkFg3)
	theme.DiffHighlightAddedColor = lipgloss.Color(gruvboxDarkGreenBright)
	theme.DiffHighlightRemovedColor = lipgloss.Color(gruvboxDarkRedBright)
	theme.DiffAddedBgColor = lipgloss.Color("#3C4C3C")  // Darker green background
	theme.DiffRemovedBgColor = lipgloss.Color("#4C3C3C")  // Darker red background
	theme.DiffContextBgColor = lipgloss.Color(gruvboxDarkBg0)
	theme.DiffLineNumberColor = lipgloss.Color(gruvboxDarkFg4)
	theme.DiffAddedLineNumberBgColor = lipgloss.Color("#32432F")   // Slightly darker green
	theme.DiffRemovedLineNumberBgColor = lipgloss.Color("#43322F")   // Slightly darker red

	// Markdown colors
	theme.MarkdownTextColor = lipgloss.Color(gruvboxDarkFg1)
	theme.MarkdownHeadingColor = lipgloss.Color(gruvboxDarkYellowBright)
	theme.MarkdownLinkColor = lipgloss.Color(gruvboxDarkBlueBright)
	theme.MarkdownLinkTextColor = lipgloss.Color(gruvboxDarkAquaBright)
	theme.MarkdownCodeColor = lipgloss.Color(gruvboxDarkGreenBright)
	theme.MarkdownBlockQuoteColor = lipgloss.Color(gruvboxDarkAquaBright)
	theme.MarkdownEmphColor = lipgloss.Color(gruvboxDarkYellowBright)
	theme.MarkdownStrongColor = lipgloss.Color(gruvboxDarkOrangeBright)
	theme.MarkdownHorizontalRuleColor = lipgloss.Color(gruvboxDarkBg3)
	theme.MarkdownListItemColor = lipgloss.Color(gruvboxDarkBlueBright)
	theme.MarkdownListEnumerationColor = lipgloss.Color(gruvboxDarkBlueBright)
	theme.MarkdownImageColor = lipgloss.Color(gruvboxDarkPurpleBright)
	theme.MarkdownImageTextColor = lipgloss.Color(gruvboxDarkAquaBright)
	theme.MarkdownCodeBlockColor = lipgloss.Color(gruvboxDarkFg1)

	// Syntax highlighting colors
	theme.SyntaxCommentColor = lipgloss.Color(gruvboxDarkGray)
	theme.SyntaxKeywordColor = lipgloss.Color(gruvboxDarkRedBright)
	theme.SyntaxFunctionColor = lipgloss.Color(gruvboxDarkGreenBright)
	theme.SyntaxVariableColor = lipgloss.Color(gruvboxDarkBlueBright)
	theme.SyntaxStringColor = lipgloss.Color(gruvboxDarkYellowBright)
	theme.SyntaxNumberColor = lipgloss.Color(gruvboxDarkPurpleBright)
	theme.SyntaxTypeColor = lipgloss.Color(gruvboxDarkYellow)
	theme.SyntaxOperatorColor = lipgloss.Color(gruvboxDarkAquaBright)
	theme.SyntaxPunctuationColor = lipgloss.Color(gruvboxDarkFg1)

	return theme
}

// NewGruvboxLightTheme creates a new instance of the Gruvbox Light theme.
func NewGruvboxLightTheme() *GruvboxTheme {
	theme := &GruvboxTheme{}

	// Base colors
	theme.PrimaryColor = lipgloss.Color(gruvboxLightBlueBright)
	theme.SecondaryColor = lipgloss.Color(gruvboxLightPurpleBright)
	theme.AccentColor = lipgloss.Color(gruvboxLightOrangeBright)

	// Status colors
	theme.ErrorColor = lipgloss.Color(gruvboxLightRedBright)
	theme.WarningColor = lipgloss.Color(gruvboxLightYellowBright)
	theme.SuccessColor = lipgloss.Color(gruvboxLightGreenBright)
	theme.InfoColor = lipgloss.Color(gruvboxLightBlueBright)

	// Text colors
	theme.TextColor = lipgloss.Color(gruvboxLightFg1)
	theme.TextMutedColor = lipgloss.Color(gruvboxLightFg4)
	theme.TextEmphasizedColor = lipgloss.Color(gruvboxLightYellowBright)

	// Background colors
	theme.BackgroundColor = lipgloss.Color(gruvboxLightBg0)
	theme.BackgroundSecondaryColor = lipgloss.Color(gruvboxLightBg1)
	theme.BackgroundDarkerColor = lipgloss.Color(gruvboxLightBg0Soft)

	// Border colors
	theme.BorderNormalColor = lipgloss.Color(gruvboxLightBg2)
	theme.BorderFocusedColor = lipgloss.Color(gruvboxLightBlueBright)
	theme.BorderDimColor = lipgloss.Color(gruvboxLightBg1)

	// Diff view colors
	theme.DiffAddedColor = lipgloss.Color(gruvboxLightGreenBright)
	theme.DiffRemovedColor = lipgloss.Color(gruvboxLightRedBright)
	theme.DiffContextColor = lipgloss.Color(gruvboxLightFg4)
	theme.DiffHunkHeaderColor = lipgloss.Color(gruvboxLightFg3)
	theme.DiffHighlightAddedColor = lipgloss.Color(gruvboxLightGreenBright)
	theme.DiffHighlightRemovedColor = lipgloss.Color(gruvboxLightRedBright)
	theme.DiffAddedBgColor = lipgloss.Color("#E8F5E9") // Light green background
	theme.DiffRemovedBgColor = lipgloss.Color("#FFEBEE") // Light red background
	theme.DiffContextBgColor = lipgloss.Color(gruvboxLightBg0)
	theme.DiffLineNumberColor = lipgloss.Color(gruvboxLightFg4)
	theme.DiffAddedLineNumberBgColor = lipgloss.Color("#C8E6C9") // Light green
	theme.DiffRemovedLineNumberBgColor = lipgloss.Color("#FFCDD2") // Light red

	// Markdown colors
	theme.MarkdownTextColor = lipgloss.Color(gruvboxLightFg1)
	theme.MarkdownHeadingColor = lipgloss.Color(gruvboxLightYellowBright)
	theme.MarkdownLinkColor = lipgloss.Color(gruvboxLightBlueBright)
	theme.MarkdownLinkTextColor = lipgloss.Color(gruvboxLightAquaBright)
	theme.MarkdownCodeColor = lipgloss.Color(gruvboxLightGreenBright)
	theme.MarkdownBlockQuoteColor = lipgloss.Color(gruvboxLightAquaBright)
	theme.MarkdownEmphColor = lipgloss.Color(gruvboxLightYellowBright)
	theme.MarkdownStrongColor = lipgloss.Color(gruvboxLightOrangeBright)
	theme.MarkdownHorizontalRuleColor = lipgloss.Color(gruvboxLightBg3)
	theme.MarkdownListItemColor = lipgloss.Color(gruvboxLightBlueBright)
	theme.MarkdownListEnumerationColor = lipgloss.Color(gruvboxLightBlueBright)
	theme.MarkdownImageColor = lipgloss.Color(gruvboxLightPurpleBright)
	theme.MarkdownImageTextColor = lipgloss.Color(gruvboxLightAquaBright)
	theme.MarkdownCodeBlockColor = lipgloss.Color(gruvboxLightFg1)

	// Syntax highlighting colors
	theme.SyntaxCommentColor = lipgloss.Color(gruvboxLightGray)
	theme.SyntaxKeywordColor = lipgloss.Color(gruvboxLightRedBright)
	theme.SyntaxFunctionColor = lipgloss.Color(gruvboxLightGreenBright)
	theme.SyntaxVariableColor = lipgloss.Color(gruvboxLightBlueBright)
	theme.SyntaxStringColor = lipgloss.Color(gruvboxLightYellowBright)
	theme.SyntaxNumberColor = lipgloss.Color(gruvboxLightPurpleBright)
	theme.SyntaxTypeColor = lipgloss.Color(gruvboxLightYellow)
	theme.SyntaxOperatorColor = lipgloss.Color(gruvboxLightAquaBright)
	theme.SyntaxPunctuationColor = lipgloss.Color(gruvboxLightFg1)

	return theme
}

func init() {
	// Register the Gruvbox themes with the theme manager
	RegisterTheme("gruvbox", NewGruvboxTheme())
	RegisterTheme("gruvbox-light", NewGruvboxLightTheme())
}