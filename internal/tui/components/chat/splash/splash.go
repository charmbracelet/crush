package splash

import (
	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/tui/components/core/layout"
	"github.com/charmbracelet/crush/internal/tui/components/logo"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/crush/internal/version"
	"github.com/charmbracelet/lipgloss/v2"
)

type Splash interface {
	util.Model
	layout.Sizeable
	layout.Help
}

const (
	SplashScreenPaddingX = 2 // Padding X for the splash screen
	SplashScreenPaddingY = 1 // Padding Y for the splash screen
)

type splashCmp struct {
	width, height int
	keyMap        KeyMap
	logoRendered  string
}

func New() Splash {
	return &splashCmp{
		width:        0,
		height:       0,
		keyMap:       DefaultKeyMap(),
		logoRendered: "",
	}
}

// GetSize implements SplashPage.
func (s *splashCmp) GetSize() (int, int) {
	return s.width, s.height
}

// Init implements SplashPage.
func (s *splashCmp) Init() tea.Cmd {
	return nil
}

// SetSize implements SplashPage.
func (s *splashCmp) SetSize(width int, height int) tea.Cmd {
	s.width = width
	s.height = height
	s.logoRendered = s.logoBlock()
	return nil
}

// Update implements SplashPage.
func (s *splashCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return s, s.SetSize(msg.Width, msg.Height)
	}
	return s, nil
}

// View implements SplashPage.
func (s *splashCmp) View() tea.View {
	t := styles.CurrentTheme()
	content := lipgloss.JoinVertical(lipgloss.Left, s.logoRendered)
	return tea.NewView(
		t.S().Base.
			Width(s.width).
			Height(s.height).
			PaddingTop(SplashScreenPaddingY).
			PaddingLeft(SplashScreenPaddingX).
			PaddingRight(SplashScreenPaddingX).
			PaddingBottom(SplashScreenPaddingY).
			Render(
				content,
			),
	)
}

func (s *splashCmp) logoBlock() string {
	t := styles.CurrentTheme()
	const padding = 2
	return logo.Render(version.Version, false, logo.Opts{
		FieldColor:   t.Primary,
		TitleColorA:  t.Secondary,
		TitleColorB:  t.Primary,
		CharmColor:   t.Secondary,
		VersionColor: t.Primary,
		Width:        s.width - (SplashScreenPaddingX * 2),
	})
}

// Bindings implements SplashPage.
func (s *splashCmp) Bindings() []key.Binding {
	return []key.Binding{
		s.keyMap.Cancel,
	}
}
