package agent

import (
	"context"
	"errors"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

type compactionTestModel struct {
	mu    sync.Mutex
	calls []fantasy.Call
}

type toolLoopCompactionTestModel struct {
	mu              sync.Mutex
	calls           []fantasy.Call
	mainCalls       int
	compactionCalls int
}

type compactionFaultModel struct {
	mu        sync.Mutex
	calls     []fantasy.Call
	failAt    int
	failErr   error
	empty     bool
	block     bool
	blockAt   int
	entered   chan struct{}
	release   chan struct{}
	enterOnce sync.Once
}

func (m *compactionFaultModel) Provider() string { return "test" }
func (m *compactionFaultModel) Model() string    { return "fault-model" }

func (m *compactionFaultModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return nil, errors.New("not implemented")
}

func (m *compactionFaultModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	m.mu.Lock()
	m.calls = append(m.calls, call)
	callNumber := len(m.calls)
	m.mu.Unlock()

	if m.block && (m.blockAt == 0 || m.blockAt == callNumber) {
		m.enterOnce.Do(func() { close(m.entered) })
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-m.release:
		}
	}
	if m.failAt == callNumber {
		return nil, m.failErr
	}
	if m.empty {
		return textStream(""), nil
	}
	return textStream("bounded summary"), nil
}

func (m *compactionFaultModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *compactionFaultModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *compactionFaultModel) recordedCalls() []fantasy.Call {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]fantasy.Call(nil), m.calls...)
}

func (m *compactionTestModel) Provider() string { return "test" }
func (m *compactionTestModel) Model() string    { return "test-model" }

func (m *compactionTestModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return nil, errors.New("not implemented")
}

func (m *compactionTestModel) Stream(_ context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	m.mu.Lock()
	m.calls = append(m.calls, call)
	callNumber := len(m.calls)
	m.mu.Unlock()

	text := "summary for bounded chunk " + strings.Repeat("x", callNumber)
	usage := fantasy.Usage{InputTokens: 100, OutputTokens: 10, TotalTokens: 110}
	return func(yield func(fantasy.StreamPart) bool) {
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "summary"}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "summary", Delta: text}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "summary"}) {
			return
		}
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop, Usage: usage})
	}, nil
}

func (m *compactionTestModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *compactionTestModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *compactionTestModel) recordedCalls() []fantasy.Call {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]fantasy.Call(nil), m.calls...)
}

func (m *toolLoopCompactionTestModel) Provider() string { return "test" }
func (m *toolLoopCompactionTestModel) Model() string    { return "test-model" }

func (m *toolLoopCompactionTestModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return nil, errors.New("not implemented")
}

func (m *toolLoopCompactionTestModel) Stream(_ context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	m.mu.Lock()
	m.calls = append(m.calls, call)
	isCompaction := callContainsText(call, "You are summarizing a conversation")
	if isCompaction {
		m.compactionCalls++
	} else {
		m.mainCalls++
	}
	mainCall := m.mainCalls
	m.mu.Unlock()

	if isCompaction {
		return textStream("bounded compacted context"), nil
	}
	if mainCall == 1 {
		return toolCallStream("large-output", "tool-1", `{}`), nil
	}
	return textStream("continued after compaction"), nil
}

func (m *toolLoopCompactionTestModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *toolLoopCompactionTestModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *toolLoopCompactionTestModel) state() (calls []fantasy.Call, mainCalls, compactionCalls int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]fantasy.Call(nil), m.calls...), m.mainCalls, m.compactionCalls
}

func callContainsText(call fantasy.Call, text string) bool {
	for _, msg := range call.Prompt {
		for _, part := range msg.Content {
			if content, ok := fantasy.AsMessagePart[fantasy.TextPart](part); ok && strings.Contains(content.Text, text) {
				return true
			}
		}
	}
	return false
}

func textStream(text string) fantasy.StreamResponse {
	return func(yield func(fantasy.StreamPart) bool) {
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "text"}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "text", Delta: text}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "text"}) {
			return
		}
		yield(fantasy.StreamPart{
			Type:         fantasy.StreamPartTypeFinish,
			FinishReason: fantasy.FinishReasonStop,
			Usage:        fantasy.Usage{InputTokens: 100, OutputTokens: 10, TotalTokens: 110},
		})
	}
}

