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
					Color:           stringPtr(charmtone.Charple.Hex()),
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
