package list

import "strings"

// RenderedItem represents a rendered item as a string.
type RenderedItem interface {
	Item
	// Height returns the height of the rendered item in lines.
	Height() int
}

// Item represents a single item in the [List] component.
type Item interface {
	// ID returns the unique identifier of the item.
	ID() string
	// Render returns the rendered string representation of the item.
	Render() string
}

// StringItem is a simple implementation of the [Item] interface that holds a
// string.
type StringItem struct {
	ItemID  string
	Content string
}

// NewStringItem creates a new StringItem with the given ID and content.
func NewStringItem(id, content string) StringItem {
	return StringItem{
		ItemID:  id,
		Content: content,
	}
}

// ID returns the unique identifier of the string item.
func (s StringItem) ID() string {
	return s.ItemID
}

// Render returns the rendered string representation of the string item.
func (s StringItem) Render() string {
	return s.Content
}

// Gap is [GapItem] to be used as a vertical gap in the list.
var Gap = GapItem{}

// GapItem is a one-line vertical gap in the list.
type GapItem struct{}

// ID returns the unique identifier of the gap.
func (g GapItem) ID() string {
	return "gap"
}

// Render returns the rendered string representation of the gap.
func (g GapItem) Render() string {
	return "\n"
}

// Height returns the height of the rendered gap in lines.
func (g GapItem) Height() int {
	return 1
}

// CachedItem wraps an Item and caches its rendered string representation and height.
type CachedItem struct {
	item     Item
	rendered string
	height   int
}

// NewCachedItem creates a new CachedItem from the given Item.
func NewCachedItem(item Item, rendered string) CachedItem {
	height := 1 + strings.Count(rendered, "\n")
	return CachedItem{
		item:     item,
		rendered: rendered,
		height:   height,
	}
}

// ID returns the unique identifier of the cached item.
func (c CachedItem) ID() string {
	return c.item.ID()
}

// Item returns the underlying Item.
func (c CachedItem) Item() Item {
	return c.item
}

// Render returns the cached rendered string representation of the item.
func (c CachedItem) Render() string {
	return c.rendered
}

// Height returns the cached height of the rendered item in lines.
func (c CachedItem) Height() int {
	return c.height
}
