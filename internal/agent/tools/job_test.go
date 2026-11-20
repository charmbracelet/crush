package tools

import (
	"context"
	"encoding/json"
	"errors"
	"runtime"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/shell"
	"github.com/stretchr/testify/require"
)

const testMaxJobs = 50

func TestBackgroundShell_Integration(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	ctx := context.Background()

	// Start a background shell
	bgManager := shell.GetBackgroundShellManager()
	bgShell, err := bgManager.Start(ctx, shell.JobScope{}, workingDir, nil, shell.CoreutilsTurtle, "echo 'hello background' && echo 'done'", "", testMaxJobs)
	require.NoError(t, err)
	require.NotEmpty(t, bgShell.ID)

	// Wait for completion
	bgShell.Wait()

	// Check final output
	stdout, stderr, done, err := bgShell.GetOutput()
	require.NoError(t, err)
	require.Contains(t, stdout, "hello background")
	require.Contains(t, stdout, "done")
	require.True(t, done)
	require.Empty(t, stderr)

	// Clean up
	bgManager.Kill(ctx, bgShell.ID)
}

func TestBackgroundShell_Kill(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	ctx := context.Background()

	// Start a long-running background shell
	bgManager := shell.GetBackgroundShellManager()
	bgShell, err := bgManager.Start(ctx, shell.JobScope{}, workingDir, nil, shell.CoreutilsTurtle, "sleep 100", "", testMaxJobs)
	require.NoError(t, err)

	// Kill it
	err = bgManager.Kill(ctx, bgShell.ID)
	require.NoError(t, err)

	// Verify it's gone
	_, ok := bgManager.Get(bgShell.ID)
	require.False(t, ok)

	// Verify the shell is done
	require.True(t, bgShell.IsDone())
}

func TestBackgroundShell_MultipleOutputCalls(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	ctx := context.Background()

	// Start a background shell
	bgManager := shell.GetBackgroundShellManager()
	bgShell, err := bgManager.Start(ctx, shell.JobScope{}, workingDir, nil, shell.CoreutilsTurtle, "echo 'step 1' && echo 'step 2' && echo 'step 3'", "", testMaxJobs)
	require.NoError(t, err)
	defer bgManager.Kill(ctx, bgShell.ID)

	// Check that we can call GetOutput multiple times while running
	for range 5 {
		_, _, done, _ := bgShell.GetOutput()
		if done {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for completion
	bgShell.Wait()

	// Multiple calls after completion should return the same result
	stdout1, _, done1, _ := bgShell.GetOutput()
	require.True(t, done1)
	require.Contains(t, stdout1, "step 1")
	require.Contains(t, stdout1, "step 2")
	require.Contains(t, stdout1, "step 3")

	stdout2, _, done2, _ := bgShell.GetOutput()
	require.True(t, done2)
	require.Equal(t, stdout1, stdout2, "Multiple GetOutput calls should return same result")
}

func TestBackgroundShell_EmptyOutput(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("This test is flacky on Windows for some reason")
	}

	workingDir := t.TempDir()
	ctx := context.Background()

	// Start a background shell with no output
	bgManager := shell.GetBackgroundShellManager()
	bgShell, err := bgManager.Start(ctx, shell.JobScope{}, workingDir, nil, shell.CoreutilsTurtle, "sleep 0.1", "", testMaxJobs)
	require.NoError(t, err)
	defer bgManager.Kill(ctx, bgShell.ID)

	// Wait for completion
	bgShell.Wait()

	stdout, stderr, done, err := bgShell.GetOutput()
	require.NoError(t, err)
	require.Empty(t, stdout)
	require.Empty(t, stderr)
	require.True(t, done)
}

func TestBackgroundShell_ExitCode(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	ctx := context.Background()

	// Start a background shell that exits with non-zero code
	bgManager := shell.GetBackgroundShellManager()
	bgShell, err := bgManager.Start(ctx, shell.JobScope{}, workingDir, nil, shell.CoreutilsTurtle, "echo 'failing' && exit 42", "", testMaxJobs)
	require.NoError(t, err)
	defer bgManager.Kill(ctx, bgShell.ID)

	// Wait for completion
	bgShell.Wait()

	stdout, _, done, execErr := bgShell.GetOutput()
	require.True(t, done)
	require.Contains(t, stdout, "failing")
	require.Error(t, execErr)

	exitCode := shell.ExitCode(execErr)
	require.Equal(t, 42, exitCode)
}

func TestBackgroundShell_WithBlockFuncs(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	ctx := context.Background()

	blockFuncs := []shell.BlockFunc{
		shell.CommandsBlocker([]string{"curl", "wget"}),
	}

	// Start a background shell with a blocked command
	bgManager := shell.GetBackgroundShellManager()
	bgShell, err := bgManager.Start(ctx, shell.JobScope{}, workingDir, blockFuncs, shell.CoreutilsTurtle, "curl example.com", "", testMaxJobs)
	require.NoError(t, err)
	defer bgManager.Kill(ctx, bgShell.ID)

	// Wait for completion
	bgShell.Wait()

	stdout, stderr, done, execErr := bgShell.GetOutput()
	require.True(t, done)

	// The command should have been blocked, check stderr or error
	if execErr != nil {
		// Error might contain the message
		require.Contains(t, execErr.Error(), "is a dangerous command")
	} else {
		// Or it might be in stderr
		output := stdout + stderr
		require.Contains(t, output, "is a dangerous command")
	}
}

func TestBackgroundShell_StdoutAndStderr(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	ctx := context.Background()

	// Start a background shell with both stdout and stderr
	bgManager := shell.GetBackgroundShellManager()
	bgShell, err := bgManager.Start(ctx, shell.JobScope{}, workingDir, nil, shell.CoreutilsTurtle, "echo 'stdout message' && echo 'stderr message' >&2", "", testMaxJobs)
	require.NoError(t, err)
	defer bgManager.Kill(ctx, bgShell.ID)

	// Wait for completion
	bgShell.Wait()

	stdout, stderr, done, err := bgShell.GetOutput()
	require.NoError(t, err)
	require.True(t, done)
	require.Contains(t, stdout, "stdout message")
	require.Contains(t, stderr, "stderr message")
}

func TestBackgroundShell_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	ctx := context.Background()

	// Start a background shell
	bgManager := shell.GetBackgroundShellManager()
	bgShell, err := bgManager.Start(ctx, shell.JobScope{}, workingDir, nil, shell.CoreutilsTurtle, "for i in 1 2 3 4 5; do echo \"line $i\"; sleep 0.05; done", "", testMaxJobs)
	require.NoError(t, err)
	defer bgManager.Kill(ctx, bgShell.ID)

	// Access output concurrently from multiple goroutines
	done := make(chan struct{})
	errors := make(chan error, 10)

	for range 10 {
		go func() {
			for {
				select {
				case <-done:
					return
				default:
					_, _, _, err := bgShell.GetOutput()
					if err != nil {
						errors <- err
					}
					dir := bgShell.WorkingDir
					if dir == "" {
						errors <- err
					}
					time.Sleep(10 * time.Millisecond)
				}
			}
		}()
	}

	// Let it run for a bit
	time.Sleep(300 * time.Millisecond)
	close(done)

	// Check for any errors
	select {
	case err := <-errors:
		t.Fatalf("Concurrent access caused error: %v", err)
	case <-time.After(100 * time.Millisecond):
		// No errors - success
	}
}

