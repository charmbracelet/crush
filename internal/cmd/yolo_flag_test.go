package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestYoloFlagNotPersistent guards against --yolo being registered as a
// persistent flag: it is a local flag on the root command and on the
// run subcommand only, so no other subcommand inherits a flag that
// silently disables permission prompts.
func TestYoloFlagNotPersistent(t *testing.T) {
	t.Parallel()

	require.NotNil(t, rootCmd.Flags().Lookup("yolo"),
		"--yolo must stay available on `crush` (rootCmd)")
	require.NotNil(t, runCmd.Flags().Lookup("yolo"),
		"--yolo must be available on `crush run` (register it locally on runCmd)")
	require.Nil(t, rootCmd.PersistentFlags().Lookup("yolo"),
		"--yolo must not be persistent: no subcommand may inherit it")
}

// TestYoloFlagParsesBeforeSubcommand verifies the harness invocation
// `crush --yolo run "prompt"` parses: cobra hands the whole argv to the
// leaf command, so runCmd's local registration covers a flag placed
// before the subcommand name as well as after it.
func TestYoloFlagParsesBeforeSubcommand(t *testing.T) {
	t.Parallel()

	for _, argv := range [][]string{
		{"--yolo", "run", "prompt"},
		{"run", "--yolo", "prompt"},
		{"-y", "run", "prompt"},
	} {
		cmd, flags, err := rootCmd.Find(argv)
		require.NoError(t, err, "argv: %v", argv)
		require.Equal(t, "run", cmd.Name(), "argv: %v", argv)
		require.NoError(t, cmd.ParseFlags(flags), "argv: %v", argv)
		yolo, err := cmd.Flags().GetBool("yolo")
		require.NoError(t, err, "argv: %v", argv)
		require.True(t, yolo, "argv: %v", argv)
	}
}
