package dispatch

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/stretchr/testify/require"
)

type testEnv struct {
	ctx context.Context
	q   *db.Queries
	svc Service
}

func setupTest(t *testing.T) *testEnv {
	t.Helper()

	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	q := db.New(conn)
	return &testEnv{
		ctx: t.Context(),
		q:   q,
		svc: NewService(q, conn, Config{APIEndpoint: "http://localhost:8080"}),
	}
}

func TestService_Send(t *testing.T) {
	env := setupTest(t)

	msg, err := env.svc.Send(env.ctx, SendMessageParams{
		FromAgent: "agent-a",
		ToAgent:   "agent-b",
		Task:      "Test task",
		Priority:  5,
	})
	require.NoError(t, err)
	require.NotEmpty(t, msg.ID)
	require.Equal(t, "agent-a", msg.FromAgent)
	require.Equal(t, "agent-b", msg.ToAgent)
	require.Equal(t, "Test task", msg.Task)
	require.Equal(t, StatusPending, msg.Status)
	require.Equal(t, 5, msg.Priority)
}

func TestService_SendWithContext(t *testing.T) {
	env := setupTest(t)

	ctx := map[string]any{"key": "value", "count": 42}
	msg, err := env.svc.Send(env.ctx, SendMessageParams{
		FromAgent: "agent-a",
		ToAgent:   "agent-b",
		Task:      "Task with context",
		Context:   ctx,
	})
	require.NoError(t, err)
	require.NotNil(t, msg.Context)
	require.Equal(t, "value", msg.Context["key"])
	require.Equal(t, 42.0, msg.Context["count"]) // JSON unmarshals numbers as float64
}

func TestService_Get(t *testing.T) {
	env := setupTest(t)

	created, err := env.svc.Send(env.ctx, SendMessageParams{
		FromAgent: "agent-a",
		ToAgent:   "agent-b",
		Task:      "Test task",
	})
	require.NoError(t, err)

	retrieved, err := env.svc.Get(env.ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, retrieved.ID)
	require.Equal(t, created.Task, retrieved.Task)
}

func TestService_Get_NotFound(t *testing.T) {
	env := setupTest(t)

	_, err := env.svc.Get(env.ctx, "nonexistent-id")
	require.Error(t, err)
}

func TestService_List(t *testing.T) {
	env := setupTest(t)

	// Create multiple messages
	_, err := env.svc.Send(env.ctx, SendMessageParams{
		FromAgent: "agent-a",
		ToAgent:   "agent-b",
		Task:      "Task 1",
	})
	require.NoError(t, err)

	_, err = env.svc.Send(env.ctx, SendMessageParams{
		FromAgent: "agent-a",
		ToAgent:   "agent-c",
		Task:      "Task 2",
	})
	require.NoError(t, err)

	_, err = env.svc.Send(env.ctx, SendMessageParams{
		FromAgent: "agent-b",
		ToAgent:   "agent-c",
		Task:      "Task 3",
	})
	require.NoError(t, err)

	// List all
	all, err := env.svc.List(env.ctx, ListMessagesParams{})
	require.NoError(t, err)
	require.Len(t, all, 3)

	// Filter by from agent
	fromA, err := env.svc.List(env.ctx, ListMessagesParams{FromAgent: "agent-a"})
	require.NoError(t, err)
	require.Len(t, fromA, 2)

	// Filter by to agent
	toC, err := env.svc.List(env.ctx, ListMessagesParams{ToAgent: "agent-c"})
	require.NoError(t, err)
	require.Len(t, toC, 2)
}

func TestService_Poll(t *testing.T) {
	env := setupTest(t)

	// Create messages with different priorities
	_, err := env.svc.Send(env.ctx, SendMessageParams{
		FromAgent: "agent-a",
		ToAgent:   "agent-b",
		Task:      "Low priority task",
		Priority:  1,
	})
	require.NoError(t, err)

	_, err = env.svc.Send(env.ctx, SendMessageParams{
		FromAgent: "agent-a",
		ToAgent:   "agent-b",
		Task:      "High priority task",
		Priority:  10,
	})
	require.NoError(t, err)

	// Poll for agent-b
	messages, err := env.svc.Poll(env.ctx, "agent-b", 10)
	require.NoError(t, err)
	require.Len(t, messages, 2)

	// Higher priority should come first
	require.Equal(t, "High priority task", messages[0].Task)
	require.Equal(t, "Low priority task", messages[1].Task)

	// Poll for agent with no messages
	none, err := env.svc.Poll(env.ctx, "agent-c", 10)
	require.NoError(t, err)
	require.Empty(t, none)
}

func TestService_Claim(t *testing.T) {
	env := setupTest(t)

	created, err := env.svc.Send(env.ctx, SendMessageParams{
		FromAgent: "agent-a",
		ToAgent:   "agent-b",
		Task:      "Task to claim",
	})
	require.NoError(t, err)
	require.Equal(t, StatusPending, created.Status)

	claimed, err := env.svc.Claim(env.ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, StatusInProgress, claimed.Status)

	// Verify the status is persisted
	retrieved, err := env.svc.Get(env.ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, StatusInProgress, retrieved.Status)
}