func TestBackgroundShell_List(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	ctx := context.Background()

	bgManager := shell.GetBackgroundShellManager()

	// Start multiple background shells
	shells := make([]*shell.BackgroundShell, 3)
	for i := range 3 {
		bgShell, err := bgManager.Start(ctx, shell.JobScope{}, workingDir, nil, shell.CoreutilsTurtle, "sleep 1", "", testMaxJobs)
		require.NoError(t, err)
		shells[i] = bgShell
	}

	// Get the list
	ids := bgManager.List()

	// Verify all our shells are in the list
	for _, sh := range shells {
		require.Contains(t, ids, sh.ID, "Shell %s not found in list", sh.ID)
	}

	// Clean up
	for _, sh := range shells {
		bgManager.Kill(ctx, sh.ID)
	}
}

func TestBackgroundShell_AutoBackground(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	ctx := context.Background()

	// Test that a quick command completes synchronously
	t.Run("quick command completes synchronously", func(t *testing.T) {
		t.Parallel()
		bgManager := shell.GetBackgroundShellManager()
		bgShell, err := bgManager.Start(ctx, shell.JobScope{}, workingDir, nil, shell.CoreutilsTurtle, "echo 'quick'", "", testMaxJobs)
		require.NoError(t, err)

		// Wait threshold time
		time.Sleep(5 * time.Second)

		// Should be done by now
		stdout, stderr, done, err := bgShell.GetOutput()
		require.NoError(t, err)
		require.True(t, done, "Quick command should be done")
		require.Contains(t, stdout, "quick")
		require.Empty(t, stderr)

		// Clean up
		bgManager.Kill(ctx, bgShell.ID)
	})

	// Test that a long command stays in background
	t.Run("long command stays in background", func(t *testing.T) {
		t.Parallel()
		bgManager := shell.GetBackgroundShellManager()
		bgShell, err := bgManager.Start(ctx, shell.JobScope{}, workingDir, nil, shell.CoreutilsTurtle, "sleep 20 && echo '20 seconds completed'", "", testMaxJobs)
		require.NoError(t, err)
		defer bgManager.Kill(ctx, bgShell.ID)

		// Wait threshold time
		time.Sleep(5 * time.Second)

		// Should still be running
		stdout, stderr, done, err := bgShell.GetOutput()
		require.NoError(t, err)
		require.False(t, done, "Long command should still be running")
		require.Empty(t, stdout, "No output yet from sleep command")
		require.Empty(t, stderr)

		// Verify we can get the shell from manager
		retrieved, ok := bgManager.Get(bgShell.ID)
		require.True(t, ok, "Should be able to retrieve background shell")
		require.Equal(t, bgShell.ID, retrieved.ID)
	})
}

