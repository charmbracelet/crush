// Package compaction decides what a compaction rewrites as a checkpoint and
// what it leaves verbatim, so that a long session keeps its working set
// instead of paraphrasing it away one summary at a time.
package compaction

import (
	"github.com/charmbracelet/crush/internal/message"
)

const (
	// LargeContextWindowThreshold is the window size above which a fixed
	// reserve is enough headroom for the next round trip.
	largeContextWindowThreshold = 200_000
	// LargeContextWindowBuffer is that fixed reserve.
	largeContextWindowBuffer = 20_000
	// SmallContextWindowRatio is the reserve used for smaller windows,
	// where a fixed buffer is either most of the window or noise.
	smallContextWindowRatio = 0.2
	// maxOutputReserveRatio caps the share of a window a single pending
	// completion may claim before compaction is asked to make room for it.
	// Without a cap, a model whose default max_tokens is generous would
	// compact every session at the cap's fraction of the window.
	maxOutputReserveRatio = 0.25
)

// Policy bounds how much of a transcript a compaction is allowed to discard.
type Policy struct {
	// KeepRecentTokens is the budget of most recent conversation retained
	// verbatim instead of summarized.
	KeepRecentTokens int64
	// MinTailTurns is the number of recent user turns to retain when they fit.
	// It is a preference, not a guarantee: one agentic turn now routinely runs
	// to a hundred tool calls, and treating the floor as unconditional lets a
	// single turn swallow the whole budget, which is how a compaction ends up
	// keeping everything it meant to free.
	MinTailTurns int
	// MaxTailTokens is how large the retained tail may grow while honoring
	// MinTailTurns. Zero means twice KeepRecentTokens.
	MaxTailTokens int64
}

// DefaultPolicy keeps the most recent ~20k tokens and, when they fit, no fewer
// than two user turns. Use TailBudget to size the tail for a real model.
var DefaultPolicy = Policy{
	KeepRecentTokens: 20_000,
	MinTailTurns:     2,
}

const (
	// TailWindowRatio is the share of a model's context window a retained tail
	// may claim. A fixed token count is wrong in both directions: 20k is a
	// rounding error on a 600k window and most of a 65k one, and the tail has
	// to leave room for the fixed cost every request carries - system prompt
	// and tool definitions - on top of the checkpoint.
	TailWindowRatio = 0.15
	// MinTailTokens is the smallest tail ever retained, below which the model
	// is left with no working set at all.
	MinTailTokens = 4_096
)

// TailBudget returns how many tokens of recent conversation a compaction keeps
// verbatim: an explicit configuration always wins, otherwise a fraction of the
// model's context window.
func TailBudget(contextWindow, configured int64) int64 {
	if configured > 0 {
		return configured
	}
	if contextWindow <= 0 {
		return DefaultPolicy.KeepRecentTokens
	}
	return max(int64(float64(contextWindow)*TailWindowRatio), MinTailTokens)
}

const (
	// MinCheckpointTokens is the smallest output budget ever handed to a
	// checkpoint request. Below this the summary is not worth its latency, so
	// the request is sent anyway and the caller warns.
	MinCheckpointTokens = 1_024
	// MaxCheckpointTokens is the largest output budget a checkpoint request
	// asks for. A checkpoint that needs more than this is transcribing the
	// session rather than distilling it.
	MaxCheckpointTokens = 8_192
)

// CheckpointOutputBudget returns the max_tokens a summarization request should
// ask for, and whether the window could not even afford the floor.
//
// Left unset, the provider applies its own default, and a model whose default
// is generous can push prompt plus completion past the context window. A
// request to summarize a nearly-full session is then rejected for being too
// long, which is exactly the failure compaction exists to prevent. The
// reserve for future turns is deliberately not subtracted here: this request
// only has to fit itself.
func CheckpointOutputBudget(contextWindow, promptTokens, modelDefaultMaxTokens int64) (int64, bool) {
	budget := int64(MaxCheckpointTokens)
	if modelDefaultMaxTokens > 0 && modelDefaultMaxTokens < budget {
		budget = modelDefaultMaxTokens
	}

	tight := false
	if contextWindow > 0 {
		if room := contextWindow - promptTokens; room < budget {
			budget = room
		}
		tight = budget < MinCheckpointTokens
	}

	return max(budget, MinCheckpointTokens), tight
}

// Reserve returns the number of tokens that must stay free before another
// round trip is worth attempting.
//
// maxOutputTokens is the completion the next request will ask for: providers
// reject a request outright when prompt plus max_tokens exceeds the context
// window, so a pending completion that does not fit has to be paid for before
// the cliff edge rather than at it.
func Reserve(contextWindow, maxOutputTokens int64) int64 {
	if contextWindow <= 0 {
		return 0
	}

	reserve := int64(float64(contextWindow) * smallContextWindowRatio)
	if contextWindow > largeContextWindowThreshold {
		reserve = largeContextWindowBuffer
	}

	if maxOutputTokens > 0 {
		ceiling := int64(float64(contextWindow) * maxOutputReserveRatio)
		if want := min(maxOutputTokens, ceiling); want > reserve {
			reserve = want
		}
	}

	return reserve
}

