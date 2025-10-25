package styles

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/charmbracelet/bubbles/v2/filepicker"
	"github.com/charmbracelet/bubbles/v2/help"
	"github.com/charmbracelet/bubbles/v2/textarea"
	"github.com/charmbracelet/bubbles/v2/textinput"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/tui/exp/diffview"
	"github.com/charmbracelet/glamour/v2/ansi"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/x/exp/charmtone"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/rivo/uniseg"
)

const (
	defaultListIndent      = 2
	defaultListLevelIndent = 4
	defaultMargin          = 2
)

type MarkdownColors struct {
	// Basic Markdown elements
	Document     color.Color
	Heading      color.Color
	H1Bg         color.Color
	H1Fg         color.Color
	H6           color.Color
	Rule         color.Color
	BlockQuote   color.Color
	Item         color.Color
	Enumeration  color.Color
	TaskTicked   color.Color
	TaskUnticked color.Color
	Link         color.Color
	LinkText     color.Color
	Image        color.Color
	ImageText    color.Color
	Code         color.Color
	CodeBg       color.Color
	CodeBlock    color.Color

	// Chroma syntax highlighting
	Chroma ChromaColors
}

type ChromaColors struct {
	Text                color.Color
	Error               color.Color
	Comment             color.Color
	CommentPreproc      color.Color
	Keyword             color.Color
	KeywordReserved     color.Color
	KeywordNamespace    color.Color
	KeywordType         color.Color
	Operator            color.Color
	Punctuation         color.Color
	Name                color.Color
	NameBuiltin         color.Color
	NameTag             color.Color
	NameAttribute       color.Color
	NameClass           color.Color
	NameDecorator       color.Color
	NameFunction        color.Color
	LiteralNumber       color.Color
	LiteralString       color.Color
	LiteralStringEscape color.Color
	GenericDeleted      color.Color
	GenericInserted     color.Color
	GenericSubheading   color.Color
	Background          color.Color
}

type Theme struct {
	Name   string
	IsDark bool

	Primary   color.Color
	Secondary color.Color
	Tertiary  color.Color
	Accent    color.Color

	BgBase        color.Color
	BgBaseLighter color.Color
	BgSubtle      color.Color
	BgOverlay     color.Color

	FgBase      color.Color
	FgMuted     color.Color
	FgHalfMuted color.Color
	FgSubtle    color.Color
	FgSelected  color.Color

	Border      color.Color
	BorderFocus color.Color

	Success color.Color
	Error   color.Color
	Warning color.Color
	Info    color.Color

	// Colors
	// White
	White color.Color

	// Blues
	BlueLight color.Color
	Blue      color.Color

	// Yellows
	Yellow color.Color
	Citron color.Color

	// Greens
	Green      color.Color
	GreenDark  color.Color
	GreenLight color.Color

	// Reds
	Red      color.Color
	RedDark  color.Color
	RedLight color.Color
	Cherry   color.Color

	// Markdown colors
	Markdown MarkdownColors

	// Text selection.
	TextSelection lipgloss.Style

	// LSP and MCP status indicators.
	ItemOfflineIcon lipgloss.Style
	ItemBusyIcon    lipgloss.Style
	ItemErrorIcon   lipgloss.Style
	ItemOnlineIcon  lipgloss.Style

	// Editor: Yolo Mode
	YoloIconFocused lipgloss.Style
	YoloIconBlurred lipgloss.Style
	YoloDotsFocused lipgloss.Style
	YoloDotsBlurred lipgloss.Style

	styles *Styles
}

type Styles struct {
	Base         lipgloss.Style
	SelectedBase lipgloss.Style

	Title        lipgloss.Style
	Subtitle     lipgloss.Style
	Text         lipgloss.Style
	TextSelected lipgloss.Style
	Muted        lipgloss.Style
	Subtle       lipgloss.Style

	Success lipgloss.Style
	Error   lipgloss.Style
	Warning lipgloss.Style
	Info    lipgloss.Style

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
}

func (t *Theme) S() *Styles {
	if t.styles == nil {
		t.styles = t.buildStyles()
	}
	return t.styles
}

