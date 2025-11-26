package list

import (
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
)

func TestNewList(t *testing.T) {
	items := []Item{
		NewStringItem("1", "Item 1"),
		NewStringItem("2", "Item 2"),
		NewStringItem("3", "Item 3"),
	}

	l := New(items...)
	l.SetSize(80, 24)

	if len(l.items) != 3 {
		t.Errorf("expected 3 items, got %d", len(l.items))
	}

	if l.width != 80 || l.height != 24 {
		t.Errorf("expected size 80x24, got %dx%d", l.width, l.height)
	}
}

func TestListDraw(t *testing.T) {
	items := []Item{
		NewStringItem("1", "Item 1"),
		NewStringItem("2", "Item 2"),
		NewStringItem("3", "Item 3"),
	}

	l := New(items...)
	l.SetSize(80, 10)

	// Create a screen buffer to draw into
	screen := uv.NewScreenBuffer(80, 10)
	area := uv.Rect(0, 0, 80, 10)

	// Draw the list
	l.Draw(&screen, area)

	// Verify the buffer has content
	output := screen.Render()
	if len(output) == 0 {
		t.Error("expected non-empty output")
	}
}

func TestListAppendItem(t *testing.T) {
	items := []Item{
		NewStringItem("1", "Item 1"),
	}

	l := New(items...)
	l.AppendItem(NewStringItem("2", "Item 2"))

	if len(l.items) != 2 {
		t.Errorf("expected 2 items after append, got %d", len(l.items))
	}

	if l.items[1].ID() != "2" {
		t.Errorf("expected item ID '2', got '%s'", l.items[1].ID())
	}
}

func TestListDeleteItem(t *testing.T) {
	items := []Item{
		NewStringItem("1", "Item 1"),
		NewStringItem("2", "Item 2"),
		NewStringItem("3", "Item 3"),
	}

	l := New(items...)
	l.DeleteItem("2")

	if len(l.items) != 2 {
		t.Errorf("expected 2 items after delete, got %d", len(l.items))
	}

	if l.items[1].ID() != "3" {
		t.Errorf("expected item ID '3', got '%s'", l.items[1].ID())
	}
}

func TestListUpdateItem(t *testing.T) {
	items := []Item{
		NewStringItem("1", "Item 1"),
		NewStringItem("2", "Item 2"),
	}

	l := New(items...)
	l.SetSize(80, 10)

	// Update item
	newItem := NewStringItem("2", "Updated Item 2")
	l.UpdateItem("2", newItem)

	if l.items[1].(*StringItem).content != "Updated Item 2" {
		t.Errorf("expected updated content, got '%s'", l.items[1].(*StringItem).content)
	}
}

func TestListSelection(t *testing.T) {
	items := []Item{
		NewStringItem("1", "Item 1"),
		NewStringItem("2", "Item 2"),
		NewStringItem("3", "Item 3"),
	}

	l := New(items...)
	l.SetSelectedIndex(0)

	if l.SelectedIndex() != 0 {
		t.Errorf("expected selected index 0, got %d", l.SelectedIndex())
	}

	l.SelectNext()
	if l.SelectedIndex() != 1 {
		t.Errorf("expected selected index 1 after SelectNext, got %d", l.SelectedIndex())
	}

	l.SelectPrev()
	if l.SelectedIndex() != 0 {
		t.Errorf("expected selected index 0 after SelectPrev, got %d", l.SelectedIndex())
	}
}

func TestListScrolling(t *testing.T) {
	items := []Item{
		NewStringItem("1", "Item 1"),
		NewStringItem("2", "Item 2"),
		NewStringItem("3", "Item 3"),
		NewStringItem("4", "Item 4"),
		NewStringItem("5", "Item 5"),
	}

	l := New(items...)
	l.SetSize(80, 2) // Small viewport

	// Draw to initialize the master buffer
	screen := uv.NewScreenBuffer(80, 2)
	area := uv.Rect(0, 0, 80, 2)
	l.Draw(&screen, area)

	if l.Offset() != 0 {
		t.Errorf("expected initial offset 0, got %d", l.Offset())
	}

	l.ScrollBy(2)
	if l.Offset() != 2 {
		t.Errorf("expected offset 2 after ScrollBy(2), got %d", l.Offset())
	}

	l.ScrollToTop()
	if l.Offset() != 0 {
		t.Errorf("expected offset 0 after ScrollToTop, got %d", l.Offset())
	}
}

// FocusableTestItem is a test item that implements Focusable.
type FocusableTestItem struct {
	id      string
	content string
	focused bool
}

func (f *FocusableTestItem) ID() string {
	return f.id
}

func (f *FocusableTestItem) Height(width int) int {
	return 1
}

func (f *FocusableTestItem) Draw(scr uv.Screen, area uv.Rectangle) {
	prefix := "[ ]"
	if f.focused {
		prefix = "[X]"
	}
	content := prefix + " " + f.content
	styled := uv.NewStyledString(content)
	styled.Draw(scr, area)
}

func (f *FocusableTestItem) Focus() {
	f.focused = true
}

func (f *FocusableTestItem) Blur() {
	f.focused = false
}

func (f *FocusableTestItem) IsFocused() bool {
	return f.focused
}

func TestListFocus(t *testing.T) {
	items := []Item{
		&FocusableTestItem{id: "1", content: "Item 1"},
		&FocusableTestItem{id: "2", content: "Item 2"},
	}

	l := New(items...)
	l.SetSize(80, 10)
	l.SetSelectedIndex(0)

	// Focus the list
	l.Focus()

	if !l.Focused() {
		t.Error("expected list to be focused")
	}

	// Check if selected item is focused
	selectedItem := l.SelectedItem().(*FocusableTestItem)
	if !selectedItem.IsFocused() {
		t.Error("expected selected item to be focused")
	}

	// Select next and check focus changes
	l.SelectNext()
	if selectedItem.IsFocused() {
		t.Error("expected previous item to be blurred")
	}

	newSelectedItem := l.SelectedItem().(*FocusableTestItem)
	if !newSelectedItem.IsFocused() {
		t.Error("expected new selected item to be focused")
	}

	// Blur the list
	l.Blur()
	if l.Focused() {
		t.Error("expected list to be blurred")
	}
}
