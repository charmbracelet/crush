package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"charm.land/log/v2"
	"github.com/charmbracelet/crush/internal/event"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run as a headless server for plugin hooks (e.g. ACP)",
	Long: `Start Crush in headless server mode without a TUI or initial prompt.

Plugin hooks (such as the ACP server) are started automatically and the process
stays alive waiting for incoming requests. All permission requests are
auto-approved.

This is useful for running Crush as a background ACP agent that other tools
can interact with over HTTP.`,
	Example: `
# Start as an ACP server on the default port
crush serve

# Start with verbose logging
crush serve --verbose

# Start with a specific model
crush serve --model claude-sonnet-4-20250514
  `,
	RunE: func(cmd *cobra.Command, args []string) error {
		verbose, _ := cmd.Flags().GetBool("verbose")
		largeModel, _ := cmd.Flags().GetString("model")
		smallModel, _ := cmd.Flags().GetString("small-model")

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

		if largeModel != "" || smallModel != "" {
			if err := app.OverrideModels(ctx, largeModel, smallModel); err != nil {
				return fmt.Errorf("failed to override models: %w", err)
			}
		}

		event.SetNonInteractive(true)
		event.AppInitialized()

		return app.RunServe(ctx)
	},
}

func init() {
	serveCmd.Flags().BoolP("verbose", "v", false, "Show logs")
	serveCmd.Flags().StringP("model", "m", "", "Model to use")
	serveCmd.Flags().String("small-model", "", "Small model to use")
}
