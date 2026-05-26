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
	// NotificationsID is the identifier for the notification style picker dialog.
	NotificationsID              = "notifications"
	notificationsDialogMaxWidth  = 50
	notificationsDialogMaxHeight = 12
)

// NotificationStyle represents a notification backend option.
type NotificationStyle struct {
	ID          string
	Title       string
	Description string
}

// AllNotificationStyles lists all available notification styles in order.
var AllNotificationStyles = []NotificationStyle{
	{ID: "auto", Title: "Auto", Description: "Automatically detect the best backend"},
	{ID: "native", Title: "Native", Description: "Use system notifications (macOS/Linux/Windows)"},
	{ID: "osc", Title: "OSC", Description: "Use terminal OSC escape sequences"},
	{ID: "bell", Title: "Bell", Description: "Use terminal bell character"},
	{ID: "disabled", Title: "Disabled", Description: "Turn off notifications"},
}

// Notifications represents a dialog for selecting notification style.
type Notifications struct {
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

// NotificationItem represents a notification style list item.
type NotificationItem struct {
	*list.Versioned
	style     NotificationStyle
	isCurrent bool
	t         *styles.Styles
	m         fuzzy.Match
	cache     map[int]string
	focused   bool
}

// Finished implements list.Item. Notification items are render-stable
// outside of explicit SetFocused / SetMatch.
func (n *NotificationItem) Finished() bool {
	return true
}

var (
	_ Dialog   = (*Notifications)(nil)
	_ ListItem = (*NotificationItem)(nil)
)

// NewNotifications creates a new notification style picker dialog.
func NewNotifications(com *common.Common) *Notifications {
	n := &Notifications{com: com}

	h := help.New()
	h.Styles = com.Styles.DialogHelpStyles()
	n.help = h

	n.list = list.NewFilterableList()
	n.list.Focus()

	n.input = textinput.New()
	n.input.SetVirtualCursor(false)
	n.input.Placeholder = "Type to filter"
	n.input.SetStyles(com.Styles.TextInput)
	n.input.Focus()

	n.keyMap.Select = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "confirm"),
	)
	n.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("↓", "next item"),
	)
	n.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑", "previous item"),
	)
	n.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑/↓", "choose"),
	)
	n.keyMap.Close = CloseKey

	n.setItems()
	return n
}

// ID implements Dialog.
func (n *Notifications) ID() string {
	return NotificationsID
}

// HandleMsg implements [Dialog].
func (n *Notifications) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, n.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, n.keyMap.Previous):
			n.list.Focus()
			if n.list.IsSelectedFirst() {
				n.list.SelectLast()
				n.list.ScrollToBottom()
				break
			}
			n.list.SelectPrev()
			n.list.ScrollToSelected()
		case key.Matches(msg, n.keyMap.Next):
			n.list.Focus()
			if n.list.IsSelectedLast() {
				n.list.SelectFirst()
				n.list.ScrollToTop()
				break
			}
			n.list.SelectNext()
			n.list.ScrollToSelected()
		case key.Matches(msg, n.keyMap.Select):
			selectedItem := n.list.SelectedItem()
			if selectedItem == nil {
				break
			}
			notifItem, ok := selectedItem.(*NotificationItem)
			if !ok {
				break
			}
			return ActionSelectNotificationStyle{Style: notifItem.style.ID}
		default:
			var cmd tea.Cmd
			n.input, cmd = n.input.Update(msg)
			value := n.input.Value()
			n.list.SetFilter(value)
			n.list.ScrollToTop()
			n.list.SetSelected(0)
			return ActionCmd{cmd}
		}
	}
	return nil
}

// Cursor returns the cursor position relative to the dialog.
func (n *Notifications) Cursor() *tea.Cursor {
	return InputCursor(n.com.Styles, n.input.Cursor())
}

