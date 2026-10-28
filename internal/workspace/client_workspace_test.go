package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/client"
	"github.com/charmbracelet/crush/internal/commands"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/skills"
	"github.com/charmbracelet/crush/internal/ui/util"
	"github.com/charmbracelet/crush/internal/version"
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

// TestProtoToMessagePrismModel ensures the Prism-routed model fields survive
// the conversion from proto. Without them the client TUI cannot show which
// model actually served each turn on Hyper's model router.
func TestProtoToMessagePrismModel(t *testing.T) {
	t.Parallel()

	src := proto.Message{
		ID:             "m1",
		Role:           proto.Assistant,
		Model:          "prism-model",
		Provider:       "hyper",
		PrismModelID:   "prism-42",
		PrismModelName: "GLM 5.3",

		PrismHypercreditSavings: protoPtrFloat(1.5),
		PrismDollarSavings:      protoPtrFloat(0.002),
	}

	got := protoToMessage(src)
	require.Equal(t, "prism-42", got.PrismModelID)
	require.Equal(t, "GLM 5.3", got.PrismModelName)
	require.NotNil(t, got.PrismHypercreditSavings)
	require.Equal(t, 1.5, *got.PrismHypercreditSavings)
	require.NotNil(t, got.PrismDollarSavings)
	require.Equal(t, 0.002, *got.PrismDollarSavings)
}

func protoPtrFloat(v float64) *float64 { return &v }

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
				Params:      `{"repo":"charmbracelet/crush"}`,
			}
			tc.call(ws, perm)

			require.Equal(t, tc.want, got.Action)
			require.Equal(t, "req-1", got.Permission.ID)
			require.Equal(t, "sess-1", got.Permission.SessionID)
			require.Equal(t, "tc-1", got.Permission.ToolCallID)
			require.Equal(t, "tool", got.Permission.ToolName)
			require.Equal(t, "act", got.Permission.Action)
			require.Equal(t, "/tmp/p", got.Permission.Path)
			// The answer must not carry the params back: older
			// servers reject an MCP tool's params, which are a JSON
			// string, so sending them would lose the answer.
			require.Nil(t, got.Permission.Params)
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

