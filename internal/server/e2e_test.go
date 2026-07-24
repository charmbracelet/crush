package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/backend"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// e2eHarness wires a Server, its Backend (with a custom shutdownFn we
// can observe), an httptest.NewServer, and a synthetic Workspace whose
// embedded App has a live event broker. It is the minimum scaffolding
// the multi-client end-to-end scenarios in PLAN item 6 need.
type e2eHarness struct {
	httpSrv     *httptest.Server
	srv         *Server
	backend     *backend.Backend
	workspace   *backend.Workspace
	app         *app.App
	shutdownHit atomic.Bool

	// sseWG tracks every SSE reader goroutine spawned by
	// [e2eHarness.subscribeSSE]. The harness's cleanup hook waits on
	// it after the httptest server has been closed so that the test
	// cannot leave behind background readers (and therefore unclosed
	// response bodies) after returning.
	sseWG sync.WaitGroup
}

// installServer attaches a fresh Server (with a custom shutdown
// callback that flips [e2eHarness.shutdownHit]) wrapped in an
// [httptest.Server] onto h. It registers the cleanup hooks for the
// httptest server and the SSE reader WaitGroup in the order required
// by the LIFO contract documented on [newE2EHarness].
//
// Callers that want a fully synthetic workspace use [newE2EHarness];
// callers that want to drive the real CreateWorkspace HTTP path use
// [newRealCreateHarness] and then [e2eHarness.postWorkspace].
func (h *e2eHarness) installServer(t *testing.T) {
	t.Helper()
	srv := &Server{}
	srv.backend = backend.New(context.Background(), nil, func() {
		h.shutdownHit.Store(true)
	})
	srv.installHandler()

	hs := httptest.NewServer(srv.Handler())
	// Order matters: t.Cleanup is LIFO and the test's own per-
	// stream cancels (cancelA/cancelB) run first. After those, we
	// want hs.Close to fire first (so any handler still parked in
	// its `select` returns), THEN sseWG.Wait so every reader
	// goroutine exits and closes its response body. Any caller-
	// owned cleanups registered *before* installServer (e.g. App
	// teardown for the synthetic harness) therefore run LAST,
	// after the readers have drained.
	t.Cleanup(h.sseWG.Wait)
	t.Cleanup(hs.Close)

	h.httpSrv = hs
	h.srv = srv
	h.backend = srv.backend
}

// newE2EHarness builds an in-process server + a synthetic Workspace
// whose embedded App is a real [app.App] constructed via
// [app.NewForTest], so its event broker delivers everything the SSE
// pipeline expects. Used by the scenarios that do not need to
// exercise the path-dedupe behavior of [backend.CreateWorkspace].
//
// Cleanup tears down the App's broker only after sseWG.Wait and
// hs.Close have run, so SSE readers cannot observe a dead broker.
func newE2EHarness(t *testing.T) *e2eHarness {
	t.Helper()

	h := &e2eHarness{}

	// Register the App teardown FIRST so LIFO order puts it AFTER
	// the cleanups that installServer registers below (hs.Close +
	// sseWG.Wait).
	appCtx, cancel := context.WithCancel(context.Background())
	a := app.NewForTest(appCtx)
	t.Cleanup(func() {
		cancel()
		a.ShutdownForTest()
	})

	h.installServer(t)

	ws := &backend.Workspace{
		ID:   uuid.New().String(),
		Path: t.TempDir(),
		App:  a,
	}
	// Synthetic workspaces have an incomplete App; bypass the
	// default teardown so the "last workspace removed" path can run
	// without panicking inside [app.App.Shutdown].
	backend.SetWorkspaceShutdownFnForTest(ws, func() {})
	backend.InsertWorkspaceForTest(h.backend, ws)

	h.workspace = ws
	h.app = a
	return h
}

// newRealCreateHarness builds an in-process server WITHOUT any
// pre-inserted workspace, intended for tests that drive the real
// [backend.CreateWorkspace] HTTP path (path-dedupe scenario). It
// isolates HOME/XDG_* via [t.Setenv] so [config.Init] doesn't read
// the host machine's config, which means callers MUST NOT mark the
// test as parallel.
func newRealCreateHarness(t *testing.T) *e2eHarness {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	h := &e2eHarness{}
	h.installServer(t)
	return h
}

