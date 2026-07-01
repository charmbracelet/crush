package model

import (
	"unicode"
)

// cjkCharsPerToken is the approximate ratio of characters to tokens for CJK
// scripts, where each character is roughly one token.
const cjkCharsPerToken = 1.0

// latinCharsPerToken is the approximate ratio of characters to tokens for
// Latin and other non-CJK scripts (roughly 4 chars per token for English).
const latinCharsPerToken = 1.0 / 4.0

// isCJKRune returns true if the rune belongs to a CJK script (Chinese,
// Japanese, Korean) where characters map roughly 1:1 to tokens.
func isCJKRune(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r) ||
		unicode.Is(unicode.Hangul, r)
}

// estimateTextTokens returns a rough token count for the given text using a
// script-aware heuristic: CJK characters count as ~1 token each, while
// Latin and other scripts use the standard ~4 chars/token ratio. This is
// more accurate than a flat chars/4 estimate for multilingual content.
func estimateTextTokens(text string) int {
	if text == "" {
		return 0
	}
	var cjkTokens, latinChars int
	for _, r := range text {
		if isCJKRune(r) {
			cjkTokens++
		} else {
			latinChars++
		}
	}
	latinTokens := float64(latinChars) * latinCharsPerToken
	return cjkTokens + int(latinTokens+0.5)
}
