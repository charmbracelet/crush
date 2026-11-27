package agent

import (
	"context"
	"fmt"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

type repeatedCompactionModel struct {
	finishStreamModel
	streams int
}

func (m *repeatedCompactionModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	m.streams++
	step := m.streams
	// The first two turns hit the context limit with a tool call. The
	// intervening requests generate summaries, and the last turn finishes.
	if step != 1 && step != 3 {
		return m.finishStreamModel.Stream(ctx, call)
	}
	return func(yield func(fantasy.StreamPart) bool) {
		if !yield(fantasy.StreamPart{
			Type:          fantasy.StreamPartTypeToolCall,
			ID:            fmt.Sprintf("call-%d", step),
			ToolCallName:  "probe",
			ToolCallInput: "{}",
		}) {
			return
		}
		yield(fantasy.StreamPart{
			Type:         fantasy.StreamPartTypeFinish,
			FinishReason: fantasy.FinishReasonToolCalls,
			Usage:        fantasy.Usage{InputTokens: 190_000, OutputTokens: 1},
		})
	}, nil
}

func TestAutoCompactionContinuationKeepsOriginalPrompt(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	large := &repeatedCompactionModel{finishStreamModel: finishStreamModel{text: "done"}}
	small := &finishStreamModel{text: "title"}
	probe := fantasy.NewAgentTool("probe", "Return a test result.", func(_ context.Context, _ struct{}, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
		return fantasy.NewTextResponse("ok"), nil
	})
	agent := testSessionAgent(env, large, small, "system", probe)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)
	const prompt = "Inspect the workspace."
	const note = "The previous session was interrupted because it got too long, the initial user request was: `Inspect the workspace.`"
	_, err = agent.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: prompt})
	require.NoError(t, err)

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	var prompts []string
	var summaries int
	for _, msg := range msgs {
		if msg.Role == message.User {
			prompts = append(prompts, msg.Content().Text)
		}
		if msg.IsSummaryMessage {
			summaries++
		}
	}
	require.Equal(t, 2, summaries)
	require.Len(t, prompts, 3)
	require.Equal(t, prompt, prompts[0])
	require.Equal(t, note, prompts[1], "first compaction")
	require.Equal(t, note, prompts[2], "second compaction")
}