// postWorkspace drives the real POST /v1/workspaces handler and
// returns the resolved workspace proto. This is how scenario 1
// exercises the path-dedupe behavior from PLAN item 1: two calls
// with the same Path and distinct ClientIDs must return the same
// workspace ID.
func (h *e2eHarness) postWorkspace(t *testing.T, args proto.Workspace) proto.Workspace {
	t.Helper()
	body, err := json.Marshal(args)
	require.NoError(t, err)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		h.httpSrv.URL+"/v1/workspaces", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.httpSrv.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "POST /v1/workspaces must succeed")
	var out proto.Workspace
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	require.NotEmpty(t, out.ID, "server must return a workspace id")
	return out
}

// subscribeSSE opens an SSE stream against the test server for the
// given workspace and client ID. It returns a channel of decoded
// envelopes plus a cancel function that closes the stream. The
// returned channel is closed when the stream ends.
func (h *e2eHarness) subscribeSSE(t *testing.T, ctx context.Context, workspaceID, clientID string) (<-chan any, context.CancelFunc) {
	t.Helper()
	streamCtx, cancel := context.WithCancel(ctx)

	q := url.Values{"client_id": []string{clientID}}
	reqURL := h.httpSrv.URL + "/v1/workspaces/" + workspaceID + "/events?" + q.Encode()
	req, err := http.NewRequestWithContext(streamCtx, http.MethodGet, reqURL, nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := h.httpSrv.Client().Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "SSE subscribe should return 200")

	out := make(chan any, 64)
	h.sseWG.Go(func() {
		defer resp.Body.Close()
		defer close(out)
		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				return
			}
			line = bytes.TrimSpace(line)
			if len(line) == 0 {
				continue
			}
			data, ok := bytes.CutPrefix(line, []byte("data:"))
			if !ok {
				continue
			}
			data = bytes.TrimSpace(data)
			var p pubsub.Payload
			if err := json.Unmarshal(data, &p); err != nil {
				continue
			}
			ev, decoded := decodeSSEEnvelope(p)
			if !decoded {
				continue
			}
			select {
			case out <- ev:
			case <-streamCtx.Done():
				return
			}
		}
	})
	return out, cancel
}

// decodeSSEEnvelope decodes the discriminated SSE envelope into the
// concrete pubsub.Event[proto.X] payload the e2e tests care about.
// Unknown payload types are skipped so tests can match on type
// assertions without worrying about envelope noise.
func decodeSSEEnvelope(p pubsub.Payload) (any, bool) {
	switch p.Type {
	case pubsub.PayloadTypePermissionRequest:
		var e pubsub.Event[proto.PermissionRequest]
		if err := json.Unmarshal(p.Payload, &e); err != nil {
			return nil, false
		}
		return e, true
	case pubsub.PayloadTypePermissionNotification:
		var e pubsub.Event[proto.PermissionNotification]
		if err := json.Unmarshal(p.Payload, &e); err != nil {
			return nil, false
		}
		return e, true
	case pubsub.PayloadTypeQuestionNotification:
		var e pubsub.Event[proto.QuestionNotification]
		if err := json.Unmarshal(p.Payload, &e); err != nil {
			return nil, false
		}
		return e, true
	case pubsub.PayloadTypeMessage:
		var e pubsub.Event[proto.Message]
		if err := json.Unmarshal(p.Payload, &e); err != nil {
			return nil, false
		}
		return e, true
	case pubsub.PayloadTypeAgentEvent:
		var e pubsub.Event[proto.AgentEvent]
		if err := json.Unmarshal(p.Payload, &e); err != nil {
			return nil, false
		}
		return e, true
	case pubsub.PayloadTypeRunComplete:
		var e pubsub.Event[proto.RunComplete]
		if err := json.Unmarshal(p.Payload, &e); err != nil {
			return nil, false
		}
		return e, true
	}
	return nil, false
}

// grantPermission posts a permission grant via the HTTP surface and
// returns the server's "resolved" verdict. Mirrors the client-side
// GrantPermission flow without importing internal/client (which
// would create an import cycle from this in-package test).
func (h *e2eHarness) grantPermission(t *testing.T, ctx context.Context, workspaceID string, req proto.PermissionGrant) bool {
	t.Helper()
	body, err := json.Marshal(req)
	require.NoError(t, err)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		h.httpSrv.URL+"/v1/workspaces/"+workspaceID+"/permissions/grant",
		bytes.NewReader(body))
	require.NoError(t, err)
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := h.httpSrv.Client().Do(httpReq)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var out proto.PermissionGrantResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	return out.Resolved
}

