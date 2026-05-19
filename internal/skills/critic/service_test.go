package critic

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

func TestNewCriticService(t *testing.T) {
	t.Parallel()
	cfg := CriticSkillConfig{Enabled: true}
	cs := NewCriticService(cfg, (pubsub.Publisher[any])(nil))
	require.NotNil(t, cs)
	require.True(t, cs.Enabled())
}

func TestCriticService_Enabled_False(t *testing.T) {
	t.Parallel()
	cfg := CriticSkillConfig{Enabled: false}
	cs := NewCriticService(cfg, nil)
	require.False(t, cs.Enabled())
}

func TestCriticService_Review_NoEmitter(t *testing.T) {
	t.Parallel()
	cfg := CriticSkillConfig{Enabled: true}
	cs := NewCriticService(cfg, nil)
	_, err := cs.Review(context.Background(), "sid", Checkpoint{Type: CheckpointEdit})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no checkpoint emitter configured")
}

func TestCriticService_Review_WithEmitter(t *testing.T) {
	t.Parallel()
	cfg := CriticSkillConfig{Enabled: true}
	cs := NewCriticService(cfg, nil)

	cs.SetCheckpointEmitter(func(ctx context.Context, cp Checkpoint) (*CriticFeedback, error) {
		return &CriticFeedback{Verdict: "approve", Confidence: 0.9}, nil
	})

	fb, err := cs.Review(context.Background(), "sid", Checkpoint{Type: CheckpointEdit})
	require.NoError(t, err)
	require.Equal(t, "approve", fb.Verdict)
}

func TestCriticService_Review_CacheHit(t *testing.T) {
	t.Parallel()
	cfg := CriticSkillConfig{Enabled: true, CacheSize: 10}
	cs := NewCriticService(cfg, nil)

	callCount := 0
	cs.SetCheckpointEmitter(func(ctx context.Context, cp Checkpoint) (*CriticFeedback, error) {
		callCount++
		return &CriticFeedback{Verdict: "approve", Confidence: 0.9}, nil
	})

	cp := Checkpoint{Type: CheckpointEdit, PrimaryDiff: "same"}
	_, err := cs.Review(context.Background(), "sid", cp)
	require.NoError(t, err)
	require.Equal(t, 1, callCount)

	_, err = cs.Review(context.Background(), "sid", cp)
	require.NoError(t, err)
	require.Equal(t, 1, callCount) // Cached.
}

func TestCriticService_Publish(t *testing.T) {
	t.Parallel()
	broker := pubsub.NewBroker[any]()
	cfg := CriticSkillConfig{Enabled: true}
	cs := NewCriticService(cfg, broker)

	cs.SetCheckpointEmitter(func(ctx context.Context, cp Checkpoint) (*CriticFeedback, error) {
		return &CriticFeedback{Verdict: "revise", Confidence: 0.7, Concerns: []CriticConcern{{}}}, nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := broker.Subscribe(ctx)

	_, err := cs.Review(context.Background(), "sid", Checkpoint{Type: CheckpointEdit})
	require.NoError(t, err)

	event := <-ch
	require.Equal(t, pubsub.CriticVerdictRenderedEvent, event.Type)
	payload := event.Payload.(CriticVerdictEvent)
	require.Equal(t, "revise", payload.Verdict)
	require.Equal(t, 1, payload.ConcernCount)
}

func TestCriticService_ShouldAutoApprove(t *testing.T) {
	t.Parallel()

	// AutoApprove=true always approves.
	cs := NewCriticService(CriticSkillConfig{AutoApprove: true}, nil)
	require.True(t, cs.ShouldAutoApprove(&CriticFeedback{Confidence: 0.1}))

	// AutoApprove=false, high confidence approves.
	cs = NewCriticService(CriticSkillConfig{AutoApprove: false, Threshold: 0.8}, nil)
	require.True(t, cs.ShouldAutoApprove(&CriticFeedback{Confidence: 0.9}))

	// AutoApprove=false, low confidence without critical concern does not auto-approve.
	require.False(t, cs.ShouldAutoApprove(&CriticFeedback{Confidence: 0.5}))

	// AutoApprove=false, low confidence with critical concern does not approve.
	require.False(t, cs.ShouldAutoApprove(&CriticFeedback{
		Confidence: 0.5,
		Concerns:   []CriticConcern{{Severity: "critical"}},
	}))
}

func TestCriticService_PublishLoopCompleted(t *testing.T) {
	t.Parallel()
	broker := pubsub.NewBroker[any]()
	cs := NewCriticService(CriticSkillConfig{}, broker)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := broker.Subscribe(ctx)

	cs.PublishLoopCompleted("sid", 2, "approve")

	event := <-ch
	require.Equal(t, pubsub.CriticLoopCompletedEvent, event.Type)
	payload := event.Payload.(pubsub.CriticLoopEvent)
	require.Equal(t, "approve", payload.FinalVerdict)
	require.Equal(t, 2, payload.Iterations)
}