func (t *Theme) buildStyles() *Styles {
	base := lipgloss.NewStyle().
		Foreground(t.FgBase)
	return &Styles{
		Base: base,

		SelectedBase: base.Background(t.Primary),

		Title: base.
			Foreground(t.Accent).
			Bold(true),

		Subtitle: base.
			Foreground(t.Secondary).
			Bold(true),

		Text:         base,
		TextSelected: base.Background(t.Primary).Foreground(t.FgSelected),

		Muted: base.Foreground(t.FgMuted),

		Subtle: base.Foreground(t.FgSubtle),

		Success: base.Foreground(t.Success),

		Error: base.Foreground(t.Error),

		Warning: base.Foreground(t.Warning),

		Info: base.Foreground(t.Info),

		TextInput: textinput.Styles{
			Focused: textinput.StyleState{
				Text:        base,
				Placeholder: base.Foreground(t.FgSubtle),
				Prompt:      base.Foreground(t.Tertiary),
				Suggestion:  base.Foreground(t.FgSubtle),
			},
			Blurred: textinput.StyleState{
				Text:        base.Foreground(t.FgMuted),
				Placeholder: base.Foreground(t.FgSubtle),
				Prompt:      base.Foreground(t.FgMuted),
				Suggestion:  base.Foreground(t.FgSubtle),
			},
			Cursor: textinput.CursorStyle{
				Color: t.Secondary,
				Shape: tea.CursorBar,
				Blink: true,
			},
		},
		TextArea: textarea.Styles{
			Focused: textarea.StyleState{
				Base:             base,
				Text:             base,
				LineNumber:       base.Foreground(t.FgSubtle),
				CursorLine:       base,
				CursorLineNumber: base.Foreground(t.FgSubtle),
				Placeholder:      base.Foreground(t.FgSubtle),
				Prompt:           base.Foreground(t.Tertiary),
			},
			Blurred: textarea.StyleState{
				Base:             base,
				Text:             base.Foreground(t.FgMuted),
				LineNumber:       base.Foreground(t.FgMuted),
				CursorLine:       base,
				CursorLineNumber: base.Foreground(t.FgMuted),
				Placeholder:      base.Foreground(t.FgSubtle),
				Prompt:           base.Foreground(t.FgMuted),
			},
			Cursor: textarea.CursorStyle{
				Color: t.Secondary,
				Shape: tea.CursorBar,
				Blink: true,
			},
		},

		Markdown: ansi.StyleConfig{
			Document: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					// BlockPrefix: "\n",
					// BlockSuffix: "\n",
					Color: colorHex(t.getMdColor(t.Markdown.Document, t.FgBase)),
				},
				// Margin: uintPtr(defaultMargin),
			},
			BlockQuote: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color: colorHex(t.getMdColor(t.Markdown.BlockQuote, t.FgMuted)),
				},
				Indent:      uintPtr(1),
				IndentToken: stringPtr("│ "),
			},
			List: ansi.StyleList{
				LevelIndent: defaultListIndent,
			},
			Heading: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					BlockSuffix: "\n",
					Color:       colorHex(t.getMdColor(t.Markdown.Heading, t.Accent)),
					Bold:        boolPtr(true),
				},
			},
			H1: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Prefix:          " ",
					Suffix:          " ",
					Color:           colorHex(t.getMdColor(t.Markdown.H1Fg, t.Markdown.H1Bg)),
					BackgroundColor: colorHex(t.getMdColor(t.Markdown.H1Bg, t.Primary)),
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
					Color:  colorHex(t.getMdColor(t.Markdown.H6, t.FgMuted)),
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
				Color:  colorHex(t.getMdColor(t.Markdown.Rule, t.Border)),
				Format: "\n--------\n",
			},
			Item: ansi.StylePrimitive{
				BlockPrefix: "• ",
				Color:       colorHex(t.getMdColor(t.Markdown.Item, t.FgBase)),
			},
			Enumeration: ansi.StylePrimitive{
				BlockPrefix: ". ",
				Color:       colorHex(t.getMdColor(t.Markdown.Enumeration, t.FgBase)),
			},
			Task: ansi.StyleTask{
				StylePrimitive: ansi.StylePrimitive{
					Color: colorHex(t.getMdColor(t.Markdown.TaskTicked, t.FgBase)),
				},
				Ticked:   "[✓] ",
				Unticked: "[ ] ",
			},
			Link: ansi.StylePrimitive{
				Color:     colorHex(t.getMdColor(t.Markdown.Link, t.Accent)),
				Underline: boolPtr(true),
			},
			LinkText: ansi.StylePrimitive{
				Color: colorHex(t.getMdColor(t.Markdown.LinkText, t.Accent)),
				Bold:  boolPtr(true),
			},
			Image: ansi.StylePrimitive{
				Color:     colorHex(t.getMdColor(t.Markdown.Image, t.Accent)),
				Underline: boolPtr(true),
			},
			ImageText: ansi.StylePrimitive{
				Color:  colorHex(t.getMdColor(t.Markdown.ImageText, t.FgMuted)),
				Format: "Image: {{.text}} →",
			},
			Code: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Prefix:          " ",
					Suffix:          " ",
					Color:           colorHex(t.getMdColor(t.Markdown.Code, t.Accent)),
					BackgroundColor: colorHex(t.getMdColor(t.Markdown.CodeBg, t.BgSubtle)),
				},
			},
			CodeBlock: ansi.StyleCodeBlock{
				StyleBlock: ansi.StyleBlock{
					StylePrimitive: ansi.StylePrimitive{
						Color: colorHex(t.getMdColor(t.Markdown.CodeBlock, t.FgBase)),
					},
					Margin: uintPtr(defaultMargin),
				},
				Chroma: &ansi.Chroma{
					Text: ansi.StylePrimitive{
						Color: colorHex(t.getMdColor(t.Markdown.Chroma.Text, t.FgBase)),
					},
					Error: ansi.StylePrimitive{
						Color:           colorHex(t.getMdColor(t.Markdown.Chroma.Error, t.Error)),
						BackgroundColor: stringPtr(charmtone.Sriracha.Hex()),
					},
					Comment: ansi.StylePrimitive{
						Color: colorHex(t.getMdColor(t.Markdown.Chroma.Comment, t.FgMuted)),
					},
					CommentPreproc: ansi.StylePrimitive{
						Color: colorHex(t.getMdColor(t.Markdown.Chroma.CommentPreproc, t.FgMuted)),
					},
					Keyword: ansi.StylePrimitive{
						Color: colorHex(t.getMdColor(t.Markdown.Chroma.Keyword, t.Accent)),
					},
					KeywordReserved: ansi.StylePrimitive{
						Color: colorHex(t.getMdColor(t.Markdown.Chroma.KeywordReserved, t.Accent)),
					},
					KeywordNamespace: ansi.StylePrimitive{
						Color: colorHex(t.getMdColor(t.Markdown.Chroma.KeywordNamespace, t.Accent)),
					},
					KeywordType: ansi.StylePrimitive{
						Color: colorHex(t.getMdColor(t.Markdown.Chroma.KeywordType, t.Secondary)),
					},
					Operator: ansi.StylePrimitive{
						Color: colorHex(t.getMdColor(t.Markdown.Chroma.Operator, t.FgBase)),
					},
					Punctuation: ansi.StylePrimitive{
						Color: colorHex(t.getMdColor(t.Markdown.Chroma.Punctuation, t.FgBase)),
					},
					Name: ansi.StylePrimitive{
						Color: colorHex(t.getMdColor(t.Markdown.Chroma.Name, t.FgBase)),
					},
					NameBuiltin: ansi.StylePrimitive{
						Color: colorHex(t.getMdColor(t.Markdown.Chroma.NameBuiltin, t.Accent)),
					},
					NameTag: ansi.StylePrimitive{
						Color: colorHex(t.getMdColor(t.Markdown.Chroma.NameTag, t.Secondary)),
					},
					NameAttribute: ansi.StylePrimitive{
						Color: colorHex(t.getMdColor(t.Markdown.Chroma.NameAttribute, t.Tertiary)),
					},
					NameClass: ansi.StylePrimitive{
						Color:     colorHex(t.getMdColor(t.Markdown.Chroma.NameClass, t.FgBase)),
						Underline: boolPtr(true),
						Bold:      boolPtr(true),
					},
					NameDecorator: ansi.StylePrimitive{
						Color: colorHex(t.getMdColor(t.Markdown.Chroma.NameDecorator, t.Tertiary)),
					},
					NameFunction: ansi.StylePrimitive{
						Color: colorHex(t.getMdColor(t.Markdown.Chroma.NameFunction, t.Accent)),
					},
					LiteralNumber: ansi.StylePrimitive{
						Color: colorHex(t.getMdColor(t.Markdown.Chroma.LiteralNumber, t.Secondary)),
					},
					LiteralString: ansi.StylePrimitive{
						Color: colorHex(t.getMdColor(t.Markdown.Chroma.LiteralString, t.Secondary)),
					},
					LiteralStringEscape: ansi.StylePrimitive{
						Color: colorHex(t.getMdColor(t.Markdown.Chroma.LiteralStringEscape, t.FgBase)),
					},
					GenericDeleted: ansi.StylePrimitive{
						Color: colorHex(t.getMdColor(t.Markdown.Chroma.GenericDeleted, t.Error)),
					},
					GenericEmph: ansi.StylePrimitive{
						Italic: boolPtr(true),
					},
					GenericInserted: ansi.StylePrimitive{
						Color: colorHex(t.getMdColor(t.Markdown.Chroma.GenericInserted, t.Success)),
					},
					GenericStrong: ansi.StylePrimitive{
						Bold: boolPtr(true),
					},
					GenericSubheading: ansi.StylePrimitive{
						Color: colorHex(t.getMdColor(t.Markdown.Chroma.GenericSubheading, t.FgMuted)),
					},
					Background: ansi.StylePrimitive{
						BackgroundColor: colorHex(t.getMdColor(t.Markdown.Chroma.Background, t.BgSubtle)),
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
		},

		Help: help.Styles{
			ShortKey:       base.Foreground(t.FgMuted),
			ShortDesc:      base.Foreground(t.FgSubtle),
			ShortSeparator: base.Foreground(t.Border),
			Ellipsis:       base.Foreground(t.Border),
			FullKey:        base.Foreground(t.FgMuted),
			FullDesc:       base.Foreground(t.FgSubtle),
			FullSeparator:  base.Foreground(t.Border),
		},

		Diff: diffview.Style{
			DividerLine: diffview.LineStyle{
				LineNumber: lipgloss.NewStyle().
					Foreground(t.FgHalfMuted).
					Background(t.BgBaseLighter),
				Code: lipgloss.NewStyle().
					Foreground(t.FgHalfMuted).
					Background(t.BgBaseLighter),
			},
			MissingLine: diffview.LineStyle{
				LineNumber: lipgloss.NewStyle().
					Background(t.BgBaseLighter),
				Code: lipgloss.NewStyle().
					Background(t.BgBaseLighter),
			},
			EqualLine: diffview.LineStyle{
				LineNumber: lipgloss.NewStyle().
					Foreground(t.FgMuted).
					Background(t.BgBase),
				Code: lipgloss.NewStyle().
					Foreground(t.FgMuted).
					Background(t.BgBase),
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
		},
		FilePicker: filepicker.Styles{
			DisabledCursor:   base.Foreground(t.FgMuted),
			Cursor:           base.Foreground(t.FgBase),
			Symlink:          base.Foreground(t.FgSubtle),
			Directory:        base.Foreground(t.Primary),
			File:             base.Foreground(t.FgBase),
			DisabledFile:     base.Foreground(t.FgMuted),
			DisabledSelected: base.Background(t.BgOverlay).Foreground(t.FgMuted),
			Permission:       base.Foreground(t.FgMuted),
			Selected:         base.Background(t.Primary).Foreground(t.FgBase),
			FileSize:         base.Foreground(t.FgMuted),
			EmptyDirectory:   base.Foreground(t.FgMuted).PaddingLeft(2).SetString("Empty directory"),
		},
	}
}

type Manager struct {
	themes  map[string]*Theme
	current *Theme
}

var defaultManager *Manager

func SetDefaultManager(m *Manager) {
	defaultManager = m
}

func DefaultManager() *Manager {
	if defaultManager == nil {
		defaultManager = NewManager()
	}
	return defaultManager
}

// InitializeWithConfig sets up the default theme manager using config
func InitializeWithConfig(cfg interface{}) {
	// Accept interface{} to avoid circular imports, but we expect it to have a method to get theme
	if configGetter, ok := cfg.(interface {
		GetTheme() string
	}); ok {
		themeName := configGetter.GetTheme()
		if themeName != "" {
			defaultManager = NewManagerWithTheme(themeName)
			return
		}
	}
	// Fall back to default if no config or theme specified
	defaultManager = NewManager()
}

func CurrentTheme() *Theme {
	if defaultManager == nil {
		defaultManager = NewManager()
	}
	return defaultManager.Current()
}

func NewManager() *Manager {
	m := &Manager{
		themes: make(map[string]*Theme),
	}

	// Register default dark theme
	darkTheme := NewCharmtoneTheme()
	m.Register(darkTheme)

	// Register light theme
	lightTheme := NewCharmtoneLightTheme()
	m.Register(lightTheme)

	// Set dark theme as default
	m.current = m.themes[darkTheme.Name]

	return m
}

// NewManagerWithTheme creates a new theme manager and sets the specified theme
func NewManagerWithTheme(themeName string) *Manager {
	m := NewManager()
	if err := m.SetTheme(themeName); err != nil {
		// If the theme doesn't exist, fall back to default
		return m
	}
	return m
}

func (m *Manager) Register(theme *Theme) {
	m.themes[theme.Name] = theme
}

func (m *Manager) Current() *Theme {
	return m.current
}

func (m *Manager) SetTheme(name string) error {
	if theme, ok := m.themes[name]; ok {
		m.current = theme
		return nil
	}
	return fmt.Errorf("theme %s not found", name)
}

func (m *Manager) List() []string {
	names := make([]string, 0, len(m.themes))
	for name := range m.themes {
		names = append(names, name)
	}
	return names
}

// ParseHex converts hex string to color
func ParseHex(hex string) color.Color {
	var r, g, b uint8
	fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)
	return color.RGBA{R: r, G: g, B: b, A: 255}
}

