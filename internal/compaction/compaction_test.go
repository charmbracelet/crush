package compaction

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

func userMsg(text string) message.Message {
	return message.Message{
		ID:    "u-" + text,
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: text}},
	}
}

func assistantMsg(text string) message.Message {
	return message.Message{
		ID:    "a-" + text,
		Role:  message.Assistant,
		Parts: []message.ContentPart{message.TextContent{Text: text}},
	}
}

// filler builds a message of roughly n tokens under Estimate.
func filler(role message.MessageRole, id string, n int) message.Message {
	return message.Message{
		ID:    id,
		Role:  role,
		Parts: []message.ContentPart{message.TextContent{Text: strings.Repeat("x", n*charsPerToken)}},
	}
}

// turn builds a user turn: the user message plus an assistant reply, both
// sized so that each message is about n tokens.
func turn(id string, n int) []message.Message {
	return []message.Message{
		filler(message.User, "u-"+id, n),
		filler(message.Assistant, "a-"+id, n),
	}
}

func TestReserve(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		contextWindow   int64
		maxOutputTokens int64
		want            int64
	}{
		{name: "unknown window reserves nothing", contextWindow: 0, want: 0},
		// These three pin the historical thresholds: adding output-aware
		// reservation must not move compaction for models that report no
		// max output.
		{name: "200k window keeps 20 percent", contextWindow: 200_000, want: 40_000},
		{name: "128k window keeps 20 percent", contextWindow: 128_000, want: 25_600},
		{name: "large window keeps a fixed buffer", contextWindow: 400_000, want: 20_000},
		{name: "small pending completion leaves the base reserve", contextWindow: 200_000, maxOutputTokens: 8_000, want: 40_000},
		{name: "pending completion above the base raises the reserve", contextWindow: 200_000, maxOutputTokens: 45_000, want: 45_000},
		{name: "pending completion is capped at a quarter of the window", contextWindow: 200_000, maxOutputTokens: 120_000, want: 50_000},
		{name: "fixed buffer wins until the completion exceeds it", contextWindow: 1_000_000, maxOutputTokens: 10_000, want: 20_000},
		{name: "completion above the fixed buffer is honored", contextWindow: 1_000_000, maxOutputTokens: 100_000, want: 100_000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, Reserve(tt.contextWindow, tt.maxOutputTokens))
		})
	}
}

func TestShouldCompact(t *testing.T) {
	t.Parallel()

	require.False(t, ShouldCompact(999_999, 0, 0), "an unknown window must never trigger compaction")
	require.False(t, ShouldCompact(100_000, 200_000, 0))
	require.True(t, ShouldCompact(160_000, 200_000, 0), "reserve is 40k, so 160k used is at the line")
	require.False(t, ShouldCompact(159_999, 200_000, 0))
	// The 64k completion is capped to a 50k reserve, so the session has to
	// compact once 50k or less is left rather than at the usual 40k.
	require.False(t, ShouldCompact(149_999, 200_000, 64_000))
	require.True(t, ShouldCompact(150_000, 200_000, 64_000), "a pending completion must fit before the next request")
}

func TestPlanKeepsNothingWhenTheTranscriptFits(t *testing.T) {
	t.Parallel()

	msgs := append(turn("1", 100), turn("2", 100)...)
	require.False(t, Plan(msgs, Policy{KeepRecentTokens: 20_000, MinTailTurns: 2}).Found)
}

func TestPlanCutsAtATurnBoundary(t *testing.T) {
	t.Parallel()

	// Six turns of 4k tokens each against a 10k tail budget: the newest
	// three turns are ~12k, so the cut lands on the turn that reaches the
	// budget and the summarized region stays a whole number of turns.
	var msgs []message.Message
	for i := range 6 {
		msgs = append(msgs, turn(string(rune('a'+i)), 2_000)...)
	}

	plan := Plan(msgs, Policy{KeepRecentTokens: 10_000, MinTailTurns: 2})
	require.True(t, plan.Found)
	// The budget is reached partway through turn 4 (index 6), and the cut
	// snaps forward to the next boundary: the tail may come in under budget,
	// but it always starts a clean turn.
	require.Equal(t, 8, plan.Index)
	require.Zero(t, plan.Index%2, "the cut must fall on a turn boundary")
	require.Equal(t, 8, plan.Summarized)
	require.Equal(t, 4, plan.Kept)
	require.Equal(t, len(msgs)-plan.Index, plan.Kept)
	require.Equal(t, msgs[plan.Index].ID, plan.MessageID)
	require.Equal(t, int64(8_000), plan.KeptTokens)
	require.Equal(t, 2, countTurns(msgs[plan.Index:]))
}

