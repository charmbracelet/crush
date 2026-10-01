package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"math/rand/v2"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/pubsub"
)

type retryModel struct {
	fantasy.LanguageModel
	policy  config.ResolvedRetry
	now     func() time.Time
	wait    func(context.Context, time.Duration) error
	random  func() float64
	publish pubsub.Publisher[notify.Notification]
}

func newRetryModel(model fantasy.LanguageModel, policy config.ResolvedRetry, publisher ...pubsub.Publisher[notify.Notification]) fantasy.LanguageModel {
	wrapped := retryModel{
		LanguageModel: model, policy: policy, now: time.Now,
		wait: func(ctx context.Context, delay time.Duration) error {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-timer.C:
				return ctx.Err()
			case <-ctx.Done():
				return ctx.Err()
			}
		},
		random: rand.Float64,
	}
	if len(publisher) > 0 {
		wrapped.publish = publisher[0]
	}
	return wrapped
}

func retryableModelError(ctx context.Context, err error) bool {
	if ctx.Err() != nil || err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var timeout *requestTimeoutError
	if errors.As(err, &timeout) {
		return false
	}
	var provider *fantasy.ProviderError
	if errors.As(err, &provider) {
		message := strings.ToLower(provider.Message)
		if provider.IsContextTooLarge() || provider.AuthError || provider.StatusCode == http.StatusUnauthorized || provider.StatusCode == http.StatusForbidden || strings.Contains(message, "exceeds the context window") || strings.Contains(message, "maximum context length") || strings.Contains(message, "context_length_exceeded") || strings.Contains(message, "context window exceeded") {
			return false
		}
		for name, value := range provider.ResponseHeaders {
			if strings.EqualFold(name, "x-should-retry") && strings.EqualFold(value, "false") {
				return false
			}
		}
		return provider.IsRetryable()
	}
	var fantasyErr *fantasy.Error
	if errors.As(err, &fantasyErr) {
		return false
	}
	var network net.Error
	return errors.Is(err, io.ErrUnexpectedEOF) || errors.As(err, &network) || fantasy.IsTransportError(err)
}

func serverRetryDelay(err error, now time.Time) time.Duration {
	var provider *fantasy.ProviderError
	if !errors.As(err, &provider) {
		return 0
	}
	for name, value := range provider.ResponseHeaders {
		if strings.EqualFold(name, "retry-after-ms") {
			if delay, parseErr := strconv.ParseFloat(value, 64); parseErr == nil && delay > 0 && delay <= float64(math.MaxInt64)/float64(time.Millisecond) {
				return time.Duration(delay * float64(time.Millisecond))
			}
		}
	}
	for name, value := range provider.ResponseHeaders {
		if !strings.EqualFold(name, "retry-after") {
			continue
		}
		if seconds, parseErr := strconv.ParseFloat(value, 64); parseErr == nil && seconds > 0 && seconds <= float64(math.MaxInt64)/float64(time.Second) {
			return time.Duration(seconds * float64(time.Second))
		}
		if date, parseErr := http.ParseTime(value); parseErr == nil {
			return max(0, date.Sub(now))
		}
	}
	return 0
}

func (m retryModel) delay(attempt int, err error) time.Duration {
	capDelay := float64(m.policy.InitialDelay) * math.Pow(m.policy.BackoffMultiplier, float64(attempt))
	capDelay = math.Min(float64(m.policy.MaxDelay), capDelay)
	delay := time.Duration(capDelay)
	if m.policy.Jitter == "full" {
		delay = time.Duration(float64(delay) * m.random())
	}
	return max(delay, serverRetryDelay(err, m.now()))
}

func (m retryModel) next(ctx context.Context, firstFailure *time.Time, attempt int, err error) error {
	if !retryableModelError(ctx, err) || attempt >= m.policy.MaxRetries {
		return err
	}
	if firstFailure.IsZero() {
		*firstFailure = m.now()
	}
	delay := m.delay(attempt, err)
	if delay >= m.policy.MaxElapsed-m.now().Sub(*firstFailure) {
		return fmt.Errorf("retry time budget exhausted: %w", err)
	}
	slog.Warn("Provider request failed, retrying", "provider", m.Provider(), "model", m.Model(), "attempt", attempt+2, "max_attempts", m.policy.MaxRetries+1, "delay", delay)
	m.sendRetry(ctx, notify.Notification{Type: notify.TypeRetry, Attempt: attempt + 2, MaxAttempts: m.policy.MaxRetries + 1, DelayMS: delay.Milliseconds(), Category: "transient", Phase: "waiting"})
	waitCtx, cancel := context.WithDeadline(ctx, firstFailure.Add(m.policy.MaxElapsed))
	defer cancel()
	if waitErr := m.wait(waitCtx, delay); waitErr != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("retry time budget exhausted: %w", err)
	}
	return nil
}

