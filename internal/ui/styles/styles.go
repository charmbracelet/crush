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
	"github.com/alecthomas/chroma/v2"
	"github.com/charmbracelet/crush/internal/tui/exp/diffview"
	"github.com/charmbracelet/x/exp/charmtone"
)

const (
	CheckIcon    string = "✓"
	ErrorIcon    string = "×"
	WarningIcon  string = "⚠"
	InfoIcon     string = "ⓘ"
	HintIcon     string = "∵"
	SpinnerIcon  string = "..."
	LoadingIcon  string = "⟳"
	DocumentIcon string = "🖼"
	ModelIcon    string = "◇"

	ArrowRightIcon string = "→"

	ToolPending string = "●"
	ToolSuccess string = "✓"
	ToolError   string = "×"

	RadioOn  string = "◉"
	RadioOff string = "○"

	BorderThin  string = "│"
	BorderThick string = "▌"

	SectionSeparator string = "─"

	TodoCompletedIcon  string = "✓"
	TodoPendingIcon    string = "•"
	TodoInProgressIcon string = "→"
)

const (
	defaultMargin     = 2
	defaultListIndent = 2
)

type Styles struct {
	WindowTooSmall lipgloss.Style

	// Reusable text styles
	Base      lipgloss.Style
	Muted     lipgloss.Style
	HalfMuted lipgloss.Style
	Subtle    lipgloss.Style

	// Tags
	TagBase  lipgloss.Style
	TagError lipgloss.Style
	TagInfo  lipgloss.Style

	// Headers
	HeaderTool       lipgloss.Style
	HeaderToolNested lipgloss.Style

	// Panels
	PanelMuted lipgloss.Style
	PanelBase  lipgloss.Style

	// Line numbers for code blocks
	LineNumber lipgloss.Style

	// Message borders
	FocusedMessageBorder lipgloss.Border

	// Tool calls
	ToolCallPending   lipgloss.Style
	ToolCallError     lipgloss.Style
	ToolCallSuccess   lipgloss.Style
	ToolCallCancelled lipgloss.Style
	EarlyStateMessage lipgloss.Style

	// Text selection
	TextSelection lipgloss.Style

	// LSP and MCP status indicators
	ItemOfflineIcon lipgloss.Style
	ItemBusyIcon    lipgloss.Style
	ItemErrorIcon   lipgloss.Style
	ItemOnlineIcon  lipgloss.Style

	// Markdown & Chroma
	Markdown      ansi.StyleConfig
	PlainMarkdown ansi.StyleConfig

	// Inputs
	TextInput textinput.Styles
	TextArea  textarea.Styles

	// Help
	Help help.Styles

	// Diff
	Diff diffview.Style

	// FilePicker
	FilePicker filepicker.Styles

	// Buttons
	ButtonFocus lipgloss.Style
	ButtonBlur  lipgloss.Style

	// Borders
	BorderFocus lipgloss.Style
	BorderBlur  lipgloss.Style

	// Editor
	EditorPromptNormalFocused   lipgloss.Style
	EditorPromptNormalBlurred   lipgloss.Style
	EditorPromptYoloIconFocused lipgloss.Style
	EditorPromptYoloIconBlurred lipgloss.Style
	EditorPromptYoloDotsFocused lipgloss.Style
	EditorPromptYoloDotsBlurred lipgloss.Style

	// Radio
	RadioOn  lipgloss.Style
	RadioOff lipgloss.Style

	// Background
	Background color.Color

	// Logo
	LogoFieldColor   color.Color
	LogoTitleColorA  color.Color
	LogoTitleColorB  color.Color
	LogoCharmColor   color.Color
	LogoVersionColor color.Color

	// Colors - semantic colors for tool rendering.
	Primary       color.Color
	Secondary     color.Color
	Tertiary      color.Color
	BgBase        color.Color
	BgBaseLighter color.Color
	BgSubtle      color.Color
	BgOverlay     color.Color
	FgBase        color.Color
	FgMuted       color.Color
	FgHalfMuted   color.Color
	FgSubtle      color.Color
	Border        color.Color
	BorderColor   color.Color // Border focus color
	Warning       color.Color
	Info          color.Color
	White         color.Color
	BlueLight     color.Color
	Blue          color.Color
	BlueDark      color.Color
	Green         color.Color
	GreenDark     color.Color
	Red           color.Color
	RedDark       color.Color
	Yellow        color.Color

	// Section Title
	Section struct {
		Title lipgloss.Style
		Line  lipgloss.Style
	}

	// Initialize
	Initialize struct {
		Header  lipgloss.Style
		Content lipgloss.Style
		Accent  lipgloss.Style
	}

	// LSP
	LSP struct {
		ErrorDiagnostic   lipgloss.Style
		WarningDiagnostic lipgloss.Style
		HintDiagnostic    lipgloss.Style
		InfoDiagnostic    lipgloss.Style
	}

	// Files
	Files struct {
		Path      lipgloss.Style
		Additions lipgloss.Style
		Deletions lipgloss.Style
	}

	// Chat
	Chat struct {
		// Message item styles
		Message struct {
			UserBlurred      lipgloss.Style
			UserFocused      lipgloss.Style
			AssistantBlurred lipgloss.Style
			AssistantFocused lipgloss.Style
			NoContent        lipgloss.Style
			Thinking         lipgloss.Style
			ErrorTag         lipgloss.Style
			ErrorTitle       lipgloss.Style
			ErrorDetails     lipgloss.Style
			Attachment       lipgloss.Style
			ToolCallFocused  lipgloss.Style
			ToolCallBlurred  lipgloss.Style
			SectionHeader    lipgloss.Style

			// Thinking section styles
			ThinkingBox            lipgloss.Style // Background for thinking content
			ThinkingTruncationHint lipgloss.Style // "… (N lines hidden)" hint
			ThinkingFooterTitle    lipgloss.Style // "Thought for" text
			ThinkingFooterDuration lipgloss.Style // Duration value
		}
	}

	// Tool - styles for tool call rendering
	Tool struct {
		// Icon styles with tool status
		IconPending   lipgloss.Style // Pending operation icon
		IconSuccess   lipgloss.Style // Successful operation icon
		IconError     lipgloss.Style // Error operation icon
		IconCancelled lipgloss.Style // Cancelled operation icon

		// Tool name styles
		NameNormal lipgloss.Style // Normal tool name
		NameNested lipgloss.Style // Nested tool name

		// Parameter list styles
		ParamMain lipgloss.Style // Main parameter
		ParamKey  lipgloss.Style // Parameter keys

		// Content rendering styles
		ContentLine           lipgloss.Style // Individual content line with background and width
		ContentTruncation     lipgloss.Style // Truncation message "… (N lines)"
		ContentCodeLine       lipgloss.Style // Code line with background and width
		ContentCodeTruncation lipgloss.Style // Code truncation message with bgBase
		ContentCodeBg         color.Color    // Background color for syntax highlighting
		Body                  lipgloss.Style // Body content padding (PaddingLeft(2))

		// Deprecated - kept for backward compatibility
		ContentBg         lipgloss.Style // Content background
		ContentText       lipgloss.Style // Content text
		ContentLineNumber lipgloss.Style // Line numbers in code

		// State message styles
		StateWaiting   lipgloss.Style // "Waiting for tool response..."
		StateCancelled lipgloss.Style // "Canceled."

		// Error styles
		ErrorTag     lipgloss.Style // ERROR tag
		ErrorMessage lipgloss.Style // Error message text

		// Diff styles
		DiffTruncation lipgloss.Style // Diff truncation message with padding

		// Multi-edit note styles
		NoteTag     lipgloss.Style // NOTE tag (yellow background)
		NoteMessage lipgloss.Style // Note message text

		// Job header styles (for bash jobs)
		JobIconPending lipgloss.Style // Pending job icon (green dark)
		JobIconError   lipgloss.Style // Error job icon (red dark)
		JobIconSuccess lipgloss.Style // Success job icon (green)
		JobToolName    lipgloss.Style // Job tool name "Bash" (blue)
		JobAction      lipgloss.Style // Action text (Start, Output, Kill)
		JobPID         lipgloss.Style // PID text
		JobDescription lipgloss.Style // Description text

		// Agent task styles
		AgentTaskTag lipgloss.Style // Agent task tag (blue background, bold)
		AgentPrompt  lipgloss.Style // Agent prompt text
	}

	// Dialog styles
	Dialog struct {
		Title lipgloss.Style
		// View is the main content area style.
		View lipgloss.Style
		// HelpView is the line that contains the help.
		HelpView lipgloss.Style
		Help     struct {
			Ellipsis       lipgloss.Style
			ShortKey       lipgloss.Style
			ShortDesc      lipgloss.Style
			ShortSeparator lipgloss.Style
			FullKey        lipgloss.Style
			FullDesc       lipgloss.Style
			FullSeparator  lipgloss.Style
		}
		NormalItem   lipgloss.Style
		SelectedItem lipgloss.Style
		InputPrompt  lipgloss.Style

		List lipgloss.Style

		Commands struct{}
	}
}

