package styles

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/exp/charmtone"
	"github.com/rivo/uniseg"
)

// RainbowColors defines the color stops for a rainbow gradient using
// charmtone palette colors.
var RainbowColors = []color.Color{
	charmtone.Coral,
	charmtone.Dolly,
	charmtone.Citron,
	charmtone.Julep,
	charmtone.Malibu,
	charmtone.Charple,
	charmtone.Orchid,
}

// ForegroundGrad returns a slice of strings representing the input string
// rendered with a horizontal gradient foreground from color1 to color2. Each
// string in the returned slice corresponds to a grapheme cluster in the input
// string. If bold is true, the rendered strings will be bolded.
func ForegroundGrad(base lipgloss.Style, input string, bold bool, color1, color2 color.Color) []string {
	if input == "" {
		return []string{""}
	}
	if len(input) == 1 {
		style := base.Foreground(color1)
		if bold {
			style.Bold(true)
		}
		return []string{style.Render(input)}
	}
	var clusters []string
	gr := uniseg.NewGraphemes(input)
	for gr.Next() {
		clusters = append(clusters, string(gr.Runes()))
	}

	ramp := lipgloss.Blend1D(len(clusters), color1, color2)
	for i, c := range ramp {
		style := base.Foreground(c)
		if bold {
			style.Bold(true)
		}
		clusters[i] = style.Render(clusters[i])
	}
	return clusters
}

// ApplyForegroundGrad renders a given string with a horizontal gradient
// foreground.
func ApplyForegroundGrad(base lipgloss.Style, input string, color1, color2 color.Color) string {
	if input == "" {
		return ""
	}
	var o strings.Builder
	clusters := ForegroundGrad(base, input, false, color1, color2)
	for _, c := range clusters {
		fmt.Fprint(&o, c)
	}
	return o.String()
}

// ApplyBoldForegroundGrad renders a given string with a horizontal gradient
// foreground.
func ApplyBoldForegroundGrad(base lipgloss.Style, input string, color1, color2 color.Color) string {
	if input == "" {
		return ""
	}
	var o strings.Builder
	clusters := ForegroundGrad(base, input, true, color1, color2)
	for _, c := range clusters {
		fmt.Fprint(&o, c)
	}
	return o.String()
}

// ApplyRainbowGrad renders a string with a multi-stop rainbow gradient
// foreground, interpolating through the provided color stops.
func ApplyRainbowGrad(base lipgloss.Style, input string, stops []color.Color) string {
	if input == "" || len(stops) == 0 {
		return ""
	}
	if len(stops) == 1 {
		return base.Foreground(stops[0]).Render(input)
	}

	var clusters []string
	gr := uniseg.NewGraphemes(input)
	for gr.Next() {
		clusters = append(clusters, string(gr.Runes()))
	}

	n := len(clusters)
	if n == 0 {
		return ""
	}

	var o strings.Builder
	for i, ch := range clusters {
		t := float64(i) / float64(n-1)
		pos := t * float64(len(stops)-1)
		seg := int(pos)
		if seg >= len(stops)-1 {
			seg = len(stops) - 2
		}
		frac := pos - float64(seg)
		ramp := lipgloss.Blend1D(101, stops[seg], stops[seg+1])
		idx := int(frac * 100)
		fmt.Fprint(&o, base.Foreground(ramp[idx]).Render(ch))
	}
	return o.String()
}
