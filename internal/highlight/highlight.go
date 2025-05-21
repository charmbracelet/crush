package highlight

import (
	"bytes"
	"fmt"
	"image/color"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
)

func SyntaxHighlight(source, fileName string, bg color.Color) (string, error) {
	t := theme.CurrentTheme()

	// Determine the language lexer to use
	l := lexers.Match(fileName)
	if l == nil {
		l = lexers.Analyse(source)
	}
	if l == nil {
		l = lexers.Fallback
	}
	l = chroma.Coalesce(l)

	// Get the formatter
	f := formatters.Get("terminal16m")
	if f == nil {
		f = formatters.Fallback
	}

	// Dynamic theme based on current theme values
	syntaxThemeXml := fmt.Sprintf(`
	<style name="opencode-theme">
	<!-- Base colors -->
	<entry type="Text" style="%s"/>
	<entry type="Other" style="%s"/>
	<entry type="Error" style="%s"/>
	<!-- Keywords -->
	<entry type="Keyword" style="%s"/>
	<entry type="KeywordConstant" style="%s"/>
	<entry type="KeywordDeclaration" style="%s"/>
	<entry type="KeywordNamespace" style="%s"/>
	<entry type="KeywordPseudo" style="%s"/>
	<entry type="KeywordReserved" style="%s"/>
	<entry type="KeywordType" style="%s"/>
	<!-- Names -->
	<entry type="Name" style="%s"/>
	<entry type="NameAttribute" style="%s"/>
	<entry type="NameBuiltin" style="%s"/>
	<entry type="NameBuiltinPseudo" style="%s"/>
	<entry type="NameClass" style="%s"/>
	<entry type="NameConstant" style="%s"/>
	<entry type="NameDecorator" style="%s"/>
	<entry type="NameEntity" style="%s"/>
	<entry type="NameException" style="%s"/>
	<entry type="NameFunction" style="%s"/>
	<entry type="NameLabel" style="%s"/>
	<entry type="NameNamespace" style="%s"/>
	<entry type="NameOther" style="%s"/>
	<entry type="NameTag" style="%s"/>
	<entry type="NameVariable" style="%s"/>
	<entry type="NameVariableClass" style="%s"/>
	<entry type="NameVariableGlobal" style="%s"/>
	<entry type="NameVariableInstance" style="%s"/>
	<!-- Literals -->
	<entry type="Literal" style="%s"/>
	<entry type="LiteralDate" style="%s"/>
	<entry type="LiteralString" style="%s"/>
	<entry type="LiteralStringBacktick" style="%s"/>
	<entry type="LiteralStringChar" style="%s"/>
	<entry type="LiteralStringDoc" style="%s"/>
	<entry type="LiteralStringDouble" style="%s"/>
	<entry type="LiteralStringEscape" style="%s"/>
	<entry type="LiteralStringHeredoc" style="%s"/>
	<entry type="LiteralStringInterpol" style="%s"/>
	<entry type="LiteralStringOther" style="%s"/>
	<entry type="LiteralStringRegex" style="%s"/>
	<entry type="LiteralStringSingle" style="%s"/>
	<entry type="LiteralStringSymbol" style="%s"/>
	<!-- Numbers -->
	<entry type="LiteralNumber" style="%s"/>
	<entry type="LiteralNumberBin" style="%s"/>
	<entry type="LiteralNumberFloat" style="%s"/>
	<entry type="LiteralNumberHex" style="%s"/>
	<entry type="LiteralNumberInteger" style="%s"/>
	<entry type="LiteralNumberIntegerLong" style="%s"/>
	<entry type="LiteralNumberOct" style="%s"/>
	<!-- Operators -->
	<entry type="Operator" style="%s"/>
	<entry type="OperatorWord" style="%s"/>
	<entry type="Punctuation" style="%s"/>
	<!-- Comments -->
	<entry type="Comment" style="%s"/>
	<entry type="CommentHashbang" style="%s"/>
	<entry type="CommentMultiline" style="%s"/>
	<entry type="CommentSingle" style="%s"/>
	<entry type="CommentSpecial" style="%s"/>
	<entry type="CommentPreproc" style="%s"/>
	<!-- Generic styles -->
	<entry type="Generic" style="%s"/>
	<entry type="GenericDeleted" style="%s"/>
	<entry type="GenericEmph" style="italic %s"/>
	<entry type="GenericError" style="%s"/>
	<entry type="GenericHeading" style="bold %s"/>
	<entry type="GenericInserted" style="%s"/>
	<entry type="GenericOutput" style="%s"/>
	<entry type="GenericPrompt" style="%s"/>
	<entry type="GenericStrong" style="bold %s"/>
	<entry type="GenericSubheading" style="bold %s"/>
	<entry type="GenericTraceback" style="%s"/>
	<entry type="GenericUnderline" style="underline"/>
	<entry type="TextWhitespace" style="%s"/>
</style>
`,
		getColor(t.Text()),  // Text
		getColor(t.Text()),  // Other
		getColor(t.Error()), // Error

		getColor(t.SyntaxKeyword()), // Keyword
		getColor(t.SyntaxKeyword()), // KeywordConstant
		getColor(t.SyntaxKeyword()), // KeywordDeclaration
		getColor(t.SyntaxKeyword()), // KeywordNamespace
		getColor(t.SyntaxKeyword()), // KeywordPseudo
		getColor(t.SyntaxKeyword()), // KeywordReserved
		getColor(t.SyntaxType()),    // KeywordType

		getColor(t.Text()),           // Name
		getColor(t.SyntaxVariable()), // NameAttribute
		getColor(t.SyntaxType()),     // NameBuiltin
		getColor(t.SyntaxVariable()), // NameBuiltinPseudo
		getColor(t.SyntaxType()),     // NameClass
		getColor(t.SyntaxVariable()), // NameConstant
		getColor(t.SyntaxFunction()), // NameDecorator
		getColor(t.SyntaxVariable()), // NameEntity
		getColor(t.SyntaxType()),     // NameException
		getColor(t.SyntaxFunction()), // NameFunction
		getColor(t.Text()),           // NameLabel
		getColor(t.SyntaxType()),     // NameNamespace
		getColor(t.SyntaxVariable()), // NameOther
		getColor(t.SyntaxKeyword()),  // NameTag
		getColor(t.SyntaxVariable()), // NameVariable
		getColor(t.SyntaxVariable()), // NameVariableClass
		getColor(t.SyntaxVariable()), // NameVariableGlobal
		getColor(t.SyntaxVariable()), // NameVariableInstance

		getColor(t.SyntaxString()), // Literal
		getColor(t.SyntaxString()), // LiteralDate
		getColor(t.SyntaxString()), // LiteralString
		getColor(t.SyntaxString()), // LiteralStringBacktick
		getColor(t.SyntaxString()), // LiteralStringChar
		getColor(t.SyntaxString()), // LiteralStringDoc
		getColor(t.SyntaxString()), // LiteralStringDouble
		getColor(t.SyntaxString()), // LiteralStringEscape
		getColor(t.SyntaxString()), // LiteralStringHeredoc
		getColor(t.SyntaxString()), // LiteralStringInterpol
		getColor(t.SyntaxString()), // LiteralStringOther
		getColor(t.SyntaxString()), // LiteralStringRegex
		getColor(t.SyntaxString()), // LiteralStringSingle
		getColor(t.SyntaxString()), // LiteralStringSymbol

		getColor(t.SyntaxNumber()), // LiteralNumber
		getColor(t.SyntaxNumber()), // LiteralNumberBin
		getColor(t.SyntaxNumber()), // LiteralNumberFloat
		getColor(t.SyntaxNumber()), // LiteralNumberHex
		getColor(t.SyntaxNumber()), // LiteralNumberInteger
		getColor(t.SyntaxNumber()), // LiteralNumberIntegerLong
		getColor(t.SyntaxNumber()), // LiteralNumberOct

		getColor(t.SyntaxOperator()),    // Operator
		getColor(t.SyntaxKeyword()),     // OperatorWord
		getColor(t.SyntaxPunctuation()), // Punctuation

		getColor(t.SyntaxComment()), // Comment
		getColor(t.SyntaxComment()), // CommentHashbang
		getColor(t.SyntaxComment()), // CommentMultiline
		getColor(t.SyntaxComment()), // CommentSingle
		getColor(t.SyntaxComment()), // CommentSpecial
		getColor(t.SyntaxKeyword()), // CommentPreproc

		getColor(t.Text()),      // Generic
		getColor(t.Error()),     // GenericDeleted
		getColor(t.Text()),      // GenericEmph
		getColor(t.Error()),     // GenericError
		getColor(t.Text()),      // GenericHeading
		getColor(t.Success()),   // GenericInserted
		getColor(t.TextMuted()), // GenericOutput
		getColor(t.Text()),      // GenericPrompt
		getColor(t.Text()),      // GenericStrong
		getColor(t.Text()),      // GenericSubheading
		getColor(t.Error()),     // GenericTraceback
		getColor(t.Text()),      // TextWhitespace
	)

	r := strings.NewReader(syntaxThemeXml)
	style := chroma.MustNewXMLStyle(r)

	// Modify the style to use the provided background
	s, err := style.Builder().Transform(
		func(t chroma.StyleEntry) chroma.StyleEntry {
			r, g, b, _ := bg.RGBA()
			t.Background = chroma.NewColour(uint8(r>>8), uint8(g>>8), uint8(b>>8))
			return t
		},
	).Build()
	if err != nil {
		s = styles.Fallback
	}

	// Tokenize and format
	it, err := l.Tokenise(nil, source)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = f.Format(&buf, s, it)
	return buf.String(), err
}

func getColor(c color.Color) string {
	rgba := color.RGBAModel.Convert(c).(color.RGBA)
	return fmt.Sprintf("#%02x%02x%02x", rgba.R, rgba.G, rgba.B)
}
