package cmd

import (
	"os"

	"github.com/charmbracelet/crush/internal/lsphost"
	"github.com/spf13/cobra"
)

// lspDialCmd is the hidden subcommand Crush runs as a language server when
// a host application, such as an editor running Crush in a pane, shares
// its own language servers. It connects to the host's socket, says hello,
// and copies its stdin and stdout to and from the socket, so the LSP
// client talks to the host's server as if it were a process of its own.
var lspDialCmd = &cobra.Command{
	Use:    lsphost.Command,
	Short:  "Internal: connect an LSP client to a host's language server",
	Hidden: true,
	// The flags are parsed by lsphost.Main, which tests run directly.
	DisableFlagParsing: true,
	Run: func(cmd *cobra.Command, args []string) {
		// The exit code is the only result: errors go to stderr as one
		// plain line, because Crush logs a server's stderr.
		os.Exit(lsphost.Main(cmd.Context(), args, os.Stdin, os.Stdout, os.Stderr))
	},
}

func init() {
	rootCmd.AddCommand(lspDialCmd)
}
