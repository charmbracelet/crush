package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/crush/internal/format"
	"github.com/charmbracelet/crush/internal/update"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(updateCmd)
}

var updateCmd = &cobra.Command{
	Use:   "check-update",
	Short: "Check for updates",
	Long:  `Check if a new version of crush is available.`,
	Example: `
# Check for updates
crush check-update
  `,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
		defer cancel()

		sp := format.NewSpinner(ctx, cancel, "Checking for updates")
		sp.Start()
		defer sp.Stop()

		info, err := update.CheckForUpdate(ctx)
		if err != nil {
			return fmt.Errorf("failed to check for updates: %w", err)
		}
		sp.Stop()

		if !info.Available {
			fmt.Printf("You are running the latest version: %s\n", info.CurrentVersion)
			return nil
		}

		fmt.Printf("\n🎉 A new version of crush is available!\n\n")
		fmt.Printf("Current version: %s\n", info.CurrentVersion)
		fmt.Printf("Latest version:  %s\n\n", info.LatestVersion)
		fmt.Printf("Visit %s to download the latest version.\n", info.ReleaseURL)

		return nil
	},
}
