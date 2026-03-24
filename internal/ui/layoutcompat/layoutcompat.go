// Package layoutcompat provides backward-compatible layout helpers that were
// removed from ultraviolet's layout package.
package layoutcompat

import uv "github.com/charmbracelet/ultraviolet"

// Constraint applies a size constraint to a given dimension.
type Constraint interface {
	Apply(size int) int
}

// Fixed is a constraint that represents a fixed size.
type Fixed int

// Apply returns the fixed size, clamped to the given dimension.
func (f Fixed) Apply(size int) int {
	return min(int(f), size)
}

// SplitVertical splits the area vertically into two parts based on the given
// constraint. It returns the top and bottom rectangles.
func SplitVertical(area uv.Rectangle, constraint Constraint) (top uv.Rectangle, bottom uv.Rectangle) {
	height := min(constraint.Apply(area.Dy()), area.Dy())
	top = uv.Rectangle{Min: area.Min, Max: uv.Position{X: area.Max.X, Y: area.Min.Y + height}}
	bottom = uv.Rectangle{Min: uv.Position{X: area.Min.X, Y: area.Min.Y + height}, Max: area.Max}
	return top, bottom
}

// SplitHorizontal splits the area horizontally into two parts based on the
// given constraint. It returns the left and right rectangles.
func SplitHorizontal(area uv.Rectangle, constraint Constraint) (left uv.Rectangle, right uv.Rectangle) {
	width := min(constraint.Apply(area.Dx()), area.Dx())
	left = uv.Rectangle{Min: area.Min, Max: uv.Position{X: area.Min.X + width, Y: area.Max.Y}}
	right = uv.Rectangle{Min: uv.Position{X: area.Min.X + width, Y: area.Min.Y}, Max: area.Max}
	return left, right
}