type fakeSessions map[string]session.Session

func (f fakeSessions) Get(_ context.Context, id string) (session.Session, error) {
	if sess, ok := f[id]; ok {
		return sess, nil
	}
	return session.Session{}, errors.New("session not found")
}

func sessionCtx(t *testing.T, id string) context.Context {
	return context.WithValue(t.Context(), SessionIDContextKey, id)
}

func mustScope(t *testing.T, jobs BackgroundJobs, id string) shell.JobScope {
	t.Helper()
	scope, err := jobs.Scope(sessionCtx(t, id))
	require.NoError(t, err)
	return scope
}

// TestJobTools_ScopedToSessionTree checks that the job tools see the jobs
// their owner started in the current session and its subagents, and
// nobody else's.
func TestJobTools_ScopedToSessionTree(t *testing.T) {
	t.Parallel()

	owner := new(int)
	sessions := fakeSessions{
		"parent": {ID: "parent"},
		"child":  {ID: "child", ParentSessionID: "parent"},
		"other":  {ID: "other"},
	}
	jobs := NewBackgroundJobs(owner, sessions, new(session.Roots))

	bgManager := shell.GetBackgroundShellManager()
	started, err := bgManager.Start(t.Context(), mustScope(t, jobs, "child"), t.TempDir(), nil, shell.CoreutilsTurtle, "sleep 10", "child job", testMaxJobs)
	require.NoError(t, err)
	t.Cleanup(func() { _ = bgManager.Kill(context.Background(), started.ID) })
	started.MarkBackground()
	foreign, err := bgManager.Start(t.Context(), mustScope(t, jobs, "other"), t.TempDir(), nil, shell.CoreutilsTurtle, "sleep 10", "", testMaxJobs)
	require.NoError(t, err)
	t.Cleanup(func() { _ = bgManager.Kill(context.Background(), foreign.ID) })
	foreign.MarkBackground()

	list := NewJobListTool(jobs)
	resp, err := list.Run(sessionCtx(t, "parent"), fantasy.ToolCall{ID: "1", Name: JobListToolName, Input: "{}"})
	require.NoError(t, err)
	require.Contains(t, resp.Content, started.ID)
	require.Contains(t, resp.Content, "running for")
	require.Contains(t, resp.Content, "(child job)")
	require.NotContains(t, resp.Content, foreign.ID)

	kill := NewJobKillTool(jobs)
	resp, err = kill.Run(sessionCtx(t, "parent"), fantasy.ToolCall{ID: "2", Name: JobKillToolName, Input: `{"shell_id":"` + foreign.ID + `"}`})
	require.NoError(t, err)
	require.True(t, resp.IsError, "another session's job must not be reachable")
	require.False(t, foreign.IsDone())

	resp, err = kill.Run(sessionCtx(t, "parent"), fantasy.ToolCall{ID: "3", Name: JobKillToolName, Input: `{"shell_id":"` + started.ID + `"}`})
	require.NoError(t, err)
	require.False(t, resp.IsError, resp.Content)
	require.True(t, started.IsDone())
}

// TestJobTools_ForegroundShellUnreachable guards that a shell still running
// in the foreground cannot be read or killed by ID, even from its own
// session, until it moves to the background.
func TestJobTools_ForegroundShellUnreachable(t *testing.T) {
	t.Parallel()

	jobs := NewBackgroundJobs(new(int), nil, nil)
	bgManager := shell.GetBackgroundShellManager()
	foreground, err := bgManager.Start(t.Context(), mustScope(t, jobs, "s"), t.TempDir(), nil, shell.CoreutilsTurtle, "sleep 10", "", testMaxJobs)
	require.NoError(t, err)
	t.Cleanup(func() { _ = bgManager.Kill(context.Background(), foreground.ID) })

	input := `{"shell_id":"` + foreground.ID + `"}`
	resp, err := NewJobOutputTool(t.TempDir(), jobs, nil).Run(sessionCtx(t, "s"), fantasy.ToolCall{ID: "1", Name: JobOutputToolName, Input: input})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	resp, err = NewJobKillTool(jobs).Run(sessionCtx(t, "s"), fantasy.ToolCall{ID: "2", Name: JobKillToolName, Input: input})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.False(t, foreground.IsDone())

	foreground.MarkBackground()
	resp, err = NewJobKillTool(jobs).Run(sessionCtx(t, "s"), fantasy.ToolCall{ID: "3", Name: JobKillToolName, Input: input})
	require.NoError(t, err)
	require.False(t, resp.IsError, resp.Content)
}

