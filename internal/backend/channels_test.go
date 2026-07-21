package backend

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent"
	mcptools "github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// fakeChannelSessions is a channelSessionStore backed by in-memory
// fixtures. created records the titles passed to Create.
type fakeChannelSessions struct {
	sessions map[string]session.Session
	listed   []session.Session
	created  []string
	getErr   error
}

func (f *fakeChannelSessions) Create(_ context.Context, title string) (session.Session, error) {
	f.created = append(f.created, title)
	return session.Session{ID: "created-" + title}, nil
}

func (f *fakeChannelSessions) Get(_ context.Context, id string) (session.Session, error) {
	if f.getErr != nil {
		return session.Session{}, f.getErr
	}
	s, ok := f.sessions[id]
	if !ok {
		return session.Session{}, errors.New("not found")
	}
	return s, nil
}

func (f *fakeChannelSessions) List(context.Context) ([]session.Session, error) {
	return f.listed, nil
}

func TestChannelTargetSession(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	t.Run("single viewed session wins", func(t *testing.T) {
		t.Parallel()
		got, err := channelTargetSession(ctx, []string{"s1"}, &fakeChannelSessions{})
		require.NoError(t, err)
		require.Equal(t, "s1", got)
	})

	t.Run("multiple viewed picks most recently updated", func(t *testing.T) {
		t.Parallel()
		store := &fakeChannelSessions{sessions: map[string]session.Session{
			"old": {ID: "old", UpdatedAt: 100},
			"new": {ID: "new", UpdatedAt: 200},
		}}
		got, err := channelTargetSession(ctx, []string{"new", "old"}, store)
		require.NoError(t, err)
		require.Equal(t, "new", got)
	})

	t.Run("viewed session ties break on smallest ID", func(t *testing.T) {
		t.Parallel()
		store := &fakeChannelSessions{sessions: map[string]session.Session{
			"b": {ID: "b", UpdatedAt: 100},
			"a": {ID: "a", UpdatedAt: 100},
		}}
		got, err := channelTargetSession(ctx, []string{"b", "a"}, store)
		require.NoError(t, err)
		require.Equal(t, "a", got)
	})

	t.Run("none viewed reuses most recent top-level session", func(t *testing.T) {
		t.Parallel()
		store := &fakeChannelSessions{listed: []session.Session{
			{ID: "recent"}, {ID: "older"},
		}}
		got, err := channelTargetSession(ctx, nil, store)
		require.NoError(t, err)
		require.Equal(t, "recent", got)
		require.Empty(t, store.created)
	})

	t.Run("none viewed and no sessions creates one", func(t *testing.T) {
		t.Parallel()
		store := &fakeChannelSessions{}
		got, err := channelTargetSession(ctx, nil, store)
		require.NoError(t, err)
		require.Equal(t, "created-New Session", got)
		require.Equal(t, []string{"New Session"}, store.created)
	})

	t.Run("all viewed sessions unloadable falls back", func(t *testing.T) {
		t.Parallel()
		store := &fakeChannelSessions{getErr: errors.New("boom")}
		got, err := channelTargetSession(ctx, []string{"x", "y"}, store)
		require.NoError(t, err)
		require.Equal(t, "created-New Session", got)
	})
}

func TestWorkspaceViewedSessions(t *testing.T) {
	t.Parallel()
	ws := &Workspace{clients: map[string]*clientState{
		"attached-a":   {streams: 1, currentSessionID: "s2"},
		"attached-b":   {streams: 2, currentSessionID: "s1"},
		"attached-c":   {streams: 1, currentSessionID: "s1"}, // duplicate of b
		"landing":      {streams: 1, currentSessionID: ""},   // landing screen
		"hold-only":    {streams: 0, currentSessionID: "s3"}, // no live stream
		"hold-landing": {streams: 0},
	}}
	require.Equal(t, []string{"s1", "s2"}, ws.viewedSessions())
}

// recordingCoordinator is a minimal agent.Coordinator that records the
// session/prompt of each RunAccepted call, plus the per-turn channel
// provenance carried on the context.
type recordingCoordinator struct {
	runs     chan [2]string // {sessionID, prompt}
	channels chan string    // per-turn channel provenance
}

func newRecordingCoordinator() *recordingCoordinator {
	return &recordingCoordinator{runs: make(chan [2]string, 8), channels: make(chan string, 8)}
}

func (c *recordingCoordinator) Run(context.Context, string, string, ...message.Attachment) (*fantasy.AgentResult, error) {
	return nil, nil
}

func (c *recordingCoordinator) RunAccepted(ctx context.Context, _ *agent.AcceptedRun, sessionID, prompt string, _ ...message.Attachment) (*fantasy.AgentResult, error) {
	c.channels <- agent.ChannelFromContext(ctx)
	c.runs <- [2]string{sessionID, prompt}
	return nil, nil
}

func (c *recordingCoordinator) BeginAccepted(string) *agent.AcceptedRun       { return nil }
func (c *recordingCoordinator) Cancel(string)                                 {}
func (c *recordingCoordinator) CancelAll()                                    {}
func (c *recordingCoordinator) IsBusy() bool                                  { return false }
func (c *recordingCoordinator) IsSessionBusy(string) bool                     { return false }
func (c *recordingCoordinator) QueuedPrompts(string) int                      { return 0 }
func (c *recordingCoordinator) QueuedPromptsList(string) []string             { return nil }
func (c *recordingCoordinator) ClearQueue(string)                             {}
func (c *recordingCoordinator) Summarize(context.Context, string) error       { return nil }
func (c *recordingCoordinator) Model() agent.Model                            { return agent.Model{} }
func (c *recordingCoordinator) UpdateModels(context.Context) error            { return nil }
func (c *recordingCoordinator) GenerateTitle(context.Context, string, string) {}

