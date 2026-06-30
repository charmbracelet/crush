package main

import (
	"context"
	"fmt"
	"log"
	"os"

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
	agent, err := crush.NewAgent("reviewer",
		crush.WithAgentName("Reviewer"),
		crush.WithAgentDescription("Reviews code with read-only access"),
		crush.WithAgentAllowedTools("view", "grep", "glob", "ls"),
	)
	if err != nil {
		log.Fatalf("failed to create agent: %v", err)
	}
	cfg.Agents = map[string]crush.Agent{
		"reviewer": agent,
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

	// ── 3. Run a prompt with the custom agent ─────────────────────────────
	prompt := "Review the main.go file in this directory for any issues."
	fmt.Fprintf(os.Stderr, "Prompt: %s\n", prompt)

	err = app.RunPrompt(ctx, prompt, crush.WithAgentID("reviewer"))
	if err != nil {
		log.Fatal(err)
	}
}
