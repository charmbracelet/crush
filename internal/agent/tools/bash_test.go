package tools

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/shell"
	"github.com/stretchr/testify/require"
)

type mockBashPermissionService struct {
	*pubsub.Broker[permission.PermissionRequest]
}

func (m *mockBashPermissionService) Request(ctx context.Context, req permission.CreatePermissionRequest) (bool, error) {
	return true, nil
}

func (m *mockBashPermissionService) Grant(req permission.PermissionRequest) {}

func (m *mockBashPermissionService) Deny(req permission.PermissionRequest) {}

func (m *mockBashPermissionService) GrantPersistent(req permission.PermissionRequest) {}

func (m *mockBashPermissionService) AutoApproveSession(sessionID string) {}

func (m *mockBashPermissionService) SetSessionAutoApprove(sessionID string, enabled bool) {}

func (m *mockBashPermissionService) IsSessionAutoApprove(sessionID string) bool {
	return false
}

func (m *mockBashPermissionService) SetSkipRequests(skip bool) {}

func (m *mockBashPermissionService) SkipRequests() bool {
	return false
}

func (m *mockBashPermissionService) SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[permission.PermissionNotification] {
	return make(<-chan pubsub.Event[permission.PermissionNotification])
}

func TestBashTool_DefaultAutoBackgroundThreshold(t *testing.T) {
	workingDir := t.TempDir()
	tool := newBashToolForTest(workingDir)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

	resp := runBashTool(t, tool, ctx, BashParams{
		Description: "default threshold",
		Command:     "echo done",
	})

	require.False(t, resp.IsError)
	var meta BashResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
	require.False(t, meta.Background)
	require.Empty(t, meta.ShellID)
	require.Contains(t, meta.Output, "done")
}

func TestBashTool_CustomAutoBackgroundThreshold(t *testing.T) {
	workingDir := t.TempDir()
	tool := newBashToolForTest(workingDir)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

	resp := runBashTool(t, tool, ctx, BashParams{
		Description:         "custom threshold",
		Command:             "sleep 1.5 && echo done",
		AutoBackgroundAfter: 1,
	})

	require.False(t, resp.IsError)
	var meta BashResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
	require.True(t, meta.Background)
	require.NotEmpty(t, meta.ShellID)
	require.Contains(t, resp.Content, "moved to background")

	bgManager := shell.GetBackgroundShellManager()
	require.NoError(t, bgManager.Kill(meta.ShellID))
}

func TestBashTool_HookPassthroughFallsBackToOriginalCommand(t *testing.T) {
	workingDir := t.TempDir()
	rewriteHook := helperBinary(t, "rewrite-hook", `package main
import (
	"fmt"
	"os"
)
func main() {
	if len(os.Args) < 2 {
		os.Exit(1)
	}
	fmt.Print("false")
}`)

	enabled := true
	hookMgr, err := hooks.NewManager([]hooks.HookConfig{
		{
			Name:    "rewrite",
			Enabled: &enabled,
			Events:  []hooks.Event{hooks.EventPreToolUse},
			Type:    hooks.HandlerTypeCommand,
			Command: &hooks.CommandConfig{
				Command:     rewriteHook,
				Passthrough: true,
			},
		},
	})
	require.NoError(t, err)

	tool := newBashToolForTestWithHooks(workingDir, hookMgr)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

	resp := runBashTool(t, tool, ctx, BashParams{
		Description: "hook fallback",
		Command:     "echo done",
	})

	require.False(t, resp.IsError)

	var meta BashResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
	require.Contains(t, meta.Output, "done")
	require.NotContains(t, meta.Output, "Exit code 1")
}

func newBashToolForTest(workingDir string) fantasy.AgentTool {
	return newBashToolForTestWithHooks(workingDir, nil)
}

func newBashToolForTestWithHooks(workingDir string, hookMgr *hooks.Manager) fantasy.AgentTool {
	return newBashToolForTestWithHooksAndOptions(workingDir, hookMgr)
}