func toolCallStream(toolName, toolCallID, input string) fantasy.StreamResponse {
	return func(yield func(fantasy.StreamPart) bool) {
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolInputStart, ID: toolCallID, ToolCallName: toolName}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolInputDelta, ID: toolCallID, Delta: input}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolInputEnd, ID: toolCallID}) {
			return
		}
		if !yield(fantasy.StreamPart{
			Type:          fantasy.StreamPartTypeToolCall,
			ID:            toolCallID,
			ToolCallName:  toolName,
			ToolCallInput: input,
		}) {
			return
		}
		yield(fantasy.StreamPart{
			Type:         fantasy.StreamPartTypeFinish,
			FinishReason: fantasy.FinishReasonToolCalls,
			Usage:        fantasy.Usage{InputTokens: 100, OutputTokens: 10, TotalTokens: 110},
		})
	}
}

func TestCompactMessagesRecoversHistoryLargerThanContextWindow(t *testing.T) {
	t.Parallel()

	provider := &compactionTestModel{}
	model := Model{
		Model: provider,
		CatwalkCfg: catwalk.Model{
			ContextWindow:    4_096,
			DefaultMaxTokens: 512,
		},
	}
	messages := []fantasy.Message{
		fantasy.NewUserMessage(strings.Repeat("large historical context ", 1_000)),
	}

	result, err := (&sessionAgent{}).compactMessages(
		t.Context(),
		model,
		messages,
		nil,
		nil,
		"",
		"session-id",
	)
	require.NoError(t, err)
	require.NotEmpty(t, result.summary)

	calls := provider.recordedCalls()
	require.Equal(t, len(calls), result.calls)
	require.Positive(t, result.passes)
	require.Positive(t, result.estimatedSpend)
	require.Greater(t, len(calls), 1, "oversized history must be summarized in multiple bounded calls")
	for _, call := range calls {
		inputTokens, estimateErr := estimateCallInputTokens(call)
		require.NoError(t, estimateErr)
		require.LessOrEqual(t, inputTokens+contextWindowOutputReserve(model.CatwalkCfg.ContextWindow, call.MaxOutputTokens), model.CatwalkCfg.ContextWindow)
	}
}

func TestCompactMessagesRejectsEmptyConversationWithoutProviderCall(t *testing.T) {
	t.Parallel()

	provider := &compactionFaultModel{}
	_, err := (&sessionAgent{}).compactMessages(t.Context(), Model{
		Model:      provider,
		CatwalkCfg: catwalk.Model{ContextWindow: 4_096, DefaultMaxTokens: 512},
	}, nil, nil, nil, "", "session-id")

	require.ErrorContains(t, err, "cannot compact an empty conversation")
	require.Empty(t, provider.recordedCalls())
}

func TestCompactionOutputTokens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		contextWindow    int64
		defaultMaxTokens int64
		expected         int64
	}{
		{name: "unknown context preserves default", defaultMaxTokens: 1_024, expected: 1_024},
		{name: "default below reserve", contextWindow: 4_096, defaultMaxTokens: 512, expected: 512},
		{name: "default above reserve", contextWindow: 4_096, defaultMaxTokens: 2_000, expected: 819},
		{name: "missing default uses reserve", contextWindow: 204_800, expected: 20_000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expected, compactionOutputTokens(tt.contextWindow, tt.defaultMaxTokens))
		})
	}
}

func TestCompactionTokenBudget(t *testing.T) {
	t.Parallel()

	require.Equal(t, int64(600_000), compactionTokenBudget(200_000, 100_000))
	require.Equal(t, int64(600_000), compactionTokenBudget(100_000, 200_000))
	require.Equal(t, int64(0), compactionTokenBudget(0, 0))
	require.Equal(t, int64(math.MaxInt64), compactionTokenBudget(math.MaxInt64, 1))
	require.Equal(t, int64(math.MaxInt64), saturatingAdd(math.MaxInt64-1, 10))
	require.Equal(t, int64(30), saturatingAdd(10, 20))
	require.Greater(
		t,
		estimateCompactionCallSpend("prompt", "prefix", 512),
		estimateCompactionCallSpend("prompt", "", 512),
	)
}

