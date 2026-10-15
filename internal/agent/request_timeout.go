package agent

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"charm.land/fantasy"
)

// requestTimeoutError reports that an LLM request exhausted its configured
// request_timeout budget. For streaming requests this is an idle timeout:
// it only fires when the provider sends nothing for the whole window, so a
// slow but actively streaming response is never killed. It wraps the
// underlying error so callers can still match [context.DeadlineExceeded]
// through the chain.
type requestTimeoutError struct {
	timeout time.Duration
	// fired is the window that actually elapsed when the deadline fired;
	// zero means the configured timeout. It differs from timeout when the
	// deadline fires before the first stream part, which gets a larger
	// budget to absorb prompt prefill.
	fired time.Duration
	idle  bool
	// first reports that the deadline fired before any data arrived, so
	// the user-facing message says the model never produced anything
	// rather than that it stalled mid-response.
	first bool
	cause error
}

// window returns the budget the deadline actually enforced.
func (e *requestTimeoutError) window() time.Duration {
	return cmp.Or(e.fired, e.timeout)
}

func (e *requestTimeoutError) Error() string {
	msg := fmt.Sprintf("LLM request timed out after %s", e.window())
	if e.idle {
		msg = fmt.Sprintf("LLM stream received no data for %s", e.window())
	}
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", msg, e.cause)
	}
	return msg
}

func (e *requestTimeoutError) Unwrap() error { return e.cause }

// userMessage explains the timeout in the UI, including how long the
// request ran before giving up and how to change the limit.
func (e *requestTimeoutError) userMessage() string {
	hint := "Increase the limit with \"option request-timeout SECONDS\" or set it to 0 to disable the timeout."
	switch {
	case e.idle && e.first:
		return fmt.Sprintf("The model did not send any data for %s. %s", e.window(), hint)
	case e.idle:
		return fmt.Sprintf("The model stopped sending data for %s. %s", e.window(), hint)
	default:
		return fmt.Sprintf("The model did not respond within %s. %s", e.window(), hint)
	}
}

// defaultFirstPartTimeout is the minimum budget granted to the wait for
// the first stream part. Prompt prefill on large contexts (e.g. after a
// long-running tool dumped a big result into the transcript) can
// legitimately keep the provider silent for minutes before the first
// token, so that wait gets the larger of the configured idle window and
// this floor instead of dying to a timeout meant for stalled responses.
const defaultFirstPartTimeout = 5 * time.Minute

// streamBufferSize bounds the parts buffered between the provider and the
// consumer. Large enough that a brief consumer stall (a synchronous
// database write, a blocked UI callback) never stops the provider from
// being read, small enough to bound memory.
const streamBufferSize = 64

// requestTimeoutModel wraps a [fantasy.LanguageModel] so requests are
// bounded by the configured request_timeout. Non-streaming calls get a
// hard per-request deadline, applied per call so fantasy's retry loop
// gives every attempt a fresh budget — the same per-request semantics the
// provider SDKs expose. Streams instead get an idle timeout: the budget
// resets whenever a part arrives and only fires when the provider goes
// silent, so a slow but actively streaming response is never aborted.
type requestTimeoutModel struct {
	fantasy.LanguageModel
	// timeout is the inactivity window between stream parts.
	timeout time.Duration
	// firstPart is the window granted until the first part arrives.
	firstPart time.Duration
}

// newRequestTimeoutModel bounds each request to m with the given timeout.
// A timeout of zero or less, or a nil model, returns m unchanged.
func newRequestTimeoutModel(m fantasy.LanguageModel, timeout time.Duration) fantasy.LanguageModel {
	if m == nil || timeout <= 0 {
		return m
	}
	return requestTimeoutModel{
		LanguageModel: m,
		timeout:       timeout,
		firstPart:     max(timeout, defaultFirstPartTimeout),
	}
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
	if errors.Is(err, context.Canceled) {
		timeoutErr.cause = context.DeadlineExceeded
	} else {
		timeoutErr.cause = err
	}
	return timeoutErr
}

// Generate implements [fantasy.LanguageModel]. The request gets a hard
// deadline: there is no incremental progress signal, so the whole call
// must finish within the budget.
func (m requestTimeoutModel) Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
	timeoutErr := &requestTimeoutError{timeout: m.timeout}
	ctx, cancel := context.WithTimeoutCause(ctx, m.timeout, timeoutErr)
	defer cancel()
	resp, err := m.LanguageModel.Generate(ctx, call)
	return resp, wrapTimedOut(ctx, timeoutErr, err)
}

// Stream implements [fantasy.LanguageModel].
//
// A goroutine pulls parts from the provider as they arrive, resets the
// idle clock on every arrival, and forwards the parts through a bounded
// channel to the consumer. The clock therefore measures provider
// activity, not consumer speed: a stalled consumer (a slow database write
// in a stream callback, heavy disk I/O from a concurrent tool) delays
// delivery without ever looking like a dead provider. The wait for the
// first part additionally gets [requestTimeoutModel.firstPart], which is
// usually larger than the inter-part window.
//
// Both the initial connection and the gaps between parts share the idle
// budget; the goroutine and the timer are released when iteration ends,
// whether the stream finishes, the consumer stops early, or the idle
// timeout aborts it.
func (m requestTimeoutModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	timeoutErr := &requestTimeoutError{timeout: m.timeout, idle: true}
	ctx, cancel := context.WithCancelCause(ctx)

	var arrived, errored atomic.Bool
	var buffered atomic.Int64

	// The timer lives until iteration ends. A fire with parts still
	// sitting in the buffer is backpressure, not provider silence, so the
	// clock re-arms instead of aborting a stream the consumer simply has
	// not caught up with.
	var timer *time.Timer
	timer = time.AfterFunc(m.firstPart, func() {
		if buffered.Load() > 0 {
			timer.Reset(m.timeout)
			return
		}
		if !arrived.Load() {
			timeoutErr.fired = m.firstPart
			timeoutErr.first = true
		}
		cancel(timeoutErr)
	})

	inner, err := m.LanguageModel.Stream(ctx, call)
	if err != nil {
		timer.Stop()
		cancel(nil)
		return nil, wrapTimedOut(ctx, timeoutErr, err)
	}

	parts := make(chan fantasy.StreamPart, streamBufferSize)
	stop := make(chan struct{})

	go func() {
		defer close(parts)
		inner(func(part fantasy.StreamPart) bool {
			// Mark the arrival before resetting the clock so a timer
			// fire racing this part sees the buffer as non-empty and
			// re-arms instead of killing a live stream.
			buffered.Add(1)
			arrived.Store(true)
			timer.Reset(m.timeout)
			part.Error = wrapTimedOut(ctx, timeoutErr, part.Error)
			if part.Error != nil {
				errored.Store(true)
			}
			select {
			case parts <- part:
				return true
			case <-stop:
				buffered.Add(-1)
				return false
			}
		})
		// A deadline-fired stream is expected to fail loudly: SDKs
		// surface the abort as a stream error part. Some just stop
		// delivering; synthesize the error then so the consumer learns
		// why the stream died instead of seeing a truncated success.
		if context.Cause(ctx) == timeoutErr && !errored.Load() {
			select {
			case parts <- fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: timeoutErr}:
			case <-stop:
			}
		}
	}()

	return func(yield func(fantasy.StreamPart) bool) {
		// Registration order is the reverse of execution: close(stop)
		// unblocks a producer stuck on a full buffer first.
		defer cancel(nil)
		defer timer.Stop()
		defer close(stop)
		for part := range parts {
			buffered.Add(-1)
			if !yield(part) {
				return
			}
		}
	}, nil
}
