package agent

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

type autoReviewStreamModel struct {
	text          string
	err           error
	finishReason  fantasy.FinishReason
	failFirst     bool
	onStream      func(callNumber int64)
	streamCalls   atomic.Int64
	mu            sync.Mutex
	toolNames     [][]string
	systemPrompts []string
}

func (m *autoReviewStreamModel) Provider() string { return "fake" }
func (m *autoReviewStreamModel) Model() string    { return "fake-model" }

func (m *autoReviewStreamModel) Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
	return &fantasy.Response{
		Content:      fantasy.ResponseContent{fantasy.TextContent{Text: m.text}},
		FinishReason: m.finishReason,
	}, nil
}

func (m *autoReviewStreamModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	callNumber := m.streamCalls.Add(1)
	names := make([]string, 0, len(call.Tools))
	for _, tool := range call.Tools {
		names = append(names, tool.GetName())
	}
	systemPrompt := systemPromptFromCall(call)
	m.mu.Lock()
	m.toolNames = append(m.toolNames, names)
	m.systemPrompts = append(m.systemPrompts, systemPrompt)
	m.mu.Unlock()
	if m.onStream != nil {
		m.onStream(callNumber)
	}
	if m.failFirst && callNumber == 1 {
		return nil, m.err
	}
	finishReason := m.finishReason
	if finishReason == "" {
		finishReason = fantasy.FinishReasonStop
	}
	text := m.text
	return func(yield func(fantasy.StreamPart) bool) {
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "1"}) {
			return
		}
		if text != "" && !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "1", Delta: text}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "1"}) {
			return
		}
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: finishReason})
	}, nil
}

func (m *autoReviewStreamModel) toolNamesForCall(index int) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if index < 0 || index >= len(m.toolNames) {
		return nil
	}
	return append([]string(nil), m.toolNames[index]...)
}

func (m *autoReviewStreamModel) systemPromptForCall(index int) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if index < 0 || index >= len(m.systemPrompts) {
		return ""
	}
	return m.systemPrompts[index]
}

func systemPromptFromCall(call fantasy.Call) string {
	if len(call.Prompt) == 0 || call.Prompt[0].Role != fantasy.MessageRoleSystem {
		return ""
	}
	if len(call.Prompt[0].Content) == 0 {
		return ""
	}
	if textPart, ok := fantasy.AsContentType[fantasy.TextPart](call.Prompt[0].Content[0]); ok {
		return textPart.Text
	}
	return ""
}

func (m *autoReviewStreamModel) GenerateObject(ctx context.Context, call fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *autoReviewStreamModel) StreamObject(ctx context.Context, call fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

func newAutoReviewTestAgent(env fakeEnv, primary, review fantasy.LanguageModel) *sessionAgent {
	title := &autoReviewStreamModel{text: "title"}
	modelConfig := catwalk.Model{
		ContextWindow:    200000,
		DefaultMaxTokens: 10000,
	}
	primaryModel := Model{
		Model:      primary,
		CatwalkCfg: modelConfig,
		ModelCfg: config.SelectedModel{
			Provider: primary.Provider(),
			Model:    primary.Model(),
		},
	}
	titleModel := Model{
		Model:      title,
		CatwalkCfg: modelConfig,
		ModelCfg: config.SelectedModel{
			Provider: title.Provider(),
			Model:    title.Model(),
		},
	}
	reviewModel := Model{
		Model:      review,
		CatwalkCfg: modelConfig,
		ModelCfg: config.SelectedModel{
			Provider: review.Provider(),
			Model:    review.Model(),
		},
	}
	return NewSessionAgent(SessionAgentOptions{
		LargeModel:        primaryModel,
		SmallModel:        titleModel,
		SummaryModel:      primaryModel,
		ReviewModel:       reviewModel,
		SystemPrompt:      "system",
		AutoReviewEnabled: true,
		IsYolo:            true,
		Sessions:          env.sessions,
		Messages:          env.messages,
	}).(*sessionAgent)
}

func TestAutoReviewFailureCreatesReviewAndDrainsQueuedPrompt(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	primary := &autoReviewStreamModel{
		text:      "queued done",
		err:       errors.New("provider exploded"),
		failFirst: true,
	}
	review := &autoReviewStreamModel{text: "review done"}
	sa := newAutoReviewTestAgent(env, primary, review)
	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)
	primary.onStream = func(callNumber int64) {
		if callNumber == 1 {
			sa.enqueueCall(SessionAgentCall{SessionID: sess.ID, Prompt: "queued prompt"})
		}
	}

	result, err := sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "first prompt",
	})
	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, int64(2), primary.streamCalls.Load(), "failed turn should be followed by queued prompt")
	require.Equal(t, int64(1), review.streamCalls.Load(), "failed turn should create one auto-review message")
	require.Equal(t, 0, sa.QueuedPrompts(sess.ID))

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	var userPrompts []string
	var errorTurns, reviewTurns, successfulTurns int
	for _, msg := range msgs {
		switch msg.Role {
		case message.User:
			userPrompts = append(userPrompts, msg.Content().String())
		case message.Assistant:
			switch msg.FinishReason() {
			case message.FinishReasonError:
				errorTurns++
			case message.FinishReasonEndTurn:
				if msg.Content().String() == "review done" {
					reviewTurns++
				}
				if msg.Content().String() == "queued done" {
					successfulTurns++
				}
			}
		}
	}
	require.Equal(t, []string{"first prompt", "queued prompt"}, userPrompts)
	require.Equal(t, 1, errorTurns)
	require.Equal(t, 1, reviewTurns)
	require.Equal(t, 1, successfulTurns)
}

