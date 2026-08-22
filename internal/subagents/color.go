package subagents

import (
	"hash/fnv"
	"slices"
)

// Color name constants for the eight-color subagent palette.
const (
	ColorRed    = "red"
	ColorOrange = "orange"
	ColorYellow = "yellow"
	ColorGreen  = "green"
	ColorCyan   = "cyan"
	ColorBlue   = "blue"
	ColorPurple = "purple"
	ColorPink   = "pink"
)

// colorPalette is the ordered list of all eight valid color names.
var colorPalette = [8]string{
	ColorRed,
	ColorOrange,
	ColorYellow,
	ColorGreen,
	ColorCyan,
	ColorBlue,
	ColorPurple,
	ColorPink,
}

// IsValidColor reports whether color is one of the eight defined palette names.
func IsValidColor(color string) bool {
	return slices.Contains(colorPalette[:], color)
}

// AutoColor deterministically maps name to one of the palette colors using
// FNV-32a hashing modulo the palette size.
func AutoColor(name string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return colorPalette[h.Sum32()%uint32(len(colorPalette))]
}