func (m retryModel) sendRetry(ctx context.Context, event notify.Notification) {
	if m.publish == nil || tools.GetSessionFromContext(ctx) == "" {
		return
	}
	event.SessionID = tools.GetSessionFromContext(ctx)
	event.RunID = RunIDFromContext(ctx)
	m.publish.Publish(pubsub.UpdatedEvent, event)
}

func (m retryModel) finishRetry(ctx context.Context, retried bool) {
	if retried {
		m.sendRetry(ctx, notify.Notification{Type: notify.TypeRetry, Done: true})
	}
}

func (m retryModel) Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
	var firstFailure time.Time
	defer func() { m.finishRetry(ctx, !firstFailure.IsZero()) }()
	for attempt := 0; ; attempt++ {
		attemptCtx := ctx
		cancel := func() {}
		if !firstFailure.IsZero() {
			attemptCtx, cancel = context.WithDeadline(ctx, firstFailure.Add(m.policy.MaxElapsed))
		}
		response, err := m.LanguageModel.Generate(attemptCtx, call)
		attemptErr := attemptCtx.Err()
		cancel()
		if attemptErr != nil && ctx.Err() == nil {
			return nil, attemptErr
		}
		if err == nil || ctx.Err() != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return response, err
		}
		if nextErr := m.next(ctx, &firstFailure, attempt, err); nextErr != nil {
			return nil, nextErr
		}
	}
}

func (m retryModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	return func(yield func(fantasy.StreamPart) bool) {
		var firstFailure time.Time
		defer func() { m.finishRetry(ctx, !firstFailure.IsZero()) }()
		for attempt := 0; ; attempt++ {
			attemptCtx := ctx
			cancel := func() {}
			streamPolicy := m.policy.StreamPolicy
			if !firstFailure.IsZero() {
				attemptCtx, cancel = context.WithDeadline(ctx, firstFailure.Add(m.policy.MaxElapsed))
			}
			stream, err := m.LanguageModel.Stream(attemptCtx, call)
			var parts []fantasy.StreamPart
			var size int
			var finished, output, stopped bool
			if err == nil && stream == nil {
				err = errors.New("provider returned no stream")
			}
			if err == nil {
				stream(func(part fantasy.StreamPart) bool {
					if part.Type == fantasy.StreamPartTypeError {
						err = part.Error
						return false
					}
					if part.Type == fantasy.StreamPartTypeFinish {
						finished = true
					}
					if part.ProviderExecuted && streamPolicy == "safe_step" {
						streamPolicy = "before_output"
						for _, buffered := range parts {
							if !yield(buffered) {
								stopped = true
								return false
							}
						}
						parts = nil
						output = true
					}
					if part.Type != fantasy.StreamPartTypeKeepalive && part.Type != fantasy.StreamPartTypeWarnings {
						output = true
					}
					if streamPolicy == "safe_step" {
						size += len(part.Delta) + len(part.ToolCallInput) + len(part.ToolCallName) + 256
						if size > 4<<20 {
							err = errors.New("model response exceeds safe retry buffer")
							return false
						}
						parts = append(parts, part)
						return true
					}
					if !yield(part) {
						stopped = true
						return false
					}
					return true
				})
			}
			if err == nil && !finished && !stopped {
				err = fantasy.NewIncompleteStreamError()
			}
			if attemptCtx.Err() != nil && ctx.Err() == nil {
				err = attemptCtx.Err()
			}
			cancel()
			if stopped {
				return
			}
			if err == nil {
				if ctx.Err() != nil {
					yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: ctx.Err()})
					return
				}
				for _, part := range parts {
					if !yield(part) {
						return
					}
				}
				return
			}
			if ctx.Err() != nil {
				err = ctx.Err()
			}
			if (streamPolicy == "before_output" && output) || size > 4<<20 {
				yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: err})
				return
			}
			if nextErr := m.next(ctx, &firstFailure, attempt, err); nextErr != nil {
				yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: nextErr})
				return
			}
		}
	}, nil
}
