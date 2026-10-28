package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// TestResolveCwdReturnsAbsolutePath covers a relative --cwd. In client/server
// mode the result is sent to the server as the workspace path, and the server
// resolves a relative path against its own working directory, which belongs
// to whichever shell spawned it, so the client would open another project.
func TestResolveCwdReturnsAbsolutePath(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"proj/sub", "other"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, filepath.FromSlash(dir)), 0o755))
	}

	tests := []struct {
		name string
		flag string
		want string
	}{
		{name: "dot", flag: ".", want: "proj"},
		{name: "child", flag: "sub", want: "proj/sub"},
		{name: "parent", flag: "..", want: "."},
		{name: "sibling", flag: "../other", want: "other"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(filepath.Join(root, "proj"))
			cmd := &cobra.Command{}
			cmd.Flags().String("cwd", "", "")
			require.NoError(t, cmd.Flags().Set("cwd", tt.flag))

			got, err := ResolveCwd(cmd)
			require.NoError(t, err)
			require.True(t, filepath.IsAbs(got), "ResolveCwd returned relative path %q", got)

			want := filepath.Join(root, filepath.FromSlash(tt.want))
			requireSameDir(t, want, got)
			wd, err := os.Getwd()
			require.NoError(t, err)
			requireSameDir(t, want, wd)
		})
	}
}

// requireSameDir fails unless want and got name the same directory. It
// compares the directories rather than their spellings, which differ when
// the temp dir sits behind a symlink (/var -> /private/var on macOS).
func requireSameDir(t *testing.T, want, got string) {
	t.Helper()
	wantInfo, err := os.Stat(want)
	require.NoError(t, err)
	gotInfo, err := os.Stat(got)
	require.NoError(t, err)
	require.True(t, os.SameFile(wantInfo, gotInfo), "%q is not %q", got, want)
}
