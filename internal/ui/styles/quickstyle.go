package styles

import (
	"image/color"

	"charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2/ansi"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/diffview"
	"github.com/charmbracelet/x/exp/charmtone"
)

// quickStyleOpts is the palette of colors used by quickStyle to build a
// complete Styles value. Each field maps to a semantic role in the UI.
type quickStyleOpts struct {
	// Brand.
	primary   color.Color
	secondary color.Color
	accent    color.Color

	// Default foreground and background colors.
	fgBase color.Color
	bgBase color.Color

	// Low-contrast dividers, separators, and rule lines.
	separator color.Color

	fgSubtle     color.Color
	fgMoreSubtle color.Color
	fgMostSubtle color.Color

	// Contrast pairings: foregrounds designed to sit on top of a
	// matching background role.
	onPrimary color.Color // foreground on primary backgrounds.

	bgMostVisible  color.Color
	bgLessVisible  color.Color
	bgLeastVisible color.Color

	// Statuses.
	destructive       color.Color
	error             color.Color
	warning           color.Color
	warningSubtle     color.Color
	busy              color.Color
	info              color.Color
	infoMoreSubtle    color.Color
	infoMostSubtle    color.Color
	success           color.Color
	successMoreSubtle color.Color
	successMostSubtle color.Color
}

