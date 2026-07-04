package backend

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// newTestBackend returns a Backend whose teardown path skips any
// real [app.App] shutdown work. Useful for state-machine tests that
// install synthetic workspaces directly via insertTestWorkspace.
func newTestBackend(t *testing.T) (*Backend, *atomic.Int32) {
	t.Helper()
	var shutdownCount atomic.Int32
	b := &Backend{
		workspaces:  csync.NewMap[string, *Workspace](),
		pathIndex:   make(map[string]string),
		ctx:         context.Background(),
		createGrace: 50 * time.Millisecond,
		shutdownFn:  func() { shutdownCount.Add(1) },
	}
	return b, &shutdownCount
}

// insertTestWorkspace installs a synthetic workspace into b at the
// given resolved path. Its shutdownFn is recorded in the returned
// counter so tests can assert it ran exactly once.
func insertTestWorkspace(t *testing.T, b *Backend, key string) (*Workspace, *atomic.Int32) {
	t.Helper()
	var shutdowns atomic.Int32
	ws := &Workspace{
		ID:           uuid.New().String(),
		Path:         key,
		resolvedPath: key,
		clients:      make(map[string]*clientState),
		shutdownFn:   func() { shutdowns.Add(1) },
	}
	b.mu.Lock()
	b.workspaces.Set(ws.ID, ws)
	b.pathIndex[key] = ws.ID
	b.mu.Unlock()
	return ws, &shutdowns
}

func newClientID(t *testing.T) string {
	t.Helper()
	return uuid.New().String()
}

func TestResolveWorkspaceKey_AbsoluteAndSymlink(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	real, err := filepath.EvalSymlinks(tmp)
	require.NoError(t, err)

	got, err := resolveWorkspaceKey(tmp)
	require.NoError(t, err)
	require.Equal(t, real, got)
}

func TestResolveWorkspaceKey_NonExistentFallback(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "does", "not", "exist")
	got, err := resolveWorkspaceKey(missing)
	require.NoError(t, err)
	abs, err := filepath.Abs(missing)
	require.NoError(t, err)
	require.Equal(t, abs, got)
}

func TestValidateClientID(t *testing.T) {
	t.Parallel()

	_, err := validateClientID("")
	require.ErrorIs(t, err, ErrInvalidClientID)
	_, err = validateClientID("not-a-uuid")
	require.ErrorIs(t, err, ErrInvalidClientID)

	id := uuid.New().String()
	got, err := validateClientID(id)
	require.NoError(t, err)
	require.Equal(t, id, got)
}

func TestRegisterClient_Idempotent(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	ws, _ := insertTestWorkspace(t, b, "/tmp/a")

	cid := newClientID(t)
	b.registerClient(ws, cid)
	b.registerClient(ws, cid)

	ws.clientsMu.Lock()
	defer ws.clientsMu.Unlock()
	require.Len(t, ws.clients, 1)
	require.NotNil(t, ws.clients[cid].holdTimer)
	require.Equal(t, 0, ws.clients[cid].streams)
}

func TestAttachClient_ConsumesHold(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	ws, shutdowns := insertTestWorkspace(t, b, "/tmp/a")

	cid := newClientID(t)
	b.registerClient(ws, cid)
	require.NoError(t, b.AttachClient(ws.ID, cid))

	ws.clientsMu.Lock()
	require.Len(t, ws.clients, 1)
	require.Nil(t, ws.clients[cid].holdTimer, "attach must stop the grace timer")
	require.Equal(t, 1, ws.clients[cid].streams)
	ws.clientsMu.Unlock()

	// Wait past the grace window: a stopped timer must not fire.
	time.Sleep(150 * time.Millisecond)
	require.Equal(t, int32(0), shutdowns.Load(), "workspace must not be torn down while attached")
}

func TestAttachClient_WithoutPriorCreate(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	ws, _ := insertTestWorkspace(t, b, "/tmp/a")

	cid := newClientID(t)
	require.NoError(t, b.AttachClient(ws.ID, cid))

	ws.clientsMu.Lock()
	defer ws.clientsMu.Unlock()
	require.Len(t, ws.clients, 1)
	require.Equal(t, 1, ws.clients[cid].streams)
	require.Nil(t, ws.clients[cid].holdTimer)
}

func TestAttachClient_DuplicateStreams(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	ws, shutdowns := insertTestWorkspace(t, b, "/tmp/a")

	cid := newClientID(t)
	require.NoError(t, b.AttachClient(ws.ID, cid))
	require.NoError(t, b.AttachClient(ws.ID, cid))

	ws.clientsMu.Lock()
	require.Equal(t, 2, ws.clients[cid].streams)
	ws.clientsMu.Unlock()

	b.DetachClient(ws.ID, cid)
	ws.clientsMu.Lock()
	require.Equal(t, 1, ws.clients[cid].streams)
	ws.clientsMu.Unlock()
	require.Equal(t, int32(0), shutdowns.Load())

	b.DetachClient(ws.ID, cid)
	require.Equal(t, int32(1), shutdowns.Load(), "second detach tears down the workspace")
}

func TestDetachClient_LastStreamTearsDown(t *testing.T) {
	t.Parallel()

	b, srvShutdowns := newTestBackend(t)
	ws, wsShutdowns := insertTestWorkspace(t, b, "/tmp/a")

	cid := newClientID(t)
	b.registerClient(ws, cid)
	require.NoError(t, b.AttachClient(ws.ID, cid))
	b.DetachClient(ws.ID, cid)

	require.Equal(t, int32(1), wsShutdowns.Load())
	require.Equal(t, int32(1), srvShutdowns.Load(), "last workspace shut down must trigger server shutdown")
	_, err := b.GetWorkspace(ws.ID)
	require.ErrorIs(t, err, ErrWorkspaceNotFound)
}

func TestHoldExpiry_TearsDown(t *testing.T) {
	t.Parallel()

	b, srvShutdowns := newTestBackend(t)
	ws, wsShutdowns := insertTestWorkspace(t, b, "/tmp/a")

	cid := newClientID(t)
	b.registerClient(ws, cid)

	require.Eventually(t, func() bool {
		return wsShutdowns.Load() == 1 && srvShutdowns.Load() == 1
	}, 1*time.Second, 5*time.Millisecond)
}

