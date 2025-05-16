package theme

import (
	"image/color"
)

type Theme interface {
	// Base colors
	Primary() color.Color
	Secondary() color.Color
	Accent() color.Color

	// Status colors
	Error() color.Color
	Warning() color.Color
	Success() color.Color
	Info() color.Color

	// Text colors
	Text() color.Color
	TextMuted() color.Color
	TextEmphasized() color.Color

	// Background colors
	Background() color.Color
	BackgroundSecondary() color.Color
	BackgroundDarker() color.Color

	// Border colors
	BorderNormal() color.Color
	BorderFocused() color.Color
	BorderDim() color.Color

	// Diff view colors
	DiffAdded() color.Color
	DiffRemoved() color.Color
	DiffContext() color.Color
	DiffHunkHeader() color.Color
	DiffHighlightAdded() color.Color
	DiffHighlightRemoved() color.Color
	DiffAddedBg() color.Color
	DiffRemovedBg() color.Color
	DiffContextBg() color.Color
	DiffLineNumber() color.Color
	DiffAddedLineNumberBg() color.Color
	DiffRemovedLineNumberBg() color.Color

	// Markdown colors
	MarkdownText() color.Color
	MarkdownHeading() color.Color
	MarkdownLink() color.Color
	MarkdownLinkText() color.Color
	MarkdownCode() color.Color
	MarkdownBlockQuote() color.Color
	MarkdownEmph() color.Color
	MarkdownStrong() color.Color
	MarkdownHorizontalRule() color.Color
	MarkdownListItem() color.Color
	MarkdownListEnumeration() color.Color
	MarkdownImage() color.Color
	MarkdownImageText() color.Color
	MarkdownCodeBlock() color.Color

	// Syntax highlighting colors
	SyntaxComment() color.Color
	SyntaxKeyword() color.Color
	SyntaxFunction() color.Color
	SyntaxVariable() color.Color
	SyntaxString() color.Color
	SyntaxNumber() color.Color
	SyntaxType() color.Color
	SyntaxOperator() color.Color
	SyntaxPunctuation() color.Color
}

// BaseTheme provides a default implementation of the Theme interface
// that can be embedded in concrete theme implementations.
type BaseTheme struct {
	// Base colors
	PrimaryColor   color.Color
	SecondaryColor color.Color
	AccentColor    color.Color

	// Status colors
	ErrorColor   color.Color
	WarningColor color.Color
	SuccessColor color.Color
	InfoColor    color.Color

	// Text colors
	TextColor           color.Color
	TextMutedColor      color.Color
	TextEmphasizedColor color.Color

	// Background colors
	BackgroundColor          color.Color
	BackgroundSecondaryColor color.Color
	BackgroundDarkerColor    color.Color

	// Border colors
	BorderNormalColor  color.Color
	BorderFocusedColor color.Color
	BorderDimColor     color.Color

	// Diff view colors
	DiffAddedColor               color.Color
	DiffRemovedColor             color.Color
	DiffContextColor             color.Color
	DiffHunkHeaderColor          color.Color
	DiffHighlightAddedColor      color.Color
	DiffHighlightRemovedColor    color.Color
	DiffAddedBgColor             color.Color
	DiffRemovedBgColor           color.Color
	DiffContextBgColor           color.Color
	DiffLineNumberColor          color.Color
	DiffAddedLineNumberBgColor   color.Color
	DiffRemovedLineNumberBgColor color.Color

	// Markdown colors
	MarkdownTextColor            color.Color
	MarkdownHeadingColor         color.Color
	MarkdownLinkColor            color.Color
	MarkdownLinkTextColor        color.Color
	MarkdownCodeColor            color.Color
	MarkdownBlockQuoteColor      color.Color
	MarkdownEmphColor            color.Color
	MarkdownStrongColor          color.Color
	MarkdownHorizontalRuleColor  color.Color
	MarkdownListItemColor        color.Color
	MarkdownListEnumerationColor color.Color
	MarkdownImageColor           color.Color
	MarkdownImageTextColor       color.Color
	MarkdownCodeBlockColor       color.Color

	// Syntax highlighting colors
	SyntaxCommentColor     color.Color
	SyntaxKeywordColor     color.Color
	SyntaxFunctionColor    color.Color
	SyntaxVariableColor    color.Color
	SyntaxStringColor      color.Color
	SyntaxNumberColor      color.Color
	SyntaxTypeColor        color.Color
	SyntaxOperatorColor    color.Color
	SyntaxPunctuationColor color.Color
}

// Implement the Theme interface for BaseTheme
func (t *BaseTheme) Primary() color.Color   { return t.PrimaryColor }
func (t *BaseTheme) Secondary() color.Color { return t.SecondaryColor }
func (t *BaseTheme) Accent() color.Color    { return t.AccentColor }

