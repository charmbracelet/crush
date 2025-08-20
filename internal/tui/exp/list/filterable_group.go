package list

import (
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/textinput"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/tui/components/core/layout"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/sahilm/fuzzy"
)

// Pre-compiled regex for checking if a string is alphanumeric.
// Note: This is duplicated from filterable.go to avoid circular dependencies.
var alphanumericRegexGroup = regexp.MustCompile(`^[a-zA-Z0-9]*$`)

type FilterableGroupList[T FilterableItem] interface {
	GroupedList[T]
	Cursor() *tea.Cursor
	SetInputWidth(int)
	SetInputPlaceholder(string)
}
type filterableGroupList[T FilterableItem] struct {
	*groupedList[T]
	*filterableOptions
	width, height int
	groups        []Group[T]
	// stores all available items
	input      textinput.Model
	inputWidth int
	query      string
}

func NewFilterableGroupedList[T FilterableItem](items []Group[T], opts ...filterableListOption) FilterableGroupList[T] {
	t := styles.CurrentTheme()

	f := &filterableGroupList[T]{
		filterableOptions: &filterableOptions{
			inputStyle:  t.S().Base,
			placeholder: "Type to filter",
		},
	}
	for _, opt := range opts {
		opt(f.filterableOptions)
	}
	f.groupedList = NewGroupedList(items, f.listOptions...).(*groupedList[T])

	f.updateKeyMaps()

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

func (f *filterableGroupList[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		// handle movements
		case key.Matches(msg, f.keyMap.Down),
			key.Matches(msg, f.keyMap.Up),
			key.Matches(msg, f.keyMap.DownOneItem),
			key.Matches(msg, f.keyMap.UpOneItem),
			key.Matches(msg, f.keyMap.HalfPageDown),
			key.Matches(msg, f.keyMap.HalfPageUp),
			key.Matches(msg, f.keyMap.PageDown),
			key.Matches(msg, f.keyMap.PageUp),
			key.Matches(msg, f.keyMap.End),
			key.Matches(msg, f.keyMap.Home):
			u, cmd := f.groupedList.Update(msg)
			f.groupedList = u.(*groupedList[T])
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
	u, cmd := f.groupedList.Update(msg)
	f.groupedList = u.(*groupedList[T])
	return f, cmd
}

func (f *filterableGroupList[T]) View() string {
	if f.inputHidden {
		return f.groupedList.View()
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		f.inputStyle.Render(f.input.View()),
		f.groupedList.View(),
	)
}

// removes bindings that are used for search
func (f *filterableGroupList[T]) updateKeyMaps() {
	removeLettersAndNumbers := func(bindings []string) []string {
		var keep []string
		for _, b := range bindings {
			if len(b) != 1 {
				keep = append(keep, b)
				continue
			}
			if b == " " {
				continue
			}
			m := alphanumericRegexGroup.MatchString(b)
			if !m {
				keep = append(keep, b)
			}
		}
		return keep
	}

	updateBinding := func(binding key.Binding) key.Binding {
		newKeys := removeLettersAndNumbers(binding.Keys())
		if len(newKeys) == 0 {
			binding.SetEnabled(false)
			return binding
		}
		binding.SetKeys(newKeys...)
		return binding
	}

	f.keyMap.Down = updateBinding(f.keyMap.Down)
	f.keyMap.Up = updateBinding(f.keyMap.Up)
	f.keyMap.DownOneItem = updateBinding(f.keyMap.DownOneItem)
	f.keyMap.UpOneItem = updateBinding(f.keyMap.UpOneItem)
	f.keyMap.HalfPageDown = updateBinding(f.keyMap.HalfPageDown)
	f.keyMap.HalfPageUp = updateBinding(f.keyMap.HalfPageUp)
	f.keyMap.PageDown = updateBinding(f.keyMap.PageDown)
	f.keyMap.PageUp = updateBinding(f.keyMap.PageUp)
	f.keyMap.End = updateBinding(f.keyMap.End)
	f.keyMap.Home = updateBinding(f.keyMap.Home)
}

func (m *filterableGroupList[T]) GetSize() (int, int) {
	return m.width, m.height
}

func (f *filterableGroupList[T]) SetSize(w, h int) tea.Cmd {
	f.width = w
	f.height = h
	if f.inputHidden {
		return f.groupedList.SetSize(w, h)
	}
	if f.inputWidth == 0 {
		f.input.SetWidth(w)
	} else {
		f.input.SetWidth(f.inputWidth)
	}
	return f.groupedList.SetSize(w, h-(f.inputHeight()))
}

func (f *filterableGroupList[T]) inputHeight() int {
	return lipgloss.Height(f.inputStyle.Render(f.input.View()))
}

func (f *filterableGroupList[T]) Filter(query string) tea.Cmd {
	var cmds []tea.Cmd
	for _, item := range slices.Collect(f.items.Seq()) {
		if i, ok := any(item).(layout.Focusable); ok {
			cmds = append(cmds, i.Blur())
		}
		if i, ok := any(item).(HasMatchIndexes); ok {
			i.MatchIndexes(make([]int, 0))
		}
	}

	f.selectedItem = ""
	if query == "" {
		return f.groupedList.SetGroups(f.groups)
	}

	var newGroups []Group[T]
	for _, g := range f.groups {
		words := make([]string, len(g.Items))
		for i, item := range g.Items {
			words[i] = strings.ToLower(item.FilterValue())
		}

		matches := fuzzy.Find(query, words)

		sort.SliceStable(matches, func(i, j int) bool {
			return matches[i].Score > matches[j].Score
		})

		var matchedItems []T
		for _, match := range matches {
			item := g.Items[match.Index]
			if i, ok := any(item).(HasMatchIndexes); ok {
				i.MatchIndexes(match.MatchedIndexes)
			}
			matchedItems = append(matchedItems, item)
		}
		if len(matchedItems) > 0 {
			newGroups = append(newGroups, Group[T]{
				Section: g.Section,
				Items:   matchedItems,
			})
		}
	}
	cmds = append(cmds, f.groupedList.SetGroups(newGroups))
	return tea.Batch(cmds...)
}

func (f *filterableGroupList[T]) SetGroups(groups []Group[T]) tea.Cmd {
	f.groups = groups
	return f.groupedList.SetGroups(groups)
}

func (f *filterableGroupList[T]) Cursor() *tea.Cursor {
	if f.inputHidden {
		return nil
	}
	return f.input.Cursor()
}

func (f *filterableGroupList[T]) Blur() tea.Cmd {
	f.input.Blur()
	return f.groupedList.Blur()
}

func (f *filterableGroupList[T]) Focus() tea.Cmd {
	f.input.Focus()
	return f.groupedList.Focus()
}

func (f *filterableGroupList[T]) IsFocused() bool {
	return f.groupedList.IsFocused()
}

func (f *filterableGroupList[T]) SetInputWidth(w int) {
	f.inputWidth = w
}

func (f *filterableGroupList[T]) SetInputPlaceholder(ph string) {
	f.input.Placeholder = ph
	f.placeholder = ph
}
