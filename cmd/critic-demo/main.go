package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/skills/critic"
	"github.com/charmbracelet/crush/internal/skills/replacer"
)

// demoCriticEmitter prints the critic prompt instead of calling an LLM.
func demoCriticEmitter(ctx context.Context, cp critic.Checkpoint) (*critic.CriticFeedback, error) {
	prompt, err := critic.BuildCriticPrompt(cp, "")
	if err != nil {
		return nil, err
	}

	fmt.Println("╔══════════════════════════════════════════════════════════════════════╗")
	if cp.Type == critic.CheckpointEdit {
		fmt.Println("║  CRITIC REVIEW: EDIT CHECKPOINT                                      ║")
	} else {
		fmt.Println("║  CRITIC REVIEW: MESSAGE CHECKPOINT                                   ║")
	}
	fmt.Println("╚══════════════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("─── PROMPT SENT TO CRITIC LLM ───")
	fmt.Println(prompt)
	fmt.Println()

	return &critic.CriticFeedback{
		Verdict:    "approve",
		Confidence: 0.92,
		Summary:    "Demo approval",
		Concerns:   nil,
	}, nil
}

// demoReplacerLLM simulates the replacement agent coach.
type demoReplacerLLM struct {
	responses []string
	idx       int
}

func (d *demoReplacerLLM) Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
	resp := d.responses[d.idx]
	d.idx++
	if d.idx >= len(d.responses) {
		d.idx = len(d.responses) - 1
	}
	return &fantasy.Response{
		Content: []fantasy.Content{fantasy.TextContent{Text: resp}},
	}, nil
}
func (d *demoReplacerLLM) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	return func(yield func(fantasy.StreamPart) bool) {}, nil
}
func (d *demoReplacerLLM) GenerateObject(ctx context.Context, call fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, nil
}
func (d *demoReplacerLLM) StreamObject(ctx context.Context, call fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return func(yield func(fantasy.ObjectStreamPart) bool) {}, nil
}
func (d *demoReplacerLLM) Provider() string { return "demo" }
func (d *demoReplacerLLM) Model() string    { return "demo-model" }

// modifyingMockAgent simulates an agent that edits a file.
type modifyingMockAgent struct {
	path    string
	content string
}

func (m *modifyingMockAgent) Run(ctx context.Context, call agent.SessionAgentCall) (*fantasy.AgentResult, error) {
	_ = os.WriteFile(m.path, []byte(m.content), 0o644)
	return &fantasy.AgentResult{
		Response: fantasy.Response{Content: []fantasy.Content{fantasy.TextContent{Text: "Done! I've updated the file."}}},
	}, nil
}
func (m *modifyingMockAgent) SetModels(large, small agent.Model)          {}
func (m *modifyingMockAgent) SetTools(tools []fantasy.AgentTool)          {}
func (m *modifyingMockAgent) SetSystemPrompt(systemPrompt string)         {}
func (m *modifyingMockAgent) Cancel(sessionID string)                     {}
func (m *modifyingMockAgent) CancelAll()                                  {}
func (m *modifyingMockAgent) IsSessionBusy(sessionID string) bool         { return false }
func (m *modifyingMockAgent) IsBusy() bool                                { return false }
func (m *modifyingMockAgent) QueuedPrompts(sessionID string) int          { return 0 }
func (m *modifyingMockAgent) QueuedPromptsList(sessionID string) []string { return nil }
func (m *modifyingMockAgent) ClearQueue(sessionID string)                 {}
func (m *modifyingMockAgent) Summarize(context.Context, string, fantasy.ProviderOptions) error {
	return nil
}
func (m *modifyingMockAgent) Model() agent.Model { return agent.Model{} }

// chatMockAgent simulates an agent that only returns text.
type chatMockAgent struct {
	responses []string
	idx       int
}

