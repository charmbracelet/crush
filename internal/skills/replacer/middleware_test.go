package replacer

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/event"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	// Skip indicator sleeps in tests.
	coachStopDisplay = 0
	coachContinueDisplay = 0
	m.Run()
}

// drainFlashIndicators consumes any pending flash-done signals so tests don't
// leak goroutine state across assertions.
func drainFlashIndicators(t *testing.T) {
	t.Helper()
	for {
		select {
		case <-flashDoneCh:
		default:
			return
		}
	}
}

// mockAgent simulates a primary agent that returns a fixed response.
type mockAgent struct {
	response  string
	calls     []agent.SessionAgentCall
	err       error
	busy      bool
	returnNil bool
}

func (m *mockAgent) Run(ctx context.Context, call agent.SessionAgentCall) (*fantasy.AgentResult, error) {
	m.calls = append(m.calls, call)
	if m.err != nil {
		return nil, m.err
	}
	if m.returnNil {
		return nil, nil
	}
	return &fantasy.AgentResult{
		Response: fantasy.Response{
			Content: []fantasy.Content{
				fantasy.TextContent{Text: m.response},
			},
		},
	}, nil
}
func (m *mockAgent) SetModels(large, small agent.Model)          {}
func (m *mockAgent) SetTools(tools []fantasy.AgentTool)          {}
func (m *mockAgent) SetSystemPrompt(systemPrompt string)         {}
func (m *mockAgent) Cancel(sessionID string)                     {}
func (m *mockAgent) CancelAll()                                  {}
func (m *mockAgent) IsSessionBusy(sessionID string) bool         { return m.busy }
func (m *mockAgent) IsBusy() bool                                { return m.busy }
func (m *mockAgent) QueuedPrompts(sessionID string) int          { return 0 }
func (m *mockAgent) QueuedPromptsList(sessionID string) []string { return nil }
func (m *mockAgent) ClearQueue(sessionID string)                 {}
func (m *mockAgent) Summarize(context.Context, string, fantasy.ProviderOptions) error {
	return nil
}
func (m *mockAgent) Model() agent.Model { return agent.Model{} }

// mockMessageService tracks created messages.
type mockMessageService struct {
	mu          sync.Mutex
	messages    []message.Message
	listErr     error
	createErr   error
	deleteErr   error
	subscribeCh <-chan pubsub.Event[message.Message]
}

func (m *mockMessageService) Create(ctx context.Context, sessionID string, params message.CreateMessageParams) (message.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.createErr != nil {
		return message.Message{}, m.createErr
	}
	msg := message.Message{ID: "msg-" + string(rune(len(m.messages)+'a')), SessionID: sessionID, Role: params.Role, Parts: params.Parts, SpinnerLabel: params.SpinnerLabel}
	m.messages = append(m.messages, msg)
	return msg, nil
}
func (m *mockMessageService) Update(ctx context.Context, msg message.Message) error { return nil }
func (m *mockMessageService) Get(ctx context.Context, id string) (message.Message, error) {
	return message.Message{}, nil
}
func (m *mockMessageService) List(ctx context.Context, sessionID string) ([]message.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.listErr != nil {
		return nil, m.listErr
	}
	// Return the original user message plus any created messages.
	base := []message.Message{
		{ID: "u1", SessionID: sessionID, Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "hi"}}},
		{ID: "a1", SessionID: sessionID, Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "hello there"}}},
	}
	return append(base, m.messages...), nil
}
func (m *mockMessageService) ListUserMessages(ctx context.Context, sessionID string) ([]message.Message, error) {
	return nil, nil
}
func (m *mockMessageService) ListAllUserMessages(ctx context.Context) ([]message.Message, error) {
	return nil, nil
}
func (m *mockMessageService) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.deleteErr != nil {
		return m.deleteErr
	}
	for i, msg := range m.messages {
		if msg.ID == id {
			m.messages = append(m.messages[:i], m.messages[i+1:]...)
			break
		}
	}
	return nil
}
func (m *mockMessageService) DeleteSessionMessages(ctx context.Context, sessionID string) error {
	return nil
}
func (m *mockMessageService) Flush(ctx context.Context, id string) error { return nil }
func (m *mockMessageService) FlushAll(ctx context.Context) error         { return nil }
func (m *mockMessageService) Subscribe(ctx context.Context) <-chan pubsub.Event[message.Message] {
	if m.subscribeCh != nil {
		return m.subscribeCh
	}
	return nil
}

