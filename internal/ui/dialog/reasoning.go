package dialog

import (
	"errors"

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
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const (
	// ReasoningID is the identifier for the reasoning effort dialog.
	ReasoningID              = "reasoning"
	reasoningDialogMaxWidth  = 80
	reasoningDialogMaxHeight = 12
)

// Reasoning represents a dialog for selecting reasoning effort.
type Reasoning struct {
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

// ReasoningItem represents a reasoning effort list item.
type ReasoningItem struct {
	effort    string
	title     string
	isCurrent bool
	t         *styles.Styles
	m         fuzzy.Match
	cache     map[int]string
	focused   bool
}

var (
	_ Dialog   = (*Reasoning)(nil)
	_ ListItem = (*ReasoningItem)(nil)
)

// NewReasoning creates a new reasoning effort dialog.
func NewReasoning(com *common.Common) (*Reasoning, error) {
	r := &Reasoning{com: com}

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

	if err := r.setReasoningItems(); err != nil {
		return nil, err
	}

	return r, nil
}

// ID implements Dialog.
func (r *Reasoning) ID() string {
	return ReasoningID
}

// HandleMsg implements [Dialog].
func (r *Reasoning) HandleMsg(msg tea.Msg) Action {
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
			reasoningItem, ok := selectedItem.(*ReasoningItem)
			if !ok {
				break
			}
			return ActionSelectReasoningEffort{Effort: reasoningItem.effort}
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

// Cursor returns the cursor position relative to the dialog.
func (r *Reasoning) Cursor() *tea.Cursor {
	return InputCursor(r.com.Styles, r.input.Cursor())
}

// Draw implements [Dialog].
func (r *Reasoning) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := r.com.Styles
	width := max(0, min(reasoningDialogMaxWidth, area.Dx()))
	height := max(0, min(reasoningDialogMaxHeight, area.Dy()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.InputPrompt.GetVerticalFrameSize() + inputContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()

	r.input.SetWidth(innerWidth - t.Dialog.InputPrompt.GetHorizontalFrameSize() - 1)
	r.list.SetSize(innerWidth, height-heightOffset)
	r.help.SetWidth(innerWidth)

	rc := NewRenderContext(t, width)
	rc.Title = "Select Reasoning Effort"
	inputView := t.Dialog.InputPrompt.Render(r.input.View())
	rc.AddPart(inputView)

	visibleCount := len(r.list.FilteredItems())
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

// ShortHelp implements [help.KeyMap].
func (r *Reasoning) ShortHelp() []key.Binding {
	return []key.Binding{
		r.keyMap.UpDown,
		r.keyMap.Select,
		r.keyMap.Close,
	}
}

// FullHelp implements [help.KeyMap].
func (r *Reasoning) FullHelp() [][]key.Binding {
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

func (r *Reasoning) setReasoningItems() error {
	cfg := r.com.Config()
	agentCfg, ok := cfg.Agents[config.AgentCoder]
	if !ok {
		return errors.New("agent configuration not found")
	}

	selectedModel := cfg.Models[agentCfg.Model]
	model := cfg.GetModelByType(agentCfg.Model)
	if model == nil {
		return errors.New("model configuration not found")
	}

	if len(model.ReasoningLevels) == 0 {
		return errors.New("no reasoning levels available")
	}

	currentEffort := selectedModel.ReasoningEffort
	if currentEffort == "" {
		currentEffort = model.DefaultReasoningEffort
	}

	caser := cases.Title(language.English)
	items := make([]list.FilterableItem, 0, len(model.ReasoningLevels))
	selectedIndex := 0
	for i, effort := range model.ReasoningLevels {
		item := &ReasoningItem{
			effort:    effort,
			title:     caser.String(effort),
			isCurrent: effort == currentEffort,
			t:         r.com.Styles,
		}
		items = append(items, item)
		if effort == currentEffort {
			selectedIndex = i
		}
	}

	r.list.SetItems(items...)
	r.list.SetSelected(selectedIndex)
	r.list.ScrollToSelected()
	return nil
}

// Filter returns the filter value for the reasoning item.
func (r *ReasoningItem) Filter() string {
	return r.title
}

// ID returns the unique identifier for the reasoning effort.
func (r *ReasoningItem) ID() string {
	return r.effort
}

// SetFocused sets the focus state of the reasoning item.
func (r *ReasoningItem) SetFocused(focused bool) {
	if r.focused != focused {
		r.cache = nil
	}
	r.focused = focused
}

// SetMatch sets the fuzzy match for the reasoning item.
func (r *ReasoningItem) SetMatch(m fuzzy.Match) {
	r.cache = nil
	r.m = m
}

// Render returns the string representation of the reasoning item.
func (r *ReasoningItem) Render(width int) string {
	info := ""
	if r.isCurrent {
		info = "current"
	}
	return renderItem(r.t, r.title, info, r.focused, width, r.cache, &r.m)
}
