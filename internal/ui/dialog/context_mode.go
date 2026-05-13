package dialog

import (
	"errors"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/taigrr/crush/internal/config"
	"github.com/taigrr/crush/internal/ui/common"
	"github.com/taigrr/crush/internal/ui/list"
	"github.com/taigrr/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/sahilm/fuzzy"
)

const (
	// ContextModeID is the identifier for the context mode dialog.
	ContextModeID              = "context_mode"
	contextModeDialogMaxWidth  = 50
	contextModeDialogMaxHeight = 10
)

// ContextMode represents a dialog for selecting context mode.
type ContextMode struct {
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

// ContextModeItem represents a context mode list item.
type ContextModeItem struct {
	mode      config.ContextMode
	title     string
	desc      string
	isCurrent bool
	t         *styles.Styles
	m         fuzzy.Match
	cache     map[int]string
	focused   bool
}

var (
	_ Dialog   = (*ContextMode)(nil)
	_ ListItem = (*ContextModeItem)(nil)
)

// NewContextMode creates a new context mode dialog.
func NewContextMode(com *common.Common) (*ContextMode, error) {
	c := &ContextMode{com: com}

	help := help.New()
	help.Styles = com.Styles.DialogHelpStyles()
	c.help = help

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

	if err := c.setContextModeItems(); err != nil {
		return nil, err
	}

	return c, nil
}

// ID implements Dialog.
func (c *ContextMode) ID() string {
	return ContextModeID
}

// HandleMsg implements [Dialog].
func (c *ContextMode) HandleMsg(msg tea.Msg) Action {
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
			item, ok := selectedItem.(*ContextModeItem)
			if !ok {
				break
			}
			return ActionSelectContextMode{Mode: item.mode}
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
func (c *ContextMode) Cursor() *tea.Cursor {
	return InputCursor(c.com.Styles, c.input.Cursor())
}

// Draw implements [Dialog].
func (c *ContextMode) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := c.com.Styles
	width := max(0, min(contextModeDialogMaxWidth, area.Dx()))
	height := max(0, min(contextModeDialogMaxHeight, area.Dy()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.InputPrompt.GetVerticalFrameSize() + inputContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()

	c.input.SetWidth(innerWidth - t.Dialog.InputPrompt.GetHorizontalFrameSize() - 1)
	c.list.SetSize(innerWidth, height-heightOffset)
	c.help.SetWidth(innerWidth)

	rc := NewRenderContext(t, width)
	rc.Title = "Select Context Mode"
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
func (c *ContextMode) ShortHelp() []key.Binding {
	return []key.Binding{
		c.keyMap.UpDown,
		c.keyMap.Select,
		c.keyMap.Close,
	}
}

// FullHelp implements [help.KeyMap].
func (c *ContextMode) FullHelp() [][]key.Binding {
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

func (c *ContextMode) setContextModeItems() error {
	cfg := c.com.Config()
	agentCfg, ok := cfg.Agents[config.AgentCoder]
	if !ok {
		return errors.New("agent configuration not found")
	}

	selectedModel := cfg.Models[agentCfg.Model]
	model := cfg.GetModelByType(agentCfg.Model)
	if model == nil {
		return errors.New("model configuration not found")
	}

	if !model.Supports1MContext {
		return errors.New("model does not support extended context")
	}

	currentMode := selectedModel.ContextMode
	if currentMode == "" {
		currentMode = config.ContextModeStandard
	}

	modes := []struct {
		mode  config.ContextMode
		title string
	}{
		{config.ContextModeStandard, "Standard (200K)"},
		{config.ContextModeExtended, "Extended (1M)"},
		{config.ContextModeDynamic, "Dynamic (Auto)"},
	}

	items := make([]list.FilterableItem, 0, len(modes))
	selectedIndex := 0
	for i, m := range modes {
		item := &ContextModeItem{
			mode:      m.mode,
			title:     m.title,
			isCurrent: m.mode == currentMode,
			t:         c.com.Styles,
		}
		items = append(items, item)
		if m.mode == currentMode {
			selectedIndex = i
		}
	}

	c.list.SetItems(items...)
	c.list.SetSelected(selectedIndex)
	c.list.ScrollToSelected()
	return nil
}

// Filter returns the filter value for the context mode item.
func (c *ContextModeItem) Filter() string {
	return c.title
}

// ID returns the unique identifier for the context mode.
func (c *ContextModeItem) ID() string {
	return string(c.mode)
}

// SetFocused sets the focus state of the context mode item.
func (c *ContextModeItem) SetFocused(focused bool) {
	if c.focused != focused {
		c.cache = nil
	}
	c.focused = focused
}

// SetMatch sets the fuzzy match for the context mode item.
func (c *ContextModeItem) SetMatch(m fuzzy.Match) {
	c.cache = nil
	c.m = m
}

// Render returns the string representation of the context mode item.
func (c *ContextModeItem) Render(width int) string {
	info := ""
	if c.isCurrent {
		info = "current"
	}
	styles := ListItemStyles{
		ItemBlurred:     c.t.Dialog.NormalItem,
		ItemFocused:     c.t.Dialog.SelectedItem,
		InfoTextBlurred: c.t.Dialog.ListItem.InfoBlurred,
		InfoTextFocused: c.t.Dialog.ListItem.InfoFocused,
	}
	return renderItem(styles, c.title, info, c.focused, width, c.cache, &c.m)
}
