package compaction

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

// step builds one agentic step: an assistant message issuing a tool call and
// the message after it carrying the result. Crush stores a call and its result
// in separate messages, which is exactly what makes an interior cut dangerous.
func step(id string, callTokens, resultTokens int) []message.Message {
	return []message.Message{
		{
			ID:   "call-" + id,
			Role: message.Assistant,
			Parts: []message.ContentPart{message.ToolCall{
				ID:    "tc-" + id,
				Name:  "view",
				Input: strings.Repeat("i", callTokens*charsPerToken),
			}},
		},
		{
			ID:   "result-" + id,
			Role: message.Assistant,
			Parts: []message.ContentPart{message.ToolResult{
				ToolCallID: "tc-" + id,
				Name:       "view",
				Content:    strings.Repeat("r", resultTokens*charsPerToken),
			}},
		},
	}
}

// longTurn is the shape that used to defeat the budget: one user message
// followed by an arbitrary number of steps and no further user message, as in
// a session left to run on its own.
func longTurn(steps, tokensEach int) []message.Message {
	msgs := []message.Message{filler(message.User, "u-long", tokensEach)}
	for i := range steps {
		msgs = append(msgs, step(fmt.Sprint(i), tokensEach, tokensEach)...)
	}
	return msgs
}

func assertPairsIntact(t *testing.T, msgs []message.Message, cut int) {
	t.Helper()

	calls, results := map[int]string{}, map[int]string{}
	for i, msg := range msgs {
		for _, part := range msg.Parts {
			switch part := part.(type) {
			case message.ToolCall:
				calls[i] = part.ID
			case message.ToolResult:
				results[i] = part.ToolCallID
			}
		}
	}
	for _, id := range calls {
		var before, after bool
		for i, got := range results {
			if got != id {
				continue
			}
			if i < cut {
				before = true
			} else {
				after = true
			}
		}
		require.False(t, before && after,
			"tool call %s straddles the cut at %d, which would strand a result without its call", id, cut)
	}
}

// TestPlanSplitsATurnThatCannotFit is the whole reason interior cut points
// exist. The only turn boundary in the transcript begins a turn ten times the
// tail budget, so holding to boundaries alone meant retaining the entire turn
// and freeing nothing.
func TestPlanSplitsATurnThatCannotFit(t *testing.T) {
	t.Parallel()

	msgs := append(turn("first", 500), longTurn(20, 400)...)
	policy := Policy{KeepRecentTokens: 2_000, MinTailTurns: 2}

	cut := Plan(msgs, policy)

	require.True(t, cut.Found)
	require.True(t, cut.SplitTurn, "no whole-turn tail fits, so the cut moves inside the turn")
	require.LessOrEqual(t, cut.KeptTokens, policy.KeepRecentTokens*2)
	require.False(t, cut.OverCeiling(policy))
	require.False(t, startsTurn(msgs[cut.Index]), "the tail now begins mid-turn")
	assertPairsIntact(t, msgs, cut.Index)
}

// TestPlanPrefersWholeTurnsUnderTheCeiling: splitting is a remedy for an
// oversized turn, not a new default. A session of ordinary turns must keep
// cutting between them.
func TestPlanPrefersWholeTurnsUnderTheCeiling(t *testing.T) {
	t.Parallel()

	var msgs []message.Message
	for i := range 8 {
		msgs = append(msgs, turn(fmt.Sprint(i), 400)...)
	}

	cut := Plan(msgs, Policy{KeepRecentTokens: 2_000, MinTailTurns: 2})

	require.True(t, cut.Found)
	require.False(t, cut.SplitTurn, "a whole-turn tail inside the ceiling is still the answer")
	require.True(t, startsTurn(msgs[cut.Index]))
	assertPairsIntact(t, msgs, cut.Index)
}

// TestPlanReportsWhenEvenASplitTailIsTooBig covers a single message larger
// than the ceiling, which cannot be cut at all. Overstating compliance would
// hide the one case a user needs told.
func TestPlanReportsWhenEvenASplitTailIsTooBig(t *testing.T) {
	t.Parallel()

	// The current turn is a single reply larger than the whole tail budget, so
	// every boundary either keeps all of it or keeps nothing.
	msgs := append(turn("first", 100),
		filler(message.User, "u-again", 100),
		filler(message.Assistant, "a-huge", 9_000))
	policy := Policy{KeepRecentTokens: 1_000}

	cut := Plan(msgs, policy)

	require.True(t, cut.Found)
	require.False(t, cut.SplitTurn, "there is no interior boundary in a single message")
	require.True(t, cut.OverCeiling(policy))
}