// MessageCount returns the number of messages currently stored, excluding the
// base messages returned by List.
func (m *mockMessageService) MessageCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.messages)
}

func TestNewMiddleware_NilPrimary(t *testing.T) {
	t.Parallel()
	require.Nil(t, NewMiddleware(nil, ReplacerConfig{Enabled: true}))
}

func TestMiddleware_Run_Stop(t *testing.T) {
	t.Parallel()

	msgSvc := &mockMessageService{}
	primary := &mockAgent{response: "hello there"}
	cfg := ReplacerConfig{Enabled: true, MaxIterations: 2}
	mw := NewMiddleware(primary, cfg)
	mw.SetMessageService(msgSvc)

	mw.SetModelResolver(func(ctx context.Context) (fantasy.LanguageModel, error) {
		return &mockLLM{decision: `{"action":"stop","prompt":""}`}, nil
	})

	result, err := mw.Run(context.Background(), agent.SessionAgentCall{SessionID: "sid", Prompt: "hi"})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "hello there", result.Response.Content.Text())
	// Primary called once; no follow-up injected.
	require.Len(t, primary.calls, 1)
	require.Equal(t, "hi", primary.calls[0].Prompt)
}

func TestMiddleware_Run_Continue(t *testing.T) {
	t.Parallel()

	msgSvc := &mockMessageService{}
	primary := &mockAgent{response: "hello there"}
	cfg := ReplacerConfig{Enabled: true, MaxIterations: 2}
	mw := NewMiddleware(primary, cfg)
	mw.SetMessageService(msgSvc)

	callCount := 0
	mw.SetModelResolver(func(ctx context.Context) (fantasy.LanguageModel, error) {
		callCount++
		if callCount == 1 {
			return &mockLLM{decision: `{"action":"continue","prompt":"Tell me more about your project"}`}, nil
		}
		return &mockLLM{decision: `{"action":"stop","prompt":""}`}, nil
	})

	result, err := mw.Run(context.Background(), agent.SessionAgentCall{SessionID: "sid", Prompt: "hi"})
	require.NoError(t, err)
	require.NotNil(t, result)
	// Primary called twice: original + follow-up.
	require.Len(t, primary.calls, 2)
	require.Equal(t, "hi", primary.calls[0].Prompt)
	require.Equal(t, "[Coach] Tell me more about your project", primary.calls[1].Prompt)
	require.Nil(t, primary.calls[1].Attachments)
}

func TestMiddleware_Run_DuplicatePromptStops(t *testing.T) {
	t.Parallel()

	msgSvc := &mockMessageService{}
	// Seed a prior coach prompt into the message history.
	_, _ = msgSvc.Create(context.Background(), "sid", message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "[Coach] What would you like help with today?"}},
	})

	primary := &mockAgent{response: "hello there"}
	cfg := ReplacerConfig{Enabled: true, MaxIterations: 2}
	mw := NewMiddleware(primary, cfg)
	mw.SetMessageService(msgSvc)

	mw.SetModelResolver(func(ctx context.Context) (fantasy.LanguageModel, error) {
		return &mockLLM{decision: `{"action":"continue","prompt":"What would you like help with today?"}`}, nil
	})

	result, err := mw.Run(context.Background(), agent.SessionAgentCall{SessionID: "sid", Prompt: "hi"})
	require.NoError(t, err)
	require.NotNil(t, result)
	// Primary called only once because the duplicate follow-up was blocked.
	require.Len(t, primary.calls, 1)
	require.Equal(t, "hi", primary.calls[0].Prompt)
}

