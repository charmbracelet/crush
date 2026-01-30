package fsext

import (
	"strings"
)

func PasteStringToPaths(s string) []string {
	s = strings.TrimSpace(s)
	paths := strings.Split(s, `" "`)
	if len(paths) > 0 {
		paths[0] = strings.TrimPrefix(paths[0], `"`)
		paths[len(paths)-1] = strings.TrimSuffix(paths[len(paths)-1], `"`)
	}

	return paths
}
