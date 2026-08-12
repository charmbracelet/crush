package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/charmbracelet/crush/internal/client"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/version"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// controlServerOpts describes the server restartIfStale will find.
type controlServerOpts struct {
	// vi is what /version reports.
	vi proto.VersionInfo
	// legacy makes the server reject the conditional shutdown command as
	// unknown, the way every server released before the idleness guard
	// does. Such a server obeys a plain shutdown unconditionally, so a
	// client must never send it one without first verifying idleness.
	legacy bool
	// busy makes the server decline the shutdown because it is in use.
	busy bool
	// workspaces is the count the server reports for GET /workspaces,
	// used by the legacy fallback to decide whether it is safe to send
	// the unconditional shutdown. Defaults to 0 (idle).
	workspaces int
}

// controlLog records the control commands a server received.
type controlLog struct {
	mu       sync.Mutex
	commands []string
}

func (l *controlLog) all() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.commands...)
}

func newControlServer(t *testing.T, opts controlServerOpts) (*url.URL, *controlLog) {
	t.Helper()
	log := &controlLog{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/version":
			require.NoError(t, json.NewEncoder(w).Encode(opts.vi))
		case "/v1/workspaces":
			ws := make([]proto.Workspace, opts.workspaces)
			require.NoError(t, json.NewEncoder(w).Encode(ws))
		case "/v1/control":
			var req proto.ServerControl
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			log.mu.Lock()
			log.commands = append(log.commands, req.Command)
			log.mu.Unlock()
			switch {
			case opts.legacy && req.Command != proto.ServerControlShutdown:
				http.Error(w, "unknown command", http.StatusBadRequest)
			case opts.busy:
				http.Error(w, "server is hosting live workspaces", http.StatusConflict)
			default:
				w.WriteHeader(http.StatusOK)
			}
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	return &url.URL{Scheme: "tcp", Host: u.Host}, log
}

// staleVersion never matches this build, so restartIfStale always
// considers the server a candidate for replacement.
func staleVersion() proto.VersionInfo {
	return proto.VersionInfo{
		Version: "not-" + version.Version,
		BuildID: "stale-build",
	}
}

func versionCheckCmd(t *testing.T) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	return cmd
}

// TestRestartIfStale_HonorsRefusedShutdown is the headline regression.
// BuildID derives from the executable's mtime, so any rebuild — including
// every `go run` — makes a second session look like an upgrade. Launching
// it used to shut down the first session's server, taking its workspaces
// with it and stranding it on a dead workspace ID. Now the server has the
// final say, and a refusal means "keep using me".
func TestRestartIfStale_HonorsRefusedShutdown(t *testing.T) {
	t.Parallel()

	hostURL, log := newControlServer(t, controlServerOpts{
		vi:   staleVersion(),
		busy: true,
	})

	restarted, err := restartIfStale(versionCheckCmd(t), hostURL)
	require.NoError(t, err)
	require.False(t, restarted, "a refused shutdown must not be reported as a restart")
	require.Equal(t, []string{proto.ServerControlShutdownIfIdle}, log.all(),
		"the request must be the conditional one the server can veto")
}

// TestRestartIfStale_LeavesLegacyServerBusy is the upgrade-transition
// case. A server predating the idleness guard rejects the conditional
// command, and the only command it does understand is unconditional —
// sending that would kill every session it hosts. When the server
// reports active workspaces, it must be left alone.
func TestRestartIfStale_LeavesLegacyServerBusy(t *testing.T) {
	t.Parallel()

	hostURL, log := newControlServer(t, controlServerOpts{
		vi:         staleVersion(),
		legacy:     true,
		workspaces: 3,
	})

	restarted, err := restartIfStale(versionCheckCmd(t), hostURL)
	require.NoError(t, err)
	require.False(t, restarted, "a legacy server with workspaces must be reused, not replaced")
	require.Equal(t, []string{proto.ServerControlShutdownIfIdle}, log.all(),
		"the unconditional shutdown must never be sent when workspaces are active")
}

