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

// maxOverheadRatio bounds how much of a window the fixed part of a request may
// be charged with. Tool definitions and a system prompt are genuinely large -
// tens of thousands of tokens once MCP servers are loaded - but an overhead
// measured from a stale usage report should not be allowed to eat the window.
const maxOverheadRatio = 0.5

// Overhead is the part of a request that no message accounts for: the system
// prompt and the tool definitions. It is measured rather than guessed, from
// the provider's own count of the last request against the conversation text
// that request carried.
//
// Every estimate of a tail is short by this much, and the gap is not small:
// a session with a full set of tools can carry twenty-odd percent of its
// window before the first message. Returning 0 when either side is unknown
// keeps the caller's arithmetic honest instead of inventing a constant.
func Overhead(reportedPromptTokens, estimatedMessages, contextWindow int64) int64 {
	if reportedPromptTokens <= 0 || estimatedMessages <= 0 {
		return 0
	}
	overhead := reportedPromptTokens - estimatedMessages
	if overhead <= 0 {
		return 0
	}
	if contextWindow > 0 && overhead > int64(float64(contextWindow)*maxOverheadRatio) {
		// Almost always a usage report from a larger earlier request, such as
		// a model swap. Better to under-account than to starve the tail.
		return int64(float64(contextWindow) * maxOverheadRatio)
	}
	return overhead
}

// UsableWindow is how much of a context window conversation text may occupy
// once the fixed cost of a request is subtracted. Never reduced below half the
// window, so a wild overhead measurement degrades gracefully.
func UsableWindow(contextWindow, overhead int64) int64 {
	if contextWindow <= 0 {
		return 0
	}
	return max(contextWindow-overhead, contextWindow/2)
}

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

// RegionWindowRatio is the share of a context window the text being summarized
// may claim. The rest has to hold the system prompt, the tool definitions, and
// the checkpoint the request is writing.
const RegionWindowRatio = 0.6

// RegionLimit returns the largest transcript region a single summarization
// request may carry, or 0 when the window is unknown and nothing can be said.
func RegionLimit(contextWindow int64) int64 {
	if contextWindow <= 0 {
		return 0
	}
	return int64(float64(contextWindow) * RegionWindowRatio)
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
	// SplitTurn is true when the tail begins inside a user turn rather than at
	// a turn boundary, which happens only when no whole-turn tail fits the
	// ceiling. The summarized region still ends at a point where every tool
	// call has its result, so nothing is orphaned.
	SplitTurn bool
}

// OverCeiling reports whether the retained tail is larger than the policy
// allows. It is possible when no safe boundary inside the oversized turn would
// leave a working set worth keeping, and callers should surface it rather than
// pretend the budget was honored.
func (c Cut) OverCeiling(policy Policy) bool {
	if !c.Found {
		return false
	}
	return c.KeptTokens > ceilingFor(policy)
}

// ceilingFor is the most a tail may claim. Honoring MinTailTurns is a
// preference bounded by it; without a ceiling a single agentic turn can stand
// in for the whole budget.
func ceilingFor(policy Policy) int64 {
	if policy.MaxTailTokens > 0 {
		return policy.MaxTailTokens
	}
	return policy.KeepRecentTokens * 2
}

