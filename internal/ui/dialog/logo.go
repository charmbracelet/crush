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
	// LogoID is the identifier for the session logo style picker dialog.
	LogoID              = "logo"
	logoDialogMaxWidth  = 50
	logoDialogMaxHeight = 12
)

// LogoOption represents a session logo style option.
type LogoOption struct {
	ID          string
	Title       string
	Description string
}

// AllLogoStyles lists all available session logo styles in order.
var AllLogoStyles = []LogoOption{
	{ID: string(config.LogoStyleWordmark), Title: "Wordmark", Description: "Show the Crush wordmark (default)"},
	{ID: string(config.LogoStyleGradient), Title: "Gradient", Description: "Text-free gradient field, no wordmark"},
	{ID: string(config.LogoStyleHidden), Title: "Hidden", Description: "Hide the logo entirely"},
}

// Logo represents a dialog for selecting the session logo style.
type Logo struct {
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

// LogoItem represents a logo style list item.
type LogoItem struct {
	*list.Versioned
	opt       LogoOption
	isCurrent bool
	t         *styles.Styles
	m         fuzzy.Match
	cache     map[int]string
	focused   bool
}

// Finished implements list.Item. Logo items are render-stable outside of
// explicit SetFocused / SetMatch.
func (l *LogoItem) Finished() bool {
	return true
}

var (
	_ Dialog   = (*Logo)(nil)
	_ ListItem = (*LogoItem)(nil)
)

// NewLogo creates a new session logo style picker dialog.
func NewLogo(com *common.Common) *Logo {
	l := &Logo{com: com}

	h := help.New()
	h.Styles = com.Styles.DialogHelpStyles()
	l.help = h

	l.list = list.NewFilterableList()
	l.list.Focus()

	l.input = textinput.New()
	l.input.SetVirtualCursor(false)
	l.input.Placeholder = "Type to filter"
	l.input.SetStyles(com.Styles.TextInput)
	l.input.Focus()

	l.keyMap.Select = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "confirm"),
	)
	l.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("↓", "next item"),
	)
	l.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑", "previous item"),
	)
	l.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑/↓", "choose"),
	)
	l.keyMap.Close = CloseKey

	l.setItems()
	return l
}

// ID implements Dialog.
func (l *Logo) ID() string {
	return LogoID
}

// HandleMsg implements [Dialog].
func (l *Logo) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, l.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, l.keyMap.Previous):
			l.list.Focus()
			if l.list.IsSelectedFirst() {
				l.list.SelectLast()
				l.list.ScrollToBottom()
				break
			}
			l.list.SelectPrev()
			l.list.ScrollToSelected()
		case key.Matches(msg, l.keyMap.Next):
			l.list.Focus()
			if l.list.IsSelectedLast() {
				l.list.SelectFirst()
				l.list.ScrollToTop()
				break
			}
			l.list.SelectNext()
			l.list.ScrollToSelected()
		case key.Matches(msg, l.keyMap.Select):
			selectedItem := l.list.SelectedItem()
			if selectedItem == nil {
				break
			}
			logoItem, ok := selectedItem.(*LogoItem)
			if !ok {
				break
			}
			return ActionSelectLogoStyle{Style: logoItem.opt.ID}
		default:
			var cmd tea.Cmd
			l.input, cmd = l.input.Update(msg)
			value := l.input.Value()
			l.list.SetFilter(value)
			l.list.ScrollToTop()
			l.list.SetSelected(0)
			return ActionCmd{cmd}
		}
	}
	return nil
}

// Cursor returns the cursor position relative to the dialog.
func (l *Logo) Cursor() *tea.Cursor {
	return InputCursor(l.com.Styles, l.input.Cursor())
}

