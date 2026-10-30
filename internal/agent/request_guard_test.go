package agent

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"charm.land/fantasy"
	"charm.land/x/vcr"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

// fakeLanguageModel is a [fantasy.LanguageModel] stub that records the
// context its methods were called with and can be configured with a custom
// stream body.
type fakeLanguageModel struct {
	generateCtx context.Context
	streamCtx   context.Context
	stream      func(yield func(fantasy.StreamPart) bool)
}

func (f *fakeLanguageModel) Generate(ctx context.Context, _ fantasy.Call) (*fantasy.Response, error) {
	f.generateCtx = ctx
	return &fantasy.Response{}, nil
}

func (f *fakeLanguageModel) Stream(ctx context.Context, _ fantasy.Call) (fantasy.StreamResponse, error) {
	f.streamCtx = ctx
	if f.stream == nil {
		return func(yield func(fantasy.StreamPart) bool) {
			yield(fantasy.StreamPart{})
		}, nil
	}
	return f.stream, nil
}

func (f *fakeLanguageModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return &fantasy.ObjectResponse{}, nil
}

func (f *fakeLanguageModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, nil
}

func (f *fakeLanguageModel) Provider() string { return "fake" }
func (f *fakeLanguageModel) Model() string    { return "fake-model" }

// A disabled timeout drops every budget but still guards the model, since
// cancellation must stop a stream either way.
func TestNewRequestGuardModel_Disabled(t *testing.T) {
	t.Parallel()

	require.Nil(t, newRequestGuardModel(nil, time.Second))
	for _, timeout := range []time.Duration{0, -time.Second} {
		inner := &fakeLanguageModel{}
		_, err := newRequestGuardModel(inner, timeout).Generate(t.Context(), fantasy.Call{})
		require.NoError(t, err)
		_, ok := inner.generateCtx.Deadline()
		require.False(t, ok, "a disabled timeout must not impose a deadline")
	}
}

// newTestRequestGuardModel builds a model whose first-part budget equals
// the idle window, so deadline tests fire in milliseconds instead of
// waiting out [defaultFirstPartTimeout].
func newTestRequestGuardModel(m fantasy.LanguageModel, timeout time.Duration) requestGuardModel {
	return requestGuardModel{LanguageModel: m, timeout: timeout, firstPart: timeout}
}

func TestNewRequestGuardModel_FirstPartBudget(t *testing.T) {
	t.Parallel()

	// A tight idle window still grants the full first-part floor: prefill
	// before the first token must not be mistaken for a stalled provider.
	short := newRequestGuardModel(&fakeLanguageModel{}, time.Second).(requestGuardModel)
	require.Equal(t, time.Second, short.timeout)
	require.Equal(t, defaultFirstPartTimeout, short.firstPart)

	// A budget larger than the floor is used as configured.
	long := newRequestGuardModel(&fakeLanguageModel{}, 10*time.Minute).(requestGuardModel)
	require.Equal(t, 10*time.Minute, long.firstPart)
}

func TestRequestGuardModel_GenerateDeadline(t *testing.T) {
	t.Parallel()

	inner := &fakeLanguageModel{}
	m := newRequestGuardModel(inner, 5*time.Minute)

	_, err := m.Generate(t.Context(), fantasy.Call{})
	require.NoError(t, err)

	_, ok := inner.generateCtx.Deadline()
	require.True(t, ok, "Generate should run under a deadline")
}

func TestRequestGuardModel_StreamDeadlineOutlivesCall(t *testing.T) {
	t.Parallel()

	inner := &fakeLanguageModel{}
	m := newRequestGuardModel(inner, 5*time.Minute)

	stream, err := m.Stream(t.Context(), fantasy.Call{})
	require.NoError(t, err)

	require.NoError(t, inner.streamCtx.Err(), "the idle timer must not fire while the stream is being consumed")

	for range stream {
	}

	require.ErrorIs(t, inner.streamCtx.Err(), context.Canceled, "the stream context should be released after the stream ends")
}

