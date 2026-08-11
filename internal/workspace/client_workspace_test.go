package workspace

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/client"
	"github.com/charmbracelet/crush/internal/commands"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/skills"
	"github.com/stretchr/testify/require"
)

// TestProtoToMessageToolResult ensures that ToolResult metadata,
// data, and MIME type survive the conversion from proto on the
// client. Without these fields the TUI cannot render rich tool
// output (e.g. syntax-highlighted code from view, diffs from edit,
// images, etc.) and falls back to the raw LLM-facing string.
func TestProtoToMessageToolResult(t *testing.T) {
	t.Parallel()

	src := proto.Message{
		ID:   "m1",
		Role: proto.Tool,
		Parts: []proto.ContentPart{
			proto.ToolResult{
				ToolCallID: "call-1",
				Name:       "view",
				Content:    "<file>\n  1| hi\n</file>",
				Data:       "base64data",
				MIMEType:   "image/png",
				Metadata:   `{"file_path":"/tmp/x","content":"hi"}`,
				IsError:    false,
			},
		},
	}

	got := protoToMessage(src)
	require.Len(t, got.Parts, 1)
	tr, ok := got.Parts[0].(message.ToolResult)
	require.True(t, ok, "expected message.ToolResult, got %T", got.Parts[0])
	require.Equal(t, "call-1", tr.ToolCallID)
	require.Equal(t, "view", tr.Name)
	require.Equal(t, "<file>\n  1| hi\n</file>", tr.Content)
	require.Equal(t, "base64data", tr.Data)
	require.Equal(t, "image/png", tr.MIMEType)
	require.Equal(t, `{"file_path":"/tmp/x","content":"hi"}`, tr.Metadata)
	require.False(t, tr.IsError)
}

// TestClientWorkspace_PermissionGrantMapping verifies that
// PermissionGrant on the ClientWorkspace serializes a one-time grant
// (proto.PermissionAllow) and PermissionGrantPersistent serializes a
// persistent grant (proto.PermissionAllowForSession). A swap between
// these two would silently flip "allow once" into "remember for the
// session", and vice versa, so we pin the wire mapping here.
func TestClientWorkspace_PermissionGrantMapping(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		call func(*ClientWorkspace, permission.PermissionRequest)
		want proto.PermissionAction
	}{
		{
			name: "Grant -> PermissionAllow",
			call: func(w *ClientWorkspace, p permission.PermissionRequest) {
				w.PermissionGrant(p)
			},
			want: proto.PermissionAllow,
		},
		{
			name: "GrantPersistent -> PermissionAllowForSession",
			call: func(w *ClientWorkspace, p permission.PermissionRequest) {
				w.PermissionGrantPersistent(p)
			},
			want: proto.PermissionAllowForSession,
		},
		{
			name: "Deny -> PermissionDeny",
			call: func(w *ClientWorkspace, p permission.PermissionRequest) {
				w.PermissionDeny(p)
			},
			want: proto.PermissionDeny,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var got proto.PermissionGrant
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodPost, r.Method)
				require.Equal(t, "/v1/workspaces/ws-1/permissions/grant", r.URL.Path)
				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				require.NoError(t, json.Unmarshal(body, &got))
				require.NoError(t, json.NewEncoder(w).Encode(proto.PermissionGrantResponse{Resolved: true}))
			}))
			defer srv.Close()

			u, err := url.Parse(srv.URL)
			require.NoError(t, err)
			c, err := client.NewClient(t.TempDir(), "tcp", u.Host)
			require.NoError(t, err)

			ws := NewClientWorkspace(c, proto.Workspace{ID: "ws-1"})

			perm := permission.PermissionRequest{
				ID:          "req-1",
				SessionID:   "sess-1",
				ToolCallID:  "tc-1",
				ToolName:    "tool",
				Description: "do thing",
				Action:      "act",
				Path:        "/tmp/p",
			}
			tc.call(ws, perm)

			require.Equal(t, tc.want, got.Action)
			require.Equal(t, "req-1", got.Permission.ID)
			require.Equal(t, "sess-1", got.Permission.SessionID)
			require.Equal(t, "tc-1", got.Permission.ToolCallID)
			require.Equal(t, "tool", got.Permission.ToolName)
			require.Equal(t, "act", got.Permission.Action)
			require.Equal(t, "/tmp/p", got.Permission.Path)
		})
	}
}