// Draw implements [Dialog].
func (n *Notifications) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := n.com.Styles
	width := max(0, min(notificationsDialogMaxWidth, area.Dx()))
	height := max(0, min(notificationsDialogMaxHeight, area.Dy()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.InputPrompt.GetVerticalFrameSize() + inputContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()

	n.input.SetWidth(innerWidth - t.Dialog.InputPrompt.GetHorizontalFrameSize() - 1)
	n.list.SetSize(innerWidth, height-heightOffset)
	n.help.SetWidth(innerWidth)

	rc := NewRenderContext(t, width)
	rc.Title = "Notification Style"
	inputView := t.Dialog.InputPrompt.Render(n.input.View())
	rc.AddPart(inputView)

	visibleCount := len(n.list.FilteredItems())
	if n.list.Height() >= visibleCount {
		n.list.ScrollToTop()
	} else {
		n.list.ScrollToSelected()
	}

	listView := t.Dialog.List.Height(n.list.Height()).Render(n.list.Render())
	rc.AddPart(listView)
	rc.Help = n.help.View(n)

	view := rc.Render()

	cur := n.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

// ShortHelp implements [help.KeyMap].
func (n *Notifications) ShortHelp() []key.Binding {
	return []key.Binding{
		n.keyMap.UpDown,
		n.keyMap.Select,
		n.keyMap.Close,
	}
}

// FullHelp implements [help.KeyMap].
func (n *Notifications) FullHelp() [][]key.Binding {
	m := [][]key.Binding{}
	slice := []key.Binding{
		n.keyMap.Select,
		n.keyMap.Next,
		n.keyMap.Previous,
		n.keyMap.Close,
	}
	for i := 0; i < len(slice); i += 4 {
		end := min(i+4, len(slice))
		m = append(m, slice[i:end])
	}
	return m
}

func (n *Notifications) setItems() {
	cfg := n.com.Config()
	currentStyle := "auto"
	if cfg != nil && cfg.Options != nil && cfg.Options.NotificationStyle != "" {
		currentStyle = cfg.Options.NotificationStyle
	}

	items := make([]list.FilterableItem, 0, len(AllNotificationStyles))
	selectedIndex := 0
	for i, style := range AllNotificationStyles {
		item := &NotificationItem{
			Versioned: list.NewVersioned(),
			style:     style,
			isCurrent: style.ID == currentStyle,
			t:         n.com.Styles,
		}
		items = append(items, item)
		if style.ID == currentStyle {
			selectedIndex = i
		}
	}

	n.list.SetItems(items...)
	n.list.SetSelected(selectedIndex)
	n.list.ScrollToSelected()
}

// Filter returns the filter value for the notification item.
func (n *NotificationItem) Filter() string {
	return n.style.Title
}

// ID returns the unique identifier for the notification style.
func (n *NotificationItem) ID() string {
	return n.style.ID
}

// SetFocused sets the focus state of the notification item.
func (n *NotificationItem) SetFocused(focused bool) {
	if n.focused == focused {
		return
	}
	n.cache = nil
	n.focused = focused
	if n.Versioned != nil {
		n.Bump()
	}
}

// SetMatch sets the fuzzy match for the notification item.
func (n *NotificationItem) SetMatch(m fuzzy.Match) {
	if sameFuzzyMatch(n.m, m) {
		return
	}
	n.cache = nil
	n.m = m
	if n.Versioned != nil {
		n.Bump()
	}
}

// Render returns the string representation of the notification item.
func (n *NotificationItem) Render(width int) string {
	info := ""
	if n.isCurrent {
		info = "current"
	}
	st := ListItemStyles{
		ItemBlurred:     n.t.Dialog.NormalItem,
		ItemFocused:     n.t.Dialog.SelectedItem,
		InfoTextBlurred: n.t.Dialog.ListItem.InfoBlurred,
		InfoTextFocused: n.t.Dialog.ListItem.InfoFocused,
	}
	return renderItem(st, n.style.Title, info, n.focused, width, n.cache, &n.m)
}
