# List Component

A high-performance, scrollable list component for Ultraviolet that efficiently manages and renders large numbers of items with dynamic heights and focus management.

## Features

- **Efficient Rendering**: Master buffer cache with dirty item tracking
- **Dynamic Heights**: Items can have variable heights that change at runtime
- **Smart Updates**: Optimized append/prepend/delete operations and partial updates
- **Focus Management**: Built-in focus support for items with automatic non-focusable skipping
- **Scrolling**: Smooth scrolling with viewport management
- **UV Integration**: Implements `uv.Drawable` for seamless integration
- **No Allocations on Render**: Reuses buffers and copies cells directly
- **Flexible Output**: Draw to screen buffers or render directly to string

## Basic Usage

### Creating a List

```go
// Create items
items := []list.Item{
    list.NewStringItem("1", "First item"),
    list.NewMarkdownItem("2", "## Second item\n\nWith **markdown**!", nil, nil, nil),
    list.NewStringItem("3", "Third item"),
}

// Create list
l := list.New(items...)
l.SetSize(80, 24)

// Draw to screen buffer
screen := uv.NewScreenBuffer(80, 24)
area := uv.Rect(0, 0, 80, 24)
l.Draw(&screen, area)
```

### Rendering Directly to String

```go
l := list.New(items...)
l.SetSize(80, 24)

// Render visible viewport to string
output := l.Render()
```

## Built-in Item Types

### StringItem

Simple text items with optional wrapping:

```go
// Non-wrapping
item := list.NewStringItem("id", "Single line content")

// With wrapping
item := list.NewWrappingStringItem("id", "Long content that will wrap...")
```

### MarkdownItem

Renders markdown using Glamour. Optionally focusable with styles:

```go
// Basic markdown
item := list.NewMarkdownItem("id", "## Heading\n\n- List item", nil, nil, nil)

// Focusable markdown with borders
focusStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("86"))
blurStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240"))
item := list.NewMarkdownItem("id", "Content", nil, &focusStyle, &blurStyle)
```

### SpacerItem

Adds vertical spacing between items:

```go
spacer := list.NewSpacerItem("gap-1", 2) // 2 lines of space
```

### FocusableItem

Wraps any item to add focus behavior:

```go
focusStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("86"))
blurStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240"))

inner := list.NewStringItem("id", "Content")
focusable := list.NewFocusableItem(inner, &focusStyle, &blurStyle)
```

## Custom Items

Implement the `Item` interface:

```go
type Item interface {
    uv.Drawable
    ID() string
    Height(width int) int
}
```

Optionally implement `Focusable` for focus support:

```go
type Focusable interface {
    Focus()
    Blur()
    IsFocused() bool
}
```

Example custom item:

```go
type MyItem struct {
    id      string
    content string
}

func (m *MyItem) ID() string { return m.id }

func (m *MyItem) Height(width int) int {
    return strings.Count(m.content, "\n") + 1
}

func (m *MyItem) Draw(scr uv.Screen, area uv.Rectangle) {
    styled := uv.NewStyledString(m.content)
    styled.Draw(scr, area)
}
```

## List Operations

### Managing Items

```go
// Set all items at once (full rebuild)
l.SetItems(items)

// Efficient operations (no full rebuild)
l.AppendItem(newItem)   // Add to end
l.PrependItem(newItem)  // Add to beginning
l.UpdateItem("id", updated) // Update existing
l.DeleteItem("id")      // Remove item

// Get items
items := l.Items()
```

### Selection & Navigation

```go
// Set selection
l.SetSelected("item-id")
l.SetSelectedIndex(0)

// Navigate (automatically skips non-focusable items when list is focused)
l.SelectNext()  // Wraps to beginning
l.SelectPrev()  // Wraps to end

// Get selection
item := l.SelectedItem()
idx := l.SelectedIndex()
```

### Focus Management

```go
// Focus the list (also focuses selected item if focusable)
l.Focus()

// Blur the list
l.Blur()

// Check focus state
if l.IsFocused() {
    // List is focused
}
```

### Scrolling

```go
// Scroll by offset
l.ScrollBy(10)   // Down 10 lines
l.ScrollBy(-10)  // Up 10 lines

// Scroll to positions
l.ScrollToTop()
l.ScrollToBottom()

// Scroll to specific item
l.ScrollToItem("item-id")
l.ScrollToSelected()

// Get state
offset := l.Offset()
totalHeight := l.TotalHeight()
```

### Sizing

```go
// Set viewport size
l.SetSize(80, 24)

// Width changes trigger full rebuild (items may reflow)
// Height changes only affect viewport clipping (efficient)
```

## Performance Optimizations

The list component is highly optimized for common operations:

1. **Master Buffer Caching**: All items pre-rendered, viewport extracted on demand
2. **Dirty Item Tracking**: Only changed items are re-rendered
3. **Efficient Mutations**: 
   - Append/Prepend: Direct buffer manipulation, no full rebuild
   - Delete: Slice operations, position adjustments
   - Update: In-place re-render if height unchanged
4. **Height-Change Optimization**: Multiple items changing height → rebuild, but all same height → fast in-place updates
5. **Viewport Height Changes**: No buffer rebuild, only offset clamping
6. **Focus Navigation**: Automatically skips non-focusable items (Gap, Spacer, etc.)

## Example: Chat-like Interface

```go
// Create list
l := list.New()
l.SetSize(80, 24)
l.Focus()

// Add messages with gaps
l.AppendItem(list.NewMarkdownItem("msg-1", "**User:** Hello!", nil, nil, nil))
l.AppendItem(list.NewSpacerItem("gap-1", 1))

l.AppendItem(list.NewMarkdownItem("msg-2", "**AI:** Hi there!", nil, nil, nil))
l.AppendItem(list.NewSpacerItem("gap-2", 1))

// Auto-scroll to bottom
l.ScrollToBottom()

// Navigate with keyboard (skips spacers automatically)
l.SelectPrev() // Jumps from msg-2 to msg-1, skipping gap-2
```
