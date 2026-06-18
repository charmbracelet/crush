package dialog

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/sahilm/fuzzy"
)

const (
	// RevertPickerID is the identifier for the revert message picker dialog.
	RevertPickerID             = "revert_picker"
	revertPickerDialogMaxWidth = 70
)

// RevertPicker is a dialog that displays a scrollable list of user messages
// from the current session so the user can pick one to revert to.
type RevertPicker struct {
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

var _ Dialog = (*RevertPicker)(nil)

// revertPickerItem implements list.FilterableItem for a user message.
type revertPickerItem struct {
	*list.Versioned
	messageID      string
	messageContent string
	preview        string
	t              *styles.Styles
	m              fuzzy.Match
	cache          map[int]string
	focused        bool
}

func (r *revertPickerItem) Finished() bool { return true }

func (r *revertPickerItem) Filter() string { return r.preview }

func (r *revertPickerItem) Render(width int) string {
	itemStyles := ListItemStyles{
		ItemBlurred:     r.t.Dialog.NormalItem,
		ItemFocused:     r.t.Dialog.SelectedItem,
		InfoTextBlurred: r.t.Dialog.ListItem.InfoBlurred,
		InfoTextFocused: r.t.Dialog.ListItem.InfoFocused,
	}
	// Shared list renderer: themed selection highlight (matches the sessions
	// and commands dialogs — no raw cursor glyph), fuzzy-match underline, and
	// width-aware, rune-safe truncation.
	return renderItem(itemStyles, r.preview, "", r.focused, width, r.cache, &r.m)
}

func (r *revertPickerItem) SetFocused(focused bool) {
	if r.focused != focused {
		r.focused = focused
		r.cache = nil
		r.Bump()
	}
}

func (r *revertPickerItem) SetMatch(m fuzzy.Match) {
	if !sameFuzzyMatch(r.m, m) {
		r.cache = nil
		r.m = m
		r.Bump()
	}
}

// NewRevertPicker creates a new revert message picker dialog populated with
// the given user messages (most recent first).
func NewRevertPicker(com *common.Common, userMessages []message.Message) *RevertPicker {
	r := &RevertPicker{com: com}

	help := help.New()
	help.Styles = com.Styles.DialogHelpStyles()
	r.help = help

	r.list = list.NewFilterableList()
	r.list.Focus()

	r.input = textinput.New()
	r.input.SetVirtualCursor(false)
	r.input.Placeholder = "Type to filter"
	r.input.SetStyles(com.Styles.TextInput)
	r.input.Focus()

	r.keyMap.Select = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "confirm"),
	)
	r.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("↓", "next item"),
	)
	r.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑", "previous item"),
	)
	r.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑/↓", "choose"),
	)
	r.keyMap.Close = CloseKey

	// Items arrive newest-first (see openRevertPickerDialog). Show them in that
	// order — newest at the top — and default the cursor there: "undo my last
	// turn" is the common case, and it avoids pre-selecting the very first
	// message, which would revert almost the entire session.
	items := make([]list.FilterableItem, 0, len(userMessages))
	for _, msg := range userMessages {
		text := strings.TrimSpace(msg.Content().Text)
		if text == "" {
			continue
		}
		preview := strings.ReplaceAll(text, "\n", " ")
		items = append(items, &revertPickerItem{
			Versioned:      list.NewVersioned(),
			t:              com.Styles,
			messageID:      msg.ID,
			messageContent: text,
			preview:        preview,
		})
	}
	r.list.SetItems(items...)
	if len(items) > 0 {
		r.list.SelectFirst()
		r.list.ScrollToTop()
	}

	return r
}

// ID implements Dialog.
func (*RevertPicker) ID() string { return RevertPickerID }

// HandleMsg implements Dialog.
func (r *RevertPicker) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, r.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, r.keyMap.Previous):
			r.list.Focus()
			if r.list.IsSelectedFirst() {
				r.list.SelectLast()
				r.list.ScrollToBottom()
				break
			}
			r.list.SelectPrev()
			r.list.ScrollToSelected()
		case key.Matches(msg, r.keyMap.Next):
			r.list.Focus()
			if r.list.IsSelectedLast() {
				r.list.SelectFirst()
				r.list.ScrollToTop()
				break
			}
			r.list.SelectNext()
			r.list.ScrollToSelected()
		case key.Matches(msg, r.keyMap.Select):
			selectedItem := r.list.SelectedItem()
			if selectedItem == nil {
				break
			}
			item, ok := selectedItem.(*revertPickerItem)
			if !ok {
				break
			}
			return ActionSelectRevertMessage{
				MessageID:      item.messageID,
				MessageContent: item.messageContent,
			}
		default:
			var cmd tea.Cmd
			r.input, cmd = r.input.Update(msg)
			value := r.input.Value()
			r.list.SetFilter(value)
			r.list.ScrollToTop()
			r.list.SetSelected(0)
			return ActionCmd{cmd}
		}
	}
	return nil
}

// ShortHelp implements [help.KeyMap].
func (r *RevertPicker) ShortHelp() []key.Binding {
	return []key.Binding{
		r.keyMap.UpDown,
		r.keyMap.Select,
		r.keyMap.Close,
	}
}

// FullHelp implements [help.KeyMap].
func (r *RevertPicker) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{r.keyMap.Select, r.keyMap.Next, r.keyMap.Previous, r.keyMap.Close},
	}
}

// Cursor returns the cursor position relative to the dialog.
func (r *RevertPicker) Cursor() *tea.Cursor {
	return InputCursor(r.com.Styles, r.input.Cursor())
}

// Draw implements Dialog.
func (r *RevertPicker) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := r.com.Styles
	width := max(0, min(revertPickerDialogMaxWidth, area.Dx()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.InputPrompt.GetVerticalFrameSize() + inputContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()

	r.input.SetWidth(innerWidth - t.Dialog.InputPrompt.GetHorizontalFrameSize() - 1)
	// r.list.Len() is the already-filtered count; len(FilteredItems()) would
	// re-run the fuzzy search (and re-Bump every item) on every frame.
	visibleCount := r.list.Len()
	listHeight := max(3, min(visibleCount, area.Dy()-heightOffset))
	r.list.SetSize(innerWidth, listHeight)
	r.help.SetWidth(innerWidth)

	rc := NewRenderContext(t, width)
	rc.Title = "Revert to Message"
	inputView := t.Dialog.InputPrompt.Render(r.input.View())
	rc.AddPart(inputView)

	if r.list.Height() >= visibleCount {
		r.list.ScrollToTop()
	} else {
		r.list.ScrollToSelected()
	}

	listView := t.Dialog.List.Height(r.list.Height()).Render(r.list.Render())
	rc.AddPart(listView)
	rc.Help = r.help.View(r)

	view := rc.Render()
	cur := r.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}
