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

// timeoutMessages holds each kind's log and UI wording, given the budget.
var timeoutMessages = [...]struct{ log, user string }{
	timeoutDeadline:  {"LLM request timed out after %s", "The model did not respond within %s."},
	timeoutFirstPart: {"LLM stream sent no first part within %s", "The model did not send any data within %s."},
	timeoutIdle:      {"LLM stream received no data for %s", "The model stopped sending data for %s."},
	timeoutStalled:   {"LLM stream sent only keepalives for %s", "The connection stayed open but the model sent nothing for %s."},
}

// requestTimeoutError reports an exhausted budget; it never wraps a context error.
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

// IsRetryable implements [fantasy.RetryableError]: silence usually means the network dropped.
func (e *requestTimeoutError) IsRetryable() bool { return true }

// userMessage explains the timeout in the UI and how to change the limit.
func (e *requestTimeoutError) userMessage() string {
	const hint = "Increase the limit with \"option request-timeout SECONDS\" or set it to 0 to disable the timeout."
	return fmt.Sprintf(timeoutMessages[e.kind].user, e.timeout) + " " + hint
}

// defaultFirstPartTimeout floors the first-part wait, which covers prefill and thinking.
const defaultFirstPartTimeout = 5 * time.Minute

// keepaliveGraceFactor scales the timeout into the keepalive-only (stall) budget.
const keepaliveGraceFactor = 10

// requestGuardModel ends each request when its run is cancelled or, if set,
// when a request_timeout budget runs out.
type requestGuardModel struct {
	fantasy.LanguageModel
	// timeout is the inactivity window between stream parts; zero disables it.
	timeout time.Duration
	// firstPart is the window granted until the first part arrives.
	firstPart time.Duration
}

// newRequestGuardModel guards m, bounding it by timeout when positive.
func newRequestGuardModel(m fantasy.LanguageModel, timeout time.Duration) fantasy.LanguageModel {
	if m == nil {
		return m
	}
	return requestGuardModel{
		LanguageModel: m,
		timeout:       max(timeout, 0),
		firstPart:     max(timeout, defaultFirstPartTimeout),
	}
}

// wrapTimedOut returns a copy of the timeout that cancelled ctx, caused by
// err unless err is just the cancellation echoed back; other errors pass.
func wrapTimedOut(ctx context.Context, err error) error {
	timeoutErr, ok := context.Cause(ctx).(*requestTimeoutError)
	if err == nil || !ok {
		return err
	}
	wrapped := *timeoutErr
	if !isContextErr(err) && !errors.Is(err, timeoutErr) {
		wrapped.cause = err
	}
	return &wrapped
}

// isContextErr reports an error that retry logic reads as a deliberate abort.
func isContextErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// Generate has no progress signal, so the whole call gets one deadline.
func (m requestGuardModel) Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
	if m.timeout == 0 {
		return m.LanguageModel.Generate(ctx, call)
	}
	ctx, cancel := context.WithTimeoutCause(ctx, m.timeout, &requestTimeoutError{timeout: m.timeout, kind: timeoutDeadline})
	defer cancel()
	resp, err := m.LanguageModel.Generate(ctx, call)
	return resp, wrapTimedOut(ctx, err)
}

// Stream stops at the first part after ctx is done, since buffered parts
// outlive a cancel, and ends any cut-short stream with an error part.
func (m requestGuardModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	ctx, cancel := context.WithCancelCause(ctx)
	watchdog := newStreamWatchdog(cancel, m.timeout, m.firstPart)
	inner, err := m.LanguageModel.Stream(ctx, call)
	if err != nil {
		watchdog.stop()
		err = wrapTimedOut(ctx, err)
		cancel(nil)
		return nil, err
	}
	return func(yield func(fantasy.StreamPart) bool) {
		defer cancel(nil)
		defer watchdog.stop()
		ended := false // the consumer saw an error or finish, or broke off
		inner(func(part fantasy.StreamPart) bool {
			if context.Cause(ctx) != nil {
				return false
			}
			watchdog.received(part)
			keepGoing := yield(part)
			if !keepGoing || part.Type == fantasy.StreamPartTypeError || part.Type == fantasy.StreamPartTypeFinish {
				ended = true
			}
			if keepGoing {
				watchdog.waiting()
			}
			return keepGoing
		})
		if cause := context.Cause(ctx); cause != nil && !ended {
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: wrapTimedOut(ctx, cause)})
		}
	}, nil
}

// streamWatchdog cancels a stream that never starts, goes silent, or only
// keeps alive, timing only provider waits; a nil watchdog watches nothing.
type streamWatchdog struct {
	idle, stall           time.Duration
	idleTimer, stallTimer *time.Timer
	arrived               atomic.Bool
	stallHeld             bool // the last part was content, which pauses the stall budget
}

// newStreamWatchdog starts both budgets, or returns nil when idle is unset.
func newStreamWatchdog(cancel context.CancelCauseFunc, idle, firstPart time.Duration) *streamWatchdog {
	if idle <= 0 {
		return nil
	}
	w := &streamWatchdog{idle: idle, stall: max(idle*keepaliveGraceFactor, firstPart)}
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
	return w
}

// received pauses the idle clock, and the stall clock too for content.
func (w *streamWatchdog) received(part fantasy.StreamPart) {
	if w == nil {
		return
	}
	w.arrived.Store(true)
	w.idleTimer.Stop()
	w.stallHeld = part.Type != fantasy.StreamPartTypeKeepalive
	if w.stallHeld {
		w.stallTimer.Stop()
	}
}

// waiting re-arms the clocks once the consumer is done with a part.
func (w *streamWatchdog) waiting() {
	if w == nil {
		return
	}
	w.idleTimer.Reset(w.idle)
	if w.stallHeld {
		w.stallTimer.Reset(w.stall)
	}
}

// stop releases both timers.
func (w *streamWatchdog) stop() {
	if w == nil {
		return
	}
	w.idleTimer.Stop()
	w.stallTimer.Stop()
}
