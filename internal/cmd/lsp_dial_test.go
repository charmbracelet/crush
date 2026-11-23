package cmd

import (
	"testing"

	"github.com/charmbracelet/crush/internal/lsphost"
	"github.com/stretchr/testify/require"
)

// TestLSPDialCommandIsRegistered verifies that the command hosted LSP
// clients run exists under the name they use, hidden, and takes its flags
// itself.
func TestLSPDialCommandIsRegistered(t *testing.T) {
	t.Parallel()

	args := lsphost.Args("/run/joy/lsp.sock", "gopls", "/work/proj", "w1", 1)
	cmd, rest, err := rootCmd.Find(args)
	require.NoError(t, err)
	require.Same(t, lspDialCmd, cmd)
	require.Equal(t, args[1:], rest)
	require.True(t, cmd.Hidden)
	require.True(t, cmd.DisableFlagParsing)
}
