package agent

import (
	"context"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

// A cancelled run must stop consuming the stream. Anthropic flushes a large
// tool call's arguments in one burst, so those parts are already buffered
// locally: cancelling the context cannot un-receive them, and if nothing
// checks the context while draining, iteration runs to completion and the
// cancel appears to do nothing.
func TestStreamStopsDrainingBufferedPartsOnCancel(t *testing.T) {
	t.Parallel()

	const buffered = 5000
	delivered := 0

	inner := &fakeLanguageModel{
		stream: func(yield func(fantasy.StreamPart) bool) {
			for range buffered {
				if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, Delta: "x"}) {
					return
				}
			}
		},
	}

	model := requestTimeoutModel{LanguageModel: inner, timeout: time.Minute}
	ctx, cancel := context.WithCancel(t.Context())

	stream, err := model.Stream(ctx, fantasy.Call{})
	require.NoError(t, err)

	var lastErr error
	stream(func(p fantasy.StreamPart) bool {
		delivered++
		if p.Error != nil {
			lastErr = p.Error
		}
		if delivered == 10 {
			cancel()
		}
		return true
	})

	require.Less(t, delivered, buffered,
		"iteration must stop once the run is cancelled, not drain the whole buffer")

	// Stopping quietly reads as a stream that finished. The agent only
	// closes out a half-written tool call when the turn ends in an error,
	// so a silent stop leaves the tool spinning forever.
	require.ErrorIs(t, lastErr, context.Canceled,
		"a cancelled stream must report why it stopped")
	t.Logf("delivered %d of %d buffered parts after cancel", delivered, buffered)
}
