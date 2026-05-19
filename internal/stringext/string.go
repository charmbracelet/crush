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
// converts tabs to four spaces, and trims leading and trailing whitespace.
func NormalizeSpace(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\t", "    ")
	content = strings.TrimSpace(content)
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
