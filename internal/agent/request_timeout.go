package agent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"charm.land/fantasy"
)

// timeoutKind is which request budget ran out.
type timeoutKind int

const (
	// timeoutDeadline is a non-streaming request that ran past its budget.
	timeoutDeadline timeoutKind = iota
	// timeoutIdle is a stream that sent nothing at all for its budget.
	timeoutIdle
	// timeoutStalled is a stream that sent only keepalives for its budget.
	timeoutStalled
)

// timeoutMessages holds the log and UI wording for each kind; both take the
// elapsed budget.
var timeoutMessages = [...]struct{ log, user string }{
	timeoutDeadline: {"LLM request timed out after %s", "The model did not respond within %s."},
	timeoutIdle:     {"LLM stream received no data for %s", "The model stopped sending data for %s."},
	timeoutStalled:  {"LLM stream sent only keepalives for %s", "The connection stayed open but the model sent nothing for %s."},
}

// requestTimeoutError reports that an LLM request exhausted a
// request_timeout budget. It presents as a retryable [net.Error] timeout and
// never wraps a context error, which retry logic would read as an abort.
type requestTimeoutError struct {
	timeout time.Duration
	kind    timeoutKind
	cause   error
}

func (e *requestTimeoutError) Error() string {
	msg := fmt.Sprintf(timeoutMessages[e.kind].log, e.timeout)
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", msg, e.cause)
	}
	return msg
}

func (e *requestTimeoutError) Unwrap() error { return e.cause }

// Timeout implements [net.Error], which marks this as retryable.
func (e *requestTimeoutError) Timeout() bool { return true }

// Temporary implements [net.Error].
func (e *requestTimeoutError) Temporary() bool { return true }

// userMessage explains the timeout in the UI and how to change the limit.
func (e *requestTimeoutError) userMessage() string {
	const hint = "Increase the limit with \"option request-timeout SECONDS\" or set it to 0 to disable the timeout."
	return fmt.Sprintf(timeoutMessages[e.kind].user, e.timeout) + " " + hint
}

// keepaliveGraceFactor multiplies the request timeout to get how long a
// stream may send nothing but keepalives before it is treated as wedged.
const keepaliveGraceFactor = 10

// requestTimeoutModel wraps a [fantasy.LanguageModel] so requests are
// bounded by the configured request_timeout. Non-streaming calls get a hard
// per-request deadline, applied per call so fantasy's retry loop gives every
// attempt a fresh budget. Streams are watched by a [streamWatchdog] instead,
// so a slow but actively streaming response is never aborted.
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

// wrapTimedOut replaces err with the requestTimeoutError that cancelled ctx,
// if one did. Other errors (user cancellation, provider failures) pass
// through unchanged. A context error is dropped as the cause so the result
// never reads as a deliberate abort.
func wrapTimedOut(ctx context.Context, err error) error {
	var timeoutErr *requestTimeoutError
	if err == nil || !errors.As(context.Cause(ctx), &timeoutErr) {
		return err
	}
	wrapped := *timeoutErr
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		wrapped.cause = err
	}
	return &wrapped
}

// Generate implements [fantasy.LanguageModel]. There is no incremental
// progress signal, so the whole call must finish within the budget.
func (m requestTimeoutModel) Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
	ctx, cancel := context.WithTimeoutCause(ctx, m.timeout, &requestTimeoutError{timeout: m.timeout})
	defer cancel()
	resp, err := m.LanguageModel.Generate(ctx, call)
	return resp, wrapTimedOut(ctx, err)
}

// Stream implements [fantasy.LanguageModel]. The stream is consumed after
// Stream returns, so the watchdog outlives this call and is stopped when
// iteration ends.
func (m requestTimeoutModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	ctx, watchdog := newStreamWatchdog(ctx, m.timeout)
	inner, err := m.LanguageModel.Stream(ctx, call)
	if err != nil {
		watchdog.stop()
		return nil, wrapTimedOut(ctx, err)
	}
	return func(yield func(fantasy.StreamPart) bool) {
		defer watchdog.stop()
		inner(func(part fantasy.StreamPart) bool {
			watchdog.observe(part)
			part.Error = wrapTimedOut(ctx, part.Error)
			return yield(part)
		})
	}, nil
}

// streamWatchdog cancels a stream that sends nothing for its idle budget, or
// only keepalives for its much longer stall budget. Keepalives prove the
// connection is up but not that anything is being generated, so on their own
// they must not hold a turn open forever.
type streamWatchdog struct {
	idle, stall           time.Duration
	idleTimer, stallTimer *time.Timer
	cancel                context.CancelCauseFunc
}

// newStreamWatchdog starts both budgets and returns the context they cancel.
func newStreamWatchdog(ctx context.Context, idle time.Duration) (context.Context, *streamWatchdog) {
	ctx, cancel := context.WithCancelCause(ctx)
	w := &streamWatchdog{idle: idle, stall: idle * keepaliveGraceFactor, cancel: cancel}
	w.idleTimer = time.AfterFunc(w.idle, func() {
		cancel(&requestTimeoutError{timeout: w.idle, kind: timeoutIdle})
	})
	w.stallTimer = time.AfterFunc(w.stall, func() {
		cancel(&requestTimeoutError{timeout: w.stall, kind: timeoutStalled})
	})
	return ctx, w
}

// observe resets the idle budget on any part and the stall budget on content.
func (w *streamWatchdog) observe(part fantasy.StreamPart) {
	w.idleTimer.Reset(w.idle)
	if part.Type != fantasy.StreamPartTypeKeepalive {
		w.stallTimer.Reset(w.stall)
	}
}

// stop releases both timers and the context.
func (w *streamWatchdog) stop() {
	w.idleTimer.Stop()
	w.stallTimer.Stop()
	w.cancel(nil)
}
