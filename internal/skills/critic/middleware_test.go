package critic

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

func TestMiddleware_Run_ApproveClearsSnapshot(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	p := filepath.Join(tmp, "edit.txt")
	require.NoError(t, os.WriteFile(p, []byte("original"), 0o644))

	modAgent := &modifyingMockAgent{path: p, content: "modified"}
	cfg := CriticSkillConfig{Enabled: true, MaxIterations: 3}
	m := NewMiddleware(modAgent, cfg)

	cs := NewCriticService(cfg, (pubsub.Publisher[any])(nil))
	cs.SetCheckpointEmitter(func(ctx context.Context, cp Checkpoint) (*CriticFeedback, error) {
		return &CriticFeedback{Verdict: "approve", Confidence: 0.9}, nil
	})
	m.SetCriticService(cs)
	m.SetFileTracker(&mockFileTracker{files: []string{p}})

	_, err := m.Run(context.Background(), agent.SessionAgentCall{SessionID: "sid"})
	require.NoError(t, err)

	b, err := os.ReadFile(p)
	require.NoError(t, err)
	require.Equal(t, "modified", string(b))
}

func TestMiddleware_Run_HaltRollsBack(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	p := filepath.Join(tmp, "edit.txt")
	require.NoError(t, os.WriteFile(p, []byte("original"), 0o644))

	modAgent := &modifyingMockAgent{path: p, content: "modified"}
	cfg := CriticSkillConfig{Enabled: true, MaxIterations: 3}
	m := NewMiddleware(modAgent, cfg)

	cs := NewCriticService(cfg, (pubsub.Publisher[any])(nil))
	cs.SetCheckpointEmitter(func(ctx context.Context, cp Checkpoint) (*CriticFeedback, error) {
		return &CriticFeedback{Verdict: "halt", Confidence: 0.2}, nil
	})
	m.SetCriticService(cs)
	m.SetFileTracker(&mockFileTracker{files: []string{p}})

	_, err := m.Run(context.Background(), agent.SessionAgentCall{SessionID: "sid"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "halted")

	b, err := os.ReadFile(p)
	require.NoError(t, err)
	require.Equal(t, "original", string(b))
}

func TestMiddleware_Run_RevisionInjectsMessage(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	p := filepath.Join(tmp, "edit.txt")
	require.NoError(t, os.WriteFile(p, []byte("original"), 0o644))

	modAgent := &modifyingMockAgent{path: p, content: "modified"}
	cfg := CriticSkillConfig{Enabled: true, MaxIterations: 3}
	m := NewMiddleware(modAgent, cfg)

	callCount := 0
	cs := NewCriticService(cfg, (pubsub.Publisher[any])(nil))
	cs.SetCheckpointEmitter(func(ctx context.Context, cp Checkpoint) (*CriticFeedback, error) {
		callCount++
		if callCount == 1 {
			return &CriticFeedback{
				Verdict:    "revise",
				Confidence: 0.9,
				Concerns: []CriticConcern{{
					Severity:   "major",
					Dimension:  "correctness",
					Summary:    "broken",
					Suggestion: "fix it",
				}},
				Summary: "needs work",
			}, nil
		}
		return &CriticFeedback{Verdict: "approve"}, nil
	})
	m.SetCriticService(cs)
	m.SetFileTracker(&mockFileTracker{files: []string{p}})

	msgSvc := &mockMessageService{}
	m.SetMessageService(msgSvc)

	_, err := m.Run(context.Background(), agent.SessionAgentCall{SessionID: "sid"})
	require.NoError(t, err)
	require.Equal(t, 2, callCount)
	require.Len(t, msgSvc.messages, 1)
	require.Equal(t, message.System, msgSvc.messages[0].Role)
	require.Contains(t, msgSvc.messages[0].Parts[0].(message.TextContent).Text, "CRITIC REVIEW")

	// File should be in the approved state (modified), not rolled back.
	b, err := os.ReadFile(p)
	require.NoError(t, err)
	require.Equal(t, "modified", string(b))
}

func TestMiddleware_Run_MaxIterationsExceeded(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	p := filepath.Join(tmp, "edit.txt")
	require.NoError(t, os.WriteFile(p, []byte("original"), 0o644))

	modAgent := &modifyingMockAgent{path: p, content: "modified"}
	cfg := CriticSkillConfig{Enabled: true, MaxIterations: 1}
	m := NewMiddleware(modAgent, cfg)

	cs := NewCriticService(cfg, (pubsub.Publisher[any])(nil))
	cs.SetCheckpointEmitter(func(ctx context.Context, cp Checkpoint) (*CriticFeedback, error) {
		return &CriticFeedback{Verdict: "revise", Confidence: 0.9}, nil
	})
	m.SetCriticService(cs)
	m.SetFileTracker(&mockFileTracker{files: []string{p}})
	m.SetMessageService(&mockMessageService{})

	_, err := m.Run(context.Background(), agent.SessionAgentCall{SessionID: "sid"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "max iterations")
}

func TestMiddleware_Run_SkipRevisionWhenNotAutoApproved(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	p := filepath.Join(tmp, "edit.txt")
	require.NoError(t, os.WriteFile(p, []byte("original"), 0o644))

	modAgent := &modifyingMockAgent{path: p, content: "modified"}
	cfg := CriticSkillConfig{Enabled: true, AutoApprove: false, Threshold: 0.95, MaxIterations: 3}
	m := NewMiddleware(modAgent, cfg)

	cs := NewCriticService(cfg, (pubsub.Publisher[any])(nil))
	cs.SetCheckpointEmitter(func(ctx context.Context, cp Checkpoint) (*CriticFeedback, error) {
		return &CriticFeedback{Verdict: "revise", Confidence: 0.5}, nil
	})
	m.SetCriticService(cs)
	m.SetFileTracker(&mockFileTracker{files: []string{p}})

	_, err := m.Run(context.Background(), agent.SessionAgentCall{SessionID: "sid"})
	require.NoError(t, err) // Skipped revision, returned primary result.
}

func TestMiddleware_Run_MessageReview(t *testing.T) {
	t.Parallel()
	chatAgent := &chatMockAgent{response: "Hello! How can I help you today?"}
	cfg := CriticSkillConfig{Enabled: true, MaxIterations: 3}
	m := NewMiddleware(chatAgent, cfg)

	callCount := 0
	cs := NewCriticService(cfg, (pubsub.Publisher[any])(nil))
	cs.SetCheckpointEmitter(func(ctx context.Context, cp Checkpoint) (*CriticFeedback, error) {
		callCount++
		require.Equal(t, CheckpointMessage, cp.Type)
		require.Contains(t, cp.MessageContent, "Hello!")
		if callCount == 1 {
			return &CriticFeedback{Verdict: "revise", Confidence: 0.9, Summary: "too vague"}, nil
		}
		return &CriticFeedback{Verdict: "approve"}, nil
	})
	m.SetCriticService(cs)
	m.SetFileTracker(&mockFileTracker{files: nil})

	msgSvc := &mockMessageService{}
	m.SetMessageService(msgSvc)

	_, err := m.Run(context.Background(), agent.SessionAgentCall{SessionID: "sid"})
	require.NoError(t, err)
	require.Equal(t, 2, callCount)
}

func TestMiddleware_InterfaceCompliance(t *testing.T) {
	t.Parallel()
	primary := &mockAgent{}
	m := NewMiddleware(primary, CriticSkillConfig{})
	require.NotNil(t, m)

	m.SetModels(agent.Model{}, agent.Model{})
	m.SetTools(nil)
	m.SetSystemPrompt("test")
	m.Cancel("sid")
	m.CancelAll()
	_ = m.IsSessionBusy("sid")
	_ = m.IsBusy()
	_ = m.QueuedPrompts("sid")
	_ = m.QueuedPromptsList("sid")
	m.ClearQueue("sid")
	_ = m.Summarize(context.Background(), "sid", fantasy.ProviderOptions{})
	_ = m.Model()
}

type mockCoachProvider struct {
	summary string
}

func (m *mockCoachProvider) GetCoachSummary(sessionID string) string {
	return m.summary
}

func TestMiddleware_Run_CoachSummaryEnrichment(t *testing.T) {
	t.Parallel()
	chatAgent := &chatMockAgent{response: "Hello!"}
	cfg := CriticSkillConfig{Enabled: true, MaxIterations: 3}
	m := NewMiddleware(chatAgent, cfg)

	cs := NewCriticService(cfg, (pubsub.Publisher[any])(nil))
	var capturedCheckpoint Checkpoint
	cs.SetCheckpointEmitter(func(ctx context.Context, cp Checkpoint) (*CriticFeedback, error) {
		capturedCheckpoint = cp
		return &CriticFeedback{Verdict: "approve", Confidence: 0.9}, nil
	})
	m.SetCriticService(cs)
	m.SetCoachSummaryProvider(&mockCoachProvider{summary: "- Pattern X: fired 3 times."})

	_, err := m.Run(context.Background(), agent.SessionAgentCall{SessionID: "sid"})
	require.NoError(t, err)
	require.Contains(t, capturedCheckpoint.CoachSummary, "Pattern X")
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

	cfg := CriticSkillConfig{Enabled: true, MaxIterations: 3}
	m := NewMiddleware(&chatMockAgent{response: "hello"}, cfg)
	cs := NewCriticService(cfg, (pubsub.Publisher[any])(nil))
	cs.SetCheckpointEmitter(func(ctx context.Context, cp Checkpoint) (*CriticFeedback, error) {
		return &CriticFeedback{Verdict: "approve", Confidence: 0.9}, nil
	})
	m.SetCriticService(cs)

	_, err := m.Run(context.Background(), agent.SessionAgentCall{SessionID: "sid", Prompt: "test"})
	require.NoError(t, err)

	eventsMu.Lock()
	defer eventsMu.Unlock()
	require.Contains(t, events, "critic.verdict")
	require.Contains(t, events, "critic.loop.completed")
}

// modifyingMockAgent mutates a file in Run().
type modifyingMockAgent struct {
	path    string
	content string
}

func (m *modifyingMockAgent) Run(ctx context.Context, call agent.SessionAgentCall) (*fantasy.AgentResult, error) {
	_ = os.WriteFile(m.path, []byte(m.content), 0o644)
	return &fantasy.AgentResult{}, nil
}

func (m *modifyingMockAgent) SetModels(large agent.Model, small agent.Model) {}
func (m *modifyingMockAgent) SetTools(tools []fantasy.AgentTool)             {}
func (m *modifyingMockAgent) SetSystemPrompt(systemPrompt string)            {}
func (m *modifyingMockAgent) Cancel(sessionID string)                        {}
func (m *modifyingMockAgent) CancelAll()                                     {}
func (m *modifyingMockAgent) IsSessionBusy(sessionID string) bool            { return false }
func (m *modifyingMockAgent) IsBusy() bool                                   { return false }
func (m *modifyingMockAgent) QueuedPrompts(sessionID string) int             { return 0 }
func (m *modifyingMockAgent) QueuedPromptsList(sessionID string) []string    { return nil }
func (m *modifyingMockAgent) ClearQueue(sessionID string)                    {}
func (m *modifyingMockAgent) Summarize(ctx context.Context, sessionID string, opts fantasy.ProviderOptions) error {
	return nil
}
func (m *modifyingMockAgent) Model() agent.Model { return agent.Model{} }

// chatMockAgent returns text content without modifying files.
type chatMockAgent struct {
	response string
}

func (m *chatMockAgent) Run(ctx context.Context, call agent.SessionAgentCall) (*fantasy.AgentResult, error) {
	return &fantasy.AgentResult{
		Response: fantasy.Response{
			Content: []fantasy.Content{fantasy.TextContent{Text: m.response}},
		},
	}, nil
}

func (m *chatMockAgent) SetModels(large agent.Model, small agent.Model) {}
func (m *chatMockAgent) SetTools(tools []fantasy.AgentTool)             {}
func (m *chatMockAgent) SetSystemPrompt(systemPrompt string)            {}
func (m *chatMockAgent) Cancel(sessionID string)                        {}
func (m *chatMockAgent) CancelAll()                                     {}
func (m *chatMockAgent) IsSessionBusy(sessionID string) bool            { return false }
func (m *chatMockAgent) IsBusy() bool                                   { return false }
func (m *chatMockAgent) QueuedPrompts(sessionID string) int             { return 0 }
func (m *chatMockAgent) QueuedPromptsList(sessionID string) []string    { return nil }
func (m *chatMockAgent) ClearQueue(sessionID string)                    {}
func (m *chatMockAgent) Summarize(ctx context.Context, sessionID string, opts fantasy.ProviderOptions) error {
	return nil
}
func (m *chatMockAgent) Model() agent.Model { return agent.Model{} }

type mockAgent struct{}

func (m *mockAgent) Run(ctx context.Context, call agent.SessionAgentCall) (*fantasy.AgentResult, error) {
	return nil, nil
}
func (m *mockAgent) SetModels(large agent.Model, small agent.Model) {}
func (m *mockAgent) SetTools(tools []fantasy.AgentTool)             {}
func (m *mockAgent) SetSystemPrompt(systemPrompt string)            {}
func (m *mockAgent) Cancel(sessionID string)                        {}
func (m *mockAgent) CancelAll()                                     {}
func (m *mockAgent) IsSessionBusy(sessionID string) bool            { return false }
func (m *mockAgent) IsBusy() bool                                   { return false }
func (m *mockAgent) QueuedPrompts(sessionID string) int             { return 0 }
func (m *mockAgent) QueuedPromptsList(sessionID string) []string    { return nil }
func (m *mockAgent) ClearQueue(sessionID string)                    {}
func (m *mockAgent) Summarize(ctx context.Context, sessionID string, opts fantasy.ProviderOptions) error {
	return nil
}
func (m *mockAgent) Model() agent.Model { return agent.Model{} }

type mockFileTracker struct {
	files []string
}

func (m *mockFileTracker) RecordRead(ctx context.Context, sessionID, path string) {}
func (m *mockFileTracker) LastReadTime(ctx context.Context, sessionID, path string) time.Time {
	return time.Time{}
}
func (m *mockFileTracker) ListReadFiles(ctx context.Context, sessionID string) ([]string, error) {
	return m.files, nil
}
func (m *mockFileTracker) RecordWrite(ctx context.Context, sessionID, path string) {}
func (m *mockFileTracker) ListWrittenFiles(ctx context.Context, sessionID string) ([]string, error) {
	return nil, nil
}

type mockMessageService struct {
	messages []message.Message
}

func (m *mockMessageService) Create(ctx context.Context, sessionID string, params message.CreateMessageParams) (message.Message, error) {
	msg := message.Message{
		ID:        fmt.Sprintf("msg-%d", len(m.messages)),
		Role:      params.Role,
		Parts:     params.Parts,
		SessionID: sessionID,
	}
	m.messages = append(m.messages, msg)
	return msg, nil
}
func (m *mockMessageService) Update(ctx context.Context, msg message.Message) error { return nil }
func (m *mockMessageService) Get(ctx context.Context, id string) (message.Message, error) {
	return message.Message{}, nil
}
func (m *mockMessageService) List(ctx context.Context, sessionID string) ([]message.Message, error) {
	return m.messages, nil
}
func (m *mockMessageService) ListUserMessages(ctx context.Context, sessionID string) ([]message.Message, error) {
	return nil, nil
}
func (m *mockMessageService) ListAllUserMessages(ctx context.Context) ([]message.Message, error) {
	return nil, nil
}
func (m *mockMessageService) Delete(ctx context.Context, id string) error {
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
	return nil
}
