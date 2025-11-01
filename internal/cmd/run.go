package cmd

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run [prompt...]",
	Short: "Run a single non-interactive prompt",
	Long: `Run a single prompt in non-interactive mode and exit.
The prompt can be provided as arguments or piped from stdin.`,
	Example: `
# Run a simple prompt
crush run Explain the use of context in Go

# Pipe input from stdin
echo "What is this code doing?" | crush run

# Run with quiet mode (no spinner)
crush run -q "Generate a README for this project"

# Override provider and model
crush run --provider anthropic --model claude-sonnet-4-0 "Explain this code"

# Override only the model
crush run --model gpt-4 "Write a hello world program"
  `,
	RunE: func(cmd *cobra.Command, args []string) error {
		quiet, _ := cmd.Flags().GetBool("quiet")
		provider, _ := cmd.Flags().GetString("provider")
		model, _ := cmd.Flags().GetString("model")

		// Apply provider/model overrides if specified
		var configModifier func(*config.Config) error
		if provider != "" || model != "" {
			configModifier = func(cfg *config.Config) error {
				return cfg.ApplyRuntimeOverrides(provider, model)
			}
		}

		app, err := setupAppWithConfigModifier(cmd, configModifier)
		if err != nil {
			return err
		}
		defer app.Shutdown()

		if !app.Config().IsConfigured() {
			return fmt.Errorf("no providers configured - please run 'crush' to set up a provider interactively")
		}

		prompt := strings.Join(args, " ")

		prompt, err = MaybePrependStdin(prompt)
		if err != nil {
			slog.Error("Failed to read from stdin", "error", err)
			return err
		}

		if prompt == "" {
			return fmt.Errorf("no prompt provided")
		}

		// Run non-interactive flow using the App method
		return app.RunNonInteractive(cmd.Context(), prompt, quiet)
	},
}

func init() {
	runCmd.Flags().BoolP("quiet", "q", false, "Hide spinner")
	runCmd.Flags().String("provider", "", "Override the LLM provider (e.g., openai, anthropic)")
	runCmd.Flags().String("model", "", "Override the LLM model (e.g., gpt-4, claude-sonnet-4-0)")
}
