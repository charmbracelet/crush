package agent

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"testing"
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

func TestNewRequestTimeoutModel_Disabled(t *testing.T) {
	t.Parallel()

	inner := &fakeLanguageModel{}
	require.Same(t, inner, newRequestTimeoutModel(inner, 0))
	require.Same(t, inner, newRequestTimeoutModel(inner, -time.Second))
}

// newTestRequestTimeoutModel builds a model whose first-part budget equals
// the idle window, so deadline tests fire in milliseconds instead of
// waiting out [defaultFirstPartTimeout].
func newTestRequestTimeoutModel(m fantasy.LanguageModel, timeout time.Duration) requestTimeoutModel {
	return requestTimeoutModel{LanguageModel: m, timeout: timeout, firstPart: timeout}
}

func TestNewRequestTimeoutModel_FirstPartBudget(t *testing.T) {
	t.Parallel()

	// A tight idle window still grants the full first-part floor: prefill
	// before the first token must not be mistaken for a stalled provider.
	short := newRequestTimeoutModel(&fakeLanguageModel{}, time.Second).(requestTimeoutModel)
	require.Equal(t, time.Second, short.timeout)
	require.Equal(t, defaultFirstPartTimeout, short.firstPart)

	// A budget larger than the floor is used as configured.
	long := newRequestTimeoutModel(&fakeLanguageModel{}, 10*time.Minute).(requestTimeoutModel)
	require.Equal(t, 10*time.Minute, long.firstPart)
}

func TestRequestTimeoutModel_GenerateDeadline(t *testing.T) {
	t.Parallel()

	inner := &fakeLanguageModel{}
	m := newRequestTimeoutModel(inner, 5*time.Minute)

	_, err := m.Generate(t.Context(), fantasy.Call{})
	require.NoError(t, err)

	_, ok := inner.generateCtx.Deadline()
	require.True(t, ok, "Generate should run under a deadline")
}

func TestRequestTimeoutModel_StreamDeadlineOutlivesCall(t *testing.T) {
	t.Parallel()

	inner := &fakeLanguageModel{}
	m := newRequestTimeoutModel(inner, 5*time.Minute)

	stream, err := m.Stream(t.Context(), fantasy.Call{})
	require.NoError(t, err)

	require.NoError(t, inner.streamCtx.Err(), "the idle timer must not fire while the stream is being consumed")

	for range stream {
	}

	require.ErrorIs(t, inner.streamCtx.Err(), context.Canceled, "the stream context should be released after the stream ends")
}

func TestRequestTimeoutModel_StreamAbortsWhenIdle(t *testing.T) {
	t.Parallel()

	inner := &fakeLanguageModel{}
	// A stream that outlives the idle window: it waits for the context to
	// be done and reports what it observed.
	streamObserved := make(chan error, 1)
	inner.stream = func(yield func(fantasy.StreamPart) bool) {
		<-inner.streamCtx.Done()
		streamObserved <- inner.streamCtx.Err()
	}
	m := newTestRequestTimeoutModel(inner, 10*time.Millisecond)
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

func TestRequestTimeoutModel_ActiveStreamSurvives(t *testing.T) {
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
	m := newRequestTimeoutModel(inner, 25*time.Millisecond)
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

func TestRequestTimeoutModel_GenerateReportsTimeout(t *testing.T) {
	t.Parallel()

	m := newRequestTimeoutModel(&blockingModel{}, 10*time.Millisecond)

	_, err := m.Generate(t.Context(), fantasy.Call{})
	require.Error(t, err)

	var timeoutErr *requestTimeoutError
	require.ErrorAs(t, err, &timeoutErr)
	require.Equal(t, 10*time.Millisecond, timeoutErr.timeout)
	require.ErrorIs(t, err, context.DeadlineExceeded, "the deadline must stay detectable through the chain")
	require.Contains(t, err.Error(), "timed out after 10ms")
}

func TestRequestTimeoutModel_StreamReportsTimeout(t *testing.T) {
	t.Parallel()

	inner := &fakeLanguageModel{}
	// A provider stream that fails with the context error once the deadline
	// fires, mirroring how SDKs surface mid-stream aborts.
	inner.stream = func(yield func(fantasy.StreamPart) bool) {
		<-inner.streamCtx.Done()
		yield(fantasy.StreamPart{Error: inner.streamCtx.Err()})
	}
	m := newTestRequestTimeoutModel(inner, 10*time.Millisecond)
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
	require.True(t, timeoutErr.first, "nothing arrived, so this is a no-data timeout")
	require.ErrorIs(t, got, context.DeadlineExceeded)
}

func TestRequestTimeoutModel_ParentCancelPassesThrough(t *testing.T) {
	t.Parallel()

	m := newRequestTimeoutModel(&blockingModel{}, 5*time.Minute)

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

	err.cause = context.DeadlineExceeded
	require.Equal(t, "LLM request timed out after 1s: context deadline exceeded", err.Error())

	idle := &requestTimeoutError{timeout: 2 * time.Second, idle: true}
	require.Equal(t, "LLM stream received no data for 2s", idle.Error())
	require.Contains(t, idle.userMessage(), "stopped sending data for 2s")
	require.Contains(t, idle.userMessage(), "request-timeout")
}

// timeoutOnlyModel streams a single error part shaped exactly like the one
// requestTimeoutModel produces when its deadline fires.
type timeoutOnlyModel struct {
	fakeLanguageModel
}

func (m *timeoutOnlyModel) Stream(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
	timeoutErr := &requestTimeoutError{timeout: time.Second, idle: true, cause: context.DeadlineExceeded}
	return func(yield func(fantasy.StreamPart) bool) {
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: timeoutErr})
	}, nil
}

