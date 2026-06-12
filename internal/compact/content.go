package compact

import (
	"strings"
	"unicode/utf8"
)

const defaultMaxLen = 200

// Clip truncates text to max characters, preferring word boundaries.
func Clip(text string, max int) string {
	if max <= 0 {
		max = defaultMaxLen
	}
	if len(text) <= max {
		return text
	}
	// Try to cut at a word boundary.
	cut := strings.LastIndexByte(text[:max], ' ')
	end := cut
	if end < max*6/10 {
		end = max
	}
	// Avoid splitting a rune.
	for end > 0 && !utf8.RuneStart(text[end]) {
		end--
	}
	return text[:end]
}

// ClipSentence truncates at the last sentence boundary within max chars.
func ClipSentence(text string, max int) string {
	if max <= 0 {
		max = defaultMaxLen
	}
	if len(text) <= max {
		return text
	}
	window := text[:max]
	lastEnd := -1
	for i := 0; i < len(window); i++ {
		if window[i] == '.' || window[i] == '!' || window[i] == '?' {
			if i+1 < len(window) && (window[i+1] == ' ' || window[i+1] == '\n') {
				lastEnd = i + 1
			}
		}
	}
	if lastEnd >= max*5/10 {
		return text[:lastEnd]
	}
	return Clip(text, max)
}

// FirstLine returns the first line of text, clipped to max chars.
func FirstLine(text string, max int) string {
	if max <= 0 {
		max = defaultMaxLen
	}
	before, _, ok := strings.Cut(text, "\n")
	line := text
	if ok {
		line = before
	}
	return Clip(line, max)
}

// NonEmptyLines splits text into trimmed non-empty lines.
func NonEmptyLines(text string) []string {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}