// waitForAttached spins until the workspace's clients map reports at
// least n entries with streams > 0. Catches the race where a test
// publishes events before the server-side AttachClient has completed.
func (h *e2eHarness) waitForAttached(t *testing.T, n int) {
	t.Helper()
	h.waitForAttachedOn(t, h.workspace, n)
}

// waitForAttachedOn is the workspace-explicit form of waitForAttached.
// Tests that drive a workspace whose pointer is not stored on the
// harness (e.g. the real CreateWorkspace path) pass the workspace in.
func (h *e2eHarness) waitForAttachedOn(t *testing.T, ws *backend.Workspace, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if backend.WorkspaceLiveStreamCountForTest(ws) >= n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("expected %d attached streams, have %d", n,
		backend.WorkspaceLiveStreamCountForTest(ws))
}

// drainUntil reads from evc until it sees an event of type T that
// satisfies match, or ctx expires. Returns the matching event and
// ok=true, or the zero value and ok=false on timeout.
func drainUntil[T any](ctx context.Context, evc <-chan any, match func(T) bool) (T, bool) {
	var zero T
	for {
		select {
		case <-ctx.Done():
			return zero, false
		case ev, ok := <-evc:
			if !ok {
				return zero, false
			}
			typed, isT := ev.(T)
			if !isT {
				continue
			}
			if match == nil || match(typed) {
				return typed, true
			}
		}
	}
}

// TestE2E_TwoClientsReceiveSameMessage covers PLAN item 6 scenario 1:
// two clients POST /v1/workspaces with the same Path and observe
// that the server returns a single workspace (path-dedupe from PLAN
// item 1) and that an event published on that workspace fans out to
// both SSE streams.
//
// Cannot run in parallel: it isolates HOME/XDG_* via t.Setenv so
// config.Init does not read the host machine's real config.
func TestE2E_TwoClientsReceiveSameMessage(t *testing.T) {
	h := newRealCreateHarness(t)
	// Shorten the create-grace window so the workspace's pending
	// creation holds release quickly during test cleanup once both
	// SSE streams have been detached.
	h.backend.SetCreateGrace(200 * time.Millisecond)

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	cidA := uuid.New().String()
	cidB := uuid.New().String()

	// Shared workspace path. Two POSTs with this path must
	// deduplicate at the backend's pathIndex and return the same
	// workspace id.
	wsPath := t.TempDir()
	dataDir := t.TempDir()
	args := proto.Workspace{Path: wsPath, DataDir: dataDir}

	argsA := args
	argsA.ClientID = cidA
	wsRespA := h.postWorkspace(t, argsA)

	argsB := args
	argsB.ClientID = cidB
	wsRespB := h.postWorkspace(t, argsB)

	require.Equal(t, wsRespA.ID, wsRespB.ID,
		"POST /v1/workspaces with the same Path must return the same workspace id")

	// Look up the resulting workspace on the backend so the test
	// can publish events through its real [app.App] event broker.
	ws, err := h.backend.GetWorkspace(wsRespA.ID)
	require.NoError(t, err)
	// Override the shutdown callback so test cleanup doesn't run
	// the full app.Shutdown path (which would tear down LSP/MCP
	// resources the test doesn't need to exercise), but still
	// release the pooled DB connection so Windows can clean up
	// the temp data directory.
	wsDataDir := ws.Cfg.Config().Options.DataDirectory
	backend.SetWorkspaceShutdownFnForTest(ws, func() {
		_ = db.Release(wsDataDir)
	})

	evcA, cancelA := h.subscribeSSE(t, ctx, ws.ID, cidA)
	t.Cleanup(cancelA)
	evcB, cancelB := h.subscribeSSE(t, ctx, ws.ID, cidB)
	t.Cleanup(cancelB)

	h.waitForAttachedOn(t, ws, 2)

	const sessionID = "s-e2e-1"
	msg := message.Message{
		ID:        "m-1",
		SessionID: sessionID,
		Role:      message.Assistant,
		Parts:     []message.ContentPart{message.TextContent{Text: "hello multi-client"}},
	}
	ws.SendEvent(pubsub.Event[message.Message]{
		Type:    pubsub.CreatedEvent,
		Payload: msg,
	})

	pickCtx, pickCancel := context.WithTimeout(ctx, 3*time.Second)
	defer pickCancel()
	gotA, okA := drainUntil(pickCtx, evcA, func(e pubsub.Event[proto.Message]) bool {
		return e.Payload.ID == "m-1"
	})
	require.True(t, okA, "client A must receive the MessageEvent")
	require.Equal(t, sessionID, gotA.Payload.SessionID)

	gotB, okB := drainUntil(pickCtx, evcB, func(e pubsub.Event[proto.Message]) bool {
		return e.Payload.ID == "m-1"
	})
	require.True(t, okB, "client B must receive the same MessageEvent")
	require.Equal(t, sessionID, gotB.Payload.SessionID)
}

