package dialog

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/sahilm/fuzzy"
)

const (
	ThemeID              = "theme"
	themeDialogMaxWidth  = 50
	themeDialogMaxHeight = 20
)

type themesMode uint8

const (
	themesModeNormal themesMode = iota
	themesModeRenaming
)

// Theme is the theme management dialog. It lists all available themes
// with shortcuts to switch, rename, and create new themes.
type Theme struct {
	com   *common.Common
	help  help.Model
	list  *list.FilterableList
	input textinput.Model

	mode          themesMode
	renameInput   textinput.Model
	selectedIndex int

	keyMap struct {
		Select        key.Binding
		Next          key.Binding
		Previous      key.Binding
		UpDown        key.Binding
		Rename        key.Binding
		ConfirmRename key.Binding
		CancelRename  key.Binding
		NewTheme      key.Binding
		Close         key.Binding
	}
}

// ThemeItem represents a single theme entry in the picker.
type ThemeItem struct {
	*list.Versioned
	name      string
	label     string
	isCurrent bool
	t         *styles.Styles
	m         fuzzy.Match
	cache     map[int]string
	focused   bool

	renameMode  bool
	renameInput textinput.Model
}

// Finished implements list.Item. Theme items are render-stable outside
// of explicit SetFocused / SetMatch calls.
func (r *ThemeItem) Finished() bool {
	return true
}

var (
	_ Dialog   = (*Theme)(nil)
	_ ListItem = (*ThemeItem)(nil)
)

// NewTheme creates a new theme management dialog.
func NewTheme(com *common.Common) *Theme {
	th := &Theme{com: com}

	h := help.New()
	h.Styles = com.Styles.DialogHelpStyles()
	th.help = h

	th.list = list.NewFilterableList()
	th.list.Focus()

	th.input = textinput.New()
	th.input.SetVirtualCursor(false)
	th.input.Placeholder = "Type to filter"
	th.input.SetStyles(com.Styles.TextInput)
	th.input.Focus()

	th.renameInput = textinput.New()
	th.renameInput.SetVirtualCursor(false)
	th.renameInput.Placeholder = "Enter theme name"
	th.renameInput.SetStyles(com.Styles.TextInput)

	th.keyMap.Select = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "choose"),
	)
	th.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("↓", "next item"),
	)
	th.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑", "previous item"),
	)
	th.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑/↓", "choose"),
	)
	th.keyMap.Rename = key.NewBinding(
		key.WithKeys("ctrl+r"),
		key.WithHelp("ctrl+r", "rename"),
	)
	th.keyMap.ConfirmRename = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "confirm"),
	)
	th.keyMap.CancelRename = key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "cancel"),
	)
	th.keyMap.NewTheme = key.NewBinding(
		key.WithKeys("ctrl+n"),
		key.WithHelp("ctrl+n", "new theme"),
	)
	th.keyMap.Close = CloseKey

	th.setThemeItems()
	return th
}

func (th *Theme) ID() string {
	return ThemeID
}

func (th *Theme) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch th.mode {
		case themesModeRenaming:
			switch {
			case key.Matches(msg, th.keyMap.ConfirmRename):
				return th.confirmRename()
			case key.Matches(msg, th.keyMap.CancelRename):
				th.mode = themesModeNormal
				th.setThemeItems()
			default:
				item := th.list.SelectedItem()
				if item == nil {
					return nil
				}
				if themeItem, ok := item.(*ThemeItem); ok {
					return themeItem.HandleInput(msg)
				}
			}
		default:
			switch {
			case key.Matches(msg, th.keyMap.Close):
				return ActionRevertThemePreview{}
			case key.Matches(msg, th.keyMap.Rename):
				selectedItem := th.list.SelectedItem()
				if selectedItem == nil {
					break
				}
				themeItem, ok := selectedItem.(*ThemeItem)
				if !ok {
					break
				}
				if styles.IsBuiltinTheme(themeItem.name) {
					break
				}
				th.selectedIndex = th.list.Selected()
				th.mode = themesModeRenaming
				th.setThemeItems()
			case key.Matches(msg, th.keyMap.NewTheme):
				return ActionOpenDialog{ThemeNewID}
			case key.Matches(msg, th.keyMap.Previous):
				th.list.Focus()
				if th.list.IsSelectedFirst() {
					th.list.SelectLast()
					th.list.ScrollToBottom()
				} else {
					th.list.SelectPrev()
					th.list.ScrollToSelected()
				}
				return th.previewAction()
			case key.Matches(msg, th.keyMap.Next):
				th.list.Focus()
				if th.list.IsSelectedLast() {
					th.list.SelectFirst()
					th.list.ScrollToTop()
				} else {
					th.list.SelectNext()
					th.list.ScrollToSelected()
				}
				return th.previewAction()
			case key.Matches(msg, th.keyMap.Select):
				selectedItem := th.list.SelectedItem()
				if selectedItem == nil {
					break
				}
				themeItem, ok := selectedItem.(*ThemeItem)
				if !ok {
					break
				}
				return ActionSwitchTheme{Theme: themeItem.name}
			default:
				var cmd tea.Cmd
				th.input, cmd = th.input.Update(msg)
				value := th.input.Value()
				th.list.SetFilter(value)
				th.list.ScrollToTop()
				th.list.SetSelected(0)
				return ActionCmd{cmd}
			}
		}
	}
	return nil
}

