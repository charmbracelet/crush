package compaction

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

var testPolicy = PrunePolicy{
	ProtectTokens:  1_000,
	ProtectTurns:   2,
	MinimumReclaim: 500,
	ProtectedTools: []string{"question"},
}

// toolTurn builds one user turn: a prompt, an assistant tool call, and that
// call's result carrying roughly tokens words of output.
func toolTurn(label string, tokens int, result message.ToolResult) []message.Message {
	body := strings.Repeat("o", tokens*charsPerToken)
	result.Content = body + " " + label
	return []message.Message{
		{ID: "u-" + label, Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "turn " + label}}},
		{ID: "a-" + label, Role: message.Assistant, Parts: []message.ContentPart{
			message.ToolCall{ID: "call-" + label, Name: result.Name},
		}},
		{ID: "t-" + label, Role: message.Tool, Parts: []message.ContentPart{result}},
	}
}

// staleSession returns turns whose only large output is in the first one, with
// enough later turns that the first falls outside a two-turn protection.
func staleSession(turns ...[]message.Message) []message.Message {
	var msgs []message.Message
	for _, turn := range turns {
		msgs = append(msgs, turn...)
	}
	return msgs
}

func recent(label string) []message.Message {
	return toolTurn(label, 5, message.ToolResult{Name: "view", ToolCallID: "call-" + label})
}

func onlyResult(t *testing.T, msg message.Message) message.ToolResult {
	t.Helper()
	result, ok := msg.Parts[0].(message.ToolResult)
	require.True(t, ok, "expected a tool result part, got %T", msg.Parts[0])
	return result
}

func isPruned(result message.ToolResult) bool {
	return strings.HasPrefix(result.Content, prunePrefix)
}

func TestPruneDisabled(t *testing.T) {
	t.Parallel()

	msgs := staleSession(
		toolTurn("old", 4_000, message.ToolResult{Name: "view", ToolCallID: "call-old"}),
		[]message.Message{recent("r0")[0], recent("r0")[1], recent("r0")[2]},
		recent("r1"),
	)

	got, saved := Prune(msgs, PrunePolicy{Disabled: true, ProtectTokens: 1, MinimumReclaim: 1})
	require.Equal(t, msgs, got)
	require.Zero(t, saved)
}

func TestPruneReplacesStaleOutputAndKeepsTheCall(t *testing.T) {
	t.Parallel()

	msgs := staleSession(
		toolTurn("old", 4_000, message.ToolResult{Name: "view", ToolCallID: "call-old"}),
		recent("r0"),
		recent("r1"),
	)
	storedContent := onlyResult(t, msgs[2]).Content

	got, saved := Prune(msgs, testPolicy)
	require.Positive(t, saved)
	require.Len(t, got, len(msgs), "pruning rewrites, it never removes")

	pruned := onlyResult(t, got[2])
	require.True(t, isPruned(pruned), pruned.Content)
	require.Equal(t, "view", pruned.Name, "the model must still know what ran")
	require.Equal(t, "call-old", pruned.ToolCallID)
	require.Regexp(t, `~4[01]\d\d tokens`, pruned.Content)
	require.Less(t, len(pruned.Content), len(storedContent)/10)

	// Recent turns stay verbatim and the caller's stored messages are not
	// mutated underneath it.
	require.False(t, isPruned(onlyResult(t, got[5])))
	require.False(t, isPruned(onlyResult(t, got[8])))
	require.Equal(t, storedContent, onlyResult(t, msgs[2]).Content)
}

func TestPruneSkipsWhenSavingTooLittle(t *testing.T) {
	t.Parallel()

	msgs := staleSession(
		toolTurn("old", 700, message.ToolResult{Name: "grep", ToolCallID: "call-g"}),
		recent("r0"),
		recent("r1"),
	)
	policy := testPolicy
	policy.ProtectTokens = 100
	policy.MinimumReclaim = 5_000

	got, saved := Prune(msgs, policy)
	require.Equal(t, msgs, got, "a cache miss for 700 tokens is not a saving")
	require.Zero(t, saved)
}

