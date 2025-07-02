package uv

// Drawable represents a drawable component on a [Screen].
type Drawable interface {
	// Draw renders the component on the screen for the given area.
	Draw(scr Screen, area Rectangle)
}

// WidthMethod determines how many columns a grapheme occupies on the screen.
type WidthMethod interface {
	StringWidth(s string) int
}

// Screen represents a screen that can be drawn to.
type Screen interface {
	// Bounds returns the bounds of the screen. This is the rectangle that
	// includes the start and end points of the screen.
	Bounds() Rectangle

	// CellAt returns the cell at the given position. If the position is out of
	// bounds, it returns nil. Otherwise, it always returns a cell, even if it
	// is empty (i.e., a cell with a space character and a width of 1).
	CellAt(x, y int) *Cell

	// SetCell sets the cell at the given position. A nil cell is treated as an
	// empty cell with a space character and a width of 1.
	SetCell(x, y int, c *Cell)

	// WidthMethod returns the width method used by the screen.
	WidthMethod() WidthMethod
}

// CursorShape represents a terminal cursor shape.
type CursorShape int

// Cursor shapes.
const (
	CursorBlock CursorShape = iota
	CursorUnderline
	CursorBar
)

// Encode returns the encoded value for the cursor shape.
func (s CursorShape) Encode(blink bool) int {
	// We're using the ANSI escape sequence values for cursor styles.
	// We need to map both [style] and [steady] to the correct value.
	s = (s * 2) + 1 //nolint:mnd
	if !blink {
		s++
	}
	return int(s)
}
