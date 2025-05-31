package styles

import (
	"fmt"
	"image/color"

	"github.com/charmbracelet/bubbles/v2/textarea"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

type Theme struct {
	Name   string
	IsDark bool

	Primary   color.Color
	Secondary color.Color
	Tertiary  color.Color
	Accent    color.Color

	BgBase    color.Color
	BgSubtle  color.Color
	BgOverlay color.Color

	FgBase   color.Color
	FgMuted  color.Color
	FgSubtle color.Color

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
	Base lipgloss.Style

	Title    lipgloss.Style
	Subtitle lipgloss.Style
	Text     lipgloss.Style
	Muted    lipgloss.Style
	Subtle   lipgloss.Style

	Success lipgloss.Style
	Error   lipgloss.Style
	Warning lipgloss.Style
	Info    lipgloss.Style

	// Inputs
	TextArea textarea.Styles
}

func (t *Theme) S() *Styles {
	if t.styles == nil {
		t.styles = t.buildStyles()
	}
	return t.styles
}

func (t *Theme) buildStyles() *Styles {
	base := lipgloss.NewStyle().
		Background(t.BgBase).
		Foreground(t.FgBase)
	return &Styles{
		Base: base,

		Title: base.
			Foreground(t.Accent).
			Bold(true),

		Subtitle: base.
			Foreground(t.Secondary).
			Bold(true),

		Text: base,

		Muted: base.Foreground(t.FgMuted),

		Subtle: base.Foreground(t.FgSubtle),

		Success: base.Foreground(t.Success),

		Error: base.Foreground(t.Error),

		Warning: base.Foreground(t.Warning),

		Info: base.Foreground(t.Info),

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
