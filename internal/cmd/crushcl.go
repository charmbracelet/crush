package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var crushclCmd = &cobra.Command{
	Use:     "crushcl",
	Aliases: []string{"cc"},
	Short:   "Run crushcl in Claude Code compatible mode",
	Long: `Run crushcl with settings optimized for Claude Code compatibility.
This mode enables specific behaviors and configurations for users
familiar with Claude Code's interface and workflows.`,
	Example: `
# Run in Claude Code compatible mode
crushcl crushcl

# Short alias
crushcl cc
  `,
	RunE: func(cmd *cobra.Command, args []string) error {
		os.Setenv("CRUSH_COMPAT_MODE", "claudec")
		return rootCmd.RunE(cmd, args)
	},
}