func (t *BaseTheme) Error() color.Color   { return t.ErrorColor }
func (t *BaseTheme) Warning() color.Color { return t.WarningColor }
func (t *BaseTheme) Success() color.Color { return t.SuccessColor }
func (t *BaseTheme) Info() color.Color    { return t.InfoColor }

func (t *BaseTheme) Text() color.Color           { return t.TextColor }
func (t *BaseTheme) TextMuted() color.Color      { return t.TextMutedColor }
func (t *BaseTheme) TextEmphasized() color.Color { return t.TextEmphasizedColor }

func (t *BaseTheme) Background() color.Color          { return t.BackgroundColor }
func (t *BaseTheme) BackgroundSecondary() color.Color { return t.BackgroundSecondaryColor }
func (t *BaseTheme) BackgroundDarker() color.Color    { return t.BackgroundDarkerColor }

func (t *BaseTheme) BorderNormal() color.Color  { return t.BorderNormalColor }
func (t *BaseTheme) BorderFocused() color.Color { return t.BorderFocusedColor }
func (t *BaseTheme) BorderDim() color.Color     { return t.BorderDimColor }

func (t *BaseTheme) DiffAdded() color.Color               { return t.DiffAddedColor }
func (t *BaseTheme) DiffRemoved() color.Color             { return t.DiffRemovedColor }
func (t *BaseTheme) DiffContext() color.Color             { return t.DiffContextColor }
func (t *BaseTheme) DiffHunkHeader() color.Color          { return t.DiffHunkHeaderColor }
func (t *BaseTheme) DiffHighlightAdded() color.Color      { return t.DiffHighlightAddedColor }
func (t *BaseTheme) DiffHighlightRemoved() color.Color    { return t.DiffHighlightRemovedColor }
func (t *BaseTheme) DiffAddedBg() color.Color             { return t.DiffAddedBgColor }
func (t *BaseTheme) DiffRemovedBg() color.Color           { return t.DiffRemovedBgColor }
func (t *BaseTheme) DiffContextBg() color.Color           { return t.DiffContextBgColor }
func (t *BaseTheme) DiffLineNumber() color.Color          { return t.DiffLineNumberColor }
func (t *BaseTheme) DiffAddedLineNumberBg() color.Color   { return t.DiffAddedLineNumberBgColor }
func (t *BaseTheme) DiffRemovedLineNumberBg() color.Color { return t.DiffRemovedLineNumberBgColor }

func (t *BaseTheme) MarkdownText() color.Color            { return t.MarkdownTextColor }
func (t *BaseTheme) MarkdownHeading() color.Color         { return t.MarkdownHeadingColor }
func (t *BaseTheme) MarkdownLink() color.Color            { return t.MarkdownLinkColor }
func (t *BaseTheme) MarkdownLinkText() color.Color        { return t.MarkdownLinkTextColor }
func (t *BaseTheme) MarkdownCode() color.Color            { return t.MarkdownCodeColor }
func (t *BaseTheme) MarkdownBlockQuote() color.Color      { return t.MarkdownBlockQuoteColor }
func (t *BaseTheme) MarkdownEmph() color.Color            { return t.MarkdownEmphColor }
func (t *BaseTheme) MarkdownStrong() color.Color          { return t.MarkdownStrongColor }
func (t *BaseTheme) MarkdownHorizontalRule() color.Color  { return t.MarkdownHorizontalRuleColor }
func (t *BaseTheme) MarkdownListItem() color.Color        { return t.MarkdownListItemColor }
func (t *BaseTheme) MarkdownListEnumeration() color.Color { return t.MarkdownListEnumerationColor }
func (t *BaseTheme) MarkdownImage() color.Color           { return t.MarkdownImageColor }
func (t *BaseTheme) MarkdownImageText() color.Color       { return t.MarkdownImageTextColor }
func (t *BaseTheme) MarkdownCodeBlock() color.Color       { return t.MarkdownCodeBlockColor }

func (t *BaseTheme) SyntaxComment() color.Color     { return t.SyntaxCommentColor }
func (t *BaseTheme) SyntaxKeyword() color.Color     { return t.SyntaxKeywordColor }
func (t *BaseTheme) SyntaxFunction() color.Color    { return t.SyntaxFunctionColor }
func (t *BaseTheme) SyntaxVariable() color.Color    { return t.SyntaxVariableColor }
func (t *BaseTheme) SyntaxString() color.Color      { return t.SyntaxStringColor }
func (t *BaseTheme) SyntaxNumber() color.Color      { return t.SyntaxNumberColor }
func (t *BaseTheme) SyntaxType() color.Color        { return t.SyntaxTypeColor }
func (t *BaseTheme) SyntaxOperator() color.Color    { return t.SyntaxOperatorColor }
func (t *BaseTheme) SyntaxPunctuation() color.Color { return t.SyntaxPunctuationColor }
