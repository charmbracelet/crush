package theme

import (
	catppuccin "github.com/catppuccin/go"
	"github.com/charmbracelet/lipgloss/v2"
)

// CatppuccinTheme implements the Theme interface with Catppuccin colors.
// It provides both dark (Mocha) and light (Latte) variants.
type CatppuccinTheme struct {
	BaseTheme
}

// NewCatppuccinMochaTheme creates a new instance of the Catppuccin Mocha theme.
func NewCatppuccinMochaTheme() *CatppuccinTheme {
	// Get the Catppuccin palette
	mocha := catppuccin.Mocha

	theme := &CatppuccinTheme{}

	// Base colors
	theme.PrimaryColor = lipgloss.Color(mocha.Blue().Hex)
	theme.SecondaryColor = lipgloss.Color(mocha.Mauve().Hex)
	theme.AccentColor = lipgloss.Color(mocha.Peach().Hex)

	// Status colors
	theme.ErrorColor = lipgloss.Color(mocha.Red().Hex)
	theme.WarningColor = lipgloss.Color(mocha.Peach().Hex)
	theme.SuccessColor = lipgloss.Color(mocha.Green().Hex)
	theme.InfoColor = lipgloss.Color(mocha.Blue().Hex)

	// Text colors
	theme.TextColor = lipgloss.Color(mocha.Text().Hex)
	theme.TextMutedColor = lipgloss.Color(mocha.Subtext0().Hex)
	theme.TextEmphasizedColor = lipgloss.Color(mocha.Lavender().Hex)

	// Background colors
	theme.BackgroundColor = lipgloss.Color("#212121")          // From existing styles
	theme.BackgroundSecondaryColor = lipgloss.Color("#2c2c2c") // From existing styles
	theme.BackgroundDarkerColor = lipgloss.Color("#181818")    // From existing styles

	// Border colors
	theme.BorderNormalColor = lipgloss.Color("#4b4c5c") // From existing styles
	theme.BorderFocusedColor = lipgloss.Color(mocha.Blue().Hex)
	theme.BorderDimColor = lipgloss.Color(mocha.Surface0().Hex)

	// Diff view colors
	theme.DiffAddedColor = lipgloss.Color("#478247")               // From existing diff.go
	theme.DiffRemovedColor = lipgloss.Color("#7C4444")             // From existing diff.go
	theme.DiffContextColor = lipgloss.Color("#a0a0a0")             // From existing diff.go
	theme.DiffHunkHeaderColor = lipgloss.Color("#a0a0a0")          // From existing diff.go
	theme.DiffHighlightAddedColor = lipgloss.Color("#DAFADA")      // From existing diff.go
	theme.DiffHighlightRemovedColor = lipgloss.Color("#FADADD")    // From existing diff.go
	theme.DiffAddedBgColor = lipgloss.Color("#303A30")             // From existing diff.go
	theme.DiffRemovedBgColor = lipgloss.Color("#3A3030")           // From existing diff.go
	theme.DiffContextBgColor = lipgloss.Color("#212121")           // From existing diff.go
	theme.DiffLineNumberColor = lipgloss.Color("#888888")          // From existing diff.go
	theme.DiffAddedLineNumberBgColor = lipgloss.Color("#293229")   // From existing diff.go
	theme.DiffRemovedLineNumberBgColor = lipgloss.Color("#332929") // From existing diff.go

	// Markdown colors
	theme.MarkdownTextColor = lipgloss.Color(mocha.Text().Hex)
	theme.MarkdownHeadingColor = lipgloss.Color(mocha.Mauve().Hex)
	theme.MarkdownLinkColor = lipgloss.Color(mocha.Sky().Hex)
	theme.MarkdownLinkTextColor = lipgloss.Color(mocha.Pink().Hex)
	theme.MarkdownCodeColor = lipgloss.Color(mocha.Green().Hex)
	theme.MarkdownBlockQuoteColor = lipgloss.Color(mocha.Yellow().Hex)
	theme.MarkdownEmphColor = lipgloss.Color(mocha.Yellow().Hex)
	theme.MarkdownStrongColor = lipgloss.Color(mocha.Peach().Hex)
	theme.MarkdownHorizontalRuleColor = lipgloss.Color(mocha.Overlay0().Hex)
	theme.MarkdownListItemColor = lipgloss.Color(mocha.Blue().Hex)
	theme.MarkdownListEnumerationColor = lipgloss.Color(mocha.Sky().Hex)
	theme.MarkdownImageColor = lipgloss.Color(mocha.Sapphire().Hex)
	theme.MarkdownImageTextColor = lipgloss.Color(mocha.Pink().Hex)
	theme.MarkdownCodeBlockColor = lipgloss.Color(mocha.Text().Hex)

	// Syntax highlighting colors
	theme.SyntaxCommentColor = lipgloss.Color(mocha.Overlay1().Hex)
	theme.SyntaxKeywordColor = lipgloss.Color(mocha.Pink().Hex)
	theme.SyntaxFunctionColor = lipgloss.Color(mocha.Green().Hex)
	theme.SyntaxVariableColor = lipgloss.Color(mocha.Sky().Hex)
	theme.SyntaxStringColor = lipgloss.Color(mocha.Yellow().Hex)
	theme.SyntaxNumberColor = lipgloss.Color(mocha.Teal().Hex)
	theme.SyntaxTypeColor = lipgloss.Color(mocha.Sky().Hex)
	theme.SyntaxOperatorColor = lipgloss.Color(mocha.Pink().Hex)
	theme.SyntaxPunctuationColor = lipgloss.Color(mocha.Text().Hex)

	return theme
}

