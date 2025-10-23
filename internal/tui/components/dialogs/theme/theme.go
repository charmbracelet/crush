package theme

import (
	"github.com/charmbracelet/bubbles/v2/help"
	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/themes"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/exp/list"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/lipgloss/v2"
)

const (
	ThemeDialogID dialogs.DialogID = "theme"
	defaultWidth                   = 40
	defaultHeight                  = 12
)

// ThemeSelectedMsg is sent when a theme is selected
type ThemeSelectedMsg struct {
	Theme string
}

// ThemeChangedMsg is sent to trigger UI refresh with new theme
type ThemeChangedMsg struct{}

// ThemeDialog interface for the theme selection dialog
type ThemeDialog interface {
	dialogs.DialogModel
}

type themeDialogCmp struct {
	width   int
	wWidth  int
	wHeight int

	list    list.FilterableList[list.CompletionItem[string]]
	keyMap  KeyMap
	help    help.Model
	themes  []string
	config  *config.Config
	manager *themes.Manager
}

// KeyMap defines keyboard bindings for theme dialog
type KeyMap struct {
	Select key.Binding
	Close  key.Binding
	Next   key.Binding
	Prev   key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Close: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "close"),
		),
		Next: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "next"),
		),
		Prev: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "prev"),
		),
	}
}

// KeyBindings returns all key bindings
func (k KeyMap) KeyBindings() []key.Binding {
	return []key.Binding{
		k.Select,
		k.Close,
		k.Next,
		k.Prev,
	}
}

// FullHelp implements help.KeyMap
func (k KeyMap) FullHelp() [][]key.Binding {
	m := [][]key.Binding{}
	slice := k.KeyBindings()
	for i := 0; i < len(slice); i += 4 {
		end := min(i+4, len(slice))
		m = append(m, slice[i:end])
	}
	return m
}

// ShortHelp implements help.KeyMap
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Select,
		k.Close,
	}
}

func NewThemeDialogCmp(cfg *config.Config) ThemeDialog {
	keyMap := DefaultKeyMap()

	// Create theme manager
	manager := themes.NewManager(cfg)

	// Get available themes
	availableThemes := manager.ListThemes()

	// Create list items - no filtering, just simple selection
	items := make([]list.CompletionItem[string], len(availableThemes))
	for i, theme := range availableThemes {
		items[i] = list.NewCompletionItem(theme, theme, list.WithCompletionID(theme))
	}

	// Create a simple list without filtering
	listKeyMap := list.DefaultKeyMap()
	listKeyMap.Down.SetEnabled(false)
	listKeyMap.Up.SetEnabled(false)
	listKeyMap.DownOneItem = keyMap.Next
	listKeyMap.UpOneItem = keyMap.Prev

	themeList := list.NewFilterableList(
		items,
		list.WithFilterPlaceholder(""), // No placeholder
		list.WithFilterListOptions(
			list.WithKeyMap(listKeyMap),
			list.WithWrapNavigation(),
		),
	)

	t := styles.CurrentTheme()
	help := help.New()
	help.Styles = t.S().Help

	return &themeDialogCmp{
		list:    themeList,
		keyMap:  keyMap,
		help:    help,
		themes:  availableThemes,
		config:  cfg,
		manager: manager,
		width:   defaultWidth,
	}
}

func (t *themeDialogCmp) Init() tea.Cmd {
	var cmds []tea.Cmd
	cmds = append(cmds, t.list.Init())
	cmds = append(cmds, t.list.Focus())
	return tea.Sequence(cmds...)
}

func (t *themeDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		var cmds []tea.Cmd
		t.wWidth = msg.Width
		t.wHeight = msg.Height
		t.width = min(60, t.wWidth-8)
		t.list.SetInputWidth(t.listWidth() - 2)
		cmds = append(cmds, t.list.SetSize(t.listWidth(), t.listHeight()))
		return t, tea.Batch(cmds...)
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, t.keyMap.Select):
			selectedItem := t.list.SelectedItem()
			if selectedItem != nil {
				selected := (*selectedItem).Value()
				// Set the theme
				if err := t.manager.SetTheme(selected); err != nil {
					return t, util.ReportError(err)
				}
				return t, tea.Sequence(
					util.CmdHandler(dialogs.CloseDialogMsg{}),
					util.CmdHandler(ThemeSelectedMsg{Theme: selected}),
					util.ReportInfo("Theme changed to "+selected),
				)
			}
		case key.Matches(msg, t.keyMap.Close):
			return t, util.CmdHandler(dialogs.CloseDialogMsg{})
		default:
			u, cmd := t.list.Update(msg)
			t.list = u.(list.FilterableList[list.CompletionItem[string]])
			return t, cmd
		}
	}
	return t, nil
}

func (t *themeDialogCmp) View() string {
	theme := styles.CurrentTheme()

	// Get current theme for highlighting
	currentTheme := t.manager.CurrentName()

	// Update list items to show current theme
	items := make([]list.CompletionItem[string], len(t.themes))
	for i, themeName := range t.themes {
		if themeName == currentTheme {
			items[i] = list.NewCompletionItem(themeName+" ✓", themeName, list.WithCompletionID(themeName))
		} else {
			items[i] = list.NewCompletionItem(themeName, themeName, list.WithCompletionID(themeName))
		}
	}
	t.list.SetItems(items)

	listView := t.list.View()
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		theme.S().Base.Padding(0, 1, 1, 1).Render(core.Title("Select Theme", t.width-4)),
		listView,
		"",
		theme.S().Base.Width(t.width-2).PaddingLeft(1).AlignHorizontal(lipgloss.Left).Render(t.help.View(t.keyMap)),
	)
	return t.style().Render(content)
}

func (t *themeDialogCmp) Cursor() *tea.Cursor {
	if cursor, ok := t.list.(util.Cursor); ok {
		cursor := cursor.Cursor()
		if cursor != nil {
			cursor = t.moveCursor(cursor)
		}
		return cursor
	}
	return nil
}

func (t *themeDialogCmp) style() lipgloss.Style {
	theme := styles.CurrentTheme()
	return theme.S().Base.
		Width(t.width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.BorderFocus)
}

func (t *themeDialogCmp) listWidth() int {
	return t.width - 2
}

func (t *themeDialogCmp) listHeight() int {
	return min(t.wHeight/3, 8)
}

func (t *themeDialogCmp) Position() (int, int) {
	row := t.wHeight/4 - 2 // just a bit above the center
	col := t.wWidth / 2
	col -= t.width / 2
	return row, col
}

func (t *themeDialogCmp) moveCursor(cursor *tea.Cursor) *tea.Cursor {
	row, col := t.Position()
	offset := row + 3 // Border + title
	cursor.Y += offset
	cursor.X = cursor.X + col + 2
	return cursor
}

func (t *themeDialogCmp) ID() dialogs.DialogID {
	return ThemeDialogID
}