func TestCompactMessagesRejectsCallBudgetBeforeProviderDispatch(t *testing.T) {
	t.Parallel()

	provider := &compactionFaultModel{}
	_, err := (&sessionAgent{}).compactMessages(t.Context(), Model{
		Model:      provider,
		CatwalkCfg: catwalk.Model{ContextWindow: 4_096, DefaultMaxTokens: 512},
	}, []fantasy.Message{
		fantasy.NewUserMessage(strings.Repeat("history ", 50_000)),
	}, nil, nil, "", "session-id")

	require.ErrorContains(t, err, "64-call safety limit")
	require.Empty(t, provider.recordedCalls(), "unsafe compaction must be rejected before incurring provider cost")
}

func TestCompactMessagesRejectsTokenSpendBudgetBeforeProviderDispatch(t *testing.T) {
	t.Parallel()

	provider := &compactionFaultModel{}
	_, err := (&sessionAgent{}).compactMessages(t.Context(), Model{
		Model:      provider,
		CatwalkCfg: catwalk.Model{ContextWindow: 4_096, DefaultMaxTokens: 512},
	}, []fantasy.Message{
		fantasy.NewUserMessage(strings.Repeat("history ", 1_000)),
	}, nil, nil, strings.Repeat("prefix ", 400), "session-id")

	require.ErrorContains(t, err, "estimated token-spend safety budget")
	require.Empty(t, provider.recordedCalls(), "over-budget pass must be rejected atomically")
}

func TestCompactMessagesRejectsContextTooSmallForPromptOverhead(t *testing.T) {
	t.Parallel()

	provider := &compactionFaultModel{}
	_, err := (&sessionAgent{}).compactMessages(t.Context(), Model{
		Model:      provider,
		CatwalkCfg: catwalk.Model{ContextWindow: 512, DefaultMaxTokens: 128},
	}, []fantasy.Message{fantasy.NewUserMessage("history")}, nil, nil, "", "session-id")

	require.ErrorContains(t, err, "context window is too small for compaction")
	require.Empty(t, provider.recordedCalls())
}

func TestCompactMessagesRejectsEmptyModelSummary(t *testing.T) {
	t.Parallel()

	provider := &compactionFaultModel{empty: true}
	result, err := (&sessionAgent{}).compactMessages(t.Context(), Model{
		Model:      provider,
		CatwalkCfg: catwalk.Model{ContextWindow: 4_096, DefaultMaxTokens: 512},
	}, []fantasy.Message{fantasy.NewUserMessage("history")}, nil, nil, "", "session-id")

	require.ErrorContains(t, err, "model returned an empty summary")
	require.Len(t, provider.recordedCalls(), 1)
	require.Equal(t, int64(100), result.totalUsage.InputTokens)
	require.Equal(t, int64(10), result.totalUsage.OutputTokens)
}

func TestSummarizeChunkFailureDoesNotAdvanceSessionCutoff(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	provider := &compactionFaultModel{failAt: 2, failErr: errors.New("chunk failed")}
	model := Model{
		Model: provider,
		CatwalkCfg: catwalk.Model{
			ContextWindow:    4_096,
			DefaultMaxTokens: 512,
			CostPer1MIn:      10,
			CostPer1MOut:     20,
		},
	}
	agent := testSessionAgent(env, provider, provider, "system prompt").(*sessionAgent)
	agent.SetModels(model, model)

	sess, err := env.sessions.Create(t.Context(), "failure atomicity")
	require.NoError(t, err)
	original, err := env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: strings.Repeat("large historical context ", 1_000)},
		},
	})
	require.NoError(t, err)

	err = agent.Summarize(t.Context(), sess.ID, nil)
	require.ErrorContains(t, err, "chunk failed")
	require.GreaterOrEqual(t, len(provider.recordedCalls()), 2)

	sess, err = env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Empty(t, sess.SummaryMessageID, "failed compaction must not hide the original history")
	require.Equal(t, int64(100), sess.PromptTokens)
	require.Equal(t, int64(10), sess.CompletionTokens)
	require.InDelta(t, 0.0012, sess.Cost, 0.0000001)
	stored, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(stored), 2)
	require.Equal(t, original.ID, stored[0].ID)
	require.True(t, stored[len(stored)-1].IsSummaryMessage, "failed placeholder should be terminally marked for the UI")
}

