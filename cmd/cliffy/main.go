package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/joho/godotenv/autoload"

	"github.com/bwl/cliffy/internal/config"
	"github.com/bwl/cliffy/internal/llm/tools"
	"github.com/bwl/cliffy/internal/runner"
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
)

var rootCmd = &cobra.Command{
	Use:   "cliffy [prompt]",
	Short: "Fast AI coding assistant for one-off tasks",
	Long: fmt.Sprintf(`%s  Cliffy - Fast AI coding assistant

Fast, focused AI coding assistant for one-off tasks.
Cliffy zips in, executes your task, and gets back to ready position.

USAGE
  cliffy [flags] "your task"

EXAMPLES
  cliffy "list all Go files in internal/"
  cliffy --fast "count lines of code"
  cliffy --smart "refactor this function for clarity"
  cliffy --show-thinking "explain this algorithm"

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

		// Handle convenience flags (--fast, --smart)
		if fast {
			model = string(config.SelectedModelTypeSmall)
		} else if smart {
			model = string(config.SelectedModelTypeLarge)
		}

		// Skip banner - keep it clean for pipeline use

		// Track execution time
		startTime := time.Now()

		// Create runner
		r, err := runner.New(cfg, runner.Options{
			ShowThinking:   showThinking,
			ThinkingFormat: thinkingFormat,
			OutputFormat:   outputFormat,
			Model:          model,
			Quiet:          quiet,
			ShowStats:      showStats,
		})
		if err != nil {
			return err
		}

		// Execute
		prompt := strings.Join(args, " ")
		err = r.Execute(ctx, prompt)

		// Print stats summary if requested
		if showStats && !quiet {
			printStats(r, startTime)
		}

		return err
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
	rootCmd.Flags().BoolVarP(&showVersion, "version", "v", false, "Show version info")
}

func main() {
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		printError(err)
		os.Exit(1)
	}
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

func printStats(r *runner.Runner, startTime time.Time) {
	elapsed := time.Since(startTime)

	fmt.Fprintf(os.Stderr, "\n---\n")
	// TODO: Add token usage from agent when available
	// For now just show timing
	fmt.Fprintf(os.Stderr, "Completed in %s\n", elapsed.Round(time.Millisecond))
}

func formatNumber(n int) string {
	// Format number with commas
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}

	var result []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, c)
	}
	return string(result)
}
