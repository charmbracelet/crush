// Package list implements a UI component for displaying a list of items.
package list

import (
	"io"
	"slices"
	"strings"

	uv "github.com/charmbracelet/ultraviolet"
)

// ItemRenderer is an interface for rendering items in the list.
type ItemRenderer interface {
	// Render renders the item as a string.
	Render(w io.Writer, l *List, index int, item Item)
}

// DefaultItemRenderer is the default implementation of [ItemRenderer].
type DefaultItemRenderer struct{}

// Render renders the item as a string using its content.
func (r *DefaultItemRenderer) Render(w io.Writer, list *List, index int, item Item) {
	_, _ = io.WriteString(w, item.Content())
}

// List represents a component that renders a list of [Item]s via
// [ItemRenderer]s. It supports focus management and styling.
type List struct {
	// items is the master list of items.
	items []Item

	// rend is the item renderer for the list.
	rend ItemRenderer

	// width is the width of the list.
	width int

	// yOffset is the vertical scroll offset. -1 means scrolled to bottom.
	yOffset int
}

// New creates a new [List] component with the given items.
func New(rend ItemRenderer, items ...Item) *List {
	if rend == nil {
		rend = &DefaultItemRenderer{}
	}
	l := &List{
		rend:    rend,
		yOffset: -1,
	}
	l.Append(items...)
	return l
}

// SetWidth sets the width of the list.
func (l *List) SetWidth(width int) {
	l.width = width
}

// Width returns the width of the list.
func (l *List) Width() int {
	return l.width
}

// Len returns the number of items in the list.
func (l *List) Len() int {
	return len(l.items)
}

// Items returns a new slice of all items in the list.
func (l *List) Items() []Item {
	return l.items
}

// Update updates an item at the given index.
func (l *List) Update(index int, item Item) bool {
	if index < 0 || index >= len(l.items) {
		return false
	}
	l.items[index] = item
	return true
}

// At returns the item at the given index.
func (l *List) At(index int) (Item, bool) {
	if index < 0 || index >= len(l.items) {
		return nil, false
	}
	return l.items[index], true
}

// Delete removes the item at the given index.
func (l *List) Delete(index int) bool {
	if index < 0 || index >= len(l.items) {
		return false
	}
	l.items = slices.Delete(l.items, index, index+1)
	return true
}

// Append adds new items to the end of the list.
func (l *List) Append(items ...Item) {
	l.items = append(l.items, items...)
}

// GotoBottom scrolls the list to the bottom.
func (l *List) GotoBottom() {
	l.yOffset = -1
}

// GotoTop scrolls the list to the top.
func (l *List) GotoTop() {
	l.yOffset = 0
}

// TotalHeight returns the total height of all items in the list.
func (l *List) TotalHeight() int {
	total := 0
	for _, item := range l.items {
		total += item.Height()
	}
	return total
}

// ScrollUp scrolls the list up by the given number of lines.
func (l *List) ScrollUp(lines int) {
	if l.yOffset == -1 {
		// Calculate total height
		totalHeight := l.TotalHeight()
		l.yOffset = totalHeight
	}
	l.yOffset -= lines
	if l.yOffset < 0 {
		l.yOffset = 0
	}
}

// ScrollDown scrolls the list down by the given number of lines.
func (l *List) ScrollDown(lines int) {
	if l.yOffset == -1 {
		// Already at bottom
		return
	}
	l.yOffset += lines
	totalHeight := l.TotalHeight()
	if l.yOffset >= totalHeight {
		l.yOffset = -1 // Scroll to bottom
	}
}

// YOffset returns the current vertical scroll offset.
func (l *List) YOffset() int {
	if l.yOffset == -1 {
		return l.TotalHeight()
	}
	return l.yOffset
}

// Render renders the whole list as a string.
func (l *List) Render() string {
	return l.RenderRange(0, len(l.items))
}

// Draw draws the list to the given [uv.Screen] in the specified area.
func (l *List) Draw(scr uv.Screen, area uv.Rectangle) {
	yOffset := l.YOffset()
	rendered := l.RenderLines(yOffset, yOffset+area.Dy())
	uv.NewStyledString(rendered).Draw(scr, area)
}

// RenderRange renders a range of items from start to end indices.
func (l *List) RenderRange(start, end int) string {
	var b strings.Builder
	for i := start; i < end && i < len(l.items); i++ {
		item, ok := l.At(i)
		if !ok {
			continue
		}

		l.rend.Render(&b, l, i, item)
		if i < end-1 && i < len(l.items)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// RenderLines renders the list based on the start and end y offsets.
func (l *List) RenderLines(startY, endY int) string {
	var b strings.Builder
	currentY := 0
	for i := 0; i < len(l.items); i++ {
		item, ok := l.At(i)
		if !ok {
			continue
		}

		itemHeight := item.Height()
		if currentY+itemHeight <= startY {
			// Skip this item as it's above the startY
			currentY += itemHeight
			continue
		}
		if currentY >= endY {
			// Stop rendering as we've reached endY
			break
		}

		// Render the item to a temporary buffer if needed
		if currentY < startY || currentY+itemHeight > endY {
			var tempBuf strings.Builder
			l.rend.Render(&tempBuf, l, i, item)
			lines := strings.Split(tempBuf.String(), "\n")

			// Calculate the visible lines
			startLine := 0
			if currentY < startY {
				startLine = startY - currentY
			}
			endLine := itemHeight
			if currentY+itemHeight > endY {
				endLine = endY - currentY
			}

			// Write only the visible lines
			for j := startLine; j < endLine && j < len(lines); j++ {
				b.WriteString(lines[j])
				b.WriteString("\n")
			}
		} else {
			// Render the whole item directly
			l.rend.Render(&b, l, i, item)
			b.WriteString("\n")
		}

		currentY += itemHeight
	}

	return strings.TrimRight(b.String(), "\n")
}