// ChromaTheme converts the current markdown chroma styles to a chroma
// StyleEntries map.
func (s *Styles) ChromaTheme() chroma.StyleEntries {
	rules := s.Markdown.CodeBlock

	return chroma.StyleEntries{
		chroma.Text:                chromaStyle(rules.Chroma.Text),
		chroma.Error:               chromaStyle(rules.Chroma.Error),
		chroma.Comment:             chromaStyle(rules.Chroma.Comment),
		chroma.CommentPreproc:      chromaStyle(rules.Chroma.CommentPreproc),
		chroma.Keyword:             chromaStyle(rules.Chroma.Keyword),
		chroma.KeywordReserved:     chromaStyle(rules.Chroma.KeywordReserved),
		chroma.KeywordNamespace:    chromaStyle(rules.Chroma.KeywordNamespace),
		chroma.KeywordType:         chromaStyle(rules.Chroma.KeywordType),
		chroma.Operator:            chromaStyle(rules.Chroma.Operator),
		chroma.Punctuation:         chromaStyle(rules.Chroma.Punctuation),
		chroma.Name:                chromaStyle(rules.Chroma.Name),
		chroma.NameBuiltin:         chromaStyle(rules.Chroma.NameBuiltin),
		chroma.NameTag:             chromaStyle(rules.Chroma.NameTag),
		chroma.NameAttribute:       chromaStyle(rules.Chroma.NameAttribute),
		chroma.NameClass:           chromaStyle(rules.Chroma.NameClass),
		chroma.NameConstant:        chromaStyle(rules.Chroma.NameConstant),
		chroma.NameDecorator:       chromaStyle(rules.Chroma.NameDecorator),
		chroma.NameException:       chromaStyle(rules.Chroma.NameException),
		chroma.NameFunction:        chromaStyle(rules.Chroma.NameFunction),
		chroma.NameOther:           chromaStyle(rules.Chroma.NameOther),
		chroma.Literal:             chromaStyle(rules.Chroma.Literal),
		chroma.LiteralNumber:       chromaStyle(rules.Chroma.LiteralNumber),
		chroma.LiteralDate:         chromaStyle(rules.Chroma.LiteralDate),
		chroma.LiteralString:       chromaStyle(rules.Chroma.LiteralString),
		chroma.LiteralStringEscape: chromaStyle(rules.Chroma.LiteralStringEscape),
		chroma.GenericDeleted:      chromaStyle(rules.Chroma.GenericDeleted),
		chroma.GenericEmph:         chromaStyle(rules.Chroma.GenericEmph),
		chroma.GenericInserted:     chromaStyle(rules.Chroma.GenericInserted),
		chroma.GenericStrong:       chromaStyle(rules.Chroma.GenericStrong),
		chroma.GenericSubheading:   chromaStyle(rules.Chroma.GenericSubheading),
		chroma.Background:          chromaStyle(rules.Chroma.Background),
	}
}

