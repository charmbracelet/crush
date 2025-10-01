package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/bwl/cliffy/internal/config"
	"github.com/bwl/cliffy/internal/csync"
	"github.com/bwl/cliffy/internal/llm/agent"
	"github.com/bwl/cliffy/internal/lsp"
	"github.com/bwl/cliffy/internal/message"
	"github.com/bwl/cliffy/internal/volley"
	"github.com/spf13/cobra"
)

var (
	volleyContext          string
	volleyContextFile      string
	volleyMaxConcurrent    int
	volleyMaxRetries       int
	volleyQuiet            bool
	volleyOutputFormat     string
	volleyFailFast         bool
	volleyEstimate         bool
	volleySkipConfirmation bool
)

var volleyCmd = &cobra.Command{
	Use:   "volley [flags] <task1> <task2> <task3> ...",
	Short: "Execute multiple AI tasks in parallel with smart rate limiting",
	Long: `Execute multiple AI tasks in parallel with intelligent concurrency management.

Volley runs multiple tasks efficiently while respecting API rate limits and
managing retries automatically.

Examples:
  # Basic parallel execution
  cliffy volley "analyze auth.go" "analyze db.go" "analyze api.go"

  # With shared context
  cliffy volley --context "$(cat refactoring-plan.md)" \
    "refactor auth.go" \
    "refactor db.go" \
    "refactor api.go"

  # Control concurrency
  cliffy volley --max-concurrent 5 task1 task2 task3

  # Estimate cost before running
  cliffy volley --estimate task1 task2 task3`,
	Args: cobra.MinimumNArgs(1),
	RunE: runVolley,
}

func init() {
	volleyCmd.Flags().StringVar(&volleyContext, "context", "", "Shared context prepended to each task")
	volleyCmd.Flags().StringVar(&volleyContextFile, "context-file", "", "Load shared context from file")
	volleyCmd.Flags().IntVar(&volleyMaxConcurrent, "max-concurrent", 3, "Maximum concurrent tasks")
	volleyCmd.Flags().IntVar(&volleyMaxRetries, "max-retries", 3, "Maximum retry attempts per task")
	volleyCmd.Flags().BoolVar(&volleyQuiet, "quiet", false, "Suppress progress output")
	volleyCmd.Flags().StringVarP(&volleyOutputFormat, "output-format", "o", "text", "Output format (text|json)")
	volleyCmd.Flags().BoolVar(&volleyFailFast, "fail-fast", false, "Stop on first task failure")
	volleyCmd.Flags().BoolVar(&volleyEstimate, "estimate", false, "Show cost estimation before running")
	volleyCmd.Flags().BoolVarP(&volleySkipConfirmation, "yes", "y", false, "Skip confirmation prompts")

	rootCmd.AddCommand(volleyCmd)
}

func runVolley(cmd *cobra.Command, args []string) error {
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
	if volleyContextFile != "" {
		content, err := os.ReadFile(volleyContextFile)
		if err != nil {
			return fmt.Errorf("failed to read context file: %w", err)
		}
		volleyContext = string(content)
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
	opts := volley.VolleyOptions{
		Context:          volleyContext,
		MaxConcurrent:    volleyMaxConcurrent,
		MaxRetries:       volleyMaxRetries,
		ShowProgress:     !volleyQuiet,
		OutputFormat:     volleyOutputFormat,
		FailFast:         volleyFailFast,
		Estimate:         volleyEstimate,
		SkipConfirmation: volleySkipConfirmation,
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

	// Output results
	if err := outputResults(results, summary, opts); err != nil {
		return fmt.Errorf("failed to output results: %w", err)
	}

	// Return error if any tasks failed
	if summary.FailedTasks > 0 {
		return fmt.Errorf("%d/%d tasks failed", summary.FailedTasks, summary.TotalTasks)
	}

	return nil
}

func outputResults(results []volley.TaskResult, summary volley.VolleySummary, opts volley.VolleyOptions) error {
	if opts.OutputFormat == "json" {
		return outputResultsJSON(results, summary)
	}

	return outputResultsText(results, summary)
}

func outputResultsText(results []volley.TaskResult, summary volley.VolleySummary) error {
	// Output each task result
	for _, result := range results {
		fmt.Println(strings.Repeat("═", 63))
		fmt.Printf("Task %d/%d: %s\n", result.Task.Index, len(results), result.Task.Prompt)
		fmt.Println(strings.Repeat("═", 63))
		fmt.Println()

		if result.Status == volley.TaskStatusSuccess {
			fmt.Println(result.Output)
		} else if result.Status == volley.TaskStatusFailed {
			fmt.Printf("Task failed: %v\n", result.Error)
			if result.Retries > 0 {
				fmt.Printf("(After %d retries)\n", result.Retries)
			}
		}

		fmt.Println()
	}

	// Output summary
	fmt.Println(strings.Repeat("═", 63))
	fmt.Println("Volley Summary")
	fmt.Println(strings.Repeat("═", 63))
	fmt.Println()

	fmt.Printf("Completed:  %d/%d tasks\n", summary.SucceededTasks, summary.TotalTasks)
	if summary.FailedTasks > 0 {
		fmt.Printf("Failed:     %d/%d tasks\n", summary.FailedTasks, summary.TotalTasks)
	}
	fmt.Printf("Duration:   %.1fs\n", summary.Duration.Seconds())
	fmt.Printf("Tokens:     %s total (avg %s/task)\n",
		formatNumber(int(summary.TotalTokens)),
		formatNumber(int(summary.AvgTokensPerTask)))
	fmt.Printf("Cost:       $%.4f total\n", summary.TotalCost)

	// Show models used
	models := collectModels(results)
	if len(models) == 1 {
		fmt.Printf("Model:      %s\n", models[0])
	} else if len(models) > 1 {
		fmt.Printf("Models:     %s\n", strings.Join(models, ", "))
	}

	fmt.Printf("Workers:    %d concurrent (max)\n", summary.MaxConcurrentUsed)
	if summary.TotalRetries > 0 {
		fmt.Printf("Retries:    %d total\n", summary.TotalRetries)
	}
	fmt.Println()

	return nil
}

func outputResultsJSON(results []volley.TaskResult, summary volley.VolleySummary) error {
	// Build JSON output
	output := map[string]interface{}{
		"volley_id": fmt.Sprintf("volley-%d", summary.Duration.Nanoseconds()),
		"status":    "completed",
		"summary": map[string]interface{}{
			"total_tasks":         summary.TotalTasks,
			"succeeded":           summary.SucceededTasks,
			"failed":              summary.FailedTasks,
			"duration_sec":        summary.Duration.Seconds(),
			"total_tokens":        summary.TotalTokens,
			"total_cost":          summary.TotalCost,
			"avg_tokens_per_task": summary.AvgTokensPerTask,
			"max_concurrent_used": summary.MaxConcurrentUsed,
			"total_retries":       summary.TotalRetries,
		},
		"tasks": results,
	}

	// TODO: Use proper JSON marshaling
	fmt.Printf("%+v\n", output)

	return nil
}

// collectModels returns unique model IDs used in the volley
func collectModels(results []volley.TaskResult) []string {
	seen := make(map[string]bool)
	var models []string

	for _, result := range results {
		if result.Model != "" && !seen[result.Model] {
			seen[result.Model] = true
			models = append(models, result.Model)
		}
	}

	return models
}