func TestPlanNeverSplitsAToolCallFromItsResult(t *testing.T) {
	t.Parallel()

	msgs := []message.Message{
		userMsg("start"),
		filler(message.Assistant, "a-big", 4_000),
		{
			ID:   "tool",
			Role: message.Tool,
			Parts: []message.ContentPart{
				message.ToolResult{ToolCallID: "call-1", Name: "view", Content: strings.Repeat("y", 4_000*charsPerToken)},
			},
		},
	}
	for i := range 3 {
		msgs = append(msgs, turn(string(rune('a'+i)), 2_000)...)
	}

	plan := Plan(msgs, Policy{KeepRecentTokens: 5_000, MinTailTurns: 1})
	require.True(t, plan.Found)
	require.NotEqual(t, message.Tool, msgs[plan.Index].Role, "a retained region cannot begin with an orphaned tool result")
	require.NotEqual(t, message.Tool, msgs[plan.Index-1].Role)
}

func TestPlanEnforcesMinimumTailTurns(t *testing.T) {
	t.Parallel()

	msgs := append(turn("1", 5_000), turn("2", 10)...)
	msgs = append(msgs, turn("3", 10)...)

	// The budget is satisfied inside turn 3, but the policy insists on two
	// turns, so the cut walks back to the start of turn 2.
	plan := Plan(msgs, Policy{KeepRecentTokens: 5, MinTailTurns: 2})
	require.True(t, plan.Found)
	require.Equal(t, 2, plan.Index)
	require.Equal(t, "u-2", plan.MessageID)
	require.Equal(t, 2, countTurns(msgs[plan.Index:]))
}

func TestPlanNeedsAUserBoundary(t *testing.T) {
	t.Parallel()

	msgs := []message.Message{
		userMsg("only turn"),
		filler(message.Assistant, "a-1", 30_000),
	}
	require.False(t, Plan(msgs, Policy{KeepRecentTokens: 1_000, MinTailTurns: 1}).Found,
		"the single user message cannot be summarized away")
}

func TestPlanIgnoresNonPositiveBudgets(t *testing.T) {
	t.Parallel()

	msgs := append(turn("1", 100), turn("2", 100)...)
	require.False(t, Plan(msgs, Policy{KeepRecentTokens: 0}).Found)
	require.False(t, Plan(nil, Policy{KeepRecentTokens: 1_000}).Found)
}

func TestCheckpointOutputBudget(t *testing.T) {
	t.Parallel()

	// A model whose default is smaller than the cap keeps its default.
	tokens, tight := CheckpointOutputBudget(200_000, 10_000, 4_096)
	require.Equal(t, int64(4_096), tokens)
	require.False(t, tight)

	// A generous default is capped: a checkpoint longer than this is a
	// transcript rather than a distillation.
	tokens, tight = CheckpointOutputBudget(200_000, 10_000, 64_000)
	require.Equal(t, int64(MaxCheckpointTokens), tokens)
	require.False(t, tight)

	// An unset model default falls back to the cap.
	tokens, _ = CheckpointOutputBudget(200_000, 10_000, 0)
	require.Equal(t, int64(MaxCheckpointTokens), tokens)

	// A nearly-full window only gets what is actually left.
	tokens, tight = CheckpointOutputBudget(200_000, 196_000, 0)
	require.Equal(t, int64(4_000), tokens)
	require.False(t, tight)

	// Below the floor the request is still sent, flagged so the caller can
	// warn that the checkpoint will be thin.
	tokens, tight = CheckpointOutputBudget(200_000, 199_900, 0)
	require.Equal(t, int64(MinCheckpointTokens), tokens)
	require.True(t, tight)

	// An unknown window cannot constrain anything.
	tokens, tight = CheckpointOutputBudget(0, 0, 0)
	require.Equal(t, int64(MaxCheckpointTokens), tokens)
	require.False(t, tight)
}

func TestEstimate(t *testing.T) {
	t.Parallel()

	require.Zero(t, Estimate(message.Message{}))
	require.Equal(t, int64(3), Estimate(userMsg("abcdefghyyyy")))

	toolCall := message.Message{
		Role:  message.Assistant,
		Parts: []message.ContentPart{message.ToolCall{Name: "view", Input: "abcd"}},
	}
	require.Equal(t, int64(framingTokens+2), Estimate(toolCall))

	toolResult := message.Message{
		Role:  message.Tool,
		Parts: []message.ContentPart{message.ToolResult{Name: "view", Content: "abcd"}},
	}
	require.Equal(t, int64(framingTokens+1), Estimate(toolResult))

	image := message.Message{
		Role:  message.User,
		Parts: []message.ContentPart{message.BinaryContent{MIMEType: "image/png", Data: make([]byte, 100_000)}},
	}
	require.Equal(t, int64(imageTokens), Estimate(image))

	text := message.Message{
		Role:  message.User,
		Parts: []message.ContentPart{message.BinaryContent{MIMEType: "text/plain", Data: []byte("abcdefgh")}},
	}
	require.Equal(t, int64(2), Estimate(text))
}

func TestEstimateAll(t *testing.T) {
	t.Parallel()

	require.Equal(t, Estimate(userMsg("hello"))+Estimate(assistantMsg("world")),
		EstimateAll([]message.Message{userMsg("hello"), assistantMsg("world")}))
}
