package agent

import (
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

func TestSessionAgentResetRetriedStepClearsToolState(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	agent := &sessionAgent{
		messages: env.messages,
	}

	sess, err := env.sessions.Create(t.Context(), "Retry Cleanup")
	require.NoError(t, err)

	assistant, err := env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: "partial"},
			message.ToolCall{
				ID:       "tool-1",
				Name:     "ls",
				Input:    "{}",
				Finished: true,
			},
		},
	})
	require.NoError(t, err)

	toolMsg, err := env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role: message.Tool,
		Parts: []message.ContentPart{
			message.ToolResult{
				ToolCallID: "tool-1",
				Name:       "ls",
				Content:    "stale result",
			},
		},
	})
	require.NoError(t, err)

	err = agent.resetRetriedStep(t.Context(), &assistant, []string{toolMsg.ID})
	require.NoError(t, err)

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Len(t, msgs, 1)

	updatedAssistant, err := env.messages.Get(t.Context(), assistant.ID)
	require.NoError(t, err)
	require.Empty(t, updatedAssistant.Parts)
}
