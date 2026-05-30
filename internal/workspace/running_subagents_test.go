package workspace

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/subagents"
)

// -- minimal session.Service stub for token-enrichment tests --

type stubSessionService struct {
	sessions map[string]session.Session
}

func (s *stubSessionService) Subscribe(context.Context) <-chan pubsub.Event[session.Session] {
	return make(chan pubsub.Event[session.Session])
}

func (s *stubSessionService) Create(_ context.Context, title string) (session.Session, error) {
	return session.Session{ID: "new", Title: title}, nil
}

func (s *stubSessionService) CreateTitleSession(context.Context, string) (session.Session, error) {
	return session.Session{}, nil
}

func (s *stubSessionService) CreateTaskSession(context.Context, string, string, string) (session.Session, error) {
	return session.Session{}, nil
}

func (s *stubSessionService) Get(_ context.Context, id string) (session.Session, error) {
	if sess, ok := s.sessions[id]; ok {
		return sess, nil
	}
	return session.Session{}, sql.ErrNoRows
}

func (s *stubSessionService) GetLast(context.Context) (session.Session, error) {
	return session.Session{}, sql.ErrNoRows
}

func (s *stubSessionService) List(context.Context) ([]session.Session, error) {
	return nil, nil
}

func (s *stubSessionService) Save(_ context.Context, sess session.Session) (session.Session, error) {
	return sess, nil
}

func (s *stubSessionService) UpdateTitleAndUsage(context.Context, string, string, int64, int64, float64) error {
	return nil
}

func (s *stubSessionService) Rename(context.Context, string, string) error { return nil }

func (s *stubSessionService) Delete(context.Context, string) error { return nil }

func (s *stubSessionService) CreateAgentToolSessionID(messageID, toolCallID string) string {
	return fmt.Sprintf("%s$$%s", messageID, toolCallID)
}

