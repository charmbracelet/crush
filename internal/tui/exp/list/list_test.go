package list

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/tui/components/core/layout"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestBackwardList(t *testing.T) {
	t.Run("within height", func(t *testing.T) {
		t.Parallel()
		l := New(WithDirection(Backward), WithGap(1)).(*list)
		l.SetSize(10, 20)
		items := []Item{}
		for i := range 5 {
			item := NewSimpleItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		cmd := l.SetItems(items)
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
			item := NewSimpleItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := New(WithDirection(Backward), WithGap(1), WithSelectedItem(items[2].ID())).(*list)
		l.SetSize(10, 20)
		cmd := l.SetItems(items)
		if cmd != nil {
			cmd()
		}
		// should select the last item
		assert.Equal(t, l.selectedItem, items[2].ID())
	})
	t.Run("more than height", func(t *testing.T) {
		t.Parallel()
		l := New(WithDirection(Backward))
		l.SetSize(10, 5)
		items := []Item{}
		for i := range 10 {
			item := NewSimpleItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		cmd := l.SetItems(items)
		if cmd != nil {
			cmd()
		}

		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("more than height multi line", func(t *testing.T) {
		t.Parallel()
		l := New(WithDirection(Backward))
		l.SetSize(10, 5)
		items := []Item{}
		for i := range 10 {
			item := NewSimpleItem(fmt.Sprintf("Item %d\nLine2", i))
			items = append(items, item)
		}
		cmd := l.SetItems(items)
		if cmd != nil {
			cmd()
		}

		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("should move up", func(t *testing.T) {
		t.Parallel()
		l := New(WithDirection(Backward)).(*list)
		l.SetSize(10, 5)
		items := []Item{}
		for i := range 10 {
			item := NewSimpleItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		cmd := l.SetItems(items)
		if cmd != nil {
			cmd()
		}

		l.MoveUp(1)
		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("should move at max to the top", func(t *testing.T) {
		t.Parallel()
		l := New(WithDirection(Backward)).(*list)
		l.SetSize(10, 5)
		items := []Item{}
		for i := range 10 {
			item := NewSimpleItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		cmd := l.SetItems(items)
		if cmd != nil {
			cmd()
		}

		l.MoveUp(100)
		assert.Equal(t, l.offset, lipgloss.Height(l.rendered)-l.listHeight())
		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("should do nothing with wrong move number", func(t *testing.T) {
		t.Parallel()
		l := New(WithDirection(Backward)).(*list)
		l.SetSize(10, 5)
		items := []Item{}
		for i := range 10 {
			item := NewSimpleItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		cmd := l.SetItems(items)
		if cmd != nil {
			cmd()
		}

		l.MoveUp(-10)
		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("should move to the top", func(t *testing.T) {
		t.Parallel()
		l := New(WithDirection(Backward)).(*list)
		l.SetSize(10, 5)
		items := []Item{}
		for i := range 10 {
			item := NewSimpleItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		cmd := l.SetItems(items)
		if cmd != nil {
			cmd()
		}

		l.GoToTop()
		assert.Equal(t, l.direction, Forward)
		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("should select the item above", func(t *testing.T) {
		t.Parallel()
		l := New(WithDirection(Backward)).(*list)
		l.SetSize(10, 5)
		items := []Item{}
		for i := range 10 {
			item := NewSimpleItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		cmd := l.SetItems(items)
		if cmd != nil {
			cmd()
		}

		selectedInx := len(l.items) - 2
		currentItem := items[len(l.items)-1]
		nextItem := items[selectedInx]
		assert.False(t, nextItem.(SimpleItem).IsFocused())
		assert.True(t, currentItem.(SimpleItem).IsFocused())
		cmd = l.SelectItemAbove()
		if cmd != nil {
			cmd()
		}

		assert.Equal(t, l.selectedItem, l.items[selectedInx].ID())
		assert.True(t, l.items[selectedInx].(SimpleItem).IsFocused())

		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("should move the view to be able to see the selected item", func(t *testing.T) {
		t.Parallel()
		l := New(WithDirection(Backward)).(*list)
		l.SetSize(10, 5)
		items := []Item{}
		for i := range 10 {
			item := NewSimpleItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		cmd := l.SetItems(items)
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
	t.Run("within height", func(t *testing.T) {
		t.Parallel()
		l := New(WithDirection(Forward), WithGap(1)).(*list)
		l.SetSize(10, 20)
		items := []Item{}
		for i := range 5 {
			item := NewSimpleItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		cmd := l.SetItems(items)
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
			item := NewSimpleItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := New(WithDirection(Forward), WithGap(1), WithSelectedItem(items[2].ID())).(*list)
		l.SetSize(10, 20)
		cmd := l.SetItems(items)
		if cmd != nil {
			cmd()
		}
		// should select the last item
		assert.Equal(t, l.selectedItem, items[2].ID())
	})
	t.Run("more than height", func(t *testing.T) {
		t.Parallel()
		l := New(WithDirection(Forward))
		l.SetSize(10, 5)
		items := []Item{}
		for i := range 10 {
			item := NewSimpleItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		cmd := l.SetItems(items)
		if cmd != nil {
			cmd()
		}

		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("more than height multi line", func(t *testing.T) {
		t.Parallel()
		l := New(WithDirection(Forward))
		l.SetSize(10, 5)
		items := []Item{}
		for i := range 10 {
			item := NewSimpleItem(fmt.Sprintf("Item %d\nLine2", i))
			items = append(items, item)
		}
		cmd := l.SetItems(items)
		if cmd != nil {
			cmd()
		}

		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("should move down", func(t *testing.T) {
		t.Parallel()
		l := New(WithDirection(Forward)).(*list)
		l.SetSize(10, 5)
		items := []Item{}
		for i := range 10 {
			item := NewSimpleItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		cmd := l.SetItems(items)
		if cmd != nil {
			cmd()
		}

		l.MoveDown(1)
		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("should move at max to the top", func(t *testing.T) {
		t.Parallel()
		l := New(WithDirection(Forward)).(*list)
		l.SetSize(10, 5)
		items := []Item{}
		for i := range 10 {
			item := NewSimpleItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		cmd := l.SetItems(items)
		if cmd != nil {
			cmd()
		}

		l.MoveDown(100)
		assert.Equal(t, l.offset, lipgloss.Height(l.rendered)-l.listHeight())
		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("should do nothing with wrong move number", func(t *testing.T) {
		t.Parallel()
		l := New(WithDirection(Forward)).(*list)
		l.SetSize(10, 5)
		items := []Item{}
		for i := range 10 {
			item := NewSimpleItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		cmd := l.SetItems(items)
		if cmd != nil {
			cmd()
		}

		l.MoveDown(-10)
		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("should move to the bottom", func(t *testing.T) {
		t.Parallel()
		l := New(WithDirection(Forward)).(*list)
		l.SetSize(10, 5)
		items := []Item{}
		for i := range 10 {
			item := NewSimpleItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		cmd := l.SetItems(items)
		if cmd != nil {
			cmd()
		}

		l.GoToBottom()
		assert.Equal(t, l.direction, Backward)
		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("should select the item below", func(t *testing.T) {
		t.Parallel()
		l := New(WithDirection(Forward)).(*list)
		l.SetSize(10, 5)
		items := []Item{}
		for i := range 10 {
			item := NewSimpleItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		cmd := l.SetItems(items)
		if cmd != nil {
			cmd()
		}

		selectedInx := 1
		currentItem := items[0]
		nextItem := items[selectedInx]
		assert.False(t, nextItem.(SimpleItem).IsFocused())
		assert.True(t, currentItem.(SimpleItem).IsFocused())
		cmd = l.SelectItemBelow()
		if cmd != nil {
			cmd()
		}

		assert.Equal(t, l.selectedItem, l.items[selectedInx].ID())
		assert.True(t, l.items[selectedInx].(SimpleItem).IsFocused())

		golden.RequireEqual(t, []byte(l.View()))
	})
	t.Run("should move the view to be able to see the selected item", func(t *testing.T) {
		t.Parallel()
		l := New(WithDirection(Backward)).(*list)
		l.SetSize(10, 5)
		items := []Item{}
		for i := range 10 {
			item := NewSimpleItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		cmd := l.SetItems(items)
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

type SimpleItem interface {
	Item
	layout.Focusable
}

type simpleItem struct {
	width   int
	content string
	id      string
	focused bool
}

func NewSimpleItem(content string) SimpleItem {
	return &simpleItem{
		width:   0,
		content: content,
		focused: false,
		id:      uuid.NewString(),
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
	if s.focused {
		return lipgloss.NewStyle().BorderLeft(true).BorderStyle(lipgloss.NormalBorder()).Width(s.width).Render(s.content)
	}
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

// Blur implements SimpleItem.
func (s *simpleItem) Blur() tea.Cmd {
	s.focused = false
	return nil
}

// Focus implements SimpleItem.
func (s *simpleItem) Focus() tea.Cmd {
	s.focused = true
	return nil
}

// IsFocused implements SimpleItem.
func (s *simpleItem) IsFocused() bool {
	return s.focused
}