func TestReleaseHold_NoStreams(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	ws, shutdowns := insertTestWorkspace(t, b, "/tmp/a")

	cid := newClientID(t)
	b.registerClient(ws, cid)
	require.NoError(t, b.releaseHold(ws.ID, cid))

	require.Equal(t, int32(1), shutdowns.Load())
	// Idempotent.
	require.NoError(t, b.releaseHold(ws.ID, cid))
	require.Equal(t, int32(1), shutdowns.Load())
}

func TestReleaseHold_WithActiveStream(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	ws, shutdowns := insertTestWorkspace(t, b, "/tmp/a")

	cid := newClientID(t)
	b.registerClient(ws, cid)
	require.NoError(t, b.AttachClient(ws.ID, cid))
	require.NoError(t, b.releaseHold(ws.ID, cid))

	ws.clientsMu.Lock()
	require.Equal(t, 1, ws.clients[cid].streams)
	require.Nil(t, ws.clients[cid].holdTimer)
	ws.clientsMu.Unlock()
	require.Equal(t, int32(0), shutdowns.Load())

	b.DetachClient(ws.ID, cid)
	require.Equal(t, int32(1), shutdowns.Load())
}

func TestReleaseHoldThenAttach(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	ws, shutdowns := insertTestWorkspace(t, b, "/tmp/a")

	cid := newClientID(t)
	require.NoError(t, b.releaseHold(ws.ID, cid)) // no entry yet — no-op.
	require.NoError(t, b.AttachClient(ws.ID, cid))
	ws.clientsMu.Lock()
	require.Equal(t, 1, ws.clients[cid].streams)
	ws.clientsMu.Unlock()
	require.NoError(t, b.releaseHold(ws.ID, cid)) // hold-only no-op (no hold timer).
	require.Equal(t, int32(0), shutdowns.Load())
	b.DetachClient(ws.ID, cid)
	require.Equal(t, int32(1), shutdowns.Load())
}

func TestRefcountWithSecondClient(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	ws, shutdowns := insertTestWorkspace(t, b, "/tmp/a")

	cidA := newClientID(t)
	cidB := newClientID(t)
	b.registerClient(ws, cidA)
	require.NoError(t, b.AttachClient(ws.ID, cidA))
	b.registerClient(ws, cidB)
	require.NoError(t, b.AttachClient(ws.ID, cidB))

	b.DetachClient(ws.ID, cidA)
	ws.clientsMu.Lock()
	require.Contains(t, ws.clients, cidB)
	require.NotContains(t, ws.clients, cidA)
	ws.clientsMu.Unlock()
	require.Equal(t, int32(0), shutdowns.Load(), "workspace survives while second client attached")

	b.DetachClient(ws.ID, cidB)
	require.Equal(t, int32(1), shutdowns.Load())
}

func TestAttachClient_InvalidID(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	ws, _ := insertTestWorkspace(t, b, "/tmp/a")

	require.ErrorIs(t, b.AttachClient(ws.ID, ""), ErrInvalidClientID)
	require.ErrorIs(t, b.AttachClient(ws.ID, "not-a-uuid"), ErrInvalidClientID)
}

func TestDeleteWorkspace_RejectsBadClientID(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	ws, _ := insertTestWorkspace(t, b, "/tmp/a")

	require.ErrorIs(t, b.DeleteWorkspace(ws.ID, ""), ErrInvalidClientID)
	require.ErrorIs(t, b.DeleteWorkspace(ws.ID, "not-a-uuid"), ErrInvalidClientID)
}

// TestHoldExpiry_RaceWithAttach checks that, when the grace timer fires
// while a concurrent AttachClient call is in flight, the workspace ends
// up either fully attached or fully torn down — never in a half-state.
func TestHoldExpiry_RaceWithAttach(t *testing.T) {
	t.Parallel()

	for i := range 50 {
		b, _ := newTestBackend(t)
		// Tighten the grace window further to force the race.
		b.createGrace = 1 * time.Millisecond
		ws, shutdowns := insertTestWorkspace(t, b, "/tmp/race")

		cid := newClientID(t)
		b.registerClient(ws, cid)
		// Attach concurrently with the very short grace timer.
		errCh := make(chan error, 1)
		go func() { errCh <- b.AttachClient(ws.ID, cid) }()
		<-errCh

		// Wait for any pending timer to settle.
		time.Sleep(10 * time.Millisecond)

		ws.clientsMu.Lock()
		gotShutdown := shutdowns.Load() == 1
		cs, present := ws.clients[cid]
		var (
			gotStreams   int
			gotHoldTimer *time.Timer
		)
		if present {
			gotStreams = cs.streams
			gotHoldTimer = cs.holdTimer
		}
		ws.clientsMu.Unlock()
		// Either the workspace was torn down OR the client is
		// attached with streams==1 and the hold timer cleared.
		// The state must be consistent: if shutdown, client is
		// gone; if attached, no teardown and streams==1.
		if gotShutdown {
			require.False(t, present, "iter %d: shutdown but client still present", i)
		} else {
			require.True(t, present, "iter %d: not shutdown but client missing", i)
			require.Equal(t, 1, gotStreams, "iter %d: attach winner must leave streams=1", i)
			require.Nil(t, gotHoldTimer, "iter %d: attach winner must clear holdTimer", i)
		}
	}
}

// TestConcurrentAttachDetach exercises the state machine under
// parallel attach/detach pressure with the race detector.
func TestConcurrentAttachDetach(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	ws, _ := insertTestWorkspace(t, b, "/tmp/a")

	cid := newClientID(t)
	b.registerClient(ws, cid)
	require.NoError(t, b.AttachClient(ws.ID, cid)) // ensure refcount stays > 0.

	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			cid2 := newClientID(t)
			_ = b.AttachClient(ws.ID, cid2)
			b.DetachClient(ws.ID, cid2)
		}()
	}
	wg.Wait()

	ws.clientsMu.Lock()
	defer ws.clientsMu.Unlock()
	require.Len(t, ws.clients, 1)
	require.Contains(t, ws.clients, cid)
}