func (s *stubSessionService) ParseAgentToolSessionID(sessionID string) (string, string, bool) {
	parts := strings.Split(sessionID, "$$")
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func (s *stubSessionService) IsAgentToolSession(sessionID string) bool {
	_, _, ok := s.ParseAgentToolSessionID(sessionID)
	return ok
}

// newStoreForWorkDir returns a ConfigStore whose WorkingDir() reports workDir.
func newStoreForWorkDir(workDir string) *config.ConfigStore {
	return config.NewTestStoreWithWorkingDir(&config.Config{}, workDir)
}

// TestAppWorkspace_RunningSubagents_Empty verifies that a nil SubagentRuntime
// returns a nil slice without panicking.
func TestAppWorkspace_RunningSubagents_Empty(t *testing.T) {
	t.Parallel()

	w := &AppWorkspace{
		app:   &app.App{SubagentRuntime: nil},
		store: config.NewTestStore(&config.Config{}),
	}

	got := w.RunningSubagents("parent-1")
	require.Nil(t, got)
}

// TestAppWorkspace_RunningSubagents_WithEntries verifies that entries registered
// on the Runtime are mapped to RunningSubagentInfo with the correct fields.
func TestAppWorkspace_RunningSubagents_WithEntries(t *testing.T) {
	t.Parallel()

	rt := subagents.NewRuntime()
	t.Cleanup(rt.Shutdown)

	rt.Register("parent-1", "child-A", "agent-alpha", "blue")
	rt.Register("parent-1", "child-B", "agent-beta", "red")

	w := &AppWorkspace{
		app: &app.App{
			SubagentRuntime: rt,
			Sessions:        &stubSessionService{sessions: map[string]session.Session{}},
		},
		store: config.NewTestStore(&config.Config{}),
	}

	got := w.RunningSubagents("parent-1")
	require.Len(t, got, 2)

	byChild := map[string]RunningSubagentInfo{}
	for _, info := range got {
		byChild[info.ChildSessionID] = info
	}

	a := byChild["child-A"]
	require.Equal(t, "parent-1", a.ParentSessionID)
	require.Equal(t, "agent-alpha", a.Name)
	require.Equal(t, "blue", a.Color)
	require.Equal(t, "running", a.Status)
	require.False(t, a.StartedAt.IsZero())

	b := byChild["child-B"]
	require.Equal(t, "agent-beta", b.Name)
	require.Equal(t, "red", b.Color)
}

// TestAppWorkspace_RunningSubagents_TokenEnrichment verifies that when a child
// session exists, its PromptTokens and CompletionTokens are included in the
// returned RunningSubagentInfo.
func TestAppWorkspace_RunningSubagents_TokenEnrichment(t *testing.T) {
	t.Parallel()

	rt := subagents.NewRuntime()
	t.Cleanup(rt.Shutdown)

	rt.Register("parent-1", "child-tok", "agent-tok", "green")

	sessions := &stubSessionService{
		sessions: map[string]session.Session{
			"child-tok": {
				ID:               "child-tok",
				PromptTokens:     100,
				CompletionTokens: 200,
			},
		},
	}

	w := &AppWorkspace{
		app: &app.App{
			SubagentRuntime: rt,
			Sessions:        sessions,
		},
		store: config.NewTestStore(&config.Config{}),
	}

	got := w.RunningSubagents("parent-1")
	require.Len(t, got, 1)
	require.Equal(t, int64(100), got[0].PromptTokens)
	require.Equal(t, int64(200), got[0].CompletionTokens)
}

// TestAppWorkspace_SubscribeSubagentRuntime_NilRuntime verifies that a nil
// SubagentRuntime returns a closed channel without panicking.
func TestAppWorkspace_SubscribeSubagentRuntime_NilRuntime(t *testing.T) {
	t.Parallel()

	w := &AppWorkspace{
		app:   &app.App{SubagentRuntime: nil},
		store: config.NewTestStore(&config.Config{}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	ch := w.SubscribeSubagentRuntime(ctx)
	require.NotNil(t, ch)

	select {
	case _, ok := <-ch:
		require.False(t, ok, "channel must be closed when SubagentRuntime is nil")
	default:
		t.Fatal("channel was not immediately closed for nil SubagentRuntime")
	}
}

// TestAppWorkspace_CancelSubagent_NilCoordinator verifies that calling
// CancelSubagent with a nil AgentCoordinator does not panic.
func TestAppWorkspace_CancelSubagent_NilCoordinator(t *testing.T) {
	t.Parallel()

	w := &AppWorkspace{
		app:   &app.App{AgentCoordinator: nil},
		store: config.NewTestStore(&config.Config{}),
	}

	require.NotPanics(t, func() {
		w.CancelSubagent("child-session-id")
	})
}

// TestAppWorkspace_AllSubagents_NilManager verifies that a nil Subagents
// manager returns nil without panicking.
func TestAppWorkspace_AllSubagents_NilManager(t *testing.T) {
	t.Parallel()

	w := &AppWorkspace{
		app:   &app.App{Subagents: nil},
		store: config.NewTestStore(&config.Config{}),
	}

	got := w.AllSubagents()
	require.Nil(t, got)
}

// TestAppWorkspace_AllSubagents_ScopeDetection verifies that the Scope field on
// returned SubagentDefInfo is set to "project" for agents whose file path is
// under the workspace working directory, "user" for agents outside, and
// "builtin" for agents with an empty FilePath.
func TestAppWorkspace_AllSubagents_ScopeDetection(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()

	projectFile := filepath.Join(workDir, ".crush", "subagents", "proj-agent.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(projectFile), 0o755))
	require.NoError(t, os.WriteFile(projectFile,
		[]byte("---\nname: proj-agent\ndescription: Project agent.\n---\n\nBody.\n"),
		0o644,
	))

	userDir := t.TempDir()
	userFile := filepath.Join(userDir, "user-agent.md")
	require.NoError(t, os.WriteFile(userFile,
		[]byte("---\nname: user-agent\ndescription: User agent.\n---\n\nBody.\n"),
		0o644,
	))

	projAgent := &subagents.Subagent{Name: "proj-agent", Description: "Project agent.", FilePath: projectFile}
	userAgent := &subagents.Subagent{Name: "user-agent", Description: "User agent.", FilePath: userFile}
	builtinAgent := &subagents.Subagent{Name: "builtin-agent", Description: "Built-in agent.", FilePath: ""}

	mgr := subagents.NewManager(
		[]*subagents.Subagent{projAgent, userAgent, builtinAgent},
		[]*subagents.Subagent{projAgent, userAgent, builtinAgent},
		nil,
	)
	t.Cleanup(mgr.Shutdown)

	w := &AppWorkspace{
		app:   &app.App{Subagents: mgr},
		store: newStoreForWorkDir(workDir),
	}

	got := w.AllSubagents()
	require.Len(t, got, 3)

	byName := map[string]SubagentDefInfo{}
	for _, info := range got {
		byName[info.Name] = info
	}

	require.Equal(t, "project", byName["proj-agent"].Scope)
	require.Equal(t, "user", byName["user-agent"].Scope)
	require.Equal(t, "builtin", byName["builtin-agent"].Scope)
}

// TestAppWorkspace_DeleteUserSubagent_NotFound verifies that deleting a
// subagent by a name that doesn't exist returns an error.
func TestAppWorkspace_DeleteUserSubagent_NotFound(t *testing.T) {
	t.Parallel()

	mgr := subagents.NewManager(nil, nil, nil)
	t.Cleanup(mgr.Shutdown)

	w := &AppWorkspace{
		app:   &app.App{Subagents: mgr},
		store: config.NewTestStore(&config.Config{}),
	}

	err := w.DeleteUserSubagent("nonexistent-agent")
	require.Error(t, err)
}

// TestAppWorkspace_DeleteUserSubagent_NonUserScope verifies that deleting a
// project-scope subagent (file under workdir) returns an error.
func TestAppWorkspace_DeleteUserSubagent_NonUserScope(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()

	projectFile := filepath.Join(workDir, ".crush", "subagents", "proj-agent.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(projectFile), 0o755))
	require.NoError(t, os.WriteFile(projectFile,
		[]byte("---\nname: proj-agent\ndescription: Project agent.\n---\n\nBody.\n"),
		0o644,
	))

	projAgent := &subagents.Subagent{Name: "proj-agent", Description: "Project agent.", FilePath: projectFile}
	mgr := subagents.NewManager(
		[]*subagents.Subagent{projAgent},
		[]*subagents.Subagent{projAgent},
		nil,
	)
	t.Cleanup(mgr.Shutdown)

	w := &AppWorkspace{
		app:   &app.App{Subagents: mgr},
		store: newStoreForWorkDir(workDir),
	}

	err := w.DeleteUserSubagent("proj-agent")
	require.Error(t, err)
}

// TestAppWorkspace_DeleteUserSubagent_Success verifies that deleting a
// user-scope subagent removes the file from disk and the agent no longer
// appears in AllSubagents after the internal Manager is reloaded.
func TestAppWorkspace_DeleteUserSubagent_Success(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	userDir := t.TempDir()

	userFile := filepath.Join(userDir, "user-agent.md")
	require.NoError(t, os.WriteFile(userFile,
		[]byte("---\nname: user-agent\ndescription: User agent.\n---\n\nBody.\n"),
		0o644,
	))

	userAgent := &subagents.Subagent{Name: "user-agent", Description: "User agent.", FilePath: userFile}
	mgr := subagents.NewManager(
		[]*subagents.Subagent{userAgent},
		[]*subagents.Subagent{userAgent},
		nil,
	)
	t.Cleanup(mgr.Shutdown)

	w := &AppWorkspace{
		app:   &app.App{Subagents: mgr},
		store: newStoreForWorkDir(workDir),
	}

	err := w.DeleteUserSubagent("user-agent")
	require.NoError(t, err)

	// File must be gone from disk.
	_, statErr := os.Stat(userFile)
	require.True(t, os.IsNotExist(statErr), "file must have been deleted from disk")

	// Manager must no longer contain the deleted agent.
	for _, info := range w.AllSubagents() {
		require.NotEqual(t, "user-agent", info.Name, "deleted agent must not appear in AllSubagents after reload")
	}
}

// TestAppWorkspace_SessionTokens_Found verifies that SessionTokens returns the
// correct token counts for an existing session.
func TestAppWorkspace_SessionTokens_Found(t *testing.T) {
	t.Parallel()

	sessions := &stubSessionService{
		sessions: map[string]session.Session{
			"sess-1": {
				ID:               "sess-1",
				PromptTokens:     42,
				CompletionTokens: 77,
			},
		},
	}

	w := &AppWorkspace{
		app:   &app.App{Sessions: sessions},
		store: config.NewTestStore(&config.Config{}),
	}

	prompt, completion, err := w.SessionTokens(context.Background(), "sess-1")
	require.NoError(t, err)
	require.Equal(t, int64(42), prompt)
	require.Equal(t, int64(77), completion)
}

// TestAppWorkspace_SessionTokens_NotFound verifies that SessionTokens returns
// an error when the session does not exist.
func TestAppWorkspace_SessionTokens_NotFound(t *testing.T) {
	t.Parallel()

	sessions := &stubSessionService{
		sessions: map[string]session.Session{},
	}

	w := &AppWorkspace{
		app:   &app.App{Sessions: sessions},
		store: config.NewTestStore(&config.Config{}),
	}

	_, _, err := w.SessionTokens(context.Background(), "does-not-exist")
	require.Error(t, err)
}
