package themes

import (
	"github.com/charmbracelet/bubbles/v2/help"
	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/exp/list"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
)

const (
	ThemesDialogID dialogs.DialogID = "themes"

	defaultWidth = 50
)

// ThemeSelectedMsg is sent when a theme is selected
type ThemeSelectedMsg struct {
	ThemeName string
}

// ThemeDialog interface for the theme selection dialog
type ThemeDialog interface {
	dialogs.DialogModel
}

type themeDialogCmp struct {
	width   int
	wWidth  int
	wHeight int

	themeList list.FilterableList[list.CompletionItem[string]]
	keyMap    KeyMap
	help      help.Model
}

func NewThemeDialog() ThemeDialog {
	keyMap := DefaultKeyMap()

	listKeyMap := list.DefaultKeyMap()
	listKeyMap.Down.SetEnabled(false)
	listKeyMap.Up.SetEnabled(false)
	listKeyMap.DownOneItem = keyMap.Next
	listKeyMap.UpOneItem = keyMap.Previous

	t := styles.CurrentTheme()
	inputStyle := t.S().Base.PaddingLeft(1).PaddingBottom(1)
	themeList := list.NewFilterableList(
		[]list.CompletionItem[string]{},
		list.WithFilterInputStyle(inputStyle),
		list.WithFilterListOptions(
			list.WithKeyMap(listKeyMap),
			list.WithWrapNavigation(),
			list.WithResizeByList(),
		),
	)

	help := help.New()
	help.Styles = t.S().Help

	return &themeDialogCmp{
		themeList: themeList,
		width:     defaultWidth,
		keyMap:    DefaultKeyMap(),
		help:      help,
	}
}

func (t *themeDialogCmp) Init() tea.Cmd {
	// Get available themes from the theme manager
	manager := styles.DefaultManager()
	availableThemes := manager.List()
	currentTheme := manager.Current()

	// Create theme items
	themeItems := []list.CompletionItem[string]{}
	for _, themeName := range availableThemes {
		opts := []list.CompletionItemOption{
			list.WithCompletionID(themeName),
		}

		// Add indicator for current theme
		title := themeName
		if themeName == currentTheme.Name {
			title = "● " + themeName + " (current)"
		}

		themeItems = append(themeItems, list.NewCompletionItem(title, themeName, opts...))
	}

	return t.themeList.SetItems(themeItems)
}

func (t *themeDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		t.wWidth = msg.Width
		t.wHeight = msg.Height
		return t, t.themeList.SetSize(t.listWidth(), t.listHeight())
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, t.keyMap.Select):
			selectedItem := t.themeList.SelectedItem()
			if selectedItem == nil {
				return t, nil // No item selected, do nothing
			}
			themeName := (*selectedItem).Value()
			return t, tea.Sequence(
				util.CmdHandler(dialogs.CloseDialogMsg{}),
				util.CmdHandler(ThemeSelectedMsg{ThemeName: themeName}),
			)
		case key.Matches(msg, t.keyMap.Close):
			return t, util.CmdHandler(dialogs.CloseDialogMsg{})
		default:
			u, cmd := t.themeList.Update(msg)
			t.themeList = u.(list.FilterableList[list.CompletionItem[string]])
			return t, cmd
		}
	}
	return t, nil
}

func (t *themeDialogCmp) View() string {
	theme := styles.CurrentTheme()

	header := theme.S().Base.Padding(0, 1, 1, 1).Render(core.Title("Switch Theme", t.width-4))
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		t.themeList.View(),
		"",
		theme.S().Base.Width(t.width-2).PaddingLeft(1).AlignHorizontal(lipgloss.Left).Render(t.help.View(t.keyMap)),
	)
	return t.style().Render(content)
}

func (t *themeDialogCmp) Cursor() *tea.Cursor {
	if cursor, ok := t.themeList.(util.Cursor); ok {
		cursor := cursor.Cursor()
		if cursor != nil {
			cursor = t.moveCursor(cursor)
		}
		return cursor
	}
	return nil
}

func (t *themeDialogCmp) listWidth() int {
	return defaultWidth - 2
}

func (t *themeDialogCmp) listHeight() int {
	listHeight := len(t.themeList.Items()) + 2 + 4 // height based on items + 2 for the input + 4 for the sections
	return min(listHeight, t.wHeight/2)
}

func (t *themeDialogCmp) moveCursor(cursor *tea.Cursor) *tea.Cursor {
	row, col := t.Position()
	offset := row + 3
	cursor.Y += offset
	cursor.X = cursor.X + col + 2
	return cursor
}

func (t *themeDialogCmp) style() lipgloss.Style {
	theme := styles.CurrentTheme()
	return theme.S().Base.
		Width(t.width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.BorderFocus)
}

func (t *themeDialogCmp) Position() (int, int) {
	row := t.wHeight/4 - 2 // just a bit above the center
	col := t.wWidth / 2
	col -= t.width / 2
	return row, col
}

func (t *themeDialogCmp) ID() dialogs.DialogID {
	return ThemesDialogID
}
