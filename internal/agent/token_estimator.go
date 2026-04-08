package agent

import (
	"regexp"
	"strings"
	"unicode"
)

// MessageContent represents a message for token counting
type MessageContent struct {
	Role    string
	Content string
}

// tokenEstimator provides accurate token estimation
type tokenEstimator struct {
	asciiPattern      *regexp.Regexp
	cjkPattern        *regexp.Regexp
	codePattern       *regexp.Regexp
	whitespacePattern *regexp.Regexp
}

// newTokenEstimator creates a new estimator
func newTokenEstimator() *tokenEstimator {
	return &tokenEstimator{
		asciiPattern:      regexp.MustCompile(`[\x00-\x7F]+`),
		cjkPattern:        regexp.MustCompile(`[\p{Han}\p{Hiragana}\p{Katakana}\p{Hangul}]+`),
		codePattern:       regexp.MustCompile(`[a-zA-Z0-9_+\-*/=<>!&|(),.;:?[\]{}']+`),
		whitespacePattern: regexp.MustCompile(`\s+`),
	}
}

// EstimateTokens estimates token count for text
// Uses mixed strategy:
// - ASCII English: ~4 chars ≈ 1 token
// - CJK (Chinese/Japanese/Korean): ~1-2 chars ≈ 1 token
// - Code: ~4 chars ≈ 1 token
// - Mixed: weighted average
func (e *tokenEstimator) EstimateTokens(text string) int {
	if text == "" {
		return 0
	}

	// Fast path: pure ASCII (but not pure whitespace)
	if isASCII(text) {
		// Strip leading/trailing whitespace for token calculation
		stripped := strings.TrimLeft(text, " \t\n\r")
		if stripped == "" {
			return 0 // Pure whitespace = 0 tokens
		}
		// For ASCII with content, count all chars (including internal spaces)
		// because in real text, spaces do consume tokens
		return (len(text) + 3) / 4
	}

	runes := []rune(text)
	totalTokens := 0

	for i := 0; i < len(runes); {
		r := runes[i]
		cp := int(r)

		switch {
		case cp >= 0x00 && cp <= 0x7F:
			start := i
			for i < len(runes) && runes[i] <= 0x7F {
				i++
			}
			asciiLen := i - start
			totalTokens += (asciiLen + 3) / 4

		case unicode.Is(unicode.Han, r):
			totalTokens++
			i++

		case unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r):
			totalTokens++
			i++

		case unicode.Is(unicode.Hangul, r):
			totalTokens++
			i++

		case cp >= 0x1F300 && cp <= 0x1F9FF:
			totalTokens += 2
			i++

		default:
			if !unicode.IsSpace(r) {
				totalTokens++
			}
			i++
		}
	}

	return totalTokens
}

// EstimateMessagesTokens estimates total tokens for messages
func (e *tokenEstimator) EstimateMessagesTokens(messages []MessageContent) int {
	total := 0

	for _, msg := range messages {
		total += 4 // Role overhead
		total += e.EstimateTokens(msg.Content)
	}

	return total
}

// isASCII checks if text is pure ASCII
func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 0x7F {
			return false
		}
	}
	return true
}

// CalculateSafeBudget calculates safe context budget considering estimation errors
func CalculateSafeBudget(contextWindow int, errorMargin float64) int {
	if errorMargin <= 0 {
		errorMargin = 0.20
	}

	// Account for 20% estimation error + 10% safety buffer
	// e.g., 200000 context, 20% error → ~144000 safe budget
	safeWindow := float64(contextWindow) * (1.0 - errorMargin)
	return int(safeWindow * 0.90)
}