// DialogHelpStyles returns the styles for dialog help.
func (s *Styles) DialogHelpStyles() help.Styles {
	return help.Styles(s.Dialog.Help)
}

// DefaultStyles returns the default styles for the UI.
func DefaultStyles() Styles {
	var (
		primary   = charmtone.Charple
		secondary = charmtone.Dolly
		tertiary  = charmtone.Bok
		// accent    = charmtone.Zest

		// Backgrounds
		bgBase        = charmtone.Pepper
		bgBaseLighter = charmtone.BBQ
		bgSubtle      = charmtone.Charcoal
		bgOverlay     = charmtone.Iron

		// Foregrounds
		fgBase      = charmtone.Ash
		fgMuted     = charmtone.Squid
		fgHalfMuted = charmtone.Smoke
		fgSubtle    = charmtone.Oyster
		// fgSelected  = charmtone.Salt

		// Borders
		border      = charmtone.Charcoal
		borderFocus = charmtone.Charple

		// Status
		warning = charmtone.Zest
		info    = charmtone.Malibu

		// Colors
		white = charmtone.Butter

		blueLight = charmtone.Sardine
		blue      = charmtone.Malibu
		blueDark  = charmtone.Damson

		// yellow = charmtone.Mustard
		yellow = charmtone.Mustard
		// citron = charmtone.Citron

		green     = charmtone.Julep
		greenDark = charmtone.Guac
		// greenLight = charmtone.Bok

		red     = charmtone.Coral
		redDark = charmtone.Sriracha
		// redLight = charmtone.Salmon
		// cherry   = charmtone.Cherry
	)

	normalBorder := lipgloss.NormalBorder()

	base := lipgloss.NewStyle().Foreground(fgBase)

	s := Styles{}

	s.Background = bgBase

	// Populate color fields
	s.Primary = primary
	s.Secondary = secondary
	s.Tertiary = tertiary
	s.BgBase = bgBase
	s.BgBaseLighter = bgBaseLighter
	s.BgSubtle = bgSubtle
	s.BgOverlay = bgOverlay
	s.FgBase = fgBase
	s.FgMuted = fgMuted
	s.FgHalfMuted = fgHalfMuted
	s.FgSubtle = fgSubtle
	s.Border = border
	s.BorderColor = borderFocus
	s.Warning = warning
	s.Info = info
	s.White = white
	s.BlueLight = blueLight
	s.Blue = blue
	s.BlueDark = blueDark
	s.Green = green
	s.GreenDark = greenDark
	s.Red = red
	s.RedDark = redDark
	s.Yellow = yellow

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

	s.TextArea = textarea.Styles{
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
				Color: stringPtr(charmtone.Smoke.Hex()),
			},
			// Margin: uintPtr(defaultMargin),
		},
		BlockQuote: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{},
			Indent:         uintPtr(1),
			IndentToken:    stringPtr("│ "),
		},
		List: ansi.StyleList{
			LevelIndent: defaultListIndent,
		},
		Heading: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockSuffix: "\n",
				Color:       stringPtr(charmtone.Malibu.Hex()),
				Bold:        boolPtr(true),
			},
		},
		H1: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          " ",
				Suffix:          " ",
				Color:           stringPtr(charmtone.Zest.Hex()),
				BackgroundColor: stringPtr(charmtone.Charple.Hex()),
				Bold:            boolPtr(true),
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
				Color:  stringPtr(charmtone.Guac.Hex()),
				Bold:   boolPtr(false),
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
			Color:  stringPtr(charmtone.Charcoal.Hex()),
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
			Color:     stringPtr(charmtone.Zinc.Hex()),
			Underline: boolPtr(true),
		},
		LinkText: ansi.StylePrimitive{
			Color: stringPtr(charmtone.Guac.Hex()),
			Bold:  boolPtr(true),
		},
		Image: ansi.StylePrimitive{
			Color:     stringPtr(charmtone.Cheeky.Hex()),
			Underline: boolPtr(true),
		},
		ImageText: ansi.StylePrimitive{
			Color:  stringPtr(charmtone.Squid.Hex()),
			Format: "Image: {{.text}} →",
		},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          " ",
				Suffix:          " ",
				Color:           stringPtr(charmtone.Coral.Hex()),
				BackgroundColor: stringPtr(charmtone.Charcoal.Hex()),
			},
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color: stringPtr(charmtone.Charcoal.Hex()),
				},
				Margin: uintPtr(defaultMargin),
			},
			Chroma: &ansi.Chroma{
				Text: ansi.StylePrimitive{
					Color: stringPtr(charmtone.Smoke.Hex()),
				},
				Error: ansi.StylePrimitive{
					Color:           stringPtr(charmtone.Butter.Hex()),
					BackgroundColor: stringPtr(charmtone.Sriracha.Hex()),
				},
				Comment: ansi.StylePrimitive{
					Color: stringPtr(charmtone.Oyster.Hex()),
				},
				CommentPreproc: ansi.StylePrimitive{
					Color: stringPtr(charmtone.Bengal.Hex()),
				},
				Keyword: ansi.StylePrimitive{
					Color: stringPtr(charmtone.Malibu.Hex()),
				},
				KeywordReserved: ansi.StylePrimitive{
					Color: stringPtr(charmtone.Pony.Hex()),
				},
				KeywordNamespace: ansi.StylePrimitive{
					Color: stringPtr(charmtone.Pony.Hex()),
				},
				KeywordType: ansi.StylePrimitive{
					Color: stringPtr(charmtone.Guppy.Hex()),
				},
				Operator: ansi.StylePrimitive{
					Color: stringPtr(charmtone.Salmon.Hex()),
				},
				Punctuation: ansi.StylePrimitive{
					Color: stringPtr(charmtone.Zest.Hex()),
				},
				Name: ansi.StylePrimitive{
					Color: stringPtr(charmtone.Smoke.Hex()),
				},
				NameBuiltin: ansi.StylePrimitive{
					Color: stringPtr(charmtone.Cheeky.Hex()),
				},
				NameTag: ansi.StylePrimitive{
					Color: stringPtr(charmtone.Mauve.Hex()),
				},
				NameAttribute: ansi.StylePrimitive{
					Color: stringPtr(charmtone.Hazy.Hex()),
				},
				NameClass: ansi.StylePrimitive{
					Color:     stringPtr(charmtone.Salt.Hex()),
					Underline: boolPtr(true),
					Bold:      boolPtr(true),
				},
				NameDecorator: ansi.StylePrimitive{
					Color: stringPtr(charmtone.Citron.Hex()),
				},
				NameFunction: ansi.StylePrimitive{
					Color: stringPtr(charmtone.Guac.Hex()),
				},
				LiteralNumber: ansi.StylePrimitive{
					Color: stringPtr(charmtone.Julep.Hex()),
				},
				LiteralString: ansi.StylePrimitive{
					Color: stringPtr(charmtone.Cumin.Hex()),
				},
				LiteralStringEscape: ansi.StylePrimitive{
					Color: stringPtr(charmtone.Bok.Hex()),
				},
				GenericDeleted: ansi.StylePrimitive{
					Color: stringPtr(charmtone.Coral.Hex()),
				},
				GenericEmph: ansi.StylePrimitive{
					Italic: boolPtr(true),
				},
				GenericInserted: ansi.StylePrimitive{
					Color: stringPtr(charmtone.Guac.Hex()),
				},
				GenericStrong: ansi.StylePrimitive{
					Bold: boolPtr(true),
				},
				GenericSubheading: ansi.StylePrimitive{
					Color: stringPtr(charmtone.Squid.Hex()),
				},
				Background: ansi.StylePrimitive{
					BackgroundColor: stringPtr(charmtone.Charcoal.Hex()),
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

	// PlainMarkdown style - muted colors on subtle background for thinking content.
	plainBg := stringPtr(bgBaseLighter.Hex())
	plainFg := stringPtr(fgMuted.Hex())
	s.PlainMarkdown = ansi.StyleConfig{
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
				Color:           plainFg,
				BackgroundColor: plainBg,
			},
		},
		H1: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          " ",
				Suffix:          " ",
				Bold:            boolPtr(true),
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
			CrossedOut:      boolPtr(true),
			Color:           plainFg,
			BackgroundColor: plainBg,
		},
		Emph: ansi.StylePrimitive{
			Italic:          boolPtr(true),
			Color:           plainFg,
			BackgroundColor: plainBg,
		},
		Strong: ansi.StylePrimitive{
			Bold:            boolPtr(true),
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
			Underline:       boolPtr(true),
			Color:           plainFg,
			BackgroundColor: plainBg,
		},
		LinkText: ansi.StylePrimitive{
			Bold:            boolPtr(true),
			Color:           plainFg,
			BackgroundColor: plainBg,
		},
		Image: ansi.StylePrimitive{
			Underline:       boolPtr(true),
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
				Margin: uintPtr(defaultMargin),
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
	s.FocusedMessageBorder = lipgloss.Border{Left: BorderThick}

	// text presets
	s.Base = lipgloss.NewStyle().Foreground(fgBase)
	s.Muted = lipgloss.NewStyle().Foreground(fgMuted)
	s.HalfMuted = lipgloss.NewStyle().Foreground(fgHalfMuted)
	s.Subtle = lipgloss.NewStyle().Foreground(fgSubtle)

	s.WindowTooSmall = s.Muted

	// tag presets
	s.TagBase = lipgloss.NewStyle().Padding(0, 1).Foreground(white)
	s.TagError = s.TagBase.Background(redDark)
	s.TagInfo = s.TagBase.Background(blueLight)

	// headers
	s.HeaderTool = lipgloss.NewStyle().Foreground(blue)
	s.HeaderToolNested = lipgloss.NewStyle().Foreground(fgHalfMuted)

	// panels
	s.PanelMuted = s.Muted.Background(bgBaseLighter)
	s.PanelBase = lipgloss.NewStyle().Background(bgBase)

	// code line number
	s.LineNumber = lipgloss.NewStyle().Foreground(fgMuted).Background(bgBase).PaddingRight(1).PaddingLeft(1)

	// Tool calls
	s.ToolCallPending = lipgloss.NewStyle().Foreground(greenDark).SetString(ToolPending)
	s.ToolCallError = lipgloss.NewStyle().Foreground(redDark).SetString(ToolError)
	s.ToolCallSuccess = lipgloss.NewStyle().Foreground(green).SetString(ToolSuccess)
	// Cancelled uses muted tone but same glyph as pending
	s.ToolCallCancelled = s.Muted.SetString(ToolPending)
	s.EarlyStateMessage = s.Subtle.PaddingLeft(2)

	// Tool rendering styles
	s.Tool.IconPending = base.Foreground(greenDark).SetString(ToolPending)
	s.Tool.IconSuccess = base.Foreground(green).SetString(ToolSuccess)
	s.Tool.IconError = base.Foreground(redDark).SetString(ToolError)
	s.Tool.IconCancelled = s.Muted.SetString(ToolPending)

	s.Tool.NameNormal = base.Foreground(blue)
	s.Tool.NameNested = base.Foreground(fgHalfMuted)

	s.Tool.ParamMain = s.Subtle
	s.Tool.ParamKey = s.Subtle

	// Content rendering - prepared styles that accept width parameter
	s.Tool.ContentLine = s.Muted.Background(bgBaseLighter)
	s.Tool.ContentTruncation = s.Muted.Background(bgBaseLighter)
	s.Tool.ContentCodeLine = s.Base.Background(bgBase)
	s.Tool.ContentCodeTruncation = s.Muted.Background(bgBase).PaddingLeft(2)
	s.Tool.ContentCodeBg = bgBase
	s.Tool.Body = base.PaddingLeft(2)

	// Deprecated - kept for backward compatibility
	s.Tool.ContentBg = s.Muted.Background(bgBaseLighter)
	s.Tool.ContentText = s.Muted
	s.Tool.ContentLineNumber = base.Foreground(fgMuted).Background(bgBase).PaddingRight(1).PaddingLeft(1)

	s.Tool.StateWaiting = base.Foreground(fgSubtle)
	s.Tool.StateCancelled = base.Foreground(fgSubtle)

	s.Tool.ErrorTag = base.Padding(0, 1).Background(red).Foreground(white)
	s.Tool.ErrorMessage = base.Foreground(fgHalfMuted)

	// Diff and multi-edit styles
	s.Tool.DiffTruncation = s.Muted.Background(bgBaseLighter).PaddingLeft(2)
	s.Tool.NoteTag = base.Padding(0, 1).Background(info).Foreground(white)
	s.Tool.NoteMessage = base.Foreground(fgHalfMuted)

	// Job header styles
	s.Tool.JobIconPending = base.Foreground(greenDark)
	s.Tool.JobIconError = base.Foreground(redDark)
	s.Tool.JobIconSuccess = base.Foreground(green)
	s.Tool.JobToolName = base.Foreground(blue)
	s.Tool.JobAction = base.Foreground(blueDark)
	s.Tool.JobPID = s.Muted
	s.Tool.JobDescription = s.Subtle

	// Agent task styles
	s.Tool.AgentTaskTag = base.Bold(true).Padding(0, 1).MarginLeft(2).Background(blueLight).Foreground(white)
	s.Tool.AgentPrompt = s.Muted

	// Buttons
	s.ButtonFocus = lipgloss.NewStyle().Foreground(white).Background(secondary)
	s.ButtonBlur = s.Base.Background(bgSubtle)

	// Borders
	s.BorderFocus = lipgloss.NewStyle().BorderForeground(borderFocus).Border(lipgloss.RoundedBorder()).Padding(1, 2)

	// Editor
	s.EditorPromptNormalFocused = lipgloss.NewStyle().Foreground(greenDark).SetString("::: ")
	s.EditorPromptNormalBlurred = s.EditorPromptNormalFocused.Foreground(fgMuted)
	s.EditorPromptYoloIconFocused = lipgloss.NewStyle().MarginRight(1).Foreground(charmtone.Oyster).Background(charmtone.Citron).Bold(true).SetString(" ! ")
	s.EditorPromptYoloIconBlurred = s.EditorPromptYoloIconFocused.Foreground(charmtone.Pepper).Background(charmtone.Squid)
	s.EditorPromptYoloDotsFocused = lipgloss.NewStyle().MarginRight(1).Foreground(charmtone.Zest).SetString(":::")
	s.EditorPromptYoloDotsBlurred = s.EditorPromptYoloDotsFocused.Foreground(charmtone.Squid)

	s.RadioOn = s.HalfMuted.SetString(RadioOn)
	s.RadioOff = s.HalfMuted.SetString(RadioOff)

	// Logo colors
	s.LogoFieldColor = primary
	s.LogoTitleColorA = secondary
	s.LogoTitleColorB = primary
	s.LogoCharmColor = secondary
	s.LogoVersionColor = primary

	// Section
	s.Section.Title = s.Subtle
	s.Section.Line = s.Base.Foreground(charmtone.Charcoal)

	// Initialize
	s.Initialize.Header = s.Base
	s.Initialize.Content = s.Muted
	s.Initialize.Accent = s.Base.Foreground(greenDark)

	// LSP and MCP status.
	s.ItemOfflineIcon = lipgloss.NewStyle().Foreground(charmtone.Squid).SetString("●")
	s.ItemBusyIcon = s.ItemOfflineIcon.Foreground(charmtone.Citron)
	s.ItemErrorIcon = s.ItemOfflineIcon.Foreground(charmtone.Coral)
	s.ItemOnlineIcon = s.ItemOfflineIcon.Foreground(charmtone.Guac)

	// LSP
	s.LSP.ErrorDiagnostic = s.Base.Foreground(redDark)
	s.LSP.WarningDiagnostic = s.Base.Foreground(warning)
	s.LSP.HintDiagnostic = s.Base.Foreground(fgHalfMuted)
	s.LSP.InfoDiagnostic = s.Base.Foreground(info)

	// Files
	s.Files.Path = s.Muted
	s.Files.Additions = s.Base.Foreground(greenDark)
	s.Files.Deletions = s.Base.Foreground(redDark)

	// Chat
	messageFocussedBorder := lipgloss.Border{
		Left: "▌",
	}

	s.Chat.Message.NoContent = lipgloss.NewStyle().Foreground(fgBase)
	s.Chat.Message.UserBlurred = s.Chat.Message.NoContent.PaddingLeft(1).BorderLeft(true).
		BorderForeground(primary).BorderStyle(normalBorder)
	s.Chat.Message.UserFocused = s.Chat.Message.NoContent.PaddingLeft(1).BorderLeft(true).
		BorderForeground(primary).BorderStyle(messageFocussedBorder)
	s.Chat.Message.AssistantBlurred = s.Chat.Message.NoContent.PaddingLeft(2)
	s.Chat.Message.AssistantFocused = s.Chat.Message.NoContent.PaddingLeft(1).BorderLeft(true).
		BorderForeground(greenDark).BorderStyle(messageFocussedBorder)
	s.Chat.Message.Thinking = lipgloss.NewStyle().MaxHeight(10)
	s.Chat.Message.ErrorTag = lipgloss.NewStyle().Padding(0, 1).
		Background(red).Foreground(white)
	s.Chat.Message.ErrorTitle = lipgloss.NewStyle().Foreground(fgHalfMuted)
	s.Chat.Message.ErrorDetails = lipgloss.NewStyle().Foreground(fgSubtle)

	// Message item styles
	s.Chat.Message.Attachment = lipgloss.NewStyle().MarginLeft(1).Background(bgSubtle)
	s.Chat.Message.ToolCallFocused = s.Muted.PaddingLeft(1).
		BorderStyle(messageFocussedBorder).
		BorderLeft(true).
		BorderForeground(greenDark)
	s.Chat.Message.ToolCallBlurred = s.Muted.PaddingLeft(2)
	s.Chat.Message.SectionHeader = s.Base.PaddingLeft(2)

	// Thinking section styles
	s.Chat.Message.ThinkingBox = s.Subtle.Background(bgBaseLighter)
	s.Chat.Message.ThinkingTruncationHint = s.Muted
	s.Chat.Message.ThinkingFooterTitle = s.Muted
	s.Chat.Message.ThinkingFooterDuration = s.Subtle

	// Text selection.
	s.TextSelection = lipgloss.NewStyle().Foreground(charmtone.Salt).Background(charmtone.Charple)

	// Dialog styles
	s.Dialog.Title = base.Padding(0, 1).Foreground(primary)
	s.Dialog.View = base.Border(lipgloss.RoundedBorder()).BorderForeground(borderFocus)
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

	return s
}

// Helper functions for style pointers
func boolPtr(b bool) *bool       { return &b }
func stringPtr(s string) *string { return &s }
func uintPtr(u uint) *uint       { return &u }
func chromaStyle(style ansi.StylePrimitive) string {
	var s string

	if style.Color != nil {
		s = *style.Color
	}
	if style.BackgroundColor != nil {
		if s != "" {
			s += " "
		}
		s += "bg:" + *style.BackgroundColor
	}
	if style.Italic != nil && *style.Italic {
		if s != "" {
			s += " "
		}
		s += "italic"
	}
	if style.Bold != nil && *style.Bold {
		if s != "" {
			s += " "
		}
		s += "bold"
	}
	if style.Underline != nil && *style.Underline {
		if s != "" {
			s += " "
		}
		s += "underline"
	}

	return s
}
