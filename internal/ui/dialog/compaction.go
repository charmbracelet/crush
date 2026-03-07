package dialog

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/sahilm/fuzzy"
)

const (
	// CompactionID is the identifier for the compaction method dialog.
	CompactionID              = "compaction"
	compactionDialogMaxWidth  = 50
	compactionDialogMaxHeight = 10
)

// Compaction represents a dialog for selecting the compaction method.
type Compaction struct {
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

// CompactionItem represents a compaction method list item.
type CompactionItem struct {
	method    string
	title     string
	isCurrent bool
	t         *styles.Styles
	m         fuzzy.Match
	cache     map[int]string
	focused   bool
}

var (
	_ Dialog   = (*Compaction)(nil)
	_ ListItem = (*CompactionItem)(nil)
)

// NewCompaction creates a new compaction method dialog.
func NewCompaction(com *common.Common) *Compaction {
	c := &Compaction{com: com}

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

	c.setCompactionItems()

	return c
}

// ID implements Dialog.
func (c *Compaction) ID() string {
	return CompactionID
}

// HandleMsg implements [Dialog].
func (c *Compaction) HandleMsg(msg tea.Msg) Action {
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
			compactionItem, ok := selectedItem.(*CompactionItem)
			if !ok {
				break
			}
			return ActionSelectCompactionMethod{Method: compactionItem.method}
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
func (c *Compaction) Cursor() *tea.Cursor {
	return InputCursor(c.com.Styles, c.input.Cursor())
}

// Draw implements [Dialog].
func (c *Compaction) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := c.com.Styles
	width := max(0, min(compactionDialogMaxWidth, area.Dx()))
	height := max(0, min(compactionDialogMaxHeight, area.Dy()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.InputPrompt.GetVerticalFrameSize() + inputContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()

	c.input.SetWidth(innerWidth - t.Dialog.InputPrompt.GetHorizontalFrameSize() - 1)
	c.list.SetSize(innerWidth, height-heightOffset)
	c.help.SetWidth(innerWidth)

	rc := NewRenderContext(t, width)
	rc.Title = "Select Compaction Method"
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
func (c *Compaction) ShortHelp() []key.Binding {
	return []key.Binding{
		c.keyMap.UpDown,
		c.keyMap.Select,
		c.keyMap.Close,
	}
}

// FullHelp implements [help.KeyMap].
func (c *Compaction) FullHelp() [][]key.Binding {
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

type compactionOption struct {
	method config.CompactionMethod
	title  string
}

var compactionOptions = []compactionOption{
	{config.CompactionAuto, "Auto-compaction"},
	{config.CompactionLLM, "LLM/User-driven compaction"},
}

func (c *Compaction) setCompactionItems() {
	cfg := c.com.Config()
	current := cfg.Options.CompactionMethod
	if current == "" {
		current = config.CompactionAuto
	}

	items := make([]list.FilterableItem, 0, len(compactionOptions))
	selectedIndex := 0
	for i, opt := range compactionOptions {
		item := &CompactionItem{
			method:    string(opt.method),
			title:     opt.title,
			isCurrent: opt.method == current,
			t:         c.com.Styles,
		}
		items = append(items, item)
		if opt.method == current {
			selectedIndex = i
		}
	}

	c.list.SetItems(items...)
	c.list.SetSelected(selectedIndex)
	c.list.ScrollToSelected()
}

// Filter returns the filter value for the compaction item.
func (ci *CompactionItem) Filter() string {
	return ci.title
}

// ID returns the unique identifier for the compaction method.
func (ci *CompactionItem) ID() string {
	return ci.method
}

// SetFocused sets the focus state of the compaction item.
func (ci *CompactionItem) SetFocused(focused bool) {
	if ci.focused != focused {
		ci.cache = nil
	}
	ci.focused = focused
}

// SetMatch sets the fuzzy match for the compaction item.
func (ci *CompactionItem) SetMatch(m fuzzy.Match) {
	ci.cache = nil
	ci.m = m
}

// Render returns the string representation of the compaction item.
func (ci *CompactionItem) Render(width int) string {
	info := ""
	if ci.isCurrent {
		info = "current"
	}
	s := ListItemStyles{
		ItemBlurred:     ci.t.Dialog.NormalItem,
		ItemFocused:     ci.t.Dialog.SelectedItem,
		InfoTextBlurred: ci.t.Base,
		InfoTextFocused: ci.t.Base,
	}
	return renderItem(s, ci.title, info, ci.focused, width, ci.cache, &ci.m)
}
