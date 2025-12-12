package list

import (
	"regexp"
	"slices"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/tui/components/core/layout"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/sahilm/fuzzy"
)

// Pre-compiled regex for checking if a string is alphanumeric.
var alphanumericRegex = regexp.MustCompile(`^[a-zA-Z0-9]*$`)

type FilterableItem interface {
	Item
	FilterValue() string
}

type FilterableList[T FilterableItem] interface {
	List[T]
	Cursor() *tea.Cursor
	SetInputWidth(int)
	SetInputPlaceholder(string)
	SetResultsSize(int)
	Filter(q string) tea.Cmd
	fuzzy.Source
}

type HasMatchIndexes interface {
	MatchIndexes([]int)
}

type filterableOptions struct {
	listOptions []ListOption
	placeholder string
	inputHidden bool
	inputWidth  int
	inputStyle  lipgloss.Style
}
type filterableList[T FilterableItem] struct {
	*list[T]
	*filterableOptions
	width, height int
	// stores all available items
	items       []T
	resultsSize int
	input       textinput.Model
	inputWidth  int
	query       string
}

type filterableListOption func(*filterableOptions)

func WithFilterPlaceholder(ph string) filterableListOption {
	return func(f *filterableOptions) {
		f.placeholder = ph
	}
}

func WithFilterInputHidden() filterableListOption {
	return func(f *filterableOptions) {
		f.inputHidden = true
	}
}

func WithFilterInputStyle(inputStyle lipgloss.Style) filterableListOption {
	return func(f *filterableOptions) {
		f.inputStyle = inputStyle
	}
}

func WithFilterListOptions(opts ...ListOption) filterableListOption {
	return func(f *filterableOptions) {
		f.listOptions = opts
	}
}

func WithFilterInputWidth(inputWidth int) filterableListOption {
	return func(f *filterableOptions) {
		f.inputWidth = inputWidth
	}
}

func NewFilterableList[T FilterableItem](items []T, opts ...filterableListOption) FilterableList[T] {
	t := styles.CurrentTheme()

	f := &filterableList[T]{
		filterableOptions: &filterableOptions{
			inputStyle:  t.S().Base,
			placeholder: "Type to filter",
		},
	}
	for _, opt := range opts {
		opt(f.filterableOptions)
	}
	f.list = New(items, f.listOptions...).(*list[T])

	f.updateKeyMaps()
	f.items = f.list.items

	if f.inputHidden {
		return f
	}

	ti := textinput.New()
	ti.Placeholder = f.placeholder
	ti.SetVirtualCursor(false)
	ti.Focus()
	ti.SetStyles(t.S().TextInput)
	f.input = ti
	return f
}

func (f *filterableList[T]) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		// handle movements
		case IsMovementKey(msg, f.keyMap):
			u, cmd := f.list.Update(msg)
			f.list = u.(*list[T])
			return f, cmd
		default:
			if !f.inputHidden {
				var cmds []tea.Cmd
				var cmd tea.Cmd
				f.input, cmd = f.input.Update(msg)
				cmds = append(cmds, cmd)

				if f.query != f.input.Value() {
					cmd = f.Filter(f.input.Value())
					cmds = append(cmds, cmd)
				}
				f.query = f.input.Value()
				return f, tea.Batch(cmds...)
			}
		}
	}
	u, cmd := f.list.Update(msg)
	f.list = u.(*list[T])
	return f, cmd
}

func (f *filterableList[T]) View() string {
	if f.inputHidden {
		return f.list.View()
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		f.inputStyle.Render(f.input.View()),
		f.list.View(),
	)
}