// TestTranslateEvent_MCPChannel verifies the client reconstructs a channel
// push from the wire with its type and rendered <channel> body intact, so the
// TUI's channel handler fires in client/server mode.
func TestTranslateEvent_MCPChannel(t *testing.T) {
	t.Parallel()

	w := NewClientWorkspace(nil, proto.Workspace{})
	ev := pubsub.Event[proto.MCPEvent]{
		Type: pubsub.CreatedEvent,
		Payload: proto.MCPEvent{
			Type:           proto.MCPEventChannelMessage,
			Name:           "webhook",
			ChannelMessage: `<channel source="webhook">build failed</channel>`,
		},
	}

	out := w.translateEvent(ev)
	got, ok := out.(pubsub.Event[mcp.Event])
	require.True(t, ok, "expected pubsub.Event[mcp.Event], got %T", out)
	require.Equal(t, mcp.EventChannelMessage, got.Payload.Type)
	require.Equal(t, "webhook", got.Payload.Name)
	require.Equal(t, `<channel source="webhook">build failed</channel>`, got.Payload.ChannelMessage)
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

// TestTranslateEvent_HostTheme verifies that host theme events reach the
// TUI as HostThemeEvent: the theme for an update, nil for a clear.
func TestTranslateEvent_HostTheme(t *testing.T) {
	t.Parallel()

	w := NewClientWorkspace(nil, proto.Workspace{})
	theme := &proto.HostTheme{Name: "charmtone-dark", Palette: map[string]string{"primary": "#6B50FF"}}

	out := w.translateEvent(pubsub.Event[proto.HostThemeState]{
		Type:    pubsub.UpdatedEvent,
		Payload: proto.HostThemeState{WorkspaceID: "ws1", Theme: theme},
	})
	got, ok := out.(HostThemeEvent)
	require.True(t, ok, "expected HostThemeEvent, got %T", out)
	require.Same(t, theme, got.Theme)

	out = w.translateEvent(pubsub.Event[proto.HostThemeState]{
		Type:    pubsub.DeletedEvent,
		Payload: proto.HostThemeState{WorkspaceID: "ws1"},
	})
	got, ok = out.(HostThemeEvent)
	require.True(t, ok, "expected HostThemeEvent, got %T", out)
	require.Nil(t, got.Theme)
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

// TestClientWorkspace_MCPGetStatesTimeout pins the probe deadline: a hung
// server must not wedge MCPGetStates (and with it the UI's in-flight
// refresh flag, freezing every later MCP refresh) forever. The fetch gives
// up at mcpStatesTimeout and reports nil states.
func TestClientWorkspace_MCPGetStatesTimeout(t *testing.T) {
	t.Parallel()

	// The handler never answers; it unblocks when the client's deadline
	// cancels the request.
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	orig := mcpStatesTimeout
	mcpStatesTimeout = 50 * time.Millisecond
	t.Cleanup(func() { mcpStatesTimeout = orig })

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	c, err := client.NewClient(t.TempDir(), "tcp", u.Host)
	require.NoError(t, err)
	workspace := NewClientWorkspace(c, proto.Workspace{ID: "ws-1"})

	start := time.Now()
	require.Nil(t, workspace.MCPGetStates())
	require.Less(t, time.Since(start), 5*time.Second,
		"the probe must give up at its deadline instead of hanging")
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
	// version is the server version creates report.
	version string
	// createErr, when set, fails creates instead of serving them.
	createErr func() int
	// hangUpOnCreate drops the connection mid-response so the client
	// never learns the ID of a workspace the server did register.
	hangUpOnCreate bool

	// hostTheme is the host theme the theme endpoint reports.
	hostTheme *proto.HostTheme
	// abortStreams kills each event stream mid-response, as a server
	// that dies does, instead of ending it cleanly.
	abortStreams bool
	// config is the config a created workspace reports. A configured one
	// makes recovery initialize the agent.
	config *config.Config
	// skip is the permission-skip state the server holds.
	skip bool

	creates      int
	createReqs   []proto.Workspace
	streams      int
	sessionPosts []string
	retired      []string
	deleted      []string
	// agentCalls logs agent and permission-skip calls as "<call> <id>
	// [<arg>]", in the order the server saw them.
	agentCalls []string
}

func (s *recoveryServer) start(t *testing.T) *client.Client {
	t.Helper()
	srv := httptest.NewServer(s.handler(t))
	t.Cleanup(srv.Close)

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	c, err := client.NewClient(t.TempDir(), "tcp", u.Host)
	require.NoError(t, err)
	return c
}

// handler serves the scripted API, so a test can put it on any listener.
func (s *recoveryServer) handler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/workspaces":
			s.creates++
			var req proto.Workspace
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			s.createReqs = append(s.createReqs, req)
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
				ID: s.liveID, Path: "/tmp/recover", Config: s.config, Version: s.version,
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
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/theme"):
			id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/workspaces/"), "/theme")
			if id != s.liveID {
				http.Error(w, "workspace not found", http.StatusNotFound)
				return
			}
			require.NoError(t, json.NewEncoder(w).Encode(proto.HostThemeState{
				WorkspaceID: id, Theme: s.hostTheme,
			}))
		case strings.HasSuffix(r.URL.Path, "/agent/init"),
			strings.HasSuffix(r.URL.Path, "/agent/main"),
			strings.HasSuffix(r.URL.Path, "/permissions/skip"):
			rest := strings.TrimPrefix(r.URL.Path, "/v1/workspaces/")
			id, call, _ := strings.Cut(rest, "/")
			if id != s.liveID {
				http.Error(w, "workspace not found", http.StatusNotFound)
				return
			}
			switch call {
			case "agent/init":
				s.agentCalls = append(s.agentCalls, "init "+id)
			case "agent/main":
				var req proto.AgentSetMainRequest
				require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
				s.agentCalls = append(s.agentCalls, "main "+id+" "+req.AgentID)
			case "permissions/skip":
				if r.Method == http.MethodGet {
					require.NoError(t, json.NewEncoder(w).Encode(proto.PermissionSkipRequest{Skip: s.skip}))
					return
				}
				var req proto.PermissionSkipRequest
				require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
				s.skip = req.Skip
				s.agentCalls = append(s.agentCalls, fmt.Sprintf("skip %s %t", id, req.Skip))
			}
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
			if s.abortStreams {
				panic(http.ErrAbortHandler)
			}
			// Drop the stream so the loop keeps cycling.
		default:
			w.WriteHeader(http.StatusOK)
		}
	})
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