func TestRequestGuardModel_StreamAbortsWhenIdle(t *testing.T) {
	t.Parallel()

	inner := &fakeLanguageModel{}
	// A stream that outlives the idle window: it waits for the context to
	// be done and reports what it observed.
	streamObserved := make(chan error, 1)
	inner.stream = func(yield func(fantasy.StreamPart) bool) {
		<-inner.streamCtx.Done()
		streamObserved <- inner.streamCtx.Err()
	}
	m := newTestRequestGuardModel(inner, 10*time.Millisecond)
	stream, err := m.Stream(t.Context(), fantasy.Call{})
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range stream {
		}
	}()

	select {
	case err := <-streamObserved:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("stream was not aborted by the idle timeout")
	}
	<-done
}

func TestRequestGuardModel_ActiveStreamSurvives(t *testing.T) {
	t.Parallel()

	inner := &fakeLanguageModel{}
	// A stream that keeps sending data: total runtime exceeds the timeout,
	// but every gap is shorter than the idle window, so it must finish.
	inner.stream = func(yield func(fantasy.StreamPart) bool) {
		for range 6 {
			time.Sleep(10 * time.Millisecond)
			if !yield(fantasy.StreamPart{}) {
				return
			}
		}
	}
	m := newRequestGuardModel(inner, 25*time.Millisecond)
	stream, err := m.Stream(t.Context(), fantasy.Call{})
	require.NoError(t, err)

	parts := 0
	for part := range stream {
		require.NoError(t, part.Error)
		parts++
	}
	require.Equal(t, 6, parts)
}

// blockingModel blocks until the context is done and then returns the
// context error, the way a hung provider request would.
type blockingModel struct {
	fakeLanguageModel
}

func (b *blockingModel) Generate(ctx context.Context, _ fantasy.Call) (*fantasy.Response, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestRequestGuardModel_GenerateReportsTimeout(t *testing.T) {
	t.Parallel()

	m := newRequestGuardModel(&blockingModel{}, 10*time.Millisecond)

	_, err := m.Generate(t.Context(), fantasy.Call{})
	require.Error(t, err)

	var timeoutErr *requestTimeoutError
	require.ErrorAs(t, err, &timeoutErr)
	require.Equal(t, 10*time.Millisecond, timeoutErr.timeout)
	// Retry logic treats a context error as a deliberate abort, so a
	// timeout must not match one: losing the network is worth retrying.
	require.NotErrorIs(t, err, context.DeadlineExceeded)
	require.NotErrorIs(t, err, context.Canceled)
	require.Contains(t, err.Error(), "timed out after 10ms")
}

func TestRequestGuardModel_StreamReportsTimeout(t *testing.T) {
	t.Parallel()

	inner := &fakeLanguageModel{}
	// A provider stream that fails with the context error once the deadline
	// fires, mirroring how SDKs surface mid-stream aborts.
	inner.stream = func(yield func(fantasy.StreamPart) bool) {
		<-inner.streamCtx.Done()
		yield(fantasy.StreamPart{Error: inner.streamCtx.Err()})
	}
	m := newTestRequestGuardModel(inner, 10*time.Millisecond)
	stream, err := m.Stream(t.Context(), fantasy.Call{})
	require.NoError(t, err)

	var got error
	for part := range stream {
		if part.Error != nil {
			got = part.Error
		}
	}

	var timeoutErr *requestTimeoutError
	require.ErrorAs(t, got, &timeoutErr)
	require.Equal(t, timeoutFirstPart, timeoutErr.kind, "nothing arrived, so this is a no-data timeout")
	require.NotErrorIs(t, got, context.DeadlineExceeded)
	require.NotErrorIs(t, got, context.Canceled)
}

func TestRequestGuardModel_ParentCancelPassesThrough(t *testing.T) {
	t.Parallel()

	m := newRequestGuardModel(&blockingModel{}, 5*time.Minute)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	_, err := m.Generate(ctx, fantasy.Call{})
	require.ErrorIs(t, err, context.Canceled)

	var timeoutErr *requestTimeoutError
	require.NotErrorAs(t, err, &timeoutErr, "user cancellation must not be reported as a timeout")
}

func TestRequestTimeoutErrorMessages(t *testing.T) {
	t.Parallel()

	err := &requestTimeoutError{timeout: time.Second}
	require.Equal(t, "LLM request timed out after 1s", err.Error())
	require.Contains(t, err.userMessage(), "1s")
	require.Contains(t, err.userMessage(), "request-timeout")

	err.cause = errors.New("connection reset")
	require.Equal(t, "LLM request timed out after 1s: connection reset", err.Error())

	first := &requestTimeoutError{timeout: 5 * time.Minute, kind: timeoutFirstPart}
	require.Equal(t, "LLM stream sent no first part within 5m0s", first.Error())

	idle := &requestTimeoutError{timeout: 2 * time.Second, kind: timeoutIdle}
	require.Equal(t, "LLM stream received no data for 2s", idle.Error())
	require.Contains(t, idle.userMessage(), "stopped sending data for 2s")
	require.Contains(t, idle.userMessage(), "request-timeout")

	stalled := &requestTimeoutError{timeout: 20 * time.Second, kind: timeoutStalled}
	require.Equal(t, "LLM stream sent only keepalives for 20s", stalled.Error())
	require.Contains(t, stalled.userMessage(), "model sent nothing for 20s")
	require.Contains(t, stalled.userMessage(), "request-timeout")
}

// timeoutOnlyModel streams a single error part shaped exactly like the one
// requestGuardModel produces when its deadline fires.
type timeoutOnlyModel struct {
	fakeLanguageModel
	attempts atomic.Int32
}

func (m *timeoutOnlyModel) Stream(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
	m.attempts.Add(1)
	timeoutErr := &requestTimeoutError{timeout: time.Second, kind: timeoutIdle}
	return func(yield func(fantasy.StreamPart) bool) {
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: timeoutErr})
	}, nil
}

