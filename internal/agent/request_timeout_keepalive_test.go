package agent

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

// drainStream consumes a stream and returns the part types and last error.
func drainStream(t *testing.T, model fantasy.LanguageModel) ([]fantasy.StreamPartType, error) {
	t.Helper()
	stream, err := model.Stream(t.Context(), fantasy.Call{})
	require.NoError(t, err)
	var types []fantasy.StreamPartType
	var lastErr error
	for part := range stream {
		if part.Error != nil {
			lastErr = part.Error
		}
		types = append(types, part.Type)
	}
	return types, lastErr
}

// A keepalive-only stretch longer than the idle budget is a model thinking,
// not a dead connection.
func TestRequestTimeoutSurvivesKeepaliveOnlyStretch(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		const idle = time.Minute
		inner := &fakeStreamModel{parts: func(yield func(fantasy.StreamPart) bool) {
			for range 4 {
				time.Sleep(idle * 3 / 4)
				if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeKeepalive}) {
					return
				}
			}
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, Delta: "done"})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish})
		}}

		types, err := drainStream(t, newRequestTimeoutModel(inner, idle))
		require.NoError(t, err, "keepalives should have kept the stream alive")
		require.Contains(t, types, fantasy.StreamPartTypeTextDelta)
	})
}

// The idle budget still fires on a stream that is genuinely silent.
func TestRequestTimeoutStillFiresOnRealSilence(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		const idle = time.Minute
		inner := &fakeStreamModel{parts: func(yield func(fantasy.StreamPart) bool) {
			time.Sleep(idle * 4)
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, Delta: "too late"})
		}}

		_, err := drainStream(t, newRequestTimeoutModel(inner, idle))
		require.ErrorContains(t, err, "stream received no data")
	})
}

// Keepalives alone must not hold a turn open forever: the laptop-sleep case,
// where the connection survives but nothing is being generated behind it.
func TestRequestTimeoutFiresOnKeepaliveOnlyStream(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		const idle = time.Minute
		inner := &fakeStreamModel{parts: func(yield func(fantasy.StreamPart) bool) {
			for {
				time.Sleep(idle / 2)
				if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeKeepalive}) {
					return
				}
			}
		}}

		start := time.Now()
		_, err := drainStream(t, newRequestTimeoutModel(inner, idle))
		var timeoutErr *requestTimeoutError
		require.ErrorAs(t, err, &timeoutErr)
		require.Equal(t, timeoutStalled, timeoutErr.kind)
		require.Less(t, time.Since(start), idle*keepaliveGraceFactor+idle, "the stall budget, not the idle one, must end it")
	})
}

// Content resets the stall budget, so an active stream outlives it.
func TestRequestTimeoutStallBudgetResetsOnContent(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		const idle = time.Minute
		inner := &fakeStreamModel{parts: func(yield func(fantasy.StreamPart) bool) {
			for range keepaliveGraceFactor * 2 {
				time.Sleep(idle / 2)
				if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, Delta: "."}) {
					return
				}
			}
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish})
		}}

		_, err := drainStream(t, newRequestTimeoutModel(inner, idle))
		require.NoError(t, err, "an actively streaming response must never be cut off")
	})
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