// removes bindings that are used for search
func (f *filterableList[T]) updateKeyMaps() {
	helper := NewKeyBindingHelper(alphanumericRegex)

	f.keyMap.Down = helper.UpdateBinding(f.keyMap.Down)
	f.keyMap.Up = helper.UpdateBinding(f.keyMap.Up)
	f.keyMap.DownOneItem = helper.UpdateBinding(f.keyMap.DownOneItem)
	f.keyMap.UpOneItem = helper.UpdateBinding(f.keyMap.UpOneItem)
	f.keyMap.HalfPageDown = helper.UpdateBinding(f.keyMap.HalfPageDown)
	f.keyMap.HalfPageUp = helper.UpdateBinding(f.keyMap.HalfPageUp)
	f.keyMap.PageDown = helper.UpdateBinding(f.keyMap.PageDown)
	f.keyMap.PageUp = helper.UpdateBinding(f.keyMap.PageUp)
	f.keyMap.End = helper.UpdateBinding(f.keyMap.End)
	f.keyMap.Home = helper.UpdateBinding(f.keyMap.Home)
}

func (m *filterableList[T]) GetSize() (int, int) {
	return m.width, m.height
}

func (f *filterableList[T]) SetSize(w, h int) tea.Cmd {
	f.width = w
	f.height = h
	if f.inputHidden {
		return f.list.SetSize(w, h)
	}
	if f.inputWidth == 0 {
		f.input.SetWidth(w)
	} else {
		f.input.SetWidth(f.inputWidth)
	}
	return f.list.SetSize(w, h-(f.inputHeight()))
}

func (f *filterableList[T]) inputHeight() int {
	return lipgloss.Height(f.inputStyle.Render(f.input.View()))
}

func (f *filterableList[T]) Filter(query string) tea.Cmd {
	var cmds []tea.Cmd
	for _, item := range f.items {
		if i, ok := any(item).(layout.Focusable); ok {
			cmds = append(cmds, i.Blur())
		}
		if i, ok := any(item).(HasMatchIndexes); ok {
			i.MatchIndexes(make([]int, 0))
		}
	}

	f.selectedItemIdx = -1
	if query == "" || len(f.items) == 0 {
		return f.list.SetItems(f.visibleItems(f.items))
	}

	matches := fuzzy.FindFrom(query, f)

	var matchedItems []T
	resultSize := len(matches)
	if f.resultsSize > 0 && resultSize > f.resultsSize {
		resultSize = f.resultsSize
	}
	for i := range resultSize {
		match := matches[i]
		item := f.items[match.Index]
		if it, ok := any(item).(HasMatchIndexes); ok {
			it.MatchIndexes(match.MatchedIndexes)
		}
		matchedItems = append(matchedItems, item)
	}

	if f.direction == DirectionBackward {
		slices.Reverse(matchedItems)
	}

	cmds = append(cmds, f.list.SetItems(matchedItems))
	return tea.Batch(cmds...)
}

func (f *filterableList[T]) SetItems(items []T) tea.Cmd {
	f.items = items
	return f.list.SetItems(f.visibleItems(items))
}

func (f *filterableList[T]) Cursor() *tea.Cursor {
	if f.inputHidden {
		return nil
	}
	return f.input.Cursor()
}

func (f *filterableList[T]) Blur() tea.Cmd {
	f.input.Blur()
	return f.list.Blur()
}

func (f *filterableList[T]) Focus() tea.Cmd {
	f.input.Focus()
	return f.list.Focus()
}

func (f *filterableList[T]) IsFocused() bool {
	return f.list.IsFocused()
}

func (f *filterableList[T]) SetInputWidth(w int) {
	f.inputWidth = w
}

func (f *filterableList[T]) SetInputPlaceholder(ph string) {
	f.placeholder = ph
}

func (f *filterableList[T]) SetResultsSize(size int) {
	f.resultsSize = size
}

func (f *filterableList[T]) String(i int) string {
	return f.items[i].FilterValue()
}

func (f *filterableList[T]) Len() int {
	return len(f.items)
}

// visibleItems returns the subset of items that should be rendered based on
// the configured resultsSize limit. The underlying source (f.items) remains
// intact so filtering still searches the full set.
func (f *filterableList[T]) visibleItems(items []T) []T {
	if f.resultsSize > 0 && len(items) > f.resultsSize {
		return items[:f.resultsSize]
	}
	return items
}