// TestClientWorkspace_ResyncsHostThemeAfterReconnect covers a host theme
// cleared while the stream was down: the attach replay only carries a
// theme that is set, so the client must re-read it after the reconnect and
// hand the UI the clear. Once the host sets a theme again, the next
// reconnect hands that over too.
func TestClientWorkspace_ResyncsHostThemeAfterReconnect(t *testing.T) {
	t.Cleanup(SetSSEBackoffForTest(time.Millisecond, 5*time.Millisecond))

	srv := &recoveryServer{liveID: "ws-1"}
	c := srv.start(t)
	ws := NewClientWorkspace(c, proto.Workspace{ID: "ws-1", Path: "/tmp/theme"})

	var (
		mu     sync.Mutex
		themes []*proto.HostTheme
	)
	send := func(msg tea.Msg) {
		if ev, ok := msg.(HostThemeEvent); ok {
			mu.Lock()
			themes = append(themes, ev.Theme)
			mu.Unlock()
		}
	}
	last := func() (*proto.HostTheme, int) {
		mu.Lock()
		defer mu.Unlock()
		if len(themes) == 0 {
			return nil, 0
		}
		return themes[len(themes)-1], len(themes)
	}

	done := make(chan struct{})
	go func() {
		ws.runSubscription(send)
		close(done)
	}()

	require.Eventually(t, func() bool {
		theme, n := last()
		return n > 0 && theme == nil
	}, 3*time.Second, 5*time.Millisecond,
		"a reconnect must hand the UI the host theme the server holds, even none")

	theme := &proto.HostTheme{Name: "host", Palette: map[string]string{"primary": "#112233"}}
	srv.snapshot(func(s *recoveryServer) { s.hostTheme = theme })
	require.Eventually(t, func() bool {
		got, _ := last()
		return theme.Equal(got)
	}, 3*time.Second, 5*time.Millisecond)

	ws.Shutdown()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("runSubscription did not return after Shutdown")
	}
}

// TestClientWorkspace_ResyncsHostThemeAfterAbortedStream covers a stream
// that dies mid-response (the server was killed) instead of ending
// cleanly: the loop must still resubscribe and re-read the host theme, so a
// clear published meanwhile reaches the UI.
func TestClientWorkspace_ResyncsHostThemeAfterAbortedStream(t *testing.T) {
	t.Cleanup(SetSSEBackoffForTest(time.Millisecond, 5*time.Millisecond))

	srv := &recoveryServer{liveID: "ws-1", abortStreams: true}
	c := srv.start(t)
	ws := NewClientWorkspace(c, proto.Workspace{ID: "ws-1", Path: "/tmp/abort"})

	var resynced atomic.Bool
	send := func(msg tea.Msg) {
		if ev, ok := msg.(HostThemeEvent); ok && ev.Theme == nil {
			resynced.Store(true)
		}
	}
	done := make(chan struct{})
	go func() {
		ws.runSubscription(send)
		close(done)
	}()

	require.Eventually(t, func() bool {
		var streams int
		srv.snapshot(func(s *recoveryServer) { streams = s.streams })
		return streams >= 2 && resynced.Load()
	}, 3*time.Second, 5*time.Millisecond,
		"an aborted stream must be resubscribed and the host theme re-read")

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

// shortSocketPath returns a unix socket path in a fresh directory, short
// enough for the macOS sun_path limit that t.TempDir() can exceed.
func shortSocketPath(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "crush-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "s.sock")
}

// TestClientWorkspace_RespawnsDeadServer is the dead-server regression: the
// server process died (kill -9, OOM, a crash), so nothing listens on its
// address any more. The loop used to retry that address forever, leaving
// the TUI broken until some other crush process happened to spawn a server.
// It must bring a server back through the respawn hook and re-register the
// workspace on it.
func TestClientWorkspace_RespawnsDeadServer(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows servers listen on named pipes; see the npipe twin")
	}
	sock := shortSocketPath(t)
	testRespawnsDeadServer(t, "unix", sock, func() (net.Listener, error) {
		// Clear the dead server's socket first, as ensureServer does.
		if err := os.Remove(sock); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		ln, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", sock)
		if err != nil {
			return nil, err
		}
		// A killed server leaves its socket file behind, so dialing it is
		// refused rather than finding nothing.
		ln.(*net.UnixListener).SetUnlinkOnClose(false)
		return ln, nil
	})
}

