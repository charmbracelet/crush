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
	tertiary  color.Color

	// Foregrounds.
	fgBase      color.Color
	fgMuted     color.Color
	fgHalfMuted color.Color
	fgSubtle    color.Color

	// Contrast pairings: foregrounds designed to sit on top of a
	// matching background role.
	onPrimary color.Color // foreground on primary backgrounds.
	onAccent  color.Color // foreground on saturated status/accent backgrounds.

	// Backgrounds.
	bgBase        color.Color
	bgBaseLighter color.Color
	bgSubtle      color.Color
	bgOverlay     color.Color

	// Borders.
	border      color.Color
	borderFocus color.Color

	// Status.
	danger        color.Color
	error         color.Color
	warning       color.Color
	warningStrong color.Color
	busy          color.Color
	info          color.Color
	infoSubtle    color.Color
	infoMuted     color.Color
	success       color.Color
	successSubtle color.Color
	successMuted  color.Color
}

// quickStyle builds a complete Styles value from a palette of semantic
// colors. Themes should populate quickStyleOpts and call this rather than
// re-implementing every style rule.
//
// The idea here is that you can do most of the work with quickStyle, then
// add overrides as needed.
func quickStyle(o quickStyleOpts) Styles {
	var (
		primary   = o.primary
		secondary = o.secondary
		tertiary  = o.tertiary

		fgBase      = o.fgBase
		fgMuted     = o.fgMuted
		fgHalfMuted = o.fgHalfMuted
		fgSubtle    = o.fgSubtle

		onPrimary = o.onPrimary
		onAccent  = o.onAccent

		bgBase        = o.bgBase
		bgBaseLighter = o.bgBaseLighter
		bgSubtle      = o.bgSubtle
		bgOverlay     = o.bgOverlay

		border      = o.border
		borderFocus = o.borderFocus

		danger        = o.danger
		error         = o.error
		warning       = o.warning
		warningStrong = o.warningStrong
		busy          = o.busy
		info          = o.info
		infoSubtle    = o.infoSubtle
		infoMuted     = o.infoMuted
		success       = o.success
		successSubtle = o.successSubtle
		successMuted  = o.successMuted
	)

	var (
		base   = lipgloss.NewStyle().Foreground(fgBase)
		muted  = lipgloss.NewStyle().Foreground(fgMuted)
		subtle = lipgloss.NewStyle().Foreground(fgSubtle)
		s      Styles
	)

	s.Background = bgBase

	// Populate color fields
	s.WorkingGradFromColor = primary
	s.WorkingGradToColor = secondary
	s.WorkingLabelColor = fgBase

	s.TextInput = textinput.Styles{
		Focused: textinput.StyleState{
			Text:        base,
			Placeholder: base.Foreground(fgSubtle),
			Prompt:      base.Foreground(tertiary),
			Suggestion:  base.Foreground(fgSubtle),
		},
		Blurred: textinput.StyleState{
			Text:        base.Foreground(fgMuted),
			Placeholder: base.Foreground(fgSubtle),
			Prompt:      base.Foreground(fgMuted),
			Suggestion:  base.Foreground(fgSubtle),
		},
		Cursor: textinput.CursorStyle{
			Color: secondary,
			Shape: tea.CursorBlock,
			Blink: true,
		},
	}

	s.Editor.Textarea = textarea.Styles{
		Focused: textarea.StyleState{
			Base:             base,
			Text:             base,
			LineNumber:       base.Foreground(fgSubtle),
			CursorLine:       base,
			CursorLineNumber: base.Foreground(fgSubtle),
			Placeholder:      base.Foreground(fgSubtle),
			Prompt:           base.Foreground(tertiary),
		},
		Blurred: textarea.StyleState{
			Base:             base,
			Text:             base.Foreground(fgMuted),
			LineNumber:       base.Foreground(fgMuted),
			CursorLine:       base,
			CursorLineNumber: base.Foreground(fgMuted),
			Placeholder:      base.Foreground(fgSubtle),
			Prompt:           base.Foreground(fgMuted),
		},
		Cursor: textarea.CursorStyle{
			Color: secondary,
			Shape: tea.CursorBlock,
			Blink: true,
		},
	}

	s.Markdown = ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				// BlockPrefix: "\n",
				// BlockSuffix: "\n",
				Color: hex(fgHalfMuted),
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
				Color:       hex(info),
				Bold:        new(true),
			},
		},
		H1: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          " ",
				Suffix:          " ",
				Color:           hex(warning),
				BackgroundColor: hex(primary),
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
				Color:  hex(successMuted),
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
			Color:  hex(border),
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
			Color: hex(successMuted),
			Bold:  new(true),
		},
		Image: ansi.StylePrimitive{
			Color:     hex(charmtone.Cheeky),
			Underline: new(true),
		},
		ImageText: ansi.StylePrimitive{
			Color:  hex(fgMuted),
			Format: "Image: {{.text}} →",
		},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          " ",
				Suffix:          " ",
				Color:           hex(danger),
				BackgroundColor: hex(bgSubtle),
			},
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color: hex(bgSubtle),
				},
				Margin: new(uint(defaultMargin)),
			},
			Chroma: &ansi.Chroma{
				Text: ansi.StylePrimitive{
					Color: hex(fgHalfMuted),
				},
				Error: ansi.StylePrimitive{
					Color:           hex(onAccent),
					BackgroundColor: hex(error),
				},
				Comment: ansi.StylePrimitive{
					Color: hex(fgSubtle),
				},
				CommentPreproc: ansi.StylePrimitive{
					Color: hex(charmtone.Bengal),
				},
				Keyword: ansi.StylePrimitive{
					Color: hex(info),
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
					Color: hex(warning),
				},
				Name: ansi.StylePrimitive{
					Color: hex(fgHalfMuted),
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
					Color: hex(successMuted),
				},
				LiteralNumber: ansi.StylePrimitive{
					Color: hex(success),
				},
				LiteralString: ansi.StylePrimitive{
					Color: hex(charmtone.Cumin),
				},
				LiteralStringEscape: ansi.StylePrimitive{
					Color: hex(successSubtle),
				},
				GenericDeleted: ansi.StylePrimitive{
					Color: hex(danger),
				},
				GenericEmph: ansi.StylePrimitive{
					Italic: new(true),
				},
				GenericInserted: ansi.StylePrimitive{
					Color: hex(successMuted),
				},
				GenericStrong: ansi.StylePrimitive{
					Bold: new(true),
				},
				GenericSubheading: ansi.StylePrimitive{
					Color: hex(fgMuted),
				},
				Background: ansi.StylePrimitive{
					BackgroundColor: hex(bgSubtle),
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
	plainBg := hex(bgBaseLighter)
	plainFg := hex(fgMuted)
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
		ShortKey:       base.Foreground(fgMuted),
		ShortDesc:      base.Foreground(fgSubtle),
		ShortSeparator: base.Foreground(border),
		Ellipsis:       base.Foreground(border),
		FullKey:        base.Foreground(fgMuted),
		FullDesc:       base.Foreground(fgSubtle),
		FullSeparator:  base.Foreground(border),
	}

	s.Diff = diffview.Style{
		DividerLine: diffview.LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(fgHalfMuted).
				Background(bgBaseLighter),
			Code: lipgloss.NewStyle().
				Foreground(fgHalfMuted).
				Background(bgBaseLighter),
		},
		MissingLine: diffview.LineStyle{
			LineNumber: lipgloss.NewStyle().
				Background(bgBaseLighter),
			Code: lipgloss.NewStyle().
				Background(bgBaseLighter),
		},
		EqualLine: diffview.LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(fgMuted).
				Background(bgBase),
			Code: lipgloss.NewStyle().
				Foreground(fgMuted).
				Background(bgBase),
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
				Foreground(fgHalfMuted).
				Background(bgBaseLighter),
			Code: lipgloss.NewStyle().
				Foreground(fgHalfMuted).
				Background(bgBaseLighter),
		},
	}

	s.FilePicker = filepicker.Styles{
		DisabledCursor:   base.Foreground(fgMuted),
		Cursor:           base.Foreground(fgBase),
		Symlink:          base.Foreground(fgSubtle),
		Directory:        base.Foreground(primary),
		File:             base.Foreground(fgBase),
		DisabledFile:     base.Foreground(fgMuted),
		DisabledSelected: base.Background(bgOverlay).Foreground(fgMuted),
		Permission:       base.Foreground(fgMuted),
		Selected:         base.Background(primary).Foreground(fgBase),
		FileSize:         base.Foreground(fgMuted),
		EmptyDirectory:   base.Foreground(fgMuted).PaddingLeft(2).SetString("Empty directory"),
	}

	// borders
	s.ToolCallSuccess = lipgloss.NewStyle().Foreground(success).SetString(ToolSuccess)

	s.Header.Charm = base.Foreground(secondary)
	s.Header.Diagonals = base.Foreground(primary)
	s.Header.Percentage = muted
	s.Header.Keystroke = muted
	s.Header.KeystrokeTip = subtle
	s.Header.WorkingDir = muted
	s.Header.Separator = subtle
	s.Header.Wrapper = lipgloss.NewStyle().Foreground(fgBase)
	s.Header.LogoGradCanvas = lipgloss.NewStyle()
	s.Header.LogoGradFromColor = secondary
	s.Header.LogoGradToColor = primary

	s.CompactDetails.Title = base
	s.CompactDetails.View = base.Padding(0, 1, 1, 1).Border(lipgloss.RoundedBorder()).BorderForeground(borderFocus)
	s.CompactDetails.Version = lipgloss.NewStyle().Foreground(border)

	// Tool rendering styles
	s.Tool.IconPending = base.Foreground(successMuted).SetString(ToolPending)
	s.Tool.IconSuccess = base.Foreground(success).SetString(ToolSuccess)
	s.Tool.IconError = base.Foreground(error).SetString(ToolError)
	s.Tool.IconCancelled = muted.SetString(ToolPending)

	s.Tool.NameNormal = base.Foreground(info)
	s.Tool.NameNested = base.Foreground(info)

	s.Tool.ParamMain = subtle
	s.Tool.ParamKey = subtle

	// Content rendering - prepared styles that accept width parameter
	s.Tool.ContentLine = muted.Background(bgBaseLighter)
	s.Tool.ContentTruncation = muted.Background(bgBaseLighter)
	s.Tool.ContentCodeLine = base.Background(bgBase).PaddingLeft(2)
	s.Tool.ContentCodeTruncation = muted.Background(bgBase).PaddingLeft(2)
	s.Tool.ContentCodeBg = bgBase
	s.Tool.Body = base.PaddingLeft(2)

	// Deprecated - kept for backward compatibility
	s.Tool.ContentBg = muted.Background(bgBaseLighter)
	s.Tool.ContentText = muted
	s.Tool.ContentLineNumber = base.Foreground(fgMuted).Background(bgBase).PaddingRight(1).PaddingLeft(1)

	s.Tool.StateWaiting = base.Foreground(fgSubtle)
	s.Tool.StateCancelled = base.Foreground(fgSubtle)

	s.Tool.ErrorTag = base.Padding(0, 1).Background(danger).Foreground(onAccent)
	s.Tool.ErrorMessage = base.Foreground(fgHalfMuted)

	// Diff and multi-edit styles
	s.Tool.DiffTruncation = muted.Background(bgBaseLighter).PaddingLeft(2)
	s.Tool.NoteTag = base.Padding(0, 1).Background(info).Foreground(onAccent)
	s.Tool.NoteMessage = base.Foreground(fgHalfMuted)

	// Job header styles
	s.Tool.JobIconPending = base.Foreground(successMuted)
	s.Tool.JobIconError = base.Foreground(error)
	s.Tool.JobIconSuccess = base.Foreground(success)
	s.Tool.JobToolName = base.Foreground(info)
	s.Tool.JobAction = base.Foreground(infoMuted)
	s.Tool.JobPID = muted
	s.Tool.JobDescription = subtle

	// Agent task styles
	s.Tool.AgentTaskTag = base.Bold(true).Padding(0, 1).MarginLeft(2).Background(infoSubtle).Foreground(onAccent)
	s.Tool.AgentPrompt = muted

	// Agentic fetch styles
	s.Tool.AgenticFetchPromptTag = base.Bold(true).Padding(0, 1).MarginLeft(2).Background(success).Foreground(border)

	// Todo styles
	s.Tool.TodoRatio = base.Foreground(infoMuted)
	s.Tool.TodoCompletedIcon = base.Foreground(success)
	s.Tool.TodoInProgressIcon = base.Foreground(successMuted)
	s.Tool.TodoPendingIcon = base.Foreground(fgMuted)
	s.Tool.TodoStatusNote = lipgloss.NewStyle().Foreground(fgSubtle)
	s.Tool.TodoItem = lipgloss.NewStyle().Foreground(fgBase)
	s.Tool.TodoJustStarted = lipgloss.NewStyle().Foreground(fgBase)

	// MCP styles
	s.Tool.MCPName = base.Foreground(info)
	s.Tool.MCPToolName = base.Foreground(infoMuted)
	s.Tool.MCPArrow = base.Foreground(info).SetString(ArrowRightIcon)

	// Loading indicators for images, skills
	s.Tool.ResourceLoadedText = base.Foreground(success)
	s.Tool.ResourceLoadedIndicator = base.Foreground(successMuted)
	s.Tool.ResourceName = base
	s.Tool.MediaType = base
	s.Tool.ResourceSize = base.Foreground(fgMuted)

	// Hook styles
	s.Tool.HookLabel = base.Foreground(successSubtle)
	s.Tool.HookName = base
	s.Tool.HookMatcher = base.Foreground(fgMuted)
	s.Tool.HookArrow = base.Foreground(successSubtle)
	s.Tool.HookDetail = base.Foreground(fgMuted)
	s.Tool.HookOK = base.Foreground(successMuted)
	s.Tool.HookDenied = base.Foreground(error)
	s.Tool.HookDeniedLabel = base.Foreground(danger)
	s.Tool.HookDeniedReason = base.Foreground(bgOverlay)
	s.Tool.HookRewrote = base.Foreground(bgOverlay)

	// Tool-call action verbs and result-list styling.
	s.Tool.ActionCreate = lipgloss.NewStyle().Foreground(successSubtle)
	s.Tool.ActionDestroy = lipgloss.NewStyle().Foreground(danger)
	s.Tool.ResultEmpty = lipgloss.NewStyle().Foreground(fgSubtle)
	s.Tool.ResultTruncation = lipgloss.NewStyle().Foreground(fgSubtle)
	s.Tool.ResultItemName = lipgloss.NewStyle().Foreground(fgBase)
	s.Tool.ResultItemDesc = lipgloss.NewStyle().Foreground(fgSubtle)

	// Buttons
	s.Button.Focused = lipgloss.NewStyle().Foreground(onAccent).Background(secondary)
	s.Button.Blurred = lipgloss.NewStyle().Foreground(fgBase).Background(bgSubtle)

	// Editor
	s.Editor.PromptNormalFocused = lipgloss.NewStyle().Foreground(successMuted).SetString("::: ")
	s.Editor.PromptNormalBlurred = s.Editor.PromptNormalFocused.Foreground(fgMuted)
	s.Editor.PromptYoloIconFocused = lipgloss.NewStyle().MarginRight(1).Foreground(fgSubtle).Background(busy).Bold(true).SetString(" ! ")
	s.Editor.PromptYoloIconBlurred = s.Editor.PromptYoloIconFocused.Foreground(bgBase).Background(fgMuted)
	s.Editor.PromptYoloDotsFocused = lipgloss.NewStyle().MarginRight(1).Foreground(warning).SetString(":::")
	s.Editor.PromptYoloDotsBlurred = s.Editor.PromptYoloDotsFocused.Foreground(fgMuted)

	s.Radio.On = lipgloss.NewStyle().Foreground(fgHalfMuted).SetString(RadioOn)
	s.Radio.Off = lipgloss.NewStyle().Foreground(fgHalfMuted).SetString(RadioOff)
	s.Radio.Label = lipgloss.NewStyle().Foreground(fgHalfMuted)

	// Logo
	s.Logo.FieldColor = primary
	s.Logo.TitleColorA = secondary
	s.Logo.TitleColorB = primary
	s.Logo.CharmColor = secondary
	s.Logo.VersionColor = primary
	s.Logo.SmallCharm = lipgloss.NewStyle().Foreground(secondary)
	s.Logo.SmallDiagonals = lipgloss.NewStyle().Foreground(primary)
	s.Logo.GradCanvas = lipgloss.NewStyle()
	s.Logo.SmallGradFromColor = secondary
	s.Logo.SmallGradToColor = primary

	// Section
	s.Section.Title = subtle
	s.Section.Line = base.Foreground(border)

	// Initialize
	s.Initialize.Header = base
	s.Initialize.Content = muted
	s.Initialize.Accent = base.Foreground(successMuted)

	// ResourceGroup (LSP/MCP/skills sidebar lists).
	s.Resource.Heading = lipgloss.NewStyle().Foreground(fgSubtle)
	s.Resource.Name = lipgloss.NewStyle().Foreground(fgMuted)
	s.Resource.StatusText = lipgloss.NewStyle().Foreground(fgSubtle)
	s.Resource.OfflineIcon = lipgloss.NewStyle().Foreground(bgOverlay).SetString("●")
	s.Resource.BusyIcon = s.Resource.OfflineIcon.Foreground(busy)
	s.Resource.ErrorIcon = s.Resource.OfflineIcon.Foreground(danger)
	s.Resource.OnlineIcon = s.Resource.OfflineIcon.Foreground(successMuted)
	s.Resource.DisabledIcon = lipgloss.NewStyle().Foreground(fgMuted).SetString("●")
	s.Resource.AdditionalText = lipgloss.NewStyle().Foreground(fgSubtle)
	s.Resource.CapabilityCount = lipgloss.NewStyle().Foreground(fgSubtle)
	s.Resource.RowTitleBase = lipgloss.NewStyle().Foreground(fgBase)
	s.Resource.RowDescBase = lipgloss.NewStyle().Foreground(fgBase)
	s.Resource.DefaultTitleFg = fgMuted
	s.Resource.DefaultDescFg = fgSubtle

	// LSP
	s.LSP.ErrorDiagnostic = base.Foreground(error)
	s.LSP.WarningDiagnostic = base.Foreground(warning)
	s.LSP.HintDiagnostic = base.Foreground(fgHalfMuted)
	s.LSP.InfoDiagnostic = base.Foreground(info)

	// Files
	s.Files.Path = lipgloss.NewStyle().Foreground(fgMuted)
	s.Files.Additions = lipgloss.NewStyle().Foreground(successMuted)
	s.Files.Deletions = lipgloss.NewStyle().Foreground(error)
	s.Files.SectionTitle = lipgloss.NewStyle().Foreground(fgSubtle)
	s.Files.EmptyMessage = lipgloss.NewStyle().Foreground(fgSubtle)
	s.Files.TruncationHint = lipgloss.NewStyle().Foreground(fgSubtle)

	// Sidebar
	s.Sidebar.SessionTitle = lipgloss.NewStyle().Foreground(fgMuted)
	s.Sidebar.WorkingDir = lipgloss.NewStyle().Foreground(fgMuted)

	// ModelInfo
	s.ModelInfo.Icon = lipgloss.NewStyle().Foreground(fgSubtle)
	s.ModelInfo.Name = lipgloss.NewStyle().Foreground(fgBase)
	s.ModelInfo.Provider = lipgloss.NewStyle().Foreground(fgMuted)
	s.ModelInfo.ProviderFallback = lipgloss.NewStyle().Foreground(fgMuted).PaddingLeft(2)
	s.ModelInfo.Reasoning = lipgloss.NewStyle().Foreground(fgSubtle).PaddingLeft(2)
	s.ModelInfo.TokenCount = lipgloss.NewStyle().Foreground(fgSubtle)
	s.ModelInfo.TokenPercentage = lipgloss.NewStyle().Foreground(fgMuted)
	s.ModelInfo.Cost = lipgloss.NewStyle().Foreground(fgMuted)

	// ResourceGroup
	s.Resource.DefaultTitleFg = fgMuted
	s.Resource.DefaultDescFg = fgSubtle

	// Chat
	messageFocussedBorder := lipgloss.Border{
		Left: "▌",
	}

	s.Messages.NoContent = lipgloss.NewStyle().Foreground(fgBase)
	s.Messages.UserBlurred = s.Messages.NoContent.PaddingLeft(1).BorderLeft(true).
		BorderForeground(primary).BorderStyle(lipgloss.NormalBorder())
	s.Messages.UserFocused = s.Messages.NoContent.PaddingLeft(1).BorderLeft(true).
		BorderForeground(primary).BorderStyle(messageFocussedBorder)
	s.Messages.AssistantBlurred = s.Messages.NoContent.PaddingLeft(2)
	s.Messages.AssistantFocused = s.Messages.NoContent.PaddingLeft(1).BorderLeft(true).
		BorderForeground(successMuted).BorderStyle(messageFocussedBorder)
	s.Messages.Thinking = lipgloss.NewStyle().MaxHeight(10)
	s.Messages.ErrorTag = lipgloss.NewStyle().Padding(0, 1).
		Background(danger).Foreground(onAccent)
	s.Messages.ErrorTitle = lipgloss.NewStyle().Foreground(fgHalfMuted)
	s.Messages.ErrorDetails = lipgloss.NewStyle().Foreground(fgSubtle)

	// Message item styles
	s.Messages.ToolCallFocused = muted.PaddingLeft(1).
		BorderStyle(messageFocussedBorder).
		BorderLeft(true).
		BorderForeground(successMuted)
	s.Messages.ToolCallBlurred = muted.PaddingLeft(2)
	// No padding or border for compact tool calls within messages
	s.Messages.ToolCallCompact = muted
	s.Messages.SectionHeader = base.PaddingLeft(2)
	s.Messages.AssistantInfoIcon = subtle
	s.Messages.AssistantInfoModel = muted
	s.Messages.AssistantInfoProvider = subtle
	s.Messages.AssistantInfoDuration = subtle
	s.Messages.AssistantCanceled = lipgloss.NewStyle().Foreground(fgBase).Italic(true)

	// Thinking section styles
	s.Messages.ThinkingBox = subtle.Background(bgBaseLighter)
	s.Messages.ThinkingTruncationHint = muted
	s.Messages.ThinkingFooterTitle = muted
	s.Messages.ThinkingFooterDuration = subtle

	// Text selection.
	s.TextSelection = lipgloss.NewStyle().Foreground(onPrimary).Background(primary)

	// Dialog styles
	s.Dialog.Title = base.Padding(0, 1).Foreground(primary)
	s.Dialog.TitleText = base.Foreground(primary)
	s.Dialog.TitleError = base.Foreground(danger)
	s.Dialog.TitleAccent = base.Foreground(success).Bold(true)
	s.Dialog.TitleLineBase = lipgloss.NewStyle()
	s.Dialog.TitleGradFromColor = primary
	s.Dialog.TitleGradToColor = secondary

	// Dialog.ListItem (commands, reasoning, models)
	s.Dialog.ListItem.InfoBlurred = lipgloss.NewStyle().Foreground(fgBase)
	s.Dialog.ListItem.InfoFocused = lipgloss.NewStyle().Foreground(fgBase)

	// Dialog.Models
	s.Dialog.Models.ConfiguredText = lipgloss.NewStyle().Foreground(fgSubtle)

	// Dialog.Permissions
	s.Dialog.Permissions.KeyText = lipgloss.NewStyle().Foreground(fgMuted)
	s.Dialog.Permissions.ValueText = lipgloss.NewStyle().Foreground(fgBase)
	s.Dialog.Permissions.ParamsBg = bgSubtle

	// Dialog.Quit
	s.Dialog.Quit.Content = lipgloss.NewStyle().Foreground(fgBase)
	s.Dialog.Quit.Frame = lipgloss.NewStyle().BorderForeground(borderFocus).Border(lipgloss.RoundedBorder()).Padding(1, 2)
	s.Dialog.View = base.Border(lipgloss.RoundedBorder()).BorderForeground(borderFocus)
	s.Dialog.PrimaryText = base.Padding(0, 1).Foreground(primary)
	s.Dialog.SecondaryText = base.Padding(0, 1).Foreground(fgSubtle)
	s.Dialog.HelpView = base.Padding(0, 1).AlignHorizontal(lipgloss.Left)
	s.Dialog.Help.ShortKey = base.Foreground(fgMuted)
	s.Dialog.Help.ShortDesc = base.Foreground(fgSubtle)
	s.Dialog.Help.ShortSeparator = base.Foreground(border)
	s.Dialog.Help.Ellipsis = base.Foreground(border)
	s.Dialog.Help.FullKey = base.Foreground(fgMuted)
	s.Dialog.Help.FullDesc = base.Foreground(fgSubtle)
	s.Dialog.Help.FullSeparator = base.Foreground(border)
	s.Dialog.NormalItem = base.Padding(0, 1).Foreground(fgBase)
	s.Dialog.SelectedItem = base.Padding(0, 1).Background(primary).Foreground(fgBase)
	s.Dialog.InputPrompt = base.Margin(1, 1)

	s.Dialog.List = base.Margin(0, 0, 1, 0)
	s.Dialog.ContentPanel = base.Background(bgSubtle).Foreground(fgBase).Padding(1, 2)
	s.Dialog.Spinner = base.Foreground(secondary)
	s.Dialog.ScrollbarThumb = base.Foreground(secondary)
	s.Dialog.ScrollbarTrack = base.Foreground(border)

	s.Dialog.ImagePreview = lipgloss.NewStyle().Padding(0, 1).Foreground(fgSubtle)

	// API key input dialog
	s.Dialog.APIKey.Spinner = base.Foreground(success)

	// OAuth dialog
	s.Dialog.OAuth.Spinner = base.Foreground(successSubtle)
	s.Dialog.OAuth.Instructions = lipgloss.NewStyle().Foreground(onAccent)
	s.Dialog.OAuth.UserCode = lipgloss.NewStyle().Bold(true).Foreground(onAccent)
	s.Dialog.OAuth.Success = lipgloss.NewStyle().Foreground(successSubtle)
	s.Dialog.OAuth.Link = lipgloss.NewStyle().Foreground(successMuted).Underline(true)
	s.Dialog.OAuth.Enter = lipgloss.NewStyle().Foreground(primary)
	s.Dialog.OAuth.ErrorText = lipgloss.NewStyle().Foreground(error)
	s.Dialog.OAuth.StatusText = lipgloss.NewStyle().Foreground(fgMuted)
	s.Dialog.OAuth.UserCodeBg = bgBaseLighter

	s.Dialog.Arguments.Content = base.Padding(1)
	s.Dialog.Arguments.Description = base.MarginBottom(1).MaxHeight(3)
	s.Dialog.Arguments.InputLabelBlurred = base.Foreground(fgMuted)
	s.Dialog.Arguments.InputLabelFocused = base.Bold(true)
	s.Dialog.Arguments.InputRequiredMarkBlurred = base.Foreground(fgMuted).SetString("*")
	s.Dialog.Arguments.InputRequiredMarkFocused = base.Foreground(primary).Bold(true).SetString("*")

	s.Dialog.Sessions.DeletingTitle = s.Dialog.Title.Foreground(danger)
	s.Dialog.Sessions.DeletingView = s.Dialog.View.BorderForeground(danger)
	s.Dialog.Sessions.DeletingMessage = base.Padding(1)
	s.Dialog.Sessions.DeletingTitleGradientFromColor = danger
	s.Dialog.Sessions.DeletingTitleGradientToColor = primary
	s.Dialog.Sessions.DeletingItemBlurred = s.Dialog.NormalItem.Foreground(fgSubtle)
	s.Dialog.Sessions.DeletingItemFocused = s.Dialog.SelectedItem.Background(danger).Foreground(onAccent)

	s.Dialog.Sessions.RenamingingTitle = s.Dialog.Title.Foreground(warning)
	s.Dialog.Sessions.RenamingView = s.Dialog.View.BorderForeground(warning)
	s.Dialog.Sessions.RenamingingMessage = base.Padding(1)
	s.Dialog.Sessions.RenamingTitleGradientFromColor = warning
	s.Dialog.Sessions.RenamingTitleGradientToColor = tertiary
	s.Dialog.Sessions.RenamingItemBlurred = s.Dialog.NormalItem.Foreground(fgSubtle)
	s.Dialog.Sessions.RenamingingItemFocused = s.Dialog.SelectedItem.UnsetBackground().UnsetForeground()
	s.Dialog.Sessions.RenamingPlaceholder = base.Foreground(fgMuted)
	s.Dialog.Sessions.InfoBlurred = lipgloss.NewStyle().Foreground(fgSubtle)
	s.Dialog.Sessions.InfoFocused = lipgloss.NewStyle().Foreground(fgBase)

	s.Status.Help = lipgloss.NewStyle().Padding(0, 1)
	s.Status.SuccessIndicator = base.Foreground(bgSubtle).Background(success).Padding(0, 1).Bold(true).SetString("OKAY!")
	s.Status.InfoIndicator = s.Status.SuccessIndicator
	s.Status.UpdateIndicator = s.Status.SuccessIndicator.SetString("HEY!")
	s.Status.WarnIndicator = s.Status.SuccessIndicator.Foreground(bgOverlay).Background(warningStrong).SetString("WARNING")
	s.Status.ErrorIndicator = s.Status.SuccessIndicator.Foreground(bgBase).Background(danger).SetString("ERROR")
	s.Status.SuccessMessage = base.Foreground(bgSubtle).Background(successMuted).Padding(0, 1)
	s.Status.InfoMessage = s.Status.SuccessMessage
	s.Status.UpdateMessage = s.Status.SuccessMessage
	s.Status.WarnMessage = s.Status.SuccessMessage.Foreground(bgOverlay).Background(warning)
	s.Status.ErrorMessage = s.Status.SuccessMessage.Foreground(onAccent).Background(error)

	// Completions styles
	s.Completions.Normal = base.Background(bgSubtle).Foreground(fgBase)
	s.Completions.Focused = base.Background(primary).Foreground(onAccent)
	s.Completions.Match = base.Underline(true)

	// Attachments styles
	attachmentIconStyle := base.Foreground(bgSubtle).Background(success).Padding(0, 1)
	s.Attachments.Image = attachmentIconStyle.SetString(ImageIcon)
	s.Attachments.Text = attachmentIconStyle.SetString(TextIcon)
	s.Attachments.Normal = base.Padding(0, 1).MarginRight(1).Background(fgMuted).Foreground(fgBase)
	s.Attachments.Deleting = base.Padding(0, 1).Bold(true).Background(danger).Foreground(fgBase)

	// Pills styles
	s.Pills.Base = base.Padding(0, 1)
	s.Pills.Focused = base.Padding(0, 1).BorderStyle(lipgloss.RoundedBorder()).BorderForeground(bgOverlay)
	s.Pills.Blurred = base.Padding(0, 1).BorderStyle(lipgloss.HiddenBorder())
	s.Pills.QueueItemPrefix = lipgloss.NewStyle().Foreground(fgMuted).SetString("  •")
	s.Pills.QueueItemText = lipgloss.NewStyle().Foreground(fgMuted)
	s.Pills.QueueLabel = lipgloss.NewStyle().Foreground(fgBase)
	s.Pills.QueueIconBase = lipgloss.NewStyle().Foreground(fgBase)
	s.Pills.QueueGradFromColor = error
	s.Pills.QueueGradToColor = secondary
	s.Pills.TodoLabel = lipgloss.NewStyle().Foreground(fgBase)
	s.Pills.TodoProgress = lipgloss.NewStyle().Foreground(fgMuted)
	s.Pills.TodoCurrentTask = lipgloss.NewStyle().Foreground(fgSubtle)
	s.Pills.TodoSpinner = lipgloss.NewStyle().Foreground(successMuted)
	s.Pills.HelpKey = lipgloss.NewStyle().Foreground(fgMuted)
	s.Pills.HelpText = lipgloss.NewStyle().Foreground(fgSubtle)
	s.Pills.Area = base

	return s
}