// TestPathDedupe_FullCreate exercises CreateWorkspace end-to-end
// (config init, real app.App). Two CreateWorkspace calls at the same
// path return the same workspace ID and share the clients map.
func TestPathDedupe_FullCreate(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cwd := t.TempDir()
	dataDir := t.TempDir()

	b := New(context.Background(), nil, func() {})
	b.SetCreateGrace(2 * time.Second)
	t.Cleanup(func() { drainBackend(t, b) })

	cidA := uuid.New().String()
	cidB := uuid.New().String()

	wsA, protoA, err := b.CreateWorkspace(protoWS(cwd, dataDir, cidA))
	require.NoError(t, err)
	require.NotEmpty(t, protoA.ID)
	require.Equal(t, protoA.DataDir, wsA.Cfg.Config().Options.DataDirectory)

	wsB, protoB, err := b.CreateWorkspace(protoWS(cwd, dataDir, cidB))
	require.NoError(t, err)
	require.Equal(t, wsA.ID, wsB.ID, "second create at same path must return existing workspace")
	require.Equal(t, protoA.ID, protoB.ID)

	wsA.clientsMu.Lock()
	require.Contains(t, wsA.clients, cidA)
	require.Contains(t, wsA.clients, cidB)
	wsA.clientsMu.Unlock()
}

// TestPathDedupe_DifferentPaths_DifferentWorkspaces confirms that two
// CreateWorkspace calls at distinct paths produce distinct workspaces.
func TestPathDedupe_DifferentPaths_DifferentWorkspaces(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cwdA := t.TempDir()
	cwdB := t.TempDir()
	dataA := t.TempDir()
	dataB := t.TempDir()

	b := New(context.Background(), nil, func() {})
	b.SetCreateGrace(2 * time.Second)
	t.Cleanup(func() { drainBackend(t, b) })

	wsA, _, err := b.CreateWorkspace(protoWS(cwdA, dataA, uuid.New().String()))
	require.NoError(t, err)
	wsB, _, err := b.CreateWorkspace(protoWS(cwdB, dataB, uuid.New().String()))
	require.NoError(t, err)
	require.NotEqual(t, wsA.ID, wsB.ID)
}

// TestPathDedupe_FirstWinsKeepsOriginalEnv verifies that the second
// create at the same path returns the *originating* client's Env in
// its proto and does not mutate the existing workspace's YOLO/Debug
// flags.
func TestPathDedupe_FirstWinsKeepsOriginalEnv(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cwd := t.TempDir()
	dataDir := t.TempDir()

	b := New(context.Background(), nil, func() {})
	b.SetCreateGrace(2 * time.Second)
	t.Cleanup(func() { drainBackend(t, b) })

	originalEnv := []string{"FOO=bar"}
	argsA := protoWS(cwd, dataDir, uuid.New().String())
	argsA.YOLO = true
	argsA.Env = originalEnv
	wsA, protoA, err := b.CreateWorkspace(argsA)
	require.NoError(t, err)
	require.True(t, protoA.YOLO)
	require.Equal(t, originalEnv, protoA.Env)

	argsB := protoWS(cwd, dataDir, uuid.New().String())
	argsB.YOLO = false
	argsB.Debug = true
	argsB.Env = []string{"BAZ=qux"}
	_, protoB, err := b.CreateWorkspace(argsB)
	require.NoError(t, err)
	require.Equal(t, protoA.ID, protoB.ID)
	require.True(t, protoB.YOLO, "first wins: YOLO must remain true")
	require.Equal(t, originalEnv, protoB.Env, "proto must carry the originating client's Env")
	require.Equal(t, wsA.Cfg.Overrides().SkipPermissionRequests, true)
}

// TestPathDedupe_Symlink confirms two paths that resolve to the same
// target share a workspace.
func TestPathDedupe_Symlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics differ on windows")
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(real, link))
	dataDir := t.TempDir()

	b := New(context.Background(), nil, func() {})
	b.SetCreateGrace(2 * time.Second)
	t.Cleanup(func() { drainBackend(t, b) })

	wsA, _, err := b.CreateWorkspace(protoWS(real, dataDir, uuid.New().String()))
	require.NoError(t, err)
	wsB, _, err := b.CreateWorkspace(protoWS(link, dataDir, uuid.New().String()))
	require.NoError(t, err)
	require.Equal(t, wsA.ID, wsB.ID)
}

// TestPathDedupe_NonExistentPath ensures CreateWorkspace tolerates a
// path that does not yet exist (EvalSymlinks falls back to Abs).
func TestPathDedupe_NonExistentPath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	parent := t.TempDir()
	missing := filepath.Join(parent, "does-not-exist")
	dataDir := t.TempDir()

	b := New(context.Background(), nil, func() {})
	b.SetCreateGrace(2 * time.Second)
	t.Cleanup(func() { drainBackend(t, b) })

	_, p, err := b.CreateWorkspace(protoWS(missing, dataDir, uuid.New().String()))
	require.NoError(t, err)
	require.NotEmpty(t, p.ID)
}

// TestCreateWorkspace_IdempotentSameClient checks that a duplicate
// create from the same client at the same path does not produce a
// second claim.
func TestCreateWorkspace_IdempotentSameClient(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cwd := t.TempDir()
	dataDir := t.TempDir()
	b := New(context.Background(), nil, func() {})
	b.SetCreateGrace(2 * time.Second)
	t.Cleanup(func() { drainBackend(t, b) })

	cid := uuid.New().String()
	ws1, _, err := b.CreateWorkspace(protoWS(cwd, dataDir, cid))
	require.NoError(t, err)
	ws2, _, err := b.CreateWorkspace(protoWS(cwd, dataDir, cid))
	require.NoError(t, err)
	require.Equal(t, ws1.ID, ws2.ID)

	ws1.clientsMu.Lock()
	require.Len(t, ws1.clients, 1, "duplicate create from same client must not double the claim")
	ws1.clientsMu.Unlock()
}

