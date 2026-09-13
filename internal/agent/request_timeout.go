package agent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"charm.land/fantasy"
)

// requestTimeoutError reports that an LLM request exhausted its configured
// request_timeout budget. For streaming requests this is an idle timeout:
// it only fires when the provider sends nothing for the whole window, so a
// slow but actively streaming response is never killed.
//
// It reports itself as a [net.Error] timeout. Silence for the whole window
// usually means the connection died rather than the model thinking, since
// provider keepalives reset the budget, and losing the network or sleeping
// the laptop is worth retrying. It deliberately does not wrap a context
// error: retry logic treats those as a deliberate abort and gives up.
type requestTimeoutError struct {
	timeout time.Duration
	idle    bool
	cause   error
}

func (e *requestTimeoutError) Error() string {
	msg := fmt.Sprintf("LLM request timed out after %s", e.timeout)
	if e.idle {
		msg = fmt.Sprintf("LLM stream received no data for %s", e.timeout)
	}
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", msg, e.cause)
	}
	return msg
}

func (e *requestTimeoutError) Unwrap() error { return e.cause }

// Timeout implements [net.Error], which is what marks this as retryable
// rather than an abort.
func (e *requestTimeoutError) Timeout() bool { return true }

// Temporary implements [net.Error].
func (e *requestTimeoutError) Temporary() bool { return true }

// userMessage explains the timeout in the UI, including how long the request
// ran before giving up and how to change the limit.
func (e *requestTimeoutError) userMessage() string {
	hint := "Increase the limit with \"option request-timeout SECONDS\" or set it to 0 to disable the timeout."
	if e.idle {
		return fmt.Sprintf("The model stopped sending data for %s. %s", e.timeout, hint)
	}
	return fmt.Sprintf("The model did not respond within %s. %s", e.timeout, hint)
}

// requestTimeoutModel wraps a [fantasy.LanguageModel] so requests are
// bounded by the configured request_timeout. Non-streaming calls get a hard
// per-request deadline, applied per call so fantasy's retry loop gives every
// attempt a fresh budget — the same per-request semantics the provider SDKs
// expose. Streams instead get an idle timeout: the budget resets whenever a
// part arrives and only fires when the provider goes silent, so a slow but
// actively streaming response is never aborted.
type requestTimeoutModel struct {
	fantasy.LanguageModel
	timeout time.Duration
}

// newRequestTimeoutModel bounds each request to m with the given timeout. A
// timeout of zero or less, or a nil model, returns m unchanged.
func newRequestTimeoutModel(m fantasy.LanguageModel, timeout time.Duration) fantasy.LanguageModel {
	if m == nil || timeout <= 0 {
		return m
	}
	return requestTimeoutModel{LanguageModel: m, timeout: timeout}
}

// wrapTimedOut replaces err with the requestTimeoutError when this model's
// own deadline fired. Other errors — user cancellation, outer deadlines,
// provider failures — pass through unchanged. Cancellation errors caused by
// our own timer report as [context.DeadlineExceeded] so callers never
// mistake a timeout for a user cancellation.
func wrapTimedOut(ctx context.Context, timeoutErr *requestTimeoutError, err error) error {
	if err == nil || context.Cause(ctx) != timeoutErr {
		return err
	}
	// The cause is dropped when it is a context error. Keeping it would
	// make this match [context.Canceled] or [context.DeadlineExceeded],
	// and both read as "the caller gave up" to retry logic.
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		timeoutErr.cause = err
	}
	return timeoutErr
}

// Generate implements [fantasy.LanguageModel]. The request gets a hard
// deadline: there is no incremental progress signal, so the whole call must
// finish within the budget.
func (m requestTimeoutModel) Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
	timeoutErr := &requestTimeoutError{timeout: m.timeout}
	ctx, cancel := context.WithTimeoutCause(ctx, m.timeout, timeoutErr)
	defer cancel()
	resp, err := m.LanguageModel.Generate(ctx, call)
	return resp, wrapTimedOut(ctx, timeoutErr, err)
}

// Stream implements [fantasy.LanguageModel].
//
// The stream is consumed after Stream returns, so the timer must outlive
// this call: it fires only after timeout seconds without any part arriving,
// and is released when iteration ends, whether the stream finishes, breaks,
// or the idle timeout aborts it. Both the initial connection and gaps
// between parts share the same budget.
func (m requestTimeoutModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	timeoutErr := &requestTimeoutError{timeout: m.timeout, idle: true}
	// run is the caller's context. Cancelling the turn cancels it, while
	// the idle timeout below only cancels the derived one.
	run := ctx
	ctx, cancel := context.WithCancelCause(ctx)
	timer := time.AfterFunc(m.timeout, func() { cancel(timeoutErr) })

	inner, err := m.LanguageModel.Stream(ctx, call)
	if err != nil {
		timer.Stop()
		cancel(nil)
		return nil, wrapTimedOut(ctx, timeoutErr, err)
	}
	return func(yield func(fantasy.StreamPart) bool) {
		defer timer.Stop()
		defer cancel(nil)
		inner(func(part fantasy.StreamPart) bool {
			// Stop draining once the turn is cancelled. A provider that
			// already has the rest of the response buffered keeps yielding
			// from memory, where cancelling the context reaches nothing, so
			// without this the cancel is accepted and then ignored.
			//
			// Report it rather than just stopping: ending the iteration
			// quietly looks like a stream that finished, and the caller
			// only closes out a half-written tool call when the turn ends
			// in an error.
			if err := context.Cause(run); err != nil {
				yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: err})
				return false
			}
			timer.Reset(m.timeout)
			part.Error = wrapTimedOut(ctx, timeoutErr, part.Error)
			return yield(part)
		})
	}, nil
}
