package cmd

import (
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// promptTestCmd parses args against a fresh command carrying the same
// --prompt flag as rootCmd, so tests never share mutable cobra state.
func promptTestCmd(t *testing.T, args ...string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "crush"}
	addPromptFlag(cmd.Flags())
	require.NoError(t, cmd.ParseFlags(args))
	return cmd
}

// The flag must be persistent so `crush run --prompt ...` accepts it.
func TestPromptFlagAvailableOnRunCmd(t *testing.T) {
	t.Parallel()

	flag := runCmd.Flags().Lookup("prompt")
	require.NotNil(t, flag, "--prompt must be available on `crush run`")
	require.Equal(t, "stringArray", flag.Value.Type())
}

func TestPromptOverridesNone(t *testing.T) {
	t.Parallel()

	prompts, err := promptOverrides(promptTestCmd(t))
	require.NoError(t, err)
	require.Nil(t, prompts)
}

func TestPromptOverridesRepeatable(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	coder := filepath.Join(dir, "coder.md.tpl")
	summary := filepath.Join(dir, "a,b.md")

	prompts, err := promptOverrides(promptTestCmd(t,
		"--prompt", "coder="+coder,
		"--prompt", "summary="+summary,
	))
	require.NoError(t, err)
	require.Equal(t, config.PromptPaths{"coder": coder, "summary": summary}, prompts)
}

// Relative paths are pinned to the shell's cwd before they reach a server
// that runs somewhere else.
func TestPromptOverridesMakesPathsAbsolute(t *testing.T) {
	t.Parallel()

	prompts, err := promptOverrides(promptTestCmd(t, "--prompt", "coder=coder.md.tpl"))
	require.NoError(t, err)
	want, err := filepath.Abs("coder.md.tpl")
	require.NoError(t, err)
	require.Equal(t, want, prompts["coder"])
}

func TestPromptOverridesRejectsBadInput(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ arg, err string }{
		{"coder", "expected name=path"},
		{"=coder.md", "expected name=path"},
		{"coder=", "expected name=path"},
		{"codr=coder.md", `unknown prompt "codr"`},
	} {
		_, err := promptOverrides(promptTestCmd(t, "--prompt", tc.arg))
		require.ErrorContains(t, err, tc.err, tc.arg)
	}
}

// TestResolveFlagPaths_BeforeCwd covers a relative unix --host and
// --prompt being resolved against --cwd, which the command changes into,
// instead of against the directory crush was started in.
func TestResolveFlagPaths_BeforeCwd(t *testing.T) {
	// Not parallel: it changes directory and swaps a package var.
	start, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	t.Chdir(start)
	origHost := clientHost
	clientHost = "unix://crush.sock"
	t.Cleanup(func() { clientHost = origHost })

	cmd := promptTestCmd(t, "--prompt", "coder=./coder.md.tpl")
	require.NoError(t, resolveFlagPaths(cmd))
	t.Chdir(t.TempDir())

	require.Equal(t, "unix://"+filepath.Join(start, "crush.sock"), clientHost)
	prompts, err := promptOverrides(cmd)
	require.NoError(t, err)
	require.Equal(t, config.PromptPaths{"coder": filepath.Join(start, "coder.md.tpl")}, prompts)
}