// TestPathDedupe_ParallelCreates ensures two simultaneous CreateWorkspace
// calls at the same path produce the same workspace and the clients map
// contains both client IDs.
func TestPathDedupe_ParallelCreates(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cwd := t.TempDir()
	dataDir := t.TempDir()

	b := New(context.Background(), nil, func() {})
	b.SetCreateGrace(2 * time.Second)
	t.Cleanup(func() { drainBackend(t, b) })

	cidA := uuid.New().String()
	cidB := uuid.New().String()

	type result struct {
		ws    *Workspace
		proto proto.Workspace
		err   error
	}
	ch := make(chan result, 2)
	start := make(chan struct{})
	go func() {
		<-start
		ws, p, err := b.CreateWorkspace(protoWS(cwd, dataDir, cidA))
		ch <- result{ws, p, err}
	}()
	go func() {
		<-start
		ws, p, err := b.CreateWorkspace(protoWS(cwd, dataDir, cidB))
		ch <- result{ws, p, err}
	}()
	close(start)
	r1 := <-ch
	r2 := <-ch
	require.NoError(t, r1.err)
	require.NoError(t, r2.err)
	require.Equal(t, r1.ws.ID, r2.ws.ID, "both creates must converge on one workspace ID")

	ws := r1.ws
	ws.clientsMu.Lock()
	defer ws.clientsMu.Unlock()
	require.Contains(t, ws.clients, cidA)
	require.Contains(t, ws.clients, cidB)
}

// TestCreateWorkspace_RejectsBadClientID covers the 400 path from the
// backend side.
func TestCreateWorkspace_RejectsBadClientID(t *testing.T) {
	t.Parallel()

	b := New(context.Background(), nil, func() {})

	_, _, err := b.CreateWorkspace(protoWS("/tmp/x", t.TempDir(), ""))
	require.ErrorIs(t, err, ErrInvalidClientID)
	_, _, err = b.CreateWorkspace(protoWS("/tmp/x", t.TempDir(), "not-a-uuid"))
	require.ErrorIs(t, err, ErrInvalidClientID)
}

// drainBackend tears the backend down at the end of a test by deleting
// every remaining workspace. Necessary so the test process doesn't
// leak goroutines or DB handles from the embedded [app.App] instances.
func drainBackend(t *testing.T, b *Backend) {
	t.Helper()
	for _, ws := range b.workspaces.Seq2() {
		ws.clientsMu.Lock()
		ids := make([]string, 0, len(ws.clients))
		for id := range ws.clients {
			ids = append(ids, id)
		}
		ws.clientsMu.Unlock()
		for _, cid := range ids {
			_ = b.releaseHold(ws.ID, cid)
		}
	}
}

func protoWS(path, dataDir, clientID string) proto.Workspace {
	return proto.Workspace{Path: path, DataDir: dataDir, ClientID: clientID}
}

// syncBuffer is a thread-safe buffer that can be safely read and written
// from multiple goroutines.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (sb *syncBuffer) Write(p []byte) (n int, err error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.Write(p)
}

func (sb *syncBuffer) String() string {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.String()
}

// captureDebugLogs installs a buffer-backed slog handler at Debug
// level for the duration of the test, returning the buffer. The
// previous default handler is restored via t.Cleanup.
func captureDebugLogs(t *testing.T) *syncBuffer {
	t.Helper()
	var sb syncBuffer
	prev := slog.Default()
	handler := slog.NewTextHandler(&sb, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &sb
}

// xdgIsolated points HOME and XDG_* variables at fresh tempdirs so
// CreateWorkspace's config loading does not interfere with the host
// machine's real config.
func xdgIsolated(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
}

// TestFirstWinsMismatch_LogsOnFlagDifferences verifies that the
// debug mismatch line is emitted when any of YOLO, Debug, DataDir,
// or Env differs between the first and second CreateWorkspace at
// the same path, and that the existing workspace's Debug flag is
// not overwritten.
func TestFirstWinsMismatch_LogsOnFlagDifferences(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*proto.Workspace)
	}{
		{
			name:   "yolo",
			mutate: func(p *proto.Workspace) { p.YOLO = true },
		},
		{
			name:   "debug",
			mutate: func(p *proto.Workspace) { p.Debug = true },
		},
		{
			name:   "datadir",
			mutate: func(p *proto.Workspace) { p.DataDir = "" },
		},
		{
			name:   "env",
			mutate: func(p *proto.Workspace) { p.Env = []string{"NEW=val"} },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			xdgIsolated(t)
			cwd := t.TempDir()
			dataDir := t.TempDir()

			buf := captureDebugLogs(t)
			b := New(context.Background(), nil, func() {})
			b.SetCreateGrace(2 * time.Second)
			t.Cleanup(func() { drainBackend(t, b) })

			argsA := protoWS(cwd, dataDir, uuid.New().String())
			argsA.Env = []string{"FOO=bar"}
			wsA, _, err := b.CreateWorkspace(argsA)
			require.NoError(t, err)
			originalDebug := wsA.Cfg.Config().Options.Debug
			originalYOLO := wsA.Cfg.Overrides().SkipPermissionRequests

			argsB := protoWS(cwd, dataDir, uuid.New().String())
			argsB.Env = []string{"FOO=bar"} // identical by default
			tc.mutate(&argsB)
			_, _, err = b.CreateWorkspace(argsB)
			require.NoError(t, err)

			require.Contains(
				t, buf.String(),
				"Workspace flag mismatch on duplicate create",
				"expected debug log for mismatching %s", tc.name,
			)
			// Existing workspace's YOLO and Debug must not change.
			require.Equal(t, originalYOLO, wsA.Cfg.Overrides().SkipPermissionRequests, "YOLO must be immutable on first-wins")
			require.Equal(t, originalDebug, wsA.Cfg.Config().Options.Debug, "Debug must be immutable on first-wins")
		})
	}
}

// TestFirstWinsMismatch_NoLogWhenIdentical confirms identical args
// do not emit the mismatch log line.
func TestFirstWinsMismatch_NoLogWhenIdentical(t *testing.T) {
	xdgIsolated(t)
	cwd := t.TempDir()
	dataDir := t.TempDir()

	buf := captureDebugLogs(t)
	b := New(context.Background(), nil, func() {})
	b.SetCreateGrace(2 * time.Second)
	t.Cleanup(func() { drainBackend(t, b) })

	argsA := protoWS(cwd, dataDir, uuid.New().String())
	argsA.Env = []string{"FOO=bar"}
	_, _, err := b.CreateWorkspace(argsA)
	require.NoError(t, err)

	argsB := protoWS(cwd, dataDir, uuid.New().String())
	argsB.Env = []string{"FOO=bar"}
	_, _, err = b.CreateWorkspace(argsB)
	require.NoError(t, err)

	require.False(t,
		strings.Contains(buf.String(), "Workspace flag mismatch on duplicate create"),
		"identical args must not log a mismatch: %s", buf.String())
}