func TestSummarizeCancellationDeletesPlaceholderAndClearsBusyState(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	provider := &compactionFaultModel{block: true, blockAt: 2, entered: make(chan struct{})}
	model := Model{
		Model:      provider,
		CatwalkCfg: catwalk.Model{ContextWindow: 4_096, DefaultMaxTokens: 512},
	}
	agent := testSessionAgent(env, provider, provider, "system prompt").(*sessionAgent)
	agent.SetModels(model, model)

	sess, err := env.sessions.Create(t.Context(), "cancel compaction")
	require.NoError(t, err)
	_, err = env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: strings.Repeat("large historical context ", 1_000)},
		},
	})
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() { done <- agent.Summarize(t.Context(), sess.ID, nil) }()
	select {
	case <-provider.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("compaction never reached provider")
	}
	require.True(t, agent.IsSessionBusy(sess.ID))
	agent.Cancel(sess.ID)

	select {
	case err = <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled compaction did not return")
	}
	require.False(t, agent.IsSessionBusy(sess.ID))
	sess, err = env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Empty(t, sess.SummaryMessageID)
	require.Equal(t, int64(100), sess.PromptTokens, "completed chunks before cancellation must remain accounted")
	require.Equal(t, int64(10), sess.CompletionTokens)
	stored, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Len(t, stored, 1, "cancelled summary placeholder must be deleted")
}

func TestSummarizeCallerCancellationPersistsCompletedChunkUsage(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	provider := &compactionFaultModel{block: true, blockAt: 2, entered: make(chan struct{})}
	model := Model{
		Model: provider,
		CatwalkCfg: catwalk.Model{
			ContextWindow:    4_096,
			DefaultMaxTokens: 512,
			CostPer1MIn:      10,
			CostPer1MOut:     20,
		},
	}
	agent := testSessionAgent(env, provider, provider, "system prompt").(*sessionAgent)
	agent.SetModels(model, model)

	sess, err := env.sessions.Create(t.Context(), "caller cancel compaction")
	require.NoError(t, err)
	_, err = env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: strings.Repeat("large historical context ", 1_000)},
		},
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- agent.Summarize(ctx, sess.ID, nil) }()
	select {
	case <-provider.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("compaction never reached its second provider call")
	}
	cancel()

	select {
	case err = <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("caller-cancelled compaction did not return")
	}

	sess, err = env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Empty(t, sess.SummaryMessageID)
	require.Equal(t, int64(100), sess.PromptTokens)
	require.Equal(t, int64(10), sess.CompletionTokens)
	require.InDelta(t, 0.0012, sess.Cost, 0.0000001)
	stored, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Len(t, stored, 1, "caller-cancelled summary placeholder must be deleted")
}

func TestSummarizeRejectsAlreadyBusySessionWithoutProviderCall(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	provider := &compactionFaultModel{}
	agent := testSessionAgent(env, provider, provider, "system prompt").(*sessionAgent)
	sess, err := env.sessions.Create(t.Context(), "busy summary")
	require.NoError(t, err)
	agent.activeRequests.Set(sess.ID, func() {})
	t.Cleanup(func() { agent.activeRequests.Del(sess.ID) })

	err = agent.Summarize(t.Context(), sess.ID, nil)
	require.ErrorIs(t, err, ErrSessionBusy)
	require.Empty(t, provider.recordedCalls())
}

func TestSummarizeProcessesPromptQueuedDuringCompactionExactlyOnce(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	provider := &compactionFaultModel{
		block:   true,
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	model := Model{
		Model:      provider,
		CatwalkCfg: catwalk.Model{ContextWindow: 4_096, DefaultMaxTokens: 512},
	}
	agent := testSessionAgent(env, provider, provider, "system prompt").(*sessionAgent)
	agent.SetModels(model, model)

	sess, err := env.sessions.Create(t.Context(), "queued during summary")
	require.NoError(t, err)
	_, err = env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: strings.Repeat("large historical context ", 1_000)},
		},
	})
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() { done <- agent.Summarize(t.Context(), sess.ID, nil) }()
	select {
	case <-provider.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("compaction never reached provider")
	}
	result, err := agent.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "queued follow-up",
	})
	require.NoError(t, err)
	require.Nil(t, result, "run submitted during compaction must queue")
	queued, ok := agent.messageQueue.Get(sess.ID)
	require.True(t, ok)
	require.Len(t, queued, 1)

	close(provider.release)
	select {
	case err = <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("summary did not process queued prompt")
	}
	require.False(t, agent.IsSessionBusy(sess.ID))
	queued, ok = agent.messageQueue.Get(sess.ID)
	require.True(t, !ok || len(queued) == 0)

	stored, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	var followUps int
	for _, msg := range stored {
		if msg.Role == message.User && msg.Content().Text == "queued follow-up" {
			followUps++
		}
	}
	require.Equal(t, 1, followUps, "queued prompt must be persisted and executed exactly once")
}

