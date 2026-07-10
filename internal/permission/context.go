package permission

import (
	"path/filepath"
	"strings"
)

// tokenSatisfies reports whether a stored permission token is broad enough to
// satisfy a request for a candidate token.
//
// Matching rules by token kind:
//
//   - command:<A> satisfies command:<B> iff B == A (exact) or B starts with
//     A followed by a space. This allows command:go to satisfy command:go
//     test but not command:golang, because the word boundary (space) prevents
//     partial-word prefix matches.
//
//   - path:<A> satisfies path:<B> iff B == A (exact) or B starts with A
//     followed by the OS path separator. This allows path:/tmp to satisfy
//     path:/tmp/subdir but not path:/tmpfiles.
//
//   - All other tokens (including opaque command!: tokens): exact match only.
func tokenSatisfies(stored, candidate string) bool {
	if stored == candidate {
		return true
	}
	switch {
	case strings.HasPrefix(stored, "command:") && strings.HasPrefix(candidate, "command:"):
		s := strings.TrimPrefix(stored, "command:")
		c := strings.TrimPrefix(candidate, "command:")
		// Word-boundary prefix: stored "go" satisfies "go test" but not "golang".
		return strings.HasPrefix(c, s+" ")
	case strings.HasPrefix(stored, "path:") && strings.HasPrefix(candidate, "path:"):
		s := filepath.Clean(strings.TrimPrefix(stored, "path:"))
		c := filepath.Clean(strings.TrimPrefix(candidate, "path:"))
		// Root path is a parent of every absolute path.
		if s == "/" {
			return strings.HasPrefix(c, "/")
		}
		// Directory-boundary prefix: stored "/tmp" satisfies "/tmp/subdir" but not "/tmpfiles".
		return strings.HasPrefix(c, s+string(filepath.Separator))
	default:
		return false
	}
}
