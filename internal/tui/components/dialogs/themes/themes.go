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

	defaultWidth int = 60
)

type listTheme = list.FilterableList[list.CompletionItem[Theme]]

type Theme struct {
	Name        string
	Description string
	IsCurrent   bool
}

// ThemeDialog interface for the theme selection dialog
type ThemeDialog interface {
	dialogs.DialogModel
}

type themeDialogCmp struct {
	wWidth  int
	wHeight int
	width   int

	themesList listTheme
	keyMap     KeyMap
	help       help.Model
}

// ThemeSelectedMsg is sent when a theme is selected
type ThemeSelectedMsg struct {
	ThemeName string
}

// NewThemeDialogCmp creates a new theme selection dialog
func NewThemeDialogCmp() ThemeDialog {
	keyMap := DefaultKeyMap()

	listKeyMap := list.DefaultKeyMap()
	listKeyMap.Down.SetEnabled(false)
	listKeyMap.Up.SetEnabled(false)
	listKeyMap.DownOneItem = keyMap.Next
	listKeyMap.UpOneItem = keyMap.Previous

	t := styles.CurrentTheme()

	// Get available themes
	themeManager := styles.DefaultManager()
	availableThemes := themeManager.List()
	currentTheme := themeManager.Current().Name

	items := make([]list.CompletionItem[Theme], len(availableThemes))
	for i, themeName := range availableThemes {
		isCurrent := themeName == currentTheme
		description := getThemeDescription(themeName)
		if isCurrent {
			description += " (current)"
		}

		themeItem := Theme{
			Name:        themeName,
			Description: description,
			IsCurrent:   isCurrent,
		}

		items[i] = list.NewCompletionItem(themeName, themeItem, list.WithCompletionID(themeName))
	}

	inputStyle := t.S().Base.PaddingLeft(1).PaddingBottom(1)
	themesList := list.NewFilterableList(
		items,
		list.WithFilterPlaceholder("Search for a theme..."),
		list.WithFilterInputStyle(inputStyle),
		list.WithFilterListOptions(
			list.WithKeyMap(listKeyMap),
			list.WithWrapNavigation(),
		),
	)

	help := help.New()
	help.Styles = t.S().Help

	return &themeDialogCmp{
		width:      defaultWidth,
		keyMap:     keyMap,
		themesList: themesList,
		help:       help,
	}
}

func (td *themeDialogCmp) Init() tea.Cmd {
	var cmds []tea.Cmd
	cmds = append(cmds, td.themesList.Init())
	cmds = append(cmds, td.themesList.Focus())
	return tea.Sequence(cmds...)
}

func (td *themeDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		td.wWidth = msg.Width
		td.wHeight = msg.Height
		td.width = min(60, td.wWidth-8)
		td.themesList.SetInputWidth(td.listWidth() - 2)
		return td, td.themesList.SetSize(td.listWidth(), td.listHeight())

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, td.keyMap.Select):
			selectedItem := td.themesList.SelectedItem()
			if selectedItem == nil {
				return td, nil // No item selected, do nothing
			}
			themeItem := (*selectedItem).Value()

			// Don't allow selecting the current theme
			if themeItem.IsCurrent {
				return td, nil
			}

			return td, tea.Sequence(
				util.CmdHandler(dialogs.CloseDialogMsg{}),
				util.CmdHandler(ThemeSelectedMsg{
					ThemeName: themeItem.Name,
				}),
			)

		case key.Matches(msg, td.keyMap.Close):
			return td, util.CmdHandler(dialogs.CloseDialogMsg{})

		default:
			u, cmd := td.themesList.Update(msg)
			td.themesList = u.(listTheme)
			return td, cmd
		}
	}
	return td, nil
}

func (td *themeDialogCmp) View() string {
	t := styles.CurrentTheme()

	listView := td.themesList.View()
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		t.S().Base.Padding(0, 1, 1, 1).Render(core.Title("Switch Theme", td.width-4)),
		listView,
		"",
		t.S().Base.Width(td.width-2).PaddingLeft(1).AlignHorizontal(lipgloss.Left).Render(td.help.View(td.keyMap)),
	)
	return td.style().Render(content)
}

func (td *themeDialogCmp) Cursor() *tea.Cursor {
	if cursor, ok := td.themesList.(util.Cursor); ok {
		cursor := cursor.Cursor()
		if cursor != nil {
			cursor = td.moveCursor(cursor)
		}
		return cursor
	}
	return nil
}

func (td *themeDialogCmp) style() lipgloss.Style {
	t := styles.CurrentTheme()
	return t.S().Base.
		Width(td.width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus)
}

func (td *themeDialogCmp) listWidth() int {
	return td.width - 2
}

func (td *themeDialogCmp) listHeight() int {
	return min(10, td.wHeight/2) // Limit height since there won't be many themes
}

func (td *themeDialogCmp) Position() (int, int) {
	row := td.wHeight/4 - 2 // just a bit above the center
	col := td.wWidth / 2
	col -= td.width / 2
	return row, col
}

func (td *themeDialogCmp) moveCursor(cursor *tea.Cursor) *tea.Cursor {
	row, col := td.Position()
	offset := row + 3 // Border + title
	cursor.Y += offset
	cursor.X = cursor.X + col + 2
	return cursor
}

func (td *themeDialogCmp) ID() dialogs.DialogID {
	return ThemesDialogID
}

// getThemeDescription returns a user-friendly description for the theme
func getThemeDescription(themeName string) string {
	switch themeName {
	case "charmtone":
		return "Dark theme with warm colors"
	case "charmtone-light":
		return "Light theme with comfortable contrast"
	default:
		return "Custom theme"
	}
}