// TestRequestTimeoutRunFinishMessage pins what the user sees when a request
// exhausts its timeout: a "Request timed out" finish that names the elapsed
// budget and how to change it, instead of a bare provider error.
func TestRequestTimeoutRunFinishMessage(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows for now")
	}

	env := testEnv(t)
	model := &timeoutOnlyModel{}
	agent, err := coderAgent(vcr.NewRecorder(t), env, model, model)
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
}

func TestRequestTimeoutModel_FirstPartWindowOutlivesIdle(t *testing.T) {
	t.Parallel()

	inner := &fakeLanguageModel{}
	// Silence longer than the inter-part window but within the budget
	// granted to the first part: a model still prefilling a large context,
	// or thinking before its first token, is not a hung provider.
	inner.stream = func(yield func(fantasy.StreamPart) bool) {
		time.Sleep(80 * time.Millisecond)
		yield(fantasy.StreamPart{Delta: "hello"})
	}
	m := requestTimeoutModel{
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

func TestRequestTimeoutModel_AbortsOnInterPartGap(t *testing.T) {
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
	m := requestTimeoutModel{
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
	require.False(t, timeoutErr.first, "data already arrived, so this is not a first-part timeout")
	require.Equal(t, 10*time.Millisecond, timeoutErr.timeout)
	require.Contains(t, got.Error(), "received no data for 10ms")
	require.Contains(t, timeoutErr.userMessage(), "stopped sending data for 10ms")
}

func TestRequestTimeoutModel_SlowConsumerDoesNotAbort(t *testing.T) {
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
	m := newTestRequestTimeoutModel(inner, 10*time.Millisecond)
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

// TestRequestTimeoutModel_DoesNotBoundToolCalls pins the invariant that
// request_timeout covers LLM requests only: a tool that waits far past the
// timeout for its job must finish normally, and its context must carry no
// deadline.
func TestRequestTimeoutModel_DoesNotBoundToolCalls(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows for now")
	}

	env := testEnv(t)

	inner := &oneToolCallModel{}
	slow := &slowTool{delay: 500 * time.Millisecond}
	agent := testSessionAgent(env, newRequestTimeoutModel(inner, 100*time.Millisecond), inner, "", slow)

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

// TestWrapTimedOutBreaksCycle guards against a cyclic error chain: a
// provider that reports the context cause hands this very sentinel back,
// and linking the sentinel to itself makes every errors.Is/As walk of it,
// in fantasy's retry classification and in our own, spin forever.
func TestWrapTimedOutBreaksCycle(t *testing.T) {
	t.Parallel()

	timeoutErr := &requestTimeoutError{timeout: time.Minute, idle: true}
	ctx, cancel := context.WithTimeoutCause(context.Background(), 10*time.Millisecond, timeoutErr)
	t.Cleanup(cancel)
	<-ctx.Done()

	err := wrapTimedOut(ctx, timeoutErr, fmt.Errorf("stream aborted: %w", context.Cause(ctx)))

	terminated := make(chan bool, 1)
	go func() { terminated <- errors.Is(err, context.DeadlineExceeded) }()
	select {
	case matched := <-terminated:
		require.True(t, matched, "a timeout must still match context.DeadlineExceeded")
	case <-time.After(2 * time.Second):
		t.Fatal("errors.Is never returned: the error chain is cyclic")
	}
}
