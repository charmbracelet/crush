package styles

import "github.com/charmbracelet/glamour/v2/ansi"

// TokyoNightStyleConfig is the tokyo night style.
var TokyoNightStyleConfig = ansi.StyleConfig{
	Document: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			BlockPrefix: "\n",
			BlockSuffix: "\n",
			Color:       stringPtr("#a9b1d6"),
		},
		Margin: uintPtr(defaultMargin),
	},
	BlockQuote: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{},
		Indent:         uintPtr(1),
		IndentToken:    stringPtr("│ "),
	},
	List: ansi.StyleList{
		StyleBlock: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr("#a9b1d6"),
			},
		},
		LevelIndent: defaultListIndent,
	},
	Heading: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			BlockSuffix: "\n",
			Color:       stringPtr("#bb9af7"),
			Bold:        boolPtr(true),
		},
	},
	H1: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "# ",
			Bold:   boolPtr(true),
		},
	},
	H2: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "## ",
		},
	},
	H3: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "### ",
		},
	},
	H4: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "#### ",
		},
	},
	H5: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "##### ",
		},
	},
	H6: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "###### ",
		},
	},
	Strikethrough: ansi.StylePrimitive{
		CrossedOut: boolPtr(true),
	},
	Emph: ansi.StylePrimitive{
		Italic: boolPtr(true),
	},
	Strong: ansi.StylePrimitive{
		Bold: boolPtr(true),
	},
	HorizontalRule: ansi.StylePrimitive{
		Color:  stringPtr("#565f89"),
		Format: "\n--------\n",
	},
	Item: ansi.StylePrimitive{
		BlockPrefix: "• ",
	},
	Enumeration: ansi.StylePrimitive{
		BlockPrefix: ". ",
		Color:       stringPtr("#7aa2f7"),
	},
	Task: ansi.StyleTask{
		StylePrimitive: ansi.StylePrimitive{},
		Ticked:         "[✓] ",
		Unticked:       "[ ] ",
	},
	Link: ansi.StylePrimitive{
		Color:     stringPtr("#7aa2f7"),
		Underline: boolPtr(true),
	},
	LinkText: ansi.StylePrimitive{
		Color: stringPtr("#2ac3de"),
	},
	Image: ansi.StylePrimitive{
		Color:     stringPtr("#7aa2f7"),
		Underline: boolPtr(true),
	},
	ImageText: ansi.StylePrimitive{
		Color:  stringPtr("#2ac3de"),
		Format: "Image: {{.text}} →",
	},
	Code: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Color: stringPtr("#9ece6a"),
		},
	},
	CodeBlock: ansi.StyleCodeBlock{
		StyleBlock: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr("#ff9e64"),
			},
			Margin: uintPtr(defaultMargin),
		},
		Chroma: &ansi.Chroma{
			Text: ansi.StylePrimitive{
				Color: stringPtr("#a9b1d6"),
			},
			Error: ansi.StylePrimitive{
				Color:           stringPtr("#a9b1d6"),
				BackgroundColor: stringPtr("#f7768e"),
			},
			Comment: ansi.StylePrimitive{
				Color: stringPtr("#565f89"),
			},
			CommentPreproc: ansi.StylePrimitive{
				Color: stringPtr("#2ac3de"),
			},
			Keyword: ansi.StylePrimitive{
				Color: stringPtr("#2ac3de"),
			},
			KeywordReserved: ansi.StylePrimitive{
				Color: stringPtr("#2ac3de"),
			},
			KeywordNamespace: ansi.StylePrimitive{
				Color: stringPtr("#2ac3de"),
			},
			KeywordType: ansi.StylePrimitive{
				Color: stringPtr("#7aa2f7"),
			},
			Operator: ansi.StylePrimitive{
				Color: stringPtr("#2ac3de"),
			},
			Punctuation: ansi.StylePrimitive{
				Color: stringPtr("#a9b1d6"),
			},
			Name: ansi.StylePrimitive{
				Color: stringPtr("#7aa2f7"),
			},
			NameConstant: ansi.StylePrimitive{
				Color: stringPtr("#bb9af7"),
			},
			NameBuiltin: ansi.StylePrimitive{
				Color: stringPtr("#7aa2f7"),
			},
			NameTag: ansi.StylePrimitive{
				Color: stringPtr("#2ac3de"),
			},
			NameAttribute: ansi.StylePrimitive{
				Color: stringPtr("#9ece6a"),
			},
			NameClass: ansi.StylePrimitive{
				Color: stringPtr("#7aa2f7"),
			},
			NameDecorator: ansi.StylePrimitive{
				Color: stringPtr("#9ece6a"),
			},
			NameFunction: ansi.StylePrimitive{
				Color: stringPtr("#9ece6a"),
			},
			LiteralNumber: ansi.StylePrimitive{},
			LiteralString: ansi.StylePrimitive{
				Color: stringPtr("#e0af68"),
			},
			LiteralStringEscape: ansi.StylePrimitive{
				Color: stringPtr("#2ac3de"),
			},
			GenericDeleted: ansi.StylePrimitive{
				Color: stringPtr("#f7768e"),
			},
			GenericEmph: ansi.StylePrimitive{
				Italic: boolPtr(true),
			},
			GenericInserted: ansi.StylePrimitive{
				Color: stringPtr("#9ece6a"),
			},
			GenericStrong: ansi.StylePrimitive{
				Bold: boolPtr(true),
			},
			GenericSubheading: ansi.StylePrimitive{
				Color: stringPtr("#bb9af7"),
			},
			Background: ansi.StylePrimitive{
				BackgroundColor: stringPtr("#1a1b26"),
			},
		},
	},
	Table: ansi.StyleTable{
		StyleBlock: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{},
		},
	},
	DefinitionDescription: ansi.StylePrimitive{
		BlockPrefix: "\n🠶 ",
	},
}