// TestProtoToSkillStates verifies that the wire representation of skill
// discovery states reconstructs identical values on the client,
// including synthetic errors derived from Error strings.
func TestProtoToSkillStates(t *testing.T) {
	t.Parallel()

	in := []proto.SkillState{
		{Name: "ok", Path: "/p/ok", State: proto.SkillStateNormal},
		{Name: "broken", Path: "/p/broken", State: proto.SkillStateError, Error: "bad frontmatter"},
	}

	got := protoToSkillStates(in)
	require.Len(t, got, 2)
	require.Equal(t, "ok", got[0].Name)
	require.Equal(t, skills.StateNormal, got[0].State)
	require.NoError(t, got[0].Err)
	require.Equal(t, "broken", got[1].Name)
	require.Equal(t, skills.StateError, got[1].State)
	require.EqualError(t, got[1].Err, "bad frontmatter")
}

// TestTranslateEvent_Skills verifies that an incoming proto.SkillsEvent
// is converted into pubsub.Event[skills.Event] and that the
// client-process skill cache is updated as a side effect, so callers
// reading skills.GetLatestStates see fresh data after each delta.
func TestTranslateEvent_Skills(t *testing.T) {
	// Not parallel - touches the package-level skills cache via the
	// manager constructed with WithGlobalMirror.
	prev := skills.GetLatestStates()
	t.Cleanup(func() { skills.SetLatestStates(prev) })

	skills.SetLatestStates(nil)

	w := NewClientWorkspace(nil, proto.Workspace{})
	ev := pubsub.Event[proto.SkillsEvent]{
		Type: pubsub.UpdatedEvent,
		Payload: proto.SkillsEvent{
			States: []proto.SkillState{
				{Name: "from-server", Path: "/p", State: proto.SkillStateNormal},
			},
		},
	}

	out := w.translateEvent(ev)
	got, ok := out.(pubsub.Event[skills.Event])
	require.True(t, ok, "expected pubsub.Event[skills.Event], got %T", out)
	require.Len(t, got.Payload.States, 1)
	require.Equal(t, "from-server", got.Payload.States[0].Name)

	// Manager (with WithGlobalMirror) propagated to the package cache.
	cached := skills.GetLatestStates()
	require.Len(t, cached, 1)
	require.Equal(t, "from-server", cached[0].Name)
}

// TestNewClientWorkspace_SeedsSkillsCache verifies that the snapshot in
// proto.Workspace.Skills populates the package-level cache the TUI
// reads at construction time, eliminating the race between TUI startup
// and the first SSE event.
func TestNewClientWorkspace_SeedsSkillsCache(t *testing.T) {
	// Not parallel - touches the package-level skills cache.
	prev := skills.GetLatestStates()
	t.Cleanup(func() { skills.SetLatestStates(prev) })

	skills.SetLatestStates(nil)

	_ = NewClientWorkspace(nil, proto.Workspace{
		Skills: []proto.SkillState{
			{Name: "seeded", Path: "/p", State: proto.SkillStateNormal},
		},
	})

	got := skills.GetLatestStates()
	require.Len(t, got, 1)
	require.Equal(t, "seeded", got[0].Name)
}

// TestTranslateEvent_UpdateAvailable verifies that an incoming
// proto.UpdateAvailable event is converted back into the
// app.UpdateAvailableMsg that the TUI expects, so client/server mode
// shows the same update notification as local mode.
func TestTranslateEvent_UpdateAvailable(t *testing.T) {
	t.Parallel()

	w := NewClientWorkspace(nil, proto.Workspace{})
	ev := pubsub.Event[proto.UpdateAvailable]{
		Type: pubsub.UpdatedEvent,
		Payload: proto.UpdateAvailable{
			CurrentVersion: "1.0.0",
			LatestVersion:  "1.1.0",
			IsDevelopment:  true,
		},
	}

	out := w.translateEvent(ev)
	got, ok := out.(app.UpdateAvailableMsg)
	require.True(t, ok, "expected app.UpdateAvailableMsg, got %T", out)
	require.Equal(t, "1.0.0", got.CurrentVersion)
	require.Equal(t, "1.1.0", got.LatestVersion)
	require.True(t, got.IsDevelopment)
}

