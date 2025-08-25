package list

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockVariableHeightItem is a test item with configurable height
type mockVariableHeightItem struct {
	id      string
	height  int
	content string
}

func (m *mockVariableHeightItem) ID() string {
	return m.id
}

func (m *mockVariableHeightItem) Init() tea.Cmd {
	return nil
}

func (m *mockVariableHeightItem) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m *mockVariableHeightItem) View() string {
	lines := make([]string, m.height)
	for i := 0; i < m.height; i++ {
		if i == 0 {
			lines[i] = m.content
		} else {
			lines[i] = fmt.Sprintf("  Line %d", i+1)
		}
	}
	return strings.Join(lines, "\n")
}

func (m *mockVariableHeightItem) SetSize(width, height int) tea.Cmd {
	return nil
}

func (m *mockVariableHeightItem) IsFocused() bool {
	return false
}

func (m *mockVariableHeightItem) Focus() tea.Cmd {
	return nil
}

func (m *mockVariableHeightItem) Blur() tea.Cmd {
	return nil
}

func (m *mockVariableHeightItem) GetSize() (int, int) {
	return 0, m.height
}

func TestArrowKeyNavigation(t *testing.T) {
	t.Run("should show full item when navigating with arrow keys", func(t *testing.T) {
		// Create items with varying heights
		items := []Item{
			&mockVariableHeightItem{id: "item1", height: 2, content: "Item 1 (2 lines)"},
			&mockVariableHeightItem{id: "item2", height: 3, content: "Item 2 (3 lines)"},
			&mockVariableHeightItem{id: "item3", height: 1, content: "Item 3 (1 line)"},
			&mockVariableHeightItem{id: "item4", height: 4, content: "Item 4 (4 lines)"},
			&mockVariableHeightItem{id: "item5", height: 2, content: "Item 5 (2 lines)"},
		}

		// Create list with viewport height of 6, width of 40
		l := New(items, WithDirectionForward(), WithSize(40, 6)).(*list[Item])
		execCmdNav(l, l.Init())

		// Initial state - first item should be selected
		assert.Equal(t, "item1", l.SelectedItemID())
		assert.Equal(t, 0, l.offset)

		// Navigate down to item 2
		_, cmd := l.Update(tea.KeyPressMsg(tea.Key{
			Code: tea.KeyDown,
		}))
		execCmdNav(l, cmd)

		assert.Equal(t, "item2", l.SelectedItemID())
		// Item 2 should be fully visible
		view := l.View()
		assert.Contains(t, view, "Item 2 (3 lines)")
		assert.Contains(t, view, "  Line 2")
		assert.Contains(t, view, "  Line 3")

		// Navigate down to item 3
		_, cmd = l.Update(tea.KeyPressMsg(tea.Key{
			Code: tea.KeyDown,
		}))
		execCmdNav(l, cmd)

		assert.Equal(t, "item3", l.SelectedItemID())
		view = l.View()
		assert.Contains(t, view, "Item 3 (1 line)")

		// Navigate down to item 4 (4 lines - might need scrolling)
		_, cmd = l.Update(tea.KeyPressMsg(tea.Key{
			Code: tea.KeyDown,
		}))
		execCmdNav(l, cmd)

		assert.Equal(t, "item4", l.SelectedItemID())
		view = l.View()
		// All lines of item 4 should be visible
		assert.Contains(t, view, "Item 4 (4 lines)")
		assert.Contains(t, view, "  Line 2")
		assert.Contains(t, view, "  Line 3")
		assert.Contains(t, view, "  Line 4")

		// Navigate back up to item 3
		_, cmd = l.Update(tea.KeyPressMsg(tea.Key{
			Code: tea.KeyUp,
		}))
		execCmdNav(l, cmd)

		assert.Equal(t, "item3", l.SelectedItemID())
		view = l.View()
		assert.Contains(t, view, "Item 3 (1 line)")

		// Navigate back up to item 2
		_, cmd = l.Update(tea.KeyPressMsg(tea.Key{
			Code: tea.KeyUp,
		}))
		execCmdNav(l, cmd)

		assert.Equal(t, "item2", l.SelectedItemID())
		view = l.View()
		// All lines of item 2 should be visible
		assert.Contains(t, view, "Item 2 (3 lines)")
		assert.Contains(t, view, "  Line 2")
		assert.Contains(t, view, "  Line 3")
	})

	t.Run("should not show partial items at viewport boundaries", func(t *testing.T) {
		// Create items with specific heights to test boundary conditions
		items := []Item{
			&mockVariableHeightItem{id: "item1", height: 3, content: "Item 1"},
			&mockVariableHeightItem{id: "item2", height: 3, content: "Item 2"},
			&mockVariableHeightItem{id: "item3", height: 3, content: "Item 3"},
			&mockVariableHeightItem{id: "item4", height: 3, content: "Item 4"},
		}

		// Create list with viewport height of 5, width of 40 (can't fit 2 full 3-line items)
		l := New(items, WithDirectionForward(), WithSize(40, 5)).(*list[Item])
		execCmdNav(l, l.Init())

		// Navigate to item 2
		_, cmd := l.Update(tea.KeyPressMsg(tea.Key{
			Code: tea.KeyDown,
		}))
		execCmdNav(l, cmd)

		view := l.View()
		lines := strings.Split(view, "\n")

		// Check that we have exactly 5 lines (viewport height)
		require.Len(t, lines, 5)

		// Item 2 should be fully visible
		assert.Contains(t, view, "Item 2")

		// Count how many lines of each item are visible
		item1Lines := 0
		item2Lines := 0
		for _, line := range lines {
			if strings.Contains(line, "Item 1") || (item1Lines > 0 && item1Lines < 3) {
				item1Lines++
			}
			if strings.Contains(line, "Item 2") || (item2Lines > 0 && item2Lines < 3) {
				item2Lines++
			}
		}

		// Item 2 should have all 3 lines visible
		assert.Equal(t, 3, item2Lines, "Item 2 should be fully visible")
	})

	t.Run("should handle items taller than viewport", func(t *testing.T) {
		// Create an item taller than the viewport
		items := []Item{
			&mockVariableHeightItem{id: "item1", height: 2, content: "Item 1"},
			&mockVariableHeightItem{id: "item2", height: 8, content: "Item 2 (tall)"},
			&mockVariableHeightItem{id: "item3", height: 2, content: "Item 3"},
		}

		// Create list with viewport height of 5, width of 40
		l := New(items, WithDirectionForward(), WithSize(40, 5)).(*list[Item])
		execCmdNav(l, l.Init())

		// Navigate to the tall item
		_, cmd := l.Update(tea.KeyPressMsg(tea.Key{
			Code: tea.KeyDown,
		}))
		execCmdNav(l, cmd)

		assert.Equal(t, "item2", l.SelectedItemID())
		view := l.View()

		// Should show the item from the top
		assert.Contains(t, view, "Item 2 (tall)")
		lines := strings.Split(view, "\n")
		assert.Len(t, lines, 5) // Should fill viewport
	})
}

// Helper function to execute commands
func execCmdNav(l *list[Item], cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	msg := cmd()
	if msg != nil {
		l.Update(msg)
	}
}