func TestService_Claim_AlreadyClaimed(t *testing.T) {
	env := setupTest(t)

	created, err := env.svc.Send(env.ctx, SendMessageParams{
		FromAgent: "agent-a",
		ToAgent:   "agent-b",
		Task:      "Task to claim",
	})
	require.NoError(t, err)

	// First claim succeeds
	_, err = env.svc.Claim(env.ctx, created.ID)
	require.NoError(t, err)

	// Second claim fails (no rows affected)
	_, err = env.svc.Claim(env.ctx, created.ID)
	require.Error(t, err)
}

func TestService_Complete(t *testing.T) {
	env := setupTest(t)

	created, err := env.svc.Send(env.ctx, SendMessageParams{
		FromAgent: "agent-a",
		ToAgent:   "agent-b",
		Task:      "Task to complete",
	})
	require.NoError(t, err)

	_, err = env.svc.Claim(env.ctx, created.ID)
	require.NoError(t, err)

	completed, err := env.svc.Complete(env.ctx, created.ID, "Task completed successfully")
	require.NoError(t, err)
	require.Equal(t, StatusCompleted, completed.Status)
	require.Equal(t, "Task completed successfully", completed.Result)
	require.NotNil(t, completed.CompletedAt)
}

func TestService_Fail(t *testing.T) {
	env := setupTest(t)

	created, err := env.svc.Send(env.ctx, SendMessageParams{
		FromAgent: "agent-a",
		ToAgent:   "agent-b",
		Task:      "Task to fail",
	})
	require.NoError(t, err)

	_, err = env.svc.Claim(env.ctx, created.ID)
	require.NoError(t, err)

	failed, err := env.svc.Fail(env.ctx, created.ID, "Something went wrong")
	require.NoError(t, err)
	require.Equal(t, StatusFailed, failed.Status)
	require.Equal(t, "Something went wrong", failed.Error)
	require.NotNil(t, failed.CompletedAt)
}

func TestService_Delete(t *testing.T) {
	env := setupTest(t)

	created, err := env.svc.Send(env.ctx, SendMessageParams{
		FromAgent: "agent-a",
		ToAgent:   "agent-b",
		Task:      "Task to delete",
	})
	require.NoError(t, err)

	err = env.svc.Delete(env.ctx, created.ID)
	require.NoError(t, err)

	_, err = env.svc.Get(env.ctx, created.ID)
	require.Error(t, err)
}

func TestService_Reset(t *testing.T) {
	env := setupTest(t)

	created, err := env.svc.Send(env.ctx, SendMessageParams{
		FromAgent: "agent-a",
		ToAgent:   "agent-b",
		Task:      "Task to reset",
	})
	require.NoError(t, err)

	_, err = env.svc.Claim(env.ctx, created.ID)
	require.NoError(t, err)

	retrieved, err := env.svc.Get(env.ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, StatusInProgress, retrieved.Status)

	reset, err := env.svc.Reset(env.ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, StatusPending, reset.Status)
}

func TestService_CreateAgent(t *testing.T) {
	env := setupTest(t)

	agent, err := env.svc.CreateAgent(env.ctx, CreateAgentParams{
		Name:         "test-agent",
		Description:  "A test agent",
		Capabilities: []string{"code", "test"},
		CLICommand:   "test-agent --run",
	})
	require.NoError(t, err)
	require.Equal(t, "test-agent", agent.Name)
	require.Equal(t, "A test agent", agent.Description)
	require.Equal(t, []string{"code", "test"}, agent.Capabilities)
	require.Equal(t, "test-agent --run", agent.CLICommand)
	require.True(t, agent.Enabled)
}

func TestService_GetAgent(t *testing.T) {
	env := setupTest(t)

	created, err := env.svc.CreateAgent(env.ctx, CreateAgentParams{
		Name:        "test-agent",
		Description: "A test agent",
	})
	require.NoError(t, err)

	retrieved, err := env.svc.GetAgent(env.ctx, "test-agent")
	require.NoError(t, err)
	require.Equal(t, created.Name, retrieved.Name)
	require.Equal(t, created.Description, retrieved.Description)
}

func TestService_GetAgent_NotFound(t *testing.T) {
	env := setupTest(t)

	_, err := env.svc.GetAgent(env.ctx, "nonexistent")
	require.Error(t, err)
}

