package agent

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// scriptedStep is one model turn: optional text, an optional tool call,
// the usage the step reports, and the finish reason it ends with. Usage
// matters because the auto-summarize stop condition reads the session's
// token counters, which are fed from the step's usage.
type scriptedStep struct {
	text         string
	toolCallID   string
	toolName     string
	toolInput    string
	usage        fantasy.Usage
	finishReason fantasy.FinishReason
}

// scriptedStreamModel replays a fixed script, one step per Stream call.
// The last step repeats if the agent loop asks for more, so a script
// ending in a non-continuing finish reason terminates the turn.
//
// When entered and gate are set, the first Stream call reports that it
// started and then blocks until the gate is closed. That is what holds
// the first turn "active" long enough for a test to queue a prompt
// behind it.
//
// Use it as the large model only. Title generation streams the small
// model concurrently and would consume script positions; give the agent
// a separate small model that finishes cleanly so the title never falls
// back to the large one.
type scriptedStreamModel struct {
	steps   []scriptedStep
	entered chan struct{}
	gate    chan struct{}
	calls   atomic.Int64
}

func (m *scriptedStreamModel) Provider() string { return "fake" }
func (m *scriptedStreamModel) Model() string    { return "fake-model" }

func (m *scriptedStreamModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return &fantasy.Response{
		Content:      fantasy.ResponseContent{fantasy.TextContent{Text: "title"}},
		FinishReason: fantasy.FinishReasonStop,
	}, nil
}

func (m *scriptedStreamModel) Stream(ctx context.Context, _ fantasy.Call) (fantasy.StreamResponse, error) {
	n := int(m.calls.Add(1))
	if n == 1 && m.entered != nil {
		close(m.entered)
		select {
		case <-m.gate:
		case <-ctx.Done():
		}
	}
	idx := min(n-1, len(m.steps)-1)
	step := m.steps[idx]
	return func(yield func(fantasy.StreamPart) bool) {
		if step.text != "" {
			if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "text-1"}) {
				return
			}
			if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "text-1", Delta: step.text}) {
				return
			}
			if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "text-1"}) {
				return
			}
		}
		if step.toolCallID != "" {
			if !yield(fantasy.StreamPart{
				Type:         fantasy.StreamPartTypeToolInputStart,
				ID:           step.toolCallID,
				ToolCallName: step.toolName,
			}) {
				return
			}
			if !yield(fantasy.StreamPart{
				Type:  fantasy.StreamPartTypeToolInputDelta,
				ID:    step.toolCallID,
				Delta: step.toolInput,
			}) {
				return
			}
			if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolInputEnd, ID: step.toolCallID}) {
				return
			}
			if !yield(fantasy.StreamPart{
				Type:          fantasy.StreamPartTypeToolCall,
				ID:            step.toolCallID,
				ToolCallName:  step.toolName,
				ToolCallInput: step.toolInput,
			}) {
				return
			}
		}
		yield(fantasy.StreamPart{
			Type:         fantasy.StreamPartTypeFinish,
			FinishReason: step.finishReason,
			Usage:        step.usage,
		})
	}, nil
}

func (m *scriptedStreamModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *scriptedStreamModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

type echoToolParams struct {
	Value string `json:"value"`
}

// newEchoTool returns a trivial tool plus the counter of how many times
// it actually ran, so a test can prove a tool call was never dispatched.
func newEchoTool() (fantasy.AgentTool, *atomic.Int64) {
	var runs atomic.Int64
	tool := fantasy.NewAgentTool(
		"echo",
		"Echoes its input back.",
		func(_ context.Context, params echoToolParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			runs.Add(1)
			return fantasy.NewTextResponse("echoed: " + params.Value), nil
		},
	)
	return tool, &runs
}

// toolResultFor returns the stored tool result for the given tool call
// ID, or nil when the transcript has none.
func toolResultFor(msgs []message.Message, toolCallID string) *message.ToolResult {
	for _, m := range msgs {
		if m.Role != message.Tool {
			continue
		}
		for _, tr := range m.ToolResults() {
			if tr.ToolCallID == toolCallID {
				return &tr
			}
		}
	}
	return nil
}

// TestRun_TruncatedStepFinalizesUnrunToolCalls is the regression test for
// the reported mid-run freeze. fantasy skips the whole buffered tool
// dispatch when a step ends with finish_reason=length and still returns
// no error, while crush has already persisted the tool calls. The turn
// used to end "successfully" with tool calls that had no result, which
// every client renders as still running, forever.
//
// Run must instead write a terminal error result for each un-run call
// and fail the turn with ErrToolCallsNotRun.
func TestRun_TruncatedStepFinalizesUnrunToolCalls(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	tool, runs := newEchoTool()
	model := &scriptedStreamModel{steps: []scriptedStep{{
		toolCallID:   "call-1",
		toolName:     "echo",
		toolInput:    `{"value":"hi"}`,
		finishReason: fantasy.FinishReasonLength,
	}}}
	titleModel := &finishStreamModel{text: "a title"}
	sa := testSessionAgent(env, model, titleModel, "system", tool).(*sessionAgent)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "write three files"})
	require.ErrorIs(t, err, ErrToolCallsNotRun,
		"a turn whose tool calls never ran must not report success")
	assert.Zero(t, runs.Load(), "the truncated tool call must not have executed")

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)

	var assistant *message.Message
	for i, m := range msgs {
		if m.Role == message.Assistant {
			assistant = &msgs[i]
		}
	}
	require.NotNil(t, assistant)
	require.Len(t, assistant.ToolCalls(), 1)
	assert.Equal(t, message.FinishReasonMaxTokens, assistant.FinishReason(),
		"the truncation must still be recorded on the assistant message")

	result := toolResultFor(msgs, "call-1")
	require.NotNil(t, result, "the un-run tool call must get a terminal result")
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "output token limit",
		"a truncated turn must say why the call never ran")
}

