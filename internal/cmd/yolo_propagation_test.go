package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// TestYoloFlagFromSubcommandReachesPermissionService proves that --yolo
// parsed on a SUBCOMMAND (the `crush run` shape) is actually honoured by
// the permission service, and not merely accepted by the flag parser.
//
// Registering the flag persistently — which is all TestYoloFlagAvailableOnRunCmd
// checks — only proves `crush run --yolo` stops failing with "unknown flag".
// It says nothing about whether the value survives the hop through
// setupLocalWorkspace -> config.RuntimeOverrides -> app.New ->
// permission.NewPermissionService. Neutering that hop (hardcoding
// `yolo := false` in setupLocalWorkspace) leaves every registration test
// green, so without this test the feature is unverified: the user would
// get no error and no effect.
//
// The command tree is built locally rather than reusing the package-level
// rootCmd/runCmd because parsing flags mutates the shared pflag.FlagSet
// that the parallel registration tests read concurrently. The persistent
// flags mirror those registered in root.go's init; the code under test —
// setupLocalWorkspace — is the real production function that runCmd.RunE
// calls.
func TestYoloFlagFromSubcommandReachesPermissionService(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want bool
	}{
		{name: "yolo set", args: []string{"run", "--yolo"}, want: true},
		{name: "yolo unset", args: []string{"run"}, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := bestEffortTempDir(t)
			t.Setenv("HOME", dir)
			t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
			t.Setenv("CRUSH_DISABLE_PROVIDER_AUTO_UPDATE", "1")

			root := &cobra.Command{Use: "crush"}
			root.PersistentFlags().BoolP("yolo", "y", false, "")
			root.PersistentFlags().StringP("cwd", "c", "", "")
			root.PersistentFlags().StringP("data-dir", "D", "", "")
			root.PersistentFlags().BoolP("debug", "d", false, "")
			root.PersistentFlags().StringSlice("channels", nil, "")

			var got bool
			run := &cobra.Command{
				Use: "run",
				RunE: func(cmd *cobra.Command, _ []string) error {
					ws, cleanup, err := setupLocalWorkspace(cmd)
					if err != nil {
						return err
					}
					defer cleanup()
					got = ws.PermissionSkipRequests()
					return nil
				},
			}
			root.AddCommand(run)

			args := append([]string{}, tc.args...)
			args = append(args, "--cwd", dir, "--data-dir", filepath.Join(dir, ".crush"))
			root.SetArgs(args)
			root.SetOut(discard{})
			root.SetErr(discard{})
			require.NoError(t, root.ExecuteContext(context.Background()))

			require.Equal(t, tc.want, got,
				"permission service skip-requests must follow the --yolo flag parsed on the subcommand")
		})
	}
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

// bestEffortTempDir returns a temp dir removed on a best-effort basis, unlike
// t.TempDir() which FAILS the test when removal fails.
//
// setupLocalWorkspace opens handles that deliberately outlive it: internal/log.Setup
// installs a process-global lumberjack rotator over <data-dir>/logs/crush.log
// behind a sync.Once and never closes it, and the sqlite connection is owned by
// the app. On Windows an open handle blocks removal, so t.TempDir() would fail
// this test for a reason that has nothing to do with what it asserts. It did:
// "TempDir RemoveAll cleanup: unlinkat ...\.crush\logs\crush.log: The process
// cannot access the file because it is being used by another process."
func bestEffortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "crush-yolo-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}
