package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/httpext"
)

// streamIdleTimeout is the maximum time to wait between stream parts before
// considering the connection stalled and triggering a retry.
var streamIdleTimeout = 45 * time.Second

// retryableStreamModel wraps a fantasy.LanguageModel and converts bare
// retryable network errors (such as io.ErrUnexpectedEOF) inside stream parts
// into *fantasy.ProviderError so the fantasy library's built-in retry
// mechanism can recognize and retry them.
type retryableStreamModel struct {
	fantasy.LanguageModel
}

// Stream implements fantasy.LanguageModel.
func (m retryableStreamModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	// Create a derived context with cancel to ensure the underlying stream
	// can be stopped when idle timeout fires or the outer function returns.
	localCtx, localCancel := context.WithCancel(ctx)

	// Attach a stream-activity channel so that HTTP response-body reads
	// (including SSE keep-alive pings silently consumed by the SDK) reset
	// the idle timer. Without this, ping events never produce StreamParts
	// and the timer fires even though the network connection is alive.
	localCtx, activityCh := httpext.WithStreamActivity(localCtx)

	stream, err := m.LanguageModel.Stream(localCtx, call)
	if err != nil {
		localCancel()
		return nil, wrapRetryableNetworkErr(err)
	}

	return func(yield func(fantasy.StreamPart) bool) {
		sawToolUse := false
		// idleTimer tracks time between stream parts to detect stalled connections.
		idleTimer := time.NewTimer(streamIdleTimeout)
		defer idleTimer.Stop()
		defer localCancel()

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
				case <-localCtx.Done():
					return false
				default:
				}

				// Send part to channel, respecting context cancellation.
				select {
				case partCh <- part:
					return true
				case <-localCtx.Done():
					return false
				}
			})
		}()

		for {
			select {
			case part := <-partCh:
				// Reset idle timer on each part received.
				resetTimer(idleTimer, streamIdleTimeout)

				if isToolStreamPart(part.Type) {
					sawToolUse = true
				}
				if !sawToolUse && part.Type == fantasy.StreamPartTypeError && part.Error != nil {
					part.Error = wrapRetryableNetworkErr(part.Error)
				}
				if !yield(part) {
					return
				}

			case <-activityCh:
				// HTTP response body received data (e.g. SSE ping events
				// that the SDK consumes without yielding StreamParts).
				// Reset the idle timer to prevent false timeouts.
				resetTimer(idleTimer, streamIdleTimeout)

			case <-idleTimer.C:
				// Stream has been idle for too long - trigger a retryable error.
				yield(fantasy.StreamPart{
					Type: fantasy.StreamPartTypeError,
					Error: &fantasy.ProviderError{
						Title:   "network error",
						Message: streamIdleTimeoutMessage(),
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

// resetTimer safely resets a timer, draining any pending fire.
func resetTimer(t *time.Timer, d time.Duration) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
	t.Reset(d)
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

func streamIdleTimeoutMessage() string {
	return fmt.Sprintf(
		"stream idle timeout: no data received for %ds",
		int(streamIdleTimeout/time.Second),
	)
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
