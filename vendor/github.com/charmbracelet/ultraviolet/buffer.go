package uv

import (
	"image"
	"io"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Position represents a position in a coordinate system.
type Position = image.Point

// Pos is a shorthand for creating a new [Position].
func Pos(x, y int) Position {
	return Position{X: x, Y: y}
}

// Rectangle represents a rectangular area.
type Rectangle = image.Rectangle

// Rect is a shorthand for creating a new [Rectangle].
func Rect(x, y, w, h int) Rectangle {
	return Rectangle{Min: image.Point{X: x, Y: y}, Max: image.Point{X: x + w, Y: y + h}}
}

// Line represents cells in a line.
type Line []Cell

// Set sets the cell at the given x position.
func (l Line) Set(x int, c *Cell) {
	// maxCellWidth is the maximum width a terminal cell is expected to have.
	const maxCellWidth = 5

	lineWidth := len(l)
	if x < 0 || x >= lineWidth {
		return
	}

	// When a wide cell is partially overwritten, we need
	// to fill the rest of the cell with space cells to
	// avoid rendering issues.
	var prev *Cell
	if prev = l.At(x); prev != nil {
		if pw := prev.Width; pw > 1 {
			// Writing to the first wide cell
			for j := 0; j < pw && x+j < lineWidth; j++ {
				l[x+j] = *prev
				l[x+j].Empty()
			}
		} else if pw == 0 {
			// Writing to wide cell placeholders
			for j := 1; j < maxCellWidth && x-j >= 0; j++ {
				if wide := l.At(x - j); wide != nil {
					if ww := wide.Width; ww > 1 && j < ww {
						for k := 0; k < ww; k++ {
							l[x-j+k] = *wide
							l[x-j+k].Empty()
						}
						break
					}
				}
			}
		}
	}

	if c == nil {
		// Nil cells are treated as blank empty cells.
		l[x] = EmptyCell
		return
	}

	l[x] = *c
	cw := c.Width
	if x+cw > lineWidth {
		// If the cell is too wide, we write blanks with the same style.
		for i := 0; i < cw && x+i < lineWidth; i++ {
			l[x+i] = *c
			l[x+i].Empty()
		}
		return
	}

	if cw > 1 {
		// Mark wide cells with an zero cells.
		// We set the wide cell down below
		for j := 1; j < cw && x+j < lineWidth; j++ {
			l[x+j] = Cell{}
		}
	}
}

// At returns the cell at the given x position.
// If the cell does not exist, it returns nil.
func (l Line) At(x int) *Cell {
	if x < 0 || x >= len(l) {
		return nil
	}

	return &l[x]
}

// String returns the string representation of the line. Any trailing spaces
// are removed.
func (l Line) String() (s string) {
	for _, c := range l {
		if c.IsZero() {
			continue
		} else {
			s += c.String()
		}
	}
	s = strings.TrimRight(s, " ")
	return
}

// Render renders the line to a string with all the required attributes and
// styles.
func (l Line) Render() string {
	var buf strings.Builder
	renderLine(&buf, l)
	return strings.TrimRight(buf.String(), " ") // Trim trailing spaces
}

func renderLine(buf io.StringWriter, l Line) {
	var pen Style
	var link Link
	var pendingLine string
	var pendingWidth int // this ignores space cells until we hit a non-space cell

	writePending := func() {
		// If there's no pending line, we don't need to do anything.
		if len(pendingLine) == 0 {
			return
		}
		buf.WriteString(pendingLine)
		pendingWidth = 0
		pendingLine = ""
	}

	for _, cell := range l {
		if cell.Width > 0 {
			// Convert the cell's style and link to the given color profile.
			cellStyle := cell.Style
			cellLink := cell.Link
			if cellStyle.IsZero() && !pen.IsZero() {
				writePending()
				buf.WriteString(ansi.ResetStyle) //nolint:errcheck
				pen = Style{}
			}
			if !cellStyle.Equal(&pen) {
				writePending()
				seq := cellStyle.DiffSequence(pen)
				buf.WriteString(seq) // nolint:errcheck
				pen = cellStyle
			}

			// Write the URL escape sequence
			if cellLink != link && link.URL != "" {
				writePending()
				buf.WriteString(ansi.ResetHyperlink()) //nolint:errcheck
				link = Link{}
			}
			if cellLink != link {
				writePending()
				buf.WriteString(ansi.SetHyperlink(cellLink.URL, cellLink.Params)) //nolint:errcheck
				link = cellLink
			}

			// We only write the cell content if it's not empty. If it is, we
			// append it to the pending line and width to be evaluated later.
			if cell.Equal(&EmptyCell) {
				pendingLine += cell.String()
				pendingWidth += cell.Width
			} else {
				writePending()
				buf.WriteString(cell.String()) //nolint:errcheck
			}
		}
	}
	if link.URL != "" {
		buf.WriteString(ansi.ResetHyperlink()) //nolint:errcheck
	}
	if !pen.IsZero() {
		buf.WriteString(ansi.ResetStyle) //nolint:errcheck
	}
}

// Buffer represents a cell buffer that contains the contents of a screen.
type Buffer struct {
	// Lines is a slice of lines that make up the cells of the buffer.
	Lines []Line
	// Touched represents the lines that have been modified or touched. It is
	// used to track which lines need to be redrawn.
	Touched []*LineData
}

// NewBuffer creates a new buffer with the given width and height.
// This is a convenience function that initializes a new buffer and resizes it.
func NewBuffer(width int, height int) *Buffer {
	b := new(Buffer)
	b.Lines = make([]Line, height)
	for i := range b.Lines {
		b.Lines[i] = make(Line, width)
		for j := range b.Lines[i] {
			b.Lines[i][j] = EmptyCell
		}
	}
	b.Touched = make([]*LineData, height)
	b.Resize(width, height)
	return b
}

// String returns the string representation of the buffer.
func (b *Buffer) String() string {
	var buf strings.Builder
	for i, l := range b.Lines {
		buf.WriteString(l.String())
		if i < len(b.Lines)-1 {
			buf.WriteString("\r\n") //nolint:errcheck
		}
	}
	return buf.String()
}

// Render renders the buffer to a string with all the required attributes and
// styles.
func (b *Buffer) Render() string {
	var buf strings.Builder
	for i, l := range b.Lines {
		renderLine(&buf, l)
		if i < len(b.Lines)-1 {
			buf.WriteString("\r\n") //nolint:errcheck
		}
	}
	return strings.TrimRight(buf.String(), " ") // Trim trailing spaces
}

// Line returns a pointer to the line at the given y position.
// If the line does not exist, it returns nil.
func (b *Buffer) Line(y int) Line {
	if y < 0 || y >= len(b.Lines) {
		return nil
	}
	return b.Lines[y]
}

// CellAt returns the cell at the given position. It returns nil if the
// position is out of bounds.
func (b *Buffer) CellAt(x int, y int) *Cell {
	if y < 0 || y >= len(b.Lines) {
		return nil
	}
	return b.Lines[y].At(x)
}

// SetCell sets the cell at the given x, y position.
func (b *Buffer) SetCell(x, y int, c *Cell) {
	if y < 0 || y >= len(b.Lines) {
		return
	}

	if !cellEqual(b.CellAt(x, y), c) {
		width := 1
		if c != nil && c.Width > 0 {
			width = c.Width
		}
		b.TouchLine(x, y, width)
	}
	b.Lines[y].Set(x, c)
}

// Touch marks the cell at the given x, y position as touched.
func (b *Buffer) Touch(x, y int) {
	b.TouchLine(x, y, 0)
}

// TouchLine marks a line n times starting at the given x position as touched.
func (b *Buffer) TouchLine(x, y, n int) {
	if y < 0 || y >= len(b.Lines) {
		return
	}

	if y >= len(b.Touched) {
		b.Touched = append(b.Touched, make([]*LineData, y-len(b.Touched)+1)...)
	}

	ch := b.Touched[y]
	if ch == nil {
		ch = &LineData{FirstCell: x, LastCell: x + n}
	} else {
		ch.FirstCell = min(ch.FirstCell, x)
		ch.LastCell = max(ch.LastCell, x+n)
	}
	b.Touched[y] = ch
}

// Height implements Screen.
func (b *Buffer) Height() int {
	return len(b.Lines)
}

// Width implements Screen.
func (b *Buffer) Width() int {
	if len(b.Lines) == 0 {
		return 0
	}
	w := len(b.Lines[0])
	for _, l := range b.Lines {
		if len(l) > w {
			w = len(l)
		}
	}
	return w
}

// Bounds returns the bounds of the buffer.
func (b *Buffer) Bounds() Rectangle {
	return Rect(0, 0, b.Width(), b.Height())
}

// Resize resizes the buffer to the given width and height.
func (b *Buffer) Resize(width int, height int) {
	curWidth, curHeight := b.Width(), b.Height()
	if curWidth == width && curHeight == height {
		// No need to resize if the dimensions are the same.
		return
	}

	if width > curWidth {
		line := make(Line, width-curWidth)
		for i := range line {
			line[i] = EmptyCell
		}
		for i := range b.Lines {
			b.Lines[i] = append(b.Lines[i], line...)
		}
	} else if width < curWidth {
		for i := range b.Lines {
			b.Lines[i] = b.Lines[i][:width]
		}
	}

	if height > len(b.Lines) {
		for i := len(b.Lines); i < height; i++ {
			line := make(Line, width)
			for j := range line {
				line[j] = EmptyCell
			}
			b.Lines = append(b.Lines, line)
		}
	} else if height < len(b.Lines) {
		b.Lines = b.Lines[:height]
	}
}

// Fill fills the buffer with the given cell and rectangle.
func (b *Buffer) Fill(c *Cell) {
	b.FillArea(c, b.Bounds())
}

// FillArea fills the buffer with the given cell and rectangle.
func (b *Buffer) FillArea(c *Cell, area Rectangle) {
	cellWidth := 1
	if c != nil && c.Width > 1 {
		cellWidth = c.Width
	}
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x += cellWidth {
			b.SetCell(x, y, c)
		}
	}
}

