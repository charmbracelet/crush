package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/scheduler"
	"github.com/stretchr/testify/require"
)

func runCronTool(t *testing.T, tool fantasy.AgentTool, name string, ctx context.Context, params any) (fantasy.ToolResponse, error) {
	t.Helper()

	input, err := json.Marshal(params)
	require.NoError(t, err)

	call := fantasy.ToolCall{
		ID:    "test-call",
		Name:  name,
		Input: string(input),
	}
	return tool.Run(ctx, call)
}

func cronTestContext(sessionID string) context.Context {
	return context.WithValue(context.Background(), SessionIDContextKey, sessionID)
}

func TestCronToolNames(t *testing.T) {
	t.Parallel()

	store := scheduler.NewStore("")
	require.Equal(t, CronCreateToolName, NewCronCreateTool(store).Info().Name)
	require.Equal(t, CronListToolName, NewCronListTool(store).Info().Name)
	require.Equal(t, CronDeleteToolName, NewCronDeleteTool(store).Info().Name)
}

func TestCronCreateRequiresSession(t *testing.T) {
	t.Parallel()

	store := scheduler.NewStore("")
	tool := NewCronCreateTool(store)

	_, err := runCronTool(t, tool, CronCreateToolName, context.Background(), CronCreateParams{
		Cron:   "* * * * *",
		Prompt: "hello",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "session ID")
}

func TestCronToolsRejectSubAgentSessions(t *testing.T) {
	t.Parallel()

	store := scheduler.NewStore("")
	ctx := cronTestContext("message-id$$tool-call-id")

	_, err := runCronTool(t, NewCronCreateTool(store), CronCreateToolName, ctx, CronCreateParams{
		Cron:   "* * * * *",
		Prompt: "hello",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "root session")

	_, err = runCronTool(t, NewCronListTool(store), CronListToolName, ctx, struct{}{})
	require.Error(t, err)

	_, err = runCronTool(t, NewCronDeleteTool(store), CronDeleteToolName, ctx, CronDeleteParams{ID: "abc"})
	require.Error(t, err)
}

func TestCronCreateValidatesExpression(t *testing.T) {
	t.Parallel()

	store := scheduler.NewStore("")
	ctx := cronTestContext("test-session")

	_, err := runCronTool(t, NewCronCreateTool(store), CronCreateToolName, ctx, CronCreateParams{
		Cron:   "not cron",
		Prompt: "hello",
	})
	require.Error(t, err)
}

func TestCronCreateDefaultsRecurring(t *testing.T) {
	t.Parallel()

	store := scheduler.NewStore("")
	ctx := cronTestContext("test-session")

	resp, err := runCronTool(t, NewCronCreateTool(store), CronCreateToolName, ctx, CronCreateParams{
		Cron:   "* * * * *",
		Prompt: "check the deploy",
	})
	require.NoError(t, err)
	require.False(t, resp.IsError)
	require.Contains(t, resp.Content, "recurring")
	require.Contains(t, resp.Content, "session-only")
	require.Contains(t, resp.Content, "check the deploy")

	tasks := store.List("test-session")
	require.Len(t, tasks, 1)
	require.True(t, tasks[0].Recurring)
	require.False(t, tasks[0].Durable)
	require.Len(t, tasks[0].ID, 8)
}

func TestCronCreateOneShotAndDurable(t *testing.T) {
	t.Parallel()

	store := scheduler.NewStore(t.TempDir() + "/scheduled_tasks.json")
	ctx := cronTestContext("test-session")

	recurring := false
	resp, err := runCronTool(t, NewCronCreateTool(store), CronCreateToolName, ctx, CronCreateParams{
		Cron:      "30 14 27 7 *",
		Prompt:    "reminder",
		Recurring: &recurring,
		Durable:   true,
	})
	require.NoError(t, err)
	require.Contains(t, resp.Content, "one-shot")
	require.Contains(t, resp.Content, "durable")

	tasks := store.List("test-session")
	require.Len(t, tasks, 1)
	require.False(t, tasks[0].Recurring)
	require.True(t, tasks[0].Durable)
}

func TestCronListEmpty(t *testing.T) {
	t.Parallel()

	store := scheduler.NewStore("")
	resp, err := runCronTool(t, NewCronListTool(store), CronListToolName, cronTestContext("test-session"), struct{}{})
	require.NoError(t, err)
	require.Contains(t, resp.Content, "No scheduled tasks")
}

func TestCronListShowsSessionTasks(t *testing.T) {
	t.Parallel()

	store := scheduler.NewStore("")
	_, err := store.Create("test-session", "* * * * *", "first task", true, false)
	require.NoError(t, err)
	_, err = store.Create("other-session", "0 9 * * *", "hidden task", true, false)
	require.NoError(t, err)

	resp, err := runCronTool(t, NewCronListTool(store), CronListToolName, cronTestContext("test-session"), struct{}{})
	require.NoError(t, err)
	require.Contains(t, resp.Content, "first task")
	require.Contains(t, resp.Content, "* * * * *")
	require.NotContains(t, resp.Content, "hidden task")

	var meta CronTasksResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
	require.Len(t, meta.Tasks, 1)
}

func TestCronDelete(t *testing.T) {
	t.Parallel()

	store := scheduler.NewStore("")
	ctx := cronTestContext("test-session")
	tool := NewCronDeleteTool(store)

	task, err := store.Create("test-session", "* * * * *", "delete me", true, false)
	require.NoError(t, err)

	resp, err := runCronTool(t, tool, CronDeleteToolName, ctx, CronDeleteParams{ID: task.ID})
	require.NoError(t, err)
	require.Contains(t, resp.Content, "deleted")
	require.Len(t, store.List("test-session"), 0)
}

func TestCronDeleteUnknownID(t *testing.T) {
	t.Parallel()

	store := scheduler.NewStore("")
	ctx := cronTestContext("test-session")

	_, err := runCronTool(t, NewCronDeleteTool(store), CronDeleteToolName, ctx, CronDeleteParams{ID: "deadbeef"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "deadbeef")
}

func TestCronDeleteOtherSessionsTask(t *testing.T) {
	t.Parallel()

	store := scheduler.NewStore("")
	task, err := store.Create("other-session", "* * * * *", "not yours", true, false)
	require.NoError(t, err)

	_, err = runCronTool(t, NewCronDeleteTool(store), CronDeleteToolName, cronTestContext("test-session"), CronDeleteParams{ID: task.ID})
	require.Error(t, err)
}

func TestCronCreateEnforcesSessionLimit(t *testing.T) {
	t.Parallel()

	store := scheduler.NewStore("")
	ctx := cronTestContext("test-session")
	tool := NewCronCreateTool(store)

	for range scheduler.MaxTasksPerSession {
		_, err := runCronTool(t, tool, CronCreateToolName, ctx, CronCreateParams{
			Cron:   "* * * * *",
			Prompt: strings.Repeat("x", 3),
		})
		require.NoError(t, err)
	}

	_, err := runCronTool(t, tool, CronCreateToolName, ctx, CronCreateParams{
		Cron:   "* * * * *",
		Prompt: "one too many",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "50")
}