func TestPruneProtectsRecentTurns(t *testing.T) {
	t.Parallel()

	var msgs []message.Message
	for i := range 5 {
		msgs = append(msgs, toolTurn(fmt.Sprint(i), 2_000, message.ToolResult{Name: "view", ToolCallID: fmt.Sprint(i)})...)
	}

	got, saved := Prune(msgs, testPolicy)
	require.Positive(t, saved)

	for i, msg := range msgs {
		result, ok := msg.Parts[0].(message.ToolResult)
		if !ok {
			continue
		}
		pruned := onlyResult(t, got[i])
		switch result.ToolCallID {
		case "3", "4":
			require.False(t, isPruned(pruned), "the last two turns stay verbatim")
		default:
			require.True(t, isPruned(pruned), "older output beyond the budget is pruned")
		}
	}
}

func TestPruneKeepsProtectedTools(t *testing.T) {
	t.Parallel()

	// Four turns: the oldest two are outside the protection, and only one of
	// them is worth keeping - the user's own answer.
	msgs := staleSession(
		toolTurn("asked", 4_000, message.ToolResult{Name: "question", ToolCallID: "call-q"}),
		toolTurn("listed", 4_000, message.ToolResult{Name: "view", ToolCallID: "call-v"}),
		recent("r0"),
		recent("r1"),
	)

	got, saved := Prune(msgs, testPolicy)
	require.Positive(t, saved)
	require.False(t, isPruned(onlyResult(t, got[2])), "the user's answers must never be discarded")
	require.True(t, isPruned(onlyResult(t, got[5])))
}

func TestPruneIsIdempotent(t *testing.T) {
	t.Parallel()

	msgs := staleSession(
		toolTurn("old", 4_000, message.ToolResult{Name: "view", ToolCallID: "call-old"}),
		recent("r0"),
		recent("r1"),
	)

	once, first := Prune(msgs, testPolicy)
	require.Positive(t, first)

	twice, second := Prune(once, testPolicy)
	require.Zero(t, second, "a second pass has nothing left to win")
	require.Equal(t, once, twice)
}

func TestPrunePreservesSpillPointerAndErrorLine(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("y", 4_000*charsPerToken)
	msgs := staleSession(
		[]message.Message{
			{ID: "u-b", Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "run"}}},
			{ID: "a-b", Role: message.Assistant, Parts: []message.ContentPart{message.ToolCall{ID: "c-b", Name: "bash"}}},
			{ID: "t-b", Role: message.Tool, Parts: []message.ContentPart{message.ToolResult{
				Name: "bash", ToolCallID: "c-b",
				Content: long + "\n... [900 lines truncated, full output: /data/.crush/shell-output/run.log] ...",
			}}},
			{ID: "u-e", Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "edit"}}},
			{ID: "a-e", Role: message.Assistant, Parts: []message.ContentPart{message.ToolCall{ID: "c-e", Name: "edit"}}},
			{ID: "t-e", Role: message.Tool, Parts: []message.ContentPart{message.ToolResult{
				Name: "edit", ToolCallID: "c-e", IsError: true,
				Content: "File has been modified since read\n" + strings.Repeat("z", 4_000*charsPerToken),
			}}},
		},
		recent("r0"),
		recent("r1"),
	)

	got, saved := Prune(msgs, testPolicy)
	require.Positive(t, saved)

	bash := onlyResult(t, got[2])
	require.Contains(t, bash.Content, "/data/.crush/shell-output/run.log",
		"the pointer to spilled bytes is the only way back to them")
	require.NotContains(t, bash.Content, strings.Repeat("y", 40))

	edit := onlyResult(t, got[5])
	require.Contains(t, edit.Content, "File has been modified since read",
		"why an attempt failed is worth keeping")
}

