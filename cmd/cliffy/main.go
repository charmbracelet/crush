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
	"github.com/bwl/cliffy/internal/output"
	"github.com/bwl/cliffy/internal/preset"
	"github.com/bwl/cliffy/internal/runner"
	"github.com/bwl/cliffy/internal/volley"
	"github.com/spf13/cobra"
)

const version = "0.1.0"

var (
	showThinking    bool
	thinkingFormat  string
	outputFormat    string
	model           string
	quiet           bool
	fast            bool
	smart           bool
	showStats       bool
	showVersion     bool
	verbose         bool
	sharedContext   string
	contextFile     string
	emitToolTrace   bool
	presetID        string
	maxConcurrent   int
	tasksFile       string
	tasksJSON       bool
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

  # Multiple tasks - runs in parallel (volley mode)
  cliffy "analyze auth.go" "analyze db.go" "analyze api.go"

  # Multiple tasks with shared context
  cliffy --context "You are a security expert" \
    "review auth.go" \
    "review db.go" \
    "review api.go"

  # Load context from a file (e.g., security guidelines)
  cliffy --context-file security-rules.md \
    "review auth.go" \
    "review payment.go"

  # JSON output for parsing and automation
  cliffy --output-format json "list all TODO comments" | jq .

  # Show progress and stats with --verbose
  cliffy --verbose "task1" "task2" "task3"

  # Tune concurrency for throughput
  cliffy --max-concurrent 10 "task1" "task2" "task3" "task4" "task5"

  # Read tasks from file (line-separated)
  cliffy --tasks-file prompts.txt

  # Read tasks from STDIN
  echo "analyze auth.go" | cliffy -

  # Read JSON task array
  cliffy --json --tasks-file tasks.json
  echo '["task1", "task2"]' | cliffy --json -

  # Model selection
  cliffy --fast "count lines of code"
  cliffy --smart "refactor this function for clarity"

  # Show LLM reasoning (helpful for debugging)
  cliffy --show-thinking "why is this code slow?"

  # Mix flags for custom workflows
  cliffy --verbose --fast --output-format json "task1" "task2"

SHELL COMPLETIONS
  Install shell completions for flag discovery:
    cliffy completion bash > /etc/bash_completion.d/cliffy
    cliffy completion zsh > "${fpath[1]}/_cliffy"
    cliffy completion fish > ~/.config/fish/completions/cliffy.fish
    cliffy completion powershell > cliffy.ps1

Built on Crush • https://cliffy.ettio.com`, tools.AsciiCliffy),
	Args: func(cmd *cobra.Command, args []string) error {
		// Allow no args if version flag is set
		if showVersion {
			return nil
		}
		// Allow no args if using --tasks-file or STDIN (-)
		if tasksFile != "" {
			return nil
		}
		if len(args) == 1 && args[0] == "-" {
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

		// Parse tasks from all possible sources
		tasks, err := parseTasks(args, tasksFile, tasksJSON)
		if err != nil {
			return fmt.Errorf("failed to parse tasks: %w", err)
		}

		// Determine verbosity level
		verbosity := config.VerbosityNormal
		if quiet {
			verbosity = config.VerbosityQuiet
		} else if verbose {
			verbosity = config.VerbosityVerbose
		}

		// Route single tasks through runner for better streaming UX
		// Route multiple tasks through volley for parallel execution
		if len(tasks) == 1 {
			return executeSingleTask(cmd, tasks[0], verbosity)
		}
		return executeVolley(cmd, tasks, verbosity)
	},
}

func init() {
	rootCmd.Flags().BoolVarP(&showThinking, "show-thinking", "t", false, "Show LLM thinking/reasoning")
	rootCmd.Flags().StringVar(&thinkingFormat, "thinking-format", "text", "Format for thinking: text|json")
	rootCmd.Flags().StringVarP(&outputFormat, "output-format", "o", "text", "Output format: text|json")
	rootCmd.Flags().StringVarP(&model, "model", "m", "", "Override model selection")
	rootCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Results only - suppress tool traces and progress")
	rootCmd.Flags().BoolVar(&fast, "fast", false, "Use small/fast model")
	rootCmd.Flags().BoolVar(&smart, "smart", false, "Use large/smart model")
	rootCmd.Flags().BoolVar(&showStats, "stats", false, "Show token usage and timing")
	rootCmd.Flags().BoolVar(&showVersion, "version", false, "Show version info")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed tool traces, thinking, and events")
	rootCmd.Flags().StringVar(&sharedContext, "context", "", "Shared context prepended to each task")
	rootCmd.Flags().StringVar(&contextFile, "context-file", "", "Load shared context from file")
	rootCmd.Flags().BoolVar(&emitToolTrace, "emit-tool-trace", false, "Emit tool execution metadata as NDJSON to stderr for automation")
	rootCmd.Flags().StringVarP(&presetID, "preset", "p", "", "Use a preset configuration (e.g., fast-qa, deep-review, sec-review)")
	rootCmd.Flags().IntVar(&maxConcurrent, "max-concurrent", 0, "Maximum concurrent tasks (default: 3, use 0 for config default)")
	rootCmd.Flags().StringVar(&tasksFile, "tasks-file", "", "Read tasks from file (line-separated or JSON with --json)")
	rootCmd.Flags().BoolVar(&tasksJSON, "json", false, "Parse tasks as JSON array (use with --tasks-file or STDIN)")

	// Add completion commands
	rootCmd.AddCommand(newCompletionCmd())
}

func main() {
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		printError(err)
		os.Exit(1)
	}
}