// ShouldCompact reports whether enough of the window is used that the next
// round trip is at risk. An unknown context window disables automatic
// compaction, which keeps custom and local models from being summarized away
// on their first turn.
func ShouldCompact(used, contextWindow, maxOutputTokens int64) bool {
	if contextWindow <= 0 {
		return false
	}
	return contextWindow-used <= Reserve(contextWindow, maxOutputTokens)
}

// Cut is the outcome of measuring a transcript against a Policy.
type Cut struct {
	// Found is false when there is nothing worth compacting: the whole
	// transcript already fits the retained tail, or no legal cut point
	// exists ahead of it. Callers that still want a checkpoint (a manual
	// summarize) should summarize everything.
	Found bool
	// Index is where the retained tail begins in the slice passed to Plan.
	// Messages before Index are summarized; messages from Index on are kept
	// exactly as they are.
	Index int
	// MessageID is the ID of the first retained message. It is persisted so
	// later reads rebuild the same view instead of re-deriving a cut that
	// may have moved as the session grew.
	MessageID string
	// Summarized and Kept count messages on each side of the cut.
	Summarized int
	Kept       int
	// KeptTokens is the estimated size of the retained tail.
	KeptTokens int64
}

// Plan picks the cut point for compacting msgs, which must be the session
// view in send order and must not include an earlier checkpoint.
//
// The cut is always a user turn boundary, so the summarized region is a whole
// number of turns and can never split a tool call from its result. Within
// that constraint it keeps as close to Policy.KeepRecentTokens as it can, then
// walks the cut backwards until at least Policy.MinTailTurns are retained, so
// long as that keeps the tail inside Policy.MaxTailTokens.
func Plan(msgs []message.Message, policy Policy) Cut {
	if policy.KeepRecentTokens <= 0 || len(msgs) < 2 {
		return Cut{}
	}

	var (
		cuts  = validCuts(msgs)
		cut   int
		total int64
	)
	if len(cuts) == 0 {
		return Cut{}
	}

	found := false
	for i := len(msgs) - 1; i >= 0; i-- {
		total += Estimate(msgs[i])
		if total >= policy.KeepRecentTokens {
			cut, found = snapForward(cuts, i), true
			break
		}
	}
	// The whole transcript fits inside the tail budget, so summarizing it
	// would trade fidelity for no space at all.
	if !found {
		return Cut{}
	}

	// Widen to honor the turn floor, but only while the tail stays inside its
	// ceiling. One agentic turn routinely carries a hundred tool calls, and
	// honoring the floor regardless of size is what let a 250-message tail
	// stand in for a 20k one.
	ceiling := policy.MaxTailTokens
	if ceiling <= 0 {
		ceiling = policy.KeepRecentTokens * 2
	}
	keptTokens := EstimateAll(msgs[cut:])
	for policy.MinTailTurns > 0 && countTurns(msgs[cut:]) < policy.MinTailTurns {
		prev := snapBackward(cuts, cut)
		if prev < 0 || prev >= cut {
			break
		}
		if wider := keptTokens + EstimateAll(msgs[prev:cut]); wider <= ceiling {
			cut, keptTokens = prev, wider
		} else {
			break
		}
	}

	if cut <= 0 {
		return Cut{}
	}

	return Cut{
		Found:      true,
		Index:      cut,
		MessageID:  msgs[cut].ID,
		Summarized: cut,
		Kept:       len(msgs) - cut,
		KeptTokens: keptTokens,
	}
}

// validCuts returns the indices a compaction may cut at: every message that
// starts a fresh user turn. Cutting before a user message keeps the
// summarized region on complete turns and leaves every tool call with the
// results that follow it.
func validCuts(msgs []message.Message) []int {
	cuts := make([]int, 0, len(msgs))
	for i := 1; i < len(msgs); i++ {
		if startsTurn(msgs[i]) {
			cuts = append(cuts, i)
		}
	}
	return cuts
}

func startsTurn(msg message.Message) bool {
	return msg.Role == message.User
}

// snapForward returns the first cut at or after i, falling back to the last
// known cut when the budget runs out past every boundary.
func snapForward(cuts []int, i int) int {
	for _, c := range cuts {
		if c >= i {
			return c
		}
	}
	return cuts[len(cuts)-1]
}

// snapBackward returns the last cut strictly before i, or -1.
func snapBackward(cuts []int, i int) int {
	prev := -1
	for _, c := range cuts {
		if c >= i {
			break
		}
		prev = c
	}
	return prev
}

func countTurns(msgs []message.Message) int {
	var turns int
	for _, msg := range msgs {
		if startsTurn(msg) {
			turns++
		}
	}
	return turns
}
