package tools

import (
	"context"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/mailbox"
	"github.com/charmbracelet/crush/internal/toolruntime"
	"github.com/stretchr/testify/require"
)

func TestSendMessageToolTargetsMailboxTask(t *testing.T) {
	t.Parallel()

	service := mailbox.NewService()
	require.NoError(t, service.Open("mb-1", []string{"task-a", "task-b"}))
	tool := NewSendMessageTool(service)

	resp, err := tool.Run(context.Background(), fantasy.ToolCall{
		ID:   "call-1",
		Name: SendMessageToolName,
		Input: `{
			"mailbox_id":"mb-1",
			"task_id":"task-a",
			"message":"sync status"
		}`,
	})
	require.NoError(t, err)
	require.False(t, resp.IsError)
	require.Contains(t, resp.Content, "task task-a")

	envelopes, err := service.Consume("mb-1", "task-a")
	require.NoError(t, err)
	require.Len(t, envelopes, 1)
	require.Equal(t, "sync status", envelopes[0].Message)
}

func TestSendMessageToolReturnsErrorForUnknownMailbox(t *testing.T) {
	t.Parallel()

	tool := NewSendMessageTool(mailbox.NewService())
	resp, err := tool.Run(context.Background(), fantasy.ToolCall{
		ID:    "call-1",
		Name:  SendMessageToolName,
		Input: `{"mailbox_id":"missing","message":"x"}`,
	})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, `mailbox "missing" not found`)
}

func TestSendMessageToolQueuesFollowUpPromptForBackgroundAgent(t *testing.T) {
	t.Parallel()

	tool := NewSendMessageTool(mailbox.NewService())
	called := false
	ctx := toolruntime.WithBackgroundAgentMessenger(context.Background(), func(_ context.Context, agentID, prompt string) (string, bool, error) {
		called = true
		require.Equal(t, "a-123", agentID)
		require.Equal(t, "continue with tests", prompt)
		return "queued", true, nil
	})
	ctx = context.WithValue(ctx, SessionIDContextKey, "session-1")
	ctx = context.WithValue(ctx, MessageIDContextKey, "msg-1")
	ctx = context.WithValue(ctx, ToolCallIDContextKey, "call-1")

	resp, err := tool.Run(ctx, fantasy.ToolCall{
		ID:    "call-1",
		Name:  SendMessageToolName,
		Input: `{"agent_id":"a-123","message":"continue with tests"}`,
	})
	require.NoError(t, err)
	require.True(t, called)
	require.False(t, resp.IsError)
	require.Contains(t, resp.Content, "queued for background agent a-123")
}

func TestSendMessageToolSupportsBackgroundAgentNameAddressing(t *testing.T) {
	t.Parallel()

	tool := NewSendMessageTool(mailbox.NewService())
	called := false
	ctx := toolruntime.WithBackgroundAgentMessenger(context.Background(), func(_ context.Context, agentID, prompt string) (string, bool, error) {
		called = true
		require.Equal(t, "researcher", agentID)
		require.Equal(t, "continue investigating", prompt)
		return "started", true, nil
	})

	resp, err := tool.Run(ctx, fantasy.ToolCall{
		ID:    "call-2",
		Name:  SendMessageToolName,
		Input: `{"agent_id":"researcher","message":"continue investigating"}`,
	})
	require.NoError(t, err)
	require.True(t, called)
	require.False(t, resp.IsError)
	require.Contains(t, resp.Content, "sent to background agent researcher")
}