func TestMiddleware_Run_DuplicatePromptInSameRunStops(t *testing.T) {
	t.Parallel()

	msgSvc := &mockMessageService{}
	primary := &mockAgent{response: "hello there"}
	cfg := ReplacerConfig{Enabled: true, MaxIterations: 3}
	mw := NewMiddleware(primary, cfg)
	mw.SetMessageService(msgSvc)

	callCount := 0
	mw.SetModelResolver(func(ctx context.Context) (fantasy.LanguageModel, error) {
		callCount++
		if callCount == 1 {
			return &mockLLM{decision: `{"action":"continue","prompt":"Tell me more"}`}, nil
		}
		// Second evaluation suggests the exact same prompt again.
		return &mockLLM{decision: `{"action":"continue","prompt":"Tell me more"}`}, nil
	})

	result, err := mw.Run(context.Background(), agent.SessionAgentCall{SessionID: "sid", Prompt: "hi"})
	require.NoError(t, err)
	require.NotNil(t, result)
	// Primary called twice: original + one follow-up. The second evaluation
	// returns the same prompt, which is caught by the in-memory seen map.
	require.Len(t, primary.calls, 2)
	require.Equal(t, "hi", primary.calls[0].Prompt)
	require.Equal(t, "[Coach] Tell me more", primary.calls[1].Prompt)
}

func TestMiddleware_Run_Disabled(t *testing.T) {
	t.Parallel()

	primary := &mockAgent{response: "hello"}
	cfg := ReplacerConfig{Enabled: false}
	mw := NewMiddleware(primary, cfg)

	result, err := mw.Run(context.Background(), agent.SessionAgentCall{SessionID: "sid", Prompt: "hi"})
	require.NoError(t, err)
	require.Equal(t, "hello", result.Response.Content.Text())
}

func TestMiddleware_Run_PerSessionDisable(t *testing.T) {
	t.Parallel()

	msgSvc := &mockMessageService{}
	primary := &mockAgent{response: "hello there"}
	cfg := ReplacerConfig{Enabled: true, MaxIterations: 2}
	mw := NewMiddleware(primary, cfg)
	mw.SetMessageService(msgSvc)
	mw.SetModelResolver(func(ctx context.Context) (fantasy.LanguageModel, error) {
		return &mockLLM{decision: `{"action":"continue","prompt":"Go on"}`}, nil
	})

	disabled := false
	result, err := mw.Run(context.Background(), agent.SessionAgentCall{SessionID: "sid", Prompt: "hi", ReplacerEnabled: &disabled})
	require.NoError(t, err)
	require.Equal(t, "hello there", result.Response.Content.Text())
	require.Len(t, primary.calls, 1)
}

func TestMiddleware_Run_EmitsEvents(t *testing.T) {
	// Not parallel because it sets a global event hook.
	var events []string
	var eventsMu sync.Mutex
	event.SetTestHook(func(event string, props ...any) {
		eventsMu.Lock()
		events = append(events, event)
		eventsMu.Unlock()
	})
	defer event.ResetTestHook()

	msgSvc := &mockMessageService{}
	primary := &mockAgent{response: "hello there"}
	cfg := ReplacerConfig{Enabled: true, MaxIterations: 2}
	mw := NewMiddleware(primary, cfg)
	mw.SetMessageService(msgSvc)
	mw.SetModelResolver(func(ctx context.Context) (fantasy.LanguageModel, error) {
		return &mockLLM{decision: `{"action":"stop","prompt":""}`}, nil
	})

	_, err := mw.Run(context.Background(), agent.SessionAgentCall{SessionID: "sid", Prompt: "hi"})
	require.NoError(t, err)

	eventsMu.Lock()
	defer eventsMu.Unlock()
	require.Contains(t, events, "replacer.decision")
	require.Contains(t, events, "replacer.loop.completed")
}

func TestMiddleware_Run_NilResult(t *testing.T) {
	t.Parallel()

	primary := &mockAgent{returnNil: true}
	msgSvc := &mockMessageService{}
	cfg := ReplacerConfig{Enabled: true, MaxIterations: 2}
	mw := NewMiddleware(primary, cfg)
	mw.SetMessageService(msgSvc)
	mw.SetModelResolver(func(ctx context.Context) (fantasy.LanguageModel, error) {
		return &mockLLM{decision: `{"action":"stop","prompt":""}`}, nil
	})

	result, err := mw.Run(context.Background(), agent.SessionAgentCall{SessionID: "sid", Prompt: "hi"})
	require.NoError(t, err)
	require.Nil(t, result)
	// Spinner should be cleaned up even when primary returns nil.
	time.Sleep(50 * time.Millisecond)
	require.Zero(t, msgSvc.MessageCount())
}

