package agent

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"charm.land/fantasy"
)

// timeoutKind is which request budget ran out.
type timeoutKind int

const (
	// timeoutDeadline is a non-streaming request that ran past its budget.
	timeoutDeadline timeoutKind = iota
	// timeoutFirstPart is a stream that sent nothing before its first part.
	timeoutFirstPart
	// timeoutIdle is a stream that went silent after it started answering.
	timeoutIdle
	// timeoutStalled is a stream that sent only keepalives for its budget.
	timeoutStalled
)

// timeoutMessages holds the log and UI wording for each kind; both take the
// elapsed budget.
var timeoutMessages = [...]struct{ log, user string }{
	timeoutDeadline:  {"LLM request timed out after %s", "The model did not respond within %s."},
	timeoutFirstPart: {"LLM stream received no data for %s", "The model did not send any data within %s."},
	timeoutIdle:      {"LLM stream received no data for %s", "The model stopped sending data for %s."},
	timeoutStalled:   {"LLM stream sent only keepalives for %s", "The connection stayed open but the model sent nothing for %s."},
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

// defaultFirstPartTimeout is the floor for the wait on a stream's first
// part. Prefill and the thinking that happens before the first token are
// model work, not a stalled connection: after a tool result lands in the
// transcript a provider can stay silent for minutes before answering.
const defaultFirstPartTimeout = 5 * time.Minute

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

// wrapTimedOut replaces err with the requestTimeoutError that cancelled ctx,
// if one did. Other errors (user cancellation, provider failures) pass
// through unchanged. The result is a copy of the sentinel, so a provider that
// hands the sentinel back cannot make the chain cyclic, and a context error
// is dropped as the cause so the result never reads as a deliberate abort.
func wrapTimedOut(ctx context.Context, err error) error {
	var timeoutErr *requestTimeoutError
	if err == nil || !errors.As(context.Cause(ctx), &timeoutErr) {
		return err
	}
	wrapped := *timeoutErr
	if !errors.Is(err, timeoutErr) && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
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
	ctx, watchdog := newStreamWatchdog(ctx, m.timeout, m.firstPart)
	inner, err := m.LanguageModel.Stream(ctx, call)
	if err != nil {
		watchdog.stop()
		return nil, wrapTimedOut(ctx, err)
	}
	return func(yield func(fantasy.StreamPart) bool) {
		defer watchdog.stop()
		inner(func(part fantasy.StreamPart) bool {
			watchdog.received(part)
			part.Error = wrapTimedOut(ctx, part.Error)
			keepGoing := yield(part)
			if keepGoing {
				watchdog.waiting(part)
			}
			return keepGoing
		})
	}, nil
}

// streamWatchdog cancels a stream whose provider goes silent: nothing for the
// first-part budget, then nothing for the idle budget between parts, or only
// keepalives for the much longer stall budget. Keepalives prove the
// connection is up but not that anything is being generated, so on their own
// they must not hold a turn open forever. The clocks only run while we wait
// on the provider, so work on this side is never charged to the model.
type streamWatchdog struct {
	idle, stall           time.Duration
	idleTimer, stallTimer *time.Timer
	arrived               atomic.Bool
	cancel                context.CancelCauseFunc
}

// newStreamWatchdog starts both budgets and returns the context they cancel.
func newStreamWatchdog(ctx context.Context, idle, firstPart time.Duration) (context.Context, *streamWatchdog) {
	ctx, cancel := context.WithCancelCause(ctx)
	w := &streamWatchdog{idle: idle, stall: max(idle*keepaliveGraceFactor, firstPart), cancel: cancel}
	w.idleTimer = time.AfterFunc(firstPart, func() {
		if w.arrived.Load() {
			cancel(&requestTimeoutError{timeout: w.idle, kind: timeoutIdle})
			return
		}
		cancel(&requestTimeoutError{timeout: firstPart, kind: timeoutFirstPart})
	})
	w.stallTimer = time.AfterFunc(w.stall, func() {
		cancel(&requestTimeoutError{timeout: w.stall, kind: timeoutStalled})
	})
	return ctx, w
}

// received holds the clocks while the consumer works on a part. Any part
// pauses the idle budget; only content pauses the stall budget.
func (w *streamWatchdog) received(part fantasy.StreamPart) {
	w.arrived.Store(true)
	w.idleTimer.Stop()
	if part.Type != fantasy.StreamPartTypeKeepalive {
		w.stallTimer.Stop()
	}
}

// waiting re-arms the clocks once the consumer is done with a part.
func (w *streamWatchdog) waiting(part fantasy.StreamPart) {
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
