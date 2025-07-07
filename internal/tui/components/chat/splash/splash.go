package splash

import (
	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/tui/components/core/layout"
	"github.com/charmbracelet/crush/internal/tui/components/core/list"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/models"
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

type SplashScreenState string

const (
	SplashScreenStateOnboarding SplashScreenState = "onboarding"
	SplashScreenStateInitialize SplashScreenState = "initialize"
	SplashScreenStateReady      SplashScreenState = "ready"
)

// OnboardingCompleteMsg is sent when onboarding is complete
type OnboardingCompleteMsg struct{}

type splashCmp struct {
	width, height        int
	keyMap               KeyMap
	logoRendered         string
	state                SplashScreenState
	modelList            *models.ModelListComponent
	cursorRow, cursorCol int
}

func New() Splash {
	keyMap := DefaultKeyMap()
	listKeyMap := list.DefaultKeyMap()
	listKeyMap.Down.SetEnabled(false)
	listKeyMap.Up.SetEnabled(false)
	listKeyMap.HalfPageDown.SetEnabled(false)
	listKeyMap.HalfPageUp.SetEnabled(false)
	listKeyMap.Home.SetEnabled(false)
	listKeyMap.End.SetEnabled(false)
	listKeyMap.DownOneItem = keyMap.Next
	listKeyMap.UpOneItem = keyMap.Previous

	t := styles.CurrentTheme()
	inputStyle := t.S().Base.Padding(0, 1, 0, 1)
	modelList := models.NewModelListComponent(listKeyMap, inputStyle, "Find your fave")
	return &splashCmp{
		width:        0,
		height:       0,
		keyMap:       keyMap,
		state:        SplashScreenStateOnboarding,
		logoRendered: "",
		modelList:    modelList,
	}
}

// GetSize implements SplashPage.
func (s *splashCmp) GetSize() (int, int) {
	return s.width, s.height
}

// Init implements SplashPage.
func (s *splashCmp) Init() tea.Cmd {
	if config.HasInitialDataConfig() {
		if b, _ := config.ProjectNeedsInitialization(); b {
			s.state = SplashScreenStateInitialize
		} else {
			s.state = SplashScreenStateReady
		}
	}
	return s.modelList.Init()
}

// SetSize implements SplashPage.
func (s *splashCmp) SetSize(width int, height int) tea.Cmd {
	s.width = width
	s.height = height
	s.logoRendered = s.logoBlock()
	listHeigh := min(40, height-(SplashScreenPaddingY*2)-lipgloss.Height(s.logoRendered)-2) // -1 for the title
	listWidth := min(60, width-(SplashScreenPaddingX*2))

	// Calculate the cursor position based on the height and logo size
	s.cursorRow = height - listHeigh
	return s.modelList.SetSize(listWidth, listHeigh)
}

// Update implements SplashPage.
func (s *splashCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return s, s.SetSize(msg.Width, msg.Height)
	case tea.KeyPressMsg:
		switch {
		default:
			u, cmd := s.modelList.Update(msg)
			s.modelList = u
			return s, cmd
		}
	}
	return s, nil
}

// View implements SplashPage.
func (s *splashCmp) View() tea.View {
	t := styles.CurrentTheme()
	var cursor *tea.Cursor

	var content string
	switch s.state {
	case SplashScreenStateOnboarding:
		// Show logo and model selector
		remainingHeight := s.height - lipgloss.Height(s.logoRendered) - (SplashScreenPaddingY * 2)
		modelListView := s.modelList.View()
		cursor = s.moveCursor(modelListView.Cursor())
		modelSelector := t.S().Base.AlignVertical(lipgloss.Bottom).Height(remainingHeight).Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				t.S().Base.PaddingLeft(1).Foreground(t.Primary).Render("Choose a Model"),
				"",
				modelListView.String(),
			),
		)
		content = lipgloss.JoinVertical(
			lipgloss.Left,
			s.logoRendered,
			modelSelector,
		)
	default:
		// Show just the logo for other states
		content = s.logoRendered
	}

	view := tea.NewView(
		t.S().Base.
			Width(s.width).
			Height(s.height).
			PaddingTop(SplashScreenPaddingY).
			PaddingLeft(SplashScreenPaddingX).
			PaddingRight(SplashScreenPaddingX).
			PaddingBottom(SplashScreenPaddingY).
			Render(content),
	)

	view.SetCursor(cursor)
	return view
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

func (m *splashCmp) moveCursor(cursor *tea.Cursor) *tea.Cursor {
	if cursor == nil {
		return nil
	}
	offset := m.cursorRow
	cursor.Y += offset
	cursor.X = cursor.X + 3 // 3 for padding
	return cursor
}

// Bindings implements SplashPage.
func (s *splashCmp) Bindings() []key.Binding {
	if s.state == SplashScreenStateOnboarding {
		return []key.Binding{
			s.keyMap.Select,
			s.keyMap.Next,
			s.keyMap.Previous,
		}
	}
	return []key.Binding{}
}
