// Package styles define styling and theming for the project.
package styles

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	"charm.land/glamour/v2/ansi"
	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/charmbracelet/crush/internal/ui/diffview"
)

const (
	CheckIcon       string = "✓"
	SpinnerIcon     string = "⋯"
	LoadingIcon     string = "⟳"
	ModelIcon       string = "◇"
	HypercreditIcon string = "◆"

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
		Hypercredit       lipgloss.Style // Style for Hypercredit count (◆ N)
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
		HypercreditIcon  lipgloss.Style // Hypercredit icon (◆)
		HypercreditText  lipgloss.Style // Remaining Hypercredits text
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

// hex returns a pointer to the "#rrggbb" representation of c. It's used to
// satisfy glamour's string-pointer API when configuring markdown colors
// from the theme palette.
func hex(c color.Color) *string {
	r, g, b, _ := c.RGBA()
	s := fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
	return &s
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
