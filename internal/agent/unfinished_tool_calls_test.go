package agent

import (
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

// A turn stopped at its output token limit finishes with FinishReasonLength,
// so nothing on the error path settles the call it left unfinished.
func TestCloseUnfinishedToolCalls(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	agent := &sessionAgent{messages: env.messages}

	sess, err := env.sessions.Create(t.Context(), "t")
	require.NoError(t, err)

	assistant, err := env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role:  message.Assistant,
		Parts: []message.ContentPart{},
	})
	require.NoError(t, err)

	// One call that landed normally, one cut off mid arguments.
	assistant.AddToolCall(message.ToolCall{ID: "done", Name: "view", Input: `{"file_path":"a"}`, Finished: true})
	assistant.AddToolCall(message.ToolCall{ID: "cutoff", Name: "write", Finished: false})
	require.NoError(t, env.messages.Update(t.Context(), assistant))

	require.NoError(t, agent.closeUnfinishedToolCalls(t.Context(), &assistant))

	calls := assistant.ToolCalls()
	require.Len(t, calls, 2)
	for _, tc := range calls {
		require.True(t, tc.Finished, "%s should be finished", tc.ID)
		require.NotEmpty(t, tc.Input, "%s should carry parseable arguments", tc.ID)
	}

	// Without a result the next request carries a tool call with no reply,
	// which providers reject.
	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	var results []message.ToolResult
	for _, m := range msgs {
		if m.Role == message.Tool {
			results = append(results, m.ToolResults()...)
		}
	}
	require.Len(t, results, 1)
	require.Equal(t, "cutoff", results[0].ToolCallID)
	require.True(t, results[0].IsError)
	require.Equal(t, truncatedToolCallResult, results[0].Content)
}

// Completed turns must be left alone, and must not gain tool result messages.
func TestCloseUnfinishedToolCallsLeavesCompleteTurnsAlone(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	agent := &sessionAgent{messages: env.messages}

	sess, err := env.sessions.Create(t.Context(), "t")
	require.NoError(t, err)

	assistant, err := env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role:  message.Assistant,
		Parts: []message.ContentPart{},
	})
	require.NoError(t, err)
	assistant.AddToolCall(message.ToolCall{ID: "done", Name: "view", Input: `{}`, Finished: true})
	require.NoError(t, env.messages.Update(t.Context(), assistant))

	require.NoError(t, agent.closeUnfinishedToolCalls(t.Context(), &assistant))

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	for _, m := range msgs {
		require.NotEqual(t, message.Tool, m.Role, "no tool result should be invented")
	}
}