// testRespawnsDeadServer kills a server the client is subscribed to, by
// dropping every connection and listener, and expects the client to bring
// up the next server through its respawn hook and re-register the
// workspace there. listen binds the server's address.
func testRespawnsDeadServer(t *testing.T, network, addr string, listen func() (net.Listener, error)) {
	t.Helper()
	t.Cleanup(SetSSEBackoffForTest(time.Millisecond, 5*time.Millisecond))

	serve := func(s *recoveryServer) (*http.Server, error) {
		ln, err := listen()
		if err != nil {
			return nil, err
		}
		hs := &http.Server{Handler: s.handler(t)}
		go func() { _ = hs.Serve(ln) }()
		t.Cleanup(func() { _ = hs.Close() })
		return hs, nil
	}

	dead := &recoveryServer{liveID: "ws-1"}
	dying, err := serve(dead)
	require.NoError(t, err)

	c, err := client.NewClient(t.TempDir(), network, addr)
	require.NoError(t, err)

	// The next server knows nothing of the dead one's workspaces.
	next := &recoveryServer{nextID: "ws-2"}
	var respawns atomic.Int32
	ws := NewClientWorkspace(c, proto.Workspace{ID: "ws-1", Path: "/tmp/respawn"},
		WithServerRespawn(func(context.Context) error {
			if _, err := serve(next); err != nil {
				return err
			}
			respawns.Add(1)
			return nil
		}))
	done := make(chan struct{})
	go func() {
		ws.runSubscription(func(tea.Msg) {})
		close(done)
	}()

	require.Eventually(t, func() bool {
		var streams int
		dead.snapshot(func(s *recoveryServer) { streams = s.streams })
		return streams > 0
	}, 3*time.Second, 5*time.Millisecond)
	require.NoError(t, dying.Close())

	require.Eventually(t, func() bool { return ws.workspaceID() == "ws-2" },
		5*time.Second, 5*time.Millisecond,
		"a dead server must be brought back and the workspace re-registered on it")
	require.Eventually(t, func() bool {
		var streams int
		next.snapshot(func(s *recoveryServer) { streams = s.streams })
		return streams > 0
	}, 3*time.Second, 5*time.Millisecond,
		"the event stream must be re-established on the new server")
	require.EqualValues(t, 1, respawns.Load(), "one dead server needs one replacement")

	ws.Shutdown()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("runSubscription did not return after Shutdown")
	}
}

// TestClientWorkspace_EscalatesWhenRespawnFails checks a server that cannot
// be brought back is escalated like a workspace that cannot be re-registered:
// the loop keeps trying, but the UI is told the connection looks stuck
// rather than showing a transient notice that quietly expires.
func TestClientWorkspace_EscalatesWhenRespawnFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("dials a unix socket path")
	}
	t.Cleanup(SetSSEBackoffForTest(time.Millisecond, 2*time.Millisecond))

	// Nothing ever listens here, so every dial finds no server.
	c, err := client.NewClient(t.TempDir(), "unix", shortSocketPath(t))
	require.NoError(t, err)

	var respawns atomic.Int32
	ws := NewClientWorkspace(c, proto.Workspace{ID: "ws-1", Path: "/tmp/respawn"},
		WithServerRespawn(func(context.Context) error {
			respawns.Add(1)
			return errors.New("server failed to start")
		}))
	rec := &connectionRecorder{}
	done := make(chan struct{})
	go func() {
		ws.runSubscription(rec.send)
		close(done)
	}()

	require.Eventually(t, rec.sawStuck, 5*time.Second, 5*time.Millisecond,
		"repeated respawn failures must escalate to a persistent notice")
	require.GreaterOrEqual(t, int(respawns.Load()), maxRecoveryEscalate,
		"the loop must keep trying to bring the server back")

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