// Clear clears the buffer with space cells and rectangle.
func (b *Buffer) Clear() {
	b.ClearArea(b.Bounds())
}

// ClearArea clears the buffer with space cells within the specified
// rectangles. Only cells within the rectangle's bounds are affected.
func (b *Buffer) ClearArea(area Rectangle) {
	b.FillArea(nil, area)
}

// CloneArea clones the area of the buffer within the specified rectangle. If
// the area is out of bounds, it returns nil.
func (b *Buffer) CloneArea(area Rectangle) *Buffer {
	bounds := b.Bounds()
	if !area.In(bounds) {
		return nil
	}
	n := NewBuffer(area.Dx(), area.Dy())
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			c := b.CellAt(x, y)
			n.SetCell(x-area.Min.X, y-area.Min.Y, c)
		}
	}
	return n
}

// Clone clones the entire buffer into a new buffer.
func (b *Buffer) Clone() *Buffer {
	return b.CloneArea(b.Bounds())
}

// Draw draws the buffer to the given screen at the specified area.
// It implements the [Drawable] interface.
func (b *Buffer) Draw(scr Screen, area Rectangle) {
	if area.Empty() {
		return
	}

	// Ensure the area is within the bounds of the screen.
	bounds := scr.Bounds()
	if !area.In(bounds) {
		return
	}

	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			c := b.CellAt(x-area.Min.X, y-area.Min.Y)
			if c == nil || c.IsZero() {
				continue
			}
			scr.SetCell(x, y, c)
		}
	}
}