// quickStyle builds a complete Styles value from a palette of semantic
// colors. Themes should populate quickStyleOpts and call this rather than
// re-implementing every style rule.
//
// The idea here is that you can do most of the work with quickStyle, then
// add overrides as needed.
func quickStyle(o quickStyleOpts) Styles {
	var (
		base   = lipgloss.NewStyle().Foreground(o.fgBase)
		muted  = lipgloss.NewStyle().Foreground(o.fgMoreSubtle)
		subtle = lipgloss.NewStyle().Foreground(o.fgMostSubtle)
		s      Styles
	)

	s.Background = o.bgBase

	// Populate color fields
	s.WorkingGradFromColor = o.primary
	s.WorkingGradToColor = o.secondary
	s.WorkingLabelColor = o.fgBase

	s.TextInput = textinput.Styles{
		Focused: textinput.StyleState{
			Text:        base,
			Placeholder: base.Foreground(o.fgMostSubtle),
			Prompt:      base.Foreground(o.accent),
			Suggestion:  base.Foreground(o.fgMostSubtle),
		},
		Blurred: textinput.StyleState{
			Text:        base.Foreground(o.fgMoreSubtle),
			Placeholder: base.Foreground(o.fgMostSubtle),
			Prompt:      base.Foreground(o.fgMoreSubtle),
			Suggestion:  base.Foreground(o.fgMostSubtle),
		},
		Cursor: textinput.CursorStyle{
			Color: o.secondary,
			Shape: tea.CursorBlock,
			Blink: true,
		},
	}

	s.Editor.Textarea = textarea.Styles{
		Focused: textarea.StyleState{
			Base:             base,
			Text:             base,
			LineNumber:       base.Foreground(o.fgMostSubtle),
			CursorLine:       base,
			CursorLineNumber: base.Foreground(o.fgMostSubtle),
			Placeholder:      base.Foreground(o.fgMostSubtle),
			Prompt:           base.Foreground(o.accent),
		},
		Blurred: textarea.StyleState{
			Base:             base,
			Text:             base.Foreground(o.fgMoreSubtle),
			LineNumber:       base.Foreground(o.fgMoreSubtle),
			CursorLine:       base,
			CursorLineNumber: base.Foreground(o.fgMoreSubtle),
			Placeholder:      base.Foreground(o.fgMostSubtle),
			Prompt:           base.Foreground(o.fgMoreSubtle),
		},
		Cursor: textarea.CursorStyle{
			Color: o.secondary,
			Shape: tea.CursorBlock,
			Blink: true,
		},
	}

	s.Markdown = ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				// BlockPrefix: "\n",
				// BlockSuffix: "\n",
				Color: hex(o.fgSubtle),
			},
			// Margin: new(uint(defaultMargin)),
		},
		BlockQuote: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{},
			Indent:         new(uint(1)),
			IndentToken:    new("│ "),
		},
		List: ansi.StyleList{
			LevelIndent: defaultListIndent,
		},
		Heading: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockSuffix: "\n",
				Color:       hex(o.info),
				Bold:        new(true),
			},
		},
		H1: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          " ",
				Suffix:          " ",
				Color:           hex(o.warningSubtle),
				BackgroundColor: hex(o.primary),
				Bold:            new(true),
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
				Color:  hex(o.successMostSubtle),
				Bold:   new(false),
			},
		},
		Strikethrough: ansi.StylePrimitive{
			CrossedOut: new(true),
		},
		Emph: ansi.StylePrimitive{
			Italic: new(true),
		},
		Strong: ansi.StylePrimitive{
			Bold: new(true),
		},
		HorizontalRule: ansi.StylePrimitive{
			Color:  hex(o.separator),
			Format: "\n--------\n",
		},
		Item: ansi.StylePrimitive{
			BlockPrefix: "• ",
		},
		Enumeration: ansi.StylePrimitive{
			BlockPrefix: ". ",
		},
		Task: ansi.StyleTask{
			StylePrimitive: ansi.StylePrimitive{},
			Ticked:         "[✓] ",
			Unticked:       "[ ] ",
		},
		Link: ansi.StylePrimitive{
			Color:     hex(charmtone.Zinc),
			Underline: new(true),
		},
		LinkText: ansi.StylePrimitive{
			Color: hex(o.successMostSubtle),
			Bold:  new(true),
		},
		Image: ansi.StylePrimitive{
			Color:     hex(charmtone.Cheeky),
			Underline: new(true),
		},
		ImageText: ansi.StylePrimitive{
			Color:  hex(o.fgMoreSubtle),
			Format: "Image: {{.text}} →",
		},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          " ",
				Suffix:          " ",
				Color:           hex(o.destructive),
				BackgroundColor: hex(o.bgLessVisible),
			},
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color: hex(o.bgLessVisible),
				},
				Margin: new(uint(defaultMargin)),
			},
			Chroma: &ansi.Chroma{
				Text: ansi.StylePrimitive{
					Color: hex(o.fgSubtle),
				},
				Error: ansi.StylePrimitive{
					Color:           hex(o.primary),
					BackgroundColor: hex(o.error),
				},
				Comment: ansi.StylePrimitive{
					Color: hex(o.fgMostSubtle),
				},
				CommentPreproc: ansi.StylePrimitive{
					Color: hex(charmtone.Bengal),
				},
				Keyword: ansi.StylePrimitive{
					Color: hex(o.info),
				},
				KeywordReserved: ansi.StylePrimitive{
					Color: hex(charmtone.Pony),
				},
				KeywordNamespace: ansi.StylePrimitive{
					Color: hex(charmtone.Pony),
				},
				KeywordType: ansi.StylePrimitive{
					Color: hex(charmtone.Guppy),
				},
				Operator: ansi.StylePrimitive{
					Color: hex(charmtone.Salmon),
				},
				Punctuation: ansi.StylePrimitive{
					Color: hex(o.warningSubtle),
				},
				Name: ansi.StylePrimitive{
					Color: hex(o.fgSubtle),
				},
				NameBuiltin: ansi.StylePrimitive{
					Color: hex(charmtone.Cheeky),
				},
				NameTag: ansi.StylePrimitive{
					Color: hex(charmtone.Mauve),
				},
				NameAttribute: ansi.StylePrimitive{
					Color: hex(charmtone.Hazy),
				},
				NameClass: ansi.StylePrimitive{
					Color:     hex(charmtone.Salt),
					Underline: new(true),
					Bold:      new(true),
				},
				NameDecorator: ansi.StylePrimitive{
					Color: hex(charmtone.Citron),
				},
				NameFunction: ansi.StylePrimitive{
					Color: hex(o.successMostSubtle),
				},
				LiteralNumber: ansi.StylePrimitive{
					Color: hex(o.success),
				},
				LiteralString: ansi.StylePrimitive{
					Color: hex(charmtone.Cumin),
				},
				LiteralStringEscape: ansi.StylePrimitive{
					Color: hex(o.successMoreSubtle),
				},
				GenericDeleted: ansi.StylePrimitive{
					Color: hex(o.destructive),
				},
				GenericEmph: ansi.StylePrimitive{
					Italic: new(true),
				},
				GenericInserted: ansi.StylePrimitive{
					Color: hex(o.successMostSubtle),
				},
				GenericStrong: ansi.StylePrimitive{
					Bold: new(true),
				},
				GenericSubheading: ansi.StylePrimitive{
					Color: hex(o.fgMoreSubtle),
				},
				Background: ansi.StylePrimitive{
					BackgroundColor: hex(o.bgLessVisible),
				},
			},
		},
		Table: ansi.StyleTable{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{},
			},
		},
		DefinitionDescription: ansi.StylePrimitive{
			BlockPrefix: "\n ",
		},
	}

	// QuietMarkdown style - muted colors on subtle background for thinking content.
	plainBg := hex(o.bgLeastVisible)
	plainFg := hex(o.fgMoreSubtle)
	s.QuietMarkdown = ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:           plainFg,
				BackgroundColor: plainBg,
			},
		},
		BlockQuote: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:           plainFg,
				BackgroundColor: plainBg,
			},
			Indent:      new(uint(1)),
			IndentToken: new("│ "),
		},
		List: ansi.StyleList{
			LevelIndent: defaultListIndent,
		},
		Heading: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockSuffix:     "\n",
				Bold:            new(true),
				Color:           plainFg,
				BackgroundColor: plainBg,
			},
		},
		H1: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          " ",
				Suffix:          " ",
				Bold:            new(true),
				Color:           plainFg,
				BackgroundColor: plainBg,
			},
		},
		H2: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          "## ",
				Color:           plainFg,
				BackgroundColor: plainBg,
			},
		},
		H3: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          "### ",
				Color:           plainFg,
				BackgroundColor: plainBg,
			},
		},
		H4: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          "#### ",
				Color:           plainFg,
				BackgroundColor: plainBg,
			},
		},
		H5: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          "##### ",
				Color:           plainFg,
				BackgroundColor: plainBg,
			},
		},
		H6: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          "###### ",
				Color:           plainFg,
				BackgroundColor: plainBg,
			},
		},
		Strikethrough: ansi.StylePrimitive{
			CrossedOut:      new(true),
			Color:           plainFg,
			BackgroundColor: plainBg,
		},
		Emph: ansi.StylePrimitive{
			Italic:          new(true),
			Color:           plainFg,
			BackgroundColor: plainBg,
		},
		Strong: ansi.StylePrimitive{
			Bold:            new(true),
			Color:           plainFg,
			BackgroundColor: plainBg,
		},
		HorizontalRule: ansi.StylePrimitive{
			Format:          "\n--------\n",
			Color:           plainFg,
			BackgroundColor: plainBg,
		},
		Item: ansi.StylePrimitive{
			BlockPrefix:     "• ",
			Color:           plainFg,
			BackgroundColor: plainBg,
		},
		Enumeration: ansi.StylePrimitive{
			BlockPrefix:     ". ",
			Color:           plainFg,
			BackgroundColor: plainBg,
		},
		Task: ansi.StyleTask{
			StylePrimitive: ansi.StylePrimitive{
				Color:           plainFg,
				BackgroundColor: plainBg,
			},
			Ticked:   "[✓] ",
			Unticked: "[ ] ",
		},
		Link: ansi.StylePrimitive{
			Underline:       new(true),
			Color:           plainFg,
			BackgroundColor: plainBg,
		},
		LinkText: ansi.StylePrimitive{
			Bold:            new(true),
			Color:           plainFg,
			BackgroundColor: plainBg,
		},
		Image: ansi.StylePrimitive{
			Underline:       new(true),
			Color:           plainFg,
			BackgroundColor: plainBg,
		},
		ImageText: ansi.StylePrimitive{
			Format:          "Image: {{.text}} →",
			Color:           plainFg,
			BackgroundColor: plainBg,
		},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          " ",
				Suffix:          " ",
				Color:           plainFg,
				BackgroundColor: plainBg,
			},
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color:           plainFg,
					BackgroundColor: plainBg,
				},
				Margin: new(uint(defaultMargin)),
			},
		},
		Table: ansi.StyleTable{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color:           plainFg,
					BackgroundColor: plainBg,
				},
			},
		},
		DefinitionDescription: ansi.StylePrimitive{
			BlockPrefix:     "\n ",
			Color:           plainFg,
			BackgroundColor: plainBg,
		},
	}

	s.Help = help.Styles{
		ShortKey:       base.Foreground(o.fgMoreSubtle),
		ShortDesc:      base.Foreground(o.fgMostSubtle),
		ShortSeparator: base.Foreground(o.separator),
		Ellipsis:       base.Foreground(o.separator),
		FullKey:        base.Foreground(o.fgMoreSubtle),
		FullDesc:       base.Foreground(o.fgMostSubtle),
		FullSeparator:  base.Foreground(o.separator),
	}

	s.Diff = diffview.Style{
		DividerLine: diffview.LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(o.fgSubtle).
				Background(o.bgLeastVisible),
			Code: lipgloss.NewStyle().
				Foreground(o.fgSubtle).
				Background(o.bgLeastVisible),
		},
		MissingLine: diffview.LineStyle{
			LineNumber: lipgloss.NewStyle().
				Background(o.bgLeastVisible),
			Code: lipgloss.NewStyle().
				Background(o.bgLeastVisible),
		},
		EqualLine: diffview.LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(o.fgMoreSubtle).
				Background(o.bgBase),
			Code: lipgloss.NewStyle().
				Foreground(o.fgMoreSubtle).
				Background(o.bgBase),
		},
		InsertLine: diffview.LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#629657")).
				Background(lipgloss.Color("#2b322a")),
			Symbol: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#629657")).
				Background(lipgloss.Color("#323931")),
			Code: lipgloss.NewStyle().
				Background(lipgloss.Color("#323931")),
		},
		DeleteLine: diffview.LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#a45c59")).
				Background(lipgloss.Color("#312929")),
			Symbol: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#a45c59")).
				Background(lipgloss.Color("#383030")),
			Code: lipgloss.NewStyle().
				Background(lipgloss.Color("#383030")),
		},
		Filename: diffview.LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(o.fgSubtle).
				Background(o.bgLeastVisible),
			Code: lipgloss.NewStyle().
				Foreground(o.fgSubtle).
				Background(o.bgLeastVisible),
		},
	}

	s.FilePicker = filepicker.Styles{
		DisabledCursor:   base.Foreground(o.fgMoreSubtle),
		Cursor:           base.Foreground(o.fgBase),
		Symlink:          base.Foreground(o.fgMostSubtle),
		Directory:        base.Foreground(o.primary),
		File:             base.Foreground(o.fgBase),
		DisabledFile:     base.Foreground(o.fgMoreSubtle),
		DisabledSelected: base.Background(o.bgMostVisible).Foreground(o.fgMoreSubtle),
		Permission:       base.Foreground(o.fgMoreSubtle),
		Selected:         base.Background(o.primary).Foreground(o.fgBase),
		FileSize:         base.Foreground(o.fgMoreSubtle),
		EmptyDirectory:   base.Foreground(o.fgMoreSubtle).PaddingLeft(2).SetString("Empty directory"),
	}

	// borders
	s.ToolCallSuccess = lipgloss.NewStyle().Foreground(o.success).SetString(ToolSuccess)

	s.Header.Charm = base.Foreground(o.secondary)
	s.Header.Diagonals = base.Foreground(o.primary)
	s.Header.Percentage = muted
	s.Header.Keystroke = muted
	s.Header.KeystrokeTip = subtle
	s.Header.WorkingDir = muted
	s.Header.Separator = subtle
	s.Header.Wrapper = lipgloss.NewStyle().Foreground(o.fgBase)
	s.Header.LogoGradCanvas = lipgloss.NewStyle()
	s.Header.LogoGradFromColor = o.secondary
	s.Header.LogoGradToColor = o.primary

	s.CompactDetails.Title = base
	s.CompactDetails.View = base.Padding(0, 1, 1, 1).Border(lipgloss.RoundedBorder()).BorderForeground(o.primary)
	s.CompactDetails.Version = lipgloss.NewStyle().Foreground(o.separator)

	// Tool rendering styles
	s.Tool.IconPending = base.Foreground(o.successMostSubtle).SetString(ToolPending)
	s.Tool.IconSuccess = base.Foreground(o.success).SetString(ToolSuccess)
	s.Tool.IconError = base.Foreground(o.error).SetString(ToolError)
	s.Tool.IconCancelled = muted.SetString(ToolPending)

	s.Tool.NameNormal = base.Foreground(o.info)
	s.Tool.NameNested = base.Foreground(o.info)

	s.Tool.ParamMain = subtle
	s.Tool.ParamKey = subtle

	// Content rendering - prepared styles that accept width parameter
	s.Tool.ContentLine = muted.Background(o.bgLeastVisible)
	s.Tool.ContentTruncation = muted.Background(o.bgLeastVisible)
	s.Tool.ContentCodeLine = base.Background(o.bgBase).PaddingLeft(2)
	s.Tool.ContentCodeTruncation = muted.Background(o.bgBase).PaddingLeft(2)
	s.Tool.ContentCodeBg = o.bgBase
	s.Tool.Body = base.PaddingLeft(2)

	// Deprecated - kept for backward compatibility
	s.Tool.ContentBg = muted.Background(o.bgLeastVisible)
	s.Tool.ContentText = muted
	s.Tool.ContentLineNumber = base.Foreground(o.fgMoreSubtle).Background(o.bgBase).PaddingRight(1).PaddingLeft(1)

	s.Tool.StateWaiting = base.Foreground(o.fgMostSubtle)
	s.Tool.StateCancelled = base.Foreground(o.fgMostSubtle)

	s.Tool.ErrorTag = base.Padding(0, 1).Background(o.destructive).Foreground(o.primary)
	s.Tool.ErrorMessage = base.Foreground(o.fgSubtle)

	// Diff and multi-edit styles
	s.Tool.DiffTruncation = muted.Background(o.bgLeastVisible).PaddingLeft(2)
	s.Tool.NoteTag = base.Padding(0, 1).Background(o.info).Foreground(o.primary)
	s.Tool.NoteMessage = base.Foreground(o.fgSubtle)

	// Job header styles
	s.Tool.JobIconPending = base.Foreground(o.successMostSubtle)
	s.Tool.JobIconError = base.Foreground(o.error)
	s.Tool.JobIconSuccess = base.Foreground(o.success)
	s.Tool.JobToolName = base.Foreground(o.info)
	s.Tool.JobAction = base.Foreground(o.infoMostSubtle)
	s.Tool.JobPID = muted
	s.Tool.JobDescription = subtle

	// Agent task styles
	s.Tool.AgentTaskTag = base.Bold(true).Padding(0, 1).MarginLeft(2).Background(o.infoMoreSubtle).Foreground(o.primary)
	s.Tool.AgentPrompt = muted

	// Agentic fetch styles
	s.Tool.AgenticFetchPromptTag = base.Bold(true).Padding(0, 1).MarginLeft(2).Background(o.success).Foreground(o.separator)

	// Todo styles
	s.Tool.TodoRatio = base.Foreground(o.infoMostSubtle)
	s.Tool.TodoCompletedIcon = base.Foreground(o.success)
	s.Tool.TodoInProgressIcon = base.Foreground(o.successMostSubtle)
	s.Tool.TodoPendingIcon = base.Foreground(o.fgMoreSubtle)
	s.Tool.TodoStatusNote = lipgloss.NewStyle().Foreground(o.fgMostSubtle)
	s.Tool.TodoItem = lipgloss.NewStyle().Foreground(o.fgBase)
	s.Tool.TodoJustStarted = lipgloss.NewStyle().Foreground(o.fgBase)

	// MCP styles
	s.Tool.MCPName = base.Foreground(o.info)
	s.Tool.MCPToolName = base.Foreground(o.infoMostSubtle)
	s.Tool.MCPArrow = base.Foreground(o.info).SetString(ArrowRightIcon)

	// Loading indicators for images, skills
	s.Tool.ResourceLoadedText = base.Foreground(o.success)
	s.Tool.ResourceLoadedIndicator = base.Foreground(o.successMostSubtle)
	s.Tool.ResourceName = base
	s.Tool.MediaType = base
	s.Tool.ResourceSize = base.Foreground(o.fgMoreSubtle)

	// Hook styles
	s.Tool.HookLabel = base.Foreground(o.successMoreSubtle)
	s.Tool.HookName = base
	s.Tool.HookMatcher = base.Foreground(o.fgMoreSubtle)
	s.Tool.HookArrow = base.Foreground(o.successMoreSubtle)
	s.Tool.HookDetail = base.Foreground(o.fgMoreSubtle)
	s.Tool.HookOK = base.Foreground(o.successMostSubtle)
	s.Tool.HookDenied = base.Foreground(o.error)
	s.Tool.HookDeniedLabel = base.Foreground(o.destructive)
	s.Tool.HookDeniedReason = base.Foreground(o.bgMostVisible)
	s.Tool.HookRewrote = base.Foreground(o.bgMostVisible)

	// Tool-call action verbs and result-list styling.
	s.Tool.ActionCreate = lipgloss.NewStyle().Foreground(o.successMoreSubtle)
	s.Tool.ActionDestroy = lipgloss.NewStyle().Foreground(o.destructive)
	s.Tool.ResultEmpty = lipgloss.NewStyle().Foreground(o.fgMostSubtle)
	s.Tool.ResultTruncation = lipgloss.NewStyle().Foreground(o.fgMostSubtle)
	s.Tool.ResultItemName = lipgloss.NewStyle().Foreground(o.fgBase)
	s.Tool.ResultItemDesc = lipgloss.NewStyle().Foreground(o.fgMostSubtle)

	// Buttons
	s.Button.Focused = lipgloss.NewStyle().Foreground(o.primary).Background(o.secondary)
	s.Button.Blurred = lipgloss.NewStyle().Foreground(o.fgBase).Background(o.bgLessVisible)

	// Editor
	s.Editor.PromptNormalFocused = lipgloss.NewStyle().Foreground(o.successMostSubtle).SetString("::: ")
	s.Editor.PromptNormalBlurred = s.Editor.PromptNormalFocused.Foreground(o.fgMoreSubtle)
	s.Editor.PromptYoloIconFocused = lipgloss.NewStyle().MarginRight(1).Foreground(o.fgMostSubtle).Background(o.busy).Bold(true).SetString(" ! ")
	s.Editor.PromptYoloIconBlurred = s.Editor.PromptYoloIconFocused.Foreground(o.bgBase).Background(o.fgMoreSubtle)
	s.Editor.PromptYoloDotsFocused = lipgloss.NewStyle().MarginRight(1).Foreground(o.warningSubtle).SetString(":::")
	s.Editor.PromptYoloDotsBlurred = s.Editor.PromptYoloDotsFocused.Foreground(o.fgMoreSubtle)

	s.Radio.On = lipgloss.NewStyle().Foreground(o.fgSubtle).SetString(RadioOn)
	s.Radio.Off = lipgloss.NewStyle().Foreground(o.fgSubtle).SetString(RadioOff)
	s.Radio.Label = lipgloss.NewStyle().Foreground(o.fgSubtle)

	// Logo
	s.Logo.FieldColor = o.primary
	s.Logo.TitleColorA = o.secondary
	s.Logo.TitleColorB = o.primary
	s.Logo.CharmColor = o.secondary
	s.Logo.VersionColor = o.primary
	s.Logo.SmallCharm = lipgloss.NewStyle().Foreground(o.secondary)
	s.Logo.SmallDiagonals = lipgloss.NewStyle().Foreground(o.primary)
	s.Logo.GradCanvas = lipgloss.NewStyle()
	s.Logo.SmallGradFromColor = o.secondary
	s.Logo.SmallGradToColor = o.primary

	// Section
	s.Section.Title = subtle
	s.Section.Line = base.Foreground(o.separator)

	// Initialize
	s.Initialize.Header = base
	s.Initialize.Content = muted
	s.Initialize.Accent = base.Foreground(o.successMostSubtle)

	// ResourceGroup (LSP/MCP/skills sidebar lists).
	s.Resource.Heading = lipgloss.NewStyle().Foreground(o.fgMostSubtle)
	s.Resource.Name = lipgloss.NewStyle().Foreground(o.fgMoreSubtle)
	s.Resource.StatusText = lipgloss.NewStyle().Foreground(o.fgMostSubtle)
	s.Resource.OfflineIcon = lipgloss.NewStyle().Foreground(o.bgMostVisible).SetString("●")
	s.Resource.BusyIcon = s.Resource.OfflineIcon.Foreground(o.busy)
	s.Resource.ErrorIcon = s.Resource.OfflineIcon.Foreground(o.destructive)
	s.Resource.OnlineIcon = s.Resource.OfflineIcon.Foreground(o.successMostSubtle)
	s.Resource.DisabledIcon = lipgloss.NewStyle().Foreground(o.fgMoreSubtle).SetString("●")
	s.Resource.AdditionalText = lipgloss.NewStyle().Foreground(o.fgMostSubtle)
	s.Resource.CapabilityCount = lipgloss.NewStyle().Foreground(o.fgMostSubtle)
	s.Resource.RowTitleBase = lipgloss.NewStyle().Foreground(o.fgBase)
	s.Resource.RowDescBase = lipgloss.NewStyle().Foreground(o.fgBase)
	s.Resource.DefaultTitleFg = o.fgMoreSubtle
	s.Resource.DefaultDescFg = o.fgMostSubtle

	// LSP
	s.LSP.ErrorDiagnostic = base.Foreground(o.error)
	s.LSP.WarningDiagnostic = base.Foreground(o.warningSubtle)
	s.LSP.HintDiagnostic = base.Foreground(o.fgSubtle)
	s.LSP.InfoDiagnostic = base.Foreground(o.info)

	// Files
	s.Files.Path = lipgloss.NewStyle().Foreground(o.fgMoreSubtle)
	s.Files.Additions = lipgloss.NewStyle().Foreground(o.successMostSubtle)
	s.Files.Deletions = lipgloss.NewStyle().Foreground(o.error)
	s.Files.SectionTitle = lipgloss.NewStyle().Foreground(o.fgMostSubtle)
	s.Files.EmptyMessage = lipgloss.NewStyle().Foreground(o.fgMostSubtle)
	s.Files.TruncationHint = lipgloss.NewStyle().Foreground(o.fgMostSubtle)

	// Sidebar
	s.Sidebar.SessionTitle = lipgloss.NewStyle().Foreground(o.fgMoreSubtle)
	s.Sidebar.WorkingDir = lipgloss.NewStyle().Foreground(o.fgMoreSubtle)

	// ModelInfo
	s.ModelInfo.Icon = lipgloss.NewStyle().Foreground(o.fgMostSubtle)
	s.ModelInfo.Name = lipgloss.NewStyle().Foreground(o.fgBase)
	s.ModelInfo.Provider = lipgloss.NewStyle().Foreground(o.fgMoreSubtle)
	s.ModelInfo.ProviderFallback = lipgloss.NewStyle().Foreground(o.fgMoreSubtle).PaddingLeft(2)
	s.ModelInfo.Reasoning = lipgloss.NewStyle().Foreground(o.fgMostSubtle).PaddingLeft(2)
	s.ModelInfo.TokenCount = lipgloss.NewStyle().Foreground(o.fgMostSubtle)
	s.ModelInfo.TokenPercentage = lipgloss.NewStyle().Foreground(o.fgMoreSubtle)
	s.ModelInfo.Cost = lipgloss.NewStyle().Foreground(o.fgMoreSubtle)

	// ResourceGroup
	s.Resource.DefaultTitleFg = o.fgMoreSubtle
	s.Resource.DefaultDescFg = o.fgMostSubtle

	// Chat
	messageFocussedBorder := lipgloss.Border{
		Left: "▌",
	}

	s.Messages.NoContent = lipgloss.NewStyle().Foreground(o.fgBase)
	s.Messages.UserBlurred = s.Messages.NoContent.PaddingLeft(1).BorderLeft(true).
		BorderForeground(o.primary).BorderStyle(lipgloss.NormalBorder())
	s.Messages.UserFocused = s.Messages.NoContent.PaddingLeft(1).BorderLeft(true).
		BorderForeground(o.primary).BorderStyle(messageFocussedBorder)
	s.Messages.AssistantBlurred = s.Messages.NoContent.PaddingLeft(2)
	s.Messages.AssistantFocused = s.Messages.NoContent.PaddingLeft(1).BorderLeft(true).
		BorderForeground(o.successMostSubtle).BorderStyle(messageFocussedBorder)
	s.Messages.Thinking = lipgloss.NewStyle().MaxHeight(10)
	s.Messages.ErrorTag = lipgloss.NewStyle().Padding(0, 1).
		Background(o.destructive).Foreground(o.primary)
	s.Messages.ErrorTitle = lipgloss.NewStyle().Foreground(o.fgSubtle)
	s.Messages.ErrorDetails = lipgloss.NewStyle().Foreground(o.fgMostSubtle)

	// Message item styles
	s.Messages.ToolCallFocused = muted.PaddingLeft(1).
		BorderStyle(messageFocussedBorder).
		BorderLeft(true).
		BorderForeground(o.successMostSubtle)
	s.Messages.ToolCallBlurred = muted.PaddingLeft(2)
	// No padding or border for compact tool calls within messages
	s.Messages.ToolCallCompact = muted
	s.Messages.SectionHeader = base.PaddingLeft(2)
	s.Messages.AssistantInfoIcon = subtle
	s.Messages.AssistantInfoModel = muted
	s.Messages.AssistantInfoProvider = subtle
	s.Messages.AssistantInfoDuration = subtle
	s.Messages.AssistantCanceled = lipgloss.NewStyle().Foreground(o.fgBase).Italic(true)

	// Thinking section styles
	s.Messages.ThinkingBox = subtle.Background(o.bgLeastVisible)
	s.Messages.ThinkingTruncationHint = muted
	s.Messages.ThinkingFooterTitle = muted
	s.Messages.ThinkingFooterDuration = subtle

	// Text selection.
	s.TextSelection = lipgloss.NewStyle().Foreground(o.onPrimary).Background(o.primary)

	// Dialog styles
	s.Dialog.Title = base.Padding(0, 1).Foreground(o.primary)
	s.Dialog.TitleText = base.Foreground(o.primary)
	s.Dialog.TitleError = base.Foreground(o.destructive)
	s.Dialog.TitleAccent = base.Foreground(o.success).Bold(true)
	s.Dialog.TitleLineBase = lipgloss.NewStyle()
	s.Dialog.TitleGradFromColor = o.primary
	s.Dialog.TitleGradToColor = o.secondary

	// Dialog.ListItem (commands, reasoning, models)
	s.Dialog.ListItem.InfoBlurred = lipgloss.NewStyle().Foreground(o.fgBase)
	s.Dialog.ListItem.InfoFocused = lipgloss.NewStyle().Foreground(o.fgBase)

	// Dialog.Models
	s.Dialog.Models.ConfiguredText = lipgloss.NewStyle().Foreground(o.fgMostSubtle)

	// Dialog.Permissions
	s.Dialog.Permissions.KeyText = lipgloss.NewStyle().Foreground(o.fgMoreSubtle)
	s.Dialog.Permissions.ValueText = lipgloss.NewStyle().Foreground(o.fgBase)
	s.Dialog.Permissions.ParamsBg = o.bgLessVisible

	// Dialog.Quit
	s.Dialog.Quit.Content = lipgloss.NewStyle().Foreground(o.fgBase)
	s.Dialog.Quit.Frame = lipgloss.NewStyle().BorderForeground(o.primary).Border(lipgloss.RoundedBorder()).Padding(1, 2)
	s.Dialog.View = base.Border(lipgloss.RoundedBorder()).BorderForeground(o.primary)
	s.Dialog.PrimaryText = base.Padding(0, 1).Foreground(o.primary)
	s.Dialog.SecondaryText = base.Padding(0, 1).Foreground(o.fgMostSubtle)
	s.Dialog.HelpView = base.Padding(0, 1).AlignHorizontal(lipgloss.Left)
	s.Dialog.Help.ShortKey = base.Foreground(o.fgMoreSubtle)
	s.Dialog.Help.ShortDesc = base.Foreground(o.fgMostSubtle)
	s.Dialog.Help.ShortSeparator = base.Foreground(o.separator)
	s.Dialog.Help.Ellipsis = base.Foreground(o.separator)
	s.Dialog.Help.FullKey = base.Foreground(o.fgMoreSubtle)
	s.Dialog.Help.FullDesc = base.Foreground(o.fgMostSubtle)
	s.Dialog.Help.FullSeparator = base.Foreground(o.separator)
	s.Dialog.NormalItem = base.Padding(0, 1).Foreground(o.fgBase)
	s.Dialog.SelectedItem = base.Padding(0, 1).Background(o.primary).Foreground(o.fgBase)
	s.Dialog.InputPrompt = base.Margin(1, 1)

	s.Dialog.List = base.Margin(0, 0, 1, 0)
	s.Dialog.ContentPanel = base.Background(o.bgLessVisible).Foreground(o.fgBase).Padding(1, 2)
	s.Dialog.Spinner = base.Foreground(o.secondary)
	s.Dialog.ScrollbarThumb = base.Foreground(o.secondary)
	s.Dialog.ScrollbarTrack = base.Foreground(o.separator)

	s.Dialog.ImagePreview = lipgloss.NewStyle().Padding(0, 1).Foreground(o.fgMostSubtle)

	// API key input dialog
	s.Dialog.APIKey.Spinner = base.Foreground(o.success)

	// OAuth dialog
	s.Dialog.OAuth.Spinner = base.Foreground(o.successMoreSubtle)
	s.Dialog.OAuth.Instructions = lipgloss.NewStyle().Foreground(o.primary)
	s.Dialog.OAuth.UserCode = lipgloss.NewStyle().Bold(true).Foreground(o.primary)
	s.Dialog.OAuth.Success = lipgloss.NewStyle().Foreground(o.successMoreSubtle)
	s.Dialog.OAuth.Link = lipgloss.NewStyle().Foreground(o.successMostSubtle).Underline(true)
	s.Dialog.OAuth.Enter = lipgloss.NewStyle().Foreground(o.primary)
	s.Dialog.OAuth.ErrorText = lipgloss.NewStyle().Foreground(o.error)
	s.Dialog.OAuth.StatusText = lipgloss.NewStyle().Foreground(o.fgMoreSubtle)
	s.Dialog.OAuth.UserCodeBg = o.bgLeastVisible

	s.Dialog.Arguments.Content = base.Padding(1)
	s.Dialog.Arguments.Description = base.MarginBottom(1).MaxHeight(3)
	s.Dialog.Arguments.InputLabelBlurred = base.Foreground(o.fgMoreSubtle)
	s.Dialog.Arguments.InputLabelFocused = base.Bold(true)
	s.Dialog.Arguments.InputRequiredMarkBlurred = base.Foreground(o.fgMoreSubtle).SetString("*")
	s.Dialog.Arguments.InputRequiredMarkFocused = base.Foreground(o.primary).Bold(true).SetString("*")

	s.Dialog.Sessions.DeletingTitle = s.Dialog.Title.Foreground(o.destructive)
	s.Dialog.Sessions.DeletingView = s.Dialog.View.BorderForeground(o.destructive)
	s.Dialog.Sessions.DeletingMessage = base.Padding(1)
	s.Dialog.Sessions.DeletingTitleGradientFromColor = o.destructive
	s.Dialog.Sessions.DeletingTitleGradientToColor = o.primary
	s.Dialog.Sessions.DeletingItemBlurred = s.Dialog.NormalItem.Foreground(o.fgMostSubtle)
	s.Dialog.Sessions.DeletingItemFocused = s.Dialog.SelectedItem.Background(o.destructive).Foreground(o.onPrimary)

	s.Dialog.Sessions.RenamingingTitle = s.Dialog.Title.Foreground(o.warningSubtle)
	s.Dialog.Sessions.RenamingView = s.Dialog.View.BorderForeground(o.warningSubtle)
	s.Dialog.Sessions.RenamingingMessage = base.Padding(1)
	s.Dialog.Sessions.RenamingTitleGradientFromColor = o.warningSubtle
	s.Dialog.Sessions.RenamingTitleGradientToColor = o.accent
	s.Dialog.Sessions.RenamingItemBlurred = s.Dialog.NormalItem.Foreground(o.fgMostSubtle)
	s.Dialog.Sessions.RenamingingItemFocused = s.Dialog.SelectedItem.UnsetBackground().UnsetForeground()
	s.Dialog.Sessions.RenamingPlaceholder = base.Foreground(o.fgMoreSubtle)
	s.Dialog.Sessions.InfoBlurred = lipgloss.NewStyle().Foreground(o.fgMostSubtle)
	s.Dialog.Sessions.InfoFocused = lipgloss.NewStyle().Foreground(o.fgBase)

	s.Status.Help = lipgloss.NewStyle().Padding(0, 1)
	s.Status.SuccessIndicator = base.Foreground(o.bgLessVisible).Background(o.success).Padding(0, 1).Bold(true).SetString("OKAY!")
	s.Status.InfoIndicator = s.Status.SuccessIndicator
	s.Status.UpdateIndicator = s.Status.SuccessIndicator.SetString("HEY!")
	s.Status.WarnIndicator = s.Status.SuccessIndicator.Foreground(o.bgMostVisible).Background(o.warning).SetString("WARNING")
	s.Status.ErrorIndicator = s.Status.SuccessIndicator.Foreground(o.bgBase).Background(o.destructive).SetString("ERROR")
	s.Status.SuccessMessage = base.Foreground(o.bgLessVisible).Background(o.successMostSubtle).Padding(0, 1)
	s.Status.InfoMessage = s.Status.SuccessMessage
	s.Status.UpdateMessage = s.Status.SuccessMessage
	s.Status.WarnMessage = s.Status.SuccessMessage.Foreground(o.bgMostVisible).Background(o.warningSubtle)
	s.Status.ErrorMessage = s.Status.SuccessMessage.Foreground(o.primary).Background(o.error)

	// Completions styles
	s.Completions.Normal = base.Background(o.bgLessVisible).Foreground(o.fgBase)
	s.Completions.Focused = base.Background(o.primary).Foreground(o.primary)
	s.Completions.Match = base.Underline(true)

	// Attachments styles
	attachmentIconStyle := base.Foreground(o.bgLessVisible).Background(o.success).Padding(0, 1)
	s.Attachments.Image = attachmentIconStyle.SetString(ImageIcon)
	s.Attachments.Text = attachmentIconStyle.SetString(TextIcon)
	s.Attachments.Normal = base.Padding(0, 1).MarginRight(1).Background(o.fgMoreSubtle).Foreground(o.fgBase)
	s.Attachments.Deleting = base.Padding(0, 1).Bold(true).Background(o.destructive).Foreground(o.fgBase)

	// Pills styles
	s.Pills.Base = base.Padding(0, 1)
	s.Pills.Focused = base.Padding(0, 1).BorderStyle(lipgloss.RoundedBorder()).BorderForeground(o.bgMostVisible)
	s.Pills.Blurred = base.Padding(0, 1).BorderStyle(lipgloss.HiddenBorder())
	s.Pills.QueueItemPrefix = lipgloss.NewStyle().Foreground(o.fgMoreSubtle).SetString("  •")
	s.Pills.QueueItemText = lipgloss.NewStyle().Foreground(o.fgMoreSubtle)
	s.Pills.QueueLabel = lipgloss.NewStyle().Foreground(o.fgBase)
	s.Pills.QueueIconBase = lipgloss.NewStyle().Foreground(o.fgBase)
	s.Pills.QueueGradFromColor = o.error
	s.Pills.QueueGradToColor = o.secondary
	s.Pills.TodoLabel = lipgloss.NewStyle().Foreground(o.fgBase)
	s.Pills.TodoProgress = lipgloss.NewStyle().Foreground(o.fgMoreSubtle)
	s.Pills.TodoCurrentTask = lipgloss.NewStyle().Foreground(o.fgMostSubtle)
	s.Pills.TodoSpinner = lipgloss.NewStyle().Foreground(o.successMostSubtle)
	s.Pills.HelpKey = lipgloss.NewStyle().Foreground(o.fgMoreSubtle)
	s.Pills.HelpText = lipgloss.NewStyle().Foreground(o.fgMostSubtle)
	s.Pills.Area = base

	return s
}
