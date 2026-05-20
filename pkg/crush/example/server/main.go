package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/pkg/crush"
	"charm.land/catwalk/pkg/catwalk"
)

func main() {
	ctx := context.Background()

	// ── 1. Build configuration programmatically ────────────────────────────
	// Instead of loading crush.json from disk, we define the entire config in
	// code. This lets us create a custom agent that is restricted to a subset
	// of tools (Option B).
	cfg := &crush.Config{
		Models: map[crush.SelectedModelType]crush.SelectedModel{
			crush.SelectedModelTypeLarge: {
				Provider: "openai",
				Model:    "gpt-4o",
			},
		},
		Providers: crush.NewMapFrom(map[string]crush.ProviderConfig{
			"openai": {
				ID:     "openai",
				Name:   "OpenAI",
				Type:   catwalk.TypeOpenAI,
				APIKey: os.Getenv("OPENAI_API_KEY"),
				Models: []catwalk.Model{
					{ID: "gpt-4o", Name: "GPT-4o"},
				},
			},
		}),
		Options: &crush.Options{
			DataDirectory: ".crush",
		},
	}

	// Create a custom agent limited to only view, edit, and bash tools.
	agent, err := crush.NewAgent("limited",
		crush.WithAgentName("Limited Server Agent"),
		crush.WithAgentDescription("Only reads, edits, and runs shell commands"),
		crush.WithAgentAllowedTools("view", "edit", "bash"),
	)
	if err != nil {
		log.Fatalf("failed to create agent: %v", err)
	}
	cfg.Agents = map[string]crush.Agent{
		"limited": agent,
	}
	cfg.SetupAgents()

	// ── 2. Open database and initialize the application ───────────────────
	if err := os.MkdirAll(cfg.Options.DataDirectory, 0o755); err != nil {
		log.Fatalf("failed to create data directory: %v", err)
	}
	conn, err := db.Connect(ctx, cfg.Options.DataDirectory)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}

	store := crush.NewConfigStore(cfg)
	app, err := crush.NewApp(ctx, conn, store, nil)
	if err != nil {
		log.Fatalf("failed to initialize app: %v", err)
	}
	defer app.Shutdown()

	// ── 3. Create a named session up front ─────────────────────────────────
	sess, err := app.Sessions.Create(ctx, "Hello World")
	if err != nil {
		log.Fatalf("failed to create session: %v", err)
	}
	sessionID := sess.ID
	fmt.Printf("Session created: %s\n", sessionID)

	// ── 4. Start a background goroutine to consume live message events ────
	var wg sync.WaitGroup
	wg.Add(1)

	evtCtx, evtCancel := context.WithCancel(ctx)
	defer evtCancel()

	go func() {
		defer wg.Done()
		ch := crush.SubscribeSessionMessages(evtCtx, app, sessionID)
		for ev := range ch {
			switch ev.Type {
			case crush.EventCreated:
				fmt.Fprintf(os.Stderr, "[CREATED] %s role=%s\n", ev.Payload.ID, ev.Payload.Role)
			case crush.EventUpdated:
				msg := ev.Payload
				if rc := msg.ReasoningContent(); rc.Thinking != "" {
					fmt.Fprintf(os.Stderr, "[THINKING] %s\n", rc.Thinking)
				}
				if txt := msg.Content(); txt.Text != "" {
					fmt.Fprintf(os.Stderr, "[TEXT] %s\n", txt.Text)
				}
				for _, tc := range msg.ToolCalls() {
					fmt.Fprintf(os.Stderr, "[TOOL_CALL] %s %s\n", tc.Name, tc.ID)
				}
				for _, tr := range msg.ToolResults() {
					fmt.Fprintf(os.Stderr, "[TOOL_RESULT] %s\n", tr.ToolCallID)
				}
				if msg.IsFinished() {
					fmt.Fprintf(os.Stderr, "[FINISHED] reason=%s\n", msg.FinishReason())
				}
			case crush.EventDeleted:
				fmt.Fprintf(os.Stderr, "[DELETED] %s\n", ev.Payload.ID)
			}
		}
	}()

	// ── 5. Run a prompt in the existing session with the limited agent ────
	var out bytes.Buffer
	prompt := "Write a hello world program in Go"

	err = app.RunPromptInSession(ctx, &out, sessionID, prompt, crush.WithAgentID("limited"))
	if err != nil {
		log.Fatalf("failed to run prompt: %v", err)
	}
	fmt.Printf("\nFinal response:\n%s\n", out.String())

	// Signal the event consumer to finish.
	evtCancel()

	// ── 6. List all sessions ───────────────────────────────────────────────
	sessions, err := app.Sessions.List(ctx)
	if err != nil {
		log.Fatalf("failed to list sessions: %v", err)
	}
	fmt.Printf("\nAll sessions:\n")
	for _, s := range sessions {
		fmt.Printf("  %s: %s (messages: %d)\n", s.ID, s.Title, s.MessageCount)
	}

	// ── 7. Retrieve messages for the session ─────────────────────────────
	msgs, err := app.Messages.List(ctx, sessionID)
	if err != nil {
		log.Fatalf("failed to list messages: %v", err)
	}
	fmt.Printf("\nMessages in session %s:\n", sessionID)
	for _, msg := range msgs {
		fmt.Printf("  Role: %s\n", msg.Role)
		for _, part := range msg.Parts {
			switch p := part.(type) {
			case crush.TextContent:
				fmt.Printf("    Text: %s\n", p.Text)
			case crush.ToolCall:
				fmt.Printf("    ToolCall: %s %s\n", p.Name, p.ID)
			case crush.ToolResult:
				fmt.Printf("    ToolResult: %s\n", p.ToolCallID)
			}
		}
	}

	// Wait for the event consumer to finish.
	wg.Wait()
}
