package styles

import (
	"fmt"
	"image/color"

	"github.com/charmbracelet/glamour/v2"
	"github.com/charmbracelet/glamour/v2/ansi"
)

// lipglossColorToHex converts a color.Color to hex string
func lipglossColorToHex(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
}

// Helper functions for style pointers
func boolPtr(b bool) *bool       { return &b }
func stringPtr(s string) *string { return &s }
func uintPtr(u uint) *uint       { return &u }

// returns a glamour TermRenderer configured with the current theme
func GetMarkdownRenderer(width int) *glamour.TermRenderer {
	t := CurrentTheme()
	r, _ := glamour.NewTermRenderer(
		glamour.WithStyles(t.S().Markdown),
		glamour.WithWordWrap(width),
	)
	return r
}

// returns a glamour TermRenderer with no colors (plain text with structure)
func GetPlainMarkdownRenderer(width int) *glamour.TermRenderer {
	r, _ := glamour.NewTermRenderer(
		glamour.WithStyles(PlainMarkdownStyle()),
		glamour.WithWordWrap(width),
	)
	return r
}

// PlainMarkdownStyle returns a glamour style config with no colors
func PlainMarkdownStyle() ansi.StyleConfig {
	t := CurrentTheme()
	bgColor := stringPtr(lipglossColorToHex(t.BgBaseLighter))
	fgColor := stringPtr(lipglossColorToHex(t.FgMuted))
	return ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:           fgColor,
				BackgroundColor: bgColor,
			},
		},
		BlockQuote: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:           fgColor,
				BackgroundColor: bgColor,
			},
			Indent:      uintPtr(1),
			IndentToken: stringPtr("│ "),
		},
		List: ansi.StyleList{
			LevelIndent: defaultListIndent,
		},
		Heading: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockSuffix:     "\n",
				Bold:            boolPtr(true),
				Color:           fgColor,
				BackgroundColor: bgColor,
			},
		},
		H1: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          " ",
				Suffix:          " ",
				Bold:            boolPtr(true),
				Color:           fgColor,
				BackgroundColor: bgColor,
			},
		},
		H2: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          "## ",
				Color:           fgColor,
				BackgroundColor: bgColor,
			},
		},
		H3: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          "### ",
				Color:           fgColor,
				BackgroundColor: bgColor,
			},
		},
		H4: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          "#### ",
				Color:           fgColor,
				BackgroundColor: bgColor,
			},
		},
		H5: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          "##### ",
				Color:           fgColor,
				BackgroundColor: bgColor,
			},
		},
		H6: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          "###### ",
				Color:           fgColor,
				BackgroundColor: bgColor,
			},
		},
		Strikethrough: ansi.StylePrimitive{
			CrossedOut:      boolPtr(true),
			Color:           fgColor,
			BackgroundColor: bgColor,
		},
		Emph: ansi.StylePrimitive{
			Italic:          boolPtr(true),
			Color:           fgColor,
			BackgroundColor: bgColor,
		},
		Strong: ansi.StylePrimitive{
			Bold:            boolPtr(true),
			Color:           fgColor,
			BackgroundColor: bgColor,
		},
		HorizontalRule: ansi.StylePrimitive{
			Format:          "\n--------\n",
			Color:           fgColor,
			BackgroundColor: bgColor,
		},
		Item: ansi.StylePrimitive{
			BlockPrefix:     "• ",
			Color:           fgColor,
			BackgroundColor: bgColor,
		},
		Enumeration: ansi.StylePrimitive{
			BlockPrefix:     ". ",
			Color:           fgColor,
			BackgroundColor: bgColor,
		},
		Task: ansi.StyleTask{
			StylePrimitive: ansi.StylePrimitive{
				Color:           fgColor,
				BackgroundColor: bgColor,
			},
			Ticked:   "[✓] ",
			Unticked: "[ ] ",
		},
		Link: ansi.StylePrimitive{
			Underline:       boolPtr(true),
			Color:           fgColor,
			BackgroundColor: bgColor,
		},
		LinkText: ansi.StylePrimitive{
			Bold:            boolPtr(true),
			Color:           fgColor,
			BackgroundColor: bgColor,
		},
		Image: ansi.StylePrimitive{
			Underline:       boolPtr(true),
			Color:           fgColor,
			BackgroundColor: bgColor,
		},
		ImageText: ansi.StylePrimitive{
			Format:          "Image: {{.text}} →",
			Color:           fgColor,
			BackgroundColor: bgColor,
		},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          " ",
				Suffix:          " ",
				Color:           fgColor,
				BackgroundColor: bgColor,
			},
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color:           fgColor,
					BackgroundColor: bgColor,
				},
				Margin: uintPtr(defaultMargin),
			},
		},
		Table: ansi.StyleTable{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color:           fgColor,
					BackgroundColor: bgColor,
				},
			},
		},
		DefinitionDescription: ansi.StylePrimitive{
			BlockPrefix:     "\n ",
			Color:           fgColor,
			BackgroundColor: bgColor,
		},
	}
}
