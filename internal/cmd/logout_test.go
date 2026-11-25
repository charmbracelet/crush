package cmd

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestLogoutCmd_Aliases(t *testing.T) {
	t.Parallel()

	require.Equal(t, "signout", logoutCmd.Aliases[0])
}

func TestLogoutCmd_HasForceFlag(t *testing.T) {
	t.Parallel()

	flag := logoutCmd.Flags().Lookup("force")
	require.NotNil(t, flag)
	require.Equal(t, "f", flag.Shorthand)
	require.Equal(t, "false", flag.DefValue)
}

func TestLogoutCmd_ValidArgs(t *testing.T) {
	t.Parallel()

	validPlatforms := map[string]bool{}
	for _, p := range logoutCmd.ValidArgs {
		validPlatforms[p] = true
	}
	require.True(t, validPlatforms["hyper"])
	require.True(t, validPlatforms["copilot"])
	require.True(t, validPlatforms["github"])
	require.True(t, validPlatforms["github-copilot"])
	require.True(t, validPlatforms["openai"])
	require.True(t, validPlatforms["chatgpt"])
}

// TestLogout_LocalModeStartsNoServer guards logout in local mode, which used
// to start a background crush server just to remove credentials from the
// config.
func TestLogout_LocalModeStartsNoServer(t *testing.T) {
	if os.Getenv(childTestEnv) == "1" {
		logoutWithoutServer(t, os.Getenv("CRUSH_TEST_PROJECT"))
		return
	}
	t.Parallel()

	runChildTest(t,
		"CRUSH_CLIENT_SERVER=0",
		"CRUSH_GLOBAL_CONFIG="+t.TempDir(),
		"CRUSH_GLOBAL_DATA="+t.TempDir(),
		"CRUSH_CACHE_DIR="+t.TempDir(),
		"CRUSH_DISABLE_PROVIDER_AUTO_UPDATE=1",
		"CRUSH_DISABLE_METRICS=1",
		"CRUSH_TEST_PROJECT="+t.TempDir(),
	)
}

// logoutWithoutServer runs `crush logout` from project and fails if it
// tries to start a server. It runs in a child process: the command changes
// into project and opens its log and database there.
func logoutWithoutServer(t *testing.T, project string) {
	clientHost = "unix://" + filepath.Join(t.TempDir(), "crush.sock")
	spawned := 0
	startDetachedServer = func(*cobra.Command, *url.URL) (*spawnedServer, error) {
		spawned++
		return nil, errors.New("no server may be started")
	}

	cmd := &cobra.Command{}
	cmd.Flags().String("host", "", "")
	cmd.Flags().Bool("debug", false, "")
	cmd.Flags().String("data-dir", "", "")
	cmd.Flags().String("cwd", project, "")
	cmd.Flags().Count("yolo", "")
	cmd.Flags().StringSlice("channels", nil, "")
	addPromptFlag(cmd.Flags())
	cmd.Flags().Bool("force", true, "")
	cmd.SetContext(t.Context())

	require.NoError(t, logoutCmd.RunE(cmd, []string{"hyper"}))
	require.Zero(t, spawned, "logout started a crush server in local mode")
}
