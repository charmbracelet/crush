package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

func listCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [binary]",
		Short: "List registered extensions in a binary",
		Long: `List the extensions that are compiled into a Crush binary.
This runs the binary with a special flag to enumerate registered plugins.`,
		Example: `  xcrush list ./crush-custom`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(args[0])
		},
	}

	return cmd
}

func runList(binaryPath string) error {
	// Check if binary exists.
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		return fmt.Errorf("binary not found: %s", binaryPath)
	}

	// Run the binary with --list-plugins flag.
	listCmd := exec.Command(binaryPath, "--list-plugins")
	output, err := listCmd.CombinedOutput()
	if err != nil {
		// If the flag doesn't exist, it's a vanilla Crush or old version.
		if strings.Contains(string(output), "unknown flag") {
			fmt.Println("No plugins detected (vanilla Crush or version without plugin support)")
			return nil
		}
		return fmt.Errorf("failed to list plugins: %w\n%s", err, output)
	}

	fmt.Print(string(output))
	return nil
}
