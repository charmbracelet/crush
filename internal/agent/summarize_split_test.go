package agent

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/compaction"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

// seedStepTurn creates one user turn running a given number of agentic steps.
// The bulk lives in the call inputs, unlike seedToolTurn: pruning rewrites tool
// results, so a tail made of results would shrink on its way into the planner
// and the test would pass without ever needing to split.
func seedStepTurn(t *testing.T, env fakeEnv, sessionID string, steps, callTokens int) {
	t.Helper()
	ctx := t.Context()

	_, err := env.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "run the long job"}},
	})
	require.NoError(t, err)

	for i := range steps {
		call := message.ToolCall{
			ID:    fmt.Sprintf("tc-%d", i),
			Name:  "view",
			Input: strings.Repeat("i", callTokens*4),
		}
		_, err = env.messages.Create(ctx, sessionID, message.CreateMessageParams{
			Role:  message.Assistant,
			Parts: []message.ContentPart{call},
		})
		require.NoError(t, err)
		_, err = env.messages.Create(ctx, sessionID, message.CreateMessageParams{
			Role: message.Tool,
			Parts: []message.ContentPart{message.ToolResult{
				ToolCallID: call.ID,
				Name:       call.Name,
				Content:    "done",
			}},
		})
		require.NoError(t, err)
	}
}

// TestSummarizeSplitsATurnTooBigToRetain is the long-autonomous-session case.
// One turn larger than the tail ceiling used to be retained whole, because a
// turn boundary was the only legal cut point. It is now cut inside the turn,
// and the cut must not strand a tool result away from its call.
func TestSummarizeSplitsATurnTooBigToRetain(t *testing.T) {
	env := testEnv(t)
	model := &recordingModel{text: "the checkpoint"}
	sa := testSessionAgent(env, model, nil, "test prompt").(*sessionAgent)
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)
	// A settled turn, then one turn of roughly 160k tokens against a 30k tail
	// budget and its 60k ceiling for the 200k window the test model reports.
	seedSizedTurns(t, env, sess.ID, 1, 200)
	seedStepTurn(t, env, sess.ID, 20, 4_000)

	require.NoError(t, sa.Summarize(ctx, sess.ID, "", fantasy.ProviderOptions{}, nil))

	updated, err := env.sessions.Get(ctx, sess.ID)
	require.NoError(t, err)
	require.NotEmpty(t, updated.SummaryCutMessageID)

	view, err := sa.getSessionMessages(ctx, updated)
	require.NoError(t, err)
	require.True(t, view[0].IsSummaryMessage, "the checkpoint leads the session")

	tail := view[1:]
	require.Equal(t, message.Assistant, tail[0].Role,
		"the tail starts inside the oversized turn rather than swallowing it whole")
	require.LessOrEqual(t, compaction.EstimateAll(tail), 2*compaction.TailBudget(200_000, 0),
		"a split tail still has to fit the ceiling")

	calls := map[string]bool{}
	for _, msg := range tail {
		for _, part := range msg.Parts {
			if call, ok := part.(message.ToolCall); ok {
				calls[call.ID] = true
			}
		}
	}
	for _, msg := range tail {
		for _, part := range msg.Parts {
			if result, ok := part.(message.ToolResult); ok {
				require.True(t, calls[result.ToolCallID],
					"the retained tail carries a result whose call was summarized away")
			}
		}
	}

	require.NotEmpty(t, model.calls)
	require.Contains(t, model.calls[0], "tc-0", "the steps behind the cut are what gets checkpointed")
}
