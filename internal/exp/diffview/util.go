package diffview

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func pad(v any, width int) string {
	s := fmt.Sprintf("%v", v)
	w := ansi.StringWidth(s)
	if w >= width {
		return s
	}
	return strings.Repeat(" ", width-w) + s
}
