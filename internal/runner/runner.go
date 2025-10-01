package runner

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/bwl/cliffy/internal/config"
	"github.com/bwl/cliffy/internal/csync"
	"github.com/bwl/cliffy/internal/llm/agent"
	"github.com/bwl/cliffy/internal/log"
	"github.com/bwl/cliffy/internal/lsp"
	"github.com/bwl/cliffy/internal/message"
	"github.com/google/uuid"
)

type Options struct {
	ShowThinking   bool
	ThinkingFormat string
	OutputFormat   string
	Model          string
	Quiet          bool
	Timings        bool
}

type Runner struct {
	cfg     *config.Config
	options Options
	stdout  io.Writer
	stderr  io.Writer
	stats   ExecutionStats
}

type ExecutionStats struct {
	FilesRead    int
	FilesWritten int
	ToolCalls    int
	Errors       int
}

func New(cfg *config.Config, opts Options) (*Runner, error) {
	return &Runner{
		cfg:     cfg,
		options: opts,
		stdout:  os.Stdout,
		stderr:  os.Stderr,
	}, nil
}

func (r *Runner) Execute(ctx context.Context, prompt string) error {
	// Setup logging if not already initialized
	if !log.Initialized() {
		// Use a default log location
		logFile := ".crush/cliffy.log"
		log.Setup(logFile, r.cfg.Options.Debug)
	}

	// Initialize LSP clients (lazy initialization)
	lspClients := csync.NewMap[string, *lsp.Client]()

	// Create in-memory message store
	messageStore := message.NewStore()

	// Get agent configuration (default to "coder" agent)
	agentCfg, ok := r.cfg.Agents["coder"]
	if !ok {
		return fmt.Errorf("coder agent not found in config")
	}

	// Override model if specified
	if r.options.Model != "" {
		agentCfg.Model = config.SelectedModelType(r.options.Model)
	}

	// Create agent
	ag, err := agent.NewAgent(ctx, agentCfg, messageStore, lspClients)
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// Generate a session ID for this run
	sessionID := uuid.New().String()

	// Run the agent
	events, err := ag.Run(ctx, sessionID, prompt)
	if err != nil {
		return fmt.Errorf("failed to run agent: %w", err)
	}

	// If events is nil, the request was queued (shouldn't happen in headless mode)
	if events == nil {
		return fmt.Errorf("request was queued unexpectedly")
	}

	// Process events
	for event := range events {
		if err := r.handleEvent(ctx, event, messageStore, sessionID); err != nil {
			return err
		}
	}

	// Print summary if not quiet
	if !r.options.Quiet && r.options.Timings {
		r.printSummary()
	}

	return nil
}

func (r *Runner) handleEvent(ctx context.Context, event agent.AgentEvent, store *message.Store, sessionID string) error {
	switch event.Type {
	case agent.AgentEventTypeError:
		return event.Error

	case agent.AgentEventTypeResponse:
		return r.handleResponse(ctx, event.Message, store, sessionID)

	case agent.AgentEventTypeSummarize:
		// Summarize events not expected in headless mode
		slog.Warn("Unexpected summarize event in headless mode")
		return nil
	}

	return nil
}

func (r *Runner) handleResponse(ctx context.Context, msg message.Message, store *message.Store, sessionID string) error {
	// Get all messages to stream everything
	messages, err := store.List(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to list messages: %w", err)
	}

	// Stream all messages
	for _, m := range messages {
		// Show thinking if requested
		if r.options.ShowThinking {
			reasoning := m.ReasoningContent()
			if reasoning.Thinking != "" {
				if r.options.ThinkingFormat == "json" {
					fmt.Fprintf(r.stderr, `{"type":"thinking","content":%q}`+"\n", reasoning.Thinking)
				} else {
					fmt.Fprintf(r.stderr, "[THINKING]\n%s\n[/THINKING]\n", reasoning.Thinking)
				}
			}
		}

		// Show content
		if m.Role == message.Assistant {
			content := m.Content()
			if content.Text != "" {
				fmt.Fprint(r.stdout, content.Text)
			}
		}

		// Show tool calls if not quiet
		if len(m.ToolCalls()) > 0 {
			r.stats.ToolCalls += len(m.ToolCalls())
			if !r.options.Quiet {
				for _, tc := range m.ToolCalls() {
					fmt.Fprintf(r.stderr, "[TOOL] %s\n", tc.Name)

					// Track specific tool types
					switch tc.Name {
					case "View", "Glob", "Grep":
						r.stats.FilesRead++
					case "Edit", "Write":
						r.stats.FilesWritten++
					}
				}
			}
		}
	}

	// Ensure final newline
	fmt.Fprintln(r.stdout)

	return nil
}

func (r *Runner) printSummary() {
	fmt.Fprintf(r.stderr, "\nStats:\n")
	if r.stats.FilesRead > 0 {
		fmt.Fprintf(r.stderr, "  • %d files read\n", r.stats.FilesRead)
	}
	if r.stats.FilesWritten > 0 {
		fmt.Fprintf(r.stderr, "  • %d files updated\n", r.stats.FilesWritten)
	}
	if r.stats.ToolCalls > 0 {
		fmt.Fprintf(r.stderr, "  • %d tool calls\n", r.stats.ToolCalls)
	}
	if r.stats.Errors > 0 {
		fmt.Fprintf(r.stderr, "  • %d errors\n", r.stats.Errors)
	}
}
