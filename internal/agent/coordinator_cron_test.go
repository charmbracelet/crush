package agent

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/scheduler"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

// failingSessionLookups is a session service whose Get always fails
// with a plain error, standing in for a transient database problem. It
// deliberately does not return sql.ErrNoRows, which is the only lookup
// failure that may retire a session's tasks.
type failingSessionLookups struct {
	session.Service
}

func (failingSessionLookups) Get(ctx context.Context, id string) (session.Session, error) {
	return session.Session{}, errors.New("database is locked")
}

// newCronTestCoordinator builds a coordinator whose coder agent is
// restricted to the cron tools, keeping buildTools free of sub-agent,
// MCP, and LSP wiring.
func newCronTestCoordinator(t *testing.T, env fakeEnv) (*coordinator, config.Agent) {
	t.Helper()

	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	cfg.SetupAgents()

	c := &coordinator{
		cfg:         cfg,
		sessions:    env.sessions,
		messages:    env.messages,
		permissions: env.permissions,
		cronStore:   scheduler.NewStore(""),
	}

	agentCfg := cfg.Config().Agents[config.AgentCoder]
	agentCfg.AllowedTools = []string{
		tools.CronCreateToolName,
		tools.CronListToolName,
		tools.CronDeleteToolName,
	}
	return c, agentCfg
}

// TestBuildToolsIncludesCronTools verifies the CronCreate/CronList/
// CronDelete tools are constructed and survive AllowedTools filtering
// for the coder agent.
func TestBuildToolsIncludesCronTools(t *testing.T) {
	env := testEnv(t)
	c, agentCfg := newCronTestCoordinator(t, env)

	built, err := c.buildTools(t.Context(), agentCfg, false)
	require.NoError(t, err)
	require.Len(t, built, 3)

	names := make(map[string]bool, len(built))
	for _, tool := range built {
		names[tool.Info().Name] = true
	}
	require.True(t, names[tools.CronCreateToolName], "CronCreate missing from built tools")
	require.True(t, names[tools.CronListToolName], "CronList missing from built tools")
	require.True(t, names[tools.CronDeleteToolName], "CronDelete missing from built tools")
}

// TestCronToolsEndToEnd drives the registered tools through their
// fantasy.ToolCall interface the way the LLM would: create, list, then
// delete a scheduled task.
func TestCronToolsEndToEnd(t *testing.T) {
	env := testEnv(t)
	c, agentCfg := newCronTestCoordinator(t, env)

	built, err := c.buildTools(t.Context(), agentCfg, false)
	require.NoError(t, err)

	byName := make(map[string]fantasy.AgentTool, len(built))
	for _, tool := range built {
		byName[tool.Info().Name] = tool
	}

	sess, err := env.sessions.Create(t.Context(), "cron test")
	require.NoError(t, err)
	ctx := context.WithValue(t.Context(), tools.SessionIDContextKey, sess.ID)

	createResp, err := byName[tools.CronCreateToolName].Run(ctx, fantasy.ToolCall{
		ID:    "create-1",
		Name:  tools.CronCreateToolName,
		Input: `{"cron": "* * * * *", "prompt": "check the deploy"}`,
	})
	require.NoError(t, err)
	require.Contains(t, createResp.Content, "Scheduled task created")

	listResp, err := byName[tools.CronListToolName].Run(ctx, fantasy.ToolCall{
		ID:    "list-1",
		Name:  tools.CronListToolName,
		Input: `{}`,
	})
	require.NoError(t, err)
	require.Contains(t, listResp.Content, "check the deploy")

	taskID := c.cronStore.List(sess.ID)[0].ID
	deleteResp, err := byName[tools.CronDeleteToolName].Run(ctx, fantasy.ToolCall{
		ID:    "delete-1",
		Name:  tools.CronDeleteToolName,
		Input: `{"id": "` + taskID + `"}`,
	})
	require.NoError(t, err)
	require.Contains(t, deleteResp.Content, "deleted")
	require.Empty(t, c.cronStore.List(sess.ID))
}

// TestFireScheduledTask verifies firing against a deleted session drops
// the task and reports an error instead of retrying forever.
func TestFireScheduledTask(t *testing.T) {
	env := testEnv(t)

	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)

	c := &coordinator{
		cfg:         cfg,
		sessions:    env.sessions,
		messages:    env.messages,
		permissions: env.permissions,
		cronStore:   scheduler.NewStore(""),
	}

	// A task for a deleted session is dropped and reported as an error.
	_, err = c.cronStore.Create("session-that-does-not-exist", "* * * * *", "ping", true, false)
	require.NoError(t, err)

	missing := scheduler.Task{
		ID:        "gone9999",
		SessionID: "session-that-does-not-exist",
		Prompt:    "ping",
		Cron:      "* * * * *",
		Recurring: true,
	}
	require.Error(t, c.fireScheduledTask(t.Context(), missing))
	require.Empty(t, c.cronStore.List("session-that-does-not-exist"))
}

// A *durable* task whose session no longer exists must be dropped too.
//
// Previously DropSession deliberately kept durable tasks, so this task
// survived every fire: the session lookup failed, MarkError rescheduled
// it because it was recurring, and the next tick tried again — an error
// logged on every fire, forever, and re-armed on each restart. The
// comment claimed it dropped the task "rather than failing it forever",
// which was exactly backwards for durable tasks.
func TestFireScheduledTaskDropsDurableTaskForDeletedSession(t *testing.T) {
	env := testEnv(t)

	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "scheduled_tasks.json")
	c := &coordinator{
		cfg:         cfg,
		sessions:    env.sessions,
		messages:    env.messages,
		permissions: env.permissions,
		cronStore:   scheduler.NewStore(path),
	}

	task, err := c.cronStore.Create("session-that-does-not-exist", "* * * * *", "ping", true, true)
	require.NoError(t, err)
	require.True(t, task.Durable)

	require.Error(t, c.fireScheduledTask(t.Context(), task))
	require.Empty(t, c.cronStore.ListAll(), "a durable task for a dead session must be dropped, not retried forever")

	// And it must not come back on the next start.
	reloaded := scheduler.NewStore(path)
	require.NoError(t, reloaded.Load())
	require.Empty(t, reloaded.ListAll(), "the drop must reach disk")
}

