package dialog

import (
	"cmp"
	"slices"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/shell"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/sahilm/fuzzy"
)

const (
	// TerminalsID is the identifier for the running terminals dialog.
	TerminalsID              = "terminals"
	terminalsDialogMaxWidth  = 76
	terminalsDialogMaxHeight = 12
)

// TerminalEntry describes a running interactive terminal for the
// terminals dialog.
type TerminalEntry struct {
	Session *shell.InteractiveSession
	// Shown reports whether the terminal's panel is on screen.
	Shown bool
	// Chat names the chat the terminal belongs to, empty when unknown.
	Chat string
}

// Terminals lists the running interactive terminals so the user can
// switch to one or end it.
type Terminals struct {
	com   *common.Common
	help  help.Model
	list  *list.FilterableList
	input textinput.Model

	keyMap struct {
		Select   key.Binding
		Kill     key.Binding
		Next     key.Binding
		Previous key.Binding
		UpDown   key.Binding
		Close    key.Binding
	}
}

// TerminalItem is a running terminal in the terminals dialog.
type TerminalItem struct {
	*list.Versioned
	entry   TerminalEntry
	t       *styles.Styles
	m       fuzzy.Match
	cache   map[int]string
	focused bool
}

var (
	_ Dialog   = (*Terminals)(nil)
	_ ListItem = (*TerminalItem)(nil)
)

// NewTerminals creates the terminals dialog listing entries.
func NewTerminals(com *common.Common, entries []TerminalEntry) *Terminals {
	d := &Terminals{com: com}

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
		key.WithHelp("enter", "open"),
	)
	d.keyMap.Kill = key.NewBinding(
		key.WithKeys("ctrl+x"),
		key.WithHelp("ctrl+x", "kill"),
	)
	d.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("↓", "next item"),
	)
	d.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑", "previous item"),
	)
	d.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑/↓", "choose"),
	)
	d.keyMap.Close = CloseKey

	items := make([]list.FilterableItem, 0, len(entries))
	for _, entry := range entries {
		items = append(items, &TerminalItem{Versioned: list.NewVersioned(), entry: entry, t: com.Styles})
	}
	d.list.SetItems(items...)
	d.list.SetSelected(0)
	return d
}

// ID implements Dialog.
func (d *Terminals) ID() string {
	return TerminalsID
}

// HandleMsg implements [Dialog].
func (d *Terminals) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, d.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, d.keyMap.Previous):
			d.list.Focus()
			if d.list.IsSelectedFirst() {
				d.list.SelectLast()
				d.list.ScrollToBottom()
				break
			}
			d.list.SelectPrev()
			d.list.ScrollToSelected()
		case key.Matches(msg, d.keyMap.Next):
			d.list.Focus()
			if d.list.IsSelectedLast() {
				d.list.SelectFirst()
				d.list.ScrollToTop()
				break
			}
			d.list.SelectNext()
			d.list.ScrollToSelected()
		case key.Matches(msg, d.keyMap.Select):
			if item, ok := d.list.SelectedItem().(*TerminalItem); ok {
				return ActionOpenTerminal{Session: item.entry.Session}
			}
		case key.Matches(msg, d.keyMap.Kill):
			item, ok := d.list.SelectedItem().(*TerminalItem)
			if !ok {
				break
			}
			// The process ends asynchronously; drop it from the list now.
			d.list.SetItems(slices.DeleteFunc(d.list.Items(), func(other list.FilterableItem) bool {
				return other == item
			})...)
			d.list.SetFilter(d.input.Value())
			d.list.SetSelected(0)
			return ActionKillTerminal{Session: item.entry.Session}
		default:
			prevValue := d.input.Value()
			var cmd tea.Cmd
			d.input, cmd = d.input.Update(msg)
			if value := d.input.Value(); value != prevValue {
				d.list.SetFilter(value)
				d.list.ScrollToTop()
				d.list.SetSelected(0)
			}
			return ActionCmd{cmd}
		}
	}
	return nil
}

// Cursor returns the cursor position relative to the dialog.
func (d *Terminals) Cursor() *tea.Cursor {
	return InputCursor(d.com.Styles, d.input.Cursor())
}

// Draw implements [Dialog].
func (d *Terminals) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := d.com.Styles
	width := max(0, min(terminalsDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	height := max(0, min(terminalsDialogMaxHeight, area.Dy()-t.Dialog.View.GetVerticalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.InputPrompt.GetVerticalFrameSize() + inputContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()

	d.input.SetWidth(dialogInputTextWidth(t, d.input, innerWidth))
	d.list.SetSize(innerWidth, max(0, height-heightOffset))

	rc := NewRenderContext(t, width)
	rc.Title = "Terminals"
	rc.AddPart(t.Dialog.InputPrompt.Render(d.input.View()))

	if len(d.list.Items()) == 0 {
		rc.AddPart(t.Dialog.SecondaryText.Width(innerWidth).Padding(0, 1).Render("No running terminals."))
	} else {
		if d.list.Height() >= len(d.list.FilteredItems()) {
			d.list.ScrollToTop()
		} else {
			d.list.ScrollToSelected()
		}
		rc.AddPart(t.Dialog.List.Height(d.list.Height()).Render(d.list.Render()))
	}
	rc.Help = renderDialogHelp(t, &d.help, d, innerWidth)

	cur := d.Cursor()
	DrawCenterCursor(scr, area, rc.Render(), cur)
	return cur
}

// ShortHelp implements [help.KeyMap].
func (d *Terminals) ShortHelp() []key.Binding {
	return []key.Binding{d.keyMap.UpDown, d.keyMap.Select, d.keyMap.Kill, d.keyMap.Close}
}

// FullHelp implements [help.KeyMap].
func (d *Terminals) FullHelp() [][]key.Binding {
	return [][]key.Binding{{d.keyMap.Select, d.keyMap.Kill, d.keyMap.Next, d.keyMap.Previous}, {d.keyMap.Close}}
}

// title is the command the terminal runs, or "shell" for a bare shell.
func (i *TerminalItem) title() string {
	return cmp.Or(i.entry.Session.Command(), "shell")
}

// Filter implements [list.FilterableItem].
func (i *TerminalItem) Filter() string {
	return i.title()
}

// ID returns the interactive session ID.
func (i *TerminalItem) ID() string {
	return i.entry.Session.ID()
}

// SetFocused sets the focus state of the item.
func (i *TerminalItem) SetFocused(focused bool) {
	if i.focused == focused {
		return
	}
	i.cache = nil
	i.focused = focused
	i.Bump()
}

// SetMatch sets the fuzzy match for the item.
func (i *TerminalItem) SetMatch(m fuzzy.Match) {
	if sameFuzzyMatch(i.m, m) {
		return
	}
	i.cache = nil
	i.m = m
	i.Bump()
}

// Render returns the string representation of the item.
func (i *TerminalItem) Render(width int) string {
	info := []string{"hidden"}
	if i.entry.Shown {
		info[0] = "shown"
	}
	if i.entry.Chat != "" {
		info = append(info, i.entry.Chat)
	}
	if i.entry.Session.AgentDriven() {
		info = append(info, "agent drives")
	} else {
		info = append(info, "you drive")
	}
	st := ListItemStyles{
		ItemBlurred:     i.t.Dialog.NormalItem,
		ItemFocused:     i.t.Dialog.SelectedItem,
		InfoTextBlurred: i.t.Dialog.ListItem.InfoBlurred,
		InfoTextFocused: i.t.Dialog.ListItem.InfoFocused,
	}
	return renderItem(st, i.title(), strings.Join(info, " · "), i.focused, width, i.cache, &i.m)
}
