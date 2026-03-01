package dialog

import (
	"strconv"

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
	// MuseID is the identifier for the Muse settings dialog.
	MuseID              = "muse"
	museDialogMaxWidth  = 70
	museDialogMaxHeight = 18
	museDescription     = "Muse triggers background thinking during inactivity,\nmaking Crush a living assistant that proactively works."
)

// Muse represents a dialog for configuring Muse settings.
type Muse struct {
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

// MuseItem represents a Muse settings list item.
type MuseItem struct {
	id        string
	title     string
	info      string
	t         *styles.Styles
	m         fuzzy.Match
	cache     map[int]string
	focused   bool
}

var (
	_ Dialog   = (*Muse)(nil)
	_ ListItem = (*MuseItem)(nil)
)

// NewMuse creates a new Muse settings dialog.
func NewMuse(com *common.Common) (*Muse, error) {
	m := &Muse{com: com}

	help := help.New()
	help.Styles = com.Styles.DialogHelpStyles()
	m.help = help

	m.list = list.NewFilterableList()
	m.list.Focus()

	m.input = textinput.New()
	m.input.SetVirtualCursor(false)
	m.input.Placeholder = "Type to filter"
	m.input.SetStyles(com.Styles.TextInput)
	m.input.Focus()

	m.keyMap.Select = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "confirm"),
	)
	m.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("↓", "next item"),
	)
	m.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑", "previous item"),
	)
	m.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑/↓", "choose"),
	)
	m.keyMap.Close = CloseKey

	m.setMuseItems()

	return m, nil
}

// ID implements Dialog.
func (m *Muse) ID() string {
	return MuseID
}

// HandleMsg implements [Dialog].
func (m *Muse) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, m.keyMap.Previous):
			m.list.Focus()
			if m.list.IsSelectedFirst() {
				m.list.SelectLast()
				m.list.ScrollToBottom()
				break
			}
			m.list.SelectPrev()
			m.list.ScrollToSelected()
		case key.Matches(msg, m.keyMap.Next):
			m.list.Focus()
			if m.list.IsSelectedLast() {
				m.list.SelectFirst()
				m.list.ScrollToTop()
				break
			}
			m.list.SelectNext()
			m.list.ScrollToSelected()
		case key.Matches(msg, m.keyMap.Select):
			selectedItem := m.list.SelectedItem()
			if selectedItem == nil {
				break
			}
			museItem, ok := selectedItem.(*MuseItem)
			if !ok {
				break
			}
			// Handle special edit actions
			switch museItem.id {
			case "edit_prompt":
				return ActionEditMusePrompt{}
			case "edit_timeout":
				return ActionEditMuseTimeout{}
			case "edit_interval":
				return ActionEditMuseInterval{}
			}
			return nil
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			value := m.input.Value()
			m.list.SetFilter(value)
			m.list.ScrollToTop()
			m.list.SetSelected(0)
			return ActionCmd{cmd}
		}
	}
	return nil
}

// Cursor returns the cursor position relative to the dialog.
func (m *Muse) Cursor() *tea.Cursor {
	return InputCursor(m.com.Styles, m.input.Cursor())
}

// Draw implements [Dialog].
func (m *Muse) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := m.com.Styles
	width := max(0, min(museDialogMaxWidth, area.Dx()))
	height := max(0, min(museDialogMaxHeight, area.Dy()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.InputPrompt.GetVerticalFrameSize() + inputContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize() +
		2 // description lines

	m.input.SetWidth(innerWidth - t.Dialog.InputPrompt.GetHorizontalFrameSize() - 1)
	m.list.SetSize(innerWidth, height-heightOffset)
	m.help.SetWidth(innerWidth)

	rc := NewRenderContext(t, width)
	rc.Title = "Muse Mode Settings"
	inputView := t.Dialog.InputPrompt.Render(m.input.View())
	rc.AddPart(inputView)

	// Add description text
	descStyle := t.Base.PaddingLeft(1).Width(innerWidth)
	rc.AddPart(descStyle.Render(museDescription))

	visibleCount := len(m.list.FilteredItems())
	if m.list.Height() >= visibleCount {
		m.list.ScrollToTop()
	} else {
		m.list.ScrollToSelected()
	}

	listView := t.Dialog.List.Height(m.list.Height()).Render(m.list.Render())
	rc.AddPart(listView)
	rc.Help = m.help.View(m)

	view := rc.Render()

	cur := m.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

// ShortHelp implements [help.KeyMap].
func (m *Muse) ShortHelp() []key.Binding {
	return []key.Binding{
		m.keyMap.UpDown,
		m.keyMap.Select,
		m.keyMap.Close,
	}
}

// FullHelp implements [help.KeyMap].
func (m *Muse) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{m.keyMap.Select, m.keyMap.Next, m.keyMap.Previous, m.keyMap.Close},
	}
}

func (m *Muse) setMuseItems() {
	cfg := m.com.Config()

	// Get current values
	timeout := 120
	interval := 0
	if cfg.Options != nil {
		if cfg.Options.MuseTimeout > 0 {
			timeout = cfg.Options.MuseTimeout
		}
		if cfg.Options.MuseInterval > 0 {
			interval = cfg.Options.MuseInterval
		}
	}

	// Format interval display
	intervalDisplay := "once (no repeat)"
	if interval > 0 {
		intervalDisplay = formatDuration(interval)
	}

	items := []list.FilterableItem{
		&MuseItem{
			id:      "edit_timeout",
			title:   "Edit Timeout",
			info:    formatDuration(timeout),
			t:       m.com.Styles,
		},
		&MuseItem{
			id:      "edit_interval",
			title:   "Edit Interval",
			info:    intervalDisplay,
			t:       m.com.Styles,
		},
		&MuseItem{
			id:      "edit_prompt",
			title:   "Edit Prompt",
			info:    "open editor",
			t:       m.com.Styles,
		},
	}

	m.list.SetItems(items...)
	m.list.SetSelected(0)
	m.list.ScrollToTop()
}

// Filter returns the filter value for the Muse item.
func (m *MuseItem) Filter() string {
	return m.title
}

// ID returns the unique identifier for the Muse item.
func (m *MuseItem) ID() string {
	return m.id
}

// SetFocused sets the focus state of the Muse item.
func (m *MuseItem) SetFocused(focused bool) {
	if m.focused != focused {
		m.cache = nil
	}
	m.focused = focused
}

// SetMatch sets the fuzzy match for the Muse item.
func (m *MuseItem) SetMatch(match fuzzy.Match) {
	m.cache = nil
	m.m = match
}

// Render returns the string representation of the Muse item.
func (m *MuseItem) Render(width int) string {
	styles := ListItemStyles{
		ItemBlurred:     m.t.Dialog.NormalItem,
		ItemFocused:     m.t.Dialog.SelectedItem,
		InfoTextBlurred: m.t.Base,
		InfoTextFocused: m.t.Base,
	}
	return renderItem(styles, m.title, m.info, m.focused, width, m.cache, &m.m)
}

// UpdateMuseItems refreshes the Muse dialog items with current config.
func (m *Muse) UpdateMuseItems() {
	m.setMuseItems()
}

// formatDuration formats seconds into a human-readable string.
func formatDuration(seconds int) string {
	if seconds < 60 {
		return strconv.Itoa(seconds) + "s"
	} else if seconds < 3600 {
		mins := seconds / 60
		return strconv.Itoa(mins) + "m"
	}
	hours := seconds / 3600
	return strconv.Itoa(hours) + "h"
}
