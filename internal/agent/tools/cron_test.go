package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/permission"
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

// stubCronPermissions is a permission.Service whose Request always
// answers allowed. Embedding the interface keeps the stub to the one
// method the cron tools call.
type stubCronPermissions struct {
	permission.Service
	allowed bool
}

func (s *stubCronPermissions) Request(ctx context.Context, opts permission.CreatePermissionRequest) (bool, error) {
	return s.allowed, nil
}

func allowPermissions() *stubCronPermissions { return &stubCronPermissions{allowed: true} }
func denyPermissions() *stubCronPermissions  { return &stubCronPermissions{allowed: false} }

func TestCronToolNames(t *testing.T) {
	t.Parallel()

	store := scheduler.NewStore("")
	require.Equal(t, CronCreateToolName, NewCronCreateTool(store, allowPermissions()).Info().Name)
	require.Equal(t, CronListToolName, NewCronListTool(store).Info().Name)
	require.Equal(t, CronDeleteToolName, NewCronDeleteTool(store).Info().Name)
}

// TestCronToolsUnavailableWithoutStore covers the headless coordinator,
// which wires no cron store (EnableScheduler=false): the tools stay
// registered but must answer with a clear, model-readable error instead
// of failing obscurely or silently succeeding.
func TestCronToolsUnavailableWithoutStore(t *testing.T) {
	t.Parallel()

	ctx := cronTestContext("test-session")

	createResp, err := runCronTool(t, NewCronCreateTool(nil, allowPermissions()), CronCreateToolName, ctx, CronCreateParams{
		Cron: "* * * * *", Prompt: "ping",
	})
	require.NoError(t, err)
	require.Contains(t, createResp.Content, "not available in non-interactive mode")

	listResp, err := runCronTool(t, NewCronListTool(nil), CronListToolName, ctx, struct{}{})
	require.NoError(t, err)
	require.Contains(t, listResp.Content, "not available in non-interactive mode")

	deleteResp, err := runCronTool(t, NewCronDeleteTool(nil), CronDeleteToolName, ctx, CronDeleteParams{ID: "abc"})
	require.NoError(t, err)
	require.Contains(t, deleteResp.Content, "not available in non-interactive mode")
}

func TestCronCreateRequiresSession(t *testing.T) {
	t.Parallel()

	store := scheduler.NewStore("")
	tool := NewCronCreateTool(store, allowPermissions())

	resp, err := runCronTool(t, tool, CronCreateToolName, context.Background(), CronCreateParams{
		Cron:   "* * * * *",
		Prompt: "hello",
	})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "session ID")
}

func TestCronToolsRejectSubAgentSessions(t *testing.T) {
	t.Parallel()

	store := scheduler.NewStore("")
	ctx := cronTestContext("message-id$$tool-call-id")

	resp, err := runCronTool(t, NewCronCreateTool(store, allowPermissions()), CronCreateToolName, ctx, CronCreateParams{
		Cron:   "* * * * *",
		Prompt: "hello",
	})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "root session")

	resp, err = runCronTool(t, NewCronListTool(store), CronListToolName, ctx, struct{}{})
	require.NoError(t, err)
	require.True(t, resp.IsError)

	resp, err = runCronTool(t, NewCronDeleteTool(store), CronDeleteToolName, ctx, CronDeleteParams{ID: "abc"})
	require.NoError(t, err)
	require.True(t, resp.IsError)
}

func TestCronCreateValidatesExpression(t *testing.T) {
	t.Parallel()

	store := scheduler.NewStore("")
	ctx := cronTestContext("test-session")

	resp, err := runCronTool(t, NewCronCreateTool(store, allowPermissions()), CronCreateToolName, ctx, CronCreateParams{
		Cron:   "not cron",
		Prompt: "hello",
	})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "invalid cron expression")
	require.Empty(t, store.List("test-session"))
}

