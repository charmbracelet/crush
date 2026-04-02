package agent

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/mailbox"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

func TestTaskGraphMailboxBridgeUpdatesTodosAndConsumesMessages(t *testing.T) {
	env := testEnv(t)
	sessionItem, err := env.sessions.Create(context.Background(), "mailbox-bridge")
	require.NoError(t, err)
	svc := mailbox.NewService()
	bridge, err := newTaskGraphMailboxBridge(svc, env.sessions, sessionItem.ID, "mb-1", []taskGraphTask{{ID: "a", Description: "Task A"}})
	require.NoError(t, err)
	defer bridge.Close()

	bridge.MarkPending("a")
	bridge.MarkInProgress("a")
	_, err = svc.Send("mb-1", "a", "sync")
	require.NoError(t, err)

	effects, err := bridge.Consume("a")
	require.NoError(t, err)
	require.Equal(t, []string{"sync"}, effects.Messages)
	require.False(t, effects.Stop)

	bridge.MarkResult("a", message.ToolResultSubtaskStatusCompleted, "done")

	sess, err := env.sessions.Get(context.Background(), sessionItem.ID)
	require.NoError(t, err)
	require.Len(t, sess.Todos, 1)
	require.Equal(t, "a", sess.Todos[0].ID)
	require.Equal(t, session.TodoStatusCompleted, sess.Todos[0].Status)
	require.Equal(t, 100, sess.Todos[0].Progress)
	require.Contains(t, sess.Todos[0].Content, "mailbox:sync")
}

func TestTaskGraphMailboxBridgeStopEffect(t *testing.T) {
	env := testEnv(t)
	sessionItem, err := env.sessions.Create(context.Background(), "mailbox-bridge-stop")
	require.NoError(t, err)
	svc := mailbox.NewService()
	bridge, err := newTaskGraphMailboxBridge(svc, env.sessions, sessionItem.ID, "mb-2", []taskGraphTask{{ID: "a", Description: "Task A"}})
	require.NoError(t, err)
	defer bridge.Close()

	_, err = svc.Stop("mb-2", "a", "cancel now")
	require.NoError(t, err)
	effects, err := bridge.Consume("a")
	require.NoError(t, err)
	require.True(t, effects.Stop)
	require.Equal(t, "cancel now", effects.Reason)

	sess, err := env.sessions.Get(context.Background(), sessionItem.ID)
	require.NoError(t, err)
	require.Len(t, sess.Todos, 1)
	require.Equal(t, session.TodoStatusCanceled, sess.Todos[0].Status)
}
