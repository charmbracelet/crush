package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/themes"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(themeCmd)
}

var themeCmd = &cobra.Command{
	Use:   "theme",
	Short: "Manage CRUSH themes",
	Long: `Manage themes for the CRUSH TUI interface.

Available themes:
  - charmtone (default)
  - nord
  - dracula  
  - monokai

Usage:
  crush theme list           List all available themes
  crush theme set <name>     Set the current theme
  crush theme current        Show the current theme`,
	Example: `crush theme list
crush theme set nord
crush theme current`,
}

var themeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available themes",
	Long:  "List all available themes for the CRUSH TUI interface.",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create a temporary config to get available themes
		cfg := &config.Config{}
		manager := themes.NewManager(cfg)
		availableThemes := manager.ListThemes()

		// Sort themes alphabetically
		sort.Strings(availableThemes)

		fmt.Println("Available themes:")
		for _, theme := range availableThemes {
			fmt.Printf("  %s\n", theme)
		}
		return nil
	},
}

var themeSetCmd = &cobra.Command{
	Use:   "set [theme-name]",
	Short: "Set the current theme",
	Long: `Set the current theme for the CRUSH TUI interface.
The theme change will persist across restarts.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		themeName := strings.ToLower(strings.TrimSpace(args[0]))

		// Setup app to get config
		app, err := setupApp(cmd)
		if err != nil {
			return err
		}
		defer app.Shutdown()

		// Create theme manager
		manager := themes.NewManager(app.Config())

		// Validate theme exists
		if err := manager.ValidateTheme(themeName); err != nil {
			return fmt.Errorf("invalid theme: %w", err)
		}

		// Set the theme
		if err := manager.SetTheme(themeName); err != nil {
			return fmt.Errorf("failed to set theme: %w", err)
		}

		fmt.Printf("Theme changed to %s\n", themeName)
		fmt.Println("The new theme will be applied when you restart CRUSH.")
		return nil
	},
}

var themeCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show the current theme",
	Long:  "Show the currently active theme for the CRUSH TUI interface.",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Setup app to get config
		app, err := setupApp(cmd)
		if err != nil {
			return err
		}
		defer app.Shutdown()

		// Create theme manager
		manager := themes.NewManager(app.Config())
		currentTheme := manager.CurrentName()

		fmt.Printf("Current theme: %s\n", currentTheme)

		// Show theme info if available
		if info, err := manager.GetThemeInfo(currentTheme); err == nil {
			if isDark, ok := info["isDark"].(bool); ok {
				darkText := "light"
				if isDark {
					darkText = "dark"
				}
				fmt.Printf("Type: %s theme\n", darkText)
			}
		}

		return nil
	},
}

func init() {
	themeCmd.AddCommand(themeListCmd)
	themeCmd.AddCommand(themeSetCmd)
	themeCmd.AddCommand(themeCurrentCmd)

	// Add tab completion for theme set command
	themeSetCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		cfg := &config.Config{}
		manager := themes.NewManager(cfg)
		availableThemes := manager.ListThemes()

		var themes []string
		for _, theme := range availableThemes {
			if strings.HasPrefix(theme, toComplete) {
				themes = append(themes, theme)
			}
		}

		return themes, cobra.ShellCompDirectiveNoFileComp
	}
}
