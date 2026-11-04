package dialog

import (
	"context"
	"image"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/util"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

const (
	// PlansID is the identifier for the saved plans dialog.
	PlansID              = "plans"
	plansDialogMaxWidth  = 70
	plansDialogMinHeight = 8
	plansDialogMaxHeight = 16
)

// Plans represents a dialog for browsing saved plans.
type Plans struct {
	bodyArea image.Rectangle
	loading  bool
	com      *common.Common
	help     help.Model
	list     *list.FilterableList
	input    textinput.Model

	keyMap struct {
		Select   key.Binding
		Next     key.Binding
		Previous key.Binding
		UpDown   key.Binding
		Close    key.Binding
	}
}

// NewPlans creates a new saved plans dialog.
func NewPlans(com *common.Common) *Plans {
	r := &Plans{com: com}

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
		key.WithHelp("enter", "open"),
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

	r.loading = true
	return r
}

// ID implements Dialog.
func (r *Plans) ID() string {
	return PlansID
}

// HandleMsg implements [Dialog].
func (r *Plans) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case plansLoadedMsg:
		if msg.owner != r {
			return nil
		}
		r.loading = false
		items := make([]list.FilterableItem, 0, len(msg.files))
		for _, path := range msg.files {
			items = append(items, NewCommandItem(r.com.Styles, path, path, "", nil))
		}
		r.list.SetItems(items...)
		r.list.SetFilter(r.input.Value())
		r.list.SetSelected(0)
		if msg.err != nil {
			return ActionCmd{util.ReportError(msg.err)}
		}
	case tea.MouseClickMsg:
		if msg.Button != tea.MouseLeft || !image.Pt(msg.X, msg.Y).In(r.bodyArea) {
			return nil
		}
		index, _ := r.list.ItemIndexAtPosition(msg.X-r.bodyArea.Min.X, msg.Y-r.bodyArea.Min.Y)
		if index < 0 {
			return nil
		}
		r.list.SetSelected(index)
		return r.openSelected()
	case common.CoalescedWheelMsg:
		if image.Pt(msg.Mouse.X, msg.Mouse.Y).In(r.bodyArea) {
			r.list.ScrollBy(int(msg.DeltaY))
		}
	case tea.MouseWheelMsg:
		if image.Pt(msg.X, msg.Y).In(r.bodyArea) {
			if msg.Button == tea.MouseWheelUp {
				r.list.ScrollBy(-3)
			} else if msg.Button == tea.MouseWheelDown {
				r.list.ScrollBy(3)
			}
		}
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
			return r.openSelected()
		default:
			prevValue := r.input.Value()
			var cmd tea.Cmd
			r.input, cmd = r.input.Update(msg)
			value := r.input.Value()
			if value != prevValue {
				r.list.SetFilter(value)
				r.list.ScrollToTop()
				r.list.SetSelected(0)
			}
			return ActionCmd{cmd}
		}
	}
	return nil
}

// Cursor returns the cursor position relative to the dialog.
func (r *Plans) Cursor() *tea.Cursor {
	return InputCursor(r.com.Styles, r.input.Cursor())
}

// Draw implements [Dialog].
func (r *Plans) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := r.com.Styles
	width := max(0, min(plansDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	innerWidth := max(0, width-t.Dialog.View.GetHorizontalFrameSize())

	r.input.SetWidth(dialogInputTextWidth(t, r.input, innerWidth))

	// Size the dialog to fit the list content, clamped to min/max bounds.
	listTotalHeight := r.list.TotalHeight()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.InputPrompt.GetVerticalFrameSize() + inputContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()
	desiredHeight := heightOffset + listTotalHeight
	maxAvailable := area.Dy() - t.Dialog.View.GetVerticalBorderSize()
	height := max(0, min(max(plansDialogMinHeight, desiredHeight), plansDialogMaxHeight, maxAvailable))

	listHeight, listTotalHeight, _ := sizeDialogList(t, r.list, innerWidth, height)

	rc := NewRenderContext(t, width)
	rc.Title = "My plans"
	inputView := t.Dialog.InputPrompt.Render(r.input.View())
	rc.AddPart(inputView)

	listView := t.Dialog.List.Height(r.list.Height()).Render(r.list.Render())
	listView = joinScrollbar(t, listView, listHeight, listTotalHeight, listHeight, r.list.Offset())
	if len(r.list.FilteredItems()) == 0 && listHeight > 0 {
		label := "No plans found"
		if r.loading {
			label = "Loading plans…"
		}
		listView = t.Dialog.List.Width(innerWidth).Height(listHeight).Render(ansi.Truncate(label, innerWidth, "…"))
	}
	rc.AddPart(listView)
	rc.Help = renderDialogHelp(t, &r.help, r, innerWidth)

	view := rc.Render()
	viewWidth, viewHeight := lipgloss.Size(view)
	bounds := common.CenterRect(area, min(viewWidth, area.Dx()), min(viewHeight, area.Dy()))
	viewStyle, bodyStyle := t.Dialog.View, t.Dialog.List
	x := bounds.Min.X + viewStyle.GetMarginLeft() + viewStyle.GetBorderLeftSize() + viewStyle.GetPaddingLeft() + bodyStyle.GetMarginLeft() + bodyStyle.GetBorderLeftSize() + bodyStyle.GetPaddingLeft()
	y := bounds.Max.Y - viewStyle.GetMarginBottom() - viewStyle.GetBorderBottomSize() - viewStyle.GetPaddingBottom() - lipgloss.Height(rc.Help) - lipgloss.Height(listView) + bodyStyle.GetMarginTop() + bodyStyle.GetBorderTopSize() + bodyStyle.GetPaddingTop()
	r.bodyArea = image.Rect(x, y, x+r.list.Width(), y+listHeight).Intersect(bounds).Intersect(area)

	cur := r.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

// ShortHelp implements [help.KeyMap].
func (r *Plans) ShortHelp() []key.Binding {
	return []key.Binding{
		r.keyMap.UpDown,
		r.keyMap.Select,
		r.keyMap.Close,
	}
}

// FullHelp implements [help.KeyMap].
func (r *Plans) FullHelp() [][]key.Binding {
	m := [][]key.Binding{}
	slice := []key.Binding{
		r.keyMap.Select,
		r.keyMap.Next,
		r.keyMap.Previous,
		r.keyMap.Close,
	}
	for i := 0; i < len(slice); i += 4 {
		end := min(i+4, len(slice))
		m = append(m, slice[i:end])
	}
	return m
}

type plansAvailabilityMsg struct {
	owner *Commands
	files []string
	err   error
}

type plansLoadedMsg struct {
	owner *Plans
	files []string
	err   error
}

// InitialCmd reads the current project plans outside the update loop.
func (r *Plans) InitialCmd() tea.Cmd {
	ws := r.com.Workspace
	return func() tea.Msg {
		files, err := ws.ListPlans(context.Background())
		return plansLoadedMsg{owner: r, files: files, err: err}
	}
}

// PlanReadMsg delivers plan content for an existing list dialog.
type PlanReadMsg struct {
	Owner   *Plans
	Name    string
	Content string
	Err     error
}

func (r *Plans) openSelected() Action {
	item, ok := r.list.SelectedItem().(*CommandItem)
	if !ok {
		return nil
	}
	name := item.ID()
	ws := r.com.Workspace
	return ActionCmd{Cmd: func() tea.Msg {
		content, err := ws.ReadPlan(context.Background(), name)
		return PlanReadMsg{Owner: r, Name: name, Content: content, Err: err}
	}}
}
