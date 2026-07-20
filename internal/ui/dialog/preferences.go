package dialog

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	uv "github.com/charmbracelet/ultraviolet"
)

const (
	// PreferencesID is the identifier for the customization submenu dialog.
	PreferencesID                = "customization"
	customizationDialogMaxWidth  = 50
	customizationDialogMaxHeight = 10
)

// Preferences represents a submenu dialog for UI customization settings.
type Preferences struct {
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

var _ Dialog = (*Preferences)(nil)

// NewPreferences creates a new customization submenu dialog.
func NewPreferences(com *common.Common) *Preferences {
	c := &Preferences{com: com}

	h := help.New()
	h.Styles = com.Styles.DialogHelpStyles()
	c.help = h

	c.list = list.NewFilterableList()
	c.list.Focus()

	c.input = textinput.New()
	c.input.SetVirtualCursor(false)
	c.input.Placeholder = "Type to filter"
	c.input.SetStyles(com.Styles.TextInput)
	c.input.Focus()

	c.keyMap.Select = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "confirm"),
	)
	c.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("↓", "next item"),
	)
	c.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑", "previous item"),
	)
	c.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑/↓", "choose"),
	)
	c.keyMap.Close = CloseKey

	c.setItems()
	return c
}

// ID implements Dialog.
func (c *Preferences) ID() string {
	return PreferencesID
}

// HandleMsg implements [Dialog].
func (c *Preferences) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, c.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, c.keyMap.Previous):
			c.list.Focus()
			if c.list.IsSelectedFirst() {
				c.list.SelectLast()
				c.list.ScrollToBottom()
				break
			}
			c.list.SelectPrev()
			c.list.ScrollToSelected()
		case key.Matches(msg, c.keyMap.Next):
			c.list.Focus()
			if c.list.IsSelectedLast() {
				c.list.SelectFirst()
				c.list.ScrollToTop()
				break
			}
			c.list.SelectNext()
			c.list.ScrollToSelected()
		case key.Matches(msg, c.keyMap.Select):
			selectedItem := c.list.SelectedItem()
			if selectedItem == nil {
				break
			}
			cmdItem, ok := selectedItem.(*CommandItem)
			if !ok {
				break
			}
			return cmdItem.Action()
		default:
			var cmd tea.Cmd
			c.input, cmd = c.input.Update(msg)
			value := c.input.Value()
			c.list.SetFilter(value)
			c.list.ScrollToTop()
			c.list.SetSelected(0)
			return ActionCmd{cmd}
		}
	}
	return nil
}

// Cursor returns the cursor position relative to the dialog.
func (c *Preferences) Cursor() *tea.Cursor {
	return InputCursor(c.com.Styles, c.input.Cursor())
}

// Draw implements [Dialog].
func (c *Preferences) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := c.com.Styles
	width := max(0, min(customizationDialogMaxWidth, area.Dx()))
	height := max(0, min(customizationDialogMaxHeight, area.Dy()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.InputPrompt.GetVerticalFrameSize() + inputContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()

	c.input.SetWidth(innerWidth - t.Dialog.InputPrompt.GetHorizontalFrameSize() - 1)
	c.list.SetSize(innerWidth, height-heightOffset)
	c.help.SetWidth(innerWidth)

	rc := NewRenderContext(t, width)
	rc.Title = "Preferences"
	inputView := t.Dialog.InputPrompt.Render(c.input.View())
	rc.AddPart(inputView)

	visibleCount := len(c.list.FilteredItems())
	if c.list.Height() >= visibleCount {
		c.list.ScrollToTop()
	} else {
		c.list.ScrollToSelected()
	}

	listView := t.Dialog.List.Height(c.list.Height()).Render(c.list.Render())
	rc.AddPart(listView)
	rc.Help = c.help.View(c)

	view := rc.Render()

	cur := c.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

// ShortHelp implements [help.KeyMap].
func (c *Preferences) ShortHelp() []key.Binding {
	return []key.Binding{
		c.keyMap.UpDown,
		c.keyMap.Select,
		c.keyMap.Close,
	}
}

// FullHelp implements [help.KeyMap].
func (c *Preferences) FullHelp() [][]key.Binding {
	m := [][]key.Binding{}
	slice := []key.Binding{
		c.keyMap.Select,
		c.keyMap.Next,
		c.keyMap.Previous,
		c.keyMap.Close,
	}
	for i := 0; i < len(slice); i += 4 {
		end := min(i+4, len(slice))
		m = append(m, slice[i:end])
	}
	return m
}

func (c *Preferences) setItems() {
	cfg := c.com.Config()
	transparentLabel := "Enable Transparent Mode"
	if cfg != nil && cfg.Options != nil && cfg.Options.TUI.Transparent != nil && *cfg.Options.TUI.Transparent {
		transparentLabel = "Disable Transparent Mode"
	}

	items := []list.FilterableItem{
		NewCommandItem(c.com.Styles, "notification_style", "Notification Style", "", ActionOpenDialog{DialogID: NotificationsID}),
		NewCommandItem(c.com.Styles, "scrollbar_style", "Scrollbar", "", ActionOpenDialog{DialogID: ScrollbarID}),
		NewCommandItem(c.com.Styles, "toggle_transparent", transparentLabel, "", ActionToggleTransparentBackground{}),
	}

	c.list.SetItems(items...)
	if c.list.Selected() == -1 {
		c.list.SetSelected(0)
		c.list.ScrollToTop()
	}
}

// RefreshItems rebuilds the menu items to reflect current config state.
func (c *Preferences) RefreshItems() {
	c.setItems()
}
