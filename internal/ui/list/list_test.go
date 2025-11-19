package list

import (
	"image"
	"testing"
)

func TestNewList(t *testing.T) {
	items := []Item{
		NewStringItem("1", "Item 1"),
		NewStringItem("2", "Item 2"),
		NewStringItem("3", "Item 3"),
	}

	bounds := image.Rect(0, 0, 10, 5)
	list := New(bounds, items...)

	if list.Count() != len(items) {
		t.Errorf("expected list count %d, got %d", len(items), list.Count())
	}

	for i, item := range items {
		gotItem, ok := list.At(i)
		if !ok {
			t.Errorf("expected item at index %d to exist", i)
			continue
		}
		if gotItem.ID() != item.ID() {
			t.Errorf("expected item ID %s, got %s", item.ID(), gotItem.ID())
		}
	}
}

func TestListAppend(t *testing.T) {
	bounds := image.Rect(0, 0, 10, 5)
	list := New(bounds)

	newItems := []Item{
		NewStringItem("1", "Item A"),
		NewStringItem("2", "Item B"),
	}

	list.Append(newItems...)

	if list.Count() != len(newItems) {
		t.Errorf("expected list count %d, got %d", len(newItems), list.Count())
	}

	for i, item := range newItems {
		gotItem, ok := list.At(i)
		if !ok {
			t.Errorf("expected item at index %d to exist", i)
			continue
		}
		if gotItem.ID() != item.ID() {
			t.Errorf("expected item ID %s, got %s", item.ID(), gotItem.ID())
		}
	}
}

func TestListUpdate(t *testing.T) {
	items := []Item{
		NewStringItem("1", "Old Item 1"),
		NewStringItem("2", "Old Item 2"),
	}

	bounds := image.Rect(0, 0, 10, 5)
	list := New(bounds, items...)

	updatedItem := NewStringItem("1", "New Item 1")
	success := list.Update(0, updatedItem)
	if !success {
		t.Errorf("expected update to succeed")
	}

	gotItem, ok := list.At(0)
	if !ok {
		t.Errorf("expected item at index 0 to exist")
	} else if gotItem.ID() != updatedItem.ID() {
		t.Errorf("expected item ID %s, got %s", updatedItem.ID(), gotItem.ID())
	}
}

func TestListDelete(t *testing.T) {
	items := []Item{
		NewStringItem("1", "Item 1"),
		NewStringItem("2", "Item 2"),
		NewStringItem("3", "Item 3"),
	}

	bounds := image.Rect(0, 0, 10, 5)
	list := New(bounds, items...)

	success := list.Delete(1)
	if !success {
		t.Errorf("expected delete to succeed")
	}

	if list.Count() != 2 {
		t.Errorf("expected list count 2, got %d", list.Count())
	}

	expectedItems := []Item{
		NewStringItem("1", "Item 1"),
		NewStringItem("3", "Item 3"),
	}

	for i, item := range expectedItems {
		gotItem, ok := list.At(i)
		if !ok {
			t.Errorf("expected item at index %d to exist", i)
			continue
		}
		if gotItem.ID() != item.ID() {
			t.Errorf("expected item ID %s, got %s", item.ID(), gotItem.ID())
		}
	}
}

func TestListRender(t *testing.T) {
	items := []Item{
		NewStringItem("1", "Line 1\nLine 2"),
		NewStringItem("2", "Line 3"),
		NewStringItem("3", "Line 4\nLine 5\nLine 6"),
	}

	bounds := image.Rect(0, 0, 10, 4)
	list := New(bounds, items...)

	rendered := list.Render()
	expected := "Line 1\nLine 2\nLine 3\n"

	if rendered != expected {
		t.Errorf("expected rendered output:\n%s\ngot:\n%s", expected, rendered)
	}
}

func TestListRenderReverse(t *testing.T) {
	items := []Item{
		NewStringItem("1", "Line 1\nLine 2"),
		NewStringItem("2", "Line 3"),
		NewStringItem("3", "Line 4\nLine 5\nLine 6"),
	}

	bounds := image.Rect(0, 0, 10, 4)
	list := New(bounds, items...)
	list.SetReverse(true)

	rendered := list.Render()
	expected := "Line 4\nLine 5\nLine 6\n"

	if rendered != expected {
		t.Errorf("expected rendered output:\n%s\ngot:\n%s", expected, rendered)
	}
}