func TestClientWorkspaceListMCPPrompts(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/workspaces/ws-1/mcp/prompts", r.URL.Path)
		require.NoError(t, json.NewEncoder(w).Encode([]proto.MCPPrompt{
			{
				ID:       "server:review",
				PromptID: "review",
				ClientID: "server",
				Arguments: []proto.MCPPromptArgument{
					{ID: "focus", Title: "Focus", Required: true},
				},
			},
		}))
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	c, err := client.NewClient(t.TempDir(), "tcp", u.Host)
	require.NoError(t, err)
	workspace := NewClientWorkspace(c, proto.Workspace{ID: "ws-1"})

	got, err := workspace.ListMCPPrompts(t.Context())
	require.NoError(t, err)
	require.Equal(t, []commands.MCPPrompt{
		{
			ID:       "server:review",
			PromptID: "review",
			ClientID: "server",
			Arguments: []commands.Argument{
				{ID: "focus", Title: "Focus", Required: true},
			},
		},
	}, got)
}

func TestClientWorkspaceListMCPPromptsServerError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	c, err := client.NewClient(t.TempDir(), "tcp", u.Host)
	require.NoError(t, err)
	workspace := NewClientWorkspace(c, proto.Workspace{ID: "ws-1"})

	_, err = workspace.ListMCPPrompts(t.Context())
	require.Error(t, err)
	require.Contains(t, err.Error(), "status code 500")
}

// TestClientWorkspace_ReconnectsOnStreamDrop verifies that the event
// subscription loop reconnects after the SSE stream drops instead of
// leaving the TUI permanently orphaned (which surfaced as a stuck
// "coder agent is offline"), and that Shutdown stops the loop.
func TestClientWorkspace_ReconnectsOnStreamDrop(t *testing.T) {
	// Shrink the backoff so several reconnects happen quickly.
	origInitial, origMax := sseReconnectInitialBackoff, sseReconnectMaxBackoff
	sseReconnectInitialBackoff = 5 * time.Millisecond
	sseReconnectMaxBackoff = 20 * time.Millisecond
	t.Cleanup(func() {
		sseReconnectInitialBackoff = origInitial
		sseReconnectMaxBackoff = origMax
	})

	var subscribes atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/events") {
			// Any other bookkeeping call (e.g. GetWorkspace) just
			// gets an empty OK; the test only cares about the stream.
			w.WriteHeader(http.StatusOK)
			return
		}
		subscribes.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		// Drop the stream immediately so the client must reconnect.
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	c, err := client.NewClient(t.TempDir(), "tcp", u.Host)
	require.NoError(t, err)

	ws := NewClientWorkspace(c, proto.Workspace{ID: "ws-1"})

	done := make(chan struct{})
	go func() {
		ws.runSubscription(func(tea.Msg) {})
		close(done)
	}()

	// The loop must reconnect several times as the server keeps
	// dropping the stream.
	require.Eventually(t, func() bool { return subscribes.Load() >= 3 },
		2*time.Second, 5*time.Millisecond,
		"subscription loop should reconnect after the stream drops")

	// Shutdown cancels the subscription context; the loop must return.
	ws.Shutdown()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runSubscription did not return after Shutdown")
	}
}

// TestClientWorkspace_SubscriptionStopsWhenServerDown verifies the
// reconnect loop does not spin forever after Shutdown even when it can
// never connect (server unreachable).
func TestClientWorkspace_SubscriptionStopsWhenServerDown(t *testing.T) {
	origInitial, origMax := sseReconnectInitialBackoff, sseReconnectMaxBackoff
	sseReconnectInitialBackoff = 5 * time.Millisecond
	sseReconnectMaxBackoff = 20 * time.Millisecond
	t.Cleanup(func() {
		sseReconnectInitialBackoff = origInitial
		sseReconnectMaxBackoff = origMax
	})

	// Port 1 is not listening: SubscribeEvents fails on every attempt.
	c, err := client.NewClient(t.TempDir(), "tcp", "127.0.0.1:1")
	require.NoError(t, err)
	ws := NewClientWorkspace(c, proto.Workspace{ID: "ws-1"})

	done := make(chan struct{})
	go func() {
		ws.runSubscription(func(tea.Msg) {})
		close(done)
	}()

	// Let it retry a few times, then shut down.
	time.Sleep(30 * time.Millisecond)
	ws.Shutdown()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runSubscription did not return after Shutdown while server was down")
	}
}

