package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	_ "github.com/joho/godotenv/autoload"

	"github.com/bwl/cliffy/internal/config"
	"github.com/bwl/cliffy/internal/runner"
	"github.com/spf13/cobra"
)

var (
	showThinking   bool
	thinkingFormat string
	outputFormat   string
	model          string
	quiet          bool
	fast           bool
	smart          bool
)

var rootCmd = &cobra.Command{
	Use:   "cliffy [prompt]",
	Short: "Fast, headless AI coding assistant",
	Long: `Cliffy is a headless fork of Crush optimized for one-off AI coding tasks.
It's fast, transparent (shows thinking), and designed for CLI/automation use.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
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

		// Create runner
		r, err := runner.New(cfg, runner.Options{
			ShowThinking:   showThinking,
			ThinkingFormat: thinkingFormat,
			OutputFormat:   outputFormat,
			Model:          model,
			Quiet:          quiet,
		})
		if err != nil {
			return err
		}

		// Execute
		prompt := strings.Join(args, " ")
		return r.Execute(ctx, prompt)
	},
}

func init() {
	rootCmd.Flags().BoolVarP(&showThinking, "show-thinking", "t", false, "Show LLM thinking/reasoning on stderr")
	rootCmd.Flags().StringVar(&thinkingFormat, "thinking-format", "text", "Thinking format: json|text")
	rootCmd.Flags().StringVarP(&outputFormat, "output-format", "o", "text", "Output format: text|json")
	rootCmd.Flags().StringVarP(&model, "model", "m", "", "Override model (e.g., sonnet, haiku)")
	rootCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Minimal output (no tool execution logs)")
	rootCmd.Flags().BoolVar(&fast, "fast", false, "Use fast/cheap model (alias for --model small)")
	rootCmd.Flags().BoolVar(&smart, "smart", false, "Use powerful model (alias for --model large)")
}

func main() {
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
