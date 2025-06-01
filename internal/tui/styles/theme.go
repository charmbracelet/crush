package styles

import (
	"fmt"
	"image/color"

	"github.com/charmbracelet/bubbles/v2/help"
	"github.com/charmbracelet/bubbles/v2/textarea"
	"github.com/charmbracelet/bubbles/v2/textinput"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/glamour/v2/ansi"
	"github.com/charmbracelet/lipgloss/v2"
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

	PrimaryLight color.Color

	BgBase    color.Color
	BgSubtle  color.Color
	BgOverlay color.Color

	FgBase     color.Color
	FgMuted    color.Color
	FgSubtle   color.Color
	FgSelected color.Color

	Border      color.Color
	BorderFocus color.Color

	Success color.Color
	Error   color.Color
	Warning color.Color
	Info    color.Color

	// TODO: add more syntax colors, maybe just use a chroma theme here.
	SyntaxBg      color.Color
	SyntaxKeyword color.Color
	SyntaxString  color.Color
	SyntaxComment color.Color

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
				Placeholder: base.Foreground(t.FgMuted),
				Prompt:      base.Foreground(t.Tertiary),
				Suggestion:  base.Foreground(t.FgMuted),
			},
			Blurred: textinput.StyleState{
				Text:        base.Foreground(t.FgMuted),
				Placeholder: base.Foreground(t.FgMuted),
				Prompt:      base.Foreground(t.FgMuted),
				Suggestion:  base.Foreground(t.FgMuted),
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
				Placeholder:      base.Foreground(t.FgMuted),
				Prompt:           base.Foreground(t.Tertiary),
			},
			Blurred: textarea.StyleState{
				Base:             base,
				Text:             base.Foreground(t.FgMuted),
				LineNumber:       base.Foreground(t.FgMuted),
				CursorLine:       base,
				CursorLineNumber: base.Foreground(t.FgMuted),
				Placeholder:      base.Foreground(t.FgMuted),
				Prompt:           base.Foreground(t.FgMuted),
			},
			Cursor: textarea.CursorStyle{
				Color: t.Secondary,
				Shape: tea.CursorBar,
				Blink: true,
			},
		},

		// TODO:  update using the colors and add colors if missing
		Markdown: ansi.StyleConfig{
			Document: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					BlockPrefix: "\n",
					BlockSuffix: "\n",
					Color:       stringPtr("252"),
				},
				Margin: uintPtr(defaultMargin),
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
					Color:       stringPtr("39"),
					Bold:        boolPtr(true),
				},
			},
			H1: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Prefix:          " ",
					Suffix:          " ",
					Color:           stringPtr("228"),
					BackgroundColor: stringPtr("63"),
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
					Color:  stringPtr("35"),
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
				Color:  stringPtr("240"),
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
				Color:     stringPtr("30"),
				Underline: boolPtr(true),
			},
			LinkText: ansi.StylePrimitive{
				Color: stringPtr("35"),
				Bold:  boolPtr(true),
			},
			Image: ansi.StylePrimitive{
				Color:     stringPtr("212"),
				Underline: boolPtr(true),
			},
			ImageText: ansi.StylePrimitive{
				Color:  stringPtr("243"),
				Format: "Image: {{.text}} →",
			},
			Code: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Prefix:          " ",
					Suffix:          " ",
					Color:           stringPtr("203"),
					BackgroundColor: stringPtr("236"),
				},
			},
			CodeBlock: ansi.StyleCodeBlock{
				StyleBlock: ansi.StyleBlock{
					StylePrimitive: ansi.StylePrimitive{
						Color: stringPtr("244"),
					},
					Margin: uintPtr(defaultMargin),
				},
				Chroma: &ansi.Chroma{
					Text: ansi.StylePrimitive{
						Color: stringPtr("#C4C4C4"),
					},
					Error: ansi.StylePrimitive{
						Color:           stringPtr("#F1F1F1"),
						BackgroundColor: stringPtr("#F05B5B"),
					},
					Comment: ansi.StylePrimitive{
						Color: stringPtr("#676767"),
					},
					CommentPreproc: ansi.StylePrimitive{
						Color: stringPtr("#FF875F"),
					},
					Keyword: ansi.StylePrimitive{
						Color: stringPtr("#00AAFF"),
					},
					KeywordReserved: ansi.StylePrimitive{
						Color: stringPtr("#FF5FD2"),
					},
					KeywordNamespace: ansi.StylePrimitive{
						Color: stringPtr("#FF5F87"),
					},
					KeywordType: ansi.StylePrimitive{
						Color: stringPtr("#6E6ED8"),
					},
					Operator: ansi.StylePrimitive{
						Color: stringPtr("#EF8080"),
					},
					Punctuation: ansi.StylePrimitive{
						Color: stringPtr("#E8E8A8"),
					},
					Name: ansi.StylePrimitive{
						Color: stringPtr("#C4C4C4"),
					},
					NameBuiltin: ansi.StylePrimitive{
						Color: stringPtr("#FF8EC7"),
					},
					NameTag: ansi.StylePrimitive{
						Color: stringPtr("#B083EA"),
					},
					NameAttribute: ansi.StylePrimitive{
						Color: stringPtr("#7A7AE6"),
					},
					NameClass: ansi.StylePrimitive{
						Color:     stringPtr("#F1F1F1"),
						Underline: boolPtr(true),
						Bold:      boolPtr(true),
					},
					NameDecorator: ansi.StylePrimitive{
						Color: stringPtr("#FFFF87"),
					},
					NameFunction: ansi.StylePrimitive{
						Color: stringPtr("#00D787"),
					},
					LiteralNumber: ansi.StylePrimitive{
						Color: stringPtr("#6EEFC0"),
					},
					LiteralString: ansi.StylePrimitive{
						Color: stringPtr("#C69669"),
					},
					LiteralStringEscape: ansi.StylePrimitive{
						Color: stringPtr("#AFFFD7"),
					},
					GenericDeleted: ansi.StylePrimitive{
						Color: stringPtr("#FD5B5B"),
					},
					GenericEmph: ansi.StylePrimitive{
						Italic: boolPtr(true),
					},
					GenericInserted: ansi.StylePrimitive{
						Color: stringPtr("#00D787"),
					},
					GenericStrong: ansi.StylePrimitive{
						Bold: boolPtr(true),
					},
					GenericSubheading: ansi.StylePrimitive{
						Color: stringPtr("#777777"),
					},
					Background: ansi.StylePrimitive{
						BackgroundColor: stringPtr("#373737"),
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
		defaultManager = NewManager("crush")
	}
	return defaultManager
}

func CurrentTheme() *Theme {
	if defaultManager == nil {
		defaultManager = NewManager("crush")
	}
	return defaultManager.Current()
}

func NewManager(defaultTheme string) *Manager {
	m := &Manager{
		themes: make(map[string]*Theme),
	}

	m.Register(NewCrushTheme())

	m.current = m.themes[defaultTheme]

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
