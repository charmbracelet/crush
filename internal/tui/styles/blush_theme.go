// Package styles provides styling for the Blush TUI.
package styles

import (
	"fmt"
	"github.com/charmbracelet/lipgloss/v2"
	"image/color"
)

// colorToHex converts a color.Color to a hex string
func colorToHex(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", uint8(r>>8), uint8(g>>8), uint8(b>>8))
}

// Define our custom blush colors
var (
	blushPink      = color.RGBA{R: 255, G: 182, B: 193, A: 255} // Light pink
	blushPurple    = color.RGBA{R: 147, G: 112, B: 219, A: 255} // Medium purple
	blushLavender  = color.RGBA{R: 230, G: 230, B: 250, A: 255} // Lavender
	blushMagenta   = color.RGBA{R: 255, G: 0, B: 255, A: 255}   // Magenta
	blushOffWhite  = color.RGBA{R: 250, G: 240, B: 245, A: 255} // Off-white
	blushGray      = color.RGBA{R: 180, G: 180, B: 190, A: 255} // Gray
	blushCharcoal  = color.RGBA{R: 40, G: 30, B: 35, A: 255}    // Charcoal
	blushSuccess   = color.RGBA{R: 144, G: 238, B: 144, A: 255} // Light green
	blushError     = color.RGBA{R: 255, G: 99, B: 71, A: 255}   // Tomato red
	blushWarning   = color.RGBA{R: 255, G: 165, B: 0, A: 255}   // Orange
	blushInfo      = color.RGBA{R: 100, G: 149, B: 237, A: 255} // Cornflower blue
)

// NewBlushTheme creates a new theme with distinctive Blush branding colors.
func NewBlushTheme() *Theme {
	t := &Theme{
		Name:   "blush",
		IsDark: true,

		// Primary blush branding colors
		Primary:   blushPink,     // Main pink brand color
		Secondary: blushPurple,   // Secondary purple brand color
		Tertiary:  blushLavender, // Tertiary lavender brand color
		Accent:    blushMagenta,  // Accent magenta color

		// Backgrounds
		BgBase:        blushCharcoal,
		BgBaseLighter: color.RGBA{R: 80, G: 80, B: 90, A: 255},  // Dark gray
		BgSubtle:      blushGray,
		BgOverlay:     color.RGBA{R: 220, G: 220, B: 230, A: 255}, // Light gray

		// Foregrounds
		FgBase:      blushOffWhite,
		FgMuted:     blushGray,
		FgHalfMuted: color.RGBA{R: 220, G: 220, B: 230, A: 255}, // Light gray
		FgSubtle:    color.RGBA{R: 255, G: 253, B: 208, A: 255}, // Cream
		FgSelected:  blushOffWhite,

		// Borders
		Border:      blushGray,
		BorderFocus: blushPink,

		// Status
		Success: blushSuccess,
		Error:   blushError,
		Warning: blushWarning,
		Info:    blushInfo,

		// Colors
		White: blushOffWhite,

		BlueLight: blushInfo,
		Blue:      color.RGBA{R: 65, G: 105, B: 225, A: 255}, // Royal blue

		Yellow: blushWarning,
		Citron: color.RGBA{R: 228, G: 208, B: 10, A: 255}, // Citron

		Green:      blushSuccess,
		GreenDark:  color.RGBA{R: 0, G: 100, B: 0, A: 255},    // Dark green
		GreenLight: color.RGBA{R: 144, G: 238, B: 144, A: 255}, // Light green

		Red:      blushError,
		RedDark:  color.RGBA{R: 139, G: 0, B: 0, A: 255},   // Dark red
		RedLight: color.RGBA{R: 255, G: 99, B: 71, A: 255}, // Light red
		Cherry:   color.RGBA{R: 222, G: 49, B: 99, A: 255}, // Cherry
	}

	// Text selection
	t.TextSelection = lipgloss.NewStyle().Foreground(blushOffWhite).Background(blushPink)

	// LSP and MCP status indicators
	t.ItemOfflineIcon = lipgloss.NewStyle().Foreground(blushGray).SetString("●")
	t.ItemBusyIcon = t.ItemOfflineIcon.Foreground(blushWarning)
	t.ItemErrorIcon = t.ItemOfflineIcon.Foreground(blushError)
	t.ItemOnlineIcon = t.ItemOfflineIcon.Foreground(blushSuccess)

	// Editor: Yolo Mode
	t.YoloIconFocused = lipgloss.NewStyle().Foreground(blushCharcoal).Background(blushWarning).Bold(true).SetString(" ! ")
	t.YoloIconBlurred = t.YoloIconFocused.Foreground(blushOffWhite).Background(blushGray)
	t.YoloDotsFocused = lipgloss.NewStyle().Foreground(blushPink).SetString(":::")
	t.YoloDotsBlurred = t.YoloDotsFocused.Foreground(blushGray)

	return t
}