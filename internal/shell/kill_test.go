package shell

import (
	"bytes"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKillUsesExternalBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("kill is not an external binary on windows")
	}
	var stdout, stderr bytes.Buffer
	err := Run(t.Context(), RunOptions{
		Command: "kill -l",
		Cwd:     t.TempDir(),
		Env:     os.Environ(),
		Stdout:  &stdout,
		Stderr:  &stderr,
	})
	combined := stdout.String() + stderr.String()
	require.NoError(t, err, combined)
	require.NotContains(t, combined, "unsupported builtin")
	require.Contains(t, strings.ToUpper(combined), "TERM")
}

func TestBlockedKillDoesNotRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("kill is not an external binary on windows")
	}
	var stderr bytes.Buffer
	err := Run(t.Context(), RunOptions{
		Command: "kill -l",
		Cwd:     t.TempDir(),
		Env:     os.Environ(),
		Stderr:  &stderr,
		BlockFuncs: []BlockFunc{
			func(args []string) bool {
				return len(args) > 0 && args[0] == "kill"
			},
		},
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "not allowed for security reasons")
}