// TestRestartIfStale_ShutsDownLegacyServerWhenIdle covers the case where
// a legacy server (predating shutdown_if_idle) is idle. The client
// verifies idleness via ListWorkspaces, then sends the unconditional
// shutdown the old server understands.
func TestRestartIfStale_ShutsDownLegacyServerWhenIdle(t *testing.T) {
	t.Parallel()

	hostURL, log := newControlServer(t, controlServerOpts{
		vi:     staleVersion(),
		legacy: true,
	})

	restarted, err := restartIfStale(versionCheckCmd(t), hostURL)
	require.NoError(t, err)
	require.True(t, restarted, "an idle legacy server must be replaced")
	require.Equal(t,
		[]string{proto.ServerControlShutdownIfIdle, proto.ServerControlShutdown},
		log.all(),
		"the conditional shutdown is tried first, then the unconditional fallback")
}

// TestRestartIfStale_RestartsWhenServerAgrees preserves the upgrade path:
// a mismatched server that grants the shutdown is replaced promptly.
func TestRestartIfStale_RestartsWhenServerAgrees(t *testing.T) {
	t.Parallel()

	hostURL, log := newControlServer(t, controlServerOpts{vi: staleVersion()})

	restarted, err := restartIfStale(versionCheckCmd(t), hostURL)
	require.NoError(t, err)
	require.True(t, restarted, "an idle stale server must be restarted")
	require.Equal(t, []string{proto.ServerControlShutdownIfIdle}, log.all())
}

// TestRestartIfStale_MatchingServerUntouched is the control: same version
// and build ID means no control request at all.
func TestRestartIfStale_MatchingServerUntouched(t *testing.T) {
	t.Parallel()

	hostURL, log := newControlServer(t, controlServerOpts{
		vi: proto.VersionInfo{Version: version.Version, BuildID: version.BuildID},
	})

	restarted, err := restartIfStale(versionCheckCmd(t), hostURL)
	require.NoError(t, err)
	require.False(t, restarted)
	require.Empty(t, log.all())
}

// TestCreateWorkspaceOnLiveServer_RetriesPastExitingServer covers the
// startup race left over from making the shutdown decision final: a client
// can reach a server in the instant between its committing to an idle
// shutdown and its socket disappearing. Failing the CLI there would be a
// regression, so the client has to bring up a replacement and ask again.
func TestCreateWorkspaceOnLiveServer_RetriesPastExitingServer(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	replaced := 0
	creates := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/workspaces", r.URL.Path)
		mu.Lock()
		creates++
		// Only once the client has stood up a replacement does the create
		// succeed, so passing this test requires the retry to be real.
		serve := replaced > 0
		mu.Unlock()
		if !serve {
			http.Error(w, "server is shutting down", http.StatusServiceUnavailable)
			return
		}
		require.NoError(t, json.NewEncoder(w).Encode(proto.Workspace{ID: "ws-new"}))
	}))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)

	c, err := client.NewClient(t.TempDir(), "tcp", u.Host)
	require.NoError(t, err)

	ws, err := createWorkspaceOnLiveServer(t.Context(), c, proto.Workspace{Path: t.TempDir()},
		func() error {
			mu.Lock()
			replaced++
			mu.Unlock()
			return nil
		})
	require.NoError(t, err)
	require.Equal(t, "ws-new", ws.ID)

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 1, replaced, "exactly one replacement must be spawned")
	require.Equal(t, 2, creates)
}

// TestCreateWorkspaceOnLiveServer_GivesUpOnOtherFailures makes sure the
// retry is scoped to the shutdown race and does not paper over real
// errors by respawning servers.
func TestCreateWorkspaceOnLiveServer_GivesUpOnOtherFailures(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)

	c, err := client.NewClient(t.TempDir(), "tcp", u.Host)
	require.NoError(t, err)

	_, err = createWorkspaceOnLiveServer(t.Context(), c, proto.Workspace{Path: t.TempDir()},
		func() error {
			t.Fatal("a non-lifecycle failure must not spawn a replacement server")
			return nil
		})
	require.Error(t, err)
}
