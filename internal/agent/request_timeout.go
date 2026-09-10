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
// slow but actively streaming response is never killed. It wraps the
// underlying error so callers can still match [context.DeadlineExceeded]
// through the chain.
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
	if errors.Is(err, context.Canceled) {
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
// The stream is consumed after Stream returns, so the timer must outlive
// this call: it fires only after timeout seconds without any part arriving,
// and is released when iteration ends, whether the stream finishes, breaks,
// or the idle timeout aborts it. Both the initial connection and gaps
// between parts share the same budget.
func (m requestTimeoutModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	timeoutErr := &requestTimeoutError{timeout: m.timeout, idle: true}
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
			timer.Reset(m.timeout)
			part.Error = wrapTimedOut(ctx, timeoutErr, part.Error)
			return yield(part)
		})
	}, nil
}
