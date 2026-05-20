// Command example demonstrates the braille progress meter by animating it
// from 0% to 100% in single-dot increments.
package main

import (
	"fmt"
	"time"

	"github.com/charmbracelet/crush/internal/ui/progress"
)

func main() {
	const width = 30
	steps := width * 8
	for i := 0; i <= steps; i++ {
		pct := float64(i) / float64(steps)
		fmt.Printf("\r%5.1f%% [%s]", pct*100, progress.Render(width, pct))
		time.Sleep(20 * time.Millisecond)
	}
	fmt.Println()
}
