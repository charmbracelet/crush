package cmd

import (
	"github.com/charmbracelet/crush/internal/acp"
	"github.com/charmbracelet/crush/internal/event"
	"github.com/spf13/cobra"
)

var acpCmd = &cobra.Command{
	Use:   "acp",
	Short: "Start Crush as an ACP server",
	Long: `Start Crush in Agent-Client Protocol mode.

This allows external ACP clients (web, desktop, mobile) to drive Crush
over stdio using JSON-RPC. The client sends prompts and receives
streaming updates about agent activity.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := setupApp(cmd)
		if err != nil {
			return err
		}
		defer app.Shutdown()

		if shouldEnableMetrics() {
			event.Init()
		}

		event.AppInitialized()
		defer event.AppExited()

		server := acp.NewServer(cmd.Context())
		defer server.Shutdown()

		agent := acp.NewAgent(app)
		return server.Run(agent)
	},
}
