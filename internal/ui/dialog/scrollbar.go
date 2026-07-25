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
	// ScrollbarID is the identifier for the scrollbar style picker dialog.
	ScrollbarID              = "scrollbar"
	scrollbarDialogMaxWidth  = 50
	scrollbarDialogMaxHeight = 10
)

// ScrollbarStyle represents a scrollbar visibility option.
type ScrollbarStyle struct {
	ID          string
	Title       string
	Description string
}

// AllScrollbarStyles lists all available scrollbar styles in order.
var AllScrollbarStyles = []ScrollbarStyle{
	{ID: config.ScrollbarDefault, Title: "Default", Description: "Auto-hide after 2 seconds"},
	{ID: config.ScrollbarAlways, Title: "Always", Description: "Always show when content exceeds viewport"},
	{ID: config.ScrollbarNever, Title: "Never", Description: "Never show scrollbar"},
}

// Scrollbar represents a dialog for selecting scrollbar visibility.
type Scrollbar struct {
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

// ScrollbarItem represents a scrollbar style list item.
type ScrollbarItem struct {
	*list.Versioned
	style     ScrollbarStyle
	isCurrent bool
	t         *styles.Styles
	m         fuzzy.Match
	cache     map[int]string
	focused   bool
}

// Finished implements list.Item. Scrollbar items are render-stable
// outside of explicit SetFocused / SetMatch.
func (s *ScrollbarItem) Finished() bool {
	return true
}

var (
	_ Dialog   = (*Scrollbar)(nil)
	_ ListItem = (*ScrollbarItem)(nil)
)

// NewScrollbar creates a new scrollbar style picker dialog.
func NewScrollbar(com *common.Common) *Scrollbar {
	s := &Scrollbar{com: com}

	h := help.New()
	h.Styles = com.Styles.DialogHelpStyles()
	s.help = h

	s.list = list.NewFilterableList()
	s.list.Focus()

	s.input = textinput.New()
	s.input.SetVirtualCursor(false)
	s.input.Placeholder = "Type to filter"
	s.input.SetStyles(com.Styles.TextInput)
	s.input.Focus()

	s.keyMap.Select = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "confirm"),
	)
	s.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("↓", "next item"),
	)
	s.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑", "previous item"),
	)
	s.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑/↓", "choose"),
	)
	s.keyMap.Close = CloseKey

	s.setItems()
	return s
}

// ID implements Dialog.
func (s *Scrollbar) ID() string {
	return ScrollbarID
}

// HandleMsg implements [Dialog].
func (s *Scrollbar) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, s.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, s.keyMap.Previous):
			s.list.Focus()
			if s.list.IsSelectedFirst() {
				s.list.SelectLast()
				s.list.ScrollToBottom()
				break
			}
			s.list.SelectPrev()
			s.list.ScrollToSelected()
		case key.Matches(msg, s.keyMap.Next):
			s.list.Focus()
			if s.list.IsSelectedLast() {
				s.list.SelectFirst()
				s.list.ScrollToTop()
				break
			}
			s.list.SelectNext()
			s.list.ScrollToSelected()
		case key.Matches(msg, s.keyMap.Select):
			selectedItem := s.list.SelectedItem()
			if selectedItem == nil {
				break
			}
			scrollbarItem, ok := selectedItem.(*ScrollbarItem)
			if !ok {
				break
			}
			return ActionSelectScrollbarStyle{Style: scrollbarItem.style.ID}
		default:
			var cmd tea.Cmd
			s.input, cmd = s.input.Update(msg)
			value := s.input.Value()
			s.list.SetFilter(value)
			s.list.ScrollToTop()
			s.list.SetSelected(0)
			return ActionCmd{cmd}
		}
	}
	return nil
}

// Cursor returns the cursor position relative to the dialog.
func (s *Scrollbar) Cursor() *tea.Cursor {
	return InputCursor(s.com.Styles, s.input.Cursor())
}

