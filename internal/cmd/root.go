package cmd

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

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

func init() {
	rootCmd.PersistentFlags().StringP("cwd", "c", "", "Current working directory")
	rootCmd.PersistentFlags().StringP("data-dir", "D", "", "Custom crush data directory")
	rootCmd.PersistentFlags().BoolP("debug", "d", false, "Debug")

	rootCmd.Flags().BoolP("help", "h", false, "Help")
	rootCmd.Flags().BoolP("yolo", "y", false, "Automatically accept all permissions (dangerous mode)")

	// Model override (applies to all subcommands)
	rootCmd.PersistentFlags().StringP("model", "m", "", "Override the large model for this run (provider:model or model)")

	// Non-interactive single-prompt flags
	rootCmd.Flags().StringP("prompt", "p", "", "Run a single prompt and exit (non-interactive mode)")
	rootCmd.Flags().BoolP("quiet", "q", false, "Hide spinner when using --prompt")

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(updateProvidersCmd)
}

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

	# Run with debug logging in a specific directory
	crush -d -c /path/to/project

	# Run with custom data directory
	crush -D /path/to/custom/.crush

	# Print version
	crush -v

	# Run a single non-interactive prompt using --prompt flag
	crush --prompt "Create a responsive React calculator component"

	# Run a single non-interactive prompt using -p flag
	crush -p "Explain the use of context in Go"

	# Override model for this run (provider:model or model)
	crush -p "Explain the use of context in Go" -m openai:gpt-4o
	crush -p "Explain the use of context in Go" -m gpt-4o

	# Run with prompt and quiet mode
	crush -p "Generate a README for this project" -q

	# Run in dangerous mode with prompt
	crush -p "Create a simple API server" -y

	# Run a single non-interactive prompt (alternative method)
	crush run "Explain the use of context in Go"

	# Run in dangerous mode (auto-accept all permissions)
	crush -y
  `,
	RunE: func(cmd *cobra.Command, args []string) error {
		// If a prompt is provided via flag, run non-interactive and exit
		prompt, _ := cmd.Flags().GetString("prompt")
		if prompt != "" {
			return handlePromptFlag(cmd, prompt)
		}

		app, err := setupApp(cmd)
		if err != nil {
			return err
		}
		defer app.Shutdown()

		// Set up the TUI.
		program := tea.NewProgram(
			tui.New(app),
			tea.WithAltScreen(),
			tea.WithContext(cmd.Context()),
			tea.WithMouseCellMotion(),            // Use cell motion instead of all motion to reduce event flooding
			tea.WithFilter(tui.MouseEventFilter), // Filter mouse events based on focus state
		)

		go app.Subscribe(program)

		if _, err := program.Run(); err != nil {
			slog.Error("TUI run error", "error", err)
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

// setupApp handles the common setup logic for both interactive and non-interactive modes.
// It returns the app instance, config, cleanup function, and any error.
func setupApp(cmd *cobra.Command) (*app.App, error) {
	debug, _ := cmd.Flags().GetBool("debug")
	yolo, _ := cmd.Flags().GetBool("yolo")
	dataDir, _ := cmd.Flags().GetString("data-dir")
	ctx := cmd.Context()

	cwd, err := ResolveCwd(cmd)
	if err != nil {
		return nil, err
	}

	// Auto-load .env variables in YOLO mode to pick up provider API keys
	if yolo {
		_ = loadDotEnv(cwd)
	}

	cfg, err := config.Init(cwd, dataDir, debug)
	if err != nil {
		return nil, err
	}

	if cfg.Permissions == nil {
		cfg.Permissions = &config.Permissions{}
	}
	cfg.Permissions.SkipRequests = yolo

	// Apply runtime model override if provided
	if modelFlag, _ := cmd.Flags().GetString("model"); modelFlag != "" {
		var providerID, modelID string
		if strings.Contains(modelFlag, ":") {
			parts := strings.SplitN(modelFlag, ":", 2)
			providerID, modelID = parts[0], parts[1]
			if cfg.GetModel(providerID, modelID) == nil {
				return nil, fmt.Errorf("model %s not found in provider %s", modelID, providerID)
			}
		} else {
			found := false
			ambiguous := false
			for p := range cfg.Providers.Seq() {
				if p.Disable {
					continue
				}
				for _, m := range p.Models {
					if m.ID == modelFlag {
						if found {
							ambiguous = true
							break
						}
						providerID = p.ID
						modelID = m.ID
						found = true
					}
				}
				if ambiguous {
					break
				}
			}
			if ambiguous {
				return nil, fmt.Errorf("model %s is available in multiple providers; use provider:model", modelFlag)
			}
			if !found {
				return nil, fmt.Errorf("model %s not found in any enabled provider", modelFlag)
			}
		}
		model := cfg.GetModel(providerID, modelID)
		selected := config.SelectedModel{
			Provider:        providerID,
			Model:           modelID,
			MaxTokens:       model.DefaultMaxTokens,
			ReasoningEffort: model.DefaultReasoningEffort,
		}
		// In-memory override for this run only
		if cfg.Models == nil {
			cfg.Models = make(map[config.SelectedModelType]config.SelectedModel)
		}
		cfg.Models[config.SelectedModelTypeLarge] = selected
	}

	if err := createDotCrushDir(cfg.Options.DataDirectory); err != nil {
		return nil, err
	}

	// Connect to DB; this will also run migrations.
	conn, err := db.Connect(ctx, cfg.Options.DataDirectory)
	if err != nil {
		return nil, err
	}

	appInstance, err := app.New(ctx, conn, cfg)
	if err != nil {
		slog.Error("Failed to create app instance", "error", err)
		return nil, err
	}

	return appInstance, nil
}

// loadDotEnv loads KEY=VALUE pairs from a .env file in the given directory into the process environment.
// Existing environment variables are not overridden.
func loadDotEnv(dir string) error {
	path := filepath.Join(dir, ".env")
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if fi.IsDir() {
		return nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(b), "\n")
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(strings.ToLower(line), "export ") {
			line = strings.TrimSpace(line[7:])
		}
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		val = strings.TrimSuffix(val, "\r")
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
	return nil
}

func MaybePrependStdin(prompt string) (string, error) {
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

func ResolveCwd(cmd *cobra.Command) (string, error) {
	cwd, _ := cmd.Flags().GetString("cwd")
	if cwd != "" {
		err := os.Chdir(cwd)
		if err != nil {
			return "", fmt.Errorf("failed to change directory: %v", err)
		}
		return cwd, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %v", err)
	}
	return cwd, nil
}

func createDotCrushDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failed to create data directory: %q %w", dir, err)
	}

	gitIgnorePath := filepath.Join(dir, ".gitignore")
	if _, err := os.Stat(gitIgnorePath); os.IsNotExist(err) {
		if err := os.WriteFile(gitIgnorePath, []byte("*\n"), 0o644); err != nil {
			return fmt.Errorf("failed to create .gitignore file: %q %w", gitIgnorePath, err)
		}
	}

	return nil
}

// handlePromptFlag processes the --prompt flag for non-interactive execution
func handlePromptFlag(cmd *cobra.Command, prompt string) error {
	quiet, _ := cmd.Flags().GetBool("quiet")

	app, err := setupApp(cmd)
	if err != nil {
		return err
	}
	defer app.Shutdown()

	if !app.Config().IsConfigured() {
		return fmt.Errorf("no providers configured - please run 'crush' to set up a provider interactively")
	}

	// Handle stdin input if available
	finalPrompt, err := MaybePrependStdin(prompt)
	if err != nil {
		slog.Error("Failed to read from stdin", "error", err)
		return err
	}

	if finalPrompt == "" {
		return fmt.Errorf("no prompt provided")
	}

	// Run non-interactive flow using the App method
	return app.RunNonInteractive(cmd.Context(), finalPrompt, quiet)
}