// Draw implements [Dialog].
func (l *Logo) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := l.com.Styles
	width := max(0, min(logoDialogMaxWidth, area.Dx()))
	height := max(0, min(logoDialogMaxHeight, area.Dy()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.InputPrompt.GetVerticalFrameSize() + inputContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()

	l.input.SetWidth(innerWidth - t.Dialog.InputPrompt.GetHorizontalFrameSize() - 1)
	l.list.SetSize(innerWidth, height-heightOffset)
	l.help.SetWidth(innerWidth)

	rc := NewRenderContext(t, width)
	rc.Title = "Session Logo"
	inputView := t.Dialog.InputPrompt.Render(l.input.View())
	rc.AddPart(inputView)

	visibleCount := len(l.list.FilteredItems())
	if l.list.Height() >= visibleCount {
		l.list.ScrollToTop()
	} else {
		l.list.ScrollToSelected()
	}

	listView := t.Dialog.List.Height(l.list.Height()).Render(l.list.Render())
	rc.AddPart(listView)
	rc.Help = l.help.View(l)

	view := rc.Render()

	cur := l.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

// ShortHelp implements [help.KeyMap].
func (l *Logo) ShortHelp() []key.Binding {
	return []key.Binding{
		l.keyMap.UpDown,
		l.keyMap.Select,
		l.keyMap.Close,
	}
}

// FullHelp implements [help.KeyMap].
func (l *Logo) FullHelp() [][]key.Binding {
	m := [][]key.Binding{}
	slice := []key.Binding{
		l.keyMap.Select,
		l.keyMap.Next,
		l.keyMap.Previous,
		l.keyMap.Close,
	}
	for i := 0; i < len(slice); i += 4 {
		end := min(i+4, len(slice))
		m = append(m, slice[i:end])
	}
	return m
}

func (l *Logo) setItems() {
	cfg := l.com.Config()
	currentStyle := string(config.LogoStyleWordmark)
	if cfg != nil && cfg.Options != nil && cfg.Options.TUI != nil && cfg.Options.TUI.Logo != "" {
		currentStyle = string(cfg.Options.TUI.Logo)
	}

	items := make([]list.FilterableItem, 0, len(AllLogoStyles))
	selectedIndex := 0
	for i, opt := range AllLogoStyles {
		item := &LogoItem{
			Versioned: list.NewVersioned(),
			opt:       opt,
			isCurrent: opt.ID == currentStyle,
			t:         l.com.Styles,
		}
		items = append(items, item)
		if opt.ID == currentStyle {
			selectedIndex = i
		}
	}

	l.list.SetItems(items...)
	l.list.SetSelected(selectedIndex)
	l.list.ScrollToSelected()
}

// Filter returns the filter value for the logo item.
func (l *LogoItem) Filter() string {
	return l.opt.Title
}

// ID returns the unique identifier for the logo style.
func (l *LogoItem) ID() string {
	return l.opt.ID
}

// SetFocused sets the focus state of the logo item.
func (l *LogoItem) SetFocused(focused bool) {
	if l.focused == focused {
		return
	}
	l.cache = nil
	l.focused = focused
	if l.Versioned != nil {
		l.Bump()
	}
}

// SetMatch sets the fuzzy match for the logo item.
func (l *LogoItem) SetMatch(m fuzzy.Match) {
	if sameFuzzyMatch(l.m, m) {
		return
	}
	l.cache = nil
	l.m = m
	if l.Versioned != nil {
		l.Bump()
	}
}

// Render returns the string representation of the logo item.
func (l *LogoItem) Render(width int) string {
	info := ""
	if l.isCurrent {
		info = "current"
	}
	st := ListItemStyles{
		ItemBlurred:     l.t.Dialog.NormalItem,
		ItemFocused:     l.t.Dialog.SelectedItem,
		InfoTextBlurred: l.t.Dialog.ListItem.InfoBlurred,
		InfoTextFocused: l.t.Dialog.ListItem.InfoFocused,
	}
	return renderItem(st, l.opt.Title, info, l.focused, width, l.cache, &l.m)
}
