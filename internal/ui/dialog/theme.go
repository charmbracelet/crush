package dialog

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/themes"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/sahilm/fuzzy"
)

const (
	// ThemeID is the identifier for the theme picker dialog.
	ThemeID              = "theme"
	themeDialogMaxWidth  = 50
	themeDialogMaxHeight = 20
)

// ThemePicker is a dialog for browsing and selecting a theme. Navigating
// the list live-previews the highlighted theme across the whole UI, and the
// theme names re-render in the newly applied theme's colors.
type ThemePicker struct {
	com   *common.Common
	help  help.Model
	list  *list.FilterableList
	input textinput.Model

	schemes []themes.Scheme
	items   []*ThemeItem

	keyMap struct {
		Select   key.Binding
		Next     key.Binding
		Previous key.Binding
		UpDown   key.Binding
		Close    key.Binding
	}
}

type ThemeItem struct {
	*list.Versioned
	scheme    themes.Scheme
	isCurrent bool
	t         *styles.Styles
	m         fuzzy.Match
	focused   bool
}

var (
	_ Dialog   = (*ThemePicker)(nil)
	_ ListItem = (*ThemeItem)(nil)
)

func NewThemePicker(com *common.Common) *ThemePicker {
	d := &ThemePicker{com: com}

	h := help.New()
	h.Styles = com.Styles.DialogHelpStyles()
	d.help = h

	d.list = list.NewFilterableList()
	d.list.Focus()

	d.input = textinput.New()
	d.input.SetVirtualCursor(false)
	d.input.Placeholder = "Type to filter"
	d.input.SetStyles(com.Styles.TextInput)
	d.input.Focus()

	d.keyMap.Select = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "confirm"),
	)
	d.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("↓", "next theme"),
	)
	d.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑", "previous theme"),
	)
	d.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑/↓", "preview"),
	)
	d.keyMap.Close = CloseKey

	d.setItems()
	return d
}

// ID implements Dialog.
func (d *ThemePicker) ID() string {
	return ThemeID
}

// HandleMsg implements Dialog.
func (d *ThemePicker) HandleMsg(msg tea.Msg) Action {
	if kp, ok := msg.(tea.KeyPressMsg); ok {
		return d.handleKey(kp)
	}
	return nil
}

func (d *ThemePicker) handleKey(msg tea.KeyPressMsg) Action {
	switch {
	case key.Matches(msg, d.keyMap.Close):
		return ActionClose{}
	case key.Matches(msg, d.keyMap.Previous):
		return d.moveSelection(-1)
	case key.Matches(msg, d.keyMap.Next):
		return d.moveSelection(1)
	case key.Matches(msg, d.keyMap.Select):
		item := d.list.SelectedItem()
		if item == nil {
			break
		}
		ti, ok := item.(*ThemeItem)
		if !ok {
			break
		}
		return ActionSelectTheme{Name: ti.scheme.Name}
	default:
		prevValue := d.input.Value()
		var cmd tea.Cmd
		d.input, cmd = d.input.Update(msg)
		if d.input.Value() != prevValue {
			d.list.SetFilter(d.input.Value())
			d.list.ScrollToTop()
			d.list.SetSelected(0)
		}
		return ActionCmd{cmd}
	}
	return nil
}

func (d *ThemePicker) moveSelection(delta int) Action {
	d.list.Focus()
	if delta < 0 {
		if d.list.IsSelectedFirst() {
			d.list.SelectLast()
			d.list.ScrollToBottom()
		} else {
			d.list.SelectPrev()
			d.list.ScrollToSelected()
		}
	} else {
		if d.list.IsSelectedLast() {
			d.list.SelectFirst()
			d.list.ScrollToTop()
		} else {
			d.list.SelectNext()
			d.list.ScrollToSelected()
		}
	}
	d.bumpItems()
	return d.previewAction()
}

func (d *ThemePicker) previewAction() Action {
	item := d.list.SelectedItem()
	if ti, ok := item.(*ThemeItem); ok && ti != nil {
		return ActionPreviewTheme{Name: ti.scheme.Name}
	}
	return nil
}

// bumpItems bumps every item's version so the list re-renders them in the
// newly applied theme on the next draw.
func (d *ThemePicker) bumpItems() {
	for _, it := range d.items {
		it.Bump()
	}
}

