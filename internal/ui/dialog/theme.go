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
	// ThemeID is the identifier for the theme dialog.
	ThemeID              = "theme"
	themeDialogMaxWidth  = 50
	themeDialogMinHeight = 8
	themeDialogMaxHeight = 16
)

// Theme represents a dialog for selecting the UI theme.
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

// ThemeItem represents a theme list item.
type ThemeItem struct {
	*list.Versioned
	name      string
	title     string
	isCurrent bool
	t         *styles.Styles
	m         fuzzy.Match
	cache     map[int]string
	focused   bool
}

// Finished implements list.Item. Theme items are render-stable outside of
// explicit SetFocused / SetMatch.
func (t *ThemeItem) Finished() bool { return true }

var (
	_ Dialog   = (*Theme)(nil)
	_ ListItem = (*ThemeItem)(nil)
)

// NewTheme creates a new theme dialog, marking `current` as active.
func NewTheme(com *common.Common, current string) (*Theme, error) {
	th := &Theme{com: com}

	help := help.New()
	help.Styles = com.Styles.DialogHelpStyles()
	th.help = help

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

	items := make([]list.FilterableItem, 0, len(styles.ThemeNames()))
	for _, name := range styles.ThemeNames() {
		items = append(items, newThemeItem(com.Styles, name, name == current))
	}
	th.list.SetItems(items...)
	th.list.SetSelected(0)

	return th, nil
}

func newThemeItem(t *styles.Styles, name string, isCurrent bool) *ThemeItem {
	return &ThemeItem{
		Versioned: list.NewVersioned(),
		t:         t,
		name:      name,
		title:     name,
		isCurrent: isCurrent,
	}
}

// ID implements Dialog.
func (t *Theme) ID() string { return ThemeID }

// HandleMsg implements [Dialog].
func (t *Theme) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, t.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, t.keyMap.Previous):
			t.list.Focus()
			if t.list.IsSelectedFirst() {
				t.list.SelectLast()
				t.list.ScrollToBottom()
				break
			}
			t.list.SelectPrev()
			t.list.ScrollToSelected()
		case key.Matches(msg, t.keyMap.Next):
			t.list.Focus()
			if t.list.IsSelectedLast() {
				t.list.SelectFirst()
				t.list.ScrollToTop()
				break
			}
			t.list.SelectNext()
			t.list.ScrollToSelected()
		case key.Matches(msg, t.keyMap.Select):
			selectedItem := t.list.SelectedItem()
			if selectedItem == nil {
				break
			}
			item, ok := selectedItem.(*ThemeItem)
			if !ok {
				break
			}
			return ActionApplyTheme{Name: item.name}
		default:
			var cmd tea.Cmd
			t.input, cmd = t.input.Update(msg)
			value := t.input.Value()
			t.list.SetFilter(value)
			t.list.ScrollToTop()
			t.list.SetSelected(0)
			return ActionCmd{cmd}
		}
	}
	return nil
}

// Cursor returns the cursor position relative to the dialog.
func (t *Theme) Cursor() *tea.Cursor {
	return InputCursor(t.com.Styles, t.input.Cursor())
}

// ShortHelp implements [help.KeyMap].
func (t *Theme) ShortHelp() []key.Binding {
	return []key.Binding{
		t.keyMap.UpDown,
		t.keyMap.Select,
		t.keyMap.Close,
	}
}

// FullHelp implements [help.KeyMap].
func (t *Theme) FullHelp() [][]key.Binding {
	slice := []key.Binding{
		t.keyMap.Select,
		t.keyMap.Next,
		t.keyMap.Previous,
		t.keyMap.Close,
	}
	m := [][]key.Binding{}
	for i := 0; i < len(slice); i += 4 {
		end := min(i+4, len(slice))
		m = append(m, slice[i:end])
	}
	return m
}

// Filter returns the filter value for the theme item.
func (ti *ThemeItem) Filter() string { return ti.title }

// ID returns the unique identifier for the theme.
func (ti *ThemeItem) ID() string { return ti.name }

// SetFocused sets the focus state of the theme item.
func (ti *ThemeItem) SetFocused(focused bool) {
	if ti.focused == focused {
		return
	}
	ti.cache = nil
	ti.focused = focused
	if ti.Versioned != nil {
		ti.Bump()
	}
}

// SetMatch sets the fuzzy match for the theme item.
func (ti *ThemeItem) SetMatch(m fuzzy.Match) {
	if sameFuzzyMatch(ti.m, m) {
		return
	}
	ti.cache = nil
	ti.m = m
	if ti.Versioned != nil {
		ti.Bump()
	}
}

// Render returns the string representation of the theme item.
func (ti *ThemeItem) Render(width int) string {
	info := ""
	if ti.isCurrent {
		info = "current"
	}
	listStyles := ListItemStyles{
		ItemBlurred:     ti.t.Dialog.NormalItem,
		ItemFocused:     ti.t.Dialog.SelectedItem,
		InfoTextBlurred: ti.t.Dialog.ListItem.InfoBlurred,
		InfoTextFocused: ti.t.Dialog.ListItem.InfoFocused,
	}
	return renderItem(listStyles, ti.title, info, ti.focused, width, ti.cache, &ti.m)
}

// Draw implements [Dialog].
func (t *Theme) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	st := t.com.Styles
	width := max(0, min(themeDialogMaxWidth, area.Dx()-st.Dialog.View.GetHorizontalBorderSize()))
	innerWidth := width - st.Dialog.View.GetHorizontalFrameSize()
	height := max(themeDialogMinHeight, min(themeDialogMaxHeight, area.Dy()-st.Dialog.View.GetVerticalBorderSize()))

	t.input.SetWidth(dialogInputTextWidth(st, t.input, innerWidth))
	t.list.SetSize(innerWidth, max(0, height-4))

	rc := NewRenderContext(st, width)
	rc.Title = "Theme"

	inputView := st.Dialog.InputPrompt.Render(t.input.View())
	rc.AddPart(inputView)

	listView := st.Dialog.List.Height(t.list.Height()).Render(t.list.Render())
	rc.AddPart(listView)

	rc.Help = renderDialogHelp(st, &t.help, t, innerWidth)

	view := rc.Render()
	cur := t.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}
