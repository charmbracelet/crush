package cmd

import (
	"fmt"
	"log/slog"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/exp/charmtone"
	"github.com/spf13/cobra"

	"github.com/charmbracelet/crush/internal/config"
)

var updateProvidersCmd = &cobra.Command{
	Use:   "update-providers [path-or-url]",
	Short: "Update providers",
	Long:  `Update the list of providers from a specified local path or remote URL.`,
	Example: `
# Update providers remotely from Catwalk
crush update-providers

# Update providers from a custom URL
crush update-providers https://example.com/

# Update providers from a local file
crush update-providers /path/to/local-providers.json

# Update providers from embedded version
crush update-providers embedded
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// NOTE(@andreynering): We want to skip logging output do stdout here.
		slog.SetDefault(slog.New(slog.DiscardHandler))

		var pathOrUrl string
		if len(args) > 0 {
			pathOrUrl = args[0]
		}

		if err := config.UpdateProviders(pathOrUrl); err != nil {
			return err
		}

		// Initialize config so UpdateCopilotModels can access it
		debug, _ := cmd.Flags().GetBool("debug")
		dataDir, _ := cmd.Flags().GetString("data-dir")
		cwd, err := ResolveCwd(cmd)
		if err != nil {
			return err
		}
		if _, err := config.Init(cwd, dataDir, debug); err != nil {
			slog.Warn("Failed to initialize config for Copilot models update", "err", err)
		} else {
			err := config.UpdateCopilotModels()
			if err != nil {
				slog.Warn("Failed to update Copilot models", "err", err)
			}
		}

		// NOTE(@andreynering): This style is more-or-less copied from Fang's
		// error message, adapted for success.
		headerStyle := lipgloss.NewStyle().
			Foreground(charmtone.Butter).
			Background(charmtone.Guac).
			Bold(true).
			Padding(0, 1).
			Margin(1).
			MarginLeft(2).
			SetString("SUCCESS")
		textStyle := lipgloss.NewStyle().
			MarginLeft(2).
			SetString("Providers updated successfully.")

		fmt.Printf("%s\n%s\n\n", headerStyle.Render(), textStyle.Render())
		return nil
	},
}
