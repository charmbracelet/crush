package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/shell"
	"github.com/stretchr/testify/require"
)

func jobOutputMetadata(t *testing.T, resp fantasy.ToolResponse) JobOutputResponseMetadata {
	t.Helper()
	var meta JobOutputResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
	return meta
}

func TestJobOutputOffsetReturnsOnlyNewOutput(t *testing.T) {
	t.Parallel()

	bgManager := shell.GetBackgroundShellManager()
	bgShell, err := bgManager.Start(t.Context(), shell.StartOptions{
		WorkingDir: t.TempDir(),
		Command:    "echo first; sleep 0.5; echo second",
	})
	require.NoError(t, err)
	t.Cleanup(func() { bgManager.Kill(bgShell.ID) })

	tool := NewJobOutputTool()
	ctx := context.WithValue(t.Context(), SessionIDContextKey, "session-1")

	// First poll: still running, and it hands back where to resume.
	var firstMeta JobOutputResponseMetadata
	require.Eventually(t, func() bool {
		resp, err := tool.Run(ctx, fantasy.ToolCall{Input: `{"shell_id":"` + bgShell.ID + `"}`})
		if err != nil {
			return false
		}
		if !strings.Contains(resp.Content, "first") {
			return false
		}
		firstMeta = jobOutputMetadata(t, resp)
		return true
	}, 5*time.Second, 25*time.Millisecond)

	require.Positive(t, firstMeta.NextOffset)
	require.False(t, firstMeta.Done)
	require.NotEmpty(t, firstMeta.LogPath)

	bgShell.Wait()

	// Second poll from that offset shows the new line and not the old one.
	input, err := json.Marshal(JobOutputParams{ShellID: bgShell.ID, Offset: firstMeta.NextOffset})
	require.NoError(t, err)
	resp, err := tool.Run(ctx, fantasy.ToolCall{Input: string(input)})
	require.NoError(t, err)

	require.Contains(t, resp.Content, "second")
	require.NotContains(t, resp.Content, "first", "an offset read must not repeat earlier output")
	require.Contains(t, resp.Content, "Status: completed")

	meta := jobOutputMetadata(t, resp)
	require.True(t, meta.Done)
	require.Greater(t, meta.NextOffset, firstMeta.NextOffset)
}

func TestJobOutputOffsetAtEndReportsNoNewOutput(t *testing.T) {
	t.Parallel()

	bgManager := shell.GetBackgroundShellManager()
	bgShell, err := bgManager.Start(t.Context(), shell.StartOptions{
		WorkingDir: t.TempDir(),
		Command:    "echo only",
	})
	require.NoError(t, err)
	t.Cleanup(func() { bgManager.Kill(bgShell.ID) })
	bgShell.Wait()

	tool := NewJobOutputTool()
	ctx := context.WithValue(t.Context(), SessionIDContextKey, "session-1")

	resp, err := tool.Run(ctx, fantasy.ToolCall{Input: `{"shell_id":"` + bgShell.ID + `"}`})
	require.NoError(t, err)
	meta := jobOutputMetadata(t, resp)

	input, err := json.Marshal(JobOutputParams{ShellID: bgShell.ID, Offset: meta.NextOffset})
	require.NoError(t, err)
	resp, err = tool.Run(ctx, fantasy.ToolCall{Input: string(input)})
	require.NoError(t, err)
	require.Contains(t, resp.Content, "no new output")
}

func TestJobOutputReportsExitCode(t *testing.T) {
	t.Parallel()

	bgManager := shell.GetBackgroundShellManager()
	bgShell, err := bgManager.Start(t.Context(), shell.StartOptions{
		WorkingDir: t.TempDir(),
		Command:    "echo nope; exit 9",
	})
	require.NoError(t, err)
	t.Cleanup(func() { bgManager.Kill(bgShell.ID) })
	bgShell.Wait()

	tool := NewJobOutputTool()
	ctx := context.WithValue(t.Context(), SessionIDContextKey, "session-1")
	resp, err := tool.Run(ctx, fantasy.ToolCall{Input: `{"shell_id":"` + bgShell.ID + `"}`})
	require.NoError(t, err)

	require.Contains(t, resp.Content, "Status: completed")
	require.Contains(t, resp.Content, "Exit code 9")
}

func TestJobListShowsOnlyThisSessionsJobs(t *testing.T) {
	t.Parallel()

	bgManager := shell.GetBackgroundShellManager()

	mine, err := bgManager.Start(t.Context(), shell.StartOptions{
		WorkingDir:  t.TempDir(),
		Command:     "sleep 30",
		Description: "my job",
		SessionID:   "list-session-mine",
	})
	require.NoError(t, err)
	t.Cleanup(func() { bgManager.Kill(mine.ID) })

	theirs, err := bgManager.Start(t.Context(), shell.StartOptions{
		WorkingDir:  t.TempDir(),
		Command:     "sleep 30",
		Description: "their job",
		SessionID:   "list-session-theirs",
	})
	require.NoError(t, err)
	t.Cleanup(func() { bgManager.Kill(theirs.ID) })

	tool := NewJobListTool()
	ctx := context.WithValue(t.Context(), SessionIDContextKey, "list-session-mine")
	resp, err := tool.Run(ctx, fantasy.ToolCall{Input: `{}`})
	require.NoError(t, err)

	require.Contains(t, resp.Content, mine.ID)
	require.Contains(t, resp.Content, "my job")
	require.NotContains(t, resp.Content, theirs.ID, "a session must not see another session's jobs")
	require.NotContains(t, resp.Content, "their job")
}

func TestJobListRunningOnly(t *testing.T) {
	t.Parallel()

	bgManager := shell.GetBackgroundShellManager()

	done, err := bgManager.Start(t.Context(), shell.StartOptions{
		WorkingDir:  t.TempDir(),
		Command:     "true",
		Description: "finished job",
		SessionID:   "running-only-session",
	})
	require.NoError(t, err)
	t.Cleanup(func() { bgManager.Remove(done.ID) })
	done.Wait()

	live, err := bgManager.Start(t.Context(), shell.StartOptions{
		WorkingDir:  t.TempDir(),
		Command:     "sleep 30",
		Description: "live job",
		SessionID:   "running-only-session",
	})
	require.NoError(t, err)
	t.Cleanup(func() { bgManager.Kill(live.ID) })

	tool := NewJobListTool()
	ctx := context.WithValue(t.Context(), SessionIDContextKey, "running-only-session")

	all, err := tool.Run(ctx, fantasy.ToolCall{Input: `{}`})
	require.NoError(t, err)
	require.Contains(t, all.Content, "finished job")
	require.Contains(t, all.Content, "live job")

	onlyRunning, err := tool.Run(ctx, fantasy.ToolCall{Input: `{"running_only":true}`})
	require.NoError(t, err)
	require.NotContains(t, onlyRunning.Content, "finished job")
	require.Contains(t, onlyRunning.Content, "live job")
}

func TestJobListEmpty(t *testing.T) {
	t.Parallel()

	tool := NewJobListTool()
	ctx := context.WithValue(t.Context(), SessionIDContextKey, "session-with-no-jobs")
	resp, err := tool.Run(ctx, fantasy.ToolCall{Input: `{}`})
	require.NoError(t, err)
	require.Contains(t, resp.Content, "No background jobs.")
}
