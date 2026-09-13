package agent

import (
	"context"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

// A cancelled run must stop consuming a stream whose parts are already
// buffered locally, where cancelling the context reaches nothing.
func TestStreamStopsDrainingBufferedPartsOnCancel(t *testing.T) {
	t.Parallel()

	const buffered = 5000
	inner := &fakeLanguageModel{
		stream: func(yield func(fantasy.StreamPart) bool) {
			for range buffered {
				if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, Delta: "x"}) {
					return
				}
			}
		},
	}

	// Through the same wiring as the coordinator, with the timeout disabled.
	model := newStopOnCancelModel(newRequestTimeoutModel(inner, 0))
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	stream, err := model.Stream(ctx, fantasy.Call{})
	require.NoError(t, err)

	delivered := 0
	var lastErr error
	for part := range stream {
		delivered++
		if part.Error != nil {
			lastErr = part.Error
		}
		if delivered == 10 {
			cancel()
		}
	}

	require.Equal(t, 11, delivered, "iteration must stop once the run is cancelled")
	// A quiet stop reads as a finished stream and leaves the tool call open.
	require.ErrorIs(t, lastErr, context.Canceled, "a cancelled stream must report why it stopped")
}
