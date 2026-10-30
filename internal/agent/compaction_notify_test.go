package agent

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

// recordNotifier collects agent notifications so tests can assert on what
// observers were told.
type recordNotifier struct {
	mu   sync.Mutex
	sent []notify.Notification
}

func (r *recordNotifier) Publish(_ pubsub.EventType, n notify.Notification) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent = append(r.sent, n)
}

func (r *recordNotifier) PublishMustDeliver(ctx context.Context, t pubsub.EventType, n notify.Notification) {
	r.Publish(t, n)
}

func (r *recordNotifier) byType(want notify.Type) []notify.Notification {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []notify.Notification
	for _, n := range r.sent {
		if n.Type == want {
			out = append(out, n)
		}
	}
	return out
}

// TestSummarizingTypeMatchesWire guards the server mapping, which turns a
// notification into proto.AgentEventTypeSummarize by string identity.
func TestSummarizingTypeMatchesWire(t *testing.T) {
	t.Parallel()
	require.Equal(t, "summarize", string(notify.TypeSummarizing))
}

func TestSummarizeAnnouncesStartAndOutcome(t *testing.T) {
	env := testEnv(t)
	model := &recordingModel{text: "the checkpoint"}
	notifier := &recordNotifier{}
	sa := testSessionAgentWithNotifier(env, model, nil, "test prompt", notifier).(*sessionAgent)
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "compact me")
	require.NoError(t, err)
	seedSizedTurns(t, env, sess.ID, 8, 5_000)

	require.NoError(t, sa.Summarize(ctx, sess.ID, "", fantasy.ProviderOptions{}, nil))

	events := notifier.byType(notify.TypeSummarizing)
	require.False(t, events[0].Done)
	require.Equal(t, "Compacting the session", events[0].Progress)

	// A streaming compaction reports how far along it is, throttled rather
	// than once per delta, and none of that may read as the end.
	var sawProgress bool
	for _, e := range events[:len(events)-1] {
		require.False(t, e.Done)
		if strings.Contains(e.Progress, "checkpoint ") {
			sawProgress = true
		}
	}
	require.True(t, sawProgress, "a streaming compaction must say how far along it is")

	last := events[len(events)-1]
	require.True(t, last.Done)
	require.Equal(t, sess.ID, last.SessionID)
	require.Equal(t, "compact me", last.SessionTitle)
	require.Contains(t, last.Progress, "Compacted ")
	require.Contains(t, last.Progress, " verbatim")
}

func TestSummarizeAnnouncesFailure(t *testing.T) {
	env := testEnv(t)
	model := &recordingModel{fail: errors.New("provider exploded")}
	notifier := &recordNotifier{}
	sa := testSessionAgentWithNotifier(env, model, nil, "test prompt", notifier).(*sessionAgent)
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)
	seedSizedTurns(t, env, sess.ID, 4, 5_000)

	require.Error(t, sa.Summarize(ctx, sess.ID, "", fantasy.ProviderOptions{}, nil))

	// A failed compaction must still close the pair, or observers keep
	// showing the session as summarizing forever.
	events := notifier.byType(notify.TypeSummarizing)
	require.Len(t, events, 2)
	require.True(t, events[1].Done)
	require.Equal(t, "Compaction failed", events[1].Progress)
}

func TestSummarizeAnnouncesNothingWhenThereIsNothingToDo(t *testing.T) {
	env := testEnv(t)
	model := &recordingModel{text: "unused"}
	notifier := &recordNotifier{}
	sa := testSessionAgentWithNotifier(env, model, nil, "test prompt", notifier).(*sessionAgent)
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)

	require.NoError(t, sa.Summarize(ctx, sess.ID, "", fantasy.ProviderOptions{}, nil))
	require.Empty(t, notifier.byType(notify.TypeSummarizing))
}