// TestPlanAcrossTurnShapes is the pairing invariant stated as a property: for
// any shape, whatever cut comes back, no tool call is left with its result on
// the other side.
func TestPlanAcrossTurnShapes(t *testing.T) {
	t.Parallel()

	shapes := map[string][]message.Message{
		"single long turn":   longTurn(30, 300),
		"turns of steps":     append(append(turn("a", 300), longTurn(10, 300)...), turn("b", 300)...),
		"trailing long turn": append(turn("a", 300), longTurn(25, 500)...),
	}
	for name, msgs := range shapes {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			for _, budget := range []int64{500, 2_000, 8_000} {
				cut := Plan(msgs, Policy{KeepRecentTokens: budget, MinTailTurns: 2})
				if !cut.Found {
					continue
				}
				assertPairsIntact(t, msgs, cut.Index)
			}
		})
	}
}

func TestSafeCutsSkipsBetweenACallAndItsResult(t *testing.T) {
	t.Parallel()

	msgs := longTurn(2, 100)
	safe := safeCuts(msgs)

	for _, c := range safe {
		require.NotEqual(t, "result-0", msgs[c].ID, "a cut may not separate tc-0 from its call")
		require.NotEqual(t, "result-1", msgs[c].ID)
	}
	// Boundaries exist before the turn and before each new step. The indices
	// between a call and its result do not, and neither does an index past the
	// end, which would retain nothing.
	require.Equal(t, []int{1, 3}, safe)
}

func TestSafeCutsDoesNotWaitOnProviderExecutedCalls(t *testing.T) {
	t.Parallel()

	msgs := []message.Message{
		userMsg("go"),
		{
			ID:    "a-executed",
			Role:  message.Assistant,
			Parts: []message.ContentPart{message.ToolCall{ID: "tc-x", Name: "web_search", ProviderExecuted: true}},
		},
		assistantMsg("after"),
	}

	// The boundary ahead of the follow-up message is only legal because the
	// executed call will never produce a local result to wait for.
	require.Contains(t, safeCuts(msgs), 2)

	msgs[1].Parts[0] = message.ToolCall{ID: "tc-x", Name: "view"}
	require.NotContains(t, safeCuts(msgs), 2,
		"a local call still awaiting its result gates the boundary after it")
}

func TestOverhead(t *testing.T) {
	t.Parallel()

	require.Equal(t, int64(24_387), Overhead(179_907, 155_520, 600_000),
		"the gap measured on a real session: system prompt plus tool definitions")
	require.Zero(t, Overhead(0, 155_520, 600_000), "no reported usage yet, so nothing is known")
	require.Zero(t, Overhead(100_000, 100_000, 600_000), "a request with no fixed cost")
	require.Zero(t, Overhead(90_000, 100_000, 600_000), "estimates run high sometimes; never go negative")
	require.Equal(t, int64(300_000), Overhead(1_000_000, 10_000, 600_000),
		"a stale reading cannot eat the whole window")
}

func TestUsableWindow(t *testing.T) {
	t.Parallel()

	require.Equal(t, int64(575_613), UsableWindow(600_000, 24_387))
	require.Equal(t, int64(600_000), UsableWindow(600_000, 0))
	require.Equal(t, int64(300_000), UsableWindow(600_000, 900_000),
		"never reduced below half the window")
	require.Zero(t, UsableWindow(0, 1_000), "an unknown window stays unknown")
}

// TestTailBudgetFollowsTheUsableWindow is the point of measuring overhead: the
// tail is sized from space that actually exists.
func TestTailBudgetFollowsTheUsableWindow(t *testing.T) {
	t.Parallel()

	full := TailBudget(600_000, 0)
	afterOverhead := TailBudget(UsableWindow(600_000, 24_387), 0)

	require.Equal(t, int64(90_000), full)
	require.Less(t, afterOverhead, full)
	require.Equal(t, int64(8_000), TailBudget(UsableWindow(600_000, 24_387), 8_000),
		"an explicit configuration is never adjusted")
}