// Alpha returns a color with transparency
func Alpha(c color.Color, alpha uint8) color.Color {
	r, g, b, _ := c.RGBA()
	return color.RGBA{
		R: uint8(r >> 8),
		G: uint8(g >> 8),
		B: uint8(b >> 8),
		A: alpha,
	}
}

// Darken makes a color darker by percentage (0-100)
func Darken(c color.Color, percent float64) color.Color {
	r, g, b, a := c.RGBA()
	factor := 1.0 - percent/100.0
	return color.RGBA{
		R: uint8(float64(r>>8) * factor),
		G: uint8(float64(g>>8) * factor),
		B: uint8(float64(b>>8) * factor),
		A: uint8(a >> 8),
	}
}

// Lighten makes a color lighter by percentage (0-100)
func Lighten(c color.Color, percent float64) color.Color {
	r, g, b, a := c.RGBA()
	factor := percent / 100.0
	return color.RGBA{
		R: uint8(min(255, float64(r>>8)+255*factor)),
		G: uint8(min(255, float64(g>>8)+255*factor)),
		B: uint8(min(255, float64(b>>8)+255*factor)),
		A: uint8(a >> 8),
	}
}

func ForegroundGrad(input string, bold bool, color1, color2 color.Color) []string {
	if input == "" {
		return []string{""}
	}
	t := CurrentTheme()
	if len(input) == 1 {
		style := t.S().Base.Foreground(color1)
		if bold {
			style.Bold(true)
		}
		return []string{style.Render(input)}
	}
	var clusters []string
	gr := uniseg.NewGraphemes(input)
	for gr.Next() {
		clusters = append(clusters, string(gr.Runes()))
	}

	ramp := blendColors(len(clusters), color1, color2)
	for i, c := range ramp {
		style := t.S().Base.Foreground(c)
		if bold {
			style.Bold(true)
		}
		clusters[i] = style.Render(clusters[i])
	}
	return clusters
}

