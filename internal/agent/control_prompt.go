package agent

import "strings"

// IsExplicitCancelPrompt identifies a standalone user control message. It is
// intentionally narrow so task prompts such as "stop the service" still reach
// the model.
func IsExplicitCancelPrompt(prompt string) bool {
	normalized := strings.ToLower(strings.TrimSpace(prompt))
	normalized = strings.Trim(normalized, ".!?")
	normalized = strings.Join(strings.Fields(normalized), " ")

	switch normalized {
	case "stop", "stop now", "stop please",
		"cancel", "cancel it", "cancel this", "cancel now",
		"cancel current run", "cancel the current run",
		"interrupt", "interrupt current run", "interrupt the current run",
		"abort", "abort current run", "abort the current run":
		return true
	default:
		return false
	}
}
