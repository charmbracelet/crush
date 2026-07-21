package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestChannelsFlagAvailableOnRunCmd guards against the --channels flag being
// registered as a local root flag (rootCmd.Flags) rather than a persistent
// one. When local, `crush run --channels server:webhook` fails with
// "unknown flag" because runCmd does not inherit root's local flags. The
// flag must be persistent so non-interactive and client/server runs can opt
// in to channels too.
func TestChannelsFlagAvailableOnRunCmd(t *testing.T) {
	t.Parallel()

	// The flag must be parseable on runCmd — not just rootCmd. Persistent
	// flags are inherited by subcommands; local flags are not.
	require.True(t, runCmd.Flags().HasFlags(), "runCmd flags should be accessible")

	flag := runCmd.Flags().Lookup("channels")
	require.NotNil(t, flag, "the --channels flag must be available on `crush run` (register it as a persistent flag on rootCmd)")
	require.Equal(t, "stringSlice", flag.Value.Type(), "--channels must be a string slice flag")
}

// TestChannelsFlagAvailableOnRootCmd ensures the flag is still present on the
// root command for interactive mode.
func TestChannelsFlagAvailableOnRootCmd(t *testing.T) {
	t.Parallel()

	flag := rootCmd.Flags().Lookup("channels")
	if flag == nil {
		flag = rootCmd.PersistentFlags().Lookup("channels")
	}
	require.NotNil(t, flag, "the --channels flag must be available on `crush` (rootCmd)")
}
