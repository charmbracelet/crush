package agent

import (
	"context"
	"errors"
	"io"
	"time"

	"charm.land/fantasy"
)

// streamIdleTimeout is the maximum time to wait between stream parts before
// considering the connection stalled and triggering a retry.
const streamIdleTimeout = 120 * time.Second

// retryableStreamModel wraps a fantasy.LanguageModel and converts bare
// retryable network errors (such as io.ErrUnexpectedEOF) inside stream parts
// into *fantasy.ProviderError so the fantasy library's built-in retry
// mechanism can recognize and retry them.
type retryableStreamModel struct {
	fantasy.LanguageModel
}

// Stream implements fantasy.LanguageModel.
func (m retryableStreamModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	stream, err := m.LanguageModel.Stream(ctx, call)
	if err != nil {
		return nil, wrapRetryableNetworkErr(err)
	}

	return func(yield func(fantasy.StreamPart) bool) {
		sawToolUse := false
		// idleTimer tracks time between stream parts to detect stalled connections.
		idleTimer := time.NewTimer(streamIdleTimeout)
		defer idleTimer.Stop()

		// Create a channel to receive stream parts from the underlying stream.
		// This allows us to use select with a timeout.
		partCh := make(chan fantasy.StreamPart)
		doneCh := make(chan struct{})

		// Run the underlying stream in a goroutine.
		go func() {
			defer close(doneCh)
			stream(func(part fantasy.StreamPart) bool {
				// Check if context is cancelled.
				select {
				case <-ctx.Done():
					return false
				default:
				}

				// Send part to channel, respecting context cancellation.
				select {
				case partCh <- part:
					return true
				case <-ctx.Done():
					return false
				}
			})
		}()

		for {
			select {
			case part := <-partCh:
				// Reset idle timer on each part received.
				if !idleTimer.Stop() {
					select {
					case <-idleTimer.C:
					default:
					}
				}
				idleTimer.Reset(streamIdleTimeout)

				if isToolStreamPart(part.Type) {
					sawToolUse = true
				}
				if !sawToolUse && part.Type == fantasy.StreamPartTypeError && part.Error != nil {
					part.Error = wrapRetryableNetworkErr(part.Error)
				}
				if !yield(part) {
					return
				}

			case <-idleTimer.C:
				// Stream has been idle for too long - trigger a retryable error.
				yield(fantasy.StreamPart{
					Type: fantasy.StreamPartTypeError,
					Error: &fantasy.ProviderError{
						Title:   "network error",
						Message: "stream idle timeout: no data received for 120s",
						Cause:   errors.New("stream idle timeout"),
					},
				})
				return

			case <-ctx.Done():
				// Context cancelled - propagate cancellation error.
				yield(fantasy.StreamPart{
					Type:  fantasy.StreamPartTypeError,
					Error: ctx.Err(),
				})
				return

			case <-doneCh:
				// Stream completed normally.
				return
			}
		}
	}, nil
}

func isToolStreamPart(partType fantasy.StreamPartType) bool {
	switch partType {
	case fantasy.StreamPartTypeToolInputStart,
		fantasy.StreamPartTypeToolInputDelta,
		fantasy.StreamPartTypeToolInputEnd,
		fantasy.StreamPartTypeToolCall,
		fantasy.StreamPartTypeToolResult:
		return true
	default:
		return false
	}
}

// wrapRetryableNetworkErr wraps known retryable network errors into
// *fantasy.ProviderError so the fantasy retry mechanism can recognize them as
// retryable. If the error is not a known retryable network error, it is
// returned unchanged.
func wrapRetryableNetworkErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return &fantasy.ProviderError{
			Title:   "network error",
			Message: err.Error(),
			Cause:   err,
		}
	}
	return err
}
