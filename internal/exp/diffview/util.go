package diffview

import (
	"fmt"
	"strings"
)

func pad(v any, width int) string {
	s := fmt.Sprintf("%v", v)
	if len(s) >= width {
		return s
	}
	return strings.Repeat(" ", width-len(s)) + s
}
