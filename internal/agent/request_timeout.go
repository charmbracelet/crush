package agent

import (
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
	// timeout is the window that actually ran out, which is wider than the
	// configured value when the deadline fired before the first part.
	timeout time.Duration
	idle    bool
	// first reports that nothing at all arrived before the deadline, so
	// the model never started answering rather than stopping midway.
	first bool
	cause error
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

// userMessage explains the timeout in the UI, including how long the request
// ran before giving up and how to change the limit.
func (e *requestTimeoutError) userMessage() string {
	hint := "Increase the limit with \"option request-timeout SECONDS\" or set it to 0 to disable the timeout."
	if e.idle && e.first {
		return fmt.Sprintf("The model did not send any data within %s. %s", e.timeout, hint)
	}
	if e.idle {
		return fmt.Sprintf("The model stopped sending data for %s. %s", e.timeout, hint)
	}
	return fmt.Sprintf("The model did not respond within %s. %s", e.timeout, hint)
}

// defaultFirstPartTimeout is the floor for the wait on a stream's first
// part. Prefill and the thinking that happens before the first token are
// model work, not a stalled connection: after a tool result lands in the
// transcript a provider can stay silent for minutes before answering, and
// the inter-part window is far too small to absorb that. A stream that
// stays silent longer than this is treated as dead.
const defaultFirstPartTimeout = 5 * time.Minute

// requestTimeoutModel wraps a [fantasy.LanguageModel] so requests are
// bounded by the configured request_timeout. Non-streaming calls get a hard
// per-request deadline, applied per call so fantasy's retry loop gives every
// attempt a fresh budget, the same per-request semantics the provider SDKs
// expose. Streams get an idle timeout instead: the clock only runs while we
// wait on the provider, so neither a slow but active response nor work on
// this side (a long-running tool, a heavy database write) is ever mistaken
// for a dead provider.
type requestTimeoutModel struct {
	fantasy.LanguageModel
	// timeout is the inactivity window between stream parts.
	timeout time.Duration
	// firstPart is the window granted until the first part arrives.
	firstPart time.Duration
}

// newRequestTimeoutModel bounds each request to m with the given timeout. A
// timeout of zero or less, or a nil model, returns m unchanged.
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
	if errors.Is(err, timeoutErr) || errors.Is(err, context.Canceled) {
		timeoutErr.cause = context.DeadlineExceeded
	} else {
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
// The clock measures provider silence and nothing else. It runs from the
// call until the first part arrives, under the wider first-part budget,
// and then from the moment the consumer is done with a part until the next
// one arrives. Time spent on this side, in a stream callback or in a tool
// waiting on a long-running job, is never charged to the model, so the
// timer only starts once that work has stopped. It is released when
// iteration ends, whether the stream finishes, breaks, or the idle timeout
// aborts it.
func (m requestTimeoutModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	timeoutErr := &requestTimeoutError{timeout: m.timeout, idle: true}
	ctx, cancel := context.WithCancelCause(ctx)

	var arrived atomic.Bool
	timer := time.AfterFunc(m.firstPart, func() {
		if !arrived.Load() {
			// Nothing ever arrived, so the first-part budget is the
			// window that ran out.
			timeoutErr.timeout, timeoutErr.first = m.firstPart, true
		}
		cancel(timeoutErr)
	})

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
			arrived.Store(true)
			// The provider is alive. Hold the clock while the consumer
			// works on this part and re-arm only once we are waiting on
			// the next one.
			timer.Stop()
			part.Error = wrapTimedOut(ctx, timeoutErr, part.Error)
			keepGoing := yield(part)
			if keepGoing {
				timer.Reset(m.timeout)
			}
			return keepGoing
		})
	}, nil
}