// NewCatppuccinLatteTheme creates a new instance of the Catppuccin Latte theme.
func NewCatppuccinLatteTheme() *CatppuccinTheme {
	// Get the Catppuccin palette
	latte := catppuccin.Latte

	theme := &CatppuccinTheme{}

	// Base colors
	theme.PrimaryColor = lipgloss.Color(latte.Blue().Hex)
	theme.SecondaryColor = lipgloss.Color(latte.Mauve().Hex)
	theme.AccentColor = lipgloss.Color(latte.Peach().Hex)

	// Status colors
	theme.ErrorColor = lipgloss.Color(latte.Red().Hex)
	theme.WarningColor = lipgloss.Color(latte.Peach().Hex)
	theme.SuccessColor = lipgloss.Color(latte.Green().Hex)
	theme.InfoColor = lipgloss.Color(latte.Blue().Hex)

	// Text colors
	theme.TextColor = lipgloss.Color(latte.Text().Hex)
	theme.TextMutedColor = lipgloss.Color(latte.Subtext0().Hex)
	theme.TextEmphasizedColor = lipgloss.Color(latte.Lavender().Hex)

	// Background colors
	theme.BackgroundColor = lipgloss.Color("#EEEEEE")          // Light equivalent
	theme.BackgroundSecondaryColor = lipgloss.Color("#E0E0E0") // Light equivalent
	theme.BackgroundDarkerColor = lipgloss.Color("#F5F5F5")    // Light equivalent

	// Border colors
	theme.BorderNormalColor = lipgloss.Color("#BDBDBD") // Light equivalent
	theme.BorderFocusedColor = lipgloss.Color(latte.Blue().Hex)
	theme.BorderDimColor = lipgloss.Color(latte.Surface0().Hex)

	// Diff view colors
	theme.DiffAddedColor = lipgloss.Color("#2E7D32")               // Light equivalent
	theme.DiffRemovedColor = lipgloss.Color("#C62828")             // Light equivalent
	theme.DiffContextColor = lipgloss.Color("#757575")             // Light equivalent
	theme.DiffHunkHeaderColor = lipgloss.Color("#757575")          // Light equivalent
	theme.DiffHighlightAddedColor = lipgloss.Color("#A5D6A7")      // Light equivalent
	theme.DiffHighlightRemovedColor = lipgloss.Color("#EF9A9A")    // Light equivalent
	theme.DiffAddedBgColor = lipgloss.Color("#E8F5E9")             // Light equivalent
	theme.DiffRemovedBgColor = lipgloss.Color("#FFEBEE")           // Light equivalent
	theme.DiffContextBgColor = lipgloss.Color("#F5F5F5")           // Light equivalent
	theme.DiffLineNumberColor = lipgloss.Color("#9E9E9E")          // Light equivalent
	theme.DiffAddedLineNumberBgColor = lipgloss.Color("#C8E6C9")   // Light equivalent
	theme.DiffRemovedLineNumberBgColor = lipgloss.Color("#FFCDD2") // Light equivalent

	// Markdown colors
	theme.MarkdownTextColor = lipgloss.Color(latte.Text().Hex)
	theme.MarkdownHeadingColor = lipgloss.Color(latte.Mauve().Hex)
	theme.MarkdownLinkColor = lipgloss.Color(latte.Sky().Hex)
	theme.MarkdownLinkTextColor = lipgloss.Color(latte.Pink().Hex)
	theme.MarkdownCodeColor = lipgloss.Color(latte.Green().Hex)
	theme.MarkdownBlockQuoteColor = lipgloss.Color(latte.Yellow().Hex)
	theme.MarkdownEmphColor = lipgloss.Color(latte.Yellow().Hex)
	theme.MarkdownStrongColor = lipgloss.Color(latte.Peach().Hex)
	theme.MarkdownHorizontalRuleColor = lipgloss.Color(latte.Overlay0().Hex)
	theme.MarkdownListItemColor = lipgloss.Color(latte.Blue().Hex)
	theme.MarkdownListEnumerationColor = lipgloss.Color(latte.Sky().Hex)
	theme.MarkdownImageColor = lipgloss.Color(latte.Sapphire().Hex)
	theme.MarkdownImageTextColor = lipgloss.Color(latte.Pink().Hex)
	theme.MarkdownCodeBlockColor = lipgloss.Color(latte.Text().Hex)

	// Syntax highlighting colors
	theme.SyntaxCommentColor = lipgloss.Color(latte.Overlay1().Hex)
	theme.SyntaxKeywordColor = lipgloss.Color(latte.Pink().Hex)
	theme.SyntaxFunctionColor = lipgloss.Color(latte.Green().Hex)
	theme.SyntaxVariableColor = lipgloss.Color(latte.Sky().Hex)
	theme.SyntaxStringColor = lipgloss.Color(latte.Yellow().Hex)
	theme.SyntaxNumberColor = lipgloss.Color(latte.Teal().Hex)
	theme.SyntaxTypeColor = lipgloss.Color(latte.Sky().Hex)
	theme.SyntaxOperatorColor = lipgloss.Color(latte.Pink().Hex)
	theme.SyntaxPunctuationColor = lipgloss.Color(latte.Text().Hex)

	return theme
}

func init() {
	// Register the Catppuccin themes with the theme manager
	RegisterTheme("catppuccin-mocha", NewCatppuccinMochaTheme())
	RegisterTheme("catppuccin-latte", NewCatppuccinLatteTheme())
}
