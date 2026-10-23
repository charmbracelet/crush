package agent

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

// titleProbeModel records title-model requests while reusing the deterministic
// fastModel response used by the dispatch tests.
type titleProbeModel struct {
	fastModel
	calls atomic.Int64
}

func (m *titleProbeModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	m.calls.Add(1)
	return m.fastModel.Stream(ctx, call)
}

func TestRun_SubagentDoesNotRequestAutomaticTitle(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	titleModel := &titleProbeModel{}
	sa := testSessionAgent(env, &finishStreamModel{text: "done"}, titleModel, "system").(*sessionAgent)

	child, err := env.sessions.CreateTaskSession(t.Context(), "child-session", "parent-session", "Assigned task title")
	require.NoError(t, err)

	_, err = sa.Run(t.Context(), SessionAgentCall{
		SessionID: child.ID,
		Prompt:    "Run the assigned task",
	})
	require.NoError(t, err)

	// Title generation runs asynchronously. Keep observing after Run returns.
	require.Never(t, func() bool {
		return titleModel.calls.Load() != 0
	}, 250*time.Millisecond, time.Millisecond)

	updated, err := env.sessions.Get(t.Context(), child.ID)
	require.NoError(t, err)
	require.Equal(t, "Assigned task title", updated.Title)
}

func TestRun_RootFirstPromptStillRequestsTitle(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	titleModel := &titleProbeModel{}
	sa := testSessionAgent(env, &finishStreamModel{text: "done"}, titleModel, "system").(*sessionAgent)

	root, err := env.sessions.Create(t.Context(), "Untitled Session")
	require.NoError(t, err)

	_, err = sa.Run(t.Context(), SessionAgentCall{
		SessionID: root.ID,
		Prompt:    "Start the root task",
	})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		updated, getErr := env.sessions.Get(t.Context(), root.ID)
		return getErr == nil && updated.Title == "title"
	}, 5*time.Second, time.Millisecond)
	require.Equal(t, int64(1), titleModel.calls.Load())
}