// TestChannelOptInBoundary_DuplicateCreate verifies that channels are
// an explicit opt-in that is never shared across a duplicate create at
// the same path. A second client whose requested channels differ from
// the existing workspace (including a client that did not opt in at
// all) is rejected rather than silently inheriting the existing
// workspace's channels.
func TestChannelOptInBoundary_DuplicateCreate(t *testing.T) {
	tests := []struct {
		name            string
		firstChannels   []string
		secondChannels  []string
		wantMismatchErr bool
	}{
		{
			name:            "second omits opt-in",
			firstChannels:   []string{"server:webhook"},
			secondChannels:  nil,
			wantMismatchErr: true,
		},
		{
			name:            "second opts into extra channel",
			firstChannels:   nil,
			secondChannels:  []string{"server:webhook"},
			wantMismatchErr: true,
		},
		{
			name:            "different channels",
			firstChannels:   []string{"server:webhook"},
			secondChannels:  []string{"server:events"},
			wantMismatchErr: true,
		},
		{
			name:            "identical channels shared",
			firstChannels:   []string{"server:webhook"},
			secondChannels:  []string{"server:webhook"},
			wantMismatchErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			xdgIsolated(t)
			cwd := t.TempDir()
			dataDir := t.TempDir()

			b := New(context.Background(), nil, func() {})
			b.SetCreateGrace(2 * time.Second)
			t.Cleanup(func() { drainBackend(t, b) })

			argsA := protoWS(cwd, dataDir, uuid.New().String())
			argsA.Channels = tc.firstChannels
			wsA, _, err := b.CreateWorkspace(argsA)
			require.NoError(t, err)

			argsB := protoWS(cwd, dataDir, uuid.New().String())
			argsB.Channels = tc.secondChannels
			wsB, protoB, err := b.CreateWorkspace(argsB)

			if tc.wantMismatchErr {
				require.ErrorIs(t, err, ErrChannelOptInMismatch)
				require.Nil(t, wsB)
				// The existing workspace must not be mutated
				// and must not register the rejected client.
				require.Equal(t, tc.firstChannels, wsA.Cfg.Overrides().EnabledChannels,
					"existing channels must be immutable on rejection")
				wsA.clientsMu.Lock()
				require.Len(t, wsA.clients, 1, "rejected client must not be registered")
				wsA.clientsMu.Unlock()
				return
			}

			require.NoError(t, err)
			require.Equal(t, wsA.ID, protoB.ID)
			require.Equal(t, tc.firstChannels, protoB.Channels)
		})
	}
}

// TestRaceTwoClientsAttachOneDetaches exercises the PLAN-required
// race scenario: two clients attach concurrently, then one detaches.
// The workspace must remain alive with refcount==1 and the clients
// map must reflect the remaining client only.
func TestRaceTwoClientsAttachOneDetaches(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	ws, shutdowns := insertTestWorkspace(t, b, "/tmp/race-two")

	cidA := newClientID(t)
	cidB := newClientID(t)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		require.NoError(t, b.AttachClient(ws.ID, cidA))
	}()
	go func() {
		defer wg.Done()
		require.NoError(t, b.AttachClient(ws.ID, cidB))
	}()
	wg.Wait()

	ws.clientsMu.Lock()
	require.Len(t, ws.clients, 2, "both clients must be attached")
	ws.clientsMu.Unlock()

	b.DetachClient(ws.ID, cidA)

	ws.clientsMu.Lock()
	require.Len(t, ws.clients, 1, "refcount must be 1 after one detach")
	require.Contains(t, ws.clients, cidB, "remaining client must be cidB")
	require.NotContains(t, ws.clients, cidA, "detached client must be removed")
	ws.clientsMu.Unlock()
	require.Equal(t, int32(0), shutdowns.Load(), "workspace must remain alive")

	// Drain.
	b.DetachClient(ws.ID, cidB)
	require.Equal(t, int32(1), shutdowns.Load())
}

// TestExplicitDeleteThenAttach reproduces the PLAN scenario: start
// with a real hold, releaseHold consumes it, AttachClient from the
// same clientID creates a fresh entry with streams==1, and calling
// releaseHold again is a no-op. A second client keeps the workspace
// alive so AttachClient can still resolve the workspace ID after the
// first client's hold is released.
func TestExplicitDeleteThenAttach(t *testing.T) {
	t.Parallel()

	// Large grace window so timers cannot fire during the test
	// — we want to exercise the explicit releaseHold path.
	b, _ := newTestBackend(t)
	b.createGrace = time.Hour
	ws, shutdowns := insertTestWorkspace(t, b, "/tmp/delete-then-attach")

	// Anchor client keeps the workspace registered in
	// b.workspaces across the cid's releaseHold below.
	anchor := newClientID(t)
	require.NoError(t, b.AttachClient(ws.ID, anchor))

	cid := newClientID(t)
	// Real hold via registerClient (mirrors CreateWorkspace).
	b.registerClient(ws, cid)
	ws.clientsMu.Lock()
	require.Contains(t, ws.clients, cid)
	require.NotNil(t, ws.clients[cid].holdTimer, "hold must be live")
	require.Equal(t, 0, ws.clients[cid].streams)
	ws.clientsMu.Unlock()

	// releaseHold: consumes the hold and removes the entry
	// (streams == 0). The anchor client keeps the workspace
	// alive.
	require.NoError(t, b.releaseHold(ws.ID, cid))
	require.Equal(t, int32(0), shutdowns.Load(), "anchor must keep workspace alive")
	ws.clientsMu.Lock()
	require.NotContains(t, ws.clients, cid, "entry must be removed by releaseHold")
	ws.clientsMu.Unlock()

	// AttachClient creates a fresh entry with streams==1 and no
	// hold timer.
	require.NoError(t, b.AttachClient(ws.ID, cid))
	ws.clientsMu.Lock()
	require.Contains(t, ws.clients, cid, "fresh entry must be created")
	require.Equal(t, 1, ws.clients[cid].streams, "fresh attach must start at streams=1")
	require.Nil(t, ws.clients[cid].holdTimer, "fresh attach must have no hold timer")
	ws.clientsMu.Unlock()

	// Calling releaseHold again is a no-op (no hold timer to
	// stop, streams > 0 so the entry stays).
	require.NoError(t, b.releaseHold(ws.ID, cid))
	ws.clientsMu.Lock()
	require.Contains(t, ws.clients, cid, "releaseHold must not touch a stream-only entry")
	require.Equal(t, 1, ws.clients[cid].streams)
	require.Nil(t, ws.clients[cid].holdTimer)
	ws.clientsMu.Unlock()

	// Drain.
	b.DetachClient(ws.ID, cid)
	b.DetachClient(ws.ID, anchor)
	require.Equal(t, int32(1), shutdowns.Load())
}