// Cursor returns the cursor position relative to the dialog.
func (d *ThemePicker) Cursor() *tea.Cursor {
	return InputCursor(d.com.Styles, d.input.Cursor())
}

// Draw implements Dialog.
func (d *ThemePicker) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := d.com.Styles
	width := max(0, min(themeDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	height := max(0, min(themeDialogMaxHeight, area.Dy()-t.Dialog.View.GetVerticalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()

	d.input.SetWidth(dialogInputTextWidth(t, d.input, innerWidth))

	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.InputPrompt.GetVerticalFrameSize() + inputContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()

	d.list.SetSize(innerWidth, max(0, height-heightOffset))

	rc := NewRenderContext(t, width)
	rc.Title = "Theme"
	inputView := t.Dialog.InputPrompt.Render(d.input.View())
	rc.AddPart(inputView)

	if d.list.Height() >= len(d.list.FilteredItems()) {
		d.list.ScrollToTop()
	} else {
		d.list.ScrollToSelected()
	}

	listView := t.Dialog.List.Height(d.list.Height()).Render(d.list.Render())
	rc.AddPart(listView)
	rc.Help = renderDialogHelp(t, &d.help, d, innerWidth)

	view := rc.Render()
	cur := d.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

// ShortHelp implements help.KeyMap.
func (d *ThemePicker) ShortHelp() []key.Binding {
	return []key.Binding{
		d.keyMap.UpDown,
		d.keyMap.Select,
		d.keyMap.Close,
	}
}

// FullHelp implements help.KeyMap.
func (d *ThemePicker) FullHelp() [][]key.Binding {
	m := [][]key.Binding{}
	slice := []key.Binding{
		d.keyMap.Select,
		d.keyMap.Next,
		d.keyMap.Previous,
		d.keyMap.Close,
	}
	for i := 0; i < len(slice); i += 4 {
		end := min(i+4, len(slice))
		m = append(m, slice[i:end])
	}
	return m
}

func (d *ThemePicker) setItems() {
	current := d.currentThemeName()

	d.schemes = themes.Catalog()
	items := make([]list.FilterableItem, 0, len(d.schemes))
	themeItems := make([]*ThemeItem, 0, len(d.schemes))
	selectedIndex := 0
	for _, s := range d.schemes {
		isCurrent := strings.EqualFold(s.Name, current)
		item := &ThemeItem{
			Versioned: list.NewVersioned(),
			scheme:    s,
			isCurrent: isCurrent,
			t:         d.com.Styles,
		}
		if isCurrent {
			selectedIndex = len(items)
		}
		items = append(items, item)
		themeItems = append(themeItems, item)
	}
	d.items = themeItems

	d.list.SetItems(items...)
	d.list.SetSelected(selectedIndex)
	d.list.ScrollToSelected()
}

func (d *ThemePicker) currentThemeName() string {
	if d.com == nil || d.com.Workspace == nil {
		return "default"
	}
	if name := themes.ConfiguredTheme(d.com.Workspace.Config()); name != "" {
		return name
	}
	if d.com.IsHyper() {
		return "hyper"
	}
	return "default"
}

func (i *ThemeItem) Filter() string {
	return i.scheme.Name
}

func (i *ThemeItem) ID() string {
	return i.scheme.Name
}

// Finished implements list.Item. Theme items are render-stable outside of
// explicit SetFocused / SetMatch calls and the version bumps that signal a
// live theme change.
func (i *ThemeItem) Finished() bool {
	return true
}

func (i *ThemeItem) SetFocused(focused bool) {
	if i.focused == focused {
		return
	}
	i.focused = focused
	if i.Versioned != nil {
		i.Bump()
	}
}

func (i *ThemeItem) SetMatch(m fuzzy.Match) {
	if sameFuzzyMatch(i.m, m) {
		return
	}
	i.m = m
	if i.Versioned != nil {
		i.Bump()
	}
}

// Render styles the item with the currently applied theme's list colors.
func (i *ThemeItem) Render(width int) string {
	style := i.t.Dialog.NormalItem
	if i.focused {
		style = i.t.Dialog.SelectedItem
	}
	lineWidth := max(0, width-style.GetHorizontalFrameSize())

	name := i.scheme.Name
	if i.isCurrent {
		name += " (current)"
	}
	name = ansi.Truncate(name, lineWidth, "…")

	return style.Render(name)
}