// Draw implements [Dialog].
func (s *Scrollbar) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := s.com.Styles
	width := max(0, min(scrollbarDialogMaxWidth, area.Dx()))
	height := max(0, min(scrollbarDialogMaxHeight, area.Dy()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.InputPrompt.GetVerticalFrameSize() + inputContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()

	s.input.SetWidth(innerWidth - t.Dialog.InputPrompt.GetHorizontalFrameSize() - 1)
	s.list.SetSize(innerWidth, height-heightOffset)
	s.help.SetWidth(innerWidth)

	rc := NewRenderContext(t, width)
	rc.Title = "Scrollbar"
	inputView := t.Dialog.InputPrompt.Render(s.input.View())
	rc.AddPart(inputView)

	visibleCount := len(s.list.FilteredItems())
	if s.list.Height() >= visibleCount {
		s.list.ScrollToTop()
	} else {
		s.list.ScrollToSelected()
	}

	listView := t.Dialog.List.Height(s.list.Height()).Render(s.list.Render())
	rc.AddPart(listView)
	rc.Help = s.help.View(s)

	view := rc.Render()

	cur := s.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

// ShortHelp implements [help.KeyMap].
func (s *Scrollbar) ShortHelp() []key.Binding {
	return []key.Binding{
		s.keyMap.UpDown,
		s.keyMap.Select,
		s.keyMap.Close,
	}
}

// FullHelp implements [help.KeyMap].
func (s *Scrollbar) FullHelp() [][]key.Binding {
	m := [][]key.Binding{}
	slice := []key.Binding{
		s.keyMap.Select,
		s.keyMap.Next,
		s.keyMap.Previous,
		s.keyMap.Close,
	}
	for i := 0; i < len(slice); i += 4 {
		end := min(i+4, len(slice))
		m = append(m, slice[i:end])
	}
	return m
}

func (s *Scrollbar) setItems() {
	cfg := s.com.Config()
	currentStyle := config.ScrollbarDefault
	if cfg != nil && cfg.Options != nil && cfg.Options.TUI != nil && cfg.Options.TUI.Scrollbar != "" {
		currentStyle = cfg.Options.TUI.Scrollbar
	}

	items := make([]list.FilterableItem, 0, len(AllScrollbarStyles))
	selectedIndex := 0
	for i, style := range AllScrollbarStyles {
		item := &ScrollbarItem{
			Versioned: list.NewVersioned(),
			style:     style,
			isCurrent: style.ID == currentStyle,
			t:         s.com.Styles,
		}
		items = append(items, item)
		if style.ID == currentStyle {
			selectedIndex = i
		}
	}

	s.list.SetItems(items...)
	s.list.SetSelected(selectedIndex)
	s.list.ScrollToSelected()
}

// Filter returns the filter value for the scrollbar item.
func (s *ScrollbarItem) Filter() string {
	return s.style.Title
}

// ID returns the unique identifier for the scrollbar style.
func (s *ScrollbarItem) ID() string {
	return s.style.ID
}

// SetFocused sets the focus state of the scrollbar item.
func (s *ScrollbarItem) SetFocused(focused bool) {
	if s.focused == focused {
		return
	}
	s.cache = nil
	s.focused = focused
	if s.Versioned != nil {
		s.Bump()
	}
}

// SetMatch sets the fuzzy match for the scrollbar item.
func (s *ScrollbarItem) SetMatch(m fuzzy.Match) {
	if sameFuzzyMatch(s.m, m) {
		return
	}
	s.cache = nil
	s.m = m
	if s.Versioned != nil {
		s.Bump()
	}
}

// Render returns the string representation of the scrollbar item.
func (s *ScrollbarItem) Render(width int) string {
	info := ""
	if s.isCurrent {
		info = "current"
	}
	st := ListItemStyles{
		ItemBlurred:     s.t.Dialog.NormalItem,
		ItemFocused:     s.t.Dialog.SelectedItem,
		InfoTextBlurred: s.t.Dialog.ListItem.InfoBlurred,
		InfoTextFocused: s.t.Dialog.ListItem.InfoFocused,
	}
	return renderItem(st, s.style.Title, info, s.focused, width, s.cache, &s.m)
}