// A timeout ends the run with "Request timed out" once its retries run out;
// the bubble fakes the backoff clock.
func TestRequestTimeoutRunFinishMessage(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows for now")
	}
	synctest.Test(t, testRequestTimeoutRunFinishMessage)
}

func testRequestTimeoutRunFinishMessage(t *testing.T) {
	env := testEnv(t)
	model := &timeoutOnlyModel{}
	agent, err := coderAgent(vcr.NewRecorder(t), env, model, fastModel{})
	require.NoError(t, err)

	session, err := env.sessions.Create(t.Context(), "timeout session")
	require.NoError(t, err)

	_, err = agent.Run(t.Context(), SessionAgentCall{
		Prompt:          "Hello",
		SessionID:       session.ID,
		MaxOutputTokens: 10000,
	})
	require.Error(t, err)

	msgs, err := env.messages.List(t.Context(), session.ID)
	require.NoError(t, err)

	var finish *message.Finish
	for _, msg := range msgs {
		if msg.Role != message.Assistant {
			continue
		}
		if part := msg.FinishPart(); part != nil {
			finish = part
		}
	}
	require.NotNil(t, finish, "the assistant message should carry a finish part")
	require.Equal(t, message.FinishReasonError, finish.Reason)
	require.Equal(t, "Request timed out", finish.Message)
	require.Contains(t, finish.Details, "stopped sending data for 1s")
	require.Contains(t, finish.Details, "request-timeout")
	require.EqualValues(t, fantasy.DefaultRetryOptions().MaxRetries+1, model.attempts.Load(), "a timeout must be retried")
}

func TestRequestGuardModel_FirstPartWindowOutlivesIdle(t *testing.T) {
	t.Parallel()

	inner := &fakeLanguageModel{}
	// Silence longer than the inter-part window but within the budget
	// granted to the first part: a model still prefilling a large context,
	// or thinking before its first token, is not a hung provider.
	inner.stream = func(yield func(fantasy.StreamPart) bool) {
		time.Sleep(80 * time.Millisecond)
		yield(fantasy.StreamPart{Delta: "hello"})
	}
	m := requestGuardModel{
		LanguageModel: inner,
		timeout:       20 * time.Millisecond,
		firstPart:     200 * time.Millisecond,
	}
	stream, err := m.Stream(t.Context(), fantasy.Call{})
	require.NoError(t, err)

	parts := 0
	for part := range stream {
		require.NoError(t, part.Error)
		parts++
	}
	require.Equal(t, 1, parts, "the first part must not be cut off by the idle window")
}

