package ui

import (
	"github.com/charmbracelet/bubbles/v2/filepicker"
	"github.com/charmbracelet/bubbles/v2/help"
	"github.com/charmbracelet/bubbles/v2/textarea"
	"github.com/charmbracelet/bubbles/v2/textinput"
	"github.com/charmbracelet/crush/internal/tui/exp/diffview"
	"github.com/charmbracelet/glamour/v2/ansi"
	"github.com/charmbracelet/lipgloss/v2"
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

	ToolPending string = "●"
	ToolSuccess string = "✓"
	ToolError   string = "×"

	BorderThin  string = "│"
	BorderThick string = "▌"
)

type Styles struct {
	WindowTooSmall lipgloss.Style

	// Reusable text styles
	Base   lipgloss.Style
	Muted  lipgloss.Style
	Subtle lipgloss.Style

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
	Markdown ansi.StyleConfig

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
	ButtonSelected   lipgloss.Style
	ButtonUnselected lipgloss.Style

	// Borders
	BorderFocus lipgloss.Style
	BorderBlur  lipgloss.Style
}

func DefaultStyles() Styles {
	var (
		// primary   = charmtone.Charple
		secondary = charmtone.Dolly
		// tertiary  = charmtone.Bok
		// accent    = charmtone.Zest

		// Backgrounds
		bgBase        = charmtone.Pepper
		bgBaseLighter = charmtone.BBQ
		bgSubtle      = charmtone.Charcoal
		// bgOverlay     = charmtone.Iron

		// Foregrounds
		fgBase      = charmtone.Ash
		fgMuted     = charmtone.Squid
		fgHalfMuted = charmtone.Smoke
		fgSubtle    = charmtone.Oyster
		// fgSelected  = charmtone.Salt

		// Borders
		// border      = charmtone.Charcoal
		borderFocus = charmtone.Charple

		// Status
		// success = charmtone.Guac
		// error   = charmtone.Sriracha
		// warning = charmtone.Zest
		// info    = charmtone.Malibu

		// Colors
		white = charmtone.Butter

		blueLight = charmtone.Sardine
		blue      = charmtone.Malibu

		// yellow = charmtone.Mustard
		// citron = charmtone.Citron

		green     = charmtone.Julep
		greenDark = charmtone.Guac
		// greenLight = charmtone.Bok

		// red      = charmtone.Coral
		redDark = charmtone.Sriracha
		// redLight = charmtone.Salmon
		// cherry   = charmtone.Cherry
	)

	s := Styles{}

	// borders
	s.FocusedMessageBorder = lipgloss.Border{Left: BorderThick}

	// text presets
	s.Base = lipgloss.NewStyle().Foreground(fgBase)
	s.Muted = lipgloss.NewStyle().Foreground(fgMuted)
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

	// Buttons
	s.ButtonSelected = lipgloss.NewStyle().Foreground(white).Background(secondary)
	s.ButtonUnselected = s.Base.Background(bgSubtle)

	// Borders
	s.BorderFocus = lipgloss.NewStyle().BorderForeground(borderFocus).Border(lipgloss.RoundedBorder()).Padding(1, 2)

	return s
}
