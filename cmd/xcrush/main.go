// Package main provides the xcrush build tool for building Crush with extensions.
//
// xcrush creates a custom Crush binary with compile-time extensions by:
//  1. Creating a temporary Go module
//  2. Adding Crush and extensions as dependencies
//  3. Generating main.go importing all extensions
//  4. Building with `go build`
//
// Usage:
//
//	xcrush build --with github.com/example/crush-ext-jira@v1.0.0 --output ./crush-custom
//	xcrush build --with ./my-local-extension
//	xcrush list ./crush-custom
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "xcrush",
		Short: "Build tool for extending Crush with plugins",
		Long: `xcrush is a build tool that creates custom Crush binaries with
compile-time extensions. Extensions register via init() functions,
allowing customization without forking.`,
	}

	rootCmd.AddCommand(buildCmd())
	rootCmd.AddCommand(listCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
