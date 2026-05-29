package styles

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/exp/charmtone"
)

// SubagentDot returns a colored "●" string for the given palette color name.
// Recognized names are the eight subagent palette colors (red, orange, yellow,
// green, cyan, blue, purple, pink). Unrecognized names return a plain "●" with
// no styling applied.
func SubagentDot(color string) string {
	var fg charmtone.Key
	switch color {
	case "red":
		fg = charmtone.Cherry
	case "orange":
		fg = charmtone.Tang
	case "yellow":
		fg = charmtone.Citron
	case "green":
		fg = charmtone.Julep
	case "cyan":
		fg = charmtone.Guppy
	case "blue":
		fg = charmtone.Sapphire
	case "purple":
		fg = charmtone.Mauve
	case "pink":
		fg = charmtone.Flamingo
	default:
		return "●"
	}
	return lipgloss.NewStyle().Foreground(fg).SetString("●").String()
}