func TestRequestGuardModel_AbortsOnInterPartGap(t *testing.T) {
	t.Parallel()

	inner := &fakeLanguageModel{}
	// One part, then silence for the rest of the stream: the abort must be
	// charged to the inter-part window, not to the first-part budget.
	inner.stream = func(yield func(fantasy.StreamPart) bool) {
		if !yield(fantasy.StreamPart{Delta: "first"}) {
			return
		}
		<-inner.streamCtx.Done()
		yield(fantasy.StreamPart{Error: inner.streamCtx.Err()})
	}
	m := requestGuardModel{
		LanguageModel: inner,
		timeout:       10 * time.Millisecond,
		firstPart:     time.Hour,
	}
	stream, err := m.Stream(t.Context(), fantasy.Call{})
	require.NoError(t, err)

	var got error
	for part := range stream {
		if part.Error != nil {
			got = part.Error
		}
	}

	var timeoutErr *requestTimeoutError
	require.ErrorAs(t, got, &timeoutErr)
	require.Equal(t, timeoutIdle, timeoutErr.kind, "data already arrived, so this is not a first-part timeout")
	require.Equal(t, 10*time.Millisecond, timeoutErr.timeout)
	require.Contains(t, got.Error(), "received no data for 10ms")
	require.Contains(t, timeoutErr.userMessage(), "stopped sending data for 10ms")
}

func TestRequestGuardModel_SlowConsumerDoesNotAbort(t *testing.T) {
	t.Parallel()

	inner := &fakeLanguageModel{}
	// A provider that has everything ready to hand over. A consumer that
	// takes longer per part than the idle window used to read as provider
	// silence and killed healthy streams.
	inner.stream = func(yield func(fantasy.StreamPart) bool) {
		for i := range 20 {
			if !yield(fantasy.StreamPart{Delta: fmt.Sprint(i)}) {
				return
			}
		}
	}
	m := newTestRequestGuardModel(inner, 10*time.Millisecond)
	stream, err := m.Stream(t.Context(), fantasy.Call{})
	require.NoError(t, err)

	var got []string
	for part := range stream {
		time.Sleep(30 * time.Millisecond)
		require.NoError(t, part.Error, "work on this side must not be reported as a timeout")
		got = append(got, part.Delta)
	}

	require.Len(t, got, 20)
	for i, delta := range got {
		require.Equal(t, fmt.Sprint(i), delta, "parts must arrive in order")
	}
}

// slowTool blocks for its delay, mirroring a tool that waits on a
// long-running job, and records whether the context it was handed carries a
// deadline.
type slowTool struct {
	delay       time.Duration
	ran         bool
	hadDeadline bool
}

func (*slowTool) Info() fantasy.ToolInfo {
	return fantasy.ToolInfo{
		Name:        "sleeper",
		Description: "Sleeps past the request timeout",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
	}
}

func (*slowTool) ProviderOptions() fantasy.ProviderOptions   { return nil }
func (*slowTool) SetProviderOptions(fantasy.ProviderOptions) {}

func (t *slowTool) Run(ctx context.Context, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
	t.ran = true
	_, t.hadDeadline = ctx.Deadline()
	select {
	case <-time.After(t.delay):
		return fantasy.NewTextResponse("slept"), nil
	case <-ctx.Done():
		return fantasy.ToolResponse{}, ctx.Err()
	}
}

// oneToolCallModel streams a single tool call on the first agent step and
// ends the turn afterwards. Side calls (title generation) carry no tools and
// must not consume the scripted tool call.
type oneToolCallModel struct {
	fakeLanguageModel
	toolSent bool
}

func (m *oneToolCallModel) Stream(_ context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	first := len(call.Tools) > 0 && !m.toolSent
	if first {
		m.toolSent = true
	}
	return func(yield func(fantasy.StreamPart) bool) {
		if first {
			if !yield(fantasy.StreamPart{
				Type:          fantasy.StreamPartTypeToolCall,
				ID:            "call-1",
				ToolCallName:  "sleeper",
				ToolCallInput: "{}",
			}) {
				return
			}
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls})
			return
		}
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
	}, nil
}

