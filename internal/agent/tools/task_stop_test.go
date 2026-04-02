package tools

import (
	"context"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/mailbox"
	"github.com/stretchr/testify/require"
)

func TestTaskStopToolTargetsMailboxTask(t *testing.T) {
	t.Parallel()

	service := mailbox.NewService()
	require.NoError(t, service.Open("mb-1", []string{"task-a", "task-b"}))
	tool := NewTaskStopTool(service)

	resp, err := tool.Run(context.Background(), fantasy.ToolCall{
		ID:    "call-1",
		Name:  TaskStopToolName,
		Input: `{"mailbox_id":"mb-1","task_id":"task-b","reason":"manual stop"}`,
	})
	require.NoError(t, err)
	require.False(t, resp.IsError)
	require.Contains(t, resp.Content, "task task-b")

	envelopes, err := service.Consume("mb-1", "task-b")
	require.NoError(t, err)
	require.Len(t, envelopes, 1)
	require.Equal(t, mailbox.EnvelopeKindStop, envelopes[0].Kind)
	require.Equal(t, "manual stop", envelopes[0].Reason)
}

func TestTaskStopToolValidatesMailboxID(t *testing.T) {
	t.Parallel()

	tool := NewTaskStopTool(mailbox.NewService())
	resp, err := tool.Run(context.Background(), fantasy.ToolCall{
		ID:    "call-1",
		Name:  TaskStopToolName,
		Input: `{"reason":"x"}`,
	})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "mailbox_id is required")
}
