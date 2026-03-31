package cmd

import (
	"context"
	"log/slog"
	"os"
	"os/signal"

	"github.com/charmbracelet/crush/internal/acp"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/spf13/cobra"
)

var acpCmd = &cobra.Command{
	Use:   "acp",
	Short: "Start the Agent Client Protocol (ACP) server over stdio",
	Long: `Start Crush as an ACP agent server.

The ACP server communicates over stdin/stdout using JSON-RPC 2.0,
allowing editors and IDEs (Zed, VS Code, JetBrains, etc.) to use
Crush as a coding agent.

Logs are written to stderr.`,
	Example: `
# Start the ACP server (editors launch this automatically)
crush acp

# Start in a specific working directory
crush acp --cwd /path/to/project
  `,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
		defer cancel()

		appInstance, err := setupApp(cmd)
		if err != nil {
			return err
		}
		defer appInstance.Shutdown()

		if !appInstance.Config().IsConfigured() {
			slog.Error("No providers configured. Run 'crush' to set up a provider interactively.")
			return nil
		}

		// Do not block ACP handshake on MCP startup; editors may time out.
		// Refresh models once MCP initialization finishes so MCP tools are picked up.
		go func() {
			if err := mcp.WaitForInit(ctx); err != nil {
				slog.Warn("ACP: MCP initialization failed", "err", err)
				return
			}
			if err := appInstance.AgentCoordinator.UpdateModels(ctx); err != nil {
				slog.Warn("ACP: failed to refresh models after MCP initialization", "err", err)
			}
		}()

		adapter := acp.NewAppAdapter(
			appInstance.Sessions,
			appInstance.Messages,
			appInstance.AgentCoordinator,
			appInstance.Permissions,
			appInstance.ToolRuntime,
			appInstance.Timeline,
			appInstance.Store(),
		)

		handler := acp.NewHandler(adapter)
		server := acp.NewServer(handler)
		handler.SetServer(server)

		// Bridge permission requests to the ACP client. Without this, any tool
		// that requires user approval would block forever in headless mode
		// because no TUI is present to process the pubsub events.
		go acp.RunPermissionBridge(ctx, appInstance.Permissions, server)

		slog.Info("ACP: server started")
		return server.Serve(ctx)
	},
}

func init() {
	rootCmd.AddCommand(acpCmd)
}
