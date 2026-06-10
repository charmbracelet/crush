package workspace_test

import (
	"context"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/client"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/server"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

// xdgIsolate redirects HOME and XDG_* to fresh temp dirs so config
// loading does not touch the host's real config.
func xdgIsolate(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
}

// runtimeServer wires the production server handler around an
// httptest.NewServer for integration testing.
type runtimeServer struct {
	httpSrv *httptest.Server
	host    string
}

func newRuntimeServer(t *testing.T) *runtimeServer {
	t.Helper()
	s := server.NewServer(nil, "tcp", "127.0.0.1:0")
	hs := httptest.NewServer(s.Handler())
	t.Cleanup(hs.Close)

	u, err := url.Parse(hs.URL)
	require.NoError(t, err)
	return &runtimeServer{httpSrv: hs, host: u.Host}
}

func (r *runtimeServer) newClient(t *testing.T, path string) *client.Client {
	t.Helper()
	c, err := client.NewClient(path, "tcp", r.host)
	require.NoError(t, err)
	return c
}

// TestClientWorkspace_ConfigChangedRefreshesSiblingCache is the
// cross-client refresh end-to-end test required by PLAN item 4. Two
// ClientWorkspace instances pointed at the same backend workspace
// subscribe to events; when one mutates configuration via the server,
// the other's cached Config snapshot reflects the new value without
// a manual refresh.
func TestClientWorkspace_ConfigChangedRefreshesSiblingCache(t *testing.T) {
	xdgIsolate(t)
	rt := newRuntimeServer(t)

	cwd := t.TempDir()
	dataDir := t.TempDir()

	cA := rt.newClient(t, cwd)
	cB := rt.newClient(t, cwd)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	wsProto, err := cA.CreateWorkspace(ctx, proto.Workspace{Path: cwd, DataDir: dataDir})
	require.NoError(t, err)
	// Client B joins the same workspace by path; the server
	// deduplicates and returns the existing workspace.
	wsProtoB, err := cB.CreateWorkspace(ctx, proto.Workspace{Path: cwd, DataDir: dataDir})
	require.NoError(t, err)
	require.Equal(t, wsProto.ID, wsProtoB.ID)

	wsA := workspace.NewClientWorkspace(cA, *wsProto)
	wsB := workspace.NewClientWorkspace(cB, *wsProtoB)

	// Both clients attach event streams. They run for the
	// lifetime of the test; cancelling via context tears them
	// down. consumeEvents is exercised by Subscribe in production;
	// here we run it inline so we don't need a real *tea.Program.
	evcA, err := cA.SubscribeEvents(ctx, wsProto.ID)
	require.NoError(t, err)
	evcB, err := cB.SubscribeEvents(ctx, wsProto.ID)
	require.NoError(t, err)

	go wsA.ConsumeEventsForTest(evcA, func(tea.Msg) {})
	go wsB.ConsumeEventsForTest(evcB, func(tea.Msg) {})

	// Pre-condition: neither cache has compact mode enabled yet.
	require.NotNil(t, wsA.Config())
	require.NotNil(t, wsB.Config())
	require.False(t, compactMode(wsA.Config()), "compact mode must start disabled on client A")
	require.False(t, compactMode(wsB.Config()), "compact mode must start disabled on client B")

	// Client A flips a real config-mutating workspace operation
	// (SetCompactMode) via the server. PLAN item 4 acceptance:
	// B's cached ws.Config must reflect this change without restart.
	// SetCompactMode is used over UpdatePreferredModel because the
	// latter's autoReload reverts unknown-provider models back to
	// defaults during configureSelectedModels, which would make the
	// assertion test infrastructure rather than the cache wiring.
	require.NoError(t, wsA.SetCompactMode(config.ScopeGlobal, true))

	// Client A writes and refreshes synchronously inside
	// SetCompactMode, so its cache must already reflect the change.
	// Eventually here absorbs any background work but should pass
	// immediately.
	require.Eventually(t, func() bool { return compactMode(wsA.Config()) },
		3*time.Second, 25*time.Millisecond,
		"client A cache must reflect its own compact-mode mutation")

	// Client B must see the same change via the ConfigChanged SSE
	// event triggering its own cached refresh.
	require.Eventually(t, func() bool { return compactMode(wsB.Config()) },
		3*time.Second, 25*time.Millisecond,
		"client B cache must reflect A's compact-mode mutation via SSE")
}

// compactMode is a tiny accessor that survives nil intermediates so
// the Eventually polling loop can call it on a transient cache state.
func compactMode(cfg *config.Config) bool {
	if cfg == nil || cfg.Options == nil {
		return false
	}
	return cfg.Options.TUI.CompactMode
}

// TestClientWorkspace_ConfigChangedSignalArrives is a smaller test
// that asserts the SSE wiring delivers a ConfigChanged event to the
// raw client subscription. It catches breakage in the
// wrapEvent/decoder bridge independent of the workspace cache.
func TestClientWorkspace_ConfigChangedSignalArrives(t *testing.T) {
	xdgIsolate(t)
	rt := newRuntimeServer(t)

	cwd := t.TempDir()
	dataDir := t.TempDir()

	c := rt.newClient(t, cwd)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	wsProto, err := c.CreateWorkspace(ctx, proto.Workspace{Path: cwd, DataDir: dataDir})
	require.NoError(t, err)

	evc, err := c.SubscribeEvents(ctx, wsProto.ID)
	require.NoError(t, err)

	require.NoError(t, c.SetConfigField(ctx, wsProto.ID, config.ScopeGlobal, "options.debug", true))

	gotConfigChanged := false
	deadline := time.After(3 * time.Second)
loop:
	for !gotConfigChanged {
		select {
		case ev, ok := <-evc:
			if !ok {
				break loop
			}
			if cc, isCC := ev.(pubsub.Event[proto.ConfigChanged]); isCC {
				require.Equal(t, wsProto.ID, cc.Payload.WorkspaceID)
				gotConfigChanged = true
			}
		case <-deadline:
			break loop
		}
	}
	require.True(t, gotConfigChanged, "expected ConfigChanged event over SSE")
}
