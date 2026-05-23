package toolcoach

import "strings"

// jsonpeek extracts the string value of key from a flat JSON object without
// allocating heap memory. It handles simple escaped quotes (\") but does not
// support nested objects, arrays, or unicode escape sequences. This is a fast
// path for the small flat objects that Crush tool inputs use.
//
// The returned string is a slice of the input, so no bytes are copied.
func jsonpeek(input, key string) (string, bool) {
	keyQuoted := `"` + key + `"`
	idx := strings.Index(input, keyQuoted)
	if idx < 0 {
		return "", false
	}

	i := idx + len(keyQuoted)

	// Skip whitespace and colon.
	for i < len(input) {
		c := input[i]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			i++
			continue
		}
		break
	}
	if i >= len(input) || input[i] != ':' {
		return "", false
	}
	i++ // skip colon

	// Skip whitespace.
	for i < len(input) {
		c := input[i]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			i++
			continue
		}
		break
	}
	if i >= len(input) || input[i] != '"' {
		return "", false
	}
	i++ // skip opening quote

	start := i
	for i < len(input) {
		if input[i] == '\\' && i+1 < len(input) {
			i += 2
			continue
		}
		if input[i] == '"' {
			return input[start:i], true
		}
		i++
	}

	return "", false
}
