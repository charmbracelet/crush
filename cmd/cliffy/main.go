package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	_ "github.com/joho/godotenv/autoload"

	"github.com/bwl/cliffy/internal/config"
	"github.com/bwl/cliffy/internal/csync"
	"github.com/bwl/cliffy/internal/llm/agent"
	"github.com/bwl/cliffy/internal/llm/tools"
	"github.com/bwl/cliffy/internal/lsp"
	"github.com/bwl/cliffy/internal/message"
	"github.com/bwl/cliffy/internal/volley"
	"github.com/spf13/cobra"
)

const version = "0.1.0"

var (
	showThinking   bool
	thinkingFormat string
	outputFormat   string
	model          string
	quiet          bool
	fast           bool
	smart          bool
	showStats      bool
	showVersion    bool
	verbose        bool
	sharedContext  string
	contextFile    string
)

var rootCmd = &cobra.Command{
	Use:   "cliffy [flags] <task> [task2] [task3] ...",
	Short: "Fast AI coding assistant - single or multiple tasks",
	Long: fmt.Sprintf(`%s  Cliffy - Fast AI coding assistant

Fast, focused AI coding assistant for one-off tasks.
Execute one or multiple tasks in parallel. Results only, no noise.

USAGE
  cliffy [flags] <task> [task2] [task3] ...

EXAMPLES
  # Single task - clean output
  cliffy "what is 2+2?"
  # Output: 4

  # Multiple tasks - runs in parallel
  cliffy "analyze auth.go" "analyze db.go" "analyze api.go"

  # Multiple tasks with shared context
  cliffy --context "You are a security expert" \
    "review auth.go" \
    "review db.go" \
    "review api.go"

  # Show progress and stats with --verbose
  cliffy --verbose "task1" "task2" "task3"

  # Model selection
  cliffy --fast "count lines of code"
  cliffy --smart "refactor this function for clarity"

  # Mix flags
  cliffy --verbose --fast "task1" "task2"

Built on Crush • https://cliffy.ettio.com`, tools.AsciiCliffy),
	Args: func(cmd *cobra.Command, args []string) error {
		// Allow no args if version flag is set
		if showVersion {
			return nil
		}
		return cobra.MinimumNArgs(1)(cmd, args)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// Handle version flag
		if showVersion {
			printVersion()
			return nil
		}

		// Route to volley execution (unified path)
		return executeVolley(cmd, args, verbose)
	},
}

func init() {
	rootCmd.Flags().BoolVarP(&showThinking, "show-thinking", "t", false, "Show LLM thinking/reasoning")
	rootCmd.Flags().StringVar(&thinkingFormat, "thinking-format", "text", "Format for thinking: text|json")
	rootCmd.Flags().StringVarP(&outputFormat, "output-format", "o", "text", "Output format: text|json")
	rootCmd.Flags().StringVarP(&model, "model", "m", "", "Override model selection")
	rootCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Hide tool logs")
	rootCmd.Flags().BoolVar(&fast, "fast", false, "Use small/fast model")
	rootCmd.Flags().BoolVar(&smart, "smart", false, "Use large/smart model")
	rootCmd.Flags().BoolVar(&showStats, "stats", false, "Show token usage and timing")
	rootCmd.Flags().BoolVar(&showVersion, "version", false, "Show version info")
	rootCmd.Flags().BoolVar(&verbose, "verbose", false, "Show progress and stats")
	rootCmd.Flags().StringVar(&sharedContext, "context", "", "Shared context prepended to each task")
	rootCmd.Flags().StringVar(&contextFile, "context-file", "", "Load shared context from file")
}

func main() {
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		printError(err)
		os.Exit(1)
	}
}