func TestSummarizePersistsHierarchicalCompactionAsSessionCutoff(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	provider := &compactionTestModel{}
	model := Model{
		Model: provider,
		CatwalkCfg: catwalk.Model{
			ContextWindow:    4_096,
			DefaultMaxTokens: 512,
		},
	}
	agent := testSessionAgent(env, provider, provider, "system prompt").(*sessionAgent)
	agent.SetModels(model, model)

	sess, err := env.sessions.Create(t.Context(), "oversized session")
	require.NoError(t, err)
	_, err = env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: strings.Repeat("historical tool-driven work ", 1_000)},
		},
	})
	require.NoError(t, err)

	err = agent.Summarize(t.Context(), sess.ID, nil)
	require.NoError(t, err)

	sess, err = env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.NotEmpty(t, sess.SummaryMessageID)

	activeMessages, err := agent.getSessionMessages(t.Context(), sess)
	require.NoError(t, err)
	require.Len(t, activeMessages, 1)
	require.Equal(t, sess.SummaryMessageID, activeMessages[0].ID)
	require.True(t, activeMessages[0].IsSummaryMessage)
	require.Equal(t, message.User, activeMessages[0].Role)
	require.NotEmpty(t, activeMessages[0].Content().Text)
	require.Greater(t, len(provider.recordedCalls()), 1)
}

func TestRunAutomaticallyCompactsOversizedExistingSessionBeforeDispatch(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	provider := &compactionTestModel{}
	model := Model{
		Model: provider,
		CatwalkCfg: catwalk.Model{
			ContextWindow:    4_096,
			DefaultMaxTokens: 512,
		},
	}
	agent := testSessionAgent(env, provider, provider, "system prompt").(*sessionAgent)
	agent.SetModels(model, model)

	sess, err := env.sessions.Create(t.Context(), "oversized session")
	require.NoError(t, err)
	_, err = env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: strings.Repeat("historical tool-driven work ", 1_000)},
		},
	})
	require.NoError(t, err)

	result, err := agent.Run(t.Context(), SessionAgentCall{
		SessionID:       sess.ID,
		Prompt:          "continue the work",
		MaxOutputTokens: 512,
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	sess, err = env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.NotEmpty(t, sess.SummaryMessageID)

	calls := provider.recordedCalls()
	require.Greater(t, len(calls), 2, "pre-dispatch recovery should compact in bounded calls before the requested inference")
	for _, call := range calls {
		inputTokens, estimateErr := estimateCallInputTokens(call)
		require.NoError(t, estimateErr)
		require.LessOrEqual(t, inputTokens+contextWindowOutputReserve(model.CatwalkCfg.ContextWindow, call.MaxOutputTokens), model.CatwalkCfg.ContextWindow)
	}
}

func TestRunPreDispatchCompactionFailurePublishesOneTerminalEvent(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	provider := &compactionFaultModel{failAt: 1, failErr: errors.New("compaction unavailable")}
	model := Model{
		Model:      provider,
		CatwalkCfg: catwalk.Model{ContextWindow: 4_096, DefaultMaxTokens: 512},
	}
	agent := testSessionAgent(env, provider, provider, "system prompt").(*sessionAgent)
	agent.SetModels(model, model)

	sess, err := env.sessions.Create(t.Context(), "pre-dispatch failure")
	require.NoError(t, err)
	_, err = env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: strings.Repeat("large historical context ", 1_000)},
		},
	})
	require.NoError(t, err)

	var completions []notify.RunComplete
	_, err = agent.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		RunID:     "pre-dispatch-failure",
		Prompt:    "continue",
		Accepted:  agent.BeginAccepted(sess.ID),
		OnComplete: func(complete notify.RunComplete) {
			completions = append(completions, complete)
		},
	})
	require.ErrorContains(t, err, "compaction unavailable")
	require.Len(t, completions, 1)
	require.Equal(t, "pre-dispatch-failure", completions[0].RunID)
	require.Contains(t, completions[0].Error, "compaction unavailable")
	require.False(t, completions[0].Cancelled)
	require.False(t, agent.IsSessionBusy(sess.ID))

	sess, err = env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Empty(t, sess.SummaryMessageID)
}

