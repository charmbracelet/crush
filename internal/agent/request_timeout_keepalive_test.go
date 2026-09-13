package agent

import (
	"context"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

// A stream that sends nothing but keepalives for longer than the idle budget
// must survive: that is a model thinking, not a dead connection. Before
// keepalives were forwarded, this is exactly the case that died at the
// timeout with "the model stopped sending data".
func TestRequestTimeoutSurvivesKeepaliveOnlyStretch(t *testing.T) {
	t.Parallel()

	const idle = 150 * time.Millisecond

	inner := &fakeStreamModel{parts: func(yield func(fantasy.StreamPart) bool) {
		// Four keepalives spaced just under the budget: together they span
		// well past it, so only a timer that counts them keeps the stream.
		for range 4 {
			time.Sleep(idle * 3 / 4)
			if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeKeepalive}) {
				return
			}
		}
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, Delta: "done"})
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish})
	}}

	model := newRequestTimeoutModel(inner, idle)
	stream, err := model.Stream(context.Background(), fantasy.Call{})
	require.NoError(t, err)

	var got []fantasy.StreamPartType
	var streamErr error
	for part := range stream {
		if part.Error != nil {
			streamErr = part.Error
		}
		got = append(got, part.Type)
	}

	require.NoError(t, streamErr, "keepalives should have kept the stream alive")
	require.Contains(t, got, fantasy.StreamPartTypeTextDelta)
}

// The watchdog still fires on a stream that is genuinely silent.
func TestRequestTimeoutStillFiresOnRealSilence(t *testing.T) {
	t.Parallel()

	const idle = 100 * time.Millisecond

	inner := &fakeStreamModel{parts: func(yield func(fantasy.StreamPart) bool) {
		time.Sleep(idle * 4)
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, Delta: "too late"})
	}}

	model := newRequestTimeoutModel(inner, idle)
	stream, err := model.Stream(context.Background(), fantasy.Call{})
	require.NoError(t, err)

	var streamErr error
	for part := range stream {
		if part.Error != nil {
			streamErr = part.Error
		}
	}
	require.Error(t, streamErr)
	require.Contains(t, streamErr.Error(), "stream received no data")
}

// A connection that keeps sending keepalives but never any content must
// still die. This is the laptop-sleep case: the stream survives, keepalives
// keep resetting the idle timer, and without a separate stall budget the
// turn hangs forever with a half-written tool call and nothing to cancel.
func TestRequestTimeoutFiresOnKeepaliveOnlyStream(t *testing.T) {
	t.Parallel()

	const idle = 20 * time.Millisecond

	inner := &fakeStreamModel{parts: func(yield func(fantasy.StreamPart) bool) {
		// Keepalives forever, spaced under the idle budget so the idle
		// timer alone would never fire.
		for {
			time.Sleep(idle / 2)
			if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeKeepalive}) {
				return
			}
		}
	}}

	model := newRequestTimeoutModel(inner, idle)
	stream, err := model.Stream(context.Background(), fantasy.Call{})
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		var streamErr error
		for part := range stream {
			if part.Error != nil {
				streamErr = part.Error
			}
		}
		done <- streamErr
	}()

	select {
	case streamErr := <-done:
		require.Error(t, streamErr, "a keepalive-only stream must not run forever")
		require.Contains(t, streamErr.Error(), "only keepalives")
	case <-time.After(10 * time.Second):
		t.Fatal("keepalive-only stream was never aborted")
	}
}

// Content resets the stall budget, so a stream that keeps producing survives
// well past it.
func TestRequestTimeoutStallBudgetResetsOnContent(t *testing.T) {
	t.Parallel()

	const idle = 30 * time.Millisecond

	inner := &fakeStreamModel{parts: func(yield func(fantasy.StreamPart) bool) {
		// Runs for longer than the stall budget, but real content keeps
		// arriving throughout.
		for range keepaliveGraceFactor * 2 {
			time.Sleep(idle / 2)
			if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, Delta: "."}) {
				return
			}
		}
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish})
	}}

	model := newRequestTimeoutModel(inner, idle)
	stream, err := model.Stream(context.Background(), fantasy.Call{})
	require.NoError(t, err)

	var streamErr error
	for part := range stream {
		if part.Error != nil {
			streamErr = part.Error
		}
	}
	require.NoError(t, streamErr, "an actively streaming response must never be cut off")
}

type fakeStreamModel struct {
	fantasy.LanguageModel
	parts fantasy.StreamResponse
}

func (m *fakeStreamModel) Stream(ctx context.Context, _ fantasy.Call) (fantasy.StreamResponse, error) {
	return func(yield func(fantasy.StreamPart) bool) {
		m.parts(func(p fantasy.StreamPart) bool {
			if ctx.Err() != nil {
				yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: ctx.Err()})
				return false
			}
			return yield(p)
		})
	}, nil
}