func (c *chatMockAgent) Run(ctx context.Context, call agent.SessionAgentCall) (*fantasy.AgentResult, error) {
	resp := c.responses[c.idx]
	c.idx++
	if c.idx >= len(c.responses) {
		c.idx = len(c.responses) - 1
	}
	return &fantasy.AgentResult{
		Response: fantasy.Response{Content: []fantasy.Content{fantasy.TextContent{Text: resp}}},
	}, nil
}
func (c *chatMockAgent) SetModels(large, small agent.Model)          {}
func (c *chatMockAgent) SetTools(tools []fantasy.AgentTool)          {}
func (c *chatMockAgent) SetSystemPrompt(systemPrompt string)         {}
func (c *chatMockAgent) Cancel(sessionID string)                     {}
func (c *chatMockAgent) CancelAll()                                  {}
func (c *chatMockAgent) IsSessionBusy(sessionID string) bool         { return false }
func (c *chatMockAgent) IsBusy() bool                                { return false }
func (c *chatMockAgent) QueuedPrompts(sessionID string) int          { return 0 }
func (c *chatMockAgent) QueuedPromptsList(sessionID string) []string { return nil }
func (c *chatMockAgent) ClearQueue(sessionID string)                 {}
func (c *chatMockAgent) Summarize(context.Context, string, fantasy.ProviderOptions) error {
	return nil
}
func (c *chatMockAgent) Model() agent.Model { return agent.Model{} }

// fakeFileTracker pretends the agent has read specific files.
type fakeFileTracker struct {
	paths []string
}

func (f *fakeFileTracker) RecordRead(ctx context.Context, sessionID, path string) {}
func (f *fakeFileTracker) LastReadTime(ctx context.Context, sessionID, path string) time.Time {
	return time.Time{}
}
func (f *fakeFileTracker) ListReadFiles(ctx context.Context, sessionID string) ([]string, error) {
	return f.paths, nil
}
func (f *fakeFileTracker) RecordWrite(ctx context.Context, sessionID, path string) {}
func (f *fakeFileTracker) ListWrittenFiles(ctx context.Context, sessionID string) ([]string, error) {
	return nil, nil
}

// fakeMessageService tracks created messages and returns them in List.
type fakeMessageService struct {
	messages []message.Message
}

func (f *fakeMessageService) Create(ctx context.Context, sessionID string, params message.CreateMessageParams) (message.Message, error) {
	msg := message.Message{ID: fmt.Sprintf("msg-%d", len(f.messages)), SessionID: sessionID, Role: params.Role, Parts: params.Parts}
	f.messages = append(f.messages, msg)
	return msg, nil
}
func (f *fakeMessageService) Update(ctx context.Context, msg message.Message) error { return nil }
func (f *fakeMessageService) Get(ctx context.Context, id string) (message.Message, error) {
	return message.Message{}, nil
}
func (f *fakeMessageService) List(ctx context.Context, sessionID string) ([]message.Message, error) {
	base := []message.Message{
		{ID: "u1", SessionID: sessionID, Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "hi"}}},
	}
	return append(base, f.messages...), nil
}
func (f *fakeMessageService) ListUserMessages(ctx context.Context, sessionID string) ([]message.Message, error) {
	return nil, nil
}
func (f *fakeMessageService) ListAllUserMessages(ctx context.Context) ([]message.Message, error) {
	return nil, nil
}
func (f *fakeMessageService) Delete(ctx context.Context, id string) error { return nil }
func (f *fakeMessageService) DeleteSessionMessages(ctx context.Context, sessionID string) error {
	return nil
}
func (f *fakeMessageService) Flush(ctx context.Context, id string) error { return nil }
func (f *fakeMessageService) FlushAll(ctx context.Context) error { return nil }
func (f *fakeMessageService) Subscribe(ctx context.Context) <-chan pubsub.Event[message.Message] {
	return nil
}

