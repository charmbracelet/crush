package list

import (
	"fmt"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/tui/components/core/layout"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestListPosition(t *testing.T) {
	t.Parallel()
	type positionOffsetTest struct {
		dir      direction
		test     string
		width    int
		height   int
		numItems int

		moveUp   int
		moveDown int

		expectedStart int
		expectedEnd   int
	}
	tests := []positionOffsetTest{
		{
			dir:           Forward,
			test:          "should have correct position initially when forward",
			moveUp:        0,
			moveDown:      0,
			width:         10,
			height:        20,
			numItems:      100,
			expectedStart: 0,
			expectedEnd:   19,
		},
		{
			dir:           Forward,
			test:          "should offset start and end by one when moving down by one",
			moveUp:        0,
			moveDown:      1,
			width:         10,
			height:        20,
			numItems:      100,
			expectedStart: 1,
			expectedEnd:   20,
		},
		{
			dir:           Backward,
			test:          "should have correct position initially when backward",
			moveUp:        0,
			moveDown:      0,
			width:         10,
			height:        20,
			numItems:      100,
			expectedStart: 80,
			expectedEnd:   99,
		},
		{
			dir:           Backward,
			test:          "should offset the start and end by one when moving up by one",
			moveUp:        1,
			moveDown:      0,
			width:         10,
			height:        20,
			numItems:      100,
			expectedStart: 79,
			expectedEnd:   98,
		},
	}
	for _, c := range tests {
		t.Run(c.test, func(t *testing.T) {
			t.Parallel()
			items := []Item{}
			for i := range c.numItems {
				item := NewSelectableItem(fmt.Sprintf("Item %d", i))
				items = append(items, item)
			}
			l := New(items, WithDirection(c.dir)).(*list[Item])
			l.SetSize(c.width, c.height)
			cmd := l.Init()
			if cmd != nil {
				cmd()
			}

			if c.moveUp > 0 {
				l.MoveUp(c.moveUp)
			}
			if c.moveDown > 0 {
				l.MoveDown(c.moveDown)
			}
			start, end := l.viewPosition()
			assert.Equal(t, c.expectedStart, start)
			assert.Equal(t, c.expectedEnd, end)
		})
	}
}

