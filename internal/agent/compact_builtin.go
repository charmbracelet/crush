package agent

import (
	"fmt"

	"github.com/charmbracelet/crush/internal/message"
)

const (
	// builtinMicroCompactRecentWindow is the number of recent assistant turns
	// whose reasoning content and binary attachments are preserved during
	// micro-compaction. Older turns have these stripped to reduce token usage.
	builtinMicroCompactRecentWindow = 5

	// builtinAutoCompactRecentWindow is the number of recent assistant turns
	// to keep at full fidelity during auto-compaction. Tool results in older
	// turns are truncated to builtinAutoCompactToolResultMaxChars.
	builtinAutoCompactRecentWindow = 10

	// builtinAutoCompactToolResultMaxChars is the maximum number of characters
	// kept per tool-result during auto-compaction of older turns.
	builtinAutoCompactToolResultMaxChars = 1_000
)

// builtinMicroCompactMessages strips reasoning content and binary attachments
// from older assistant messages to reduce token usage before summarization.
// Messages within the most recent builtinMicroCompactRecentWindow assistant
// turns are left untouched.
func builtinMicroCompactMessages(msgs []message.Message) []message.Message {
	cutoff := assistantTurnCutoff(msgs, builtinMicroCompactRecentWindow)
	if cutoff == 0 {
		return msgs
	}
	changed := false
	result := make([]message.Message, len(msgs))
	copy(result, msgs)
	for i := 0; i < cutoff; i++ {
		newParts := make([]message.ContentPart, 0, len(msgs[i].Parts))
		stripped := false
		for _, part := range msgs[i].Parts {
			switch part.(type) {
			case message.ReasoningContent, message.BinaryContent, message.ImageURLContent:
				stripped = true
				changed = true
			default:
				newParts = append(newParts, part)
			}
		}
		if stripped {
			cloned := msgs[i].Clone()
			cloned.Parts = newParts
			result[i] = cloned
		}
	}
	if !changed {
		return msgs
	}
	return result
}

// builtinAutoCompactMessages truncates oversized tool results in older turns to
// a compact representation suitable for summarization context. Messages within
// the most recent builtinAutoCompactRecentWindow assistant turns are kept at
// full fidelity.
func builtinAutoCompactMessages(msgs []message.Message) []message.Message {
	cutoff := assistantTurnCutoff(msgs, builtinAutoCompactRecentWindow)
	if cutoff == 0 {
		return msgs
	}
	changed := false
	result := make([]message.Message, len(msgs))
	copy(result, msgs)
	for i := 0; i < cutoff; i++ {
		if msgs[i].Role != message.Tool {
			continue
		}
		cloned := msgs[i].Clone()
		modified := false
		for j, part := range cloned.Parts {
			tr, ok := part.(message.ToolResult)
			if !ok || tr.IsError || tr.Data != "" || tr.MIMEType != "" {
				continue
			}
			runes := []rune(tr.Content)
			if len(runes) <= builtinAutoCompactToolResultMaxChars {
				continue
			}
			omitted := len(runes) - builtinAutoCompactToolResultMaxChars
			tr.Content = string(runes[:builtinAutoCompactToolResultMaxChars]) +
				fmt.Sprintf("\n\n[%d characters omitted during context compaction]", omitted)
			cloned.Parts[j] = tr
			modified = true
			changed = true
		}
		if modified {
			result[i] = cloned
		}
	}
	if !changed {
		return msgs
	}
	return result
}

// assistantTurnCutoff returns the message index at which the n-th most recent
// assistant turn begins. All messages before this index are candidates for
// compaction. Returns 0 when there are fewer than n assistant turns, meaning
// no compaction should occur.
func assistantTurnCutoff(msgs []message.Message, n int) int {
	count := 0
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == message.Assistant {
			count++
			if count >= n {
				return i
			}
		}
	}
	return 0
}
