package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/charmbracelet/crush/pkg/crush"
)

func main() {
	ctx := context.Background()

	// ── 1. Initialize the application ─────────────────────────────────────
	app, err := crush.NewAppWithConfig(ctx, ".", ".crush", false, nil)
	if err != nil {
		log.Fatalf("failed to initialize app: %v", err)
	}
	defer app.Shutdown()

	// ── 2. Create a named session up front ─────────────────────────────────
	sess, err := app.Sessions.Create(ctx, "Hello World")
	if err != nil {
		log.Fatalf("failed to create session: %v", err)
	}
	sessionID := sess.ID
	fmt.Printf("Session created: %s\n", sessionID)

	// ── 3. Start a background goroutine to consume live message events ────
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

	// ── 4. Run a prompt in the existing session ────────────────────────────
	var out bytes.Buffer
	prompt := "Write a hello world program in Go"

	err = app.RunPromptInSession(ctx, &out, sessionID, prompt)
	if err != nil {
		log.Fatalf("failed to run prompt: %v", err)
	}
	fmt.Printf("\nFinal response:\n%s\n", out.String())

	// Signal the event consumer to finish.
	evtCancel()

	// ── 5. List all sessions ───────────────────────────────────────────────
	sessions, err := app.Sessions.List(ctx)
	if err != nil {
		log.Fatalf("failed to list sessions: %v", err)
	}
	fmt.Printf("\nAll sessions:\n")
	for _, s := range sessions {
		fmt.Printf("  %s: %s (messages: %d)\n", s.ID, s.Title, s.MessageCount)
	}

	// ── 6. Retrieve messages for the session ───────────────────────────────
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
