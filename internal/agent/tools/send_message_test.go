package tools

import (
	"context"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/mailbox"
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
