package replacer

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Decision represents the replacement agent's decision.
type Decision struct {
	Action string `json:"action"` // "stop" or "continue"
	Prompt string `json:"prompt"` // follow-up prompt if action is "continue"
}

// ParseDecision parses the replacement agent's raw response into a Decision.
func ParseDecision(raw string) (*Decision, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("replacer returned empty response")
	}

	var d Decision
	if err := json.Unmarshal([]byte(trimmed), &d); err != nil {
		// Try extracting from markdown fences.
		if start := strings.Index(trimmed, "{"); start != -1 {
			if end := strings.LastIndex(trimmed, "}"); end != -1 && end > start {
				inner := trimmed[start : end+1]
				if err2 := json.Unmarshal([]byte(inner), &d); err2 == nil {
					return &d, nil
				}
			}
		}
		return nil, fmt.Errorf("failed to parse replacer decision: %w", err)
	}

	if d.Action != "stop" && d.Action != "continue" {
		return nil, fmt.Errorf("invalid replacer action: %q", d.Action)
	}

	return &d, nil
}
