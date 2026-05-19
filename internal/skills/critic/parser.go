package critic

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var (
	// jsonFenceRE matches markdown JSON fences and captures the inner content.
	jsonFenceRE = regexp.MustCompile(`(?s)` + "```(?:json)?\\s*\\n?(.*?)\\n?" + "```")
	// trailingCommaRE removes trailing commas before closing braces/brackets.
	trailingCommaRE = regexp.MustCompile(`,\s*([}\]])`)
)

// ParseFeedback extracts CriticFeedback from raw LLM output.
//
// Strategy:
//  1. Try direct json.Unmarshal.
//  2. Try regex extraction from markdown fences.
//  3. Try JSON repair (strip trailing commas, fix quotes).
//  4. Return structured error so caller can retry or fallback.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "... [truncated]"
}

func ParseFeedback(raw string) (*CriticFeedback, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("critic returned empty response")
	}

	// Attempt 1: direct unmarshal.
	var direct CriticFeedback
	if err := json.Unmarshal([]byte(trimmed), &direct); err == nil {
		return &direct, nil
	}

	// Attempt 2: extract from markdown fences.
	if matches := jsonFenceRE.FindStringSubmatch(trimmed); len(matches) > 1 {
		inner := strings.TrimSpace(matches[1])
		if err := json.Unmarshal([]byte(inner), &direct); err == nil {
			return &direct, nil
		}
	}

	// Attempt 3: repair trailing commas.
	repaired := trailingCommaRE.ReplaceAllString(trimmed, "$1")
	if err := json.Unmarshal([]byte(repaired), &direct); err == nil {
		return &direct, nil
	}

	// Attempt 4: repair inside fences if present.
	if matches := jsonFenceRE.FindStringSubmatch(trimmed); len(matches) > 1 {
		inner := trailingCommaRE.ReplaceAllString(strings.TrimSpace(matches[1]), "$1")
		if err := json.Unmarshal([]byte(inner), &direct); err == nil {
			return &direct, nil
		}
	}

	return nil, fmt.Errorf("failed to parse critic feedback after all fallback strategies: raw=%s", truncate(raw, 500))
}
