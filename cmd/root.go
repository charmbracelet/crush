package cmd

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/tui"
	"github.com/charmbracelet/crush/internal/version"
	"github.com/charmbracelet/fang"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "crush",
	Short: "Terminal-based AI assistant for software development",
	Long: `Crush is a powerful terminal-based AI assistant that helps with software development tasks.
It provides an interactive chat interface with AI capabilities, code analysis, and LSP integration
to assist developers in writing, debugging, and understanding code directly from the terminal.`,
	Example: `
  # Run in interactive mode
  crush

  # Run with debug logging
  crush -d

  # Run with debug slog.in a specific directory
  crush -d -c /path/to/project

  # Print version
  crush -v

  # Run a single non-interactive prompt
  crush -p "Explain the use of context in Go"

  # Run a single non-interactive prompt with JSON output format
  crush -p "Explain the use of context in Go" -f json

  # Run in dangerous mode (auto-accept all permissions)
  crush -y
  `,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load the config
		debug, _ := cmd.Flags().GetBool("debug")
		cwd, _ := cmd.Flags().GetString("cwd")
		prompt, _ := cmd.Flags().GetString("prompt")
		quiet, _ := cmd.Flags().GetBool("quiet")
		yolo, _ := cmd.Flags().GetBool("yolo")

		if cwd != "" {
			err := os.Chdir(cwd)
			if err != nil {
				return fmt.Errorf("failed to change directory: %v", err)
			}
		}
		if cwd == "" {
			c, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current working directory: %v", err)
			}
			cwd = c
		}

		cfg, err := config.Init(cwd, debug)
		if err != nil {
			return err
		}
		cfg.Options.SkipPermissionsRequests = yolo

		ctx := cmd.Context()

		// Connect DB, this will also run migrations
		conn, err := db.Connect(ctx, cfg.Options.DataDirectory)
		if err != nil {
			return err
		}

		app, err := app.New(ctx, conn, cfg)
		if err != nil {
			slog.Error(fmt.Sprintf("Failed to create app instance: %v", err))
			return err
		}
		defer app.Shutdown()

		prompt, err = maybePrependStdin(prompt)
		if err != nil {
			slog.Error(fmt.Sprintf("Failed to read from stdin: %v", err))
			return err
		}

		// Non-interactive mode
		if prompt != "" {
			// Run non-interactive flow using the App method
			return app.RunNonInteractive(ctx, prompt, quiet)
		}

		// Set up the TUI
		program := tea.NewProgram(
			tui.New(app),
			tea.WithAltScreen(),
			tea.WithContext(ctx),
			tea.WithMouseCellMotion(),            // Use cell motion instead of all motion to reduce event flooding
			tea.WithFilter(tui.MouseEventFilter), // Filter mouse events based on focus state
		)

		go app.Subscribe(program)

		if _, err := program.Run(); err != nil {
			slog.Error(fmt.Sprintf("TUI run error: %v", err))
			return fmt.Errorf("TUI error: %v", err)
		}
		return nil
	},
}

func Execute() {
	if err := fang.Execute(
		context.Background(),
		rootCmd,
		fang.WithVersion(version.Version),
		fang.WithNotifySignal(os.Interrupt),
	); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringP("cwd", "c", "", "Current working directory")

	rootCmd.Flags().BoolP("help", "h", false, "Help")
	rootCmd.Flags().BoolP("debug", "d", false, "Debug")
	rootCmd.Flags().StringP("prompt", "p", "", "Prompt to run in non-interactive mode")
	rootCmd.Flags().BoolP("yolo", "y", false, "Automatically accept all permissions (dangerous mode)")

	// Add quiet flag to hide spinner in non-interactive mode
	rootCmd.Flags().BoolP("quiet", "q", false, "Hide spinner in non-interactive mode")
}

func maybePrependStdin(prompt string) (string, error) {
	if term.IsTerminal(os.Stdin.Fd()) {
		return prompt, nil
	}
	fi, err := os.Stdin.Stat()
	if err != nil {
		return prompt, err
	}
	if fi.Mode()&os.ModeNamedPipe == 0 {
		return prompt, nil
	}
	bts, err := io.ReadAll(os.Stdin)
	if err != nil {
		return prompt, err
	}
	return string(bts) + "\n\n" + prompt, nil
}