func main() {
	dir, err := os.MkdirTemp("", "critic-demo-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "mkdir:", err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)

	editFile := filepath.Join(dir, "hello.go")
	_ = os.WriteFile(editFile, []byte("package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n"), 0o644)

	fmt.Println("╔══════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║     CRUSH SELF-CRITIC + REPLACER DEMO — NO API KEYS NEEDED         ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("Temp directory:", dir)
	fmt.Println()

	// ── DEMO 1: EDIT CHECKPOINT ──
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println("DEMO 1: Agent edits a file → Critic reviews the diff")
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println()

	modAgent := &modifyingMockAgent{path: editFile, content: "package main\n\nfunc main() {\n\tprintln(\"hello world\")\n}\n"}
	criticCfg := critic.CriticSkillConfig{Enabled: true, MaxIterations: 1, AutoApprove: true}
	mw := critic.NewMiddleware(modAgent, criticCfg)
	cs := critic.NewCriticService(criticCfg, nil)
	cs.SetCheckpointEmitter(demoCriticEmitter)
	mw.SetCriticService(cs)
	mw.SetFileTracker(&fakeFileTracker{paths: []string{editFile}})

	_, _ = mw.Run(context.Background(), agent.SessionAgentCall{SessionID: "demo-edit", Prompt: "Add 'world'"})

	b, _ := os.ReadFile(editFile)
	fmt.Println("─── FINAL FILE CONTENT ───")
	fmt.Println(string(b))
	fmt.Println()

	// ── DEMO 2: MESSAGE CHECKPOINT ──
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println("DEMO 2: Agent returns text only → Critic reviews the response")
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println()

	chatAgent := &chatMockAgent{responses: []string{"Go interfaces are satisfied implicitly."}}
	mw2 := critic.NewMiddleware(chatAgent, criticCfg)
	mw2.SetCriticService(cs)
	_, _ = mw2.Run(context.Background(), agent.SessionAgentCall{SessionID: "demo-msg", Prompt: "Explain Go interfaces"})

	// ── DEMO 3: REPLACER (CONVERSATION COACH) ──
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println("DEMO 3: User says 'hi' → Coach keeps conversation going")
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println()

	msgSvc := &fakeMessageService{}
	chatAgent2 := &chatMockAgent{responses: []string{"hi there", "I can help with Go, Python, Rust, and more."}}
	replacerCfg := replacer.ReplacerConfig{Enabled: true, MaxIterations: 2}
	mw3 := replacer.NewMiddleware(chatAgent2, replacerCfg)
	mw3.SetMessageService(msgSvc)
	mw3.SetModelResolver(func(ctx context.Context) (fantasy.LanguageModel, error) {
		return &demoReplacerLLM{responses: []string{
			`{"action":"continue","prompt":"What programming language are you working with?"}`,
			`{"action":"stop","prompt":""}`,
		}}, nil
	})

	result, err := mw3.Run(context.Background(), agent.SessionAgentCall{SessionID: "demo-replacer", Prompt: "hi"})
	if err != nil {
		fmt.Println("Error:", err)
	}
	if result != nil {
		fmt.Println("─── FINAL AGENT RESPONSE ───")
		fmt.Println(result.Response.Content.Text())
	}
	fmt.Println()
	fmt.Println("─── INJECTED COACH MESSAGES ───")
	for _, m := range msgSvc.messages {
		if m.Role == message.User {
			fmt.Println("→", m.Parts[0].(message.TextContent).Text)
		}
	}
	fmt.Println()

	// ── DEMO 4: AUTO-ENABLE ──
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println("DEMO 4: Auto-enable from config")
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Println("  critic: {}   → Critic enabled: true")
	fmt.Println("  replacer: {} → Replacer enabled: true")
	fmt.Println("  Both sections auto-enable when present without explicit 'enabled: false'")
	fmt.Println()

	fmt.Println("Run this anytime:")
	fmt.Println("  go run ./cmd/critic-demo")
	fmt.Println()
}
