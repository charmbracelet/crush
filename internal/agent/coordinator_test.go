package agent

import (
	"context"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

func TestRecoverSession(t *testing.T) {
	t.Run("no messages", func(t *testing.T) {
		env := testEnv(t)

		sess, err := env.sessions.Create(t.Context(), "Test Session")
		require.NoError(t, err)

		// Create coordinator with mock services
		coordinator := &coordinator{
			sessions: env.sessions,
			messages: env.messages,
		}

		err = coordinator.RecoverSession(t.Context(), sess.ID)
		require.NoError(t, err)

		// Verify no messages were modified
		msgs, err := env.messages.List(t.Context(), sess.ID)
		require.NoError(t, err)
		require.Empty(t, msgs)
	})

	t.Run("already finished messages", func(t *testing.T) {
		env := testEnv(t)

		sess, err := env.sessions.Create(t.Context(), "Test Session")
		require.NoError(t, err)

		// Create a finished assistant message (with Finish part)
		_, err = env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
			Role:  message.Assistant,
			Parts: []message.ContentPart{message.TextContent{Text: "Hello!"}, message.Finish{Reason: message.FinishReasonEndTurn}},
		})
		require.NoError(t, err)

		// Create coordinator with mock services
		coordinator := &coordinator{
			sessions: env.sessions,
			messages: env.messages,
		}

		err = coordinator.RecoverSession(t.Context(), sess.ID)
		require.NoError(t, err)

		// Verify the message was not modified
		msgs, err := env.messages.List(t.Context(), sess.ID)
		require.NoError(t, err)
		require.Len(t, msgs, 1)
		require.True(t, msgs[0].IsFinished())
	})

	t.Run("incomplete summary message", func(t *testing.T) {
		env := testEnv(t)

		sess, err := env.sessions.Create(t.Context(), "Test Session")
		require.NoError(t, err)

		// Create an incomplete summary message (simulating a crash during summarization)
		summaryMsg, err := env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
			Role:             message.Assistant,
			Parts:            []message.ContentPart{message.TextContent{Text: "Partial summary..."}},
			Model:            "test-model",
			Provider:         "test-provider",
			IsSummaryMessage: true,
		})
		require.NoError(t, err)

		// Verify the message is not finished
		require.False(t, summaryMsg.IsFinished())

		// Create coordinator with mock services
		coordinator := &coordinator{
			sessions: env.sessions,
			messages: env.messages,
		}

		err = coordinator.RecoverSession(t.Context(), sess.ID)
		require.NoError(t, err)

		// Verify the summary message was recovered
		recoveredMsg, err := env.messages.Get(t.Context(), summaryMsg.ID)
		require.NoError(t, err)
		require.True(t, recoveredMsg.IsFinished())
		require.Equal(t, message.FinishReasonError, recoveredMsg.FinishReason())
		require.Contains(t, recoveredMsg.FinishPart().Message, "Session interrupted")
	})

	t.Run("incomplete assistant message with tool calls", func(t *testing.T) {
		env := testEnv(t)

		sess, err := env.sessions.Create(t.Context(), "Test Session")
		require.NoError(t, err)

		// Create an incomplete assistant message with tool calls
		// (simulating a crash during tool execution)
		toolCall := message.ToolCall{
			ID:               "tc-1",
			Name:             "bash",
			Input:            `echo "hello"`,
			ProviderExecuted: false,
			Finished:         false,
		}

		assistantMsg, err := env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
			Role:  message.Assistant,
			Parts: []message.ContentPart{message.ToolCall(toolCall)},
			Model: "test-model",
		})
		require.NoError(t, err)

		// Verify the message is not finished
		require.False(t, assistantMsg.IsFinished())

		// Create coordinator with mock services
		coordinator := &coordinator{
			sessions: env.sessions,
			messages: env.messages,
		}

		err = coordinator.RecoverSession(t.Context(), sess.ID)
		require.NoError(t, err)

		// Verify the assistant message was recovered
		recoveredMsg, err := env.messages.Get(t.Context(), assistantMsg.ID)
		require.NoError(t, err)
		require.True(t, recoveredMsg.IsFinished())
		require.Equal(t, message.FinishReasonError, recoveredMsg.FinishReason())
		require.Contains(t, recoveredMsg.FinishPart().Message, "Session interrupted")

		// Verify the tool call was marked as finished
		toolCalls := recoveredMsg.ToolCalls()
		require.Len(t, toolCalls, 1)
		require.True(t, toolCalls[0].Finished)
	})

	t.Run("incomplete assistant message without tool calls", func(t *testing.T) {
		env := testEnv(t)

		sess, err := env.sessions.Create(t.Context(), "Test Session")
		require.NoError(t, err)

		// Create an incomplete assistant message with partial content but no tool calls
		assistantMsg, err := env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
			Role:  message.Assistant,
			Parts: []message.ContentPart{message.TextContent{Text: "This is a partial response..."}},
			Model: "test-model",
		})
		require.NoError(t, err)

		// Verify the message is not finished
		require.False(t, assistantMsg.IsFinished())

		// Create coordinator with mock services
		coordinator := &coordinator{
			sessions: env.sessions,
			messages: env.messages,
		}

		err = coordinator.RecoverSession(t.Context(), sess.ID)
		require.NoError(t, err)

		// Verify the assistant message was recovered
		recoveredMsg, err := env.messages.Get(t.Context(), assistantMsg.ID)
		require.NoError(t, err)
		require.True(t, recoveredMsg.IsFinished())
		require.Equal(t, message.FinishReasonError, recoveredMsg.FinishReason())
		require.Contains(t, recoveredMsg.FinishPart().Message, "Session interrupted")
		require.Equal(t, "This is a partial response...", recoveredMsg.Content().Text)
	})

	t.Run("session is busy - skips recovery", func(t *testing.T) {
		env := testEnv(t)

		sess, err := env.sessions.Create(t.Context(), "Test Session")
		require.NoError(t, err)

		// Create a dummy agent that reports as busy
		agent := &dummyAgent{t: t, isBusy: true}

		coordinator := &coordinator{
			sessions:     env.sessions,
			messages:     env.messages,
			currentAgent: agent,
		}

		// Create an incomplete assistant message
		_, err = env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
			Role:  message.Assistant,
			Parts: []message.ContentPart{message.TextContent{Text: "Partial..."}},
			Model: "test-model",
		})
		require.NoError(t, err)

		err = coordinator.RecoverSession(t.Context(), sess.ID)
		require.NoError(t, err)

		// Message should NOT be recovered since session is "busy"
		msgs, err := env.messages.List(t.Context(), sess.ID)
		require.NoError(t, err)
		require.Len(t, msgs, 1)
		require.False(t, msgs[0].IsFinished(), "message should not be finished when session is busy")
	})

	t.Run("multiple incomplete messages", func(t *testing.T) {
		env := testEnv(t)

		sess, err := env.sessions.Create(t.Context(), "Test Session")
		require.NoError(t, err)

		// Create an incomplete summary message
		_, err = env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
			Role:             message.Assistant,
			Parts:            []message.ContentPart{message.TextContent{Text: "Partial summary..."}},
			IsSummaryMessage: true,
		})
		require.NoError(t, err)

		// Create an incomplete assistant message with tool calls
		toolCall := message.ToolCall{
			ID:               "tc-1",
			Name:             "bash",
			Input:            `echo "hello"`,
			ProviderExecuted: false,
			Finished:         false,
		}
		_, err = env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
			Role:  message.Assistant,
			Parts: []message.ContentPart{message.ToolCall(toolCall)},
		})
		require.NoError(t, err)

		coordinator := &coordinator{
			sessions: env.sessions,
			messages: env.messages,
		}

		err = coordinator.RecoverSession(t.Context(), sess.ID)
		require.NoError(t, err)

		// Verify both messages were recovered
		msgs, err := env.messages.List(t.Context(), sess.ID)
		require.NoError(t, err)
		require.Len(t, msgs, 2)

		for _, msg := range msgs {
			require.True(t, msg.IsFinished(), "message %s should be finished", msg.ID)
		}
	})

	t.Run("mixed finished and unfinished messages", func(t *testing.T) {
		env := testEnv(t)

		sess, err := env.sessions.Create(t.Context(), "Test Session")
		require.NoError(t, err)

		// Create a finished user message (Finish part is added automatically)
		_, err = env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
			Role:  message.User,
			Parts: []message.ContentPart{message.TextContent{Text: "Hello!"}},
		})
		require.NoError(t, err)

		// Create a finished assistant message (with Finish part)
		_, err = env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
			Role:  message.Assistant,
			Parts: []message.ContentPart{message.TextContent{Text: "Hi there!"}, message.Finish{Reason: message.FinishReasonEndTurn}},
		})
		require.NoError(t, err)

		// Create an incomplete assistant message with tool calls
		toolCall := message.ToolCall{
			ID:               "tc-1",
			Name:             "bash",
			Input:            `echo "hello"`,
			ProviderExecuted: false,
			Finished:         false,
		}
		_, err = env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
			Role:  message.Assistant,
			Parts: []message.ContentPart{message.ToolCall(toolCall)},
		})
		require.NoError(t, err)

		coordinator := &coordinator{
			sessions: env.sessions,
			messages: env.messages,
		}

		err = coordinator.RecoverSession(t.Context(), sess.ID)
		require.NoError(t, err)

		// Verify all messages are now correct
		msgs, err := env.messages.List(t.Context(), sess.ID)
		require.NoError(t, err)
		require.Len(t, msgs, 3)

		// User message should be finished (was already)
		require.True(t, msgs[0].IsFinished())

		// First assistant message should be finished (was already)
		require.True(t, msgs[1].IsFinished())

		// Second assistant message should now be finished
		require.True(t, msgs[2].IsFinished())
	})
}

// dummyAgent implements SessionAgent for testing purposes.
type dummyAgent struct {
	t      *testing.T
	isBusy bool
}

func (a *dummyAgent) Run(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
	return nil, nil
}

func (a *dummyAgent) SetModels(large, small Model) {}

func (a *dummyAgent) SetTools(tools []fantasy.AgentTool) {}

func (a *dummyAgent) SetSystemPrompt(systemPrompt string) {}

func (a *dummyAgent) Cancel(sessionID string) {}

func (a *dummyAgent) CancelAll() {}

func (a *dummyAgent) IsSessionBusy(sessionID string) bool {
	return a.isBusy
}

func (a *dummyAgent) IsBusy() bool {
	return a.isBusy
}

func (a *dummyAgent) QueuedPrompts(sessionID string) int {
	return 0
}

func (a *dummyAgent) QueuedPromptsList(sessionID string) []string {
	return nil
}

func (a *dummyAgent) ClearQueue(sessionID string) {}

func (a *dummyAgent) Summarize(ctx context.Context, sessionID string, opts fantasy.ProviderOptions) error {
	return nil
}

func (a *dummyAgent) Model() Model {
	return Model{}
}
