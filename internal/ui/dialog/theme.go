package dialog

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/sahilm/fuzzy"
)

const (
	ThemeID              = "theme"
	themeDialogMaxWidth  = 50
	themeDialogMaxHeight = 14
)

type Theme struct {
	com   *common.Common
	help  help.Model
	list  *list.FilterableList
	input textinput.Model

	keyMap struct {
		Select   key.Binding
		Next     key.Binding
		Previous key.Binding
		UpDown   key.Binding
		Close    key.Binding
	}
}

type ThemeItem struct {
	name      string
	label     string
	isCurrent bool
	t         *styles.Styles
	m         fuzzy.Match
	cache     map[int]string
	focused   bool
}

var (
	_ Dialog   = (*Theme)(nil)
	_ ListItem = (*ThemeItem)(nil)
)

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
		key.WithHelp("enter", "confirm"),
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
		switch {
		case key.Matches(msg, th.keyMap.Close):
			return ActionRevertThemePreview{}
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

func (th *Theme) Cursor() *tea.Cursor {
	return InputCursor(th.com.Styles, th.input.Cursor())
}

func (th *Theme) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := th.com.Styles
	width := max(0, min(themeDialogMaxWidth, area.Dx()))
	height := max(0, min(themeDialogMaxHeight, area.Dy()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.InputPrompt.GetVerticalFrameSize() + inputContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()

	th.input.SetWidth(innerWidth - t.Dialog.InputPrompt.GetHorizontalFrameSize() - 1)
	th.list.SetSize(innerWidth, height-heightOffset)
	th.help.SetWidth(innerWidth)

	rc := NewRenderContext(t, width)
	rc.Title = "Switch Theme"
	inputView := t.Dialog.InputPrompt.Render(th.input.View())
	rc.AddPart(inputView)

	visibleCount := len(th.list.FilteredItems())
	if th.list.Height() >= visibleCount {
		th.list.ScrollToTop()
	} else {
		th.list.ScrollToSelected()
	}

	listView := t.Dialog.List.Height(th.list.Height()).Render(th.list.Render())
	rc.AddPart(listView)
	rc.Help = th.help.View(th)

	view := rc.Render()

	cur := th.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

func (th *Theme) ShortHelp() []key.Binding {
	return []key.Binding{
		th.keyMap.UpDown,
		th.keyMap.Select,
		th.keyMap.Close,
	}
}

func (th *Theme) FullHelp() [][]key.Binding {
	m := [][]key.Binding{}
	slice := []key.Binding{
		th.keyMap.Select,
		th.keyMap.Next,
		th.keyMap.Previous,
		th.keyMap.Close,
	}
	for i := 0; i < len(slice); i += 4 {
		end := min(i+4, len(slice))
		m = append(m, slice[i:end])
	}
	return m
}

func (th *Theme) setThemeItems() {
	currentTheme := ""
	cfg := th.com.Config()
	if cfg != nil && cfg.Options != nil && cfg.Options.TUI != nil {
		currentTheme = cfg.Options.TUI.Theme
	}
	if currentTheme == "" {
		currentTheme = "charm"
	}

	names := styles.BuiltinThemeNames()
	items := make([]list.FilterableItem, 0, len(names))
	selectedIndex := 0
	for i, name := range names {
		item := &ThemeItem{
			name:      name,
			label:     name,
			isCurrent: name == currentTheme,
			t:         th.com.Styles,
		}
		items = append(items, item)
		if name == currentTheme {
			selectedIndex = i
		}
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
	}
	r.focused = focused
}

func (r *ThemeItem) SetMatch(m fuzzy.Match) {
	r.cache = nil
	r.m = m
}

func (r *ThemeItem) Render(width int) string {
	info := ""
	if r.isCurrent {
		info = "current"
	}
	s := ListItemStyles{
		ItemBlurred:     r.t.Dialog.NormalItem,
		ItemFocused:     r.t.Dialog.SelectedItem,
		InfoTextBlurred: r.t.Base,
		InfoTextFocused: r.t.Base,
	}
	return renderItem(s, r.label, info, r.focused, width, r.cache, &r.m)
}
