package dialog

import (
	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/opencode-ai/opencode/internal/tui/layout"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

// ThemeChangedMsg is sent when the theme is changed
type ThemeChangedMsg struct {
	ThemeName string
}

// CloseThemeDialogMsg is sent when the theme dialog is closed
type CloseThemeDialogMsg struct{}

// ThemeDialog interface for the theme switching dialog
type ThemeDialog interface {
	util.Model
	layout.Bindings
}

type themeDialogCmp struct {
	themes       []string
	selectedIdx  int
	width        int
	height       int
	currentTheme string
}

type themeKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Enter  key.Binding
	Escape key.Binding
	J      key.Binding
	K      key.Binding
}

var themeKeys = themeKeyMap{
	Up: key.NewBinding(
		key.WithKeys("up"),
		key.WithHelp("↑", "previous theme"),
	),
	Down: key.NewBinding(
		key.WithKeys("down"),
		key.WithHelp("↓", "next theme"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select theme"),
	),
	Escape: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "close"),
	),
	J: key.NewBinding(
		key.WithKeys("j"),
		key.WithHelp("j", "next theme"),
	),
	K: key.NewBinding(
		key.WithKeys("k"),
		key.WithHelp("k", "previous theme"),
	),
}

func (t *themeDialogCmp) Init() tea.Cmd {
	// Load available themes and update selectedIdx based on current theme
	t.themes = theme.AvailableThemes()
	t.currentTheme = theme.CurrentThemeName()

	// Find the current theme in the list
	for i, name := range t.themes {
		if name == t.currentTheme {
			t.selectedIdx = i
			break
		}
	}

	return nil
}

func (t *themeDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, themeKeys.Up) || key.Matches(msg, themeKeys.K):
			if t.selectedIdx > 0 {
				t.selectedIdx--
			}
			return t, nil
		case key.Matches(msg, themeKeys.Down) || key.Matches(msg, themeKeys.J):
			if t.selectedIdx < len(t.themes)-1 {
				t.selectedIdx++
			}
			return t, nil
		case key.Matches(msg, themeKeys.Enter):
			if len(t.themes) > 0 {
				previousTheme := theme.CurrentThemeName()
				selectedTheme := t.themes[t.selectedIdx]
				if previousTheme == selectedTheme {
					return t, util.CmdHandler(CloseThemeDialogMsg{})
				}
				if err := theme.SetTheme(selectedTheme); err != nil {
					return t, util.ReportError(err)
				}
				return t, util.CmdHandler(ThemeChangedMsg{
					ThemeName: selectedTheme,
				})
			}
		case key.Matches(msg, themeKeys.Escape):
			return t, util.CmdHandler(CloseThemeDialogMsg{})
		}
	case tea.WindowSizeMsg:
		t.width = msg.Width
		t.height = msg.Height
	}
	return t, nil
}

func (t *themeDialogCmp) View() string {
	currentTheme := theme.CurrentTheme()
	baseStyle := styles.BaseStyle()

	if len(t.themes) == 0 {
		return baseStyle.Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderBackground(currentTheme.Background()).
			BorderForeground(currentTheme.TextMuted()).
			Width(40).
			Render("No themes available")
	}

	// Calculate max width needed for theme names
	maxWidth := 40 // Minimum width
	for _, themeName := range t.themes {
		if len(themeName) > maxWidth-4 { // Account for padding
			maxWidth = len(themeName) + 4
		}
	}

	maxWidth = max(30, min(maxWidth, t.width-15)) // Limit width to avoid overflow

	// Build the theme list
	themeItems := make([]string, 0, len(t.themes))
	for i, themeName := range t.themes {
		itemStyle := baseStyle.Width(maxWidth)

		if i == t.selectedIdx {
			itemStyle = itemStyle.
				Background(currentTheme.Primary()).
				Foreground(currentTheme.Background()).
				Bold(true)
		}

		themeItems = append(themeItems, itemStyle.Padding(0, 1).Render(themeName))
	}

	title := baseStyle.
		Foreground(currentTheme.Primary()).
		Bold(true).
		Width(maxWidth).
		Padding(0, 1).
		Render("Select Theme")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		baseStyle.Width(maxWidth).Render(""),
		baseStyle.Width(maxWidth).Render(lipgloss.JoinVertical(lipgloss.Left, themeItems...)),
		baseStyle.Width(maxWidth).Render(""),
	)

	return baseStyle.Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderBackground(currentTheme.Background()).
		BorderForeground(currentTheme.TextMuted()).
		Width(lipgloss.Width(content) + 4).
		Render(content)
}

func (t *themeDialogCmp) BindingKeys() []key.Binding {
	return layout.KeyMapToSlice(themeKeys)
}

// NewThemeDialogCmp creates a new theme switching dialog
func NewThemeDialogCmp() ThemeDialog {
	return &themeDialogCmp{
		themes:       []string{},
		selectedIdx:  0,
		currentTheme: "",
	}
}