// TestBackgroundJobs_ScopeRefusesToGuess guards that a job is never tagged
// with a guessed session: no session, or a session whose top-level session
// cannot be found, is an error rather than a narrower scope.
func TestBackgroundJobs_ScopeRefusesToGuess(t *testing.T) {
	t.Parallel()

	jobs := NewBackgroundJobs(new(int), fakeSessions{}, new(session.Roots))

	_, err := jobs.Scope(t.Context())
	require.ErrorIs(t, err, errNoSession)

	_, err = jobs.Scope(sessionCtx(t, "missing"))
	require.ErrorContains(t, err, "could not find the session")

	resp, err := NewBashTool(&mockBashPermissionService{Broker: pubsub.NewBroker[permission.PermissionRequest]()}, t.TempDir(), t.TempDir(), &config.Attribution{TrailerStyle: config.TrailerStyleNone}, "test-model", testMaxJobs, shell.CoreutilsTurtle, nil, jobs, nil).
		Run(sessionCtx(t, "missing"), fantasy.ToolCall{ID: "1", Name: BashToolName, Input: `{"command":"echo hi","description":"hi"}`})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "could not find the session")
}

func runJobOutput(t *testing.T, ctx context.Context, jobs BackgroundJobs, waitLimit time.Duration, shellID string) fantasy.ToolResponse {
	t.Helper()
	input, err := json.Marshal(JobOutputParams{ShellID: shellID, Wait: true})
	require.NoError(t, err)
	tool := newJobOutputTool(t.TempDir(), jobs, nil, waitLimit)
	resp, err := tool.Run(ctx, fantasy.ToolCall{ID: "call", Name: JobOutputToolName, Input: string(input)})
	require.NoError(t, err)
	return resp
}

func TestJobOutput_WaitLimit(t *testing.T) {
	t.Parallel()

	jobs := NewBackgroundJobs(new(int), nil, nil)
	bgManager := shell.GetBackgroundShellManager()

	t.Run("gives up on a job that outlives the limit", func(t *testing.T) {
		t.Parallel()
		bgShell, err := bgManager.Start(t.Context(), mustScope(t, jobs, "s"), t.TempDir(), nil, shell.CoreutilsTurtle, "echo started && sleep 100", "", testMaxJobs)
		require.NoError(t, err)
		defer bgManager.Kill(context.Background(), bgShell.ID)
		bgShell.MarkBackground()

		resp := runJobOutput(t, sessionCtx(t, "s"), jobs, 2*time.Second, bgShell.ID)
		require.Contains(t, resp.Content, "Status: running")
		require.Contains(t, resp.Content, "started")
		require.Contains(t, resp.Content, "Stopped waiting")
		require.False(t, bgShell.IsDone())
	})

	t.Run("returns the result when the job finishes first", func(t *testing.T) {
		t.Parallel()
		bgShell, err := bgManager.Start(t.Context(), mustScope(t, jobs, "s"), t.TempDir(), nil, shell.CoreutilsTurtle, "echo hi", "", testMaxJobs)
		require.NoError(t, err)
		defer bgManager.Kill(context.Background(), bgShell.ID)
		bgShell.MarkBackground()

		resp := runJobOutput(t, sessionCtx(t, "s"), jobs, time.Minute, bgShell.ID)
		require.Contains(t, resp.Content, "Status: completed")
		require.NotContains(t, resp.Content, "Stopped waiting")
	})

	t.Run("cancellation does not claim the limit ran out", func(t *testing.T) {
		t.Parallel()
		bgShell, err := bgManager.Start(t.Context(), mustScope(t, jobs, "s"), t.TempDir(), nil, shell.CoreutilsTurtle, "sleep 100", "", testMaxJobs)
		require.NoError(t, err)
		defer bgManager.Kill(context.Background(), bgShell.ID)
		bgShell.MarkBackground()

		cctx, cancel := context.WithCancel(sessionCtx(t, "s"))
		time.AfterFunc(100*time.Millisecond, cancel)
		resp := runJobOutput(t, cctx, jobs, time.Minute, bgShell.ID)
		require.Contains(t, resp.Content, "Status: running")
		require.NotContains(t, resp.Content, "Stopped waiting")
	})
}