func TestMiddleware_Run_PrimaryError(t *testing.T) {
	t.Parallel()

	primary := &mockAgent{err: errors.New("primary failed")}
	msgSvc := &mockMessageService{}
	cfg := ReplacerConfig{Enabled: true, MaxIterations: 2}
	mw := NewMiddleware(primary, cfg)
	mw.SetMessageService(msgSvc)
	mw.SetModelResolver(func(ctx context.Context) (fantasy.LanguageModel, error) {
		return &mockLLM{decision: `{"action":"stop","prompt":""}`}, nil
	})

	result, err := mw.Run(context.Background(), agent.SessionAgentCall{SessionID: "sid", Prompt: "hi"})
	require.Error(t, err)
	require.Nil(t, result)
	// Spinner should be cleaned up even when primary returns an error.
	time.Sleep(50 * time.Millisecond)
	require.Zero(t, msgSvc.MessageCount())
}

func TestMiddleware_Run_ContextCancelledAfterPrimary(t *testing.T) {
	t.Parallel()

	msgSvc := &mockMessageService{}
	primary := &mockAgent{response: "hello there"}
	cfg := ReplacerConfig{Enabled: true, MaxIterations: 2}
	mw := NewMiddleware(primary, cfg)
	mw.SetMessageService(msgSvc)
	mw.SetModelResolver(func(ctx context.Context) (fantasy.LanguageModel, error) {
		return &mockLLM{decision: `{"action":"stop","prompt":""}`}, nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel after primary returns but before evaluation starts.
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	result, err := mw.Run(ctx, agent.SessionAgentCall{SessionID: "sid", Prompt: "hi"})
	require.NoError(t, err)
	require.NotNil(t, result)
	// Spinner should be cleaned up when context is cancelled before evaluation.
	time.Sleep(50 * time.Millisecond)
	require.Zero(t, msgSvc.MessageCount())
}

func TestMiddleware_Run_MaxIterations(t *testing.T) {
	t.Parallel()

	msgSvc := &mockMessageService{}
	primary := &mockAgent{response: "hello there"}
	cfg := ReplacerConfig{Enabled: true, MaxIterations: 1}
	mw := NewMiddleware(primary, cfg)
	mw.SetMessageService(msgSvc)

	mw.SetModelResolver(func(ctx context.Context) (fantasy.LanguageModel, error) {
		return &mockLLM{decision: `{"action":"continue","prompt":"Go on"}`}, nil
	})

	result, err := mw.Run(context.Background(), agent.SessionAgentCall{SessionID: "sid", Prompt: "hi"})
	require.NoError(t, err)
	require.NotNil(t, result)
	// Primary called twice: initial + one follow-up before max iterations reached.
	require.Len(t, primary.calls, 2)
}

func TestMiddleware_Run_EvalError(t *testing.T) {
	t.Parallel()

	msgSvc := &mockMessageService{}
	primary := &mockAgent{response: "hello there"}
	cfg := ReplacerConfig{Enabled: true, MaxIterations: 2}
	mw := NewMiddleware(primary, cfg)
	mw.SetMessageService(msgSvc)

	mw.SetModelResolver(func(ctx context.Context) (fantasy.LanguageModel, error) {
		return nil, errors.New("model resolution failed")
	})

	result, err := mw.Run(context.Background(), agent.SessionAgentCall{SessionID: "sid", Prompt: "hi"})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "hello there", result.Response.Content.Text())
}

func TestMiddleware_Run_ListMessagesError(t *testing.T) {
	t.Parallel()

	msgSvc := &mockMessageService{listErr: errors.New("list failed")}
	primary := &mockAgent{response: "hello there"}
	cfg := ReplacerConfig{Enabled: true, MaxIterations: 2}
	mw := NewMiddleware(primary, cfg)
	mw.SetMessageService(msgSvc)

	mw.SetModelResolver(func(ctx context.Context) (fantasy.LanguageModel, error) {
		return &mockLLM{decision: `{"action":"stop","prompt":""}`}, nil
	})

	result, err := mw.Run(context.Background(), agent.SessionAgentCall{SessionID: "sid", Prompt: "hi"})
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestMiddleware_Run_CancelledContext(t *testing.T) {
	t.Parallel()

	msgSvc := &mockMessageService{}
	primary := &mockAgent{response: "hello there"}
	cfg := ReplacerConfig{Enabled: true, MaxIterations: 2}
	mw := NewMiddleware(primary, cfg)
	mw.SetMessageService(msgSvc)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := mw.Run(ctx, agent.SessionAgentCall{SessionID: "sid", Prompt: "hi"})
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestMiddleware_Cancel(t *testing.T) {
	t.Parallel()

	primary := &mockAgent{}
	mw := NewMiddleware(primary, ReplacerConfig{Enabled: true})
	// Should not panic even without evalCancel set.
	mw.Cancel("sid")
}

func TestMiddleware_CancelAll(t *testing.T) {
	t.Parallel()

	primary := &mockAgent{}
	mw := NewMiddleware(primary, ReplacerConfig{Enabled: true})
	// Should not panic even without evalCancel set.
	mw.CancelAll()
}

func TestMiddleware_IsSessionBusy(t *testing.T) {
	t.Parallel()

	primary := &mockAgent{busy: false}
	mw := NewMiddleware(primary, ReplacerConfig{Enabled: true})
	require.False(t, mw.IsSessionBusy("sid"))

	mw.busy.Store(true)
	require.True(t, mw.IsSessionBusy("sid"))
}

func TestMiddleware_IsBusy(t *testing.T) {
	t.Parallel()

	primary := &mockAgent{busy: false}
	mw := NewMiddleware(primary, ReplacerConfig{Enabled: true})
	require.False(t, mw.IsBusy())

	mw.busy.Store(true)
	require.True(t, mw.IsBusy())
}

func TestMiddleware_Delegates(t *testing.T) {
	t.Parallel()

	primary := &mockAgent{}
	mw := NewMiddleware(primary, ReplacerConfig{Enabled: true})

	mw.SetModels(agent.Model{}, agent.Model{})
	mw.SetTools(nil)
	mw.SetSystemPrompt("test")
	mw.ClearQueue("sid")
	require.Equal(t, 0, mw.QueuedPrompts("sid"))
	require.Nil(t, mw.QueuedPromptsList("sid"))
	require.NoError(t, mw.Summarize(context.Background(), "sid", fantasy.ProviderOptions{}))
	require.Equal(t, agent.Model{}, mw.Model())
}

func TestCoachPrompt(t *testing.T) {
	t.Parallel()

	require.Equal(t, "[Coach] What would you like help with?", coachPrompt("What would you like help with?"))
	require.Equal(t, "[Already prefixed]", coachPrompt("[Already prefixed]"))
	require.Equal(t, "[Coach] ", coachPrompt(""))
}

func TestIsDuplicateCoachPrompt(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("nil service", func(t *testing.T) {
		t.Parallel()
		require.False(t, isDuplicateCoachPrompt(ctx, nil, "sid", "hello"))
	})

	t.Run("empty prompt", func(t *testing.T) {
		t.Parallel()
		svc := &mockMessageService{}
		require.False(t, isDuplicateCoachPrompt(ctx, svc, "sid", ""))
	})

	t.Run("no prior coach prompts", func(t *testing.T) {
		t.Parallel()
		svc := &mockMessageService{}
		require.False(t, isDuplicateCoachPrompt(ctx, svc, "sid", "What would you like help with today?"))
	})

	t.Run("duplicate found", func(t *testing.T) {
		t.Parallel()
		svc := &mockMessageService{}
		_, _ = svc.Create(ctx, "sid", message.CreateMessageParams{
			Role:  message.User,
			Parts: []message.ContentPart{message.TextContent{Text: "[Coach] What would you like help with today?"}},
		})
		require.True(t, isDuplicateCoachPrompt(ctx, svc, "sid", "What would you like help with today?"))
	})

	t.Run("duplicate with different case and spacing", func(t *testing.T) {
		t.Parallel()
		svc := &mockMessageService{}
		_, _ = svc.Create(ctx, "sid", message.CreateMessageParams{
			Role:  message.User,
			Parts: []message.ContentPart{message.TextContent{Text: "[Coach]  what WOULD you like help with TODAY?  "}},
		})
		require.True(t, isDuplicateCoachPrompt(ctx, svc, "sid", "What would you like help with today?"))
	})

	t.Run("different prompt", func(t *testing.T) {
		t.Parallel()
		svc := &mockMessageService{}
		_, _ = svc.Create(ctx, "sid", message.CreateMessageParams{
			Role:  message.User,
			Parts: []message.ContentPart{message.TextContent{Text: "[Coach] What would you like help with today?"}},
		})
		require.False(t, isDuplicateCoachPrompt(ctx, svc, "sid", "What are you working on right now?"))
	})

	t.Run("ignores non-user messages", func(t *testing.T) {
		t.Parallel()
		svc := &mockMessageService{}
		_, _ = svc.Create(ctx, "sid", message.CreateMessageParams{
			Role:  message.Assistant,
			Parts: []message.ContentPart{message.TextContent{Text: "[Coach] What would you like help with today?"}},
		})
		require.False(t, isDuplicateCoachPrompt(ctx, svc, "sid", "What would you like help with today?"))
	})

	t.Run("ignores user messages without coach prefix", func(t *testing.T) {
		t.Parallel()
		svc := &mockMessageService{}
		_, _ = svc.Create(ctx, "sid", message.CreateMessageParams{
			Role:  message.User,
			Parts: []message.ContentPart{message.TextContent{Text: "What would you like help with today?"}},
		})
		require.False(t, isDuplicateCoachPrompt(ctx, svc, "sid", "What would you like help with today?"))
	})

	t.Run("list error returns false", func(t *testing.T) {
		t.Parallel()
		svc := &mockMessageService{listErr: errors.New("list failed")}
		require.False(t, isDuplicateCoachPrompt(ctx, svc, "sid", "hello"))
	})
}

func TestFlashIndicators_NilMessages(t *testing.T) {
	t.Parallel()

	primary := &mockAgent{}
	mw := NewMiddleware(primary, ReplacerConfig{Enabled: true})
	// Should not panic with nil message service.
	mw.flashStopIndicator(context.Background(), "sid")
	mw.flashContinueIndicator(context.Background(), "sid")
}

func TestFlashIndicators_CancelledContext(t *testing.T) {
	t.Parallel()

	msgSvc := &mockMessageService{}
	primary := &mockAgent{}
	mw := NewMiddleware(primary, ReplacerConfig{Enabled: true})
	mw.SetMessageService(msgSvc)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Should not create messages when context is cancelled.
	mw.flashStopIndicator(ctx, "sid")
	mw.flashContinueIndicator(ctx, "sid")
	require.Empty(t, msgSvc.messages)
}

// mockLLM simulates a language model that returns a fixed decision.
type mockLLM struct {
	decision string
	err      error
}

func (m *mockLLM) Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &fantasy.Response{
		Content: []fantasy.Content{
			fantasy.TextContent{Text: m.decision},
		},
	}, nil
}

func (m *mockLLM) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	return func(yield func(fantasy.StreamPart) bool) {}, nil
}

func (m *mockLLM) GenerateObject(ctx context.Context, call fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, nil
}

func (m *mockLLM) StreamObject(ctx context.Context, call fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return func(yield func(fantasy.ObjectStreamPart) bool) {}, nil
}

func (m *mockLLM) Provider() string { return "mock" }
func (m *mockLLM) Model() string    { return "mock-model" }

func TestMiddleware_Run_TimeoutTreatsAsStop(t *testing.T) {
	t.Parallel()

	msgSvc := &mockMessageService{}
	primary := &mockAgent{response: "hello there"}
	cfg := ReplacerConfig{Enabled: true, MaxIterations: 2, Timeout: 50 * time.Millisecond}
	mw := NewMiddleware(primary, cfg)
	mw.SetMessageService(msgSvc)

	mw.SetModelResolver(func(ctx context.Context) (fantasy.LanguageModel, error) {
		return &mockSlowLLM{}, nil
	})

	result, err := mw.Run(context.Background(), agent.SessionAgentCall{SessionID: "sid", Prompt: "hi"})
	require.NoError(t, err)
	require.NotNil(t, result)
	// Primary called only once because the timeout was treated as satisfied (stop).
	require.Len(t, primary.calls, 1)
}

// mockSlowLLM simulates a language model that waits for context cancellation.
type mockSlowLLM struct{}

func (m *mockSlowLLM) Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}
func (m *mockSlowLLM) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	return func(yield func(fantasy.StreamPart) bool) {}, nil
}
func (m *mockSlowLLM) GenerateObject(ctx context.Context, call fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, nil
}
func (m *mockSlowLLM) StreamObject(ctx context.Context, call fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return func(yield func(fantasy.ObjectStreamPart) bool) {}, nil
}
func (m *mockSlowLLM) Provider() string { return "mock" }
func (m *mockSlowLLM) Model() string    { return "mock-model" }