// TestAttachClient_RacesWithTeardown forces AttachClient to compete
// with the teardown path triggered by DetachClient. Before the fix,
// AttachClient could observe a workspace after teardown had already
// decided to remove it (because AttachClient did not synchronize with
// Backend.mu), leaving a live stream claim attached to a workspace
// that was then removed and shut down. With the fix, the outcome must
// be deterministic: either AttachClient won and the workspace is
// alive with the client registered, or teardown won and AttachClient
// returns ErrWorkspaceNotFound — never a half-state where the
// workspace is gone but ws.clients still contains the new client.
func TestAttachClient_RacesWithTeardown(t *testing.T) {
	t.Parallel()

	for i := range 200 {
		b, _ := newTestBackend(t)
		// Keep the grace window long so it can't fire during the
		// test and confuse the bookkeeping.
		b.createGrace = time.Hour
		ws, shutdowns := insertTestWorkspace(t, b, "/tmp/race-teardown")

		// Seed: cidA holds the workspace open via a stream. The
		// imminent DetachClient(cidA) will be the *only* claim
		// drop, so teardown will run.
		cidA := newClientID(t)
		require.NoError(t, b.AttachClient(ws.ID, cidA))

		// cidB attempts to attach concurrently with the detach
		// that will tear the workspace down.
		cidB := newClientID(t)
		start := make(chan struct{})
		errCh := make(chan error, 1)
		detachDone := make(chan struct{})
		go func() {
			<-start
			errCh <- b.AttachClient(ws.ID, cidB)
		}()
		go func() {
			<-start
			b.DetachClient(ws.ID, cidA)
			close(detachDone)
		}()
		close(start)

		// Wait for both goroutines so teardown (including
		// shutdownFn) has fully run before we read state.
		attachErr := <-errCh
		<-detachDone

		_, wsStillRegistered := b.workspaces.Get(ws.ID)
		ws.clientsMu.Lock()
		_, hasA := ws.clients[cidA]
		_, hasB := ws.clients[cidB]
		clientCount := len(ws.clients)
		ws.clientsMu.Unlock()
		shutdownCount := shutdowns.Load()

		switch {
		case attachErr == nil:
			// AttachClient won. The workspace must be alive
			// (registered) with cidB in its clients map. cidA
			// may or may not still be there depending on who
			// took clientsMu first, but the workspace must
			// not have been torn down.
			require.True(t, wsStillRegistered,
				"iter %d: attach succeeded but workspace was removed", i)
			require.True(t, hasB,
				"iter %d: attach succeeded but cidB missing from clients", i)
			require.Equal(t, int32(0), shutdownCount,
				"iter %d: attach succeeded but workspace was shut down", i)
		case errors.Is(attachErr, ErrWorkspaceNotFound):
			// Teardown won. The workspace must be removed,
			// shut down exactly once, and ws.clients must be
			// empty (no half-state with cidB inserted into a
			// dead workspace's clients map).
			require.False(t, wsStillRegistered,
				"iter %d: ErrWorkspaceNotFound but workspace still registered", i)
			require.Equal(t, int32(1), shutdownCount,
				"iter %d: ErrWorkspaceNotFound but shutdown count = %d", i, shutdownCount)
			require.False(t, hasA,
				"iter %d: teardown won but cidA still in clients", i)
			require.False(t, hasB,
				"iter %d: teardown won but cidB still in clients (would be the leaked attach)", i)
			require.Zero(t, clientCount,
				"iter %d: teardown won but clients map is non-empty", i)
		default:
			t.Fatalf("iter %d: unexpected AttachClient error: %v", i, attachErr)
		}
	}
}

// TestSetCurrentSession_BasicAttachAndSwitch verifies the happy path:
// an attached client can set its current session, a second attached
// client can target the same session, and one of them can switch to a
// different session without disturbing the other's record.
func TestSetCurrentSession_BasicAttachAndSwitch(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	ws, _ := insertTestWorkspace(t, b, "/tmp/current-session-basic")

	cidA := newClientID(t)
	cidB := newClientID(t)
	require.NoError(t, b.AttachClient(ws.ID, cidA))
	require.NoError(t, b.AttachClient(ws.ID, cidB))

	require.NoError(t, b.SetCurrentSession(ws.ID, cidA, "S1"))
	ws.clientsMu.Lock()
	require.Equal(t, "S1", ws.clients[cidA].currentSessionID)
	ws.clientsMu.Unlock()

	require.NoError(t, b.SetCurrentSession(ws.ID, cidB, "S1"))
	ws.clientsMu.Lock()
	require.Equal(t, "S1", ws.clients[cidA].currentSessionID)
	require.Equal(t, "S1", ws.clients[cidB].currentSessionID)
	ws.clientsMu.Unlock()

	// B switches to S2; counts redistribute.
	require.NoError(t, b.SetCurrentSession(ws.ID, cidB, "S2"))
	ws.clientsMu.Lock()
	require.Equal(t, "S1", ws.clients[cidA].currentSessionID)
	require.Equal(t, "S2", ws.clients[cidB].currentSessionID)
	ws.clientsMu.Unlock()

	// A clears its selection.
	require.NoError(t, b.SetCurrentSession(ws.ID, cidA, ""))
	ws.clientsMu.Lock()
	require.Empty(t, ws.clients[cidA].currentSessionID)
	require.Equal(t, "S2", ws.clients[cidB].currentSessionID)
	ws.clientsMu.Unlock()

	// Drain to release the workspace.
	b.DetachClient(ws.ID, cidA)
	b.DetachClient(ws.ID, cidB)
}