func TestRunPreDispatchCompactionCancellationPublishesOneCancelledTerminalEvent(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	provider := &compactionFaultModel{block: true, entered: make(chan struct{})}
	model := Model{
		Model:      provider,
		CatwalkCfg: catwalk.Model{ContextWindow: 4_096, DefaultMaxTokens: 512},
	}
	agent := testSessionAgent(env, provider, provider, "system prompt").(*sessionAgent)
	agent.SetModels(model, model)

	sess, err := env.sessions.Create(t.Context(), "pre-dispatch cancellation")
	require.NoError(t, err)
	_, err = env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: strings.Repeat("large historical context ", 1_000)},
		},
	})
	require.NoError(t, err)

	completion := make(chan notify.RunComplete, 1)
	done := make(chan error, 1)
	go func() {
		_, runErr := agent.Run(t.Context(), SessionAgentCall{
			SessionID: sess.ID,
			RunID:     "pre-dispatch-cancel",
			Prompt:    "continue",
			OnComplete: func(complete notify.RunComplete) {
				completion <- complete
			},
		})
		done <- runErr
	}()
	select {
	case <-provider.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("pre-dispatch compaction never reached provider")
	}
	agent.Cancel(sess.ID)

	select {
	case err = <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled run did not return")
	}
	select {
	case got := <-completion:
		require.Equal(t, "pre-dispatch-cancel", got.RunID)
		require.True(t, got.Cancelled)
		require.Contains(t, got.Error, "context canceled")
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled run did not publish RunComplete")
	}
	select {
	case extra := <-completion:
		t.Fatalf("expected one RunComplete, got extra: %+v", extra)
	default:
	}
	require.False(t, agent.IsSessionBusy(sess.ID))
}

