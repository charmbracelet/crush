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
	"github.com/bwl/cliffy/internal/llm/tools"
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
	ShowStats      bool
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
	InputTokens  int
	OutputTokens int
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
	// Setup logging if not already initialized and debug mode is enabled (opt-in)
	if !log.Initialized() && r.cfg.Options.Debug {
		// Use a default log location in .cliffy directory
		logFile := ".cliffy/logs/cliffy.log"
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

	return nil
}

func (r *Runner) handleEvent(ctx context.Context, event agent.AgentEvent, store *message.Store, sessionID string) error {
	switch event.Type {
	case agent.AgentEventTypeError:
		return event.Error

	case agent.AgentEventTypeToolTrace:
		// Show tool execution in real-time if not quiet
		if !r.options.Quiet && event.ToolMetadata != nil {
			r.showToolTrace(event.ToolMetadata)
		}
		return nil

	case agent.AgentEventTypeProgress:
		// Show progress updates if not quiet
		if !r.options.Quiet && event.Progress != "" {
			fmt.Fprintf(r.stderr, "[PROGRESS] %s\n", event.Progress)
		}
		return nil

	case agent.AgentEventTypeResponse:
		// Track token usage from the event
		r.stats.InputTokens += int(event.TokenUsage.InputTokens)
		r.stats.OutputTokens += int(event.TokenUsage.OutputTokens)
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

		// Tool stats are tracked in showToolTrace via AgentEventTypeToolTrace events
		// No need to count them here to avoid double-counting
	}

	// Ensure final newline
	fmt.Fprintln(r.stdout)

	return nil
}

func (r *Runner) GetStats() ExecutionStats {
	return r.stats
}

func (r *Runner) showToolTrace(metadata *tools.ExecutionMetadata) {
	// Format tool execution trace
	toolName := metadata.ToolName

	// Simple format: [TOOL] name (duration)
	if metadata.Duration > 0 {
		fmt.Fprintf(r.stderr, "[TOOL] %s (%.2fs)\n", toolName, metadata.Duration.Seconds())
	} else {
		fmt.Fprintf(r.stderr, "[TOOL] %s\n", toolName)
	}

	// Track stats
	r.stats.ToolCalls++
	switch toolName {
	case "View", "Glob", "Grep":
		r.stats.FilesRead++
	case "Edit", "Write":
		r.stats.FilesWritten++
	}
}
