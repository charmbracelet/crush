package dialog

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/notification"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
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
	pickerRow
	style     NotificationStyle
	isCurrent bool
	t         *styles.Styles
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
	n.keyMap.UpDown = upDownKey("choose")
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
			prevValue := n.input.Value()
			var cmd tea.Cmd
			n.input, cmd = n.input.Update(msg)
			value := n.input.Value()
			if value != prevValue {
				n.list.SetFilter(value)
				n.list.ScrollToTop()
				n.list.SetSelected(0)
			}

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
	f := newPickerFrame(t, area, notificationsDialogMaxWidth, notificationsDialogMaxHeight, &n.input, &n.help, n)
	// Reserve the tallest description so moving between options doesn't resize the dialog.
	descHeight := n.descriptionHeight(f.innerWidth)
	f.fitHeight(area, notificationsDialogMaxHeight+descHeight)
	f.Title = "Notification Style"
	f.addInput()

	var descView string
	bodyHeight := f.height - f.ChromeHeight() - t.Dialog.List.GetVerticalFrameSize()
	if descHeight > 0 {
		descView = n.descriptionStyle(f.innerWidth).Height(descHeight).MarginBottom(1).Render(n.selectedDescription())
		bodyHeight -= lipgloss.Height(descView)
	}
	// Size the list to its options so no blank rows pile up before the help line.
	n.list.SetSize(f.innerWidth, min(max(0, bodyHeight), len(n.list.Items())))

	visibleCount := len(n.list.FilteredItems())
	if n.list.Height() >= visibleCount {
		n.list.ScrollToTop()
	} else {
		n.list.ScrollToSelected()
	}
	f.AddPart(t.Dialog.List.Height(n.list.Height()).Render(n.list.Render()))
	f.AddPart(descView)
	view := f.Render()

	cur := n.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

// descriptionStyle is the subtle, wrapped style of the highlighted
// option's description, inset to line up with the list items.
func (n *Notifications) descriptionStyle(width int) lipgloss.Style {
	return n.com.Styles.Dialog.ListItem.InfoBlurred.Padding(0, 1).Width(max(0, width))
}

// descriptionHeight returns the rows of the tallest option
// description wrapped at width.
func (n *Notifications) descriptionHeight(width int) int {
	st := n.descriptionStyle(width)
	h := 0
	for _, style := range AllNotificationStyles {
		h = max(h, lipgloss.Height(st.Render(style.Description)))
	}
	return h
}

// selectedDescription returns the description of the highlighted
// option, or "" when nothing matches the filter.
func (n *Notifications) selectedDescription() string {
	if item, ok := n.list.SelectedItem().(*NotificationItem); ok {
		return item.style.Description
	}
	return ""
}

// ShortHelp implements [help.KeyMap].
func (n *Notifications) ShortHelp() []key.Binding {
	return dialogHelp(n.keyMap.Select, n.keyMap.Close, n.keyMap.UpDown)
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
	if cfg != nil && cfg.Options != nil && cfg.Options.Notifications != "" {
		currentStyle = cfg.Options.Notifications
	}

	items := make([]list.FilterableItem, 0, len(AllNotificationStyles))
	selectedIndex := 0
	for _, style := range AllNotificationStyles {
		// Native OS notifications don't build on every platform
		// (illumos/solaris); hide the option where it can't work.
		if style.ID == "native" && !notification.NativeSupported {
			continue
		}
		item := &NotificationItem{
			pickerRow: newPickerRow(),
			style:     style,
			isCurrent: style.ID == currentStyle,
			t:         n.com.Styles,
		}
		if style.ID == currentStyle {
			selectedIndex = len(items)
		}
		items = append(items, item)
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

// Render returns the string representation of the notification item.
func (n *NotificationItem) Render(width int) string {
	info := ""
	if n.isCurrent {
		info = "current"
	}
	return n.render(defaultListItemStyles(n.t), n.style.Title, info, width)
}
