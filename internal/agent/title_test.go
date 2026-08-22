package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

// slowTitleModel is a small model whose Stream takes long enough to still be
// in flight when a fast large model has already finished the turn.
type slowTitleModel struct {
	delay time.Duration
	title string
	// entered, when non-nil, receives once the stream has started, so a
	// test can act at a point where title generation is provably in
	// flight.
	entered chan struct{}
}

func (slowTitleModel) Provider() string { return "fake" }
func (slowTitleModel) Model() string    { return "fake-small" }

func (slowTitleModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return nil, errors.New("not implemented")
}

func (m slowTitleModel) Stream(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
	return func(yield func(fantasy.StreamPart) bool) {
		if m.entered != nil {
			select {
			case m.entered <- struct{}{}:
			default:
			}
		}
		time.Sleep(m.delay)
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "1"})
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "1", Delta: m.title})
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "1"})
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
	}, nil
}

func (slowTitleModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (slowTitleModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

// TestRun_WaitsForTitleGeneration pins the lifetime of the title goroutine to
// the run that spawned it. Title generation is detached from the run context
// so a cancel can't drop the write; that also means nothing else can stop it,
// and when it was left unjoined it kept running after its owner tore down the
// database, writing into a closed connection ("sql: database is closed") and
// leaving the session with a placeholder title. Run must not return until the
// title has landed.
func TestRun_WaitsForTitleGeneration(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	small := slowTitleModel{delay: 250 * time.Millisecond, title: "a good title"}
	sa := testSessionAgent(env, fastModel{}, small, "system")

	sess, err := env.sessions.Create(t.Context(), "New Session")
	require.NoError(t, err)

	_, err = sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "hello",
	})
	require.NoError(t, err)

	got, err := env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Equal(t, "a good title", got.Title)
}

// TestCancelAll_WaitsForTitleGeneration covers the shutdown path. app.Shutdown
// calls CancelAll to make the agent quiescent and then closes the database, so
// CancelAll has to drain title generation too — the run cancel it issues never
// reaches those goroutines.
func TestCancelAll_WaitsForTitleGeneration(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	small := slowTitleModel{
		delay:   250 * time.Millisecond,
		title:   "shutdown title",
		entered: make(chan struct{}, 1),
	}
	sa := testSessionAgent(env, fastModel{}, small, "system").(*sessionAgent)

	sess, err := env.sessions.Create(t.Context(), "New Session")
	require.NoError(t, err)

	go func() {
		_, _ = sa.Run(t.Context(), SessionAgentCall{
			SessionID: sess.ID,
			Prompt:    "hello",
		})
	}()

	// Only cancel once title generation is provably in flight.
	select {
	case <-small.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("title generation never started")
	}

	sa.CancelAll()

	got, err := env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Equal(t, "shutdown title", got.Title)
}
