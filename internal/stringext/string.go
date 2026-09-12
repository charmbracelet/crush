package stringext

import (
	"encoding/base64"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func Capitalize(text string) string {
	return cases.Title(language.English, cases.Compact).String(text)
}

// NormalizeSpace normalizes whitespace in the given content string.
// It replaces Windows-style line endings with Unix-style line endings,
// converts tabs to four spaces, and trims leading and trailing newlines.
// Per-line indentation is preserved: trimming spaces would eat the first
// line's leading whitespace and corrupt indentation in code previews.
func NormalizeSpace(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\t", "    ")
	content = strings.Trim(content, "\n")
	return content
}

// IsValidBase64 reports whether s is canonical base64 under standard
// encoding (RFC 4648). It requires that s round-trips through
// decode/encode unchanged — rejecting whitespace, missing padding,
// and other leniencies that DecodeString alone would accept.
func IsValidBase64(s string) bool {
	if s == "" {
		return false
	}
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return false
	}
	// Round-trip check rejects whitespace, missing padding, and other
	// leniencies that DecodeString silently accepts.
	return base64.StdEncoding.EncodeToString(decoded) == s
}

// CutANSISafeLeft returns a cut point ≤ n on a rune boundary that
// also does not land inside an ANSI escape sequence.
func CutANSISafeLeft(s string, n int) int {
	n = CutRuneBoundaryLeft(s, n)
	if esc := strings.LastIndex(s[:n], "\x1b"); esc >= 0 && !ansiTerminated(s[esc:n]) {
		n = esc
	}
	return n
}

// CutANSISafeRight returns a start ≥ the requested index on a rune
// boundary that also does not land inside an ANSI escape sequence — an
// escape straddling the cut is skipped: the tail begins after its
// terminator, or at end of string if it never terminates.
func CutANSISafeRight(s string, start int) int {
	start = CutRuneBoundaryRight(s, start)
	esc := strings.LastIndex(s[:start], "\x1b")
	if esc < 0 || ansiTerminated(s[esc:start]) {
		return start
	}
	if end := ansiEnd(s[esc:]); end >= 0 {
		return esc + end
	}
	return len(s)
}

// CutRuneBoundaryLeft backs n off to a rune boundary so s[:n] never
// splits a multi-byte UTF-8 rune.
func CutRuneBoundaryLeft(s string, n int) int {
	if n > len(s) {
		n = len(s)
	}
	for n > 0 && n < len(s) && s[n]&0xC0 == 0x80 {
		n--
	}
	return n
}

// CutRuneBoundaryRight advances start to a rune boundary so s[start:]
// never splits a multi-byte UTF-8 rune.
func CutRuneBoundaryRight(s string, start int) int {
	if start < 0 {
		start = 0
	}
	for start < len(s) && s[start]&0xC0 == 0x80 {
		start++
	}
	return start
}

// ansiTerminated reports whether the escape sequence beginning at
// seq[0] (ESC) has a terminator within len(seq).
func ansiTerminated(seq string) bool {
	return ansiEnd(seq) > 0
}

// ansiEnd returns the length of the escape sequence beginning at
// seq[0] (ESC), or -1 when it never terminates within seq. Covers CSI
// (\x1b[ … final @-~), OSC-like string sequences (\x1b] \x1bP \x1bX
// \x1b^ \x1b_ … BEL or ST), and fixed-width escapes.
func ansiEnd(seq string) int {
	if len(seq) < 2 {
		return -1
	}
	switch seq[1] {
	case '[': // CSI — first byte in @-~ ends it.
		for i := 2; i < len(seq); i++ {
			if seq[i] >= 0x40 && seq[i] <= 0x7E {
				return i + 1
			}
		}
		return -1
	case ']', 'P', 'X', '^', '_': // String sequences — BEL or ST.
		for i := 2; i < len(seq); i++ {
			if seq[i] == '\a' {
				return i + 1
			}
			if seq[i] == '\\' && seq[i-1] == '\x1b' {
				return i + 1
			}
		}
		return -1
	case '(', ')', '*', '+', '#', '%': // Charset selects — 3 bytes.
		if len(seq) >= 3 {
			return 3
		}
		return -1
	default: // Two-byte escapes (\x1bM, \x1b7, …).
		return 2
	}
}