func (th *Theme) confirmRename() Action {
	item := th.selectedThemeItem()
	th.mode = themesModeNormal
	if item == nil {
		return nil
	}
	newName := strings.TrimSpace(item.InputValue())
	if newName == "" {
		th.setThemeItems()
		return nil
	}
	oldName := item.name
	if strings.EqualFold(newName, oldName) {
		th.setThemeItems()
		return nil
	}
	th.setThemeItems()
	return ActionRenameTheme{OldName: oldName, NewName: newName}
}

func (th *Theme) selectedThemeItem() *ThemeItem {
	if item := th.list.SelectedItem(); item != nil {
		return item.(*ThemeItem)
	}
	return nil
}

func (th *Theme) previewAction() Action {
	selectedItem := th.list.SelectedItem()
	if selectedItem == nil {
		return nil
	}
	themeItem, ok := selectedItem.(*ThemeItem)
	if !ok {
		return nil
	}
	return ActionPreviewTheme{Theme: themeItem.name}
}

// currentThemeName returns the active theme name from config, defaulting
// to charmtone when unset.
func (th *Theme) currentThemeName() string {
	cfg := th.com.Config()
	if cfg == nil || cfg.Options == nil || cfg.Options.TUI == nil || cfg.Options.TUI.ActiveTheme == "" {
		return "charmtone"
	}
	return cfg.Options.TUI.ActiveTheme
}

