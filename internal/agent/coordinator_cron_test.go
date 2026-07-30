package agent

import (
	"context"
	"path/filepath"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/scheduler"
	"github.com/stretchr/testify/require"
)

// newCronTestCoordinator builds a coordinator whose coder agent is
// restricted to the cron tools, keeping buildTools free of sub-agent,
// MCP, and LSP wiring.
func newCronTestCoordinator(t *testing.T, env fakeEnv) (*coordinator, config.Agent) {
	t.Helper()

	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	cfg.SetupAgents()

	c := &coordinator{
		cfg:       cfg,
		sessions:  env.sessions,
		messages:  env.messages,
		cronStore: scheduler.NewStore(""),
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
// the task and reports an error instead of retrying forever, and that
// fired prompts carry the scheduled-task marker.
func TestFireScheduledTask(t *testing.T) {
	env := testEnv(t)

	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)

	c := &coordinator{
		cfg:       cfg,
		sessions:  env.sessions,
		messages:  env.messages,
		cronStore: scheduler.NewStore(""),
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
		cfg:       cfg,
		sessions:  env.sessions,
		messages:  env.messages,
		cronStore: scheduler.NewStore(path),
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

// TestScheduledTaskPrompt verifies fired prompts are wrapped with the
// marker that tells the agent the turn is an automated firing, matching
// Claude Code's scheduled-task marker.
func TestScheduledTaskPrompt(t *testing.T) {
	task := scheduler.Task{Prompt: "check the deploy"}
	prompt := scheduledTaskPrompt(task)
	require.Contains(t, prompt, scheduledTaskPromptMarker)
	require.Contains(t, prompt, "check the deploy")
}
