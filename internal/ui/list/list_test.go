package list

import (
	"testing"
)

func TestNewList(t *testing.T) {
	items := []Item{
		NewStringItem("Item 1"),
		NewStringItem("Item 2"),
		NewStringItem("Item 3"),
	}

	var defaultRend DefaultItemRenderer
	list := New(&defaultRend, items...)

	if list.Len() != len(items) {
		t.Errorf("expected list count %d, got %d", len(items), list.Len())
	}

	rendered := list.Render()
	expected := "Item 1\nItem 2\nItem 3"
	if rendered != expected {
		t.Errorf("expected rendered output:\n%s\ngot:\n%s", expected, rendered)
	}
}

func TestListAppend(t *testing.T) {
	var defaultRend DefaultItemRenderer
	list := New(&defaultRend,
		NewStringItem("Item 1"),
	)

	list.Append(
		NewStringItem("Item 2"),
		NewStringItem("Item 3"),
	)

	if list.Len() != 3 {
		t.Errorf("expected list count 3, got %d", list.Len())
	}

	rendered := list.Render()
	expected := "Item 1\nItem 2\nItem 3"
	if rendered != expected {
		t.Errorf("expected rendered output:\n%s\ngot:\n%s", expected, rendered)
	}
}

func TestListUpdate(t *testing.T) {
	var defaultRend DefaultItemRenderer
	list := New(&defaultRend,
		NewStringItem("Item 1"),
		NewStringItem("Item 2"),
	)

	updated := list.Update(1, NewStringItem("Updated Item 2"))
	if !updated {
		t.Errorf("expected update to succeed")
	}

	rendered := list.Render()
	expected := "Item 1\nUpdated Item 2"
	if rendered != expected {
		t.Errorf("expected rendered output:\n%s\ngot:\n%s", expected, rendered)
	}
}

func TestListDelete(t *testing.T) {
	var defaultRend DefaultItemRenderer
	list := New(&defaultRend,
		NewStringItem("Item 1"),
		NewStringItem("Item 2"),
		NewStringItem("Item 3"),
	)

	deleted := list.Delete(1)
	if !deleted {
		t.Errorf("expected delete to succeed")
	}

	if list.Len() != 2 {
		t.Errorf("expected list count 2, got %d", list.Len())
	}

	rendered := list.Render()
	expected := "Item 1\nItem 3"
	if rendered != expected {
		t.Errorf("expected rendered output:\n%s\ngot:\n%s", expected, rendered)
	}
}