func (th *Theme) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := th.com.Styles
	width := max(0, min(themeDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	height := max(0, min(themeDialogMaxHeight, area.Dy()-t.Dialog.View.GetVerticalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()

	th.input.SetWidth(dialogInputTextWidth(t, th.input, innerWidth))
	listHeight, listTotalHeight, _ := sizeDialogList(t, th.list, innerWidth, height)

	var cur *tea.Cursor
	rc := NewRenderContext(t, width)
	rc.Title = "Themes"
	switch th.mode {
	case themesModeRenaming:
		rc.TitleStyle = t.Dialog.Sessions.RenamingingTitle
		rc.TitleGradientFromColor = t.Dialog.Sessions.RenamingTitleGradientFromColor
		rc.TitleGradientToColor = t.Dialog.Sessions.RenamingTitleGradientToColor
		rc.ViewStyle = t.Dialog.Sessions.RenamingView
		message := t.Dialog.Sessions.RenamingingMessage.Render("Rename this theme?")
		rc.AddPart(message)
		item := th.selectedThemeItem()
		if item == nil {
			return nil
		}
		cur = item.Cursor()
		start, end := th.list.VisibleItemIndices()
		cur = renameCursorOffset(t, cur, lipgloss.Height(message), start, end, th.list.Selected())
	default:
		inputView := t.Dialog.InputPrompt.Render(th.input.View())
		rc.AddPart(inputView)
		cur = InputCursor(t, th.input.Cursor())
	}

	visibleCount := len(th.list.FilteredItems())
	if th.list.Height() >= visibleCount {
		th.list.ScrollToTop()
	} else {
		th.list.ScrollToSelected()
	}

	listView := t.Dialog.List.Height(th.list.Height()).Render(th.list.Render())
	listView = joinScrollbar(t, listView, listHeight, listTotalHeight, listHeight, th.list.Offset())
	rc.AddPart(listView)
	rc.Help = renderDialogHelp(t, &th.help, th, innerWidth)

	view := rc.Render()

	DrawCenterCursor(scr, area, view, cur)
	return cur
}

func (th *Theme) ShortHelp() []key.Binding {
	bindings := []key.Binding{
		th.keyMap.UpDown,
		th.keyMap.Select,
		th.keyMap.Close,
	}
	if th.mode == themesModeNormal {
		bindings = append(bindings, th.keyMap.Rename, th.keyMap.NewTheme)
	}
	return bindings
}

func (th *Theme) FullHelp() [][]key.Binding {
	if th.mode == themesModeRenaming {
		return [][]key.Binding{{th.keyMap.ConfirmRename, th.keyMap.CancelRename}}
	}
	return [][]key.Binding{
		{th.keyMap.Select, th.keyMap.Next, th.keyMap.Previous},
		{th.keyMap.Rename, th.keyMap.NewTheme, th.keyMap.Close},
	}
}

func (th *Theme) setThemeItems() {
	currentTheme := th.currentThemeName()

	allThemes := styles.ListAllThemes()
	items := make([]list.FilterableItem, 0, len(allThemes))
	currentIndex := 0

	for i, info := range allThemes {
		if info.Name == currentTheme {
			currentIndex = i
		}
	}

	// In rename mode, preserve the previously selected index.
	selectedIndex := currentIndex
	if th.mode == themesModeRenaming && th.selectedIndex >= 0 && th.selectedIndex < len(allThemes) {
		selectedIndex = th.selectedIndex
	}

	for i, info := range allThemes {
		label := info.Name
		if info.Overridden {
			label += " (overridden)"
		} else if info.Source != styles.ThemeSourceBuiltin {
			label += " (" + info.Source.String() + ")"
		}
		item := &ThemeItem{
			Versioned:  &list.Versioned{},
			name:       info.Name,
			label:      label,
			isCurrent:  info.Name == currentTheme,
			t:          th.com.Styles,
			renameMode: th.mode == themesModeRenaming && i == selectedIndex,
		}
		if item.renameMode {
			item.renameInput = textinput.New()
			item.renameInput.SetVirtualCursor(false)
			item.renameInput.SetValue(info.Name)
			item.renameInput.SetStyles(th.com.Styles.TextInput)
			item.renameInput.Focus()
		}
		items = append(items, item)
	}

	th.list.SetItems(items...)
	th.list.SetSelected(selectedIndex)
	th.list.ScrollToSelected()
}

func (r *ThemeItem) Filter() string {
	return r.label
}

func (r *ThemeItem) ID() string {
	return r.name
}

func (r *ThemeItem) SetFocused(focused bool) {
	if r.focused != focused {
		r.cache = nil
		r.Bump()
	}
	r.focused = focused
}

func (r *ThemeItem) SetMatch(m fuzzy.Match) {
	if !sameFuzzyMatch(r.m, m) {
		r.cache = nil
		r.Bump()
	}
	r.m = m
}

func (r *ThemeItem) InputValue() string {
	return r.renameInput.Value()
}

func (r *ThemeItem) HandleInput(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	r.renameInput, cmd = r.renameInput.Update(msg)
	if r.Versioned != nil {
		r.Bump()
	}
	return cmd
}

func (r *ThemeItem) Cursor() *tea.Cursor {
	return r.renameInput.Cursor()
}

func (r *ThemeItem) Render(width int) string {
	if r.renameMode && r.focused {
		s := ListItemStyles{
			ItemBlurred:     r.t.Dialog.NormalItem,
			ItemFocused:     r.t.Dialog.SelectedItem,
			InfoTextBlurred: r.t.Dialog.Sessions.InfoBlurred,
			InfoTextFocused: r.t.Dialog.Sessions.InfoFocused,
		}
		const cursorPadding = 1
		inputWidth := max(0, width-s.ItemFocused.GetHorizontalFrameSize()-cursorPadding)
		r.renameInput.SetWidth(inputWidth)
		r.renameInput.Placeholder = r.name
		return s.ItemFocused.Render(r.renameInput.View())
	}

	info := ""
	if r.isCurrent {
		info = "current"
	}
	s := ListItemStyles{
		ItemBlurred:     r.t.Dialog.NormalItem,
		ItemFocused:     r.t.Dialog.SelectedItem,
		InfoTextBlurred: r.t.Dialog.Sessions.InfoBlurred,
		InfoTextFocused: r.t.Dialog.Sessions.InfoFocused,
	}
	return renderItem(s, r.label, info, r.focused, width, r.cache, &r.m)
}