func TestPruneClearsBinaryPayloads(t *testing.T) {
	t.Parallel()

	msgs := staleSession(
		[]message.Message{
			{ID: "u-i", Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "look"}}},
			{ID: "a-i", Role: message.Assistant, Parts: []message.ContentPart{message.ToolCall{ID: "c-i", Name: "view"}}},
			{ID: "t-i", Role: message.Tool, Parts: []message.ContentPart{message.ToolResult{
				Name: "view", ToolCallID: "c-i", MIMEType: "image/png", Data: strings.Repeat("Q", 30_000),
			}}},
		},
		recent("r0"),
		recent("r1"),
	)

	got, saved := Prune(msgs, testPolicy)
	require.Positive(t, saved, "base64 payloads are counted by their wire size")
	require.Empty(t, onlyResult(t, got[2]).Data)
	require.Empty(t, onlyResult(t, got[2]).MIMEType)
	require.NotEmpty(t, onlyResult(t, msgs[2]).Data, "the stored message keeps the image")
}

func TestToolResultTokens(t *testing.T) {
	t.Parallel()

	require.Equal(t, int64(framingTokens+1), toolResultTokens(message.ToolResult{Content: "abcd"}))
	require.Equal(t, int64(framingTokens+2), toolResultTokens(message.ToolResult{Content: "abcd", Data: "efgh"}))
	// Metadata is bookkeeping for the TUI and never reaches a model.
	require.Equal(t, int64(framingTokens+1), toolResultTokens(message.ToolResult{Content: "abcd", Metadata: "efgh"}))
	// A screenshot is billed by tile, not by the length of its encoding.
	require.Equal(t, int64(framingTokens+imageTokens), toolResultTokens(message.ToolResult{
		MIMEType: "image/png", Data: strings.Repeat("Q", 400_000),
	}))
}

func TestPrunePolicyForWindow(t *testing.T) {
	t.Parallel()

	// A large model keeps the documented protection window.
	big := PrunePolicyForWindow(600_000, 0)
	require.Equal(t, DefaultPrunePolicy.ProtectTokens, big.ProtectTokens)
	require.False(t, big.Disabled)

	// A small model cannot protect more than its own tail, or pruning would
	// never act where it is needed most.
	small := PrunePolicyForWindow(65_536, 0)
	require.Less(t, small.ProtectTokens, DefaultPrunePolicy.ProtectTokens)
	require.GreaterOrEqual(t, small.ProtectTokens, int64(MinPruneProtectTokens))
	require.Less(t, small.MinimumReclaim, DefaultPrunePolicy.MinimumReclaim)

	// An explicit tail is honored instead of the ratio.
	tuned := PrunePolicyForWindow(600_000, 8_000)
	require.Equal(t, int64(4_000), tuned.ProtectTokens)

	// Nothing known about the window, nothing shrunk.
	require.Equal(t, DefaultPrunePolicy, PrunePolicyForWindow(0, 0))
}

func TestRegionLimit(t *testing.T) {
	t.Parallel()

	require.Equal(t, int64(360_000), RegionLimit(600_000))
	require.Zero(t, RegionLimit(0), "an unknown window bounds nothing")
	require.Less(t, RegionLimit(100_000), int64(100_000), "a request needs room for more than the region")
}

func TestTurnBoundaryProtectsShortSessionsEntirely(t *testing.T) {
	t.Parallel()

	msgs := staleSession(recent("one"), recent("two"), recent("three"))
	require.Zero(t, turnBoundary(msgs, 5), "fewer turns than the policy protects are protected in full")
	require.Zero(t, turnBoundary(msgs, 3))
	require.Equal(t, 3, turnBoundary(msgs, 2))
	require.Equal(t, 6, turnBoundary(msgs, 1))
	require.Zero(t, turnBoundary(msgs, 0))
}