func TestService_ListAgents(t *testing.T) {
	env := setupTest(t)

	_, err := env.svc.CreateAgent(env.ctx, CreateAgentParams{Name: "agent-a"})
	require.NoError(t, err)

	_, err = env.svc.CreateAgent(env.ctx, CreateAgentParams{Name: "agent-b"})
	require.NoError(t, err)

	// Create and disable an agent
	_, err = env.svc.CreateAgent(env.ctx, CreateAgentParams{Name: "agent-c"})
	require.NoError(t, err)
	err = env.svc.SetAgentEnabled(env.ctx, "agent-c", false)
	require.NoError(t, err)

	// List only enabled
	enabled, err := env.svc.ListAgents(env.ctx, false)
	require.NoError(t, err)
	require.Len(t, enabled, 2)

	// List all
	all, err := env.svc.ListAgents(env.ctx, true)
	require.NoError(t, err)
	require.Len(t, all, 3)
}

func TestService_UpdateAgent(t *testing.T) {
	env := setupTest(t)

	_, err := env.svc.CreateAgent(env.ctx, CreateAgentParams{
		Name:        "test-agent",
		Description: "Original description",
	})
	require.NoError(t, err)

	updated, err := env.svc.UpdateAgent(env.ctx, "test-agent", CreateAgentParams{
		Description:  "Updated description",
		Capabilities: []string{"new-capability"},
	})
	require.NoError(t, err)
	require.Equal(t, "Updated description", updated.Description)
	require.Equal(t, []string{"new-capability"}, updated.Capabilities)
}

func TestService_SetAgentEnabled(t *testing.T) {
	env := setupTest(t)

	_, err := env.svc.CreateAgent(env.ctx, CreateAgentParams{Name: "test-agent"})
	require.NoError(t, err)

	err = env.svc.SetAgentEnabled(env.ctx, "test-agent", false)
	require.NoError(t, err)

	agent, err := env.svc.GetAgent(env.ctx, "test-agent")
	require.NoError(t, err)
	require.False(t, agent.Enabled)

	err = env.svc.SetAgentEnabled(env.ctx, "test-agent", true)
	require.NoError(t, err)

	agent, err = env.svc.GetAgent(env.ctx, "test-agent")
	require.NoError(t, err)
	require.True(t, agent.Enabled)
}

func TestService_DeleteAgent(t *testing.T) {
	env := setupTest(t)

	_, err := env.svc.CreateAgent(env.ctx, CreateAgentParams{Name: "test-agent"})
	require.NoError(t, err)

	err = env.svc.DeleteAgent(env.ctx, "test-agent")
	require.NoError(t, err)

	_, err = env.svc.GetAgent(env.ctx, "test-agent")
	require.Error(t, err)
}

func TestService_PubSub(t *testing.T) {
	env := setupTest(t)

	// Subscribe to events
	sub := env.svc.Subscribe(env.ctx)

	// Send a message - should trigger CreatedEvent
	_, err := env.svc.Send(env.ctx, SendMessageParams{
		FromAgent: "agent-a",
		ToAgent:   "agent-b",
		Task:      "Test task",
	})
	require.NoError(t, err)

	// Should receive the event
	select {
	case event := <-sub:
		require.Equal(t, "created", string(event.Type))
		require.Equal(t, "Test task", event.Payload.Task)
	default:
		t.Fatal("expected to receive created event")
	}
}

func TestService_Dispatch(t *testing.T) {
	env := setupTest(t)

	// Create a worker with a no-op command (echo)
	_, err := env.svc.CreateAgent(env.ctx, CreateAgentParams{
		Name:       "test-worker",
		CLICommand: "echo", // Simple command that will succeed
	})
	require.NoError(t, err)

	// Dispatch a task
	msg, err := env.svc.Dispatch(env.ctx, DispatchParams{
		Worker: "test-worker",
		Task:   "Test task for dispatch",
		Variables: map[string]any{
			"file": "test.go",
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, msg.ID)
	require.Equal(t, "test-worker", msg.ToAgent)
	require.Equal(t, "Test task for dispatch", msg.Task)
	require.Equal(t, StatusPending, msg.Status)
}

func TestService_Dispatch_WorkerNotFound(t *testing.T) {
	env := setupTest(t)

	_, err := env.svc.Dispatch(env.ctx, DispatchParams{
		Worker: "nonexistent-worker",
		Task:   "Test task",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestService_Dispatch_WorkerDisabled(t *testing.T) {
	env := setupTest(t)

	// Create and disable a worker
	_, err := env.svc.CreateAgent(env.ctx, CreateAgentParams{
		Name:       "disabled-worker",
		CLICommand: "echo",
	})
	require.NoError(t, err)
	err = env.svc.SetAgentEnabled(env.ctx, "disabled-worker", false)
	require.NoError(t, err)

	_, err = env.svc.Dispatch(env.ctx, DispatchParams{
		Worker: "disabled-worker",
		Task:   "Test task",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "disabled")
}

func TestService_Dispatch_NoCLICommand(t *testing.T) {
	env := setupTest(t)

	// Create a worker without a CLI command
	_, err := env.svc.CreateAgent(env.ctx, CreateAgentParams{
		Name: "no-command-worker",
	})
	require.NoError(t, err)

	_, err = env.svc.Dispatch(env.ctx, DispatchParams{
		Worker: "no-command-worker",
		Task:   "Test task",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no CLI command")
}
