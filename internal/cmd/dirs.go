package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/lipgloss/v2/table"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
)

var dirsCmd = &cobra.Command{
	Use:   "dirs",
	Short: "Print directories used by Crush",
	Long: `Print the directories where Crush stores its configuration and data files.
This includes the global configuration directory and data directory.`,
	Example: `
# Print all global directories
crush dirs

# Print data directory and all project specific config directories
crush dirs -p

# Print only global config directory
crush dirs config

# Print only project specific config directories
crush dirs -p config

# Print only the data directory
crush dirs data
  `,
	RunE: func(cmd *cobra.Command, args []string) error {
		dirs, err := configDirs(cmd)
		if err != nil {
			return fmt.Errorf("cannot collect config directories: %w", err)
		}

		if term.IsTerminal(os.Stdout.Fd()) {
			// We're in a TTY: make it fancy.
			t := table.New().
				Border(lipgloss.RoundedBorder()).
				StyleFunc(func(row, col int) lipgloss.Style {
					return lipgloss.NewStyle().Padding(0, 2)
				}).
				Row("Config", dirs).
				Row("Data", filepath.Dir(config.GlobalConfigData()))
			lipgloss.Println(t)
			return nil
		}
		// Not a TTY.
		cmd.Println(dirs)
		cmd.Println(filepath.Dir(config.GlobalConfigData()))

		return nil
	},
}

var configDirCmd = &cobra.Command{
	Use:   "config",
	Short: "Print the configuration directory used by Crush",
	RunE: func(cmd *cobra.Command, args []string) error {
		dirs, err := configDirs(cmd)
		if err != nil {
			return fmt.Errorf("cannot collect config directories: %w", err)
		}
		cmd.Println(dirs)
		return nil
	},
}

var dataDirCmd = &cobra.Command{
	Use:   "data",
	Short: "Print the datauration directory used by Crush",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Println(filepath.Dir(config.GlobalConfigData()))
	},
}

// configDirs returns formatted string with one or more project config paths
func configDirs(cmd *cobra.Command) (string, error) {
	configDir := filepath.Dir(config.GlobalConfig())
	if ok, _ := cmd.Flags().GetBool("project"); !ok {
		return configDir, nil
	}

	cwd, err := ResolveCwd(cmd)
	if err != nil {
		return "", fmt.Errorf("cannot resolve current working directory: %w", err)
	}

	var sb strings.Builder
	for i, path := range config.ProjectConfigs(cwd) {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(filepath.Dir(path))
	}
	return sb.String(), nil
}

func init() {
	dirsCmd.PersistentFlags().BoolP("project", "p", false, "Print project specific configs")

	dirsCmd.AddCommand(configDirCmd, dataDirCmd)
}
