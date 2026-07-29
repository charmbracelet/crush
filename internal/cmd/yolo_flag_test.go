package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestYoloFlagAvailableOnRunCmd guards against the --yolo flag being
// registered as a local root flag (rootCmd.Flags) rather than a persistent
// one. When local, `crush run --yolo "..."` fails with "unknown flag"
// because runCmd does not inherit root's local flags. The flag must be
// persistent so non-interactive and client/server runs can opt into
// auto-accepting permissions.
func TestYoloFlagAvailableOnRunCmd(t *testing.T) {
	t.Parallel()

	// The flag must be parseable on runCmd — not just rootCmd. Persistent
	// flags are inherited by subcommands; local flags are not.
	flag := runCmd.Flags().Lookup("yolo")
	require.NotNil(t, flag, "the --yolo flag must be available on `crush run` (register it as a persistent flag on rootCmd)")
	require.Equal(t, "bool", flag.Value.Type(), "--yolo must be a bool flag")
}

// TestYoloFlagAvailableOnRootCmd ensures the flag is still present on the
// root command for interactive mode.
func TestYoloFlagAvailableOnRootCmd(t *testing.T) {
	t.Parallel()

	flag := rootCmd.Flags().Lookup("yolo")
	if flag == nil {
		flag = rootCmd.PersistentFlags().Lookup("yolo")
	}
	require.NotNil(t, flag, "the --yolo flag must be available on `crush` (rootCmd)")
}
