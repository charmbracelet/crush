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
	content := lipgloss.JoinVertical(lipgloss.Left, s.logoRendered)
	return tea.NewView(content)
}

func (m *splashCmp) logoBlock() string {
	t := styles.CurrentTheme()
	return logo.Render(version.Version, false, logo.Opts{
		FieldColor:   t.Primary,
		TitleColorA:  t.Secondary,
		TitleColorB:  t.Primary,
		CharmColor:   t.Secondary,
		VersionColor: t.Primary,
		Width:        m.width - 2, // -2 for padding
	})
}

// Bindings implements SplashPage.
func (s *splashCmp) Bindings() []key.Binding {
	return []key.Binding{
		s.keyMap.Cancel,
	}
}