// TestRequestGuardModel_DoesNotBoundToolCalls pins the invariant that
// request_timeout covers LLM requests only: a tool that waits far past the
// timeout for its job must finish normally, and its context must carry no
// deadline.
func TestRequestGuardModel_DoesNotBoundToolCalls(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows for now")
	}

	env := testEnv(t)

	inner := &oneToolCallModel{}
	slow := &slowTool{delay: 500 * time.Millisecond}
	agent := testSessionAgent(env, newRequestGuardModel(inner, 100*time.Millisecond), inner, "", slow)

	session, err := env.sessions.Create(t.Context(), "tool timeout session")
	require.NoError(t, err)

	_, err = agent.Run(t.Context(), SessionAgentCall{
		Prompt:          "Hello",
		SessionID:       session.ID,
		MaxOutputTokens: 1000,
	})
	require.NoError(t, err)
	require.True(t, slow.ran, "the tool call should have run")
	require.False(t, slow.hadDeadline, "a tool call must not inherit the request timeout deadline")
}

// An idle timeout usually means the connection died rather than the model
// thinking, since provider keepalives reset the budget, so it is worth
// another attempt whatever failure it wraps.
func TestRequestTimeoutIsRetryable(t *testing.T) {
	t.Parallel()

	err := error(&requestTimeoutError{
		timeout: time.Minute,
		kind:    timeoutIdle,
		cause:   &fantasy.ProviderError{StatusCode: 400},
	})

	var retryable fantasy.RetryableError
	require.ErrorAs(t, err, &retryable)
	require.True(t, retryable.IsRetryable())

	// These are what retry logic reads as "the caller gave up".
	require.NotErrorIs(t, err, context.Canceled)
	require.NotErrorIs(t, err, context.DeadlineExceeded)
}

// drainStream consumes a stream and returns the part types and last error.
func drainStream(t *testing.T, ctx context.Context, model fantasy.LanguageModel) ([]fantasy.StreamPartType, error) {
	t.Helper()
	stream, err := model.Stream(ctx, fantasy.Call{})
	require.NoError(t, err)
	var types []fantasy.StreamPartType
	var lastErr error
	for part := range stream {
		if part.Error != nil {
			lastErr = part.Error
		}
		types = append(types, part.Type)
	}
	return types, lastErr
}

// A keepalive-only stretch longer than the idle budget is a model thinking,
// not a dead connection.
func TestRequestTimeoutSurvivesKeepaliveOnlyStretch(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		const idle = time.Minute
		inner := &fakeLanguageModel{stream: func(yield func(fantasy.StreamPart) bool) {
			for range 4 {
				time.Sleep(idle * 3 / 4)
				if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeKeepalive}) {
					return
				}
			}
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, Delta: "done"})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish})
		}}

		types, err := drainStream(t, t.Context(), newTestRequestGuardModel(inner, idle))
		require.NoError(t, err, "keepalives should have kept the stream alive")
		require.Contains(t, types, fantasy.StreamPartTypeTextDelta)
	})
}

// The first-part budget still fires on a stream that is genuinely silent.
func TestRequestTimeoutStillFiresOnRealSilence(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		const idle = time.Minute
		inner := &fakeLanguageModel{stream: func(yield func(fantasy.StreamPart) bool) {
			time.Sleep(idle * 4)
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, Delta: "too late"})
		}}

		types, err := drainStream(t, t.Context(), newTestRequestGuardModel(inner, idle))
		var timeoutErr *requestTimeoutError
		require.ErrorAs(t, err, &timeoutErr)
		require.Equal(t, timeoutFirstPart, timeoutErr.kind)
		require.NotContains(t, types, fantasy.StreamPartTypeTextDelta, "a part after the timeout must not reach the consumer")
	})
}

// Keepalives alone must not hold a turn open forever: the laptop-sleep case,
// where the connection survives but nothing is being generated behind it.
func TestRequestTimeoutFiresOnKeepaliveOnlyStream(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		const idle = time.Minute
		inner := &fakeLanguageModel{stream: func(yield func(fantasy.StreamPart) bool) {
			for {
				time.Sleep(idle / 2)
				if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeKeepalive}) {
					return
				}
			}
		}}

		start := time.Now()
		_, err := drainStream(t, t.Context(), newTestRequestGuardModel(inner, idle))
		var timeoutErr *requestTimeoutError
		require.ErrorAs(t, err, &timeoutErr)
		require.Equal(t, timeoutStalled, timeoutErr.kind)
		require.Less(t, time.Since(start), idle*keepaliveGraceFactor+idle, "the stall budget, not the idle one, must end it")
	})
}

