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

	// ── 2. Run a one-shot non-interactive prompt ───────────────────────────
	// This creates a new session, sends the prompt to the LLM, streams the
	// response to stdout, and shuts down cleanly when finished.
	prompt := "Write a tic tac toe game for desktop in Go in /tmp/hello/"
	fmt.Fprintf(os.Stderr, "Prompt: %s\n", prompt)

	err = app.RunPrompt(ctx, prompt)
	if err != nil {
		log.Fatal(err)
	}
}
