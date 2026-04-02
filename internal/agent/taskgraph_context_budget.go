package agent

import (
	"fmt"
	"strings"
)

const (
	taskGraphMailboxMessagesLimit      = 6
	taskGraphMailboxPromptCharsLimit   = 1_800
	taskGraphOutputPerTaskCharsLimit   = 20_000
	taskGraphOutputAggregateCharsLimit = 80_000
	taskGraphReducerMessageCharsLimit  = 280
	taskGraphTodoContentCharsLimit     = 240
	taskGraphTodoNodeContentCharsLimit = 120
	taskGraphTodoMailboxCharsLimit     = 80
	subAgentResponseCharsLimit         = 30_000
)

func taskGraphCompactText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return strings.Join(strings.Fields(value), " ")
}

func taskGraphEllipsize(value string, maxRunes int) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	if maxRunes <= 0 {
		return "", true
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value, false
	}
	if maxRunes == 1 {
		return "…", true
	}
	return string(runes[:maxRunes-1]) + "…", true
}

func taskGraphPromptWithMailboxMessages(basePrompt string, messages []string) string {
	base := strings.TrimSpace(basePrompt)
	if base == "" {
		base = "Continue with the assigned task."
	}
	if len(messages) == 0 {
		return base
	}

	start := max(0, len(messages)-taskGraphMailboxMessagesLimit)
	selected := make([]string, 0, len(messages)-start+1)
	used := 0
	for _, raw := range messages[start:] {
		msg := taskGraphCompactText(raw)
		if msg == "" {
			continue
		}
		runeLen := len([]rune(msg))
		if used+runeLen > taskGraphMailboxPromptCharsLimit {
			remaining := taskGraphMailboxPromptCharsLimit - used
			if remaining <= 0 {
				break
			}
			trimmed, _ := taskGraphEllipsize(msg, remaining)
			if trimmed != "" {
				selected = append(selected, trimmed)
			}
			used = taskGraphMailboxPromptCharsLimit
			break
		}
		selected = append(selected, msg)
		used += runeLen
	}

	if len(selected) == 0 {
		return base
	}
	if omitted := start; omitted > 0 {
		selected = append(selected, fmt.Sprintf("… %d earlier mailbox message(s) omitted.", omitted))
	}
	if omitted := len(messages) - start - len(selected); omitted > 0 {
		selected = append(selected, fmt.Sprintf("… %d mailbox message(s) omitted due to context budget.", omitted))
	}
	return base + "\n\nMailbox messages:\n- " + strings.Join(selected, "\n- ")
}

func taskGraphModelSafeSubAgentText(content, sessionID string) string {
	content = taskGraphCompactText(content)
	if content == "" {
		return ""
	}
	trimmed, truncated := taskGraphEllipsize(content, subAgentResponseCharsLimit)
	if !truncated {
		return trimmed
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return trimmed + " [subagent output truncated; inspect child session for full details]"
	}
	return fmt.Sprintf("%s [subagent output truncated; inspect child session %s for full details]", trimmed, sessionID)
}
