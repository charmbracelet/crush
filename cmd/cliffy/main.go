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
	timings        bool
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

		// Print startup banner (unless quiet)
		if !quiet {
			printBanner(cfg, model, fast, smart)
		}

		// Track execution time
		startTime := time.Now()

		// Create runner
		r, err := runner.New(cfg, runner.Options{
			ShowThinking:   showThinking,
			ThinkingFormat: thinkingFormat,
			OutputFormat:   outputFormat,
			Model:          model,
			Quiet:          quiet,
			Timings:        timings,
		})
		if err != nil {
			return err
		}

		// Execute
		prompt := strings.Join(args, " ")
		err = r.Execute(ctx, prompt)

		// Print timing summary if requested
		if timings && !quiet {
			printTimings(startTime)
		}

		return err
	},
}

func init() {
	rootCmd.Flags().BoolVarP(&showThinking, "show-thinking", "t", false, "Show LLM thinking/reasoning")
	rootCmd.Flags().StringVar(&thinkingFormat, "thinking-format", "text", "Format for thinking: text|json")
	rootCmd.Flags().StringVarP(&outputFormat, "output-format", "o", "text", "Output format: text|json")
	rootCmd.Flags().StringVarP(&model, "model", "m", "", "Override model selection")
	rootCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Hide tool logs and banner")
	rootCmd.Flags().BoolVar(&fast, "fast", false, "Use small/fast model")
	rootCmd.Flags().BoolVar(&smart, "smart", false, "Use large/smart model")
	rootCmd.Flags().BoolVar(&timings, "timings", false, "Show performance breakdown")
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
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "Quick setup:\n")
		fmt.Fprintf(os.Stderr, "  1. Get API key: https://openrouter.ai/settings/keys\n")
		fmt.Fprintf(os.Stderr, "  2. Set variable: export CLIFFY_OPENROUTER_API_KEY=\"sk-...\"\n")
		fmt.Fprintf(os.Stderr, "  3. Try again\n")
		fmt.Fprintf(os.Stderr, "\nOr see: https://cliffy.ettio.com/setup\n")
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

	fmt.Fprintf(os.Stderr, "\n%s  Ready to retry\n", tools.AsciiCliffy)
}

func printVersion() {
	fmt.Println(tools.AsciiTennisBall)
	fmt.Printf("\nCliffy v%s\n", version)
	fmt.Println("Fast AI coding assistant")
	fmt.Println("\nhttps://cliffy.ettio.com")
	fmt.Println("Built on Crush • Powered by OpenRouter")
	fmt.Printf("\n%s  Ready to help\n", tools.AsciiCliffy)
}

func printBanner(cfg *config.Config, model string, fast bool, smart bool) {
	// Determine which model will be used
	modelName := "large" // default
	if fast {
		modelName = "small"
	} else if smart {
		modelName = "large"
	} else if model != "" {
		modelName = model
	}

	// Get model info from config
	var modelDisplay string
	if modelCfg, ok := cfg.Models[config.SelectedModelType(modelName)]; ok {
		modelDisplay = modelCfg.Model
	} else {
		modelDisplay = modelName
	}

	fmt.Fprintf(os.Stderr, "%s  Cliffy ready | Model: %s\n", tools.AsciiCliffy, modelDisplay)
}

func printTimings(startTime time.Time) {
	elapsed := time.Since(startTime)
	fmt.Fprintf(os.Stderr, "\n%s  Task complete | Time: %s\n", tools.AsciiCliffy, elapsed.Round(time.Millisecond))
}
