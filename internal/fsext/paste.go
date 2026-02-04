package fsext

import (
	"os"
	"strings"
)

func ParsePastedFiles(s string) []string {
	s = strings.TrimSpace(s)

	// NOTE: Rio on Windows adds NULL chars for some reason.
	s = strings.ReplaceAll(s, "\x00", "")

	switch {
	case attemptStat(s):
		return strings.Split(s, "\n")
	case os.Getenv("WT_SESSION") != "":
		return windowsTerminalParsePastedFiles(s)
	default:
		return unixParsePastedFiles(s)
	}
}

func attemptStat(s string) bool {
	for path := range strings.SplitSeq(s, "\n") {
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			return false
		}
	}
	return true
}

func windowsTerminalParsePastedFiles(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}

	var (
		paths    []string
		current  strings.Builder
		inQuotes = false
	)
	for i := range len(s) {
		ch := s[i]

		switch {
		case ch == '"':
			if inQuotes {
				// End of quoted section
				if current.Len() > 0 {
					paths = append(paths, current.String())
					current.Reset()
				}
				inQuotes = false
			} else {
				// Start of quoted section
				inQuotes = true
			}
		case inQuotes:
			current.WriteByte(ch)
		case ch != ' ':
			// Text outside quotes is not allowed
			return nil
		}
	}

	// Add any remaining content if quotes were properly closed
	if current.Len() > 0 && !inQuotes {
		paths = append(paths, current.String())
	}

	// If quotes were not closed, return empty (malformed input)
	if inQuotes {
		return nil
	}

	return paths
}

func unixParsePastedFiles(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}

	var (
		paths   []string
		current strings.Builder
		escaped = false
	)
	for i := range len(s) {
		ch := s[i]

		switch {
		case escaped:
			// After a backslash, add the character as-is (including space)
			current.WriteByte(ch)
			escaped = false
		case ch == '\\':
			// Check if this backslash is at the end of the string
			if i == len(s)-1 {
				// Trailing backslash, treat as literal
				current.WriteByte(ch)
			} else {
				// Start of escape sequence
				escaped = true
			}
		case ch == ' ':
			// Space separates paths (unless escaped)
			if current.Len() > 0 {
				paths = append(paths, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(ch)
		}
	}

	// Handle trailing backslash if present
	if escaped {
		current.WriteByte('\\')
	}

	// Add the last path if any
	if current.Len() > 0 {
		paths = append(paths, current.String())
	}

	return paths
}
