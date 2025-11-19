// Package list implements a UI component for displaying a list of items.
package list

import (
	"image"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/exp/ordered"
	lru "github.com/hashicorp/golang-lru/v2"
)

// List represents a component that display a list of [Item]s.
type List struct {
	// idx is the current focused index in the list. -1 means no item is focused.
	idx int

	items []Item

	// yOffset is the current vertical offset for scrolling.
	yOffset int

	// linesCount is the cached total number of rendered lines in the list.
	linesCount int

	// rect is the bounding rectangle of the list.
	rect image.Rectangle

	// reverse indicates if the list is in reverse order.
	reverse bool

	// hasFocus indicates if the list has focus.
	hasFocus bool

	styles Styles

	cache *lru.Cache[string, RenderedItem]
}

// New creates a new [List] component with the given items.
func New(items ...Item) *List {
	cache, _ := lru.New[string, RenderedItem](256)
	l := &List{
		idx:    -1,
		items:  items,
		styles: DefaultStyles(),
		cache:  cache,
	}
	return l
}

// SetStyles sets the styles for the list.
func (l *List) SetStyles(s Styles) {
	l.styles = s
}

// SetReverse sets the reverse order of the list.
func (l *List) SetReverse(reverse bool) {
	l.reverse = reverse
}

// IsReverse returns true if the list is in reverse order.
func (l *List) IsReverse() bool {
	return l.reverse
}

// SetBounds sets the bounding rectangle of the list.
func (l *List) SetBounds(rect image.Rectangle) {
	if l.rect.Dx() != rect.Dx() {
		// Clear the cache if the width has changed. This is necessary because
		// the rendered items are wrapped to the width of the list.
		l.cache.Purge()
	}
	l.rect = rect
}

// Width returns the width of the list.
func (l *List) Width() int {
	return l.rect.Dx()
}

// Height returns the height of the list.
func (l *List) Height() int {
	return l.rect.Dy()
}

// X returns the X position of the list.
func (l *List) X() int {
	return l.rect.Min.X
}

// Y returns the Y position of the list.
func (l *List) Y() int {
	return l.rect.Min.Y
}

// Len returns the number of items in the list.
func (l *List) Len() int {
	return len(l.items)
}

// Items returns the items in the list.
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

// Focus focuses the list
func (l *List) Focus() {
	l.hasFocus = true
	if l.idx < 0 && len(l.items) > 0 {
		l.FocusFirst()
	}
}

// FocusFirst focuses the first item in the list.
func (l *List) FocusFirst() {
	if !l.hasFocus {
		l.Focus()
	}
	if l.reverse {
		l.idx = len(l.items) - 1
		return
	}
	l.idx = 0
}

// FocusLast focuses the last item in the list.
func (l *List) FocusLast() {
	if !l.hasFocus {
		l.Focus()
	}
	if l.reverse {
		l.idx = 0
		return
	}
	l.idx = len(l.items) - 1
}

// focus moves the focus by n offset. Positive n moves down, negative n moves up.
func (l *List) focus(n int) {
	if l.reverse {
		n = -n
	}

	if n < 0 {
		if l.idx+n < 0 {
			l.idx = 0
		} else {
			l.idx += n
		}
	} else if n > 0 {
		if l.idx+n >= len(l.items) {
			l.idx = len(l.items) - 1
		} else {
			l.idx += n
		}
	}
}

// FocusNext focuses the next item in the list.
func (l *List) FocusNext() {
	if !l.hasFocus {
		l.Focus()
	}
	l.focus(1)
}

// FocusPrev focuses the previous item in the list.
func (l *List) FocusPrev() {
	if !l.hasFocus {
		l.Focus()
	}
	l.focus(-1)
}

// FocusedItem returns the currently focused item.
func (l *List) FocusedItem() (Item, bool) {
	return l.At(l.idx)
}

// Blur removes focus from the list.
func (l *List) Blur() {
	l.hasFocus = false
}

// ScrollUp scrolls the list up by n lines.
func (l *List) ScrollUp(n int) {
	l.scroll(-n)
}

// ScrollDown scrolls the list down by n lines.
func (l *List) ScrollDown(n int) {
	l.scroll(n)
}

// scroll scrolls the list by n lines. Positive n scrolls down, negative n scrolls up.
func (l *List) scroll(n int) {
	if l.reverse {
		n = -n
	}

	if n > 0 {
		l.yOffset += n
		if l.linesCount > l.Height() && l.yOffset > l.linesCount-l.Height() {
			l.yOffset = l.linesCount - l.Height()
		}
	} else if n < 0 {
		l.yOffset += n
		if l.yOffset < 0 {
			l.yOffset = 0
		}
	}
}

// Render renders the first n items that fit within the list's height and
// returns the rendered string.
func (l *List) Render() string {
	var rendered []string
	availableHeight := l.Height()
	i := 0
	if l.reverse {
		i = len(l.items) - 1
	}

	// Render items until we run out of space
	for i >= 0 && i < len(l.items) {
		itemStyle := l.styles.NormalItem
		if l.hasFocus && l.idx == i {
			itemStyle = l.styles.FocusedItem
		}

		listWidth := l.Width() - itemStyle.GetHorizontalFrameSize()

		item, ok := l.At(i)
		if ok {
			cachedItem, ok := l.cache.Get(item.ID())
			if !ok {
				renderedItem := lipgloss.Wrap(item.Render(), listWidth, "")
				cachedItem = NewCachedItem(item, renderedItem)
				l.cache.Add(item.ID(), cachedItem)
			}

			renderedString := itemStyle.Render(cachedItem.Render())
			rendered = append(rendered, renderedString)
		}

		if l.reverse {
			i--
		} else {
			i++
		}
	}

	if l.reverse {
		slices.Reverse(rendered)
	}

	var sb strings.Builder
	for i, item := range rendered {
		sb.WriteString(item)
		if i < len(rendered)-1 {
			sb.WriteString("\n")
		}
	}

	linesCount := strings.Count(sb.String(), "\n") + 1
	l.linesCount = linesCount

	if linesCount <= availableHeight {
		return sb.String()
	}

	lines := strings.Split(sb.String(), "\n")
	yOffset := ordered.Clamp(l.yOffset, 0, linesCount-availableHeight)
	if l.reverse {
		start := len(lines) - availableHeight - yOffset
		end := max(availableHeight, len(lines)-l.yOffset)
		return strings.Join(lines[start:end], "\n")
	}

	start := 0 + yOffset
	end := min(len(lines), availableHeight+yOffset)
	return strings.Join(lines[start:end], "\n")
}

// View returns the rendered view of the list.
func (l *List) View() string {
	return l.Render()
}
