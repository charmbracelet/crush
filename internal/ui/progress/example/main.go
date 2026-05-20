// Command example demonstrates the braille progress meter by animating it
// from 0% to 100% in single-dot increments.
package main

import (
	"fmt"
	"image/color"
	"time"

	"github.com/charmbracelet/crush/internal/ui/progress"
)

func main() {
	const width = 30
	fg := color.RGBA{0x7f, 0xff, 0xa0, 0xff} // green
	bg := color.RGBA{0x20, 0x20, 0x20, 0xff} // dark gray
	steps := width * 8
	for i := 0; i <= steps; i++ {
		pct := float64(i) / float64(steps)
		fmt.Printf("\r%5.1f%% %s", pct*100, progress.Render(width, pct, fg, bg))
		time.Sleep(20 * time.Millisecond)
	}
	fmt.Println()
}
