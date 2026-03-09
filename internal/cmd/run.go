package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"

	"charm.land/log/v2"
	crushapp "github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/event"
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
curl https://charm.land | crush run "Summarize this website"

# Read from a file
crush run "What is this code doing?" <<< prrr.go

# Run in quiet mode (hide the spinner)
crush run --quiet "Generate a README for this project"

# Run in verbose mode
crush run --verbose "Generate a README for this project"

# Run with JSON output (includes session_id for resuming)
crush run --output-format json "Explain this code"

# Continue an existing session
crush run --session <session-id> "Continue from where we left off"

# Run in headless mode (skip all permission prompts)
crush run --dangerously-skip-permissions "Fix the bug in main.go"
  `,
	RunE: func(cmd *cobra.Command, args []string) error {
		quiet, _ := cmd.Flags().GetBool("quiet")
		verbose, _ := cmd.Flags().GetBool("verbose")
		largeModel, _ := cmd.Flags().GetString("model")
		smallModel, _ := cmd.Flags().GetString("small-model")
		sessionID, _ := cmd.Flags().GetString("session")
		outputFormat, _ := cmd.Flags().GetString("output-format")

		// Cancel on SIGINT or SIGTERM.
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
		defer cancel()

		app, err := setupApp(cmd)
		if err != nil {
			return err
		}
		defer app.Shutdown()

		if !app.Config().IsConfigured() {
			return fmt.Errorf("no providers configured - please run 'crush' to set up a provider interactively")
		}

		if verbose {
			slog.SetDefault(slog.New(log.New(os.Stderr)))
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

		event.SetNonInteractive(true)
		event.AppInitialized()

		opts := crushapp.RunOptions{
			Prompt:       prompt,
			LargeModel:   largeModel,
			SmallModel:   smallModel,
			SessionID:    sessionID,
			HideSpinner:  quiet || verbose,
			OutputFormat: crushapp.OutputFormat(outputFormat),
		}

		result, err := app.RunNonInteractive(ctx, os.Stdout, opts)
		if err != nil {
			return err
		}

		// If JSON output format, print the result as JSON
		if opts.OutputFormat == crushapp.OutputFormatJSON {
			output := struct {
				SessionID string `json:"session_id"`
				Result    string `json:"result"`
			}{
				SessionID: result.SessionID,
				Result:    result.Content,
			}
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetEscapeHTML(false)
			return encoder.Encode(output)
		}

		return nil
	},
	PostRun: func(cmd *cobra.Command, args []string) {
		event.AppExited()
	},
}

func init() {
	runCmd.Flags().BoolP("quiet", "q", false, "Hide spinner")
	runCmd.Flags().BoolP("verbose", "v", false, "Show logs")
	runCmd.Flags().StringP("model", "m", "", "Model to use. Accepts 'model' or 'provider/model' to disambiguate models with the same name across providers")
	runCmd.Flags().String("small-model", "", "Small model to use. If not provided, uses the default small model for the provider")
	runCmd.Flags().StringP("session", "s", "", "Session ID to continue an existing conversation")
	runCmd.Flags().StringP("output-format", "o", "text", "Output format: 'text' or 'json'. JSON includes session_id for resuming.")
}
