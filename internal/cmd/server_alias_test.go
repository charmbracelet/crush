package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestServerCmdAcceptsServeAlias guards the `serve` alias on `crush server`.
//
// `serve` is the spelling people reach for — the README itself said "crush
// serve" until the commit that added this test, which is how the mismatch was
// found. Cobra does no prefix matching and EnablePrefixMatching is off, so
// without an explicit alias that invocation dies with `unknown command
// "serve" for "crush"` rather than starting a server.
//
// Note that fang does not render Aliases in help output, so this test is the
// only place the alias is written down. Don't delete it as unmotivated: the
// motivation is that the alias has no other discoverability, not that some
// doc currently spells it that way.
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
