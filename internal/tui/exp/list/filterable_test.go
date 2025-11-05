package list

import (
	"fmt"
	"slices"
	"testing"

	"github.com/charmbracelet/x/exp/golden"
	"github.com/stretchr/testify/assert"
)

func TestFilterableList(t *testing.T) {
	t.Parallel()
	t.Run("should create simple filterable list", func(t *testing.T) {
		t.Parallel()
		items := []FilterableItem{}
		for i := range 5 {
			item := NewFilterableItem(fmt.Sprintf("Item %d", i))
			items = append(items, item)
		}
		l := NewFilterableList(
			items,
			WithFilterListOptions(WithDirectionForward()),
		).(*filterableList[FilterableItem])

		l.SetSize(100, 10)
		cmd := l.Init()
		if cmd != nil {
			cmd()
		}

		assert.Equal(t, 0, l.selectedItemIdx)
		golden.RequireEqual(t, []byte(l.View()))
	})
}

func TestUpdateKeyMap(t *testing.T) {
	t.Parallel()
	l := NewFilterableList(
		[]FilterableItem{},
		WithFilterListOptions(WithDirectionForward()),
	).(*filterableList[FilterableItem])

	hasJ := slices.Contains(l.keyMap.Down.Keys(), "j")
	fmt.Println(l.keyMap.Down.Keys())
	hasCtrlJ := slices.Contains(l.keyMap.Down.Keys(), "ctrl+j")

	hasUpperCaseK := slices.Contains(l.keyMap.UpOneItem.Keys(), "K")

	assert.False(t, l.keyMap.HalfPageDown.Enabled(), "should disable keys that are only letters")
	assert.False(t, hasJ, "should not contain j")
	assert.False(t, hasUpperCaseK, "should also remove upper case K")
	assert.True(t, hasCtrlJ, "should still have ctrl+j")
}

type filterableItem struct {
	*selectableItem
}

func NewFilterableItem(content string) FilterableItem {
	return &filterableItem{
		selectableItem: NewSelectableItem(content).(*selectableItem),
	}
}

func (f *filterableItem) FilterValue() string {
	return f.content
}
