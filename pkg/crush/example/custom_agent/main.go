package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/charmbracelet/crush/pkg/crush"
)

func main() {
	ctx := context.Background()

	// ── 1. Initialize the application ─────────────────────────────────────
	// Loads configuration from default paths (crush.json), opens the SQLite
	// database, and sets up the full application stack.
	app, err := crush.NewAppWithConfig(ctx, ".", ".crush", true, nil)
	if err != nil {
		log.Fatalf("failed to initialize app: %v", err)
	}
	defer app.Shutdown()

	// ── 2. Run a prompt with a specific agent ID ────────────────────────────
	// The agent configuration must exist in the loaded config (e.g. crush.json)
	// under the given ID. To create an agent programmatically, build a Config
	// in code and use crush.NewApp with a *sql.DB and *ConfigStore.
	prompt := "Review the main.go file in this directory for any issues."
	fmt.Fprintf(os.Stderr, "Prompt: %s\n", prompt)

	err = app.RunPrompt(ctx, prompt, crush.WithAgentID("reviewer"))
	if err != nil {
		log.Fatal(err)
	}
}