// TestClientWorkspace_AgentReadyErr distinguishes a server that reports
// an uninitialized agent from a server that cannot be reached, so the UI
// can show an actionable message instead of a blanket "agent offline".
func TestClientWorkspace_AgentReadyErr(t *testing.T) {
	t.Parallel()

	t.Run("ready", func(t *testing.T) {
		t.Parallel()
		ws := agentInfoWorkspace(t, proto.AgentInfo{IsReady: true})
		require.NoError(t, ws.AgentReadyErr())
		require.True(t, ws.AgentIsReady())
	})

	t.Run("not initialized", func(t *testing.T) {
		t.Parallel()
		ws := agentInfoWorkspace(t, proto.AgentInfo{IsReady: false})
		err := ws.AgentReadyErr()
		require.ErrorIs(t, err, ErrAgentNotInitialized)
		require.NotErrorIs(t, err, ErrServerUnreachable)
		require.False(t, ws.AgentIsReady())
	})

	t.Run("server unreachable", func(t *testing.T) {
		t.Parallel()
		c, err := client.NewClient(t.TempDir(), "tcp", "127.0.0.1:1")
		require.NoError(t, err)
		ws := NewClientWorkspace(c, proto.Workspace{ID: "ws-1"})
		readyErr := ws.AgentReadyErr()
		require.ErrorIs(t, readyErr, ErrServerUnreachable)
		require.NotErrorIs(t, readyErr, ErrAgentNotInitialized)
		require.False(t, ws.AgentIsReady())
	})
}

// agentInfoWorkspace returns a ClientWorkspace whose server answers the
// agent-info endpoint with the given info.
func agentInfoWorkspace(t *testing.T, info proto.AgentInfo) *ClientWorkspace {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/workspaces/ws-1/agent", r.URL.Path)
		require.NoError(t, json.NewEncoder(w).Encode(info))
	}))
	t.Cleanup(srv.Close)

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	c, err := client.NewClient(t.TempDir(), "tcp", u.Host)
	require.NoError(t, err)
	return NewClientWorkspace(c, proto.Workspace{ID: "ws-1"})
}

// -- Workspace recovery --
//
// The bug these cover: the reconnect loop retried SubscribeEvents with
// the workspace ID captured at startup, forever. Once the server no
// longer knew that ID — because the workspace was torn down under the
// client, or the server was replaced — every retry and every other
// request answered 404 permanently, and a sibling session kept the
// server (and its 404s) alive.

// recoveryServer is a scripted server for the recovery tests. It 404s
// event subscriptions for stale workspace IDs, mints a new ID on create,
// and records what the client did.
type recoveryServer struct {
	mu sync.Mutex
	// liveID is the only workspace ID whose event stream is served.
	liveID string
	// nextID names the workspace the next create hands back.
	nextID string
	// createErr, when set, fails creates instead of serving them.
	createErr func() int
	// hangUpOnCreate drops the connection mid-response so the client
	// never learns the ID of a workspace the server did register.
	hangUpOnCreate bool

	creates      int
	streams      int
	sessionPosts []string
	retired      []string
	deleted      []string
}

func (s *recoveryServer) start(t *testing.T) *client.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/workspaces":
			s.creates++
			if s.createErr != nil {
				http.Error(w, "no", s.createErr())
				return
			}
			if s.hangUpOnCreate {
				// Register the workspace, then hang up before the client
				// can read the ID. This is the case no amount of client
				// bookkeeping can name.
				s.liveID = s.nextID
				if hj, ok := w.(http.Hijacker); ok {
					conn, _, err := hj.Hijack()
					require.NoError(t, err)
					_ = conn.Close()
				}
				return
			}
			s.liveID = s.nextID
			require.NoError(t, json.NewEncoder(w).Encode(proto.Workspace{
				ID: s.liveID, Path: "/tmp/recover",
			}))
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/clients/"):
			s.retired = append(s.retired, strings.TrimPrefix(r.URL.Path, "/v1/clients/"))
			s.liveID = ""
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/workspaces/"):
			s.deleted = append(s.deleted, strings.TrimPrefix(r.URL.Path, "/v1/workspaces/"))
		case strings.HasSuffix(r.URL.Path, "/current-session"):
			id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/workspaces/"), "/current-session")
			if id != s.liveID {
				http.Error(w, "workspace not found", http.StatusNotFound)
				return
			}
			var req proto.CurrentSession
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			s.sessionPosts = append(s.sessionPosts, req.SessionID)
		case strings.HasSuffix(r.URL.Path, "/events"):
			id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/workspaces/"), "/events")
			if id != s.liveID {
				http.Error(w, "workspace not found", http.StatusNotFound)
				return
			}
			s.streams++
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			// Drop the stream so the loop keeps cycling.
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	t.Cleanup(srv.Close)

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	c, err := client.NewClient(t.TempDir(), "tcp", u.Host)
	require.NoError(t, err)
	return c
}

