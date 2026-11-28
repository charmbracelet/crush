package agent

import (
	"context"
	"fmt"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

// repeatedCompactionModel overflows the context on the first limit turns
// that offer tools, then finishes. Summary requests offer no tools.
type repeatedCompactionModel struct {
	finishStreamModel
	limit     int
	overflows int
}

func (m *repeatedCompactionModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	if len(call.Tools) == 0 || m.overflows == m.limit {
		return m.finishStreamModel.Stream(ctx, call)
	}
	m.overflows++
	id := fmt.Sprintf("call-%d", m.overflows)
	return func(yield func(fantasy.StreamPart) bool) {
		if !yield(fantasy.StreamPart{
			Type:          fantasy.StreamPartTypeToolCall,
			ID:            id,
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
	large := &repeatedCompactionModel{finishStreamModel: finishStreamModel{text: "done"}, limit: 2}
	small := &finishStreamModel{text: "title"}
	probe := fantasy.NewAgentTool("probe", "Return a test result.", func(_ context.Context, _ struct{}, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
		return fantasy.NewTextResponse("ok"), nil
	})
	agent := testSessionAgent(env, large, small, "system", probe)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)
	call := SessionAgentCall{SessionID: sess.ID, Prompt: "Inspect the workspace."}
	_, err = agent.Run(t.Context(), call)
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
	call.resumed = true
	note := call.modelPrompt()
	require.Equal(t, 2, summaries)
	require.Equal(t, []string{"Inspect the workspace.", note, note}, prompts)
}
