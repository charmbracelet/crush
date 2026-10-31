package agent

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// compactingStreamModel fills the context twice, with a summary between each
// tool step, before completing the resumed turn.
type compactingStreamModel struct {
	finishStreamModel
	summary string
	prompts []fantasy.Prompt
}

func (m *compactingStreamModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	step := len(m.prompts)
	if step >= 5 {
		return nil, fmt.Errorf("unexpected continuation after two compactions")
	}
	m.prompts = append(m.prompts, cloneFantasyMessages(call.Prompt))
	if step == 0 || step == 2 {
		return func(yield func(fantasy.StreamPart) bool) {
			if !yield(fantasy.StreamPart{
				Type:          fantasy.StreamPartTypeToolCall,
				ID:            fmt.Sprintf("call-%d", step),
				ToolCallName:  "inspect",
				ToolCallInput: "{}",
			}) {
				return
			}
			yield(fantasy.StreamPart{
				Type:         fantasy.StreamPartTypeFinish,
				FinishReason: fantasy.FinishReasonToolCalls,
				Usage:        fantasy.Usage{InputTokens: 190000, OutputTokens: 1, TotalTokens: 190001},
			})
		}, nil
	}
	text := m.summary
	if step == 4 {
		text = "done"
	}
	return (&finishStreamModel{text: text}).Stream(ctx, call)
}

func TestRun_AutoSummaryContinuationPreservesTaskContext(t *testing.T) {
	t.Parallel()

	const original = "Find and fix the race in the worker pool."
	const followUp = "Your test is stuck. Add timeouts."
	env := testEnv(t)
	model := &compactingStreamModel{summary: original + " Latest instruction: " + followUp}
	inspect := fantasy.NewAgentTool("inspect", "Inspect the worker pool", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
		return fantasy.NewTextResponse("the worker pool needs a fix"), nil
	})
	sa := testSessionAgent(env, model, model, "system", inspect)
	sess, err := env.sessions.Create(t.Context(), "worker pool")
	require.NoError(t, err)
	_, err = env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: original}},
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	_, err = sa.Run(ctx, SessionAgentCall{SessionID: sess.ID, Prompt: followUp})
	require.NoError(t, err)
	require.Len(t, model.prompts, 5, "two compactions must each resume the tool turn")

	for _, step := range []int{2, 4} {
		var text strings.Builder
		for _, msg := range model.prompts[step] {
			for _, part := range msg.Content {
				if part, ok := fantasy.AsMessagePart[fantasy.TextPart](part); ok {
					text.WriteString(part.Text)
					text.WriteByte('\n')
				}
			}
		}
		require.Contains(t, text.String(), original, "the summary must retain the task")
		require.Contains(t, text.String(), followUp, "the summary must retain the correction")
		assert.NotContains(t, text.String(), "initial user request was:", "a follow-up must not replace the task objective")
		last := model.prompts[step][len(model.prompts[step])-1]
		require.Equal(t, fantasy.MessageRoleUser, last.Role)
		continuation, ok := fantasy.AsMessagePart[fantasy.TextPart](last.Content[0])
		require.True(t, ok)
		assert.Equal(t, 1, strings.Count(continuation.Text, "previous session was interrupted"), "continuations must not nest across compactions")
	}

	saved, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	var summaries int
	for _, msg := range saved {
		if msg.IsSummaryMessage {
			summaries++
		}
	}
	require.Equal(t, 2, summaries)
	require.Zero(t, sa.QueuedPrompts(sess.ID))
	require.False(t, sa.IsSessionBusy(sess.ID))
}