func newBashToolForTestWithHooksAndOptions(workingDir string, hookMgr *hooks.Manager, opts ...BashToolOptions) fantasy.AgentTool {
	permissions := &mockBashPermissionService{Broker: pubsub.NewBroker[permission.PermissionRequest]()}
	attribution := &config.Attribution{TrailerStyle: config.TrailerStyleNone}
	return NewBashTool(permissions, workingDir, attribution, "test-model", hookMgr, opts...)
}

func runBashTool(t *testing.T, tool fantasy.AgentTool, ctx context.Context, params BashParams) fantasy.ToolResponse {
	t.Helper()

	input, err := json.Marshal(params)
	require.NoError(t, err)

	call := fantasy.ToolCall{
		ID:    "test-call",
		Name:  BashToolName,
		Input: string(input),
	}

	resp, err := tool.Run(ctx, call)
	require.NoError(t, err)
	return resp
}

func helperBinary(t *testing.T, name, src string) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not found, skipping")
	}

	dir := t.TempDir()
	srcFile := filepath.Join(dir, "main.go")
	require.NoError(t, os.WriteFile(srcFile, []byte(src), 0o644))

	binName := name
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(dir, binName)
	out, err := exec.CommandContext(t.Context(), "go", "build", "-o", binPath, srcFile).CombinedOutput()
	require.NoError(t, err, "build helper binary: %s", out)
	return binPath
}

func TestRestrictedGitBashTool_AllowsReadOnlyGitCommands(t *testing.T) {
	repoDir := initGitRepoForTest(t)
	tool := newBashToolForTestWithHooksAndOptions(repoDir, nil, BashToolOptions{
		RestrictedToGitReadOnly: true,
		DisableBackground:       true,
		DescriptionOverride:     RestrictedGitBashDescription(),
	})
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

	resp := runBashTool(t, tool, ctx, BashParams{
		Description: "inspect git status",
		Command:     "git status --short",
	})
	require.False(t, resp.IsError)
	require.Contains(t, resp.Content, "tracked.txt")

	resp = runBashTool(t, tool, ctx, BashParams{
		Description: "inspect git diff",
		Command:     "git diff -- tracked.txt",
	})
	require.False(t, resp.IsError)
	require.Contains(t, resp.Content, "+after")
}

func TestRestrictedGitBashTool_BlocksUnsafeCommands(t *testing.T) {
	repoDir := initGitRepoForTest(t)
	tool := newBashToolForTestWithHooksAndOptions(repoDir, nil, BashToolOptions{
		RestrictedToGitReadOnly: true,
		DisableBackground:       true,
		DescriptionOverride:     RestrictedGitBashDescription(),
	})
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

	cases := []string{
		"git checkout main",
		"git restore .",
		"bash -lc \"git diff\"",
		"git diff > out.txt",
		"git diff --output=out.txt",
		"git diff --output out.txt",
		"git diff --out=out.txt",
		"git diff --outp out.txt",
		"git diff --outpu=evil.txt",
	}

	for _, command := range cases {
		resp := runBashTool(t, tool, ctx, BashParams{
			Description: "unsafe",
			Command:     command,
		})
		require.True(t, resp.IsError, command)
	}
}

func TestRestrictedGitBashTool_DisablesBackgroundExecution(t *testing.T) {
	repoDir := initGitRepoForTest(t)
	tool := newBashToolForTestWithHooksAndOptions(repoDir, nil, BashToolOptions{
		RestrictedToGitReadOnly: true,
		DisableBackground:       true,
		DescriptionOverride:     RestrictedGitBashDescription(),
	})
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

	resp := runBashTool(t, tool, ctx, BashParams{
		Description:     "background",
		Command:         "git status --short",
		RunInBackground: true,
	})
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "background execution is disabled")
}

func initGitRepoForTest(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found, skipping")
	}

	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "config", "user.email", "test@example.com")

	tracked := filepath.Join(repoDir, "tracked.txt")
	require.NoError(t, os.WriteFile(tracked, []byte("before\n"), 0o644))
	runGit(t, repoDir, "add", "tracked.txt")
	runGit(t, repoDir, "commit", "-m", "init")

	require.NoError(t, os.WriteFile(tracked, []byte("before\nafter\n"), 0o644))
	return repoDir
}

func runGit(t *testing.T, repoDir string, args ...string) {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
}
