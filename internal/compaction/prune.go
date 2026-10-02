package compaction

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/crush/internal/message"
)

// prunePrefix marks a tool result whose output has already been replaced, so
// a later pass recognizes where an earlier one stopped.
const prunePrefix = "[tool output pruned:"

// skeletonErrorPreview caps how much of an error result survives: an error is
// usually one line, and that line is what tells the next model not to repeat
// the attempt.
const skeletonErrorPreview = 200

// spillPointerPattern finds the file path Crush appends when a tool spilled
// oversized output to disk. A skeleton must keep it, or pruning would destroy
// the only pointer to the bytes.
var spillPointerPattern = regexp.MustCompile(`full output: (\S+)`)

// PrunePolicy bounds what a pruning pass may remove.
type PrunePolicy struct {
	// Disabled turns the pass off entirely.
	Disabled bool
	// ProtectTokens is how much of the most recent tool output is always
	// left alone, regardless of how many turns back it is.
	ProtectTokens int64
	// ProtectTurns is the number of recent user turns whose tool output is
	// always left alone.
	ProtectTurns int
	// MinimumReclaim is the smallest saving worth a rewrite for. Pruning
	// rewrites history and so costs a prompt-cache miss; a handful of tokens
	// is not worth that.
	MinimumReclaim int64
	// ProtectedTools name tools whose output is worth keeping in full: state
	// the model cannot re-derive, or words the user typed.
	ProtectedTools []string
}

// DefaultPrunePolicy protects the recent working set and only acts when it can
// free a meaningful amount of context.
var DefaultPrunePolicy = PrunePolicy{
	ProtectTokens:  40_000,
	ProtectTurns:   2,
	MinimumReclaim: 20_000,
	ProtectedTools: []string{"question", "todos", "agent", "agentic_fetch", "crush_info"},
}

const (
	// MinPruneProtectTokens is the smallest slice of recent tool output ever
	// protected, so a model with a small window still keeps what the current
	// step is working on.
	MinPruneProtectTokens = 2_048
	// MinPruneReclaimTokens is the smallest saving worth rewriting history
	// for, however the other budgets scale.
	MinPruneReclaimTokens = 2_048
)

// PrunePolicyForWindow sizes a pruning pass for a model. The protection window
// is a share of the retained tail rather than a constant: a fixed 40k protects
// more than a small model's entire tail, which leaves pruning nothing to do
// precisely where space is scarcest.
func PrunePolicyForWindow(contextWindow, configuredTail int64) PrunePolicy {
	p := DefaultPrunePolicy
	if contextWindow <= 0 {
		return p
	}
	tail := TailBudget(contextWindow, configuredTail)
	p.ProtectTokens = min(p.ProtectTokens, max(tail/2, MinPruneProtectTokens))
	p.MinimumReclaim = max(p.ProtectTokens/2, MinPruneReclaimTokens)
	return p
}

// Prune replaces the output of stale tool results with a one-line skeleton.
// It returns the view to send to the model and how many tokens it saved.
//
// Tool output is the largest and least durable thing a session carries: a
// single file read or MCP response can be tens of thousands of tokens that
// matter for one step and then sit in the window for the rest of the session.
// The tool call itself is kept, so the model still knows what ran and can
// simply run it again.
//
// Nothing is persisted: the caller keeps full output in storage, which leaves
// the transcript, the UI, and any later compaction untouched, and makes this a
// pure view decision.
func Prune(msgs []message.Message, policy PrunePolicy) ([]message.Message, int64) {
	if policy.Disabled || len(msgs) == 0 {
		return msgs, 0
	}

	protected := make(map[string]struct{}, len(policy.ProtectedTools))
	for _, name := range policy.ProtectedTools {
		protected[name] = struct{}{}
	}

	type candidate struct {
		message int
		part    int
		tokens  int64
	}

	// Everything from this index on is still inside a protected turn.
	boundary := turnBoundary(msgs, policy.ProtectTurns)

	var (
		candidates []candidate
		seen       int64
		reclaimed  int64
	)
	for i := boundary - 1; i >= 0; i-- {
		for j, part := range msgs[i].Parts {
			result, ok := part.(message.ToolResult)
			if !ok {
				continue
			}
			// Pruning is oldest-first, so reaching output an earlier pass
			// already pruned means there is nothing left to win here.
			if strings.HasPrefix(result.Content, prunePrefix) {
				return msgs, 0
			}
			if _, ok := protected[result.Name]; ok {
				continue
			}

			size := toolResultTokens(result)
			seen += size
			if seen <= policy.ProtectTokens {
				continue
			}
			candidates = append(candidates, candidate{message: i, part: j, tokens: size})
			reclaimed += size
		}
	}

	if len(candidates) == 0 || reclaimed < policy.MinimumReclaim {
		return msgs, 0
	}

	out := make([]message.Message, len(msgs))
	copy(out, msgs)
	for _, c := range candidates {
		original, ok := out[c.message].Parts[c.part].(message.ToolResult)
		if !ok {
			continue
		}
		// Replace the part rather than mutating it: the caller's slice is
		// shared with the stored message.
		parts := make([]message.ContentPart, len(out[c.message].Parts))
		copy(parts, out[c.message].Parts)
		original.Content = skeleton(original, c.tokens)
		original.Data = ""
		original.MIMEType = ""
		parts[c.part] = original
		out[c.message].Parts = parts
	}
	return out, reclaimed
}

// turnBoundary returns the index where protected conversation begins: every
// message at or after it belongs to one of the last protectTurns user turns.
// A session with fewer turns than the policy protects is protected in full,
// which is the point of protecting them.
func turnBoundary(msgs []message.Message, protectTurns int) int {
	if protectTurns <= 0 {
		return 0
	}
	var seen int
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role != message.User {
			continue
		}
		seen++
		if seen == protectTurns {
			return i
		}
	}
	return 0
}

// skeleton describes a pruned tool result: what ran, how much is missing, and
// how to get it back.
func skeleton(result message.ToolResult, tokens int64) string {
	var details strings.Builder
	fmt.Fprintf(&details, "%s %s; ~%d tokens", prunePrefix, result.Name, tokens)
	if lines := strings.Count(result.Content, "\n") + 1; lines > 1 {
		fmt.Fprintf(&details, ", %d lines", lines)
	}
	if match := spillPointerPattern.FindStringSubmatch(result.Content); match != nil {
		fmt.Fprintf(&details, "; full output: %s", match[1])
	}
	if result.IsError {
		// The first line of an error is the lesson; dropping it would let the
		// next model make the same mistake again.
		fmt.Fprintf(&details, "; it failed: %s", firstLine(result.Content))
	}
	details.WriteString("]")
	return details.String()
}

func firstLine(content string) string {
	if i := strings.IndexByte(content, '\n'); i >= 0 {
		content = content[:i]
	}
	content = strings.TrimSpace(content)
	if len(content) > skeletonErrorPreview {
		content = content[:skeletonErrorPreview] + "…"
	}
	if content == "" {
		return "no message"
	}
	return content
}
