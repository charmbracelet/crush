package cmd

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/event"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run [prompt...]",
	Short: "Run a single prompt in headless mode",
	Long: `Execute a prompt in headless (non-interactive) mode and exit.

This command is designed for CI pipelines, containers, scripting, and automation.
It never launches the TUI and works without a TTY.

INPUT SOURCES (priority order):
  1. stdin (if piped)     - echo "prompt" | crush run
  2. --input file         - crush run --input prompt.txt
  3. command arguments    - crush run "your prompt here"

OUTPUT FORMATS:
  text        Plain text output (default, streamed)
  json        Structured JSON (buffered, for parsing)
  stream-json NDJSON (real-time events, for pipelines)
  raw         Raw model output only`,

	Example: `  # Basic usage
  crush run "Explain Go contexts"

  # Pipe from stdin
  cat file.go | crush run "Review this code"

  # JSON output for scripting
  crush run --format json "List 5 ideas" | jq '.output'

  # Real-time NDJSON streaming
  crush run --format stream-json "Build a web app" | while read -r line; do
    echo "$line" | jq -r '.type'
  done

  # Use a specific model
  crush run --model claude-sonnet-4 "Analyze this"

  # Limit execution
  crush run --max-turns 5 --timeout 2m "Quick task"

  # Allow specific tools (no prompts)
  crush run --allowed-tools view,ls,grep "Explore the codebase"

  # Custom agent instructions
  crush run --append-system-prompt "Use TypeScript only" "Create a component"

  # Continue previous session
  crush run --continue "What did we discuss?"`,

	RunE:         runHeadless,
	SilenceUsage: true, // Don't print usage on error
	PostRun: func(cmd *cobra.Command, args []string) {
		event.AppExited()
	},
}

func init() {
	// Output control
	runCmd.Flags().StringP("format", "f", "text", "Output format: text, json, stream-json, raw")
	runCmd.Flags().BoolP("quiet", "q", false, "Hide spinner (implied for json/stream-json)")
	runCmd.Flags().BoolP("verbose", "v", false, "Enable debug output to stderr")
	runCmd.Flags().StringP("output", "o", "", "Write output to file (in addition to stdout)")

	// Model & session
	runCmd.Flags().StringP("model", "m", "", "Override the default model")
	runCmd.Flags().StringP("session", "s", "", "Use or create a named session")
	runCmd.Flags().BoolP("continue", "c", false, "Continue the most recent session")

	// Execution limits
	runCmd.Flags().DurationP("timeout", "t", 0, "Maximum execution time (e.g., 5m, 1h)")
	runCmd.Flags().Int("max-turns", 0, "Limit agent turns (0 = unlimited)")

	// Input handling
	runCmd.Flags().StringP("input", "i", "", "Read prompt from file")

	// Agent behavior
	runCmd.Flags().Bool("no-skills", false, "Disable agent skills")
	runCmd.Flags().StringSlice("allowed-tools", nil, "Auto-approve these tools (e.g., view,ls,grep)")
	runCmd.Flags().String("append-system-prompt", "", "Add custom instructions to agent")
	runCmd.Flags().String("system-prompt", "", "Replace agent's system prompt entirely")
	runCmd.Flags().Bool("no-cache", false, "Disable prompt caching")
}

// runHeadless executes Crush in headless mode with comprehensive error handling.
func runHeadless(cmd *cobra.Command, args []string) error {
	// Parse all flags first
	opts, err := parseHeadlessFlags(cmd)
	if err != nil {
		outputError(opts.Format, "", err)
		os.Exit(int(app.GetExitCode(err)))
	}

	// Validate options
	if err := opts.Validate(); err != nil {
		outputError(opts.Format, "", err)
		os.Exit(int(app.ExitInvalidInput))
	}

	// Build prompt from various sources
	prompt, err := buildPrompt(args, opts)
	if err != nil {
		outputError(opts.Format, "", err)
		os.Exit(int(app.GetExitCode(err)))
	}

	// Set up context with signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Apply timeout if specified
	if opts.Timeout > 0 {
		var timeoutCancel context.CancelFunc
		ctx, timeoutCancel = context.WithTimeout(ctx, opts.Timeout)
		defer timeoutCancel()
	}

	// Initialize application
	appInstance, err := setupApp(cmd)
	if err != nil {
		he := app.NewHeadlessError(app.ErrCodeInternal, "failed to initialize application", app.ExitRuntimeError, err)
		outputError(opts.Format, prompt, he)
		os.Exit(int(app.ExitRuntimeError))
	}
	defer appInstance.Shutdown()

	// Check configuration
	if !appInstance.Config().IsConfigured() {
		he := app.NewHeadlessError(app.ErrCodeNoProvider, "no providers configured - run 'crush' interactively to set up", app.ExitConfigError, nil)
		outputError(opts.Format, prompt, he)
		os.Exit(int(app.ExitConfigError))
	}

	// Verbose logging
	if opts.Verbose {
		slog.Info("Headless execution starting",
			"format", opts.Format,
			"model", opts.ModelID,
			"timeout", opts.Timeout,
			"max_turns", opts.MaxTurns,
		)
	}

	// Set non-interactive mode
	event.SetNonInteractive(true)
	event.AppInitialized()

	// Set up output destination
	var output io.Writer = os.Stdout
	if opts.OutputFile != "" {
		file, err := os.Create(opts.OutputFile)
		if err != nil {
			he := app.NewHeadlessError(app.ErrCodeInternal, "failed to create output file", app.ExitRuntimeError, err)
			outputError(opts.Format, prompt, he)
			os.Exit(int(app.ExitRuntimeError))
		}
		defer file.Close()
		output = io.MultiWriter(os.Stdout, file)
	}

	// Execute the agent
	err = appInstance.RunNonInteractiveWithOptions(ctx, output, prompt, opts)
	if err != nil {
		return handleExecutionError(ctx, opts, prompt, err)
	}

	return nil
}

