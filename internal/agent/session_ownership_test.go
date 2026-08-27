package agent

import (
	"testing"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"

	"github.com/stretchr/testify/require"
)

// sharedOwnershipAgent builds a session agent on ownership shared with
// its siblings, which is how a coordinator builds the coder agent, the
// `agent` tool's sub-agent, and agenticFetchTool's inline agent.
func sharedOwnershipAgent(t *testing.T, env fakeEnv, own *sessionOwnership, large fantasy.LanguageModel) *sessionAgent {
	t.Helper()
	return NewSessionAgent(SessionAgentOptions{
		LargeModel: Model{Model: large, CatwalkCfg: catwalk.Model{ContextWindow: 200000, DefaultMaxTokens: 10000}},
		SmallModel: Model{Model: &finishStreamModel{text: "title"}, CatwalkCfg: catwalk.Model{ContextWindow: 200000, DefaultMaxTokens: 10000}},
		Sessions:   env.sessions,
		Messages:   env.messages,
		Ownership:  own,
	}).(*sessionAgent)
}

// startGatedTurn runs a turn that parks inside Stream, so a test can act
// on a session that is busy on another agent instance. Wait on the
// model's `entered` channel before asserting anything.
func startGatedTurn(t *testing.T, sa *sessionAgent, sessionID string) chan error {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		_, err := sa.Run(t.Context(), SessionAgentCall{
			SessionID: sessionID,
			RunID:     "run-child",
			Prompt:    "child work",
		})
		done <- err
	}()
	return done
}

// TestSharedOwnership_ChildSessionIsVisibleToItsSiblings covers the
// bookkeeping half of the defect. A sub-agent turn runs on a different
// sessionAgent instance than the coder agent the UI and the server talk
// to. While each instance kept its own run bookkeeping, that child
// session looked idle from the outside: nothing could report it busy and
// nothing could cancel it.
func TestSharedOwnership_ChildSessionIsVisibleToItsSiblings(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	own := newSessionOwnership()

	sess, err := env.sessions.Create(t.Context(), "child")
	require.NoError(t, err)

	large := &gatedToolStreamModel{
		gate:       make(chan struct{}),
		entered:    make(chan struct{}),
		toolName:   "noop",
		toolInput:  `{"value":"hi"}`,
		toolCallID: "call",
	}
	subAgent := sharedOwnershipAgent(t, env, own, large)
	coder := sharedOwnershipAgent(t, env, own, &finishStreamModel{text: "coder"})

	done := startGatedTurn(t, subAgent, sess.ID)
	select {
	case <-large.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("sub-agent run never entered Stream")
	}

	require.True(t, coder.IsSessionBusy(sess.ID),
		"a turn running on a sibling agent must still make its session busy")

	// The sibling's cancel must reach the run, which is what makes
	// Escape and a dropped client stop a sub-agent turn.
	coder.Cancel(sess.ID)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("cancel from a sibling agent never reached the running turn")
	}
	require.False(t, coder.IsSessionBusy(sess.ID))
}

// TestSharedOwnership_PromptIntoABusyChildSessionQueues covers the other
// half: the busy check that decides whether to queue a prompt or start a
// turn reads the agent's own bookkeeping. While that was per instance, a
// prompt dispatched into a session already running on the sub-agent
// passed the check and started a second concurrent turn on the same
// session — two turns writing the same message history.
func TestSharedOwnership_PromptIntoABusyChildSessionQueues(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	own := newSessionOwnership()

	sess, err := env.sessions.Create(t.Context(), "child")
	require.NoError(t, err)

	large := &gatedToolStreamModel{
		gate:       make(chan struct{}),
		entered:    make(chan struct{}),
		toolName:   "noop",
		toolInput:  `{"value":"hi"}`,
		toolCallID: "call",
	}
	subAgent := sharedOwnershipAgent(t, env, own, large)
	coder := sharedOwnershipAgent(t, env, own, &finishStreamModel{text: "coder"})

	done := startGatedTurn(t, subAgent, sess.ID)
	select {
	case <-large.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("sub-agent run never entered Stream")
	}

	res, err := coder.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		RunID:     "run-follow",
		Prompt:    "follow",
	})
	require.NoError(t, err)
	require.Nil(t, res, "a prompt into a busy session must be queued, not run")
	require.Equal(t, 1, coder.QueuedPrompts(sess.ID))

	close(large.gate)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("sub-agent run never finished")
	}
}