// TestClientWorkspace_RecoveryRestoresModes covers the modes a re-created
// workspace used to lose: it starts on the coder agent with the YOLO flag
// of the snapshot it was created from. A client in plan mode got write
// tools while its UI still said PLAN MODE, and YOLO the user had turned
// off came back on, so tools ran without prompts.
func TestClientWorkspace_RecoveryRestoresModes(t *testing.T) {
	t.Cleanup(SetSSEBackoffForTest(time.Millisecond, 5*time.Millisecond))

	cases := []struct {
		name string
		// yolo is the YOLO flag of the snapshot the client starts from.
		yolo bool
		// before runs while the original workspace is still live.
		before func(t *testing.T, ws *ClientWorkspace, srv *recoveryServer)
		// wantYOLO is the YOLO flag the recovery create must carry.
		wantYOLO bool
		// wantCalls is every agent and permission-skip call the server
		// saw, recovery included.
		wantCalls []string
	}{
		{
			name: "plan mode",
			before: func(t *testing.T, ws *ClientWorkspace, _ *recoveryServer) {
				require.NoError(t, ws.AgentSetMain(config.AgentPlan))
			},
			wantCalls: []string{"main ws-1 plan", "init ws-2", "main ws-2 plan"},
		},
		{
			// A fresh coordinator already runs the coder agent, so there
			// is nothing to restore.
			name: "back to code mode",
			before: func(t *testing.T, ws *ClientWorkspace, _ *recoveryServer) {
				require.NoError(t, ws.AgentSetMain(config.AgentPlan))
				require.NoError(t, ws.AgentSetMain(config.AgentCoder))
			},
			wantCalls: []string{"main ws-1 plan", "main ws-1 coder", "init ws-2"},
		},
		{
			name: "yolo turned off",
			yolo: true,
			before: func(_ *testing.T, ws *ClientWorkspace, _ *recoveryServer) {
				ws.PermissionSetSkipRequests(false)
			},
			wantCalls: []string{"skip ws-1 false", "init ws-2"},
		},
		{
			// The snapshot only knows the flag the workspace was created
			// with; the client learns of the change from reading it.
			name: "yolo turned off by another client",
			yolo: true,
			before: func(t *testing.T, ws *ClientWorkspace, srv *recoveryServer) {
				srv.snapshot(func(s *recoveryServer) { s.skip = false })
				require.False(t, ws.PermissionSkipRequests())
			},
			wantCalls: []string{"init ws-2"},
		},
		{
			name: "yolo turned on",
			before: func(_ *testing.T, ws *ClientWorkspace, _ *recoveryServer) {
				ws.PermissionSetSkipRequests(true)
			},
			wantYOLO:  true,
			wantCalls: []string{"skip ws-1 true", "init ws-2"},
		},
		{
			name:      "startup yolo untouched",
			yolo:      true,
			wantYOLO:  true,
			wantCalls: []string{"init ws-2"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := &recoveryServer{
				liveID: "ws-1",
				nextID: "ws-2",
				skip:   tc.yolo,
				config: &config.Config{
					Options: &config.Options{},
					Providers: csync.NewMapFrom(map[string]config.ProviderConfig{
						"fake": {ID: "fake"},
					}),
				},
			}
			c := srv.start(t)
			ws := NewClientWorkspace(c, proto.Workspace{
				ID: "ws-1", Path: "/tmp/recover", YOLO: tc.yolo,
			})
			if tc.before != nil {
				tc.before(t, ws, srv)
			}

			// The server loses the workspace, as it does when it restarts.
			srv.snapshot(func(s *recoveryServer) { s.liveID = "" })
			done := make(chan struct{})
			go func() {
				ws.runSubscription(func(tea.Msg) {})
				close(done)
			}()

			// Recovery has finished once the re-created workspace streams.
			require.Eventually(t, func() bool {
				var streams int
				srv.snapshot(func(s *recoveryServer) { streams = s.streams })
				return streams > 0
			}, 3*time.Second, 5*time.Millisecond)
			ws.Shutdown()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("runSubscription did not return after Shutdown")
			}

			srv.snapshot(func(s *recoveryServer) {
				require.Len(t, s.createReqs, 1)
				require.Equal(t, tc.wantYOLO, s.createReqs[0].YOLO,
					"the workspace must be re-created with the YOLO state the client last knew")
				require.Equal(t, tc.wantCalls, s.agentCalls,
					"recovery must restore the main agent the client last chose")
			})
		})
	}
}

