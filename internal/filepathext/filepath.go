package filepathext

import (
	"path/filepath"
	"runtime"
	"strings"
	"unicode"
)

// SmartJoin joins two paths, treating the second path as absolute if it is an
// absolute path.
func SmartJoin(one, two string) string {
	two = Normalize(two)
	if SmartIsAbs(two) {
		return two
	}
	return filepath.Join(one, two)
}

// SmartIsAbs checks if a path is absolute, considering both OS-specific and
// Unix-style paths.
func SmartIsAbs(path string) bool {
	path = Normalize(path)
	switch runtime.GOOS {
	case "windows":
		return filepath.IsAbs(path) || strings.HasPrefix(filepath.ToSlash(path), "/")
	default:
		return filepath.IsAbs(path)
	}
}

// Normalize normalizes a path string for the current platform.
func Normalize(path string) string {
	if runtime.GOOS == "windows" {
		return normalizeWindowsDrivePath(path)
	}
	return path
}

func normalizeWindowsDrivePath(path string) string {
	if len(path) < 3 || path[0] != '/' {
		return path
	}
	drive := rune(path[1])
	if !unicode.IsLetter(drive) || path[2] != ':' {
		return path
	}
	if len(path) == 3 || path[3] == '/' || path[3] == '\\' {
		return path[1:]
	}
	return path
}
