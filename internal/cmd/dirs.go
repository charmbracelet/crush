package cmd

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/x/exp/charmtone"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
)

var dirsCmd = &cobra.Command{
	Use:   "dirs",
	Short: "Show config and data directories",
	Long: `Show the canonical global configuration directory and the
single project-level configuration directory.`,
	Example: `
# Show all directories
crush dirs
  `,
	Run: func(cmd *cobra.Command, args []string) {
		entries := collectDirs(cmd)
		if term.IsTerminal(os.Stdout.Fd()) {
			printDirs(cmd, entries)
			return
		}
		for _, e := range entries {
			cmd.Println(e)
		}
	},
}

func collectDirs(cmd *cobra.Command) []string {
	cwd, err := ResolveCwd(cmd)
	if err != nil {
		return []string{filepath.Dir(config.CanonicalConfigPath())}
	}

	var dirs []string
	for _, p := range config.ProjectConfigs(cwd) {
		d := filepath.Dir(p)
		if slices.Contains(dirs, d) {
			continue
		}
		dirs = append(dirs, d)
	}

	return dirs
}

func printDirs(cmd *cobra.Command, dirs []string) {
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(charmtone.Charple)

	labels := make([]string, len(dirs))
	longest := 0
	for i := range dirs {
		l := dirLabel(i)
		labels[i] = l + ":"
		if len(labels[i]) > longest {
			longest = len(labels[i])
		}
	}

	for i, d := range dirs {
		lipgloss.Println(labelStyle.Render(labels[i]) +
			strings.Repeat(" ", longest-len(labels[i])) +
			" " + d)
	}

	lipgloss.Println(lipgloss.NewStyle().Foreground(charmtone.Squid).Render("Configs merge from top to bottom"))
}

func dirLabel(i int) string {
	switch i {
	case 0:
		return "Config"
	case 1:
		return "Project"
	default:
		return "Project"
	}
}