// Validation failures must reach the model as error responses it can
// read and retry, not as Go errors that end the tool call opaquely.
func TestCronCreateValidationErrorsAreModelReadable(t *testing.T) {
	t.Parallel()

	store := scheduler.NewStore(t.TempDir() + "/scheduled_tasks.json")
	ctx := cronTestContext("test-session")
	tool := NewCronCreateTool(store, allowPermissions())

	recurring := false
	resp, err := runCronTool(t, tool, CronCreateToolName, ctx, CronCreateParams{
		// February 30th parses but can never fire.
		Cron:      "0 0 30 2 *",
		Prompt:    "reminder",
		Recurring: &recurring,
	})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "never fire")

	// A one-shot whose fire time already passed today reports the next
	// match so the model can correct the fields.
	resp, err = runCronTool(t, tool, CronCreateToolName, ctx, CronCreateParams{
		Cron:      "* * * * *",
		Prompt:    "too late",
		Recurring: &recurring,
	})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "already passed")
}

// CronCreate goes through the permission prompt like other tools that
// have side effects; a denial must leave the store untouched.
func TestCronCreateRequiresPermission(t *testing.T) {
	t.Parallel()

	store := scheduler.NewStore("")
	ctx := cronTestContext("test-session")

	resp, err := runCronTool(t, NewCronCreateTool(store, denyPermissions()), CronCreateToolName, ctx, CronCreateParams{
		Cron:   "* * * * *",
		Prompt: "check the deploy",
	})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "denied permission")
	require.Empty(t, store.List("test-session"))

	resp, err = runCronTool(t, NewCronCreateTool(store, allowPermissions()), CronCreateToolName, ctx, CronCreateParams{
		Cron:   "* * * * *",
		Prompt: "check the deploy",
	})
	require.NoError(t, err)
	require.False(t, resp.IsError)
	require.Contains(t, resp.Content, "Scheduled task created")
	require.Len(t, store.List("test-session"), 1)
}

func TestCronCreateDefaultsRecurring(t *testing.T) {
	t.Parallel()

	store := scheduler.NewStore("")
	ctx := cronTestContext("test-session")

	resp, err := runCronTool(t, NewCronCreateTool(store, allowPermissions()), CronCreateToolName, ctx, CronCreateParams{
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
	resp, err := runCronTool(t, NewCronCreateTool(store, allowPermissions()), CronCreateToolName, ctx, CronCreateParams{
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

	resp, err := runCronTool(t, NewCronDeleteTool(store), CronDeleteToolName, ctx, CronDeleteParams{ID: "deadbeef"})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "deadbeef")
}

func TestCronDeleteOtherSessionsTask(t *testing.T) {
	t.Parallel()

	store := scheduler.NewStore("")
	task, err := store.Create("other-session", "* * * * *", "not yours", true, false)
	require.NoError(t, err)

	resp, err := runCronTool(t, NewCronDeleteTool(store), CronDeleteToolName, cronTestContext("test-session"), CronDeleteParams{ID: task.ID})
	require.NoError(t, err)
	require.True(t, resp.IsError)
}

func TestCronDeleteRequiresID(t *testing.T) {
	t.Parallel()

	store := scheduler.NewStore("")
	resp, err := runCronTool(t, NewCronDeleteTool(store), CronDeleteToolName, cronTestContext("test-session"), CronDeleteParams{})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "id is required")
}

func TestCronCreateEnforcesSessionLimit(t *testing.T) {
	t.Parallel()

	store := scheduler.NewStore("")
	ctx := cronTestContext("test-session")
	tool := NewCronCreateTool(store, allowPermissions())

	for range scheduler.MaxTasksPerSession {
		_, err := runCronTool(t, tool, CronCreateToolName, ctx, CronCreateParams{
			Cron:   "* * * * *",
			Prompt: strings.Repeat("x", 3),
		})
		require.NoError(t, err)
	}

	resp, err := runCronTool(t, tool, CronCreateToolName, ctx, CronCreateParams{
		Cron:   "* * * * *",
		Prompt: "one too many",
	})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "50")
}