// TestSetCurrentSession_DetachClearsEntry verifies the implicit
// cleanup: once a client's [clientState] entry is removed (last
// stream closed), its currentSessionID is gone with it.
func TestSetCurrentSession_DetachClearsEntry(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	ws, _ := insertTestWorkspace(t, b, "/tmp/current-session-detach")

	// Anchor client so the workspace is not torn down when cid
	// detaches.
	anchor := newClientID(t)
	require.NoError(t, b.AttachClient(ws.ID, anchor))

	cid := newClientID(t)
	require.NoError(t, b.AttachClient(ws.ID, cid))
	require.NoError(t, b.SetCurrentSession(ws.ID, cid, "S2"))

	b.DetachClient(ws.ID, cid)

	ws.clientsMu.Lock()
	_, present := ws.clients[cid]
	ws.clientsMu.Unlock()
	require.False(t, present, "detach must remove the clientState entry along with its currentSessionID")

	// A follow-up SetCurrentSession on the gone client must be
	// rejected with ErrClientNotAttached.
	require.ErrorIs(t, b.SetCurrentSession(ws.ID, cid, "S3"), ErrClientNotAttached)

	b.DetachClient(ws.ID, anchor)
}

// TestSetCurrentSession_RejectsHoldOnly verifies that a registered
// client whose only claim is a creation hold (streams == 0) cannot
// influence presence: SetCurrentSession returns ErrClientNotAttached
// and the entry's currentSessionID stays empty.
func TestSetCurrentSession_RejectsHoldOnly(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	// Keep the grace window large so the hold survives the test.
	b.createGrace = time.Hour
	ws, _ := insertTestWorkspace(t, b, "/tmp/current-session-hold")

	cid := newClientID(t)
	b.registerClient(ws, cid)

	require.ErrorIs(t, b.SetCurrentSession(ws.ID, cid, "S1"), ErrClientNotAttached)

	ws.clientsMu.Lock()
	require.Empty(t, ws.clients[cid].currentSessionID, "hold-only client must not write a session id")
	ws.clientsMu.Unlock()

	// Drain.
	require.NoError(t, b.releaseHold(ws.ID, cid))
}

// TestSetCurrentSession_UnknownClient verifies that a client with no
// entry at all is rejected with ErrClientNotAttached.
func TestSetCurrentSession_UnknownClient(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	ws, _ := insertTestWorkspace(t, b, "/tmp/current-session-unknown")

	require.ErrorIs(t, b.SetCurrentSession(ws.ID, newClientID(t), "S1"), ErrClientNotAttached)
}

// TestSetCurrentSession_RejectsBadInputs covers the validation
// branches: empty/malformed client_id and unknown workspace.
func TestSetCurrentSession_RejectsBadInputs(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	ws, _ := insertTestWorkspace(t, b, "/tmp/current-session-bad")

	require.ErrorIs(t, b.SetCurrentSession(ws.ID, "", "S1"), ErrInvalidClientID)
	require.ErrorIs(t, b.SetCurrentSession(ws.ID, "not-a-uuid", "S1"), ErrInvalidClientID)

	require.ErrorIs(
		t,
		b.SetCurrentSession("00000000-0000-0000-0000-000000000000", newClientID(t), "S1"),
		ErrWorkspaceNotFound,
	)
}

// TestSetCurrentSession_RaceWithDetach exercises concurrent
// SetCurrentSession updates from one client racing against detach
// on a second client. The final state must be self-consistent: any
// remaining clientState entries reflect a coherent
// (streams, currentSessionID) pair.
func TestSetCurrentSession_RaceWithDetach(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	ws, _ := insertTestWorkspace(t, b, "/tmp/current-session-race")

	cidA := newClientID(t)
	cidB := newClientID(t)
	require.NoError(t, b.AttachClient(ws.ID, cidA))
	require.NoError(t, b.AttachClient(ws.ID, cidB))

	var wg sync.WaitGroup
	const updates = 200
	wg.Add(3)
	go func() {
		defer wg.Done()
		for i := range updates {
			// Errors are tolerated: once cidA detaches,
			// further updates against cidA must return
			// ErrClientNotAttached but never panic.
			_ = b.SetCurrentSession(ws.ID, cidA, "SA")
			_ = i
		}
	}()
	go func() {
		defer wg.Done()
		for i := range updates {
			_ = b.SetCurrentSession(ws.ID, cidB, "SB")
			_ = i
		}
	}()
	go func() {
		defer wg.Done()
		// Single concurrent detach of cidA partway through.
		b.DetachClient(ws.ID, cidA)
	}()
	wg.Wait()

	ws.clientsMu.Lock()
	defer ws.clientsMu.Unlock()
	require.NotContains(t, ws.clients, cidA, "detached client must be gone")
	require.Contains(t, ws.clients, cidB, "remaining client must still be present")
	require.Equal(t, "SB", ws.clients[cidB].currentSessionID, "remaining client must keep its last set session")
}

