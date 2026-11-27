package agent

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

// titleProbeModel counts small-model requests (title generation).
type titleProbeModel struct {
	fastModel
	calls atomic.Int64
}

func (m *titleProbeModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	m.calls.Add(1)
	return m.fastModel.Stream(ctx, call)
}

func TestRun_TaskSessionDoesNotRequestTitle(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	small := &titleProbeModel{}
	sa := testSessionAgent(env, &finishStreamModel{text: "done"}, small, "system").(*sessionAgent)

	parent, err := env.sessions.Create(t.Context(), "Parent")
	require.NoError(t, err)
	child, err := env.sessions.CreateTaskSession(t.Context(), "child-1", parent.ID, "Assigned title")
	require.NoError(t, err)

	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: child.ID, Prompt: "do the task"})
	require.NoError(t, err)

	// Title generation is async; begin() registers it before Run returns.
	require.True(t, sa.titles.wait(5*time.Second))
	require.Zero(t, small.calls.Load())
	got, err := env.sessions.Get(t.Context(), child.ID)
	require.NoError(t, err)
	require.Equal(t, "Assigned title", got.Title)
}

func TestRun_RootSessionRequestsTitle(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	small := &titleProbeModel{}
	sa := testSessionAgent(env, &finishStreamModel{text: "done"}, small, "system").(*sessionAgent)

	root, err := env.sessions.Create(t.Context(), "Root")
	require.NoError(t, err)

	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: root.ID, Prompt: "do the task"})
	require.NoError(t, err)

	require.Eventually(t, func() bool { return small.calls.Load() != 0 }, 5*time.Second, time.Millisecond)
}
