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
	origHost := clientHost
	t.Cleanup(func() { clientHost = origHost })
	exe, err := os.Executable()
	require.NoError(t, err)
	wd, err := os.Getwd()
	require.NoError(t, err)

	newCmd := func(dataDir string) *cobra.Command {
		c := &cobra.Command{}
		c.Flags().String("data-dir", "", "")
		require.NoError(t, c.Flags().Set("data-dir", dataDir))
		return c
	}

	clientHost = server.DefaultHost()
	require.Equal(t, []string{exe}, resumeCommand(newCmd("")))

	clientHost = "tcp://127.0.0.1:9999"
	require.Equal(t,
		[]string{exe, "--host", "tcp://127.0.0.1:9999", "--data-dir", filepath.Join(wd, "data")},
		resumeCommand(newCmd("data")),
	)
}
