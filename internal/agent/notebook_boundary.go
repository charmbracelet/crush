package agent

import (
	"github.com/charmbracelet/crush/internal/message"
)

// findTurnBoundaryByTokenBudget walks backwards from the latest
// message, accumulating tokens until the budget is exceeded. The
// boundary always falls at a safe turn end (assistant message with no
// pending tool calls) to avoid splitting tool-call sequences.
//
// This is adaptive: chat-only sessions get more turns (small tokens
// each), heavy-coding sessions get fewer turns (large tokens each).
// The token budget stays bounded regardless of session type.
func findTurnBoundaryByTokenBudget(
	msgs []message.Message,
	tokenBudget int,
	estimateTokens func([]message.Message) int,
) int {
	if tokenBudget <= 0 {
		return 0
	}
	accumulated := 0
	for i := len(msgs) - 1; i >= 0; i-- {
		accumulated += estimateTokens(msgs[i : i+1])
		if accumulated > tokenBudget {
			boundary := findNextSafeBoundary(msgs, i+1)
			if boundary >= len(msgs) {
				// The safe boundary would exclude the most
				// recent completed turn. Instead of sending
				// everything as raw (returning 0), find the
				// start of the most recent turn and include
				// just that turn as raw. Older turns come
				// from the notebook.
				return findMostRecentTurnStart(msgs)
			}
			return boundary
		}
	}
	return 0
}

// findMostRecentTurnStart returns the index of the most recent user
// message (the start of the current/most-recent turn). If there is no
// user message, returns 0.
func findMostRecentTurnStart(msgs []message.Message) int {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == message.User {
			return i
		}
	}
	return 0
}

// findNextSafeBoundary finds the earliest index >= start that is a
// safe turn boundary (after an assistant message with no pending tool
// calls). This ensures we never split a tool-call sequence even when
// the token budget would force a cut mid-sequence. If no safe boundary
// is found, returns start (the caller handles the fallback).
func findNextSafeBoundary(msgs []message.Message, start int) int {
	for i := start; i < len(msgs); i++ {
		if msgs[i].Role == message.Assistant && len(msgs[i].ToolCalls()) == 0 {
			return i + 1
		}
	}
	return start
}

// findTurnStartByTurnNumber finds the message index where the given
// turn number (0-indexed) starts. A turn starts at each user message.
// Returns len(msgs) if the turn number is beyond the last turn.
func findTurnStartByTurnNumber(msgs []message.Message, turnNumber int64) int {
	if turnNumber <= 0 {
		return 0
	}
	currentTurn := int64(0)
	for i, msg := range msgs {
		if msg.Role == message.User {
			if currentTurn == turnNumber {
				return i
			}
			currentTurn++
		}
	}
	return len(msgs)
}

// findStaleTurnBoundary returns the adjusted boundary that includes
// unprocessed turns as raw. If any turns before the boundary don't
// have notebook entries yet (async generation hasn't finished), the
// boundary is moved back to the start of the oldest unprocessed turn
// so those turns are included in the raw window instead of being
// silently lost.
func findStaleTurnBoundary(msgs []message.Message, boundaryTurn int64, turnsWithEntries map[int64]bool) int {
	// Walk forward through turns, finding the oldest turn that
	// is missing from the notebook. The boundary is moved to the
	// start of that turn.
	currentTurn := int64(0)
	oldestStaleTurn := int64(-1)
	for _, msg := range msgs {
		if msg.Role == message.User {
			if currentTurn < boundaryTurn && !turnsWithEntries[currentTurn] {
				oldestStaleTurn = currentTurn
				break
			}
			currentTurn++
		}
	}
	if oldestStaleTurn < 0 {
		// No stale turns — return original boundary.
		return len(msgs)
	}
	return findTurnStartByTurnNumber(msgs, oldestStaleTurn)
}

// isTurnEnd returns true if the message at the given index is a safe
// turn boundary (assistant message with no pending tool calls, or a
// cancelled message).
func isTurnEnd(msgs []message.Message, idx int) bool {
	if idx < 0 || idx >= len(msgs) {
		return false
	}
	m := msgs[idx]
	if m.Role != message.Assistant {
		return false
	}
	return len(m.ToolCalls()) == 0
}

// estimateRawMessageTokens provides a rough token estimate for a slice
// of internal message.Message (~4 chars per token). This is distinct
// from estimateMessageTokens which operates on fantasy.Message.
func estimateRawMessageTokens(msgs []message.Message) int {
	var totalChars int
	for _, m := range msgs {
		for _, part := range m.Parts {
			switch v := part.(type) {
			case message.TextContent:
				totalChars += len(v.Text)
			case message.ToolCall:
				totalChars += len(v.Input)
			case message.ToolResult:
				if v.Superseded != nil && v.Superseded.Applied {
					// The render emits the stub, not the stored
					// original — count what the model actually sees.
					totalChars += len(v.Superseded.StubText(v))
				} else {
					totalChars += len(v.Content)
				}
			case message.ReasoningContent:
				totalChars += len(v.Thinking)
			}
		}
	}
	return totalChars / 4
}
