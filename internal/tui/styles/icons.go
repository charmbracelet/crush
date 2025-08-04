package styles

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

	// Tool call icons
	ToolPending string = "●"
	ToolSuccess string = "✓"
	ToolError   string = "×"

	BorderThin string = "│"
)

var AllIcons = []string{
	CheckIcon,
	ErrorIcon,
	WarningIcon,
	InfoIcon,
	HintIcon,
	SpinnerIcon,
	LoadingIcon,
	DocumentIcon,
	ModelIcon,

	// Tool call icons
	ToolPending,
	ToolSuccess,
	ToolError,

	BorderThin,
}