// TestE2E_PermissionFlowCrossClient covers PLAN item 6 scenario 2:
// a tool-driven permission request is granted by client A; client B
// observes a PermissionNotification; a redundant grant from B
// returns the "already resolved" indicator (resolved=false from the
// bool plumbing landed in item 3).
func TestE2E_PermissionFlowCrossClient(t *testing.T) {
	t.Parallel()
	h := newE2EHarness(t)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	cidA := uuid.New().String()
	cidB := uuid.New().String()

	evcA, cancelA := h.subscribeSSE(t, ctx, h.workspace.ID, cidA)
	t.Cleanup(cancelA)
	evcB, cancelB := h.subscribeSSE(t, ctx, h.workspace.ID, cidB)
	t.Cleanup(cancelB)

	h.waitForAttached(t, 2)

	// Drive the permission request from a goroutine simulating the
	// tool path. Request blocks until resolved; capture the outcome.
	const sessionID = "s-perm"
	const toolCallID = "tc-1"
	type result struct {
		granted bool
		err     error
	}
	done := make(chan result, 1)
	go func() {
		granted, err := h.app.Permissions.Request(ctx, permission.CreatePermissionRequest{
			SessionID:   sessionID,
			ToolCallID:  toolCallID,
			ToolName:    "view",
			Description: "read a file",
			Action:      "read",
			Path:        h.workspace.Path,
		})
		done <- result{granted: granted, err: err}
	}()

	// Wait for the PermissionRequest to arrive on client A's SSE
	// stream. We need its ID to drive the grant.
	pickCtx, pickCancel := context.WithTimeout(ctx, 3*time.Second)
	defer pickCancel()
	reqEv, ok := drainUntil(pickCtx, evcA, func(e pubsub.Event[proto.PermissionRequest]) bool {
		return e.Payload.ToolCallID == toolCallID
	})
	require.True(t, ok, "client A must receive the PermissionRequest")

	// Client A grants — first grant must report resolved=true.
	resolvedA := h.grantPermission(t, ctx, h.workspace.ID, proto.PermissionGrant{
		Permission: reqEv.Payload,
		Action:     proto.PermissionAllow,
	})
	require.True(t, resolvedA, "client A's grant must resolve the pending request")

	// The blocked Request call must now return granted=true.
	select {
	case r := <-done:
		require.NoError(t, r.err)
		require.True(t, r.granted)
	case <-pickCtx.Done():
		t.Fatal("permission Request did not return after grant")
	}

	// Client B must receive a PermissionNotification with
	// Granted=true for the same ToolCallID. The initial neither-
	// granted-nor-denied notification published at the start of
	// Request also lands on B's stream — match on the granted one.
	notif, ok := drainUntil(pickCtx, evcB, func(e pubsub.Event[proto.PermissionNotification]) bool {
		return e.Payload.ToolCallID == toolCallID && e.Payload.Granted
	})
	require.True(t, ok, "client B must receive a granting PermissionNotification")
	require.True(t, notif.Payload.Granted)
	require.False(t, notif.Payload.Denied)

	// A follow-up grant from client B must report resolved=false
	// (the request was already resolved by A).
	resolvedB := h.grantPermission(t, ctx, h.workspace.ID, proto.PermissionGrant{
		Permission: reqEv.Payload,
		Action:     proto.PermissionAllow,
	})
	require.False(t, resolvedB, "client B's follow-up grant must report already resolved")
}