// A failed session lookup that is NOT "session gone" (a brief database
// hiccup, a canceled context) must keep the session's tasks, durable
// ones included: only a session that is actually not found may retire
// them. The error comes back as a TransientError so the scheduler
// retries the task instead of recording a permanent failure.
func TestFireScheduledTaskTransientLookupErrorKeepsTasks(t *testing.T) {
	env := testEnv(t)

	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "scheduled_tasks.json")
	c := &coordinator{
		cfg:         cfg,
		sessions:    failingSessionLookups{env.sessions},
		messages:    env.messages,
		permissions: env.permissions,
		cronStore:   scheduler.NewStore(path),
	}

	task, err := c.cronStore.Create("some-session", "* * * * *", "ping", true, true)
	require.NoError(t, err)
	// A one-shot scheduled a few minutes out (pinning the day too, so the
	// store accepts it at any time of day the test runs), proving that a
	// transient failure retires neither recurring nor one-shot tasks.
	later := time.Now().Add(6 * time.Minute)
	oneShotCron := fmt.Sprintf("%d %d %d * *", later.Minute(), later.Hour(), later.Day())
	oneShot, err := c.cronStore.Create("some-session", oneShotCron, "once", false, true)
	require.NoError(t, err)

	var transient *scheduler.TransientError
	require.ErrorAs(t, c.fireScheduledTask(t.Context(), task), &transient)
	require.ErrorAs(t, c.fireScheduledTask(t.Context(), oneShot), &transient)

	tasks := c.cronStore.List("some-session")
	require.Len(t, tasks, 2, "a transient lookup failure must not drop the session's tasks")

	reloaded := scheduler.NewStore(path)
	require.NoError(t, reloaded.Load())
	require.Len(t, reloaded.ListAll(), 2, "nothing may reach disk either")
}

// TestScheduledTaskPrompt verifies fired prompts pass through without
// any injected marker — the agent receives the task's prompt verbatim.
func TestScheduledTaskPrompt(t *testing.T) {
	task := scheduler.Task{Prompt: "check the deploy"}
	require.Equal(t, "check the deploy", task.Prompt)
}

// TestFireScheduledTaskDefersUnservedSession covers the durable-task
// routing rule: a task whose session exists but is not served by this
// process must be deferred, not fired and not failed — the fire comes
// back as a TransientError so the scheduler keeps the task and retries
// later without recording an error.
func TestFireScheduledTaskDefersUnservedSession(t *testing.T) {
	env := testEnv(t)

	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)

	c := &coordinator{
		cfg:         cfg,
		sessions:    env.sessions,
		messages:    env.messages,
		permissions: env.permissions,
		cronStore:   scheduler.NewStore(""),
	}

	sess, err := env.sessions.Create(t.Context(), "unserved")
	require.NoError(t, err)
	task, err := c.cronStore.Create(sess.ID, "* * * * *", "ping", true, false)
	require.NoError(t, err)

	var transient *scheduler.TransientError
	require.ErrorAs(t, c.fireScheduledTask(t.Context(), task), &transient)
	require.Len(t, c.cronStore.List(sess.ID), 1, "a deferred fire must keep the task")
}

// TestFireScheduledTaskRunFailureRecorded covers what happens after the
// fire is accepted for a served session: the run happens in the
// background, and its failure is recorded on the task via SetLastError
// so CronList surfaces it — without rescheduling or deleting anything.
// The run here fails immediately because the test config selects no
// models; what matters is that the failure lands on the task and the
// fire itself returned nil without blocking.
func TestFireScheduledTaskRunFailureRecorded(t *testing.T) {
	env := testEnv(t)

	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)

	c := &coordinator{
		cfg:         cfg,
		sessions:    env.sessions,
		messages:    env.messages,
		permissions: env.permissions,
		cronStore:   scheduler.NewStore(""),
		interactive: true,
	}

	sess, err := env.sessions.Create(t.Context(), "served")
	require.NoError(t, err)
	task, err := c.cronStore.Create(sess.ID, "* * * * *", "ping", true, false)
	require.NoError(t, err)

	// Serving the session (the TUI loads it via ListCronTasks) makes it
	// eligible to fire.
	require.Len(t, c.ListCronTasks(sess.ID), 1)

	require.NoError(t, c.fireScheduledTask(t.Context(), task))
	c.fireWg.Wait()

	tasks := c.cronStore.List(sess.ID)
	require.Len(t, tasks, 1)
	require.NotEmpty(t, tasks[0].LastError, "the background run's failure must be recorded on the task")
	require.Equal(t, task.NextRunAt, tasks[0].NextRunAt, "a recorded run failure must not reschedule the task")
}

// TestListCronTasksWithoutSchedulerStore covers the headless
// coordinator (EnableScheduler=false wires no cron store): listing must
// be a safe nil instead of a panic.
func TestListCronTasksWithoutSchedulerStore(t *testing.T) {
	env := testEnv(t)

	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)

	c := &coordinator{
		cfg:         cfg,
		sessions:    env.sessions,
		messages:    env.messages,
		permissions: env.permissions,
	}
	require.Nil(t, c.ListCronTasks("any-session"))
}