// TestClientWorkspace_StaleSkipReadKeepsToggle covers the UI polling the
// permission-skip state while the user toggles YOLO: a read the server
// answered before the toggle must not overwrite it, or recovery would
// re-create the workspace with YOLO back on.
func TestClientWorkspace_StaleSkipReadKeepsToggle(t *testing.T) {
	t.Parallel()

	reading := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/workspaces/ws-1/permissions/skip", r.URL.Path)
		if r.Method == http.MethodGet {
			close(reading)
			<-release
			// The answer the server gave before the toggle landed.
			require.NoError(t, json.NewEncoder(w).Encode(proto.PermissionSkipRequest{Skip: true}))
		}
	}))
	t.Cleanup(func() {
		unblock()
		srv.Close()
	})

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	c, err := client.NewClient(t.TempDir(), "tcp", u.Host)
	require.NoError(t, err)
	ws := NewClientWorkspace(c, proto.Workspace{ID: "ws-1", YOLO: true})

	read := make(chan bool, 1)
	go func() { read <- ws.PermissionSkipRequests() }()
	<-reading
	ws.PermissionSetSkipRequests(false)
	unblock()
	require.True(t, <-read)

	require.False(t, ws.recreateArgs().YOLO,
		"a read answered before the toggle must not undo it")
}

// infoRecorder collects the status messages the subscription loop hands
// the UI.
type infoRecorder struct {
	mu   sync.Mutex
	msgs []util.InfoMsg
}

func (r *infoRecorder) send(msg tea.Msg) {
	info, ok := msg.(util.InfoMsg)
	if !ok {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.msgs = append(r.msgs, info)
}

func (r *infoRecorder) all() []util.InfoMsg {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]util.InfoMsg(nil), r.msgs...)
}

// runSubscriptionUntilStreams runs the subscription loop until the server
// has served n event streams, then shuts the workspace down.
func runSubscriptionUntilStreams(t *testing.T, ws *ClientWorkspace, srv *recoveryServer, n int, send func(tea.Msg)) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		ws.runSubscription(send)
		close(done)
	}()
	require.Eventually(t, func() bool {
		var streams int
		srv.snapshot(func(s *recoveryServer) { streams = s.streams })
		return streams >= n
	}, 3*time.Second, 5*time.Millisecond)
	ws.Shutdown()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("runSubscription did not return after Shutdown")
	}
}

// TestClientWorkspace_WarnsAboutServerVersionMismatch checks that the user
// is told when the server hosting the workspace runs another Crush
// version, as it does when a busy server declines to stand down for an
// upgraded client. The server used to publish this warning itself, but
// before the creating client could subscribe and in a form the event
// stream drops, so it never reached anyone.
func TestClientWorkspace_WarnsAboutServerVersionMismatch(t *testing.T) {
	t.Cleanup(SetSSEBackoffForTest(time.Millisecond, 5*time.Millisecond))

	otherVersion := "not-" + version.Version
	for _, tc := range []struct {
		name          string
		serverVersion string
		wantWarning   bool
	}{
		{name: "different version", serverVersion: otherVersion, wantWarning: true},
		{name: "same version", serverVersion: version.Version},
		// Servers that predate reporting their version.
		{name: "unknown version", serverVersion: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := &recoveryServer{liveID: "ws-1"}
			c := srv.start(t)
			ws := NewClientWorkspace(c, proto.Workspace{
				ID: "ws-1", Path: "/tmp/version", Version: tc.serverVersion,
			})

			// The test server drops every stream, so the loop reconnects
			// several times before it is stopped.
			rec := &infoRecorder{}
			runSubscriptionUntilStreams(t, ws, srv, 3, rec.send)

			msgs := rec.all()
			if !tc.wantWarning {
				require.Empty(t, msgs, "only a server on another version is worth a warning")
				return
			}
			require.Len(t, msgs, 1,
				"the user must be told, once, that the server runs another version")
			require.Equal(t, util.InfoTypeWarn, msgs[0].Type)
			require.Contains(t, msgs[0].Msg, strconv.Quote(otherVersion))
			require.Contains(t, msgs[0].Msg, strconv.Quote(version.Version))
		})
	}
}