func TestBackwardList(t *testing.T) {
	t.Parallel()
	t.Run("within height", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 5 {
			item := NewSelectableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := New(items, WithDirection(Backward), WithGap(1)).(*list[Item])
		l.SetSize(10, 20)
		cmd := l.Init()
		if cmd != nil {
			cmd()
		}

		// should select the last item
		assert.Equal(t, l.selectedItem, items[len(items)-1].ID())
		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("should not change selected item", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 5 {
			item := NewSelectableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := New(items, WithDirection(Backward), WithGap(1), WithSelectedItem(items[2].ID())).(*list[Item])
		l.SetSize(10, 20)
		cmd := l.Init()
		if cmd != nil {
			cmd()
		}
		// should select the last item
		assert.Equal(t, l.selectedItem, items[2].ID())
	})
	t.Run("more than height", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 10 {
			item := NewSelectableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := New(items, WithDirection(Backward))
		l.SetSize(10, 5)
		cmd := l.Init()
		if cmd != nil {
			cmd()
		}

		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("more than height multi line", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 10 {
			item := NewSelectableItem(fmt.Sprintf("Item %d\nLine2", i))
			items = append(items, item)
		}
		l := New(items, WithDirection(Backward))
		l.SetSize(10, 5)
		cmd := l.Init()
		if cmd != nil {
			cmd()
		}

		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("should move up", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 10 {
			item := NewSelectableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := New(items, WithDirection(Backward))
		l.SetSize(10, 5)
		cmd := l.Init()
		if cmd != nil {
			cmd()
		}

		l.MoveUp(1)
		golden.RequireEqual(t, []byte(l.View()))
	})

	t.Run("should move at max to the top", func(t *testing.T) {
		items := []Item{}
		for i := range 10 {
			item := NewSelectableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := New(items, WithDirection(Backward)).(*list[Item])
		l.SetSize(10, 5)
		cmd := l.Init()
		if cmd != nil {
			cmd()
		}

		l.MoveUp(100)
		assert.Equal(t, l.offset, lipgloss.Height(l.rendered)-l.listHeight())
		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("should do nothing with wrong move number", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 10 {
			item := NewSelectableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := New(items, WithDirection(Backward))
		l.SetSize(10, 5)
		cmd := l.Init()
		if cmd != nil {
			cmd()
		}

		l.MoveUp(-10)
		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("should move to the top", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 10 {
			item := NewSelectableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := New(items, WithDirection(Backward)).(*list[Item])
		l.SetSize(10, 5)
		cmd := l.Init()
		if cmd != nil {
			cmd()
		}

		l.GoToTop()
		assert.Equal(t, l.direction, Forward)
		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("should select the item above", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 10 {
			item := NewSelectableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := New(items, WithDirection(Backward)).(*list[Item])
		l.SetSize(10, 5)
		cmd := l.Init()
		if cmd != nil {
			cmd()
		}

		selectedInx := len(l.items) - 2
		currentItem := items[len(l.items)-1]
		nextItem := items[selectedInx]
		assert.False(t, nextItem.(SelectableItem).IsFocused())
		assert.True(t, currentItem.(SelectableItem).IsFocused())
		cmd = l.SelectItemAbove()
		if cmd != nil {
			cmd()
		}

		assert.Equal(t, l.selectedItem, l.items[selectedInx].ID())
		assert.True(t, l.items[selectedInx].(SelectableItem).IsFocused())

		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("should move the view to be able to see the selected item", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 10 {
			item := NewSelectableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := New(items, WithDirection(Backward)).(*list[Item])
		l.SetSize(10, 5)
		cmd := l.Init()
		if cmd != nil {
			cmd()
		}

		for range 5 {
			cmd = l.SelectItemAbove()
			if cmd != nil {
				cmd()
			}
		}
		golden.RequireEqual(t, []byte(l.View()))
	})
}

func TestForwardList(t *testing.T) {
	t.Parallel()
	t.Run("within height", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 5 {
			item := NewSelectableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := New(items, WithDirection(Forward), WithGap(1)).(*list[Item])
		l.SetSize(10, 20)
		cmd := l.Init()
		if cmd != nil {
			cmd()
		}

		// should select the last item
		assert.Equal(t, l.selectedItem, items[0].ID())

		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("should not change selected item", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 5 {
			item := NewSelectableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := New(items, WithDirection(Forward), WithGap(1), WithSelectedItem(items[2].ID())).(*list[Item])
		l.SetSize(10, 20)
		cmd := l.Init()
		if cmd != nil {
			cmd()
		}
		// should select the last item
		assert.Equal(t, l.selectedItem, items[2].ID())
	})
	t.Run("more than height", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 10 {
			item := NewSelectableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := New(items, WithDirection(Forward)).(*list[Item])
		l.SetSize(10, 5)
		cmd := l.Init()
		if cmd != nil {
			cmd()
		}

		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("more than height multi line", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 10 {
			item := NewSelectableItem(fmt.Sprintf("Item %d\nLine2", i))
			items = append(items, item)
		}
		l := New(items, WithDirection(Forward)).(*list[Item])
		l.SetSize(10, 5)
		cmd := l.Init()
		if cmd != nil {
			cmd()
		}

		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("should move down", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 10 {
			item := NewSelectableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := New(items, WithDirection(Forward)).(*list[Item])
		l.SetSize(10, 5)
		cmd := l.Init()
		if cmd != nil {
			cmd()
		}

		l.MoveDown(1)
		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("should move at max to the bottom", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 10 {
			item := NewSelectableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := New(items, WithDirection(Forward)).(*list[Item])
		l.SetSize(10, 5)
		cmd := l.Init()
		if cmd != nil {
			cmd()
		}

		l.MoveDown(100)
		assert.Equal(t, l.offset, lipgloss.Height(l.rendered)-l.listHeight())
		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("should do nothing with wrong move number", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 10 {
			item := NewSelectableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := New(items, WithDirection(Forward)).(*list[Item])
		l.SetSize(10, 5)
		cmd := l.Init()
		if cmd != nil {
			cmd()
		}

		l.MoveDown(-10)
		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("should move to the bottom", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 10 {
			item := NewSelectableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := New(items, WithDirection(Forward)).(*list[Item])
		l.SetSize(10, 5)
		cmd := l.Init()
		if cmd != nil {
			cmd()
		}

		l.GoToBottom()
		assert.Equal(t, l.direction, Backward)
		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("should select the item below", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 10 {
			item := NewSelectableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := New(items, WithDirection(Forward)).(*list[Item])
		l.SetSize(10, 5)
		cmd := l.Init()
		if cmd != nil {
			cmd()
		}

		selectedInx := 1
		currentItem := items[0]
		nextItem := items[selectedInx]
		assert.False(t, nextItem.(SelectableItem).IsFocused())
		assert.True(t, currentItem.(SelectableItem).IsFocused())
		cmd = l.SelectItemBelow()
		if cmd != nil {
			cmd()
		}

		assert.Equal(t, l.selectedItem, l.items[selectedInx].ID())
		assert.True(t, l.items[selectedInx].(SelectableItem).IsFocused())

		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("should move the view to be able to see the selected item", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 10 {
			item := NewSelectableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := New(items, WithDirection(Forward)).(*list[Item])
		l.SetSize(10, 5)
		cmd := l.Init()
		if cmd != nil {
			cmd()
		}

		for range 5 {
			cmd = l.SelectItemBelow()
			if cmd != nil {
				cmd()
			}
		}
		golden.RequireEqual(t, []byte(l.View()))
	})
}

func TestListSelection(t *testing.T) {
	t.Parallel()
	t.Run("should skip none selectable items initially", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		items = append(items, NewSimpleItem("None Selectable"))
		for i := range 5 {
			item := NewSelectableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := New(items, WithDirection(Forward)).(*list[Item])
		l.SetSize(100, 10)
		cmd := l.Init()
		if cmd != nil {
			cmd()
		}

		assert.Equal(t, items[1].ID(), l.selectedItem)
		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("should select the correct item on startup", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 5 {
			item := NewSelectableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := New(items, WithDirection(Forward)).(*list[Item])
		cmd := l.Init()
		otherCmd := l.SetSelected(items[3].ID())
		var wg sync.WaitGroup
		if cmd != nil {
			wg.Add(1)
			go func() {
				cmd()
				wg.Done()
			}()
		}
		if otherCmd != nil {
			wg.Add(1)
			go func() {
				otherCmd()
				wg.Done()
			}()
		}
		wg.Wait()
		l.SetSize(100, 10)
		assert.Equal(t, items[3].ID(), l.selectedItem)
		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("should skip none selectable items in the middle", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		item := NewSelectableItem("Item initial")
		items = append(items, item)
		items = append(items, NewSimpleItem("None Selectable"))
		for i := range 5 {
			item := NewSelectableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := New(items, WithDirection(Forward)).(*list[Item])
		l.SetSize(100, 10)
		cmd := l.Init()
		if cmd != nil {
			cmd()
		}
		l.SelectItemBelow()
		assert.Equal(t, items[2].ID(), l.selectedItem)
		golden.RequireEqual(t, []byte(l.View()))
	})
}

func TestListSetSelection(t *testing.T) {
	t.Parallel()
	t.Run("should move to the selected item", func(t *testing.T) {
		t.Parallel()
		items := []Item{}
		for i := range 100 {
			item := NewSelectableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := New(items, WithDirection(Forward)).(*list[Item])
		l.SetSize(100, 10)
		cmd := l.Init()
		if cmd != nil {
			cmd()
		}

		cmd = l.SetSelected(items[52].ID())
		if cmd != nil {
			cmd()
		}

		assert.Equal(t, items[52].ID(), l.selectedItem)
		golden.RequireEqual(t, []byte(l.View()))
	})
}

func TestListChanges(t *testing.T) {
	t.Parallel()
	t.Run("should append an item to the end", func(t *testing.T) {
		t.Parallel()
		items := []SelectableItem{}
		for i := range 20 {
			item := NewSelectableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := New(items, WithDirection(Backward)).(*list[SelectableItem])
		l.SetSize(100, 10)
		cmd := l.Init()
		if cmd != nil {
			cmd()
		}

		newItem := NewSelectableItem("New Item")
		l.AppendItem(newItem)

		assert.Equal(t, 21, len(l.items))
		assert.Equal(t, 21, len(l.renderedItems))
		assert.Equal(t, newItem.ID(), l.selectedItem)
		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("should should not change the selected if we moved the offset", func(t *testing.T) {
		t.Parallel()
		items := []SelectableItem{}
		for i := range 20 {
			item := NewSelectableItem(fmt.Sprintf("Item %d\nLine2", i))
			items = append(items, item)
		}
		l := New(items, WithDirection(Backward)).(*list[SelectableItem])
		l.SetSize(100, 10)
		cmd := l.Init()
		if cmd != nil {
			cmd()
		}
		l.MoveUp(1)

		newItem := NewSelectableItem("New Item")
		l.AppendItem(newItem)

		assert.Equal(t, 21, len(l.items))
		assert.Equal(t, 21, len(l.renderedItems))
		assert.Equal(t, l.items[19].ID(), l.selectedItem)
		golden.RequireEqual(t, []byte(l.View()))
	})
}

type SelectableItem interface {
	Item
	layout.Focusable
}

type simpleItem struct {
	width   int
	content string
	id      string
}
type selectableItem struct {
	*simpleItem
	focused bool
}

func NewSimpleItem(content string) *simpleItem {
	return &simpleItem{
		id:      uuid.NewString(),
		width:   0,
		content: content,
	}
}

func NewSelectableItem(content string) SelectableItem {
	return &selectableItem{
		simpleItem: NewSimpleItem(content),
		focused:    false,
	}
}

func (s *simpleItem) ID() string {
	return s.id
}

func (s *simpleItem) Init() tea.Cmd {
	return nil
}

func (s *simpleItem) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return s, nil
}

func (s *simpleItem) View() string {
	return lipgloss.NewStyle().Width(s.width).Render(s.content)
}

func (l *simpleItem) GetSize() (int, int) {
	return l.width, 0
}

// SetSize implements Item.
func (s *simpleItem) SetSize(width int, height int) tea.Cmd {
	s.width = width
	return nil
}

func (s *selectableItem) View() string {
	if s.focused {
		return lipgloss.NewStyle().BorderLeft(true).BorderStyle(lipgloss.NormalBorder()).Width(s.width).Render(s.content)
	}
	return lipgloss.NewStyle().Width(s.width).Render(s.content)
}

// Blur implements SimpleItem.
func (s *selectableItem) Blur() tea.Cmd {
	s.focused = false
	return nil
}

// Focus implements SimpleItem.
func (s *selectableItem) Focus() tea.Cmd {
	s.focused = true
	return nil
}

// IsFocused implements SimpleItem.
func (s *selectableItem) IsFocused() bool {
	return s.focused
}