// Helper to get Markdown color with fallback to theme colors
func (t *Theme) getMdColor(mdColor, fallback color.Color) color.Color {
	if mdColor != nil {
		return mdColor
	}
	return fallback
}

// Helper to convert color to hex string pointer
func colorHex(c color.Color) *string {
	if c == nil {
		return nil
	}
	// Convert color.Color to hex string
	r, g, b, _ := c.RGBA()
	hex := fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
	return &hex
}

// ApplyForegroundGrad renders a given string with a horizontal gradient
// foreground.
func ApplyForegroundGrad(input string, color1, color2 color.Color) string {
	if input == "" {
		return ""
	}
	var o strings.Builder
	clusters := ForegroundGrad(input, false, color1, color2)
	for _, c := range clusters {
		fmt.Fprint(&o, c)
	}
	return o.String()
}

// ApplyBoldForegroundGrad renders a given string with a horizontal gradient
// foreground.
func ApplyBoldForegroundGrad(input string, color1, color2 color.Color) string {
	if input == "" {
		return ""
	}
	var o strings.Builder
	clusters := ForegroundGrad(input, true, color1, color2)
	for _, c := range clusters {
		fmt.Fprint(&o, c)
	}
	return o.String()
}

// blendColors returns a slice of colors blended between the given keys.
// Blending is done in Hcl to stay in gamut.
func blendColors(size int, stops ...color.Color) []color.Color {
	if len(stops) < 2 {
		return nil
	}

	stopsPrime := make([]colorful.Color, len(stops))
	for i, k := range stops {
		stopsPrime[i], _ = colorful.MakeColor(k)
	}

	numSegments := len(stopsPrime) - 1
	blended := make([]color.Color, 0, size)

	// Calculate how many colors each segment should have.
	segmentSizes := make([]int, numSegments)
	baseSize := size / numSegments
	remainder := size % numSegments

	// Distribute the remainder across segments.
	for i := range numSegments {
		segmentSizes[i] = baseSize
		if i < remainder {
			segmentSizes[i]++
		}
	}

	// Generate colors for each segment.
	for i := range numSegments {
		c1 := stopsPrime[i]
		c2 := stopsPrime[i+1]
		segmentSize := segmentSizes[i]

		for j := range segmentSize {
			var t float64
			if segmentSize > 1 {
				t = float64(j) / float64(segmentSize-1)
			}
			c := c1.BlendHcl(c2, t)
			blended = append(blended, c)
		}
	}

	return blended
}