// InsertLine inserts n lines at the given line position, with the given
// optional cell, within the specified rectangles. If no rectangles are
// specified, it inserts lines in the entire buffer. Only cells within the
// rectangle's horizontal bounds are affected. Lines are pushed out of the
// rectangle bounds and lost. This follows terminal [ansi.IL] behavior.
// It returns the pushed out lines.
func (b *Buffer) InsertLine(y, n int, c *Cell) {
	b.InsertLineArea(y, n, c, b.Bounds())
}

// InsertLineArea inserts new lines at the given line position, with the
// given optional cell, within the rectangle bounds. Only cells within the
// rectangle's horizontal bounds are affected. Lines are pushed out of the
// rectangle bounds and lost. This follows terminal [ansi.IL] behavior.
func (b *Buffer) InsertLineArea(y, n int, c *Cell, area Rectangle) {
	if n <= 0 || y < area.Min.Y || y >= area.Max.Y || y >= b.Height() {
		return
	}

	// Limit number of lines to insert to available space
	if y+n > area.Max.Y {
		n = area.Max.Y - y
	}

	// Move existing lines down within the bounds
	for i := area.Max.Y - 1; i >= y+n; i-- {
		for x := area.Min.X; x < area.Max.X; x++ {
			// We don't need to clone c here because we're just moving lines down.
			b.Lines[i][x] = b.Lines[i-n][x]
		}
		b.TouchLine(area.Min.X, i, area.Max.X-area.Min.X)
		b.TouchLine(area.Min.X, i-n, area.Max.X-area.Min.X)
	}

	// Clear the newly inserted lines within bounds
	for i := y; i < y+n; i++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			b.SetCell(x, i, c)
		}
	}
}

