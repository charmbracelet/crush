package cmd

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestRunCommandAllowsLegacyServer(t *testing.T) {
	t.Setenv("CRUSH_CLIENT_SERVER", "true")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	stdin, err := os.Open(os.DevNull)
	require.NoError(t, err)
	defer stdin.Close()
	previousStdin, previousHost := os.Stdin, clientHost
	os.Stdin = stdin
	defer func() { os.Stdin, clientHost = previousStdin, previousHost }()
	var mu sync.Mutex
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		// Stop at workspace acquisition: reaching it proves an old server
		// is not rejected for lacking the new capability advertisement.
		http.Error(w, "workspace fixture", http.StatusBadRequest)
	}))
	defer server.Close()
	u, err := url.Parse(server.URL)
	require.NoError(t, err)
	clientHost = "tcp://" + u.Host
	command := &cobra.Command{}
	command.SetContext(t.Context())
	require.ErrorContains(t, runCmd.RunE(command, []string{"hello"}), "failed to create workspace")
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, []string{"/v1/workspaces"}, paths, "no capability gate or workspace-wide permission fallback")
}
