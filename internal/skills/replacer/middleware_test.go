package replacer

import (
	"context"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent"
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

// mockAgent simulates a primary agent that returns a fixed response.
type mockAgent struct {
	response string
	calls    []agent.SessionAgentCall
}

func (m *mockAgent) Run(ctx context.Context, call agent.SessionAgentCall) (*fantasy.AgentResult, error) {
	m.calls = append(m.calls, call)
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
func (m *mockAgent) IsSessionBusy(sessionID string) bool         { return false }
func (m *mockAgent) IsBusy() bool                                { return false }
func (m *mockAgent) QueuedPrompts(sessionID string) int          { return 0 }
func (m *mockAgent) QueuedPromptsList(sessionID string) []string { return nil }
func (m *mockAgent) ClearQueue(sessionID string)                 {}
func (m *mockAgent) Summarize(context.Context, string, fantasy.ProviderOptions) error {
	return nil
}
func (m *mockAgent) Model() agent.Model { return agent.Model{} }

// mockMessageService tracks created messages.
type mockMessageService struct {
	messages []message.Message
}

func (m *mockMessageService) Create(ctx context.Context, sessionID string, params message.CreateMessageParams) (message.Message, error) {
	msg := message.Message{ID: "msg-" + string(rune(len(m.messages)+'a')), SessionID: sessionID, Role: params.Role, Parts: params.Parts}
	m.messages = append(m.messages, msg)
	return msg, nil
}
func (m *mockMessageService) Update(ctx context.Context, msg message.Message) error { return nil }
func (m *mockMessageService) Get(ctx context.Context, id string) (message.Message, error) {
	return message.Message{}, nil
}
func (m *mockMessageService) List(ctx context.Context, sessionID string) ([]message.Message, error) {
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
func (m *mockMessageService) Delete(ctx context.Context, id string) error { return nil }
func (m *mockMessageService) DeleteSessionMessages(ctx context.Context, sessionID string) error {
	return nil
}
func (m *mockMessageService) Subscribe(ctx context.Context) <-chan pubsub.Event[message.Message] {
	return nil
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

func TestMiddleware_Run_Disabled(t *testing.T) {
	t.Parallel()

	primary := &mockAgent{response: "hello"}
	cfg := ReplacerConfig{Enabled: false}
	mw := NewMiddleware(primary, cfg)

	result, err := mw.Run(context.Background(), agent.SessionAgentCall{SessionID: "sid", Prompt: "hi"})
	require.NoError(t, err)
	require.Equal(t, "hello", result.Response.Content.Text())
}

// mockLLM simulates a language model that returns a fixed decision.
type mockLLM struct {
	decision string
}

func (m *mockLLM) Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
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