// TestClientWorkspace_WarnsAboutServerVersionAfterRecovery covers a
// workspace that recovery re-created on a server of another version, such
// as one a client of that version spawned after the old server went away.
func TestClientWorkspace_WarnsAboutServerVersionAfterRecovery(t *testing.T) {
	t.Cleanup(SetSSEBackoffForTest(time.Millisecond, 5*time.Millisecond))

	otherVersion := "not-" + version.Version
	srv := &recoveryServer{nextID: "ws-2", version: otherVersion}
	c := srv.start(t)
	ws := NewClientWorkspace(c, proto.Workspace{
		ID: "ws-1", Path: "/tmp/recover", Version: version.Version,
	})

	rec := &infoRecorder{}
	runSubscriptionUntilStreams(t, ws, srv, 3, rec.send)

	require.Equal(t, "ws-2", ws.workspaceID())
	msgs := rec.all()
	require.Len(t, msgs, 1,
		"the user must be told that the replacement server runs another version")
	require.Equal(t, util.InfoTypeWarn, msgs[0].Type)
	require.Contains(t, msgs[0].Msg, strconv.Quote(otherVersion))
}

// TestTranslateEvent_PermissionNotification verifies that the
// notification's session ID survives the proto-to-domain translation,
// keeping it available for session-scoped consumers.
func TestTranslateEvent_PermissionNotification(t *testing.T) {
	t.Parallel()

	w := NewClientWorkspace(nil, proto.Workspace{})
	ev := pubsub.Event[proto.PermissionNotification]{
		Type: pubsub.CreatedEvent,
		Payload: proto.PermissionNotification{
			SessionID:  "s1",
			ToolCallID: "tc-1",
			Granted:    true,
		},
	}

	out := w.translateEvent(ev)
	got, ok := out.(pubsub.Event[permission.PermissionNotification])
	require.True(t, ok, "expected pubsub.Event[permission.PermissionNotification], got %T", out)
	require.Equal(t, "s1", got.Payload.SessionID)
	require.Equal(t, "tc-1", got.Payload.ToolCallID)
	require.True(t, got.Payload.Granted)
}

// TestClientWorkspace_GitBranch verifies that GitBranch asks the server and
// returns what it reports. Caching and scheduling are the TUI's business, so
// nothing here pretends otherwise.
func TestClientWorkspace_GitBranch(t *testing.T) {
	t.Parallel()

	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/workspaces/ws-1/git/branch", r.URL.Path)
		hits.Add(1)
		require.NoError(t, json.NewEncoder(w).Encode(proto.GitBranchResponse{Branch: "feature/x"}))
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	c, err := client.NewClient(t.TempDir(), "tcp", u.Host)
	require.NoError(t, err)

	ws := NewClientWorkspace(c, proto.Workspace{ID: "ws-1"})
	t.Cleanup(ws.Shutdown)

	branch, err := ws.GitBranch(t.Context())
	require.NoError(t, err)
	require.Equal(t, "feature/x", branch)
	require.Equal(t, int64(1), hits.Load(), "one call must make exactly one request")
}

// TestClientWorkspace_GitBranchServerError verifies that a failing server
// surfaces as an error rather than a silently empty branch, so the caller can
// decide whether to keep what it already had.
func TestClientWorkspace_GitBranchServerError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	c, err := client.NewClient(t.TempDir(), "tcp", u.Host)
	require.NoError(t, err)

	ws := NewClientWorkspace(c, proto.Workspace{ID: "ws-1"})
	t.Cleanup(ws.Shutdown)
	_, err = ws.GitBranch(t.Context())
	require.Error(t, err)
}