// Content resets the stall budget, so an active stream outlives it.
func TestRequestTimeoutStallBudgetResetsOnContent(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		const idle = time.Minute
		inner := &fakeLanguageModel{stream: func(yield func(fantasy.StreamPart) bool) {
			for range keepaliveGraceFactor * 2 {
				time.Sleep(idle / 2)
				if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, Delta: "."}) {
					return
				}
			}
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish})
		}}

		_, err := drainStream(t, t.Context(), newTestRequestGuardModel(inner, idle))
		require.NoError(t, err, "an actively streaming response must never be cut off")
	})
}

// A cancelled run must stop consuming a stream whose parts are already
// buffered locally, where cancelling the context reaches nothing, and must
// say why it stopped: a quiet stop reads as a finished stream.
func TestStreamStopsDrainingBufferedPartsOnCancel(t *testing.T) {
	t.Parallel()

	for _, timeout := range []time.Duration{0, time.Minute} {
		t.Run(timeout.String(), func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			delivered := 0
			inner := &fakeLanguageModel{stream: func(yield func(fantasy.StreamPart) bool) {
				for range 5000 {
					if delivered++; delivered == 10 {
						cancel()
					}
					if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, Delta: "x"}) {
						return
					}
				}
			}}

			types, err := drainStream(t, ctx, newRequestGuardModel(inner, timeout))
			require.Len(t, types, 10, "iteration must stop once the run is cancelled")
			require.Equal(t, fantasy.StreamPartTypeError, types[len(types)-1])
			require.ErrorIs(t, err, context.Canceled)
		})
	}
}

// A provider that notices the cancel and returns without an error part
// still leaves the consumer with a reason.
func TestStreamReportsQuietStopOnCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	inner := &fakeLanguageModel{stream: func(yield func(fantasy.StreamPart) bool) {
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolInputStart, ID: "c1"})
		cancel()
	}}

	types, err := drainStream(t, ctx, newRequestGuardModel(inner, 0))
	require.Equal(t, []fantasy.StreamPartType{fantasy.StreamPartTypeToolInputStart, fantasy.StreamPartTypeError}, types)
	require.ErrorIs(t, err, context.Canceled)
}

// cancelAwareTool answers a cancelled context the way MCP tools do: a text
// error naming the cancellation, with no Go error to match on.
type cancelAwareTool struct {
	slowTool
	started chan struct{}
}

func (t *cancelAwareTool) Run(ctx context.Context, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
	close(t.started)
	<-ctx.Done()
	return fantasy.NewTextErrorResponse("context canceled"), nil
}

// A tool cut off by a cancelled turn is recorded as interrupted, so the model
// reads it as retryable rather than as the tool failing.
func TestCancelledTurnRecordsToolAsInterrupted(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows for now")
	}

	env := testEnv(t)
	inner := &oneToolCallModel{}
	tool := &cancelAwareTool{started: make(chan struct{})}
	agent := testSessionAgent(env, newRequestGuardModel(inner, 0), inner, "", tool)

	session, err := env.sessions.Create(t.Context(), "cancelled tool session")
	require.NoError(t, err)

	go func() {
		<-tool.started
		agent.Cancel(session.ID)
	}()
	_, err = agent.Run(t.Context(), SessionAgentCall{Prompt: "Hello", SessionID: session.ID, MaxOutputTokens: 1000})
	require.ErrorIs(t, err, context.Canceled)

	msgs, err := env.messages.List(t.Context(), session.ID)
	require.NoError(t, err)
	var results []message.ToolResult
	for _, msg := range msgs {
		results = append(results, msg.ToolResults()...)
	}
	require.Len(t, results, 1)
	require.True(t, results[0].IsError)
	require.Equal(t, interruptedToolCallResult, results[0].Content)
}