func executeVolley(cmd *cobra.Command, args []string, verbosity config.VerbosityLevel) error {
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

	// Set up volley options
	opts := volley.DefaultVolleyOptions()
	opts.Verbosity = verbosity
	opts.ShowProgress = verbosity != config.VerbosityQuiet
	opts.ShowSummary = verbosity != config.VerbosityQuiet
	opts.OutputFormat = outputFormat
	opts.Context = taskContext
	opts.ShowThinking = showThinking
	opts.ThinkingFormat = thinkingFormat
	opts.ShowStats = showStats
	opts.EmitToolTrace = emitToolTrace

	// Override max concurrent if specified (0 means use default)
	if maxConcurrent > 0 {
		opts.MaxConcurrent = maxConcurrent
	}

	// Create message store
	messageStore := message.NewStore()

	// Initialize LSP clients
	lspClients := csync.NewMap[string, *lsp.Client]()

	// Get agent configuration
	agentCfg, ok := cfg.Agents["coder"]
	if !ok {
		return fmt.Errorf("coder agent not found in config")
	}

	// Apply preset if specified (before other overrides)
	if err := applyPresetToConfig(cfg, &agentCfg, presetID); err != nil {
		return err
	}

	// Override model if specified via global flags (takes precedence over preset)
	if model != "" {
		agentCfg.Model = config.SelectedModelType(model)
	} else if fast {
		agentCfg.Model = config.SelectedModelTypeSmall
	} else if smart {
		agentCfg.Model = config.SelectedModelTypeLarge
	}

	// Validate that the selected model exists before creating agent
	if err := validateModelExists(cfg, agentCfg.Model); err != nil {
		return err
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

	// Show stats if requested or in verbose mode
	if showStats || verbosity == config.VerbosityVerbose {
		renderStats(results, summary, verbosity)
	}

	// Return error if any tasks failed
	if summary.FailedTasks > 0 {
		if verbosity == config.VerbosityQuiet {
			// In quiet mode, ensure failures are visible with context
			renderFailureSummary(results, summary)
			os.Exit(1)
		}
		return fmt.Errorf("%d/%d tasks failed", summary.FailedTasks, summary.TotalTasks)
	}

	return nil
}

func executeSingleTask(cmd *cobra.Command, prompt string, verbosity config.VerbosityLevel) error {
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

	// Get agent configuration
	agentCfg, ok := cfg.Agents["coder"]
	if !ok {
		return fmt.Errorf("coder agent not found in config")
	}

	// Apply preset if specified (before other overrides)
	if err := applyPresetToConfig(cfg, &agentCfg, presetID); err != nil {
		return err
	}

	// Update the agent in config
	cfg.Agents["coder"] = agentCfg

	// Override model selection (takes precedence over preset)
	if fast {
		agentCfg.Model = config.SelectedModelTypeSmall
	} else if smart {
		agentCfg.Model = config.SelectedModelTypeLarge
	} else if model != "" {
		agentCfg.Model = config.SelectedModelType(model)
	}

	// Validate that the selected model exists before execution
	if err := validateModelExists(cfg, agentCfg.Model); err != nil {
		return err
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

	// Prepend context to prompt if provided
	if taskContext != "" {
		prompt = taskContext + "\n\n" + prompt
	}

	// Create runner options
	opts := runner.Options{
		ShowThinking:   showThinking || verbosity == config.VerbosityVerbose,
		ThinkingFormat: thinkingFormat,
		OutputFormat:   outputFormat,
		Model:          model,
		Quiet:          verbosity == config.VerbosityQuiet,
		ShowStats:      showStats || verbosity == config.VerbosityVerbose,
	}

	// Override model selection (takes precedence over preset)
	if fast {
		opts.Model = "small"
	} else if smart {
		opts.Model = "large"
	}

	// Create runner
	r, err := runner.New(cfg, opts)
	if err != nil {
		return fmt.Errorf("failed to create runner: %w", err)
	}

	// Execute the task
	if err := r.Execute(ctx, prompt); err != nil {
		return err
	}

	// Show stats if requested
	if opts.ShowStats {
		stats := r.GetStats()
		fmt.Fprintf(os.Stderr, "\nStats: %d files read, %d files written, %d tool calls\n",
			stats.FilesRead, stats.FilesWritten, stats.ToolCalls)
		fmt.Fprintf(os.Stderr, "Tokens: %d input, %d output (%d total)\n",
			stats.InputTokens, stats.OutputTokens, stats.InputTokens+stats.OutputTokens)
	}

	return nil
}

func outputVolleyResults(results []volley.TaskResult, summary volley.VolleySummary, opts volley.VolleyOptions) error {
	// Branch on output format
	switch opts.OutputFormat {
	case "json":
		return outputVolleyResultsJSON(results, summary)
	case "diff":
		return outputVolleyResultsDiff(results)
	default: // "text" or empty
		return outputVolleyResultsText(results, summary, opts)
	}
}

// outputVolleyResultsJSON outputs results in JSON format
func outputVolleyResultsJSON(results []volley.TaskResult, summary volley.VolleySummary) error {
	jsonOutput, err := output.FormatJSON(results, summary)
	if err != nil {
		return err
	}
	fmt.Println(jsonOutput)
	return nil
}

// outputVolleyResultsDiff outputs only diffs from tool metadata
func outputVolleyResultsDiff(results []volley.TaskResult) error {
	diffOutput := output.FormatDiffOutput(results)
	fmt.Print(diffOutput)
	return nil
}

// outputVolleyResultsText outputs results in human-readable text format
func outputVolleyResultsText(results []volley.TaskResult, summary volley.VolleySummary, opts volley.VolleyOptions) error {
	// If only one task, no need for separators
	multipleResults := len(results) > 1

	for i, result := range results {
		// Add task header for multiple results
		if multipleResults {
			taskNum := i + 1
			// Show task header with status indicator
			statusIcon := "◍" // success
			if result.Status == volley.TaskStatusFailed {
				statusIcon = "✗" // failed
			}
			fmt.Printf("%s Task %d/%d: %s\n", statusIcon, taskNum, len(results), truncatePrompt(result.Task.Prompt, 60))
		}

		if result.Status == volley.TaskStatusSuccess {
			fmt.Println(result.Output)

			// Show per-task stats in verbose mode
			if opts.Verbosity == config.VerbosityVerbose {
				renderTaskStats(result)
			}
		} else if result.Status == volley.TaskStatusFailed {
			// Show error for failed tasks - always visible, even in quiet mode
			fmt.Fprintf(os.Stderr, "Error: %v\n", result.Error)
		}

		// Add blank line between tasks (but not after the last one)
		if i < len(results)-1 {
			fmt.Println()
		}
	}

	return nil
}

// truncatePrompt shortens a prompt for display
func truncatePrompt(prompt string, maxLen int) string {
	if len(prompt) <= maxLen {
		return prompt
	}
	return prompt[:maxLen-3] + "..."
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


// newCompletionCmd creates the completion command with subcommands for each shell
func newCompletionCmd() *cobra.Command {
	completionCmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for cliffy.

This command outputs shell completion code that must be evaluated by your shell.
It enables tab-completion for all cliffy commands and flags.

INSTALLATION:

  Bash:
    # Linux:
    cliffy completion bash | sudo tee /etc/bash_completion.d/cliffy
    # macOS:
    cliffy completion bash > $(brew --prefix)/etc/bash_completion.d/cliffy

  Zsh:
    cliffy completion zsh > "${fpath[1]}/_cliffy"
    # Then reload your shell or run: compinit

  Fish:
    cliffy completion fish > ~/.config/fish/completions/cliffy.fish

  PowerShell:
    cliffy completion powershell > cliffy.ps1
    # Then add to your PowerShell profile

After installation, restart your shell or source the completion file.`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return rootCmd.GenBashCompletion(os.Stdout)
			case "zsh":
				return rootCmd.GenZshCompletion(os.Stdout)
			case "fish":
				return rootCmd.GenFishCompletion(os.Stdout, true)
			case "powershell":
				return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
			default:
				return fmt.Errorf("unsupported shell: %s", args[0])
			}
		},
	}

	return completionCmd
}

// renderStats displays token usage and timing stats
func renderStats(results []volley.TaskResult, summary volley.VolleySummary, verbosity config.VerbosityLevel) {
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "═══ Statistics ═══\n")
	fmt.Fprintf(os.Stderr, "Tasks: %d total, %d succeeded, %d failed",
		summary.TotalTasks, summary.SucceededTasks, summary.FailedTasks)
	if summary.CanceledTasks > 0 {
		fmt.Fprintf(os.Stderr, ", %d canceled", summary.CanceledTasks)
	}
	fmt.Fprintf(os.Stderr, "\n")

	fmt.Fprintf(os.Stderr, "Duration: %.2fs total", summary.Duration.Seconds())
	if summary.SucceededTasks > 0 {
		avgDuration := summary.Duration.Seconds() / float64(summary.SucceededTasks)
		fmt.Fprintf(os.Stderr, ", %.2fs avg per task", avgDuration)
	}
	fmt.Fprintf(os.Stderr, "\n")

	fmt.Fprintf(os.Stderr, "Tokens: %d total", summary.TotalTokens)
	if summary.SucceededTasks > 0 {
		fmt.Fprintf(os.Stderr, ", %d avg per task", summary.AvgTokensPerTask)
	}
	fmt.Fprintf(os.Stderr, "\n")

	if summary.TotalCost > 0 {
		fmt.Fprintf(os.Stderr, "Cost: $%.4f total", summary.TotalCost)
		if summary.SucceededTasks > 0 {
			avgCost := summary.TotalCost / float64(summary.SucceededTasks)
			fmt.Fprintf(os.Stderr, ", $%.4f avg per task", avgCost)
		}
		fmt.Fprintf(os.Stderr, "\n")
	}

	if summary.TotalRetries > 0 {
		fmt.Fprintf(os.Stderr, "Retries: %d total\n", summary.TotalRetries)
	}

	fmt.Fprintf(os.Stderr, "Concurrency: %d workers used\n", summary.MaxConcurrentUsed)
}

// renderFailureSummary shows failures in quiet mode with context
func renderFailureSummary(results []volley.TaskResult, summary volley.VolleySummary) {
	fmt.Fprintf(os.Stderr, "\n%d/%d tasks failed:\n", summary.FailedTasks, summary.TotalTasks)
	for _, result := range results {
		if result.Status == volley.TaskStatusFailed {
			fmt.Fprintf(os.Stderr, "  - Task %d: %s\n", result.Task.Index, truncatePrompt(result.Task.Prompt, 60))
			fmt.Fprintf(os.Stderr, "    Error: %v\n", result.Error)
		}
	}
}

// renderTaskStats displays per-task statistics in verbose mode
// renderTaskStats displays statistics for a single task (in verbose mode)
func renderTaskStats(result volley.TaskResult) {
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "  [Stats: %s tokens ($%.6f) in %.2fs",
		formatTokenCount(result.TokensTotal),
		result.Cost,
		result.Duration.Seconds())
	if result.Retries > 0 {
		fmt.Fprintf(os.Stderr, ", %d retries", result.Retries)
	}
	if len(result.ToolMetadata) > 0 {
		fmt.Fprintf(os.Stderr, ", %d tools", len(result.ToolMetadata))
	}
	fmt.Fprintf(os.Stderr, "]\n")
}

// formatTokenCount formats token counts with k/M suffixes
func formatTokenCount(tokens int64) string {
	if tokens >= 1000000 {
		return fmt.Sprintf("%.2fM", float64(tokens)/1000000.0)
	}
	if tokens >= 1000 {
		return fmt.Sprintf("%.1fk", float64(tokens)/1000.0)
	}
	return fmt.Sprintf("%d", tokens)
}

// validateModelExists checks that the configured model exists in the provider
func validateModelExists(cfg *config.Config, modelType config.SelectedModelType) error {
	// Get the selected model configuration
	selectedModel, ok := cfg.Models[modelType]
	if !ok {
		return fmt.Errorf("model type '%s' not configured in config (check ~/.config/cliffy/cliffy.json)", modelType)
	}

	// Get the provider configuration
	providerCfg, ok := cfg.Providers.Get(selectedModel.Provider)
	if !ok {
		return fmt.Errorf("provider '%s' not found for model type '%s' (check ~/.config/cliffy/cliffy.json)", selectedModel.Provider, modelType)
	}

	if providerCfg.Disable {
		return fmt.Errorf("provider '%s' is disabled (check ~/.config/cliffy/cliffy.json)", selectedModel.Provider)
	}

	// Check if the model exists in the provider's model list
	modelFound := false
	for _, m := range providerCfg.Models {
		if m.ID == selectedModel.Model {
			modelFound = true
			break
		}
	}

	if !modelFound {
		return fmt.Errorf("model '%s' not found in provider '%s'\nAvailable models: run 'cliffy doctor' to see available models", selectedModel.Model, selectedModel.Provider)
	}

	return nil
}

// applyPresetToConfig applies a preset to the agent configuration and selected model
func applyPresetToConfig(cfg *config.Config, agentCfg *config.Agent, presetID string) error {
	if presetID == "" {
		return nil // No preset specified
	}

	mgr, err := preset.NewManager()
	if err != nil {
		return fmt.Errorf("failed to initialize preset manager: %w", err)
	}

	p, err := mgr.Get(presetID)
	if err != nil {
		return fmt.Errorf("preset not found: %s (use 'cliffy preset list' to see available presets)", presetID)
	}

	// Apply preset to agent configuration
	p.ApplyToAgent(agentCfg)

	// Apply preset to options
	if cfg.Options == nil {
		cfg.Options = &config.Options{}
	}
	p.ApplyToOptions(cfg.Options)

	// Apply preset to selected model
	if model, ok := cfg.Models[p.Model]; ok {
		p.ApplyToSelectedModel(&model)
		cfg.Models[p.Model] = model

		// Validate that the preset's model exists in the provider
		if err := validateModelExists(cfg, p.Model); err != nil {
			return fmt.Errorf("preset '%s' specifies invalid model: %w", presetID, err)
		}
	}

	return nil
}