// TestRun_HealthyToolCallTurnKeepsRealResults pins that the orphan check
// does not fire on a normal tool-calling turn: the tool runs, its real
// result is stored, and the turn succeeds.
func TestRun_HealthyToolCallTurnKeepsRealResults(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	tool, runs := newEchoTool()
	model := &scriptedStreamModel{steps: []scriptedStep{
		{
			toolCallID:   "call-1",
			toolName:     "echo",
			toolInput:    `{"value":"hi"}`,
			finishReason: fantasy.FinishReasonToolCalls,
		},
		{text: "all done", finishReason: fantasy.FinishReasonStop},
	}}
	titleModel := &finishStreamModel{text: "a title"}
	sa := testSessionAgent(env, model, titleModel, "system", tool).(*sessionAgent)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	result, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "echo hi"})
	require.NoError(t, err, "a healthy tool-calling turn must not be failed")
	require.NotNil(t, result)
	assert.Equal(t, int64(1), runs.Load(), "the tool must have executed")

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	stored := toolResultFor(msgs, "call-1")
	require.NotNil(t, stored)
	assert.False(t, stored.IsError)
	assert.Equal(t, "echoed: hi", stored.Content)
	for _, m := range msgs {
		for _, tr := range m.ToolResults() {
			assert.False(t, strings.Contains(tr.Content, "never run"),
				"a healthy turn must not get a synthetic un-run result")
		}
	}
}

// dropToolResultService drops the first stored result for one tool call
// and passes every other write through. That is the second route into a
// transcript with unanswered tool calls: fantasy discards the error the
// OnToolResult callback returns (executeSingleTool), so the tool has
// already run, the model sees its output, and the turn carries on while
// nothing records the result.
type dropToolResultService struct {
	message.Service
	toolCallID string
	dropped    atomic.Bool
}

func (s *dropToolResultService) Create(ctx context.Context, sessionID string, params message.CreateMessageParams) (message.Message, error) {
	if params.Role == message.Tool {
		for _, part := range params.Parts {
			tr, ok := part.(message.ToolResult)
			if ok && tr.ToolCallID == s.toolCallID && s.dropped.CompareAndSwap(false, true) {
				return message.Message{}, errors.New("simulated tool result write failure")
			}
		}
	}
	return s.Service.Create(ctx, sessionID, params)
}

// TestRun_LostToolResultOnEarlierStepIsRepaired pins the repair to the
// whole turn rather than its last step. PrepareStep creates one
// assistant message per step, so a result lost on step one is invisible
// to a scan of the final step's message: the turn used to report
// success with that call still rendering as running, forever.
//
// The tool did run here, so the stored result must not claim otherwise —
// a transcript that says a call never ran invites the model to repeat
// the side effect — and the turn must fail with ErrToolResultsMissing
// rather than the truncation error.
func TestRun_LostToolResultOnEarlierStepIsRepaired(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	dropper := &dropToolResultService{Service: env.messages, toolCallID: "call-1"}
	env.messages = dropper
	tool, runs := newEchoTool()
	model := &scriptedStreamModel{steps: []scriptedStep{
		{
			toolCallID:   "call-1",
			toolName:     "echo",
			toolInput:    `{"value":"hi"}`,
			finishReason: fantasy.FinishReasonToolCalls,
		},
		{text: "all done", finishReason: fantasy.FinishReasonStop},
	}}
	titleModel := &finishStreamModel{text: "a title"}
	sa := testSessionAgent(env, model, titleModel, "system", tool).(*sessionAgent)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "echo hi"})
	require.ErrorIs(t, err, ErrToolResultsMissing,
		"a turn that lost a tool result must not report success")
	require.True(t, dropper.dropped.Load(), "the test never dropped a result write")
	assert.Equal(t, int64(1), runs.Load(),
		"the tool ran; only the write of its result failed")
	assert.Equal(t, int64(2), model.calls.Load(),
		"the lost write must not stop the turn early")

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	result := toolResultFor(msgs, "call-1")
	require.NotNil(t, result,
		"the unanswered call from the first step must be repaired")
	assert.True(t, result.IsError)
	assert.NotContains(t, result.Content, "never run",
		"the tool ran, so the transcript must not say it did not")
	assert.Contains(t, result.Content, "No result was recorded")

	// The repair also has to leave the session usable: its result row is
	// appended after the second step's assistant message, so the next
	// prompt is only valid if the answer is paired with its call rather
	// than sent from where it was stored.
	history, _ := sa.preparePrompt(msgs, true)
	requireAdjacentToolResult(t, history, "call-1")
}

