package mailbox

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServiceSendAndConsumeTargeted(t *testing.T) {
	t.Parallel()

	svc := NewService()
	require.NoError(t, svc.Open("mb-1", []string{"a", "b"}))

	env, err := svc.Send("mb-1", "a", "hello")
	require.NoError(t, err)
	require.Equal(t, EnvelopeKindMessage, env.Kind)
	require.Equal(t, "a", env.TargetTaskID)

	messagesA, err := svc.Consume("mb-1", "a")
	require.NoError(t, err)
	require.Len(t, messagesA, 1)
	require.Equal(t, "hello", messagesA[0].Message)

	messagesB, err := svc.Consume("mb-1", "b")
	require.NoError(t, err)
	require.Empty(t, messagesB)
}

func TestServiceBroadcastAndStop(t *testing.T) {
	t.Parallel()

	svc := NewService()
	require.NoError(t, svc.Open("mb-2", []string{"a", "b"}))

	_, err := svc.Send("mb-2", "", "broadcast")
	require.NoError(t, err)
	stop, err := svc.Stop("mb-2", "", "manual")
	require.NoError(t, err)
	require.Equal(t, EnvelopeKindStop, stop.Kind)

	messagesA, err := svc.Consume("mb-2", "a")
	require.NoError(t, err)
	require.Len(t, messagesA, 2)
	require.Equal(t, EnvelopeKindMessage, messagesA[0].Kind)
	require.Equal(t, EnvelopeKindStop, messagesA[1].Kind)
	require.Equal(t, "manual", messagesA[1].Reason)

	messagesB, err := svc.Consume("mb-2", "b")
	require.NoError(t, err)
	require.Len(t, messagesB, 2)
}

func TestServiceErrorsForUnknownMailboxOrTask(t *testing.T) {
	t.Parallel()

	svc := NewService()
	require.NoError(t, svc.Open("mb-3", []string{"a"}))

	_, err := svc.Send("missing", "a", "x")
	require.ErrorContains(t, err, `mailbox "missing" not found`)

	_, err = svc.Send("mb-3", "missing", "x")
	require.ErrorContains(t, err, `task "missing" not found in mailbox "mb-3"`)

	err = svc.Open("mb-4", nil)
	require.ErrorContains(t, err, "task_ids is required")
}