// Plan picks the cut point for compacting msgs, which must be the session
// view in send order and must not include an earlier checkpoint.
//
// It prefers a user turn boundary, keeping the summarized region a whole number
// of turns, and holds to that whenever the result fits the ceiling. When the
// smallest whole-turn tail does not fit, it cuts at the nearest safe boundary
// inside the turn instead: a long-running turn with hundreds of tool calls
// would otherwise be retained entire, and "keep 8k of recent work" would mean
// "keep 25k", which is how compaction stops freeing anything.
//
// Either way a cut never separates a tool call from its result.
func Plan(msgs []message.Message, policy Policy) Cut {
	if policy.KeepRecentTokens <= 0 || len(msgs) < 2 {
		return Cut{}
	}

	boundaries := validCuts(msgs)
	if len(boundaries) == 0 {
		return Cut{}
	}

	// One pass of estimates, so every range below is a subtraction rather than
	// a rescan. A session can hold three thousand messages and a compaction
	// should not have to revisit each of them once per candidate cut.
	prefix := prefixEstimates(msgs)
	n := len(msgs)
	kept := func(from int) int64 { return prefix[n] - prefix[from] }

	// Land on the first turn boundary at or after the point where the running
	// total from the end reaches the budget.
	cut := -1
	for i := n - 1; i >= 0; i-- {
		if prefix[n]-prefix[i] >= policy.KeepRecentTokens {
			cut = snapForward(boundaries, i)
			break
		}
	}
	// The whole transcript fits inside the tail budget, so summarizing it
	// would trade fidelity for no space at all.
	if cut < 0 {
		return Cut{}
	}

	// Widen to honor the turn floor, but only while the tail stays inside its
	// ceiling.
	ceiling := ceilingFor(policy)
	for policy.MinTailTurns > 0 && countTurns(msgs[cut:]) < policy.MinTailTurns {
		prev := snapBackward(boundaries, cut)
		if prev < 0 || prev >= cut || kept(prev) > ceiling {
			break
		}
		cut = prev
	}

	// The ceiling is a post-condition, not only a limit on widening: the cut
	// above can land before a turn far larger than the budget, and honoring
	// that silently is the overshoot this whole path exists to stop.
	split := false
	if kept(cut) > ceiling {
		if inner := tighten(safeCuts(msgs), cut, kept, ceiling); inner > cut {
			cut = inner
			split = !startsTurn(msgs[cut])
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
		Kept:       n - cut,
		KeptTokens: kept(cut),
		SplitTurn:  split,
	}
}

// tighten moves a cut forward to the first safe boundary that fits the ceiling.
// Boundaries arrive in order and a later cut always keeps less, so the first
// one that fits is also the one that keeps the most context while still
// honoring the ceiling. Returns the original cut when nothing inside the turn
// fits: a working set that is too large beats a working set that is gone.
func tighten(safe []int, cut int, kept func(int) int64, ceiling int64) int {
	for _, c := range safe {
		if c <= cut {
			continue
		}
		if kept(c) <= ceiling {
			return c
		}
	}
	return cut
}

// safeCuts returns every index that begins a step: a point where each tool call
// already made has its result on the same side of the cut.
//
// A call and its result live in separate messages, so an arbitrary interior
// index would strand a result without its call and the provider would reject
// the request. Turn boundaries satisfy this by construction.
func safeCuts(msgs []message.Message) []int {
	var (
		pending = make(map[string]bool)
		out     []int
	)
	for i, msg := range msgs {
		if i > 0 && len(pending) == 0 {
			out = append(out, i)
		}
		for _, part := range msg.Parts {
			switch part := part.(type) {
			case message.ToolCall:
				// Provider-executed calls never produce a local result, so
				// waiting on one would forbid every boundary after it.
				if !part.ProviderExecuted {
					pending[part.ID] = true
				}
			case message.ToolResult:
				delete(pending, part.ToolCallID)
			}
		}
	}
	return out
}

// prefixEstimates holds the running estimate of msgs[:i], so the size of any
// range is a subtraction.
func prefixEstimates(msgs []message.Message) []int64 {
	prefix := make([]int64, len(msgs)+1)
	for i, msg := range msgs {
		prefix[i+1] = prefix[i] + Estimate(msg)
	}
	return prefix
}

// validCuts returns the indices a compaction prefers to cut at: every message
// that starts a fresh user turn, which keeps the summarized region a whole
// number of turns and leaves every tool call with the results that follow it.
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

// memorySharePercent is how much of the usable window recorded memory may
// occupy beside a checkpoint's summary. It is a share rather than a fixed
// number of tokens because the same session moves between models: a cap tuned
// for a 200k window would swallow a 64k one whole.
const memorySharePercent = 4

// MemoryBudget is the room a checkpoint's memory appendix may occupy, given the
// window that is actually usable and an upper bound from the checkpoint's own
// allocation. A non-positive window or cap is treated as no constraint from
// that side.
func MemoryBudget(usableWindow, ceiling int64) int64 {
	if usableWindow <= 0 {
		return ceiling
	}
	share := usableWindow * memorySharePercent / 100
	if ceiling > 0 && ceiling < share {
		return ceiling
	}
	return share
}