func TestMiddleware_SkipCoach_BeforeEval(t *testing.T) {
	t.Parallel()

	msgSvc := &mockMessageService{}
	primary := &mockAgent{response: "hello there"}
	cfg := ReplacerConfig{Enabled: true, MaxIterations: 2}
	mw := NewMiddleware(primary, cfg)
	mw.SetMessageService(msgSvc)

	callCount := 0
	mw.SetModelResolver(func(ctx context.Context) (fantasy.LanguageModel, error) {
		callCount++
		return &mockLLM{decision: `{"action":"stop","prompt":""}`}, nil
	})

	// Skip the coach before Run() starts.
	mw.SkipCoach("sid")

	result, err := mw.Run(context.Background(), agent.SessionAgentCall{SessionID: "sid", Prompt: "hi"})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "hello there", result.Response.Content.Text())
	// Model resolver should never have been called because skip happened before eval.
	require.Zero(t, callCount)
	// Spinner should be cleaned up.
	time.Sleep(50 * time.Millisecond)
	require.Zero(t, msgSvc.MessageCount())
}

func TestMiddleware_SkipCoach_DuringEval(t *testing.T) {
	t.Parallel()

	msgSvc := &mockMessageService{}
	primary := &mockAgent{response: "hello there"}
	cfg := ReplacerConfig{Enabled: true, MaxIterations: 2}
	mw := NewMiddleware(primary, cfg)
	mw.SetMessageService(msgSvc)

	mw.SetModelResolver(func(ctx context.Context) (fantasy.LanguageModel, error) {
		return &mockSlowLLM{}, nil
	})

	// Start Run() in a goroutine and skip halfway through.
	var result *fantasy.AgentResult
	var runErr error
	done := make(chan struct{})
	go func() {
		result, runErr = mw.Run(context.Background(), agent.SessionAgentCall{SessionID: "sid", Prompt: "hi"})
		close(done)
	}()

	// Wait briefly for evaluation to start, then skip.
	time.Sleep(50 * time.Millisecond)
	mw.SkipCoach("sid")

	select {
	case <-done:
		require.NoError(t, runErr)
		require.NotNil(t, result)
		require.Equal(t, "hello there", result.Response.Content.Text())
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not return after SkipCoach")
	}

	// Spinner should be cleaned up.
	time.Sleep(50 * time.Millisecond)
	require.Zero(t, msgSvc.MessageCount())
}

func TestMiddleware_SkipCoach_ResetsFlag(t *testing.T) {
	t.Parallel()

	msgSvc := &mockMessageService{}
	primary := &mockAgent{response: "hello there"}
	cfg := ReplacerConfig{Enabled: true, MaxIterations: 2}
	mw := NewMiddleware(primary, cfg)
	mw.SetMessageService(msgSvc)

	// First run: skip it.
	mw.SkipCoach("sid")
	result, err := mw.Run(context.Background(), agent.SessionAgentCall{SessionID: "sid", Prompt: "hi"})
	require.NoError(t, err)
	require.NotNil(t, result)

	// Second run: should evaluate normally because the flag was reset.
	mw.SetModelResolver(func(ctx context.Context) (fantasy.LanguageModel, error) {
		return &mockLLM{decision: `{"action":"stop","prompt":""}`}, nil
	})
	result, err = mw.Run(context.Background(), agent.SessionAgentCall{SessionID: "sid", Prompt: "hi again"})
	require.NoError(t, err)
	require.NotNil(t, result)
}
