package agent

import (
	"context"
	"errors"
	"sync"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/stretchr/testify/require"
)

// fakeCompactionHooks records the compaction events the agent fires and
// decides what each one answers.
type fakeCompactionHooks struct {
	mu       sync.Mutex
	fired    []string
	denyFor  map[string]bool
	failFor  map[string]bool
	lastCall hooks.CompactionDetail
}

func (f *fakeCompactionHooks) runnerFor(event string) CompactionHookRunner {
	return f
}

func (f *fakeCompactionHooks) RunCompaction(_ context.Context, event, _ string, detail hooks.CompactionDetail) (hooks.AggregateResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fired = append(f.fired, event)
	f.lastCall = detail
	switch {
	case f.failFor[event]:
		return hooks.AggregateResult{}, errors.New("hook exploded")
	case f.denyFor[event]:
		return hooks.AggregateResult{Decision: hooks.DecisionDeny, Reason: "not now"}, nil
	default:
		return hooks.AggregateResult{Decision: hooks.DecisionAllow}, nil
	}
}

func seedCompactionSession(t *testing.T, env fakeEnv) string {
	t.Helper()
	ctx := t.Context()
	sess, err := env.sessions.Create(ctx, "compact me")
	require.NoError(t, err)
	seedToolTurn(t, env, sess.ID, "old", 40_000)
	seedSizedTurns(t, env, sess.ID, 3, 5_000)
	return sess.ID
}

func TestPreCompactHookCanSkipCompaction(t *testing.T) {
	env := testEnv(t)
	model := &recordingModel{text: "the checkpoint"}
	sa := testSessionAgent(env, model, nil, "test prompt").(*sessionAgent)
	ctx := t.Context()
	sessionID := seedCompactionSession(t, env)

	before, err := env.messages.List(ctx, sessionID)
	require.NoError(t, err)

	hookRunner := &fakeCompactionHooks{denyFor: map[string]bool{hooks.EventPreCompact: true}}
	sa.compactionHooks = hookRunner.runnerFor

	require.NoError(t, sa.Summarize(ctx, sessionID, "keep the failing names", fantasy.ProviderOptions{}, nil))

	require.Equal(t, []string{hooks.EventPreCompact}, hookRunner.fired,
		"a denied compaction never reaches the model")
	require.Empty(t, model.calls)
	after, err := env.messages.List(ctx, sessionID)
	require.NoError(t, err)
	require.Equal(t, len(before), len(after), "no checkpoint is written for a skipped compaction")
	require.Equal(t, "keep the failing names", hookRunner.lastCall.Instructions)
	require.Equal(t, "manual", hookRunner.lastCall.Trigger)
	require.Positive(t, hookRunner.lastCall.Messages)
}

func TestPostCompactHookFiresAfterTheSessionIsWritten(t *testing.T) {
	env := testEnv(t)
	model := &recordingModel{text: "the checkpoint"}
	sa := testSessionAgent(env, model, nil, "test prompt").(*sessionAgent)
	ctx := t.Context()
	sessionID := seedCompactionSession(t, env)

	hookRunner := &fakeCompactionHooks{}
	sa.compactionHooks = hookRunner.runnerFor

	require.NoError(t, sa.Summarize(ctx, sessionID, "", fantasy.ProviderOptions{}, nil))

	require.Equal(t, []string{hooks.EventPreCompact, hooks.EventPostCompact}, hookRunner.fired)

	sess, err := env.sessions.Get(ctx, sessionID)
	require.NoError(t, err)
	require.NotEmpty(t, sess.SummaryMessageID, "the checkpoint is in place before hooks are told")
}

// TestCompactionProceedsWhenAHookFails keeps a broken shell command from
// taking the session down with it: only an explicit deny may stop compaction.
func TestCompactionProceedsWhenAHookFails(t *testing.T) {
	env := testEnv(t)
	model := &recordingModel{text: "the checkpoint"}
	sa := testSessionAgent(env, model, nil, "test prompt").(*sessionAgent)
	ctx := t.Context()
	sessionID := seedCompactionSession(t, env)

	hookRunner := &fakeCompactionHooks{failFor: map[string]bool{hooks.EventPreCompact: true}}
	sa.compactionHooks = hookRunner.runnerFor

	require.NoError(t, sa.Summarize(ctx, sessionID, "", fantasy.ProviderOptions{}, nil))
	require.Len(t, model.calls, 1)
}

// TestCompactionWithoutHooksIsUnchanged is the default path: no configured
// hooks, nothing fired, compaction proceeds.
func TestCompactionWithoutHooksIsUnchanged(t *testing.T) {
	env := testEnv(t)
	model := &recordingModel{text: "the checkpoint"}
	sa := testSessionAgent(env, model, nil, "test prompt").(*sessionAgent)
	ctx := t.Context()
	sessionID := seedCompactionSession(t, env)

	require.NoError(t, sa.Summarize(ctx, sessionID, "", fantasy.ProviderOptions{}, nil))
	require.Len(t, model.calls, 1)

	summary := false
	msgs, err := env.messages.List(ctx, sessionID)
	require.NoError(t, err)
	for _, msg := range msgs {
		if msg.IsSummaryMessage {
			summary = true
		}
	}
	require.True(t, summary)
}

func TestCompactionTriggerMarksAutoCompaction(t *testing.T) {
	t.Parallel()

	require.Equal(t, "manual", compactionTrigger(t.Context()))
	require.Equal(t, "auto", compactionTrigger(compactionTriggerContext(t.Context())))
}
