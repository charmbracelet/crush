package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/spf13/cobra"
)

var dirsCmd = &cobra.Command{
	Use:   "dirs",
	Short: "Print directories used by Crush",
	Long: `Print the directories where Crush stores its configuration and data files.
This includes the global configuration directory and data directory.`,
	Example: `
# Print all directories
crush dirs

# Print only the config directory
crush dirs --config

# Print only the data directory
crush dirs --data
  `,
	RunE: func(cmd *cobra.Command, args []string) error {
		configOnly, _ := cmd.Flags().GetBool("config")
		dataOnly, _ := cmd.Flags().GetBool("data")

		if configOnly && dataOnly {
			return fmt.Errorf("cannot specify both --config and --data flags")
		}

		configDir := filepath.Dir(config.GlobalConfig())
		dataDir := filepath.Dir(config.GlobalConfigData())

		if configOnly {
			fmt.Println(configDir)
			return nil
		}

		if dataOnly {
			fmt.Println(dataDir)
			return nil
		}

		// Print both by default
		fmt.Printf("Config directory: %s\n", configDir)
		fmt.Printf("Data directory:   %s\n", dataDir)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(dirsCmd)
	dirsCmd.Flags().Bool("config", false, "Print only the config directory")
	dirsCmd.Flags().Bool("data", false, "Print only the data directory")
}
