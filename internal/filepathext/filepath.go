package filepathext

import (
	"path/filepath"
	"runtime"
	"strings"
)

// SmartJoin joins two paths, treating the second path as absolute if it is an
// absolute path.
func SmartJoin(one, two string) string {
	if SmartIsAbs(two) {
		return two
	}
	return filepath.Join(one, two)
}

// SmartIsAbs checks if a path is absolute, considering both OS-specific and
// Unix-style paths.
func SmartIsAbs(path string) bool {
	switch runtime.GOOS {
	case "windows":
		return filepath.IsAbs(path) || strings.HasPrefix(filepath.ToSlash(path), "/")
	default:
		return filepath.IsAbs(path)
	}
}

// SplitGlobPrefix splits a glob pattern into the longest leading run of
// literal path segments and the remaining pattern. The prefix contains no
// glob metacharacters, so callers can safely use it as a directory to start
// a walk from. For "internal/agent/*.go" it returns ("internal/agent",
// "*.go"); for "**/foo.go" it returns ("", "**/foo.go").
func SplitGlobPrefix(pattern string) (prefix, rest string) {
	pattern = filepath.ToSlash(pattern)
	segments := strings.Split(pattern, "/")
	var literal []string
	for i, seg := range segments {
		if strings.ContainsAny(seg, "*?[{\\") {
			rest = strings.Join(segments[i:], "/")
			return joinLiteral(literal, pattern), rest
		}
		literal = append(literal, seg)
	}
	// Whole pattern is literal (a plain path); walk its parent and match
	// the basename so the existing match logic still applies.
	if len(literal) == 0 {
		return "", pattern
	}
	parent := joinLiteral(literal[:len(literal)-1], pattern)
	return parent, literal[len(literal)-1]
}

// joinLiteral joins literal segments back into a path. Segments that
// begin at the root keep it: "/*.go" is walked from "/", where the
// pattern points, not from the search path.
func joinLiteral(literal []string, pattern string) string {
	joined := strings.Join(literal, "/")
	if joined == "" && len(literal) > 0 {
		return "/"
	}
	// A Windows drive root needs the slash to stay absolute.
	volume := filepath.VolumeName(pattern)
	if volume != "" && joined == volume && strings.HasPrefix(pattern[len(volume):], "/") {
		return joined + "/"
	}
	return joined
}
