package workspace

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
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

func (s *stubSessionService) ListChildSessions(context.Context, string) ([]session.Session, error) {
	return nil, nil
}

func (s *stubSessionService) Save(_ context.Context, sess session.Session) (session.Session, error) {
	return sess, nil
}

func (s *stubSessionService) UpdateTitleAndUsage(context.Context, string, string, int64, int64, float64) error {
	return nil
}

func (s *stubSessionService) AddCost(context.Context, string, float64) error {
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

	rt.Register("parent-1", "child-A", "agent-alpha", "blue", "")
	rt.Register("parent-1", "child-B", "agent-beta", "red", "")

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

	rt.Register("parent-1", "child-tok", "agent-tok", "green", "")

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
	require.NoError(t, os.WriteFile(
		projectFile,
		[]byte("---\nname: proj-agent\ndescription: Project agent.\n---\n\nBody.\n"),
		0o644,
	))

	userDir := t.TempDir()
	userFile := filepath.Join(userDir, "user-agent.md")
	require.NoError(t, os.WriteFile(
		userFile,
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

// TestAppWorkspace_AllSubagents_IncludesErrorEntries verifies that definition
// files which failed to parse or validate are still surfaced (with Error set),
// so the Library can show the diagnostic instead of silently dropping them.
func TestAppWorkspace_AllSubagents_IncludesErrorEntries(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()

	goodFile := filepath.Join(workDir, "good.md")
	brokenFile := filepath.Join(workDir, "broken.md")

	goodAgent := &subagents.Subagent{Name: "good", Description: "Good.", FilePath: goodFile}
	mgr := subagents.NewManager(
		[]*subagents.Subagent{goodAgent},
		[]*subagents.Subagent{goodAgent},
		[]*subagents.SubagentState{
			{Name: "good", Path: goodFile, State: subagents.StateNormal},
			{Name: "broken", Path: brokenFile, State: subagents.StateError, Err: errors.New("unclosed frontmatter")},
			// An unnamed error (parse failure) derives its label from the file name.
			{Name: "", Path: filepath.Join(workDir, "garbage.md"), State: subagents.StateError, Err: errors.New("no YAML frontmatter found")},
		},
	)
	t.Cleanup(mgr.Shutdown)

	w := &AppWorkspace{
		app:   &app.App{Subagents: mgr},
		store: newStoreForWorkDir(workDir),
	}

	got := w.AllSubagents()
	byPath := map[string]SubagentDefInfo{}
	for _, info := range got {
		byPath[info.FilePath] = info
	}

	require.Len(t, got, 3)

	require.Empty(t, byPath[goodFile].Error)
	require.Equal(t, "unclosed frontmatter", byPath[brokenFile].Error)
	require.Equal(t, "broken", byPath[brokenFile].Name)

	unnamed := byPath[filepath.Join(workDir, "garbage.md")]
	require.Equal(t, "garbage", unnamed.Name)
	require.Equal(t, "no YAML frontmatter found", unnamed.Error)
}

// TestAppWorkspace_AllSubagents_Deletable verifies that Deletable reflects the
// same trust rule DeleteUserSubagent enforces (InGlobalDir), not the display
// scope: a file in the global user dir is deletable, a project file is not.
// Not parallel: pins the global subagents dir via CRUSH_SUBAGENTS_DIR.
func TestAppWorkspace_AllSubagents_Deletable(t *testing.T) {
	workDir := t.TempDir()
	userDir := t.TempDir()
	t.Setenv("CRUSH_SUBAGENTS_DIR", userDir)

	userFile := filepath.Join(userDir, "user-agent.md")
	require.NoError(t, os.WriteFile(
		userFile,
		[]byte("---\nname: user-agent\ndescription: User agent.\n---\n\nBody.\n"),
		0o644,
	))
	projectFile := filepath.Join(workDir, ".crush", "subagents", "proj-agent.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(projectFile), 0o755))
	require.NoError(t, os.WriteFile(
		projectFile,
		[]byte("---\nname: proj-agent\ndescription: Project agent.\n---\n\nBody.\n"),
		0o644,
	))

	userAgent := &subagents.Subagent{Name: "user-agent", Description: "User agent.", FilePath: userFile}
	projAgent := &subagents.Subagent{Name: "proj-agent", Description: "Project agent.", FilePath: projectFile}
	mgr := subagents.NewManager(
		[]*subagents.Subagent{userAgent, projAgent},
		[]*subagents.Subagent{userAgent, projAgent},
		nil,
	)
	t.Cleanup(mgr.Shutdown)

	w := &AppWorkspace{
		app:   &app.App{Subagents: mgr},
		store: config.NewTestStoreWithWorkingDir(&config.Config{}, workDir),
	}

	byName := map[string]SubagentDefInfo{}
	for _, info := range w.AllSubagents() {
		byName[info.Name] = info
	}

	require.True(t, byName["user-agent"].Deletable, "global-dir subagent must be deletable")
	require.False(t, byName["proj-agent"].Deletable, "project subagent must not be deletable")
}

// TestAppWorkspace_DeleteUserSubagent_SkipsErrorEntries verifies that a broken
// entry sharing a name with a valid one cannot shadow the valid target.
func TestAppWorkspace_DeleteUserSubagent_SkipsErrorEntries(t *testing.T) {
	t.Parallel()

	mgr := subagents.NewManager(nil, nil, []*subagents.SubagentState{
		{Name: "broken", Path: "/nowhere/broken.md", State: subagents.StateError, Err: errors.New("bad")},
	})
	t.Cleanup(mgr.Shutdown)

	w := &AppWorkspace{
		app:   &app.App{Subagents: mgr},
		store: config.NewTestStore(&config.Config{}),
	}

	err := w.DeleteUserSubagent("broken")
	require.ErrorContains(t, err, "not found")
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
	require.NoError(t, os.WriteFile(
		projectFile,
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
// subagent whose file lives in a global (user-scope) subagents directory
// removes the file from disk and the agent no longer appears in AllSubagents
// after the internal Manager is reloaded. Not parallel: pins the global
// subagents dir via CRUSH_SUBAGENTS_DIR.
func TestAppWorkspace_DeleteUserSubagent_Success(t *testing.T) {
	workDir := t.TempDir()
	userDir := t.TempDir()
	t.Setenv("CRUSH_SUBAGENTS_DIR", userDir)

	userFile := filepath.Join(userDir, "user-agent.md")
	require.NoError(t, os.WriteFile(
		userFile,
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

// TestAppWorkspace_DeleteUserSubagent_OutsideGlobalDirsRefused verifies that a
// subagent that merely *displays* as user scope (its file lives in a custom
// discovery path, not in a global subagents dir) is refused deletion — scope
// labeling is display-oriented and must not authorize file removal.
func TestAppWorkspace_DeleteUserSubagent_OutsideGlobalDirsRefused(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	customDir := t.TempDir()

	customFile := filepath.Join(customDir, "custom-agent.md")
	require.NoError(t, os.WriteFile(
		customFile,
		[]byte("---\nname: custom-agent\ndescription: Custom path agent.\n---\n\nBody.\n"),
		0o644,
	))

	customAgent := &subagents.Subagent{Name: "custom-agent", Description: "Custom path agent.", FilePath: customFile}
	mgr := subagents.NewManager(
		[]*subagents.Subagent{customAgent},
		[]*subagents.Subagent{customAgent},
		nil,
	)
	t.Cleanup(mgr.Shutdown)

	w := &AppWorkspace{
		app:   &app.App{Subagents: mgr},
		store: newStoreForWorkDir(workDir),
	}

	require.Error(t, w.DeleteUserSubagent("custom-agent"))
	_, statErr := os.Stat(customFile)
	require.NoError(t, statErr, "the file must not have been deleted")
}

// TestAppWorkspace_AllSubagents_MonorepoRootIsProjectScope verifies that a
// subagent discovered at the git worktree root — outside the working
// directory — is labeled project scope, not user scope (a user-scope label
// previously made repo-tracked files deletable).
func TestAppWorkspace_AllSubagents_MonorepoRootIsProjectScope(t *testing.T) {
	t.Parallel()

	// Resolve symlinks up front: macOS temp dirs live behind /private
	// symlinks, and git rev-parse --show-toplevel (what worktreeRoot uses)
	// reports the resolved path — file paths must match that view.
	repoRoot, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	out, err := exec.Command("git", "init", repoRoot).CombinedOutput()
	require.NoError(t, err, "git init: %s", out)

	workDir := filepath.Join(repoRoot, "apps", "web")
	require.NoError(t, os.MkdirAll(workDir, 0o755))

	rootFile := filepath.Join(repoRoot, ".agents", "subagents", "root-agent.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(rootFile), 0o755))
	require.NoError(t, os.WriteFile(
		rootFile,
		[]byte("---\nname: root-agent\ndescription: Monorepo root agent.\n---\n\nBody.\n"),
		0o644,
	))

	rootAgent := &subagents.Subagent{Name: "root-agent", Description: "Monorepo root agent.", FilePath: rootFile}
	mgr := subagents.NewManager(
		[]*subagents.Subagent{rootAgent},
		[]*subagents.Subagent{rootAgent},
		nil,
	)
	t.Cleanup(mgr.Shutdown)

	w := &AppWorkspace{
		app:   &app.App{Subagents: mgr},
		store: newStoreForWorkDir(workDir),
	}

	got := w.AllSubagents()
	require.Len(t, got, 1)
	require.Equal(t, "project", got[0].Scope, "a worktree-root subagent must be project scope")
	require.Error(t, w.DeleteUserSubagent("root-agent"), "a worktree-root subagent must not be deletable")
}

// TestAddOrRemove covers the pure list helper backing SetSubagentDisabled.
func TestAddOrRemove(t *testing.T) {
	t.Parallel()

	require.Equal(t, []string{"a"}, addOrRemove(nil, "a", true), "add to empty")
	require.Equal(t, []string{"a"}, addOrRemove([]string{"a"}, "a", true), "add dedups")
	require.Equal(t, []string{}, addOrRemove([]string{"a"}, "a", false), "remove last")
	require.Equal(t, []string{"b", "c"}, addOrRemove([]string{"b", "a", "c", "a"}, "a", false),
		"remove drops all occurrences, keeps others")
	require.Equal(t, []string{"b", "a"}, addOrRemove([]string{"b"}, "a", true), "add appends")
}

// TestAppWorkspace_DeleteUserSubagent_ReloadValidatesModel verifies that the
// reload after a delete validates model ids (passes cfg.IsKnownModel, not
// nil). A subagent referencing an unknown model must NOT become active after
// the reload — with a nil validator it would be wrongly accepted. Not
// parallel: pins the global subagents dir via CRUSH_SUBAGENTS_DIR so the
// delete passes the user-scope check.
func TestAppWorkspace_DeleteUserSubagent_ReloadValidatesModel(t *testing.T) {
	workDir := t.TempDir()
	userDir := t.TempDir()
	t.Setenv("CRUSH_SUBAGENTS_DIR", userDir)

	// One valid user subagent to delete, and one with an unknown model id.
	keepFile := filepath.Join(userDir, "keep-agent.md")
	require.NoError(t, os.WriteFile(
		keepFile,
		[]byte("---\nname: keep-agent\ndescription: Keep.\n---\n\nBody.\n"),
		0o644,
	))
	badFile := filepath.Join(userDir, "bad-agent.md")
	require.NoError(t, os.WriteFile(
		badFile,
		[]byte("---\nname: bad-agent\ndescription: Bad model.\nmodel: not-a-real-model-id\n---\n\nBody.\n"),
		0o644,
	))

	keepAgent := &subagents.Subagent{Name: "keep-agent", Description: "Keep.", FilePath: keepFile}
	mgr := subagents.NewManager(
		[]*subagents.Subagent{keepAgent},
		[]*subagents.Subagent{keepAgent},
		nil,
	)
	t.Cleanup(mgr.Shutdown)

	// Empty (but non-nil) providers => IsKnownModelID returns false for any
	// specific id, so bad-agent must be rejected on reload. SubagentsPaths
	// drives rediscovery.
	cfg := &config.Config{
		Options:   &config.Options{SubagentsPaths: []string{userDir}},
		Providers: csync.NewMap[string, config.ProviderConfig](),
	}
	w := &AppWorkspace{
		app:   &app.App{Subagents: mgr},
		store: config.NewTestStoreWithWorkingDir(cfg, workDir),
	}

	require.NoError(t, w.DeleteUserSubagent("keep-agent"))

	// bad-agent must not be a valid entry after the reload. It may still be
	// surfaced as an error diagnostic (Error set), which is how the Library
	// reports a definition that failed validation.
	for _, info := range w.AllSubagents() {
		if info.Name == "bad-agent" {
			require.NotEmpty(t, info.Error,
				"subagent with an unknown model must only appear as an error entry after reload")
		}
	}
}
