package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// The tests in this file are not parallel on purpose: parsing flags mutates
// the package-level rootCmd and runCmd, which other tests read.

// TestRunCmdAcceptsYolo guards against `crush run --yolo` failing with
// "unknown flag". Scripts and agents pass --yolo to `crush run` because the
// root command has it, so run must accept it too.
func TestRunCmdAcceptsYolo(t *testing.T) {
	for _, arg := range []string{"--yolo", "-y"} {
		t.Run(arg, func(t *testing.T) {
			cmd := parseCmd(t, "run", arg, "hello")
			require.Equal(t, runCmd, cmd)

			yolo, err := cmd.Flags().GetBool("yolo")
			require.NoError(t, err)
			require.True(t, yolo)
		})
	}
}

// TestYoloFlagIgnoredByRun ensures --yolo on `crush run` doesn't reach the
// workspace. Non-interactive runs already auto-approve their own session;
// forwarding the flag would also make a shared client/server workspace skip
// permissions for every client attached to it.
func TestYoloFlagIgnoredByRun(t *testing.T) {
	cmd := parseCmd(t, "run", "--yolo", "hello")
	require.False(t, yoloFlag(cmd), "--yolo must be a no-op on `crush run`")
}

// TestYoloFlagHonoredByRoot ensures interactive `crush --yolo` still turns
// on yolo mode.
func TestYoloFlagHonoredByRoot(t *testing.T) {
	cmd := parseCmd(t, "--yolo")
	require.Equal(t, rootCmd, cmd)
	require.True(t, yoloFlag(cmd))
}

// parseCmd resolves args against the real command tree and parses the flags
// of the command it finds, the way Execute would. --yolo, the only flag these
// tests set, is reset when the test ends.
func parseCmd(t *testing.T, args ...string) *cobra.Command {
	t.Helper()

	cmd, rest, err := rootCmd.Find(args)
	require.NoError(t, err)
	t.Cleanup(func() {
		if f := cmd.Flags().Lookup("yolo"); f != nil {
			_ = f.Value.Set(f.DefValue)
			f.Changed = false
		}
	})
	require.NoError(t, cmd.ParseFlags(rest))
	return cmd
}