func TestAutoReviewMaxTokensDoesNotLoop(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	primary := &autoReviewStreamModel{
		text:         "hit limit",
		finishReason: fantasy.FinishReasonLength,
	}
	review := &autoReviewStreamModel{text: "review done"}
	sa := newAutoReviewTestAgent(env, primary, review)
	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	result, err := sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "first prompt",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, int64(1), primary.streamCalls.Load(), "max-token auto-review must not re-enter the primary model")
	require.Equal(t, int64(1), review.streamCalls.Load(), "max-token turn should create one auto-review message")

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	var userPrompts []string
	var maxTokenTurns, reviewTurns int
	for _, msg := range msgs {
		switch msg.Role {
		case message.User:
			userPrompts = append(userPrompts, msg.Content().String())
		case message.Assistant:
			switch msg.FinishReason() {
			case message.FinishReasonMaxTokens:
				maxTokenTurns++
			case message.FinishReasonEndTurn:
				if msg.Content().String() == "review done" {
					reviewTurns++
				}
			}
		}
	}
	require.Equal(t, []string{"first prompt"}, userPrompts)
	require.Equal(t, 1, maxTokenTurns)
	require.Equal(t, 1, reviewTurns)
}

func TestContextOverflowRetriesOnceWithLeanTools(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	contextErr := &fantasy.ProviderError{
		StatusCode: 400,
		Title:      "invalid_request_error",
		Message:    "request (39707 tokens) exceeds the available context size (32768 tokens)",
	}
	primary := &autoReviewStreamModel{
		text:      "checked storage",
		err:       contextErr,
		failFirst: true,
	}
	review := &autoReviewStreamModel{text: "review should not run"}
	sa := newAutoReviewTestAgent(env, primary, review)
	sa.largePromptPrefix.Set("long provider prefix")
	sa.systemPrompt.Set("long base prompt")
	sa.recoveryContext.Set("Current working directory: C:/work/project\n\n## C:/work/project/AGENTS.md\nKeep project context during recovery.")
	sa.SetTools([]fantasy.AgentTool{
		&fakeTool{name: tools.BashToolName},
		&fakeTool{name: tools.ViewToolName},
		&fakeTool{name: tools.WriteToolName},
		&fakeTool{name: tools.ReadMCPResourceToolName},
	})
	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	result, err := sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "check the storage and cache for me",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, int64(2), primary.streamCalls.Load(), "context overflow should retry once")
	require.Equal(t, int64(0), review.streamCalls.Load(), "context overflow must not trigger auto-review")
	require.ElementsMatch(t, []string{tools.BashToolName, tools.ViewToolName, tools.WriteToolName, tools.ReadMCPResourceToolName}, primary.toolNamesForCall(0))
	require.ElementsMatch(t, []string{tools.BashToolName, tools.ViewToolName}, primary.toolNamesForCall(1))
	require.Contains(t, primary.systemPromptForCall(0), "long provider prefix")
	require.Contains(t, primary.systemPromptForCall(0), "long base prompt")
	require.Contains(t, primary.systemPromptForCall(1), "compact recovery mode")
	require.Contains(t, primary.systemPromptForCall(1), "Current working directory: C:/work/project")
	require.Contains(t, primary.systemPromptForCall(1), "Keep project context during recovery.")
	require.NotContains(t, primary.systemPromptForCall(1), "long provider prefix")
	require.NotContains(t, primary.systemPromptForCall(1), "long base prompt")

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	var userPrompts []string
	var assistants []message.Message
	for _, msg := range msgs {
		if msg.Role == message.User {
			userPrompts = append(userPrompts, msg.Content().String())
		}
		if msg.Role == message.Assistant {
			assistants = append(assistants, msg)
		}
	}
	require.Equal(t, []string{"check the storage and cache for me"}, userPrompts)
	require.Len(t, assistants, 1, "failed pre-retry assistant should be removed")
	require.Equal(t, "checked storage", assistants[0].Content().String())
}