// parseHeadlessFlags extracts and validates all command flags.
func parseHeadlessFlags(cmd *cobra.Command) (app.HeadlessOptions, error) {
	var opts app.HeadlessOptions

	// Parse format
	formatStr, _ := cmd.Flags().GetString("format")
	format, valid := app.ValidateFormat(formatStr)
	if !valid {
		return opts, app.NewHeadlessError(
			app.ErrCodeInvalidFormat,
			fmt.Sprintf("invalid format %q, must be one of: %s", formatStr, strings.Join(app.AllFormats(), ", ")),
			app.ExitInvalidInput,
			nil,
		)
	}
	opts.Format = format

	// Parse other flags
	opts.Quiet, _ = cmd.Flags().GetBool("quiet")
	opts.Verbose, _ = cmd.Flags().GetBool("verbose")
	opts.OutputFile, _ = cmd.Flags().GetString("output")
	opts.InputFile, _ = cmd.Flags().GetString("input")
	opts.ModelID, _ = cmd.Flags().GetString("model")
	opts.SessionID, _ = cmd.Flags().GetString("session")
	opts.ContinueSession, _ = cmd.Flags().GetBool("continue")
	opts.Timeout, _ = cmd.Flags().GetDuration("timeout")
	opts.MaxTurns, _ = cmd.Flags().GetInt("max-turns")
	opts.NoSkills, _ = cmd.Flags().GetBool("no-skills")
	opts.AllowedTools, _ = cmd.Flags().GetStringSlice("allowed-tools")
	opts.AppendSystemPrompt, _ = cmd.Flags().GetString("append-system-prompt")
	opts.SystemPrompt, _ = cmd.Flags().GetString("system-prompt")
	opts.DisableCache, _ = cmd.Flags().GetBool("no-cache")

	return opts, nil
}

// buildPrompt constructs the prompt from various input sources.
// Priority: stdin > --input file > command arguments
func buildPrompt(args []string, opts app.HeadlessOptions) (string, error) {
	var prompt string

	// Check stdin first (if not a TTY)
	if !term.IsTerminal(os.Stdin.Fd()) {
		stdinPrompt, err := MaybePrependStdin(strings.Join(args, " "))
		if err != nil {
			return "", app.NewHeadlessError(app.ErrCodeInvalidInput, "failed to read from stdin", app.ExitInvalidInput, err)
		}
		prompt = stdinPrompt
	} else if opts.InputFile != "" {
		// Read from file
		if _, err := os.Stat(opts.InputFile); os.IsNotExist(err) {
			return "", app.NewHeadlessError(app.ErrCodeFileNotFound, fmt.Sprintf("input file %q does not exist", opts.InputFile), app.ExitInvalidInput, nil)
		}
		content, err := os.ReadFile(opts.InputFile)
		if err != nil {
			return "", app.NewHeadlessError(app.ErrCodeInvalidInput, "failed to read input file", app.ExitInvalidInput, err)
		}
		prompt = string(content)
		// Append args if provided
		if len(args) > 0 {
			prompt = prompt + "\n\n" + strings.Join(args, " ")
		}
	} else {
		// Use args
		prompt = strings.Join(args, " ")
	}

	prompt = strings.TrimSpace(prompt)

	if prompt == "" {
		return "", app.NewHeadlessError(
			app.ErrCodeNoPrompt,
			"no prompt provided",
			app.ExitInvalidInput,
			nil,
		)
	}

	return prompt, nil
}

// handleExecutionError processes execution errors and exits appropriately.
func handleExecutionError(ctx context.Context, opts app.HeadlessOptions, prompt string, err error) error {
	// Check for context errors first
	if ctx.Err() == context.DeadlineExceeded {
		he := app.NewHeadlessError(app.ErrCodeTimeout, fmt.Sprintf("execution timed out after %v", opts.Timeout), app.ExitRuntimeError, nil)
		outputError(opts.Format, prompt, he)
		os.Exit(int(app.ExitRuntimeError))
	}
	if ctx.Err() == context.Canceled {
		slog.Info("Headless execution interrupted")
		if opts.Format.IsJSON() {
			he := app.NewHeadlessError(app.ErrCodeInterrupted, "execution interrupted", app.ExitInterrupted, nil)
			outputError(opts.Format, prompt, he)
		}
		os.Exit(int(app.ExitInterrupted))
	}

	// Output the error
	outputError(opts.Format, prompt, err)
	os.Exit(int(app.GetExitCode(err)))
	return nil
}

// outputError outputs an error in the appropriate format.
func outputError(format app.HeadlessFormat, input string, err error) {
	if format.IsJSON() {
		result := app.NewHeadlessResult().WithError(err)
		result.Input = input
		jsonBytes, _ := result.ToJSON()
		fmt.Println(string(jsonBytes))
	} else {
		// Structured stderr output for text/raw formats
		if he, ok := err.(*app.HeadlessError); ok {
			fmt.Fprintf(os.Stderr, "error [%s]: %s\n", he.Code, he.Message)
			if he.Cause != nil {
				fmt.Fprintf(os.Stderr, "  cause: %v\n", he.Cause)
			}
		} else if ve, ok := err.(*app.ValidationError); ok {
			fmt.Fprintf(os.Stderr, "validation error: %s\n", ve.Error())
		} else {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
	}
}
