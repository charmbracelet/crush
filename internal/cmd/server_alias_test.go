package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestServerCmdAcceptsServeAlias guards the `serve` alias on `crush server`.
// The README describes the shared backend as "crush serve", and cobra does no
// prefix matching, so without the alias that invocation fails with "unknown
// command" — the documented name and the real one have to agree.
func TestServerCmdAcceptsServeAlias(t *testing.T) {
	t.Parallel()

	require.Equal(t, "server", serverCmd.Name(), "the canonical name stays `server`")
	require.Contains(t, serverCmd.Aliases, "serve", "`crush serve` must resolve to the server command")
}

// TestServeResolvesFromRoot checks the alias through the actual command
// lookup rootCmd performs, rather than only asserting on the Aliases slice.
func TestServeResolvesFromRoot(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"server", "serve"} {
		cmd, _, err := rootCmd.Find([]string{name})
		require.NoError(t, err, "rootCmd should resolve %q", name)
		require.Equal(t, "server", cmd.Name(), "%q should resolve to the server command", name)
	}
}