func TestRunCompactsLargeToolResultBeforeContinuing(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	provider := &toolLoopCompactionTestModel{}
	model := Model{
		Model: provider,
		CatwalkCfg: catwalk.Model{
			ContextWindow:    4_096,
			DefaultMaxTokens: 512,
		},
	}
	largeOutputTool := fantasy.NewAgentTool[struct{}](
		"large-output",
		"Return a large result",
		func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse(strings.Repeat("large tool output ", 2_000)), nil
		},
	)
	agent := testSessionAgent(env, provider, provider, "system prompt", largeOutputTool).(*sessionAgent)
	agent.SetModels(model, model)

	sess, err := env.sessions.Create(t.Context(), "tool loop session")
	require.NoError(t, err)
	_, err = env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "prior context"}},
	})
	require.NoError(t, err)
	var completions []notify.RunComplete
	result, err := agent.Run(t.Context(), SessionAgentCall{
		SessionID:       sess.ID,
		RunID:           "tool-loop-compaction",
		Prompt:          "run the large output tool and continue",
		MaxOutputTokens: 512,
		OnComplete: func(complete notify.RunComplete) {
			completions = append(completions, complete)
		},
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	sess, err = env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	calls, mainCalls, compactionCalls := provider.state()
	require.NotEmpty(t, sess.SummaryMessageID)
	require.Equal(t, 2, mainCalls)
	require.Positive(t, compactionCalls)
	require.Len(t, completions, 1, "summarize-and-resume must complete the originating run exactly once")
	require.Equal(t, "tool-loop-compaction", completions[0].RunID)
	require.False(t, callContainsText(calls[0], "You are summarizing a conversation"))
	require.False(t, callContainsText(calls[len(calls)-1], "You are summarizing a conversation"))
	for _, call := range calls[1 : len(calls)-1] {
		require.True(t, callContainsText(call, "You are summarizing a conversation"), "only bounded compaction calls may occur between main attempts")
	}
	for _, call := range calls {
		inputTokens, estimateErr := estimateCallInputTokens(call)
		require.NoError(t, estimateErr)
		require.LessOrEqual(t, inputTokens+contextWindowOutputReserve(model.CatwalkCfg.ContextWindow, call.MaxOutputTokens), model.CatwalkCfg.ContextWindow)
	}
}

func TestSplitTextByApproxTokensPreservesUnicode(t *testing.T) {
	t.Parallel()

	text := strings.Repeat("English 中文 ", 100)
	chunks := splitTextByApproxTokens(text, 25)

	require.Greater(t, len(chunks), 1)
	require.Equal(t, text, strings.Join(chunks, ""))
	for _, chunk := range chunks {
		require.LessOrEqual(t, approxTokenCount(chunk), int64(25))
	}
}

func FuzzSplitTextByApproxTokensPreservesInputAndBudget(f *testing.F) {
	f.Add("English 中文 😀 paths/and_json:{}", int64(25))
	f.Add(strings.Repeat("x", 1_000), int64(64))
	f.Add("", int64(10))
	f.Add(string([]byte{0xd3}), int64(3))

	f.Fuzz(func(t *testing.T, text string, budget int64) {
		// A valid Unicode rune or malformed byte can conservatively cost up
		// to four estimated tokens and must remain indivisible.
		if budget < 4 || budget > 4_096 || len(text) > 64*1024 {
			t.Skip()
		}
		chunks := splitTextByApproxTokens(text, budget)
		require.Equal(t, text, strings.Join(chunks, ""))
		for _, chunk := range chunks {
			require.LessOrEqual(t, approxTokenCount(chunk), budget)
		}
	})
}

func TestRenderCompactionTranscriptOmitsMediaPayload(t *testing.T) {
	t.Parallel()

	mediaData := strings.Repeat("base64-payload", 1_000)
	messages := []fantasy.Message{
		{
			Role: fantasy.MessageRoleTool,
			Content: []fantasy.MessagePart{
				fantasy.ToolResultPart{
					ToolCallID: "call-1",
					Output: fantasy.ToolResultOutputContentMedia{
						Data:      mediaData,
						MediaType: "image/png",
						Text:      "screenshot",
					},
				},
			},
		},
	}

	transcript := renderCompactionTranscript(messages)
	require.NotContains(t, transcript, mediaData)
	require.Contains(t, transcript, "image/png")
	require.Contains(t, transcript, "screenshot")
}

func TestRenderCompactionTranscriptPreservesSemanticPartsInOrder(t *testing.T) {
	t.Parallel()

	filePayload := strings.Repeat("binary-file-payload", 100)
	messages := []fantasy.Message{
		{
			Role: fantasy.MessageRoleAssistant,
			Content: []fantasy.MessagePart{
				fantasy.TextPart{Text: "assistant text"},
				fantasy.ReasoningPart{Text: "reasoning trace"},
				fantasy.ToolCallPart{ToolName: "view", ToolCallID: "call-1", Input: `{"path":"agent.go"}`},
			},
		},
		{
			Role: fantasy.MessageRoleTool,
			Content: []fantasy.MessagePart{
				fantasy.ToolResultPart{
					ToolCallID: "call-1",
					Output:     fantasy.ToolResultOutputContentText{Text: "tool output"},
				},
				fantasy.ToolResultPart{
					ToolCallID: "call-2",
					Output:     fantasy.ToolResultOutputContentError{Error: errors.New("tool failed")},
				},
			},
		},
		{
			Role: fantasy.MessageRoleUser,
			Content: []fantasy.MessagePart{
				fantasy.FilePart{Filename: "screen.png", MediaType: "image/png", Data: []byte(filePayload)},
			},
		},
	}

	transcript := renderCompactionTranscript(messages)
	require.NotContains(t, transcript, filePayload)
	ordered := []string{
		"assistant text",
		"reasoning trace",
		"[tool call view id=call-1]",
		`{"path":"agent.go"}`,
		"[tool result id=call-1]",
		"tool output",
		"[tool result id=call-2]",
		"tool failed",
		`[file "screen.png" media_type="image/png"`,
	}
	cursor := 0
	for _, expected := range ordered {
		next := strings.Index(transcript[cursor:], expected)
		require.NotEqualf(t, -1, next, "missing %q", expected)
		cursor += next + len(expected)
	}
}

func TestAddOptionalCost(t *testing.T) {
	t.Parallel()

	one := 1.25
	two := 2.75
	require.Nil(t, addOptionalCost(nil, nil))
	first := addOptionalCost(nil, &one)
	require.NotNil(t, first)
	require.Equal(t, 1.25, *first)
	require.NotSame(t, &one, first, "cost accumulator must not alias provider metadata")
	total := addOptionalCost(first, &two)
	require.NotNil(t, total)
	require.Equal(t, 4.0, *total)
	require.Equal(t, first, addOptionalCost(first, nil))
}
