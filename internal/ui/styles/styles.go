package styles

import (
	"image/color"
	"strings"

	"charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2/ansi"
	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/charmbracelet/crush/internal/ui/diffview"
	"github.com/charmbracelet/x/exp/charmtone"
)

const (
	CheckIcon   string = "✓"
	SpinnerIcon string = "⋯"
	LoadingIcon string = "⟳"
	ModelIcon   string = "◇"

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

	ImageIcon string = "■"
	TextIcon  string = "≡"

	ScrollbarThumb string = "┃"
	ScrollbarTrack string = "│"

	LSPErrorIcon   string = "E"
	LSPWarningIcon string = "W"
	LSPInfoIcon    string = "I"
	LSPHintIcon    string = "H"
)

const (
	defaultMargin     = 2
	defaultListIndent = 2
)

type Styles struct {
	// Header
	Header struct {
		Charm             lipgloss.Style // Style for "Charm™" label
		Diagonals         lipgloss.Style // Style for diagonal separators (╱)
		Percentage        lipgloss.Style // Style for context percentage
		Keystroke         lipgloss.Style // Style for keystroke hints (e.g., "ctrl+d")
		KeystrokeTip      lipgloss.Style // Style for keystroke action text (e.g., "open", "close")
		WorkingDir        lipgloss.Style // Style for current working directory
		Separator         lipgloss.Style // Style for separator dots (•)
		Wrapper           lipgloss.Style // Outer container for the entire header row
		LogoGradCanvas    lipgloss.Style // Canvas for the compact "CRUSH" gradient
		LogoGradFromColor color.Color    // "CRUSH" wordmark gradient start
		LogoGradToColor   color.Color    // "CRUSH" wordmark gradient end
	}

	CompactDetails struct {
		View    lipgloss.Style
		Version lipgloss.Style
		Title   lipgloss.Style
	}

	// Tool calls
	ToolCallSuccess lipgloss.Style

	// Text selection
	TextSelection lipgloss.Style

	// Markdown & Chroma
	Markdown      ansi.StyleConfig
	QuietMarkdown ansi.StyleConfig

	// Inputs
	TextInput textinput.Styles

	// Help
	Help help.Styles

	// Diff
	Diff diffview.Style

	// FilePicker
	FilePicker filepicker.Styles

	// Buttons
	Button struct {
		Focused lipgloss.Style
		Blurred lipgloss.Style
	}

	// Editor
	Editor struct {
		Textarea textarea.Styles

		// Normal mode prompt (default "::: ").
		PromptNormalFocused lipgloss.Style
		PromptNormalBlurred lipgloss.Style

		// YOLO mode prompt (" ! " icon + ":::" dots).
		PromptYoloIconFocused lipgloss.Style
		PromptYoloIconBlurred lipgloss.Style
		PromptYoloDotsFocused lipgloss.Style
		PromptYoloDotsBlurred lipgloss.Style
	}

	// Radio
	Radio struct {
		On    lipgloss.Style
		Off   lipgloss.Style
		Label lipgloss.Style // Text next to a radio button
	}

	// Background
	Background color.Color

	// Logo
	Logo struct {
		FieldColor         color.Color
		TitleColorA        color.Color
		TitleColorB        color.Color
		CharmColor         color.Color
		VersionColor       color.Color
		SmallCharm         lipgloss.Style // "Charm™" label in SmallRender
		SmallDiagonals     lipgloss.Style // Diagonal line fill in SmallRender
		GradCanvas         lipgloss.Style // Blank canvas for gradient painting
		SmallGradFromColor color.Color    // Small "Crush" wordmark gradient start
		SmallGradToColor   color.Color    // Small "Crush" wordmark gradient end
	}

	// Working indicator gradient (spinners/shimmers on assistant "thinking",
	// tool-call pending, CLI generating, startup).
	WorkingGradFromColor color.Color
	WorkingGradToColor   color.Color
	WorkingLabelColor    color.Color // Label text color next to the indicator

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

	// Sidebar
	Sidebar struct {
		SessionTitle lipgloss.Style // Current session title at top of sidebar
		WorkingDir   lipgloss.Style // Working directory path (PrettyPath)
	}

	// ModelInfo (model name, provider, reasoning, token/cost summary)
	ModelInfo struct {
		Icon             lipgloss.Style // Model icon (◇)
		Name             lipgloss.Style // Model name text
		Provider         lipgloss.Style // "via <provider>" text
		ProviderFallback lipgloss.Style // Provider on its own second line
		Reasoning        lipgloss.Style // Reasoning effort text
		TokenCount       lipgloss.Style // "(42K)" token count
		TokenPercentage  lipgloss.Style // "42%" percent of context window
		Cost             lipgloss.Style // "$0.42" cost readout
	}

	// Resource styles the LSP/MCP/skills sidebar lists: their heading,
	// each row's status icon, name, status text, and truncation hints.
	Resource struct {
		Heading         lipgloss.Style // Section header ("LSPs", "MCPs", "Skills")
		Name            lipgloss.Style // Resource name (e.g. "gopls")
		StatusText      lipgloss.Style // Row status description (e.g. "starting...")
		OfflineIcon     lipgloss.Style // Offline/unstarted/stopped status icon
		DisabledIcon    lipgloss.Style // Disabled status icon
		BusyIcon        lipgloss.Style // Busy/starting status icon
		ErrorIcon       lipgloss.Style // Error status icon
		OnlineIcon      lipgloss.Style // Online/ready status icon
		AdditionalText  lipgloss.Style // "None" and "…and N more" text
		CapabilityCount lipgloss.Style // "N tools" / "N prompts" / "N resources"
		RowTitleBase    lipgloss.Style // Base style applied over row titles in common.Status
		RowDescBase     lipgloss.Style // Base style applied over row descriptions in common.Status
		DefaultTitleFg  color.Color    // Default title color when opt is zero
		DefaultDescFg   color.Color    // Default description color when opt is zero
	}

	// Files
	Files struct {
		Path           lipgloss.Style
		Additions      lipgloss.Style
		Deletions      lipgloss.Style
		SectionTitle   lipgloss.Style // "Modified Files" heading
		EmptyMessage   lipgloss.Style // "None" placeholder when no files
		TruncationHint lipgloss.Style // "…and N more" message
	}

	// Chat
	// Messages - chat message item styles
	Messages struct {
		UserBlurred      lipgloss.Style
		UserFocused      lipgloss.Style
		AssistantBlurred lipgloss.Style
		AssistantFocused lipgloss.Style
		NoContent        lipgloss.Style
		Thinking         lipgloss.Style
		ErrorTag         lipgloss.Style
		ErrorTitle       lipgloss.Style
		ErrorDetails     lipgloss.Style
		ToolCallFocused  lipgloss.Style
		ToolCallCompact  lipgloss.Style
		ToolCallBlurred  lipgloss.Style
		SectionHeader    lipgloss.Style

		// Thinking section styles
		ThinkingBox            lipgloss.Style // Background for thinking content
		ThinkingTruncationHint lipgloss.Style // "… (N lines hidden)" hint
		ThinkingFooterTitle    lipgloss.Style // "Thought for" text
		ThinkingFooterDuration lipgloss.Style // Duration value
		AssistantInfoIcon      lipgloss.Style
		AssistantInfoModel     lipgloss.Style
		AssistantInfoProvider  lipgloss.Style
		AssistantInfoDuration  lipgloss.Style
		AssistantCanceled      lipgloss.Style // Italic "Canceled" footer
	}

	// Tool - styles for tool call rendering
	Tool struct {
		// Icon styles with tool status
		IconPending   lipgloss.Style
		IconSuccess   lipgloss.Style
		IconError     lipgloss.Style
		IconCancelled lipgloss.Style

		// Tool name styles
		NameNormal lipgloss.Style // Top-level tool name
		NameNested lipgloss.Style // Nested child tool name (inside Agent/Agentic Fetch)

		// Parameter list styles
		ParamMain lipgloss.Style
		ParamKey  lipgloss.Style

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

		// Agentic fetch styles
		AgenticFetchPromptTag lipgloss.Style // Agentic fetch prompt tag (green background, bold)

		// Todo styles
		TodoRatio          lipgloss.Style // Todo ratio (e.g., "2/5")
		TodoCompletedIcon  lipgloss.Style // Completed todo icon
		TodoInProgressIcon lipgloss.Style // In-progress todo icon
		TodoPendingIcon    lipgloss.Style // Pending todo icon
		TodoStatusNote     lipgloss.Style // " · completed N" / " · starting task" trailing note
		TodoItem           lipgloss.Style // Default body text for todo list items
		TodoJustStarted    lipgloss.Style // Text of the just-started todo in tool-call bodies

		// MCP tools
		MCPName     lipgloss.Style // The mcp name
		MCPToolName lipgloss.Style // The mcp tool name
		MCPArrow    lipgloss.Style // The mcp arrow icon

		// Images and external resources
		ResourceLoadedText      lipgloss.Style
		ResourceLoadedIndicator lipgloss.Style
		ResourceName            lipgloss.Style
		ResourceSize            lipgloss.Style
		MediaType               lipgloss.Style

		// Hooks
		HookLabel        lipgloss.Style // "Hook" label
		HookName         lipgloss.Style // Hook command name
		HookMatcher      lipgloss.Style // Matcher regex pattern
		HookArrow        lipgloss.Style // Arrow indicator
		HookDetail       lipgloss.Style // Decision detail text
		HookOK           lipgloss.Style // "OK" status
		HookDenied       lipgloss.Style // "Denied" status
		HookDeniedLabel  lipgloss.Style // "Hook" label when denied
		HookDeniedReason lipgloss.Style // Denied reason text
		HookRewrote      lipgloss.Style // "Rewrote Input" indicator

		// Action verb colors for tool-call headers.
		ActionCreate  lipgloss.Style // Constructive actions (e.g. "Add", "Create")
		ActionDestroy lipgloss.Style // Destructive actions (e.g. "Remove", "Delete")

		// Tool result helpers.
		ResultEmpty      lipgloss.Style // "No results" placeholder
		ResultTruncation lipgloss.Style // "… and N more" truncation line
		ResultItemName   lipgloss.Style // Item name (left column in result lists)
		ResultItemDesc   lipgloss.Style // Item description (right column)
	}

	// Dialog styles
	Dialog struct {
		Title              lipgloss.Style
		TitleText          lipgloss.Style
		TitleError         lipgloss.Style
		TitleAccent        lipgloss.Style
		TitleLineBase      lipgloss.Style // Base for the gradient ╱╱╱ next to dialog titles
		TitleGradFromColor color.Color    // Default dialog title ╱╱╱ gradient start
		TitleGradToColor   color.Color    // Default dialog title ╱╱╱ gradient end
		// View is the main content area style.
		View          lipgloss.Style
		PrimaryText   lipgloss.Style
		SecondaryText lipgloss.Style
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

		Spinner lipgloss.Style

		// ContentPanel is used for content blocks with subtle background.
		ContentPanel lipgloss.Style

		// Scrollbar styles for scrollable content.
		ScrollbarThumb lipgloss.Style
		ScrollbarTrack lipgloss.Style

		// Arguments
		Arguments struct {
			Content                  lipgloss.Style
			Description              lipgloss.Style
			InputLabelBlurred        lipgloss.Style
			InputLabelFocused        lipgloss.Style
			InputRequiredMarkBlurred lipgloss.Style
			InputRequiredMarkFocused lipgloss.Style
		}

		// ListItem styles the info-text rendered alongside list items (commands,
		// models, reasoning options). Sessions have their own overrides below.
		ListItem struct {
			InfoBlurred lipgloss.Style
			InfoFocused lipgloss.Style
		}

		Models struct {
			ConfiguredText lipgloss.Style // "Configured" badge shown on the ModelGroup header
		}

		Permissions struct {
			KeyText   lipgloss.Style // Left key cell of a key/value row
			ValueText lipgloss.Style // Right value cell of a key/value row
			ParamsBg  color.Color    // Background color behind highlighted JSON parameters
		}

		Quit struct {
			Content lipgloss.Style // Wrapper for the quit dialog's inner content
			Frame   lipgloss.Style // Outer rounded border framing the quit dialog
		}

		APIKey struct {
			Spinner lipgloss.Style // Loading spinner while validating the key
		}

		OAuth struct {
			Spinner      lipgloss.Style // Loading spinner
			Instructions lipgloss.Style // Emphasized instruction text
			UserCode     lipgloss.Style // Prominent user code display
			Success      lipgloss.Style // Positive status text (e.g. "Authentication successful!")
			Link         lipgloss.Style // Underlined verification URL
			Enter        lipgloss.Style // "enter" keyword highlight in instructions
			ErrorText    lipgloss.Style // Error message when authentication fails
			StatusText   lipgloss.Style // Narrative status text ("Initializing...", "Verifying...", etc.)
			UserCodeBg   color.Color    // Background color of the centered user-code box
		}

		ImagePreview lipgloss.Style

		Sessions struct {
			// styles for when we are in delete mode
			DeletingView                   lipgloss.Style
			DeletingItemFocused            lipgloss.Style
			DeletingItemBlurred            lipgloss.Style
			DeletingTitle                  lipgloss.Style
			DeletingMessage                lipgloss.Style
			DeletingTitleGradientFromColor color.Color
			DeletingTitleGradientToColor   color.Color

			// styles for when we are in update mode
			RenamingView                   lipgloss.Style
			RenamingingItemFocused         lipgloss.Style
			RenamingItemBlurred            lipgloss.Style
			RenamingingTitle               lipgloss.Style
			RenamingingMessage             lipgloss.Style
			RenamingTitleGradientFromColor color.Color
			RenamingTitleGradientToColor   color.Color
			RenamingPlaceholder            lipgloss.Style

			InfoBlurred lipgloss.Style // Timestamp text on unfocused session items
			InfoFocused lipgloss.Style // Timestamp text on the focused session item
		}
	}

	// Status bar and help
	Status struct {
		Help lipgloss.Style

		ErrorIndicator   lipgloss.Style
		WarnIndicator    lipgloss.Style
		InfoIndicator    lipgloss.Style
		UpdateIndicator  lipgloss.Style
		SuccessIndicator lipgloss.Style

		ErrorMessage   lipgloss.Style
		WarnMessage    lipgloss.Style
		InfoMessage    lipgloss.Style
		UpdateMessage  lipgloss.Style
		SuccessMessage lipgloss.Style
	}

	// Completions popup styles
	Completions struct {
		Normal  lipgloss.Style
		Focused lipgloss.Style
		Match   lipgloss.Style
	}

	// Attachments styles
	Attachments struct {
		Normal   lipgloss.Style
		Image    lipgloss.Style
		Text     lipgloss.Style
		Deleting lipgloss.Style
	}

	// Pills styles for todo/queue pills
	Pills struct {
		Base               lipgloss.Style // Base pill style with padding
		Focused            lipgloss.Style // Focused pill with visible border
		Blurred            lipgloss.Style // Blurred pill with hidden border
		QueueItemPrefix    lipgloss.Style // Prefix for queue list items
		QueueItemText      lipgloss.Style // Queue list item body text
		QueueLabel         lipgloss.Style // "N Queued" label text
		QueueIconBase      lipgloss.Style // Base style for queue gradient triangles
		QueueGradFromColor color.Color    // Start color for queue indicator gradient
		QueueGradToColor   color.Color    // End color for queue indicator gradient
		TodoLabel          lipgloss.Style // "To-Do" label
		TodoProgress       lipgloss.Style // Todo ratio (e.g. "2/5")
		TodoCurrentTask    lipgloss.Style // Current in-progress task name
		TodoSpinner        lipgloss.Style // Todo spinner style
		HelpKey            lipgloss.Style // Keystroke hint style
		HelpText           lipgloss.Style // Help action text style
		Area               lipgloss.Style // Pills area container
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
		error   = charmtone.Sriracha
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

		greenLight = charmtone.Bok
		green      = charmtone.Julep
		greenDark  = charmtone.Guac
		// greenLight = charmtone.Bok

		red     = charmtone.Coral
		redDark = charmtone.Sriracha
		// redLight = charmtone.Salmon
		// cherry   = charmtone.Cherry
	)

	normalBorder := lipgloss.NormalBorder()

	base := lipgloss.NewStyle().Foreground(fgBase)
	muted := lipgloss.NewStyle().Foreground(fgMuted)
	subtle := lipgloss.NewStyle().Foreground(fgSubtle)

	s := Styles{}

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
				Color: new(charmtone.Smoke.Hex()),
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
				Color:       new(charmtone.Malibu.Hex()),
				Bold:        new(true),
			},
		},
		H1: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          " ",
				Suffix:          " ",
				Color:           new(charmtone.Zest.Hex()),
				BackgroundColor: new(charmtone.Charple.Hex()),
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
				Color:  new(charmtone.Guac.Hex()),
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
			Color:  new(charmtone.Charcoal.Hex()),
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
			Color:     new(charmtone.Zinc.Hex()),
			Underline: new(true),
		},
		LinkText: ansi.StylePrimitive{
			Color: new(charmtone.Guac.Hex()),
			Bold:  new(true),
		},
		Image: ansi.StylePrimitive{
			Color:     new(charmtone.Cheeky.Hex()),
			Underline: new(true),
		},
		ImageText: ansi.StylePrimitive{
			Color:  new(charmtone.Squid.Hex()),
			Format: "Image: {{.text}} →",
		},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          " ",
				Suffix:          " ",
				Color:           new(charmtone.Coral.Hex()),
				BackgroundColor: new(charmtone.Charcoal.Hex()),
			},
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color: new(charmtone.Charcoal.Hex()),
				},
				Margin: new(uint(defaultMargin)),
			},
			Chroma: &ansi.Chroma{
				Text: ansi.StylePrimitive{
					Color: new(charmtone.Smoke.Hex()),
				},
				Error: ansi.StylePrimitive{
					Color:           new(charmtone.Butter.Hex()),
					BackgroundColor: new(charmtone.Sriracha.Hex()),
				},
				Comment: ansi.StylePrimitive{
					Color: new(charmtone.Oyster.Hex()),
				},
				CommentPreproc: ansi.StylePrimitive{
					Color: new(charmtone.Bengal.Hex()),
				},
				Keyword: ansi.StylePrimitive{
					Color: new(charmtone.Malibu.Hex()),
				},
				KeywordReserved: ansi.StylePrimitive{
					Color: new(charmtone.Pony.Hex()),
				},
				KeywordNamespace: ansi.StylePrimitive{
					Color: new(charmtone.Pony.Hex()),
				},
				KeywordType: ansi.StylePrimitive{
					Color: new(charmtone.Guppy.Hex()),
				},
				Operator: ansi.StylePrimitive{
					Color: new(charmtone.Salmon.Hex()),
				},
				Punctuation: ansi.StylePrimitive{
					Color: new(charmtone.Zest.Hex()),
				},
				Name: ansi.StylePrimitive{
					Color: new(charmtone.Smoke.Hex()),
				},
				NameBuiltin: ansi.StylePrimitive{
					Color: new(charmtone.Cheeky.Hex()),
				},
				NameTag: ansi.StylePrimitive{
					Color: new(charmtone.Mauve.Hex()),
				},
				NameAttribute: ansi.StylePrimitive{
					Color: new(charmtone.Hazy.Hex()),
				},
				NameClass: ansi.StylePrimitive{
					Color:     new(charmtone.Salt.Hex()),
					Underline: new(true),
					Bold:      new(true),
				},
				NameDecorator: ansi.StylePrimitive{
					Color: new(charmtone.Citron.Hex()),
				},
				NameFunction: ansi.StylePrimitive{
					Color: new(charmtone.Guac.Hex()),
				},
				LiteralNumber: ansi.StylePrimitive{
					Color: new(charmtone.Julep.Hex()),
				},
				LiteralString: ansi.StylePrimitive{
					Color: new(charmtone.Cumin.Hex()),
				},
				LiteralStringEscape: ansi.StylePrimitive{
					Color: new(charmtone.Bok.Hex()),
				},
				GenericDeleted: ansi.StylePrimitive{
					Color: new(charmtone.Coral.Hex()),
				},
				GenericEmph: ansi.StylePrimitive{
					Italic: new(true),
				},
				GenericInserted: ansi.StylePrimitive{
					Color: new(charmtone.Guac.Hex()),
				},
				GenericStrong: ansi.StylePrimitive{
					Bold: new(true),
				},
				GenericSubheading: ansi.StylePrimitive{
					Color: new(charmtone.Squid.Hex()),
				},
				Background: ansi.StylePrimitive{
					BackgroundColor: new(charmtone.Charcoal.Hex()),
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
	plainBg := new(bgBaseLighter.Hex())
	plainFg := new(fgMuted.Hex())
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
	s.ToolCallSuccess = lipgloss.NewStyle().Foreground(green).SetString(ToolSuccess)

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
	s.Tool.IconPending = base.Foreground(greenDark).SetString(ToolPending)
	s.Tool.IconSuccess = base.Foreground(green).SetString(ToolSuccess)
	s.Tool.IconError = base.Foreground(redDark).SetString(ToolError)
	s.Tool.IconCancelled = muted.SetString(ToolPending)

	s.Tool.NameNormal = base.Foreground(blue)
	s.Tool.NameNested = base.Foreground(blue)

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

	s.Tool.ErrorTag = base.Padding(0, 1).Background(red).Foreground(white)
	s.Tool.ErrorMessage = base.Foreground(fgHalfMuted)

	// Diff and multi-edit styles
	s.Tool.DiffTruncation = muted.Background(bgBaseLighter).PaddingLeft(2)
	s.Tool.NoteTag = base.Padding(0, 1).Background(info).Foreground(white)
	s.Tool.NoteMessage = base.Foreground(fgHalfMuted)

	// Job header styles
	s.Tool.JobIconPending = base.Foreground(greenDark)
	s.Tool.JobIconError = base.Foreground(redDark)
	s.Tool.JobIconSuccess = base.Foreground(green)
	s.Tool.JobToolName = base.Foreground(blue)
	s.Tool.JobAction = base.Foreground(blueDark)
	s.Tool.JobPID = muted
	s.Tool.JobDescription = subtle

	// Agent task styles
	s.Tool.AgentTaskTag = base.Bold(true).Padding(0, 1).MarginLeft(2).Background(blueLight).Foreground(white)
	s.Tool.AgentPrompt = muted

	// Agentic fetch styles
	s.Tool.AgenticFetchPromptTag = base.Bold(true).Padding(0, 1).MarginLeft(2).Background(green).Foreground(border)

	// Todo styles
	s.Tool.TodoRatio = base.Foreground(blueDark)
	s.Tool.TodoCompletedIcon = base.Foreground(green)
	s.Tool.TodoInProgressIcon = base.Foreground(greenDark)
	s.Tool.TodoPendingIcon = base.Foreground(fgMuted)
	s.Tool.TodoStatusNote = lipgloss.NewStyle().Foreground(fgSubtle)
	s.Tool.TodoItem = lipgloss.NewStyle().Foreground(fgBase)
	s.Tool.TodoJustStarted = lipgloss.NewStyle().Foreground(fgBase)

	// MCP styles
	s.Tool.MCPName = base.Foreground(blue)
	s.Tool.MCPToolName = base.Foreground(blueDark)
	s.Tool.MCPArrow = base.Foreground(blue).SetString(ArrowRightIcon)

	// Loading indicators for images, skills
	s.Tool.ResourceLoadedText = base.Foreground(green)
	s.Tool.ResourceLoadedIndicator = base.Foreground(greenDark)
	s.Tool.ResourceName = base
	s.Tool.MediaType = base
	s.Tool.ResourceSize = base.Foreground(fgMuted)

	// Tool-call action verbs and result-list styling.
	s.Tool.ActionCreate = lipgloss.NewStyle().Foreground(greenLight)
	s.Tool.ActionDestroy = lipgloss.NewStyle().Foreground(red)
	s.Tool.ResultEmpty = lipgloss.NewStyle().Foreground(fgSubtle)
	s.Tool.ResultTruncation = lipgloss.NewStyle().Foreground(fgSubtle)
	s.Tool.ResultItemName = lipgloss.NewStyle().Foreground(fgBase)
	s.Tool.ResultItemDesc = lipgloss.NewStyle().Foreground(fgSubtle)

	// Hook styles
	s.Tool.HookLabel = base.Foreground(green)
	s.Tool.HookName = base
	s.Tool.HookMatcher = base.Foreground(fgMuted)
	s.Tool.HookArrow = base.Foreground(green)
	s.Tool.HookDetail = base.Foreground(fgMuted)
	s.Tool.HookOK = base.Foreground(charmtone.Guac)
	s.Tool.HookDenied = base.Foreground(charmtone.Sriracha)
	s.Tool.HookDeniedLabel = base.Foreground(red)
	s.Tool.HookDeniedReason = base.Foreground(charmtone.Iron)
	s.Tool.HookRewrote = base.Foreground(charmtone.Iron)

	// Buttons
	s.Button.Focused = lipgloss.NewStyle().Foreground(white).Background(secondary)
	s.Button.Blurred = lipgloss.NewStyle().Foreground(fgBase).Background(bgSubtle)

	// Editor
	s.Editor.PromptNormalFocused = lipgloss.NewStyle().Foreground(greenDark).SetString("::: ")
	s.Editor.PromptNormalBlurred = s.Editor.PromptNormalFocused.Foreground(fgMuted)
	s.Editor.PromptYoloIconFocused = lipgloss.NewStyle().MarginRight(1).Foreground(charmtone.Oyster).Background(charmtone.Citron).Bold(true).SetString(" ! ")
	s.Editor.PromptYoloIconBlurred = s.Editor.PromptYoloIconFocused.Foreground(charmtone.Pepper).Background(charmtone.Squid)
	s.Editor.PromptYoloDotsFocused = lipgloss.NewStyle().MarginRight(1).Foreground(charmtone.Zest).SetString(":::")
	s.Editor.PromptYoloDotsBlurred = s.Editor.PromptYoloDotsFocused.Foreground(charmtone.Squid)

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
	s.Section.Line = base.Foreground(charmtone.Charcoal)

	// Initialize
	s.Initialize.Header = base
	s.Initialize.Content = muted
	s.Initialize.Accent = base.Foreground(greenDark)

	// ResourceGroup (LSP/MCP/skills sidebar lists).
	s.Resource.Heading = lipgloss.NewStyle().Foreground(charmtone.Oyster)
	s.Resource.Name = lipgloss.NewStyle().Foreground(charmtone.Squid)
	s.Resource.StatusText = lipgloss.NewStyle().Foreground(charmtone.Oyster)
	s.Resource.OfflineIcon = lipgloss.NewStyle().Foreground(charmtone.Iron).SetString("●")
	s.Resource.BusyIcon = s.Resource.OfflineIcon.Foreground(charmtone.Citron)
	s.Resource.ErrorIcon = s.Resource.OfflineIcon.Foreground(charmtone.Coral)
	s.Resource.OnlineIcon = s.Resource.OfflineIcon.Foreground(charmtone.Guac)
	s.Resource.DisabledIcon = lipgloss.NewStyle().Foreground(fgMuted).SetString("●")
	s.Resource.AdditionalText = lipgloss.NewStyle().Foreground(charmtone.Oyster)
	s.Resource.CapabilityCount = lipgloss.NewStyle().Foreground(fgSubtle)
	s.Resource.RowTitleBase = lipgloss.NewStyle().Foreground(fgBase)
	s.Resource.RowDescBase = lipgloss.NewStyle().Foreground(fgBase)
	s.Resource.DefaultTitleFg = fgMuted
	s.Resource.DefaultDescFg = fgSubtle

	// LSP
	s.LSP.ErrorDiagnostic = base.Foreground(redDark)
	s.LSP.WarningDiagnostic = base.Foreground(warning)
	s.LSP.HintDiagnostic = base.Foreground(fgHalfMuted)
	s.LSP.InfoDiagnostic = base.Foreground(info)

	// Files
	s.Files.Path = lipgloss.NewStyle().Foreground(fgMuted)
	s.Files.Additions = lipgloss.NewStyle().Foreground(greenDark)
	s.Files.Deletions = lipgloss.NewStyle().Foreground(redDark)
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
		BorderForeground(primary).BorderStyle(normalBorder)
	s.Messages.UserFocused = s.Messages.NoContent.PaddingLeft(1).BorderLeft(true).
		BorderForeground(primary).BorderStyle(messageFocussedBorder)
	s.Messages.AssistantBlurred = s.Messages.NoContent.PaddingLeft(2)
	s.Messages.AssistantFocused = s.Messages.NoContent.PaddingLeft(1).BorderLeft(true).
		BorderForeground(greenDark).BorderStyle(messageFocussedBorder)
	s.Messages.Thinking = lipgloss.NewStyle().MaxHeight(10)
	s.Messages.ErrorTag = lipgloss.NewStyle().Padding(0, 1).
		Background(red).Foreground(white)
	s.Messages.ErrorTitle = lipgloss.NewStyle().Foreground(fgHalfMuted)
	s.Messages.ErrorDetails = lipgloss.NewStyle().Foreground(fgSubtle)

	// Message item styles
	s.Messages.ToolCallFocused = muted.PaddingLeft(1).
		BorderStyle(messageFocussedBorder).
		BorderLeft(true).
		BorderForeground(greenDark)
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
	s.TextSelection = lipgloss.NewStyle().Foreground(charmtone.Salt).Background(charmtone.Charple)

	// Dialog styles
	s.Dialog.Title = base.Padding(0, 1).Foreground(primary)
	s.Dialog.TitleText = base.Foreground(primary)
	s.Dialog.TitleError = base.Foreground(red)
	s.Dialog.TitleAccent = base.Foreground(green).Bold(true)
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
	s.Dialog.APIKey.Spinner = base.Foreground(green)

	// OAuth dialog
	s.Dialog.OAuth.Spinner = base.Foreground(greenLight)
	s.Dialog.OAuth.Instructions = lipgloss.NewStyle().Foreground(white)
	s.Dialog.OAuth.UserCode = lipgloss.NewStyle().Bold(true).Foreground(white)
	s.Dialog.OAuth.Success = lipgloss.NewStyle().Foreground(greenLight)
	s.Dialog.OAuth.Link = lipgloss.NewStyle().Foreground(greenDark).Underline(true)
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

	s.Dialog.Sessions.DeletingTitle = s.Dialog.Title.Foreground(red)
	s.Dialog.Sessions.DeletingView = s.Dialog.View.BorderForeground(red)
	s.Dialog.Sessions.DeletingMessage = base.Padding(1)
	s.Dialog.Sessions.DeletingTitleGradientFromColor = red
	s.Dialog.Sessions.DeletingTitleGradientToColor = primary
	s.Dialog.Sessions.DeletingItemBlurred = s.Dialog.NormalItem.Foreground(fgSubtle)
	s.Dialog.Sessions.DeletingItemFocused = s.Dialog.SelectedItem.Background(red).Foreground(charmtone.Butter)

	s.Dialog.Sessions.RenamingingTitle = s.Dialog.Title.Foreground(charmtone.Zest)
	s.Dialog.Sessions.RenamingView = s.Dialog.View.BorderForeground(charmtone.Zest)
	s.Dialog.Sessions.RenamingingMessage = base.Padding(1)
	s.Dialog.Sessions.RenamingTitleGradientFromColor = charmtone.Zest
	s.Dialog.Sessions.RenamingTitleGradientToColor = charmtone.Bok
	s.Dialog.Sessions.RenamingItemBlurred = s.Dialog.NormalItem.Foreground(fgSubtle)
	s.Dialog.Sessions.RenamingingItemFocused = s.Dialog.SelectedItem.UnsetBackground().UnsetForeground()
	s.Dialog.Sessions.RenamingPlaceholder = base.Foreground(charmtone.Squid)
	s.Dialog.Sessions.InfoBlurred = lipgloss.NewStyle().Foreground(fgSubtle)
	s.Dialog.Sessions.InfoFocused = lipgloss.NewStyle().Foreground(fgBase)

	s.Status.Help = lipgloss.NewStyle().Padding(0, 1)
	s.Status.SuccessIndicator = base.Foreground(bgSubtle).Background(green).Padding(0, 1).Bold(true).SetString("OKAY!")
	s.Status.InfoIndicator = s.Status.SuccessIndicator
	s.Status.UpdateIndicator = s.Status.SuccessIndicator.SetString("HEY!")
	s.Status.WarnIndicator = s.Status.SuccessIndicator.Foreground(bgOverlay).Background(yellow).SetString("WARNING")
	s.Status.ErrorIndicator = s.Status.SuccessIndicator.Foreground(bgBase).Background(red).SetString("ERROR")
	s.Status.SuccessMessage = base.Foreground(bgSubtle).Background(greenDark).Padding(0, 1)
	s.Status.InfoMessage = s.Status.SuccessMessage
	s.Status.UpdateMessage = s.Status.SuccessMessage
	s.Status.WarnMessage = s.Status.SuccessMessage.Foreground(bgOverlay).Background(warning)
	s.Status.ErrorMessage = s.Status.SuccessMessage.Foreground(white).Background(redDark)

	// Completions styles
	s.Completions.Normal = base.Background(bgSubtle).Foreground(fgBase)
	s.Completions.Focused = base.Background(primary).Foreground(white)
	s.Completions.Match = base.Underline(true)

	// Attachments styles
	attachmentIconStyle := base.Foreground(bgSubtle).Background(green).Padding(0, 1)
	s.Attachments.Image = attachmentIconStyle.SetString(ImageIcon)
	s.Attachments.Text = attachmentIconStyle.SetString(TextIcon)
	s.Attachments.Normal = base.Padding(0, 1).MarginRight(1).Background(fgMuted).Foreground(fgBase)
	s.Attachments.Deleting = base.Padding(0, 1).Bold(true).Background(red).Foreground(fgBase)

	// Pills styles
	s.Pills.Base = base.Padding(0, 1)
	s.Pills.Focused = base.Padding(0, 1).BorderStyle(lipgloss.RoundedBorder()).BorderForeground(bgOverlay)
	s.Pills.Blurred = base.Padding(0, 1).BorderStyle(lipgloss.HiddenBorder())
	s.Pills.QueueItemPrefix = lipgloss.NewStyle().Foreground(fgMuted).SetString("  •")
	s.Pills.QueueItemText = lipgloss.NewStyle().Foreground(fgMuted)
	s.Pills.QueueLabel = lipgloss.NewStyle().Foreground(fgBase)
	s.Pills.QueueIconBase = lipgloss.NewStyle().Foreground(fgBase)
	s.Pills.QueueGradFromColor = redDark
	s.Pills.QueueGradToColor = secondary
	s.Pills.TodoLabel = lipgloss.NewStyle().Foreground(fgBase)
	s.Pills.TodoProgress = lipgloss.NewStyle().Foreground(fgMuted)
	s.Pills.TodoCurrentTask = lipgloss.NewStyle().Foreground(fgSubtle)
	s.Pills.TodoSpinner = lipgloss.NewStyle().Foreground(greenDark)
	s.Pills.HelpKey = lipgloss.NewStyle().Foreground(fgMuted)
	s.Pills.HelpText = lipgloss.NewStyle().Foreground(fgSubtle)
	s.Pills.Area = base

	return s
}

func chromaStyle(style ansi.StylePrimitive) string {
	var s strings.Builder

	if style.Color != nil {
		s.WriteString(*style.Color)
	}
	if style.BackgroundColor != nil {
		if s.Len() > 0 {
			s.WriteString(" ")
		}
		s.WriteString("bg:")
		s.WriteString(*style.BackgroundColor)
	}
	if style.Italic != nil && *style.Italic {
		if s.Len() > 0 {
			s.WriteString(" ")
		}
		s.WriteString("italic")
	}
	if style.Bold != nil && *style.Bold {
		if s.Len() > 0 {
			s.WriteString(" ")
		}
		s.WriteString("bold")
	}
	if style.Underline != nil && *style.Underline {
		if s.Len() > 0 {
			s.WriteString(" ")
		}
		s.WriteString("underline")
	}

	return s.String()
}
