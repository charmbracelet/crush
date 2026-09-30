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
	// MinTailTurns is the number of recent user turns always retained,
	// however large they are. It keeps the cut out of a run of tiny turns.
	MinTailTurns int
}

// DefaultPolicy keeps the most recent ~20k tokens and never fewer than two
// user turns.
var DefaultPolicy = Policy{
	KeepRecentTokens: 20_000,
	MinTailTurns:     2,
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
// walks the cut backwards until at least Policy.MinTailTurns are retained.
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

	for policy.MinTailTurns > 0 && countTurns(msgs[cut:]) < policy.MinTailTurns {
		prev := snapBackward(cuts, cut)
		if prev < 0 || prev == cut {
			break
		}
		cut = prev
	}

	if cut <= 0 {
		return Cut{}
	}

	var keptTokens int64
	for _, msg := range msgs[cut:] {
		keptTokens += Estimate(msg)
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
