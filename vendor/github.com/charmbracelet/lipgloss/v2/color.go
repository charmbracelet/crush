package lipgloss

import (
	"image/color"
	"strconv"
	"strings"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/lucasb-eyer/go-colorful"
)

// 4-bit color constants.
const (
	Black ansi.BasicColor = iota
	Red
	Green
	Yellow
	Blue
	Magenta
	Cyan
	White

	BrightBlack
	BrightRed
	BrightGreen
	BrightYellow
	BrightBlue
	BrightMagenta
	BrightCyan
	BrightWhite
)

var noColor = NoColor{}

// NoColor is used to specify the absence of color styling. When this is active
// foreground colors will be rendered with the terminal's default text color,
// and background colors will not be drawn at all.
//
// Example usage:
//
//	var style = someStyle.Background(lipgloss.NoColor{})
type NoColor struct{}

// RGBA returns the RGBA value of this color. Because we have to return
// something, despite this color being the absence of color, we're returning
// black with 100% opacity.
//
// Red: 0x0, Green: 0x0, Blue: 0x0, Alpha: 0xFFFF.
func (n NoColor) RGBA() (r, g, b, a uint32) {
	return 0x0, 0x0, 0x0, 0xFFFF //nolint:mnd
}

// Color specifies a color by hex or ANSI256 value. For example:
//
//	ansiColor := lipgloss.Color("1") // The same as lipgloss.Red
//	ansi256Color := lipgloss.Color("21")
//	hexColor := lipgloss.Color("#0000ff")
func Color(s string) color.Color {
	if strings.HasPrefix(s, "#") {
		hex, err := colorful.Hex(s)
		if err != nil {
			return noColor
		}
		return hex
	}

	i, err := strconv.Atoi(s)
	if err != nil {
		return noColor
	}

	if i < 0 {
		// Only positive numbers
		i = -i
	}

	if i < 16 {
		return ansi.BasicColor(i) //nolint:gosec
	} else if i < 256 {
		return ANSIColor(i) //nolint:gosec
	}

	return ansi.TrueColor(i) //nolint:gosec
}

// RGBColor is a color specified by red, green, and blue values.
type RGBColor struct {
	R uint8
	G uint8
	B uint8
}

// RGBA returns the RGBA value of this color. This satisfies the Go Color
// interface.
func (c RGBColor) RGBA() (r, g, b, a uint32) {
	const shift = 8
	r |= uint32(c.R) << shift
	g |= uint32(c.G) << shift
	b |= uint32(c.B) << shift
	a = 0xFFFF
	return
}

// ANSIColor is a color specified by an ANSI256 color value.
//
// Example usage:
//
//	colorA := lipgloss.ANSIColor(8)
//	colorB := lipgloss.ANSIColor(134)
type ANSIColor = ansi.ExtendedColor

// LightDarkFunc is a function that returns a color based on whether the
// terminal has a light or dark background. You can create one of these with
// [LightDark].
//
// Example:
//
//	lightDark := lipgloss.LightDark(hasDarkBackground)
//	red, blue := lipgloss.Color("#ff0000"), lipgloss.Color("#0000ff")
//	myHotColor := lightDark(red, blue)
//
// For more info see [LightDark].
type LightDarkFunc func(light, dark color.Color) color.Color

// LightDark is a simple helper type that can be used to choose the appropriate
// color based on whether the terminal has a light or dark background.
//
//	lightDark := lipgloss.LightDark(hasDarkBackground)
//	red, blue := lipgloss.Color("#ff0000"), lipgloss.Color("#0000ff")
//	myHotColor := lightDark(red, blue)
//
// In practice, there are slightly different workflows between Bubble Tea and
// Lip Gloss standalone.
//
// In Bubble Tea, listen for tea.BackgroundColorMsg, which automatically
// flows through Update on start. This message will be received whenever the
// background color changes:
//
//	case tea.BackgroundColorMsg:
//	    m.hasDarkBackground = msg.IsDark()
//
// Later, when you're rendering use:
//
//	lightDark := lipgloss.LightDark(m.hasDarkBackground)
//	red, blue := lipgloss.Color("#ff0000"), lipgloss.Color("#0000ff")
//	myHotColor := lightDark(red, blue)
//
// In standalone Lip Gloss, the workflow is simpler:
//
//	hasDarkBG := lipgloss.HasDarkBackground(os.Stdin, os.Stdout)
//	lightDark := lipgloss.LightDark(hasDarkBG)
//	red, blue := lipgloss.Color("#ff0000"), lipgloss.Color("#0000ff")
//	myHotColor := lightDark(red, blue)
func LightDark(isDark bool) LightDarkFunc {
	return func(light, dark color.Color) color.Color {
		if isDark {
			return dark
		}
		return light
	}
}

// isDarkColor returns whether the given color is dark (based on the luminance
// portion of the color as interpreted as HSL).
//
// Example usage:
//
//	color := lipgloss.Color("#0000ff")
//	if lipgloss.isDarkColor(color) {
//		fmt.Println("It's dark! I love darkness!")
//	} else {
//		fmt.Println("It's light! Cover your eyes!")
//	}
func isDarkColor(c color.Color) bool {
	col, ok := colorful.MakeColor(c)
	if !ok {
		return true
	}

	_, _, l := col.Hsl()
	return l < 0.5 //nolint:mnd
}

// CompleteFunc is a function that returns the appropriate color based on the
// given color profile.
//
// Example usage:
//
//	p := colorprofile.Detect(os.Stderr, os.Environ())
//	complete := lipgloss.Complete(p)
//	color := complete(
//		lipgloss.Color(1), // ANSI
//		lipgloss.Color(124), // ANSI256
//		lipgloss.Color("#ff34ac"), // TrueColor
//	)
//	fmt.Println("Ooh, pretty color: ", color)
//
// For more info see [Complete].
type CompleteFunc func(ansi, ansi256, truecolor color.Color) color.Color

// Complete returns a function that will return the appropriate color based on
// the given color profile.
//
// Example usage:
//
//	p := colorprofile.Detect(os.Stderr, os.Environ())
//	complete := lipgloss.Complete(p)
//	color := complete(
//	    lipgloss.Color(1), // ANSI
//	    lipgloss.Color(124), // ANSI256
//	    lipgloss.Color("#ff34ac"), // TrueColor
//	)
//	fmt.Println("Ooh, pretty color: ", color)
func Complete(p colorprofile.Profile) CompleteFunc {
	return func(ansi, ansi256, truecolor color.Color) color.Color {
		switch p { //nolint:exhaustive
		case colorprofile.ANSI:
			return ansi
		case colorprofile.ANSI256:
			return ansi256
		case colorprofile.TrueColor:
			return truecolor
		}
		return noColor
	}
}
