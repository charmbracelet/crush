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
	"github.com/charmbracelet/x/ansi"
	"github.com/sahilm/fuzzy"
)

const (
	ThemeID = "theme"

	// newThemeItemName is the sentinel name for the "New Theme..." entry.
	newThemeItemName = "__new_theme__"
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
	selectedIndex int

	keyMap struct {
		Select        key.Binding
		Next          key.Binding
		Previous      key.Binding
		UpDown        key.Binding
		EditTheme     key.Binding
		Rename        key.Binding
		ConfirmRename key.Binding
		CancelRename  key.Binding
		NewTheme      key.Binding
		Close         key.Binding
	}
}

// ThemeSectionHeader is a non-selectable section divider in the theme
// list. It renders as a titled horizontal rule via common.Section.
type ThemeSectionHeader struct {
	*list.Versioned
	title string
	t     *styles.Styles
}

func (h *ThemeSectionHeader) Filter() string { return "" }
func (h *ThemeSectionHeader) Finished() bool { return true }
func (h *ThemeSectionHeader) Render(width int) string {
	return common.Section(h.t, " "+h.title+" ", width)
}

// themeSpacer is a filterable spacer that adds vertical space between
// sections in the theme list. Unlike list.SpacerItem it satisfies
// FilterableItem so it can live in a FilterableList.
type themeSpacer struct {
	*list.Versioned
	height int
}

func (s *themeSpacer) Filter() string          { return "" }
func (s *themeSpacer) Finished() bool          { return true }
func (s *themeSpacer) Render(width int) string { return strings.Repeat("\n", s.height) }

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
	hideInfo  bool

	mode        themesMode
	renameInput textinput.Model
}

// Finished implements list.Item. Theme items are render-stable outside
// of explicit SetFocused / SetMatch calls.
func (r *ThemeItem) Finished() bool {
	return true
}

