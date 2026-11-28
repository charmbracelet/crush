package paintbrush

import (
	"strings"
	"sync"
)

func (c *Canvas) processResults(results []TaskResult, width, height int) {
	resultIdx := make([][]*TaskResult, height)
	for i := range resultIdx {
		resultIdx[i] = make([]*TaskResult, width)
	}

	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			result := &results[i]
			resultIdx[result.CharY][result.CharX] = result
		}(i)
	}
	wg.Wait()

	var sb strings.Builder
	lastBg := "\033[0m" // Reset background
	lastFg := "\033[0m" // Reset foreground

	for charY := 0; charY < height; charY++ {
		for charX := 0; charX < width; charX++ {
			result := resultIdx[charY][charX]
			if result == nil {
				sb.WriteString(" ")
				continue
			}

			var newBg string
			if result.Bg.A < 0.5 {
				newBg = "\033[0m"
			} else {
				newBg = result.Bg.ToPixel().AnsiBg()
			}
			if newBg != lastBg {
				sb.WriteString(newBg)
				lastBg = newBg
			}

			newFg := result.Fg.ToPixel().AnsiFg()
			if newFg != lastFg {
				sb.WriteString(newFg)
				lastFg = newFg
			}
			sb.WriteString(result.Glyph.UTF8)
		}

		// Reset colors at the end of each line
		sb.WriteString("\033[0m\n")
		lastBg = "\033[0m"
		lastFg = "\033[0m"
	}

	// Remove empty newlines at the end
	c.Result = sb.String()
	for strings.HasSuffix(c.Result, "\n") {
		c.Result = c.Result[:len(c.Result)-1]
	}

	// Generate C string
	c.ResultC = strings.ReplaceAll(c.Result, "\033", "\\033")
	c.ResultC = strings.ReplaceAll(c.ResultC, "\n", "\\n")
	c.ResultC = strings.ReplaceAll(c.ResultC, "\"", "\\\"")
	c.ResultC = "char kCanvas[] = \"" + c.ResultC + "\""

	// Generate Bash string
	c.ResultBash = strings.ReplaceAll(c.Result, "\\", "\\\\")
	c.ResultBash = strings.ReplaceAll(c.ResultBash, "\033", "\\e")
	c.ResultBash = strings.ReplaceAll(c.ResultBash, "\n", "\\n")
	c.ResultBash = strings.ReplaceAll(c.ResultBash, "'", "\\x27")
	c.ResultBash = "echo -ne '" + c.ResultBash + "'"
}