// collectRunCompletes reads terminal events off the broker until it has
// one per expected RunID, keyed by RunID. It fails the test rather than
// blocking forever when an expected event never arrives, which is the
// exact symptom of a stranded queued prompt.
func collectRunCompletes(t *testing.T, ch <-chan pubsub.Event[notify.RunComplete], runIDs ...string) map[string]notify.RunComplete {
	t.Helper()
	got := make(map[string]notify.RunComplete, len(runIDs))
	deadline := time.After(10 * time.Second)
	for len(got) < len(runIDs) {
		select {
		case ev := <-ch:
			got[ev.Payload.RunID] = ev.Payload
		case <-deadline:
			t.Fatalf("timed out waiting for RunComplete events %v, got %v", runIDs, got)
		}
	}
	for _, id := range runIDs {
		require.Contains(t, got, id, "missing the terminal event for %q", id)
	}
	return got
}

// TestRun_TruncatedTurnStillRunsQueuedPrompt covers the queue side of the
// truncated-turn failure. A queued prompt that carries a RunID is left in
// the queue on purpose (drainQueueForStep) so it runs as its own turn and
// publishes its own terminal event; a second `crush run` against a busy
// session is exactly that shape, and it exits only on a RunComplete for
// its own RunID.
//
// Failing the active turn must therefore not skip the end-of-turn queue
// handoff: the queued prompt would stay in the queue with the session no
// longer busy, nothing would start it, and its client would wait forever.
func TestRun_TruncatedTurnStillRunsQueuedPrompt(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	broker := pubsub.NewBroker[notify.RunComplete]()
	t.Cleanup(broker.Shutdown)

	tool, runs := newEchoTool()
	// The first turn blocks inside Stream until the gate opens, which is
	// what makes the session busy long enough to queue behind it, and
	// then truncates. The queued turn gets the clean second step.
	model := &scriptedStreamModel{
		entered: make(chan struct{}),
		gate:    make(chan struct{}),
		steps: []scriptedStep{
			{
				toolCallID:   "call-1",
				toolName:     "echo",
				toolInput:    `{"value":"hi"}`,
				finishReason: fantasy.FinishReasonLength,
			},
			{text: "the queued turn ran", finishReason: fantasy.FinishReasonStop},
		},
	}
	sa := NewSessionAgent(SessionAgentOptions{
		LargeModel:  Model{Model: model, CatwalkCfg: catwalk.Model{ContextWindow: 200000, DefaultMaxTokens: 10000}},
		SmallModel:  Model{Model: &finishStreamModel{text: "a title"}, CatwalkCfg: catwalk.Model{ContextWindow: 200000, DefaultMaxTokens: 10000}},
		Sessions:    env.sessions,
		Messages:    env.messages,
		Tools:       []fantasy.AgentTool{tool},
		RunComplete: broker,
	}).(*sessionAgent)

	subCtx, subCancel := context.WithCancel(t.Context())
	defer subCancel()
	events := broker.Subscribe(subCtx)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	mainDone := make(chan error, 1)
	go func() {
		_, runErr := sa.Run(t.Context(), SessionAgentCall{
			SessionID: sess.ID,
			RunID:     "run-main",
			Prompt:    "main",
		})
		mainDone <- runErr
	}()

	select {
	case <-model.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the first run never entered Stream")
	}
	require.True(t, sa.IsSessionBusy(sess.ID))

	res, err := sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		RunID:     "run-follow",
		Prompt:    "follow",
	})
	require.NoError(t, err)
	require.Nil(t, res, "the second prompt must be queued, not run inline")
	require.Equal(t, 1, sa.QueuedPrompts(sess.ID))

	close(model.gate)
	require.ErrorIs(t, <-mainDone, ErrToolCallsNotRun,
		"the turn whose tool call never ran must still fail")

	completes := collectRunCompletes(t, events, "run-main", "run-follow")
	assert.Equal(t, ErrToolCallsNotRun.Error(), completes["run-main"].Error,
		"the failed turn must report its own failure under its own RunID")
	assert.Empty(t, completes["run-follow"].Error,
		"the queued turn ran on its own and succeeded")
	assert.False(t, completes["run-follow"].Cancelled,
		"the queued prompt was never cancelled")

	assert.Zero(t, sa.QueuedPrompts(sess.ID), "the queue must be drained")
	assert.Zero(t, runs.Load(), "the truncated tool call must not have executed")
	assert.Equal(t, int64(2), model.calls.Load(), "the queued prompt must reach the model")
}
