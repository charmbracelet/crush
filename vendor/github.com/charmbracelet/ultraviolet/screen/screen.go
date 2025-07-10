// Package screen provides functions and helpers to manipulate a [uv.Screen].
package screen

import uv "github.com/charmbracelet/ultraviolet"

// Clear clears the screen with empty cells. This is equivalent to filling the
// screen with empty cells.
//
// If the screen implements a [Clear] method, it will be called instead of
// filling the screen with empty cells.
func Clear(scr uv.Screen) {
	if c, ok := scr.(interface {
		Clear()
	}); ok {
		c.Clear()
		return
	}
	Fill(scr, nil)
}

// ClearArea clears the given area of the screen with empty cells. This is
// equivalent to filling the area with empty cells.
//
// If the screen implements a [ClearArea] method, it will be called instead of
// filling the area with empty cells.
func ClearArea(scr uv.Screen, area uv.Rectangle) {
	if c, ok := scr.(interface {
		ClearArea(area uv.Rectangle)
	}); ok {
		c.ClearArea(area)
		return
	}
	FillArea(scr, nil, area)
}

// Fill fills the screen with the given cell. If the cell is nil, it fills the
// screen with empty cells.
//
// If the screen implements a [Fill] method, it will be called instead of
// filling the screen with empty cells.
func Fill(scr uv.Screen, cell *uv.Cell) {
	if f, ok := scr.(interface {
		Fill(cell *uv.Cell)
	}); ok {
		f.Fill(cell)
		return
	}
	FillArea(scr, cell, scr.Bounds())
}

// FillArea fills the given area of the screen with the given cell. If the cell
// is nil, it fills the area with empty cells.
//
// If the screen implements a [FillArea] method, it will be called instead of
// filling the area with empty cells.
func FillArea(scr uv.Screen, cell *uv.Cell, area uv.Rectangle) {
	if f, ok := scr.(interface {
		FillArea(cell *uv.Cell, area uv.Rectangle)
	}); ok {
		f.FillArea(cell, area)
		return
	}
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			scr.SetCell(x, y, cell)
		}
	}
}

// CloneArea clones the given area of the screen and returns a new buffer
// with the same size as the area. The new buffer will contain the same cells
// as the area in the screen.
// Use [uv.Buffer.Draw] to draw the cloned buffer to a screen again.
//
// If the screen implements a [CloneArea] method, it will be called instead of
// cloning the area manually.
func CloneArea(scr uv.Screen, area uv.Rectangle) *uv.Buffer {
	if c, ok := scr.(interface {
		CloneArea(area uv.Rectangle) *uv.Buffer
	}); ok {
		return c.CloneArea(area)
	}
	buf := uv.NewBuffer(area.Dx(), area.Dy())
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			cell := scr.CellAt(x, y)
			if cell == nil || cell.IsZero() {
				continue
			}
			buf.SetCell(x-area.Min.X, y-area.Min.Y, cell.Clone())
		}
	}
	return buf
}

// Clone creates a new [uv.Buffer] clone of the given screen. The new buffer will
// have the same size as the screen and will contain the same cells.
// Use [uv.Buffer.Draw] to draw the cloned buffer to a screen again.
//
// If the screen implements a [Clone] method, it will be called instead of
// cloning the entire screen manually.
func Clone(scr uv.Screen) *uv.Buffer {
	if c, ok := scr.(interface {
		Clone() *uv.Buffer
	}); ok {
		return c.Clone()
	}
	return CloneArea(scr, scr.Bounds())
}