func (s *recoveryServer) snapshot(f func(*recoveryServer)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f(s)
}

// connectionRecorder collects the ConnectionEvents the subscription loop
// reports to the UI.
type connectionRecorder struct {
	mu     sync.Mutex
	events []ConnectionEvent
}

func (r *connectionRecorder) send(msg tea.Msg) {
	ev, ok := msg.(ConnectionEvent)
	if !ok {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, ev)
}

func (r *connectionRecorder) states() []ConnectionState {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]ConnectionState, len(r.events))
	for i, ev := range r.events {
		out[i] = ev.State
	}
	return out
}

func (r *connectionRecorder) sawStuck() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, ev := range r.events {
		if ev.Stuck {
			return true
		}
	}
	return false
}

// TestClientWorkspace_RecoversFromWorkspaceGone is the headline client
// regression: a 404 for the cached workspace ID must make the client
// re-register, adopt the new ID, re-assert the session it was viewing,
// and tell the UI to resync — not retry a dead ID forever.
func TestClientWorkspace_RecoversFromWorkspaceGone(t *testing.T) {
	t.Cleanup(SetSSEBackoffForTest(time.Millisecond, 5*time.Millisecond))

	srv := &recoveryServer{liveID: "", nextID: "ws-2"}
	c := srv.start(t)

	ws := NewClientWorkspace(c, proto.Workspace{ID: "ws-1", Path: "/tmp/recover"})
	// The client was viewing a session, so recovery has to restore that
	// selection: the server's presence entry died with the workspace.
	ws.mu.Lock()
	ws.lastSession = "sess-1"
	ws.mu.Unlock()

	rec := &connectionRecorder{}
	done := make(chan struct{})
	go func() {
		ws.runSubscription(rec.send)
		close(done)
	}()

	require.Eventually(t, func() bool { return ws.workspaceID() == "ws-2" },
		3*time.Second, 5*time.Millisecond,
		"a 404 must make the client re-register instead of retrying a dead ID")
	require.Eventually(t, func() bool {
		var posts []string
		srv.snapshot(func(s *recoveryServer) { posts = append(posts, s.sessionPosts...) })
		return len(posts) > 0 && posts[0] == "sess-1"
	}, 3*time.Second, 5*time.Millisecond,
		"the recovered workspace must be told which session the client is on")

	require.Eventually(t, func() bool {
		states := rec.states()
		var degraded, recovered bool
		for _, s := range states {
			degraded = degraded || s == ConnectionDegraded
			recovered = recovered || s == ConnectionRecovered
		}
		return degraded && recovered
	}, 3*time.Second, 5*time.Millisecond,
		"the UI must be told to resync after the workspace was re-created")

	ws.Shutdown()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("runSubscription did not return after Shutdown")
	}
}

// TestClientWorkspace_ResyncsAfterPlainStreamDrop covers the quieter
// failure: the stream closes and the very next subscribe succeeds. That
// used to be treated as a non-event, so the TUI silently showed state
// frozen at the moment of the drop — everything published while the
// client was away is gone, and the server had dropped its presence entry.
func TestClientWorkspace_ResyncsAfterPlainStreamDrop(t *testing.T) {
	t.Cleanup(SetSSEBackoffForTest(time.Millisecond, 5*time.Millisecond))

	srv := &recoveryServer{liveID: "ws-1"}
	c := srv.start(t)

	ws := NewClientWorkspace(c, proto.Workspace{ID: "ws-1", Path: "/tmp/blip"})
	ws.mu.Lock()
	ws.lastSession = "sess-9"
	ws.mu.Unlock()

	rec := &connectionRecorder{}
	done := make(chan struct{})
	go func() {
		ws.runSubscription(rec.send)
		close(done)
	}()

	require.Eventually(t, func() bool {
		states := rec.states()
		var degraded, recovered bool
		for _, s := range states {
			degraded = degraded || s == ConnectionDegraded
			recovered = recovered || s == ConnectionRecovered
		}
		return degraded && recovered
	}, 3*time.Second, 5*time.Millisecond,
		"a reconnect that succeeds first try must still resync")

	var posts int
	var creates int
	srv.snapshot(func(s *recoveryServer) { posts, creates = len(s.sessionPosts), s.creates })
	require.Positive(t, posts, "the session selection must be re-asserted after the drop")
	require.Zero(t, creates, "a live workspace must not be re-created")
	require.Equal(t, "ws-1", ws.workspaceID())

	ws.Shutdown()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("runSubscription did not return after Shutdown")
	}
}

