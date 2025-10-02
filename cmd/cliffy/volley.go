package main

import (
	"github.com/bwl/cliffy/internal/config"
	"github.com/spf13/cobra"
)

var volleyCmd = &cobra.Command{
	Use:   "volley [flags] <task1> <task2> <task3> ...",
	Short: "Execute tasks with verbose progress (alias for cliffy --verbose)",
	Long: `Execute tasks with verbose progress output and summary.

This is an alias for 'cliffy --verbose' that shows progress bars, timing,
and a detailed summary.

Examples:
  # Basic execution with progress
  cliffy volley "analyze auth.go" "analyze db.go"

  # Same as above (preferred)
  cliffy --verbose "analyze auth.go" "analyze db.go"

  # Single task with progress
  cliffy volley "what is 2+2?"`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Force verbose mode
		return executeVolley(cmd, args, config.VerbosityVerbose)
	},
}

func init() {
	// Note: Most flags are now handled via the root command
	// Keep volley simple as an alias for --verbose
	rootCmd.AddCommand(volleyCmd)
}