// fullFakeSessions adapts fakeChannelSessions to the full session.Service
// interface by embedding it; only the channelSessionStore subset is
// implemented, which is all the channel router touches.
type fullFakeSessions struct {
	session.Service
	*fakeChannelSessions
}

func (f *fullFakeSessions) Create(ctx context.Context, title string) (session.Session, error) {
	return f.fakeChannelSessions.Create(ctx, title)
}

func (f *fullFakeSessions) Get(ctx context.Context, id string) (session.Session, error) {
	return f.fakeChannelSessions.Get(ctx, id)
}

func (f *fullFakeSessions) List(ctx context.Context) ([]session.Session, error) {
	return f.fakeChannelSessions.List(ctx)
}

// insertChannelWorkspace installs a synthetic workspace whose config
// declares an MCP server named srvName, opted in as a channel iff
// enabled is true, mirroring the fields the channel router reads.
func insertChannelWorkspace(t *testing.T, b *Backend, srvName string, enabled bool, coord agent.Coordinator, sessions session.Service) *Workspace {
	t.Helper()

	wd := t.TempDir()
	cfgJSON, err := json.Marshal(map[string]any{
		"mcp": map[string]any{
			srvName: map[string]any{"type": "http", "url": "http://127.0.0.1:0/mcp"},
		},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(wd, "crush.json"), cfgJSON, 0o644))

	cfg, err := config.Init(wd, "", false)
	require.NoError(t, err)
	if enabled {
		cfg.Overrides().EnabledChannels = []string{srvName}
	}

	ws := &Workspace{
		ID:           uuid.New().String(),
		Path:         wd,
		Cfg:          cfg,
		resolvedPath: wd,
		clients:      make(map[string]*clientState),
		shutdownFn:   func() {},
	}
	ws.App = &app.App{AgentCoordinator: coord, Sessions: sessions}
	ws.ctx, ws.cancel = context.WithCancel(b.ctx)
	b.mu.Lock()
	b.workspaces.Set(ws.ID, ws)
	b.pathIndex[ws.resolvedPath] = ws.ID
	b.mu.Unlock()
	return ws
}

// TestRouteChannelMessage_InjectsOncePerOptedInWorkspace is the core
// exactly-once contract: a channel push is injected into the workspace
// that declares AND opted in the server (even with zero attached
// clients), and skipped for workspaces that only declare it or that
// don't know the server at all.
func TestRouteChannelMessage_InjectsOncePerOptedInWorkspace(t *testing.T) {
	xdgIsolated(t)
	b, _ := newTestBackend(t)

	optedIn := newRecordingCoordinator()
	declaredOnly := newRecordingCoordinator()
	unrelated := newRecordingCoordinator()

	wsIn := insertChannelWorkspace(t, b, "webhook", true, optedIn,
		&fullFakeSessions{fakeChannelSessions: &fakeChannelSessions{listed: []session.Session{{ID: "recent"}}}})
	insertChannelWorkspace(t, b, "webhook", false, declaredOnly,
		&fullFakeSessions{fakeChannelSessions: &fakeChannelSessions{}})
	insertChannelWorkspace(t, b, "other", true, unrelated,
		&fullFakeSessions{fakeChannelSessions: &fakeChannelSessions{}})

	const content = `<channel source="webhook">build failed</channel>`
	b.routeChannelMessage(mcptools.Event{
		Type:           mcptools.EventChannelMessage,
		Name:           "webhook",
		ChannelMessage: content,
	})

	select {
	case run := <-optedIn.runs:
		require.Equal(t, "recent", run[0], "should land in the most recent session")
		require.Equal(t, content, run[1])
		require.Equal(t, "webhook", <-optedIn.channels, "the server-routed turn must carry its originating channel")
	case <-time.After(5 * time.Second):
		t.Fatal("expected the opted-in workspace to receive the channel push")
	}
	wsIn.runWG.Wait()

	require.Empty(t, optedIn.runs, "exactly one injection for the opted-in workspace")
	require.Empty(t, declaredOnly.runs, "declared-but-not-opted-in workspace must not receive the push")
	require.Empty(t, unrelated.runs, "workspace without the server must not receive the push")
}

// TestRouteChannelMessage_PrefersViewedSession verifies the presence map
// drives targeting: with a client viewing a session, the push lands
// there instead of the most recently updated session.
func TestRouteChannelMessage_PrefersViewedSession(t *testing.T) {
	xdgIsolated(t)
	b, _ := newTestBackend(t)

	coord := newRecordingCoordinator()
	ws := insertChannelWorkspace(t, b, "webhook", true, coord,
		&fullFakeSessions{fakeChannelSessions: &fakeChannelSessions{listed: []session.Session{{ID: "recent"}}}})
	ws.clientsMu.Lock()
	ws.clients["client-1"] = &clientState{streams: 1, currentSessionID: "viewed"}
	ws.clientsMu.Unlock()

	b.routeChannelMessage(mcptools.Event{
		Type:           mcptools.EventChannelMessage,
		Name:           "webhook",
		ChannelMessage: `<channel source="webhook">hi</channel>`,
	})

	select {
	case run := <-coord.runs:
		require.Equal(t, "viewed", run[0])
	case <-time.After(5 * time.Second):
		t.Fatal("expected an injection into the viewed session")
	}
	ws.runWG.Wait()
}
