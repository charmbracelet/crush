package styles

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/tui/exp/diffview"
	"github.com/charmbracelet/glamour/v2/ansi"
	"github.com/charmbracelet/x/exp/charmtone"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/rivo/uniseg"
)

const (
	defaultListIndent      = 2
	defaultListLevelIndent = 4
	defaultMargin          = 2
)

// colorToHex converts a color.Color to hex string for ANSI styles.
func colorToHex(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
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
	BlueDark  color.Color
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
				Shape: tea.CursorBlock,
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
				Shape: tea.CursorBlock,
				Blink: true,
			},
		},

		Markdown: ansi.StyleConfig{
			Document: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					// BlockPrefix: "\n",
					// BlockSuffix: "\n",
					Color: stringPtr(colorToHex(t.FgBase)),
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
					Color:       stringPtr(colorToHex(t.Blue)),
					Bold:        boolPtr(true),
				},
			},
			H1: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Prefix:          " ",
					Suffix:          " ",
					Color:           stringPtr(colorToHex(t.Yellow)),
					BackgroundColor: stringPtr(colorToHex(t.Primary)),
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
					Color:  stringPtr(colorToHex(t.Green)),
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
				Color:  stringPtr(colorToHex(t.Border)),
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
				Color:     stringPtr(colorToHex(t.Blue)),
				Underline: boolPtr(true),
			},
			LinkText: ansi.StylePrimitive{
				Color: stringPtr(colorToHex(t.Green)),
				Bold:  boolPtr(true),
			},
			Image: ansi.StylePrimitive{
				Color:     stringPtr(colorToHex(t.Cherry)),
				Underline: boolPtr(true),
			},
			ImageText: ansi.StylePrimitive{
				Color:  stringPtr(colorToHex(t.FgMuted)),
				Format: "Image: {{.text}} →",
			},
			Code: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Prefix:          " ",
					Suffix:          " ",
					Color:           stringPtr(colorToHex(t.Red)),
					BackgroundColor: stringPtr(colorToHex(t.BgSubtle)),
				},
			},
			CodeBlock: ansi.StyleCodeBlock{
				StyleBlock: ansi.StyleBlock{
					StylePrimitive: ansi.StylePrimitive{
						Color: stringPtr(colorToHex(t.BgSubtle)),
					},
					Margin: uintPtr(defaultMargin),
				},
				Chroma: &ansi.Chroma{
					Text: ansi.StylePrimitive{
						Color: stringPtr(colorToHex(t.FgBase)),
					},
					Error: ansi.StylePrimitive{
						Color:           stringPtr(colorToHex(t.White)),
						BackgroundColor: stringPtr(colorToHex(t.Error)),
					},
					Comment: ansi.StylePrimitive{
						Color: stringPtr(colorToHex(t.FgSubtle)),
					},
					CommentPreproc: ansi.StylePrimitive{
						Color: stringPtr(colorToHex(t.Yellow)),
					},
					Keyword: ansi.StylePrimitive{
						Color: stringPtr(colorToHex(t.Blue)),
					},
					KeywordReserved: ansi.StylePrimitive{
						Color: stringPtr(colorToHex(t.Cherry)),
					},
					KeywordNamespace: ansi.StylePrimitive{
						Color: stringPtr(colorToHex(t.Cherry)),
					},
					KeywordType: ansi.StylePrimitive{
						Color: stringPtr(colorToHex(t.BlueLight)),
					},
					Operator: ansi.StylePrimitive{
						Color: stringPtr(colorToHex(t.RedLight)),
					},
					Punctuation: ansi.StylePrimitive{
						Color: stringPtr(colorToHex(t.Yellow)),
					},
					Name: ansi.StylePrimitive{
						Color: stringPtr(colorToHex(t.FgBase)),
					},
					NameBuiltin: ansi.StylePrimitive{
						Color: stringPtr(colorToHex(t.Cherry)),
					},
					NameTag: ansi.StylePrimitive{
						Color: stringPtr(colorToHex(t.Cherry)),
					},
					NameAttribute: ansi.StylePrimitive{
						Color: stringPtr(colorToHex(t.Citron)),
					},
					NameClass: ansi.StylePrimitive{
						Color:     stringPtr(colorToHex(t.FgSelected)),
						Underline: boolPtr(true),
						Bold:      boolPtr(true),
					},
					NameDecorator: ansi.StylePrimitive{
						Color: stringPtr(colorToHex(t.Citron)),
					},
					NameFunction: ansi.StylePrimitive{
						Color: stringPtr(colorToHex(t.Green)),
					},
					LiteralNumber: ansi.StylePrimitive{
						Color: stringPtr(colorToHex(t.GreenLight)),
					},
					LiteralString: ansi.StylePrimitive{
						Color: stringPtr(colorToHex(t.Yellow)),
					},
					LiteralStringEscape: ansi.StylePrimitive{
						Color: stringPtr(colorToHex(t.Tertiary)),
					},
					GenericDeleted: ansi.StylePrimitive{
						Color: stringPtr(colorToHex(t.Red)),
					},
					GenericEmph: ansi.StylePrimitive{
						Italic: boolPtr(true),
					},
					GenericInserted: ansi.StylePrimitive{
						Color: stringPtr(colorToHex(t.Green)),
					},
					GenericStrong: ansi.StylePrimitive{
						Bold: boolPtr(true),
					},
					GenericSubheading: ansi.StylePrimitive{
						Color: stringPtr(colorToHex(t.FgMuted)),
					},
					Background: ansi.StylePrimitive{
						BackgroundColor: stringPtr(colorToHex(t.BgSubtle)),
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

	t := NewCharmtoneTheme() // default theme
	m.Register(t)
	m.current = m.themes[t.Name]

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

// ThemeConfig is an interface that config.Theme implements.
// This avoids circular dependencies while allowing us to accept the config.Theme.
type ThemeConfig interface {
	GetName() string
	GetIsDark() bool
	GetPrimary() string
	GetSecondary() string
	GetTertiary() string
	GetAccent() string
	GetBackground() string
	GetBackgroundLight() string
	GetBackgroundSubtle() string
	GetBackgroundOverlay() string
	GetForeground() string
	GetForegroundMuted() string
	GetForegroundSubtle() string
	GetForegroundSelected() string
	GetBorder() string
	GetBorderFocus() string
	GetSuccess() string
	GetError() string
	GetWarning() string
	GetInfo() string
	GetRed() string
	GetGreen() string
	GetYellow() string
	GetBlue() string
	GetMagenta() string
	GetWhite() string
	GetBrightRed() string
	GetBrightGreen() string
	GetBrightYellow() string
	GetBrightBlue() string
}

// NewThemeFromConfig creates a Theme from a config theme.
// Falls back to charmtone colors for any empty/unspecified colors.
// If cfg is nil, returns the default charmtone theme.
func NewThemeFromConfig(cfg ThemeConfig) *Theme {
	if cfg == nil {
		return NewCharmtoneTheme()
	}

	// Helper to parse hex or fall back to default
	parseOrDefault := func(hex string, defaultColor color.Color) color.Color {
		if hex == "" {
			return defaultColor
		}
		return ParseHex(hex)
	}

	name := cfg.GetName()
	if name == "" {
		name = "custom"
	}

	t := &Theme{
		Name:   name,
		IsDark: cfg.GetIsDark(),

		Primary:   parseOrDefault(cfg.GetPrimary(), charmtone.Charple),
		Secondary: parseOrDefault(cfg.GetSecondary(), charmtone.Dolly),
		Tertiary:  parseOrDefault(cfg.GetTertiary(), charmtone.Bok),
		Accent:    parseOrDefault(cfg.GetAccent(), charmtone.Zest),

		BgBase:        parseOrDefault(cfg.GetBackground(), charmtone.Pepper),
		BgBaseLighter: parseOrDefault(cfg.GetBackgroundLight(), charmtone.BBQ),
		BgSubtle:      parseOrDefault(cfg.GetBackgroundSubtle(), charmtone.Charcoal),
		BgOverlay:     parseOrDefault(cfg.GetBackgroundOverlay(), charmtone.Iron),

		FgBase:      parseOrDefault(cfg.GetForeground(), charmtone.Ash),
		FgMuted:     parseOrDefault(cfg.GetForegroundMuted(), charmtone.Squid),
		FgHalfMuted: parseOrDefault(cfg.GetForegroundMuted(), charmtone.Smoke),
		FgSubtle:    parseOrDefault(cfg.GetForegroundSubtle(), charmtone.Oyster),
		FgSelected:  parseOrDefault(cfg.GetForegroundSelected(), charmtone.Salt),

		Border:      parseOrDefault(cfg.GetBorder(), charmtone.Charcoal),
		BorderFocus: parseOrDefault(cfg.GetBorderFocus(), charmtone.Charple),

		Success: parseOrDefault(cfg.GetSuccess(), charmtone.Guac),
		Error:   parseOrDefault(cfg.GetError(), charmtone.Sriracha),
		Warning: parseOrDefault(cfg.GetWarning(), charmtone.Zest),
		Info:    parseOrDefault(cfg.GetInfo(), charmtone.Malibu),

		White: parseOrDefault(cfg.GetWhite(), charmtone.Butter),

		BlueLight: parseOrDefault(cfg.GetBrightBlue(), charmtone.Sardine),
		Blue:      parseOrDefault(cfg.GetBlue(), charmtone.Malibu),

		Yellow: parseOrDefault(cfg.GetYellow(), charmtone.Mustard),
		Citron: parseOrDefault(cfg.GetBrightYellow(), charmtone.Citron),

		Green:      parseOrDefault(cfg.GetBrightGreen(), charmtone.Julep),
		GreenDark:  parseOrDefault(cfg.GetGreen(), charmtone.Guac),
		GreenLight: parseOrDefault(cfg.GetBrightGreen(), charmtone.Bok),

		Red:      parseOrDefault(cfg.GetRed(), charmtone.Coral),
		RedDark:  parseOrDefault(cfg.GetBrightRed(), charmtone.Sriracha),
		RedLight: parseOrDefault(cfg.GetBrightRed(), charmtone.Salmon),
		Cherry:   parseOrDefault(cfg.GetMagenta(), charmtone.Cherry),
	}

	// Text selection
	selectionBg := parseOrDefault(cfg.GetPrimary(), charmtone.Charple)
	selectionFg := parseOrDefault(cfg.GetForegroundSelected(), charmtone.Salt)
	t.TextSelection = lipgloss.NewStyle().Foreground(selectionFg).Background(selectionBg)

	// LSP and MCP status icons
	offlineColor := parseOrDefault(cfg.GetForegroundMuted(), charmtone.Squid)
	t.ItemOfflineIcon = lipgloss.NewStyle().Foreground(offlineColor).SetString("●")
	t.ItemBusyIcon = t.ItemOfflineIcon.Foreground(parseOrDefault(cfg.GetWarning(), charmtone.Citron))
	t.ItemErrorIcon = t.ItemOfflineIcon.Foreground(parseOrDefault(cfg.GetError(), charmtone.Coral))
	t.ItemOnlineIcon = t.ItemOfflineIcon.Foreground(parseOrDefault(cfg.GetSuccess(), charmtone.Guac))

	// YOLO mode indicators
	yoloBg := parseOrDefault(cfg.GetWarning(), charmtone.Citron)
	yoloFg := parseOrDefault(cfg.GetBackgroundSubtle(), charmtone.Oyster)
	t.YoloIconFocused = lipgloss.NewStyle().Foreground(yoloFg).Background(yoloBg).Bold(true).SetString(" ! ")
	t.YoloIconBlurred = t.YoloIconFocused.Foreground(parseOrDefault(cfg.GetBackground(), charmtone.Pepper)).Background(offlineColor)
	t.YoloDotsFocused = lipgloss.NewStyle().Foreground(parseOrDefault(cfg.GetAccent(), charmtone.Zest)).SetString(":::")
	t.YoloDotsBlurred = t.YoloDotsFocused.Foreground(offlineColor)

	return t
}