func executeVolley(cmd *cobra.Command, args []string, verboseMode bool) error {
	ctx := context.Background()

	// Load config
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	cfg, err := config.Init(cwd, ".cliffy", false)
	if err != nil {
		return fmt.Errorf("config load failed: %w", err)
	}

	// Load context from file if specified
	taskContext := sharedContext
	if contextFile != "" {
		content, err := os.ReadFile(contextFile)
		if err != nil {
			return fmt.Errorf("failed to read context file: %w", err)
		}
		taskContext = string(content)
	}

	// Parse tasks from arguments
	tasks := make([]volley.Task, len(args))
	for i, arg := range args {
		tasks[i] = volley.Task{
			Index:  i + 1,
			Prompt: arg,
		}
	}

	// Set up volley options (silent by default, unless verbose)
	opts := volley.DefaultVolleyOptions()
	opts.ShowProgress = verboseMode
	opts.ShowSummary = verboseMode
	opts.OutputFormat = outputFormat
	opts.Context = taskContext

	// Create message store
	messageStore := message.NewStore()

	// Initialize LSP clients
	lspClients := csync.NewMap[string, *lsp.Client]()

	// Get agent configuration
	agentCfg, ok := cfg.Agents["coder"]
	if !ok {
		return fmt.Errorf("coder agent not found in config")
	}

	// Override model if specified via global flags
	if model != "" {
		agentCfg.Model = config.SelectedModelType(model)
	} else if fast {
		agentCfg.Model = config.SelectedModelTypeSmall
	} else if smart {
		agentCfg.Model = config.SelectedModelTypeLarge
	}

	// Create agent
	ag, err := agent.NewAgent(ctx, agentCfg, messageStore, lspClients)
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// Create scheduler
	scheduler := volley.NewScheduler(cfg, ag, messageStore, opts)

	// Execute volley
	results, summary, err := scheduler.Execute(ctx, tasks)
	if err != nil {
		return fmt.Errorf("volley execution failed: %w", err)
	}

	// Output results (silent mode: just results, no decorations)
	if err := outputVolleyResults(results, summary, opts); err != nil {
		return fmt.Errorf("failed to output results: %w", err)
	}

	// Return error if any tasks failed (silent in non-verbose mode)
	if summary.FailedTasks > 0 {
		if !verboseMode {
			// Exit silently - errors already shown to stderr
			os.Exit(1)
		}
		return fmt.Errorf("%d/%d tasks failed", summary.FailedTasks, summary.TotalTasks)
	}

	return nil
}

func outputVolleyResults(results []volley.TaskResult, summary volley.VolleySummary, opts volley.VolleyOptions) error {
	// Output results (silent mode: just results, no decorations)
	for i, result := range results {
		if result.Status == volley.TaskStatusSuccess {
			fmt.Println(result.Output)
			// Add blank line between tasks (but not after the last one)
			if i < len(results)-1 {
				fmt.Println()
			}
		} else if result.Status == volley.TaskStatusFailed {
			// In silent mode, show minimal error
			if !opts.ShowProgress {
				fmt.Fprintf(os.Stderr, "Error: %v\n", result.Error)
			}
			// In verbose mode, progress tracker already showed the error
		}
	}

	return nil
}

func printError(err error) {
	errMsg := err.Error()

	fmt.Fprintf(os.Stderr, "\nError: %v\n", err)

	// Provide helpful recovery hints based on error type
	if strings.Contains(errMsg, "config load failed") || strings.Contains(errMsg, "API key") {
		fmt.Fprintf(os.Stderr, "\nQuick setup:\n")
		fmt.Fprintf(os.Stderr, "  1. Get API key: https://openrouter.ai/settings/keys\n")
		fmt.Fprintf(os.Stderr, "  2. Set variable: export CLIFFY_OPENROUTER_API_KEY=\"sk-...\"\n")
		fmt.Fprintf(os.Stderr, "  3. Try again\n")
		fmt.Fprintf(os.Stderr, "\nSee: https://cliffy.ettio.com/setup\n")
	} else if strings.Contains(errMsg, "model") && strings.Contains(errMsg, "not found") {
		fmt.Fprintf(os.Stderr, "\nCheck ~/.config/cliffy/cliffy.json or use:\n")
		fmt.Fprintf(os.Stderr, "  --fast (small model)\n")
		fmt.Fprintf(os.Stderr, "  --smart (large model)\n")
	} else if strings.Contains(errMsg, "rate limit") || strings.Contains(errMsg, "429") {
		fmt.Fprintf(os.Stderr, "\nRate limited. Try:\n")
		fmt.Fprintf(os.Stderr, "  - Wait a moment and retry\n")
		fmt.Fprintf(os.Stderr, "  - Use --fast for cheaper model\n")
	} else if strings.Contains(errMsg, "context") || strings.Contains(errMsg, "timeout") {
		fmt.Fprintf(os.Stderr, "\nRequest timed out. Try:\n")
		fmt.Fprintf(os.Stderr, "  - Simplify the task\n")
		fmt.Fprintf(os.Stderr, "  - Break into smaller steps\n")
	}

	fmt.Fprintf(os.Stderr, "\n")
}

func printVersion() {
	fmt.Println(tools.AsciiTennisBall)
	fmt.Printf("\nCliffy v%s\n", version)
	fmt.Println("Fast AI coding assistant")
	fmt.Println("\nhttps://cliffy.ettio.com")
	fmt.Println("Built on Crush • Powered by OpenRouter")
	fmt.Printf("\n%s  Ready to help\n", tools.AsciiCliffy)
}

