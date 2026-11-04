package agent

import (
	"context"
	"testing"
	"time"

	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

// TestRun_QueueOnlyCallBehindActiveRunRunsAsFollowUpTurn proves a QueueOnly
// call queued behind a busy session is replayed as its own turn once the
// active turn ends, instead of failing with ErrSessionIdle.
func TestRun_QueueOnlyCallBehindActiveRunRunsAsFollowUpTurn(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	large := &gatedStreamModel{
		text:    "done",
		gate:    make(chan struct{}),
		entered: make(chan struct{}),
	}
	small := &finishStreamModel{text: "title"}
	sa := NewSessionAgent(SessionAgentOptions{
		Models: fixedModels(
			Model{Model: large, CatwalkCfg: catwalk.Model{ContextWindow: 200000, DefaultMaxTokens: 10000}},
			Model{Model: small, CatwalkCfg: catwalk.Model{ContextWindow: 200000, DefaultMaxTokens: 10000}},
		),
		Sessions: env.sessions,
		Messages: env.messages,
	}).(*sessionAgent)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	mainDone := make(chan error, 1)
	go func() {
		_, runErr := sa.Run(t.Context(), SessionAgentCall{
			SessionID: sess.ID,
			Prompt:    "main",
		})
		mainDone <- runErr
	}()

	select {
	case <-large.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("main run never entered Stream")
	}

	res, err := sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "queued-only",
		QueueOnly: true,
	})
	require.NoError(t, err)
	require.Nil(t, res, "a busy-session QueueOnly call must enqueue and return (nil, nil)")

	close(large.gate)
	select {
	case runErr := <-mainDone:
		require.NoError(t, runErr, "the queued QueueOnly turn must not fail with ErrSessionIdle")
	case <-time.After(5 * time.Second):
		t.Fatal("main run never returned")
	}

	require.Equal(t, int64(2), large.calls.Load(), "the queued prompt must reach the model")
	msgs, err := env.messages.List(context.Background(), sess.ID)
	require.NoError(t, err)
	var assistants, queued int
	for _, m := range msgs {
		switch m.Role {
		case message.Assistant:
			assistants++
		case message.User:
			if m.Content().String() == "queued-only" {
				queued++
			}
		}
	}
	require.Equal(t, 1, queued, "the queued prompt must be persisted as a user message")
	require.Equal(t, 2, assistants, "the queued prompt must run as its own assistant turn")
}
