package agent

import (
	"context"
	"testing"
	"time"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/llm/provider"
	"github.com/charmbracelet/crush/internal/llm/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
)

// fakeProvider sends a single EventComplete and never closes the channel.
type fakeProvider struct{}

func (f *fakeProvider) SendMessages(ctx context.Context, messages []message.Message, tools []tools.BaseTool) (*provider.ProviderResponse, error) {
	return &provider.ProviderResponse{Content: "hello", FinishReason: message.FinishReasonEndTurn}, nil
}

func (f *fakeProvider) StreamResponse(ctx context.Context, messages []message.Message, tools []tools.BaseTool) <-chan provider.ProviderEvent {
	ch := make(chan provider.ProviderEvent, 2)
	go func() {
		// Send a content delta then a complete event, then block until context is done (to simulate a channel that isn't closed).
		ch <- provider.ProviderEvent{Type: provider.EventContentDelta, Content: "hello"}
		ch <- provider.ProviderEvent{Type: provider.EventComplete, Response: &provider.ProviderResponse{Content: "hello", FinishReason: message.FinishReasonEndTurn}}
		<-ctx.Done()
	}()
	return ch
}

func (f *fakeProvider) Model() catwalk.Model { return catwalk.Model{} }

// minimal in-memory message service used by the agent in tests
type memMessageService struct {
	pub *pubsub.Broker[message.Message]
}

func (m *memMessageService) Subscribe(ctx context.Context) <-chan pubsub.Event[message.Message] {
	return m.pub.Subscribe(ctx)
}

func (m *memMessageService) Create(ctx context.Context, sessionID string, params message.CreateMessageParams) (message.Message, error) {
	msg := message.Message{ID: "m1", SessionID: sessionID, Role: params.Role, Parts: params.Parts}
	m.pub.Publish(pubsub.CreatedEvent, msg)
	return msg, nil
}

func (m *memMessageService) Update(ctx context.Context, msg message.Message) error {
	m.pub.Publish(pubsub.UpdatedEvent, msg)
	return nil
}

func (m *memMessageService) Get(ctx context.Context, id string) (message.Message, error) {
	return message.Message{}, nil
}

func (m *memMessageService) List(ctx context.Context, sessionID string) ([]message.Message, error) {
	return nil, nil
}
func (m *memMessageService) Delete(ctx context.Context, id string) error { return nil }
func (m *memMessageService) DeleteSessionMessages(ctx context.Context, sessionID string) error {
	return nil
}

// minimal session service
type memSessionService struct{}

func (s *memSessionService) Subscribe(ctx context.Context) <-chan pubsub.Event[session.Session] {
	ch := make(chan pubsub.Event[session.Session])
	close(ch)
	return ch
}

func (s *memSessionService) Create(ctx context.Context, title string) (session.Session, error) {
	return session.Session{ID: "s1"}, nil
}

func (s *memSessionService) CreateTitleSession(ctx context.Context, parentSessionID string) (session.Session, error) {
	return session.Session{ID: "title-" + parentSessionID}, nil
}

func (s *memSessionService) CreateTaskSession(ctx context.Context, toolCallID, parentSessionID, title string) (session.Session, error) {
	return session.Session{ID: toolCallID}, nil
}

func (s *memSessionService) Get(ctx context.Context, id string) (session.Session, error) {
	return session.Session{ID: id}, nil
}
func (s *memSessionService) List(ctx context.Context) ([]session.Session, error) { return nil, nil }
func (s *memSessionService) Save(ctx context.Context, sess session.Session) (session.Session, error) {
	return sess, nil
}
func (s *memSessionService) Delete(ctx context.Context, id string) error { return nil }

func Test_StreamAndHandleEvents_EventComplete_NoClose(t *testing.T) {
	t.Parallel()
	// Minimal config so a.Model() works during TrackUsage
	// Use project config init similar to other provider tests to avoid
	// creating file-backed logs inside a test-managed temp dir on Windows.
	cfg, err := config.Init(".", "", true)
	if err != nil {
		t.Fatalf("failed to init config: %v", err)
	}
	cfg.Providers = csync.NewMap[string, config.ProviderConfig]()
	cfg.Providers.Set("test", config.ProviderConfig{
		ID:     "test",
		Models: []catwalk.Model{{ID: "m1"}},
	})
	cfg.Models = map[config.SelectedModelType]config.SelectedModel{
		config.SelectedModelTypeLarge: {Provider: "test", Model: "m1"},
	}
	// Construct minimal agent with fake provider and in-memory services
	a := &agent{
		Broker:     pubsub.NewBroker[AgentEvent](),
		messages:   &memMessageService{pub: pubsub.NewBroker[message.Message]()},
		sessions:   &memSessionService{},
		provider:   &fakeProvider{},
		providerID: "fake",
		agentCfg:   config.Agent{Model: config.SelectedModelTypeLarge},
	}

	// Provide a lazy tools slice that returns no tools
	a.tools = csync.NewLazySlice(func() []tools.BaseTool { return nil })

	// Call streamAndHandleEvents; fake provider won't close the channel after Complete.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	assistant, _, err := a.streamAndHandleEvents(ctx, "sess1", []message.Message{{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "hi"}}}})
	if err != nil {
		t.Fatalf("streamAndHandleEvents error: %v", err)
	}
	if assistant.Content().Text != "hello" {
		t.Fatalf("expected content 'hello', got %q", assistant.Content().Text)
	}
	if assistant.FinishReason() == "" {
		t.Fatalf("expected finish reason to be set")
	}
}