// TestAttachedClients_BasicLifecycle walks one session's count through
// attach -> set -> second client joins -> switch -> detach. It also
// confirms hold-only and unselected clients do not contribute.
func TestAttachedClients_BasicLifecycle(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	// Keep the grace window long so the hold-only client survives.
	b.createGrace = time.Hour
	ws, _ := insertTestWorkspace(t, b, "/tmp/attached-clients-basic")

	// No clients yet.
	n, err := b.AttachedClients(ws.ID, "S1")
	require.NoError(t, err)
	require.Zero(t, n)

	// Attach A, set to S1. Count for S1 is 1; count for S2 is 0.
	cidA := newClientID(t)
	require.NoError(t, b.AttachClient(ws.ID, cidA))
	require.NoError(t, b.SetCurrentSession(ws.ID, cidA, "S1"))

	n, err = b.AttachedClients(ws.ID, "S1")
	require.NoError(t, err)
	require.Equal(t, 1, n)
	n, err = b.AttachedClients(ws.ID, "S2")
	require.NoError(t, err)
	require.Zero(t, n)

	// Attach B, set to S1. Count for S1 is 2.
	cidB := newClientID(t)
	require.NoError(t, b.AttachClient(ws.ID, cidB))
	require.NoError(t, b.SetCurrentSession(ws.ID, cidB, "S1"))

	n, _ = b.AttachedClients(ws.ID, "S1")
	require.Equal(t, 2, n)

	// B switches to S2; counts redistribute.
	require.NoError(t, b.SetCurrentSession(ws.ID, cidB, "S2"))
	n, _ = b.AttachedClients(ws.ID, "S1")
	require.Equal(t, 1, n)
	n, _ = b.AttachedClients(ws.ID, "S2")
	require.Equal(t, 1, n)

	// A hold-only client must NOT be counted, even if we were able to
	// imagine a currentSessionID on it. registerClient leaves
	// currentSessionID empty by construction, and SetCurrentSession
	// rejects hold-only writers — so the contract holds two ways.
	cidHold := newClientID(t)
	b.registerClient(ws, cidHold)
	t.Cleanup(func() { _ = b.releaseHold(ws.ID, cidHold) })
	n, _ = b.AttachedClients(ws.ID, "S1")
	require.Equal(t, 1, n, "hold-only client must not contribute")
	n, _ = b.AttachedClients(ws.ID, "")
	require.Equal(t, 0, n,
		"empty sessionID must not match the hold-only entry (streams==0)")

	// A client with streams > 0 but currentSessionID == "" is NOT
	// counted toward any non-empty session, and is matched only
	// against the empty session id (which represents the landing
	// screen).
	cidC := newClientID(t)
	require.NoError(t, b.AttachClient(ws.ID, cidC))
	n, _ = b.AttachedClients(ws.ID, "S1")
	require.Equal(t, 1, n, "stream-only client with empty currentSessionID must not be counted toward S1")
	n, _ = b.AttachedClients(ws.ID, "")
	require.Equal(t, 1, n, "stream-only client with empty currentSessionID matches the empty session id")

	// B detaches: count for S2 drops to 0.
	b.DetachClient(ws.ID, cidB)
	n, _ = b.AttachedClients(ws.ID, "S2")
	require.Zero(t, n)
	n, _ = b.AttachedClients(ws.ID, "S1")
	require.Equal(t, 1, n, "A still on S1")

	// Final cleanup.
	b.DetachClient(ws.ID, cidA)
	b.DetachClient(ws.ID, cidC)
}

// TestAttachedClients_UnknownWorkspace verifies the error surface.
func TestAttachedClients_UnknownWorkspace(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	_, err := b.AttachedClients("00000000-0000-0000-0000-000000000000", "S1")
	require.ErrorIs(t, err, ErrWorkspaceNotFound)
}

// TestTeardown_DefersShutdownWhileCreatePending verifies the core of the
// "coder agent offline after more than one session" fix: tearing down the
// last live workspace must NOT shut the server down while another
// CreateWorkspace is mid-flight (it has committed to the slow init path
// but not yet registered its workspace). Without the pending guard, the
// teardown observed an empty workspace map and killed the whole server
// out from under the workspace being born.
func TestTeardown_DefersShutdownWhileCreatePending(t *testing.T) {
	t.Parallel()

	b, serverShutdowns := newTestBackend(t)
	ws, wsShutdowns := insertTestWorkspace(t, b, "/tmp/pending-guard")

	// Simulate a concurrent create that has passed the pending++ point
	// but has not yet registered its workspace.
	b.mu.Lock()
	b.pending = 1
	b.mu.Unlock()

	b.teardown(ws)

	require.Equal(t, int32(1), wsShutdowns.Load(),
		"the torn-down workspace's own resources must still be released")
	require.Equal(t, int32(0), serverShutdowns.Load(),
		"server must not shut down while a create is in flight")
}

// TestTeardown_ShutsDownWhenIdleAndNoPending is the control for the guard
// above: with no create in flight, tearing down the last workspace still
// triggers the server shutdown, preserving the "shut down when empty"
// behavior.
func TestTeardown_ShutsDownWhenIdleAndNoPending(t *testing.T) {
	t.Parallel()

	b, serverShutdowns := newTestBackend(t)
	ws, _ := insertTestWorkspace(t, b, "/tmp/idle-shutdown")

	b.teardown(ws)

	require.Equal(t, int32(1), serverShutdowns.Load(),
		"server must shut down once the last workspace is gone and nothing is pending")
}

// TestCreateWorkspace_PendingBalancedOnSuccess drives the real
// CreateWorkspace path and asserts the in-flight `pending` counter it
// uses to hold off server shutdown is balanced back to zero once the
// workspace is registered. This guards the bookkeeping the unit-level
// teardown tests stub out: a refactor that drops the increment or
// misplaces the deferred decrement would reintroduce the shutdown race.
func TestCreateWorkspace_PendingBalancedOnSuccess(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	b, shutdownCount := newTestBackend(t)
	// Keep the create hold alive so its grace timer can't fire and tear
	// the workspace down mid-assertion.
	b.SetCreateGrace(time.Hour)
	t.Cleanup(func() { drainBackend(t, b) })

	_, _, err := b.CreateWorkspace(protoWS(t.TempDir(), t.TempDir(), uuid.New().String()))
	require.NoError(t, err)

	b.mu.Lock()
	pending := b.pending
	b.mu.Unlock()
	require.Equal(t, 0, pending, "pending must return to zero after a successful create")
	require.Equal(t, int32(0), shutdownCount.Load(), "a live workspace must not trigger shutdown")
}

// TestCreateWorkspace_PendingBalancedAndReapsOnFailure asserts that a
// create which fails during its slow init still decrements `pending`, and
// that the deferred reap shuts an otherwise-idle server down (the safety
// net that keeps a failed create racing the last teardown from leaking an
// empty server the plain teardown declined to close).
func TestCreateWorkspace_PendingBalancedAndReapsOnFailure(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	b, shutdownCount := newTestBackend(t)

	// A data dir whose parent is a regular file makes the workspace's
	// data-directory creation fail deterministically, after the pending
	// counter has been incremented.
	tmp := t.TempDir()
	fileAsParent := filepath.Join(tmp, "notadir")
	require.NoError(t, os.WriteFile(fileAsParent, []byte("x"), 0o600))
	badDataDir := filepath.Join(fileAsParent, "data")

	_, _, err := b.CreateWorkspace(protoWS(tmp, badDataDir, uuid.New().String()))
	require.Error(t, err, "create must fail when the data dir cannot be created")

	b.mu.Lock()
	pending := b.pending
	remaining := b.workspaces.Len()
	b.mu.Unlock()
	require.Equal(t, 0, pending, "pending must return to zero after a failed create")
	require.Zero(t, remaining, "no workspace should be registered after a failed create")
	require.Equal(t, int32(1), shutdownCount.Load(),
		"a failed create that leaves the server idle must reap it")
}
