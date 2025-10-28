package cmd

import (
	"fmt"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/spf13/cobra"
)

var themeCmd = &cobra.Command{
	Use:   "theme",
	Short: "Manage TUI themes",
	Long:  `Manage the visual theme for the Crush TUI interface.`,
}

var themeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available themes",
	Long:  `List all available themes for the Crush TUI interface.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Available themes:")
		fmt.Println("  charmtone       - Dark theme (default)")
		fmt.Println("  charmtone-light - Light theme")
		return nil
	},
}

var themeSetCmd = &cobra.Command{
	Use:   "set [theme-name]",
	Short: "Set the current theme",
	Long:  `Set the theme for the Crush TUI interface. Available themes: charmtone, charmtone-light`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		themeName := args[0]

		// Validate theme name
		validThemes := map[string]bool{
			"charmtone":       true,
			"charmtone-light": true,
		}

		if !validThemes[themeName] {
			return fmt.Errorf("invalid theme '%s'. Available themes: charmtone, charmtone-light", themeName)
		}

		// Load current config
		cwd, err := ResolveCwd(cmd)
		if err != nil {
			return err
		}

		dataDir, _ := cmd.Flags().GetString("data-dir")
		debug, _ := cmd.Flags().GetBool("debug")

		cfg, err := config.Init(cwd, dataDir, debug)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Set the theme
		if err := cfg.SetTheme(themeName); err != nil {
			return fmt.Errorf("failed to set theme: %w", err)
		}

		fmt.Printf("Theme set to '%s'\n", themeName)
		return nil
	},
}

var themeCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show the current theme",
	Long:  `Display the currently configured theme for the Crush TUI interface.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load current config
		cwd, err := ResolveCwd(cmd)
		if err != nil {
			return err
		}

		dataDir, _ := cmd.Flags().GetString("data-dir")
		debug, _ := cmd.Flags().GetBool("debug")

		cfg, err := config.Init(cwd, dataDir, debug)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		currentTheme := cfg.GetTheme()
		fmt.Printf("Current theme: %s\n", currentTheme)
		return nil
	},
}

func init() {
	themeCmd.AddCommand(themeListCmd)
	themeCmd.AddCommand(themeSetCmd)
	themeCmd.AddCommand(themeCurrentCmd)
	rootCmd.AddCommand(themeCmd)
}