var (
	_ Dialog              = (*Theme)(nil)
	_ ListItem            = (*ThemeItem)(nil)
	_ list.FilterableItem = (*ThemeSectionHeader)(nil)
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
	th.keyMap.EditTheme = key.NewBinding(
		key.WithKeys("ctrl+e"),
		key.WithHelp("ctrl+e", "edit"),
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

// RefreshStyles invalidates cached renders on all theme items so they
// pick up in-place style mutations after a theme switch or preview.
func (th *Theme) RefreshStyles() {
	for _, item := range th.list.FilteredItems() {
		if ti, ok := item.(*ThemeItem); ok {
			ti.cache = nil
			ti.Bump()
		}
	}
}

// isSelectableThemeItem reports whether the item at the given index is a
// selectable ThemeItem (not a section header or spacer).
func (th *Theme) isSelectableThemeItem(idx int) bool {
	items := th.list.FilteredItems()
	if idx < 0 || idx >= len(items) {
		return false
	}
	_, ok := items[idx].(*ThemeItem)
	return ok
}

// selectNextTheme skips section headers and spacers when moving down.
func (th *Theme) selectNextTheme() {
	for {
		if th.list.IsSelectedLast() {
			th.list.SelectFirst()
			th.list.ScrollToTop()
		} else {
			th.list.SelectNext()
		}
		if th.isSelectableThemeItem(th.list.Selected()) {
			return
		}
	}
}

// selectPrevTheme skips section headers and spacers when moving up.
func (th *Theme) selectPrevTheme() {
	for {
		if th.list.IsSelectedFirst() {
			th.list.SelectLast()
			th.list.ScrollToBottom()
		} else {
			th.list.SelectPrev()
		}
		if th.isSelectableThemeItem(th.list.Selected()) {
			return
		}
	}
}

func (th *Theme) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch th.mode {
		case themesModeRenaming:
			switch {
			case key.Matches(msg, th.keyMap.ConfirmRename):
				action := th.confirmRename()
				th.setThemeItems()
				return action
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
			case key.Matches(msg, th.keyMap.EditTheme):
				selectedItem := th.list.SelectedItem()
				if selectedItem == nil {
					break
				}
				themeItem, ok := selectedItem.(*ThemeItem)
				if !ok || themeItem.name == newThemeItemName {
					break
				}
				return ActionOpenDialog{ThemeEditorID}
			case key.Matches(msg, th.keyMap.Rename):
				selectedItem := th.list.SelectedItem()
				if selectedItem == nil {
					break
				}
				themeItem, ok := selectedItem.(*ThemeItem)
				if !ok || themeItem.name == newThemeItemName {
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
				th.selectPrevTheme()
				th.list.ScrollToSelected()
				return th.previewAction()
			case key.Matches(msg, th.keyMap.Next):
				th.list.Focus()
				th.selectNextTheme()
				th.list.ScrollToSelected()
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
				if themeItem.name == newThemeItemName {
					return ActionOpenDialog{ThemeNewID}
				}
				return ActionSwitchTheme{Theme: themeItem.name}
			default:
				var cmd tea.Cmd
				th.input, cmd = th.input.Update(msg)
				value := th.input.Value()
				th.list.SetFilter(value)
				th.list.ScrollToTop()
				// Select first selectable item after filtering.
				th.list.SetSelected(0)
				if !th.isSelectableThemeItem(0) {
					th.selectNextTheme()
				}
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
		return nil
	}
	oldName := item.name
	if strings.EqualFold(newName, oldName) {
		return nil
	}
	return ActionRenameTheme{OldName: oldName, NewName: newName}
}

func (th *Theme) selectedThemeItem() *ThemeItem {
	if item := th.list.SelectedItem(); item != nil {
		if ti, ok := item.(*ThemeItem); ok {
			return ti
		}
	}
	return nil
}

func (th *Theme) previewAction() Action {
	selectedItem := th.list.SelectedItem()
	if selectedItem == nil {
		return nil
	}
	themeItem, ok := selectedItem.(*ThemeItem)
	if !ok || themeItem.name == newThemeItemName {
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
	width := max(0, min(defaultDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	height := max(0, min(defaultDialogHeight, area.Dy()-t.Dialog.View.GetVerticalBorderSize()))
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

// canRenameSelected reports whether the currently selected theme item
// supports renaming (user-defined, not the sentinel).
func (th *Theme) canRenameSelected() bool {
	item := th.selectedThemeItem()
	if item == nil || item.name == newThemeItemName {
		return false
	}
	return !styles.IsBuiltinTheme(item.name)
}

// canEditSelected reports whether the currently selected theme item
// supports editing (not the sentinel).
func (th *Theme) canEditSelected() bool {
	item := th.selectedThemeItem()
	return item != nil && item.name != newThemeItemName
}

func (th *Theme) ShortHelp() []key.Binding {
	bindings := []key.Binding{
		th.keyMap.UpDown,
	}
	if th.mode == themesModeNormal {
		if th.canEditSelected() {
			bindings = append(bindings, th.keyMap.EditTheme)
		}
		if th.canRenameSelected() {
			bindings = append(bindings, th.keyMap.Rename)
		}
	}
	bindings = append(bindings, th.keyMap.Select, th.keyMap.Close)
	return bindings
}

func (th *Theme) FullHelp() [][]key.Binding {
	if th.mode == themesModeRenaming {
		return [][]key.Binding{{th.keyMap.ConfirmRename, th.keyMap.CancelRename}}
	}
	row2 := []key.Binding{th.keyMap.Close}
	if th.canRenameSelected() {
		row2 = append([]key.Binding{th.keyMap.Rename}, row2...)
	}
	if th.canEditSelected() {
		row2 = append([]key.Binding{th.keyMap.EditTheme}, row2...)
	}
	return [][]key.Binding{
		{th.keyMap.Select, th.keyMap.Next, th.keyMap.Previous},
		row2,
	}
}

func (th *Theme) setThemeItems() {
	currentTheme := th.currentThemeName()
	allThemes := styles.ListAllThemes()

	// Separate builtin (including overridden) from user-only themes.
	var systemThemes, userThemes []styles.ThemeInfo
	for _, info := range allThemes {
		if info.Source == styles.ThemeSourceBuiltin || info.Overridden {
			systemThemes = append(systemThemes, info)
		} else {
			userThemes = append(userThemes, info)
		}
	}

	items := make([]list.FilterableItem, 0, len(allThemes)+5)

	// "New Theme..." sentinel — no divider above it.
	items = append(items, &ThemeItem{
		Versioned: &list.Versioned{},
		name:      newThemeItemName,
		label:     "New Theme...",
		t:         th.com.Styles,
		mode:      th.mode,
	})

	// Spacer after "New Theme...".
	items = append(items, &themeSpacer{Versioned: &list.Versioned{}, height: 1})

	// System section.
	items = append(items, &ThemeSectionHeader{
		Versioned: &list.Versioned{},
		title:     "System",
		t:         th.com.Styles,
	})
	for _, info := range systemThemes {
		items = append(items, th.newThemeItem(info, currentTheme))
	}

	// Spacer between sections.
	items = append(items, &themeSpacer{Versioned: &list.Versioned{}, height: 1})

	// User section.
	items = append(items, &ThemeSectionHeader{
		Versioned: &list.Versioned{},
		title:     "User",
		t:         th.com.Styles,
	})
	for _, info := range userThemes {
		items = append(items, th.newThemeItem(info, currentTheme))
	}

	th.list.SetItems(items...)

	// Restore selection or default to current theme.
	if th.mode == themesModeRenaming && th.selectedIndex >= 0 && th.selectedIndex < len(items) {
		th.list.SetSelected(th.selectedIndex)
	} else {
		// Default to "New Theme..." (index 0).
		th.list.SetSelected(0)
	}
	th.list.ScrollToSelected()
}

func (th *Theme) newThemeItem(info styles.ThemeInfo, currentTheme string) *ThemeItem {
	label := info.Name
	if info.Overridden {
		label += " (overridden)"
	} else if info.Source != styles.ThemeSourceBuiltin {
		label += " (" + info.Source.String() + ")"
	}
	item := &ThemeItem{
		Versioned: &list.Versioned{},
		name:      info.Name,
		label:     label,
		isCurrent: info.Name == currentTheme,
		t:         th.com.Styles,
		mode:      th.mode,
	}
	if th.mode == themesModeRenaming && th.selectedIndex >= 0 {
		filteredItems := th.list.FilteredItems()
		if th.selectedIndex < len(filteredItems) {
			if si, ok := filteredItems[th.selectedIndex].(*ThemeItem); ok && si.name == info.Name {
				item.renameInput = textinput.New()
				item.renameInput.SetVirtualCursor(false)
				item.renameInput.Prompt = ""
				inputStyle := th.com.Styles.TextInput
				inputStyle.Focused.Placeholder = th.com.Styles.Dialog.Sessions.RenamingPlaceholder
				item.renameInput.SetStyles(inputStyle)
				item.renameInput.SetValue(info.Name)
				item.renameInput.Focus()
			}
		}
	}
	return item
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

// InfoText returns the secondary text shown on the right of the item.
func (r *ThemeItem) InfoText() string {
	if r.isCurrent {
		return "current"
	}
	return ""
}

// SetHideInfo controls whether the info column is shown.
func (r *ThemeItem) SetHideInfo(v bool) {
	if r.hideInfo == v {
		return
	}
	r.cache = nil
	r.hideInfo = v
	if r.Versioned != nil {
		r.Bump()
	}
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
	info := r.InfoText()
	if r.hideInfo {
		info = ""
	}
	s := ListItemStyles{
		ItemBlurred:     r.t.Dialog.NormalItem,
		ItemFocused:     r.t.Dialog.SelectedItem,
		InfoTextBlurred: r.t.Dialog.Sessions.InfoBlurred,
		InfoTextFocused: r.t.Dialog.Sessions.InfoFocused,
	}

	switch r.mode {
	case themesModeRenaming:
		s.ItemBlurred = r.t.Dialog.Sessions.RenamingItemBlurred
		s.ItemFocused = r.t.Dialog.Sessions.RenamingingItemFocused
		if r.focused {
			const cursorPadding = 1
			inputWidth := max(0, width-s.ItemFocused.GetHorizontalFrameSize()-cursorPadding)
			r.renameInput.SetWidth(inputWidth)
			r.renameInput.Placeholder = ansi.Truncate(r.label, width, "…")
			return s.ItemFocused.Render(r.renameInput.View())
		}
	}

	return renderItem(s, r.label, info, r.focused, width, r.cache, &r.m)
}
