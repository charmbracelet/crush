package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/server"
	"github.com/charmbracelet/x/ansi"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestTakeResumeToken(t *testing.T) {
	t.Setenv(ansi.ProgramResumeTokenEnv, "token")
	require.Equal(t, "token", takeResumeToken())
	_, ok := os.LookupEnv(ansi.ProgramResumeTokenEnv)
	require.False(t, ok, "children must not inherit the token")
}

func TestResumeCommand(t *testing.T) {
	origHost, origArgs := clientHost, os.Args
	t.Cleanup(func() { clientHost, os.Args = origHost, origArgs })
	exe, err := os.Executable()
	require.NoError(t, err)
	wd, err := os.Getwd()
	require.NoError(t, err)

	// The absolute path the user ran, such as a Homebrew or Nix link, is kept
	// rather than resolved to the versioned file behind it.
	link := filepath.Join(t.TempDir(), "crush")
	clientHost = server.DefaultHost()
	for arg0, want := range map[string]string{
		"crush":     "crush",
		link:        link,
		"./crush":   exe,
		"bin/crush": exe,
	} {
		os.Args = []string{arg0}
		require.Equal(t, []string{want}, resumeCommand(&cobra.Command{}), "os.Args[0] = %q", arg0)
	}

	os.Args = []string{"crush"}
	c := &cobra.Command{}
	c.Flags().String("data-dir", "", "")
	c.Flags().CountP("yolo", "y", "")
	c.Flags().StringSlice("channels", nil, "")
	addPromptFlag(c.Flags())
	require.NoError(t, c.Flags().Parse([]string{
		"--data-dir", "data", "-yy", "--channels", "server:webhook",
		"--prompt", "coder=/p/coder.md", "--prompt", "task=/p/task.md",
	}))
	clientHost = "tcp://127.0.0.1:9999"
	require.Equal(t,
		[]string{
			"crush", "--host", "tcp://127.0.0.1:9999", "--data-dir", filepath.Join(wd, "data"),
			"--prompt", "coder=/p/coder.md", "--prompt", "task=/p/task.md",
		},
		resumeCommand(c),
		"yolo and channels are dropped",
	)
}
