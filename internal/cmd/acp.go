package cmd

import (
	"github.com/charmbracelet/crush/internal/acp"
	"github.com/charmbracelet/crush/internal/event"
	"github.com/spf13/cobra"
)

var acpCmd = &cobra.Command{
	Use:   "acp",
	Short: "Start the crush in ACP mode",
	Long:  `Allows crush to be connected with ACP compliant clients.`,

	RunE: func(cmd *cobra.Command, args []string) error {
		debug, _ := cmd.Flags().GetBool("debug")
		yolo, _ := cmd.Flags().GetBool("yolo")
		dataDir, _ := cmd.Flags().GetString("data-dir")

		acpServer, err := acp.NewServer(cmd.Context(), debug, yolo, dataDir)
		if err != nil {
			return err
		}
		defer acpServer.Shutdown()

		if shouldEnableMetrics(false) {
			event.Init()
		}

		event.AppInitialized()
		defer event.AppExited()

		if err = acpServer.Run(); err != nil {
			event.Error(err)
			return err
		}

		return nil
	},
}