// TestE2E_KillingClientASSEDoesNotBreakClientB covers PLAN item 6
// scenario 3: terminating client A's SSE stream does not affect
// client B's stream; client B continues to receive events.
func TestE2E_KillingClientASSEDoesNotBreakClientB(t *testing.T) {
	t.Parallel()
	h := newE2EHarness(t)
	ctxB, cancelB := context.WithCancel(t.Context())
	t.Cleanup(cancelB)
	ctxA, cancelA := context.WithCancel(t.Context())

	cidA := uuid.New().String()
	cidB := uuid.New().String()

	_, killA := h.subscribeSSE(t, ctxA, h.workspace.ID, cidA)
	t.Cleanup(killA)
	evcB, killB := h.subscribeSSE(t, ctxB, h.workspace.ID, cidB)
	t.Cleanup(killB)

	h.waitForAttached(t, 2)

	// Kill A's stream. The server's deferred DetachClient should
	// drop A's claim, leaving B as the sole attached client.
	cancelA()
	killA()

	require.Eventually(t, func() bool {
		return backend.WorkspaceLiveStreamCountForTest(h.workspace) == 1
	}, 3*time.Second, 10*time.Millisecond,
		"expected client A's stream to drop the attached count to 1")

	// Workspace must still exist (B is holding it open) and
	// shutdown callback must not have fired yet.
	_, err := h.backend.GetWorkspace(h.workspace.ID)
	require.NoError(t, err, "workspace must still exist while B is attached")
	require.False(t, h.shutdownHit.Load(),
		"shutdown callback must not fire while B is still attached")

	// Publish a fresh event; B must still receive it.
	const sessionID = "s-after-a-died"
	msg := message.Message{
		ID:        "m-after",
		SessionID: sessionID,
		Role:      message.Assistant,
		Parts:     []message.ContentPart{message.TextContent{Text: "still alive"}},
	}
	h.app.SendEvent(pubsub.Event[message.Message]{
		Type:    pubsub.CreatedEvent,
		Payload: msg,
	})

	pickCtx, pickCancel := context.WithTimeout(ctxB, 3*time.Second)
	defer pickCancel()
	got, ok := drainUntil(pickCtx, evcB, func(e pubsub.Event[proto.Message]) bool {
		return e.Payload.ID == "m-after"
	})
	require.True(t, ok, "client B must still receive events after A's stream is killed")
	require.Equal(t, sessionID, got.Payload.SessionID)
}

// TestE2E_ShutdownCallbackFiresWhenLastClientLeaves covers PLAN
// item 6 scenario 4: once both clients disconnect, the backend
// runs its "last workspace removed -> server shutdown" path — now
// after the idle-linger window rather than immediately.
func TestE2E_ShutdownCallbackFiresWhenLastClientLeaves(t *testing.T) {
	t.Parallel()
	h := newE2EHarness(t)
	// Shorten the idle linger so the test doesn't wait out the
	// production window; the callback still fires, just after the delay.
	h.backend.SetIdleShutdownDelay(200 * time.Millisecond)

	ctxA, cancelA := context.WithCancel(t.Context())
	ctxB, cancelB := context.WithCancel(t.Context())
	t.Cleanup(cancelA)
	t.Cleanup(cancelB)

	cidA := uuid.New().String()
	cidB := uuid.New().String()
	_, killA := h.subscribeSSE(t, ctxA, h.workspace.ID, cidA)
	t.Cleanup(killA)
	_, killB := h.subscribeSSE(t, ctxB, h.workspace.ID, cidB)
	t.Cleanup(killB)

	h.waitForAttached(t, 2)
	require.False(t, h.shutdownHit.Load(), "shutdown must not fire while clients are attached")

	cancelA()
	killA()
	require.Eventually(t, func() bool {
		return backend.WorkspaceLiveStreamCountForTest(h.workspace) == 1
	}, 3*time.Second, 10*time.Millisecond)
	require.False(t, h.shutdownHit.Load(),
		"shutdown must not fire after only one client disconnects")

	cancelB()
	killB()
	require.Eventually(t, h.shutdownHit.Load,
		3*time.Second, 10*time.Millisecond,
		"shutdown callback must fire once the last client disconnects (after the idle linger)")

	// Workspace must be gone from the index.
	_, err := h.backend.GetWorkspace(h.workspace.ID)
	require.ErrorIs(t, err, backend.ErrWorkspaceNotFound)

	// Subsequent GETs against the now-defunct workspace return
	// 404, confirming the http surface still reflects the teardown.
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet,
		h.httpSrv.URL+"/v1/workspaces/"+h.workspace.ID, nil)
	require.NoError(t, err)
	r, err := h.httpSrv.Client().Do(req)
	require.NoError(t, err)
	_, _ = io.Copy(io.Discard, r.Body)
	r.Body.Close()
	require.Equal(t, http.StatusNotFound, r.StatusCode)
}