// TestClientWorkspace_EscalatesUnrecoverableConnection checks the loop
// keeps trying when re-registration itself keeps failing, and escalates
// the UI notice rather than stopping: a hard stop would strand a user
// whose server comes back a minute later.
func TestClientWorkspace_EscalatesUnrecoverableConnection(t *testing.T) {
	t.Cleanup(SetSSEBackoffForTest(time.Millisecond, 2*time.Millisecond))

	srv := &recoveryServer{createErr: func() int { return http.StatusInternalServerError }}
	c := srv.start(t)

	ws := NewClientWorkspace(c, proto.Workspace{ID: "ws-1", Path: "/tmp/hopeless"})
	rec := &connectionRecorder{}
	done := make(chan struct{})
	go func() {
		ws.runSubscription(rec.send)
		close(done)
	}()

	require.Eventually(t, rec.sawStuck, 5*time.Second, 5*time.Millisecond,
		"repeated recovery failures must escalate to a persistent notice")
	var creates int
	srv.snapshot(func(s *recoveryServer) { creates = s.creates })
	require.GreaterOrEqual(t, creates, maxRecoveryEscalate,
		"the loop must keep retrying rather than giving up")

	ws.Shutdown()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("runSubscription did not return after Shutdown")
	}
}

// TestClientWorkspace_ShutdownRetiresAfterLostCreateResponse is the
// quit-during-recovery case, in its nastiest form: the client asks for a
// replacement workspace and the connection dies before it can read the
// ID. The server registered the workspace anyway, so the client is
// holding a claim it cannot name. Retiring the client is what makes
// teardown exact here — there is nothing to guess and nothing to scan.
func TestClientWorkspace_ShutdownRetiresAfterLostCreateResponse(t *testing.T) {
	t.Cleanup(SetSSEBackoffForTest(time.Millisecond, 5*time.Millisecond))

	srv := &recoveryServer{nextID: "ws-orphan", hangUpOnCreate: true}
	c := srv.start(t)

	ws := NewClientWorkspace(c, proto.Workspace{ID: "ws-1", Path: "/tmp/recover"})
	done := make(chan struct{})
	go func() {
		ws.runSubscription(func(tea.Msg) {})
		close(done)
	}()

	// Wait until the server has registered a workspace the client never
	// learned about: that is the state Shutdown has to clean up.
	require.Eventually(t, func() bool {
		var live string
		srv.snapshot(func(s *recoveryServer) { live = s.liveID })
		return live == "ws-orphan"
	}, 3*time.Second, 5*time.Millisecond)
	require.NotEqual(t, "ws-orphan", ws.workspaceID(),
		"the client must genuinely not know the workspace's ID")

	ws.Shutdown()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("runSubscription did not return after Shutdown")
	}

	srv.snapshot(func(s *recoveryServer) {
		require.Equal(t, []string{c.ClientID()}, s.retired,
			"Shutdown must retire the client, which releases claims it cannot name")
		require.Empty(t, s.liveID, "no workspace may be left holding the server open")
	})
}

// TestClientWorkspace_ShutdownFallsBackForLegacyServer keeps quits working
// against a server that predates client retirement: a 404 for the retire
// endpoint must fall back to releasing the workspace by ID, not be
// reported as a failed teardown.
func TestClientWorkspace_ShutdownFallsBackForLegacyServer(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var deleted []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case strings.HasPrefix(r.URL.Path, "/v1/clients/"):
			http.NotFound(w, r)
		case r.Method == http.MethodDelete:
			deleted = append(deleted, strings.TrimPrefix(r.URL.Path, "/v1/workspaces/"))
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	t.Cleanup(srv.Close)

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	c, err := client.NewClient(t.TempDir(), "tcp", u.Host)
	require.NoError(t, err)

	NewClientWorkspace(c, proto.Workspace{ID: "ws-1"}).Shutdown()

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, []string{"ws-1"}, deleted)
}

