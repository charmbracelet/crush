package agent

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

// concurrencyProbeModel records the maximum number of Stream iterators in
// flight at once. Each stream blocks on release, so a second dispatch that
// wrongly started a concurrent run for the same session is observable as an
// in-flight count above one.
type concurrencyProbeModel struct {
	inFlight atomic.Int32
	maxSeen  atomic.Int32
	entered  chan struct{}
	release  chan struct{}
}

func (m *concurrencyProbeModel) Provider() string { return "fake" }
func (m *concurrencyProbeModel) Model() string    { return "fake-model" }

func (m *concurrencyProbeModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return &fantasy.Response{
		Content:      fantasy.ResponseContent{fantasy.TextContent{Text: "done"}},
		FinishReason: fantasy.FinishReasonStop,
	}, nil
}

func (m *concurrencyProbeModel) Stream(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
	return func(yield func(fantasy.StreamPart) bool) {
		cur := m.inFlight.Add(1)
		for {
			mx := m.maxSeen.Load()
			if cur <= mx || m.maxSeen.CompareAndSwap(mx, cur) {
				break
			}
		}
		// Signal that a stream is in flight (non-blocking), then hold here
		// so a racing second dispatch would be caught by maxSeen.
		select {
		case m.entered <- struct{}{}:
		default:
		}
		<-m.release
		m.inFlight.Add(-1)

		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "1"})
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "1", Delta: "done"})
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "1"})
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
	}, nil
}

func (m *concurrencyProbeModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *concurrencyProbeModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

// fastModel is a non-blocking model used as the small model in concurrency
// tests. GenerateTitle runs on the small model; if it shares the probe model,
// its Stream call races into inFlight/maxSeen and produces spurious failures.
type fastModel struct{}

func (fastModel) Provider() string { return "fake" }
func (fastModel) Model() string    { return "fake-model" }

func (fastModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return &fantasy.Response{
		Content:      fantasy.ResponseContent{fantasy.TextContent{Text: "title"}},
		FinishReason: fantasy.FinishReasonStop,
	}, nil
}

func (fastModel) Stream(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
	return func(yield func(fantasy.StreamPart) bool) {
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "1"})
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "1", Delta: "title"})
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "1"})
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
	}, nil
}

func (fastModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (fastModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

// TestRun_ConcurrentInProcessDispatchStartsOneRun fires a burst of concurrent
// in-process Run calls (the path channel events use) at an idle session. Only
// one may become the active run; the rest must queue behind it. Before the
// dispatch decision was serialized under the per-session mutex, two callers
// could both pass the busy check and start two runs on the same session — this
// test catches that regression (maxSeen would exceed one).
//
// fastModel is used as the small model so GenerateTitle (which runs on the
// small model) does not race into the probe's inFlight/maxSeen counters. The
// queue count is not asserted because PrepareStep drains queued prompts into
// the active step before the model's Stream is called — by the time "entered"
// fires the queue is already empty by design.
func TestRun_ConcurrentInProcessDispatchStartsOneRun(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	model := &concurrencyProbeModel{
		entered: make(chan struct{}, 1),
		release: make(chan struct{}),
	}
	sa := testSessionAgent(env, model, fastModel{}, "system").(*sessionAgent)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	const n = 8
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = sa.Run(t.Context(), SessionAgentCall{
				SessionID: sess.ID,
				Prompt:    "event",
			})
		}()
	}

	// Wait until the active run's Stream is in flight (blocked on release).
	select {
	case <-model.entered:
	case <-time.After(5 * time.Second):
		close(model.release)
		wg.Wait()
		t.Fatal("no run became active")
	}

	// Every other dispatch must have either queued (and been folded into the
	// active step by PrepareStep) or never started its own Stream. Either way,
	// at most one Stream may be in flight and the high-water mark must be one.
	require.Equal(t, int32(1), model.inFlight.Load(), "exactly one run may be active")
	require.Equal(t, int32(1), model.maxSeen.Load(), "no two runs may stream concurrently for one session")

	close(model.release)
	wg.Wait()
}