// DeleteLineArea deletes lines at the given line position, with the given
// optional cell, within the rectangle bounds. Only cells within the
// rectangle's bounds are affected. Lines are shifted up within the bounds and
// new blank lines are created at the bottom. This follows terminal [ansi.DL]
// behavior.
func (b *Buffer) DeleteLineArea(y, n int, c *Cell, area Rectangle) {
	if n <= 0 || y < area.Min.Y || y >= area.Max.Y || y >= b.Height() {
		return
	}

	// Limit deletion count to available space in scroll region
	if n > area.Max.Y-y {
		n = area.Max.Y - y
	}

	// Shift cells up within the bounds
	for dst := y; dst < area.Max.Y-n; dst++ {
		src := dst + n
		for x := area.Min.X; x < area.Max.X; x++ {
			// We don't need to clone c here because we're just moving cells up.
			b.Lines[dst][x] = b.Lines[src][x]
		}
		b.TouchLine(area.Min.X, dst, area.Max.X-area.Min.X)
		b.TouchLine(area.Min.X, src, area.Max.X-area.Min.X)
	}

	// Fill the bottom n lines with blank cells
	for i := area.Max.Y - n; i < area.Max.Y; i++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			b.SetCell(x, i, c)
		}
	}
}

// DeleteLine deletes n lines at the given line position, with the given
// optional cell, within the specified rectangles. If no rectangles are
// specified, it deletes lines in the entire buffer.
func (b *Buffer) DeleteLine(y, n int, c *Cell) {
	b.DeleteLineArea(y, n, c, b.Bounds())
}

// InsertCell inserts new cells at the given position, with the given optional
// cell, within the specified rectangles. If no rectangles are specified, it
// inserts cells in the entire buffer. This follows terminal [ansi.ICH]
// behavior.
func (b *Buffer) InsertCell(x, y, n int, c *Cell) {
	b.InsertCellArea(x, y, n, c, b.Bounds())
}

// InsertCellArea inserts new cells at the given position, with the given
// optional cell, within the rectangle bounds. Only cells within the
// rectangle's bounds are affected, following terminal [ansi.ICH] behavior.
func (b *Buffer) InsertCellArea(x, y, n int, c *Cell, area Rectangle) {
	if n <= 0 || y < area.Min.Y || y >= area.Max.Y || y >= b.Height() ||
		x < area.Min.X || x >= area.Max.X || x >= b.Width() {
		return
	}

	// Limit number of cells to insert to available space
	if x+n > area.Max.X {
		n = area.Max.X - x
	}

	// Move existing cells within rectangle bounds to the right
	for i := area.Max.X - 1; i >= x+n && i-n >= area.Min.X; i-- {
		// We don't need to clone c here because we're just moving cells to the
		// right.
		b.Lines[y][i] = b.Lines[y][i-n]
	}
	// Touch the lines that were moved
	b.TouchLine(x, y, n)

	// Clear the newly inserted cells within rectangle bounds
	for i := x; i < x+n && i < area.Max.X; i++ {
		b.SetCell(i, y, c)
	}
}

// DeleteCell deletes cells at the given position, with the given optional
// cell, within the specified rectangles. If no rectangles are specified, it
// deletes cells in the entire buffer. This follows terminal [ansi.DCH]
// behavior.
func (b *Buffer) DeleteCell(x, y, n int, c *Cell) {
	b.DeleteCellArea(x, y, n, c, b.Bounds())
}

// DeleteCellArea deletes cells at the given position, with the given
// optional cell, within the rectangle bounds. Only cells within the
// rectangle's bounds are affected, following terminal [ansi.DCH] behavior.
func (b *Buffer) DeleteCellArea(x, y, n int, c *Cell, area Rectangle) {
	if n <= 0 || y < area.Min.Y || y >= area.Max.Y || y >= b.Height() ||
		x < area.Min.X || x >= area.Max.X || x >= b.Width() {
		return
	}

	// Calculate how many positions we can actually delete
	remainingCells := area.Max.X - x
	if n > remainingCells {
		n = remainingCells
	}

	// Shift the remaining cells to the left
	for i := x; i < area.Max.X-n; i++ {
		if i+n < area.Max.X {
			// We don't need to clone c here because we're just moving cells to
			// the left.
			b.Lines[y][i] = b.Lines[y][i+n]
		}
	}
	// Touch the line that was modified
	b.TouchLine(x, y, n)

	// Fill the vacated positions with the given cell
	for i := area.Max.X - n; i < area.Max.X; i++ {
		b.SetCell(i, y, c)
	}
}

// ScreenBuffer is a buffer that can be used as a [Screen].
type ScreenBuffer struct {
	*Buffer
	Method ansi.Method
}

var _ Screen = ScreenBuffer{}

// NewScreenBuffer creates a new ScreenBuffer with the given width and height.
func NewScreenBuffer(width, height int) ScreenBuffer {
	return ScreenBuffer{
		Buffer: NewBuffer(width, height),
		Method: ansi.WcWidth,
	}
}

// WidthMethod returns the width method used by the screen.
// It defaults to [ansi.WcWidth].
func (s ScreenBuffer) WidthMethod() WidthMethod {
	return s.Method
}