// TestClientWorkspace_AgentReadyErr_WorkspaceGone checks the status the
// UI is given while recovery runs. A 404 from a live server used to print
// "lost connection to the crush server: ... status code 404", which is
// both wrong and unactionable.
func TestClientWorkspace_AgentReadyErr_WorkspaceGone(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	c, err := client.NewClient(t.TempDir(), "tcp", u.Host)
	require.NoError(t, err)

	ws := NewClientWorkspace(c, proto.Workspace{ID: "ws-1"})
	readyErr := ws.AgentReadyErr()
	require.ErrorIs(t, readyErr, ErrWorkspaceGone)
	require.NotErrorIs(t, readyErr, ErrServerUnreachable)
	require.NotErrorIs(t, readyErr, ErrAgentNotInitialized)
}

// TestClientWorkspace_ShutdownWaitsForInFlightRecovery pins the ordering
// requirement: quitting must stop recovery before saying goodbye to the
// server. Cancelling alone does not unwind a create that is already in
// flight, so a goodbye sent first would be followed by the workspace the
// create went on to register — an orphan keeping the server alive and
// blocking the next upgrade.
func TestClientWorkspace_ShutdownWaitsForInFlightRecovery(t *testing.T) {
	t.Cleanup(SetSSEBackoffForTest(time.Millisecond, 5*time.Millisecond))

	var mu sync.Mutex
	var ops []string
	creating := make(chan struct{})
	var once sync.Once
	live := ""

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/workspaces":
			once.Do(func() { close(creating) })
			// Slow enough that Shutdown lands mid-create.
			//nolint:forbidigo // The overlap is the point of the test.
			time.Sleep(300 * time.Millisecond)
			mu.Lock()
			ops = append(ops, "create")
			live = "ws-2"
			mu.Unlock()
			require.NoError(t, json.NewEncoder(w).Encode(proto.Workspace{ID: "ws-2"}))
		case strings.HasPrefix(r.URL.Path, "/v1/clients/"):
			mu.Lock()
			ops = append(ops, "retire")
			live = ""
			mu.Unlock()
		case strings.HasSuffix(r.URL.Path, "/events"):
			mu.Lock()
			known := live == strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/workspaces/"), "/events")
			mu.Unlock()
			if !known {
				http.Error(w, "workspace not found", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	t.Cleanup(srv.Close)

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	c, err := client.NewClient(t.TempDir(), "tcp", u.Host)
	require.NoError(t, err)

	ws := NewClientWorkspace(c, proto.Workspace{ID: "ws-1", Path: "/tmp/quit-mid-recovery"})
	done := make(chan struct{})
	go func() {
		ws.runSubscription(func(tea.Msg) {})
		close(done)
	}()

	<-creating
	ws.Shutdown()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("runSubscription did not return after Shutdown")
	}

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, []string{"create", "retire"}, ops,
		"the goodbye must follow the create it was racing, not precede it")
	require.Empty(t, live, "the recovered workspace must not outlive the client")
	require.Equal(t, "ws-2", ws.workspaceID(),
		"the client must have adopted the workspace it created before releasing it")
}

// TestClientWorkspace_RecoveryCreateIsBounded checks a wedged server cannot
// pin the recovery attempt forever. The client SDK sets no request timeout,
// so without a bound here the subscription goroutine would block for good
// on a server that accepts the create and never answers.
func TestClientWorkspace_RecoveryCreateIsBounded(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/v1/workspaces" {
			<-release // Accept the create and never answer it.
			return
		}
		http.NotFound(w, r)
	}))
	// Unwedge the handler before closing the server, which waits on it.
	t.Cleanup(func() {
		close(release)
		srv.Close()
	})

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	c, err := client.NewClient(t.TempDir(), "tcp", u.Host)
	require.NoError(t, err)

	ws := NewClientWorkspace(c, proto.Workspace{ID: "ws-1", Path: "/tmp/wedged"})
	// Shrink the bound so the test does not wait out the production window.
	// The recovery must give up on its own, without anything cancelling it:
	// the create is deliberately detached from the subscription context.
	orig := recoveryCreateTimeoutForTest(50 * time.Millisecond)
	t.Cleanup(orig)

	done := make(chan error, 1)
	go func() { done <- ws.recoverWorkspace() }()

	select {
	case err := <-done:
		require.Error(t, err, "a create that never answers must not report success")
	case <-time.After(5 * time.Second):
		t.Fatal("recoverWorkspace blocked on an unresponsive server")
	}
}
