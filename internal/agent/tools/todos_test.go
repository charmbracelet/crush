package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

func newTodosSessionService(t *testing.T) session.Service {
	t.Helper()
	conn, err := db.Connect(context.Background(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
	})
	return session.NewService(db.New(conn), conn)
}

func runTodosTool(t *testing.T, tool fantasy.AgentTool, ctx context.Context, params TodosParams) fantasy.ToolResponse {
	t.Helper()
	input, err := json.Marshal(params)
	require.NoError(t, err)
	resp, err := tool.Run(ctx, fantasy.ToolCall{ID: "call-1", Name: TodosToolName, Input: string(input)})
	require.NoError(t, err)
	return resp
}

func parseTodosMetadata(t *testing.T, resp fantasy.ToolResponse) TodosResponseMetadata {
	t.Helper()
	var meta TodosResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
	return meta
}

func TestTodosToolReplaceBackwardsCompatibleAndPersistsStructuredFields(t *testing.T) {
	t.Parallel()

	sessions := newTodosSessionService(t)
	sess, err := sessions.Create(context.Background(), "todos")
	require.NoError(t, err)

	tool := NewTodosTool(sessions)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, sess.ID)

	resp := runTodosTool(t, tool, ctx, TodosParams{Todos: []TodoItem{{
		Content:    "Implement typed memory",
		Status:     "in_progress",
		ActiveForm: "Implementing typed memory",
		Progress:   40,
	}}})
	require.False(t, resp.IsError)
	require.Contains(t, resp.Content, "Todo list updated successfully.")

	meta := parseTodosMetadata(t, resp)
	require.Equal(t, "replace", meta.Action)
	require.Len(t, meta.Todos, 1)
	todo := meta.Todos[0]
	require.NotEmpty(t, todo.ID)
	require.Equal(t, 40, todo.Progress)
	require.NotZero(t, todo.CreatedAt)
	require.NotZero(t, todo.UpdatedAt)
	require.NotZero(t, todo.StartedAt)
	require.Zero(t, todo.CompletedAt)

	loaded, err := sessions.Get(context.Background(), sess.ID)
	require.NoError(t, err)
	require.Len(t, loaded.Todos, 1)
	require.Equal(t, todo.ID, loaded.Todos[0].ID)
	require.Equal(t, 40, loaded.Todos[0].Progress)
}

func TestTodosToolCRUDAndProgressTracking(t *testing.T) {
	t.Parallel()

	sessions := newTodosSessionService(t)
	sess, err := sessions.Create(context.Background(), "todos-crud")
	require.NoError(t, err)

	tool := NewTodosTool(sessions)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, sess.ID)

	createResp := runTodosTool(t, tool, ctx, TodosParams{Action: "create", Todos: []TodoItem{{
		Content:    "Wire task CRUD",
		Status:     "in_progress",
		ActiveForm: "Wiring task CRUD",
		Progress:   15,
	}}})
	createMeta := parseTodosMetadata(t, createResp)
	require.Equal(t, "create", createMeta.Action)
	require.Len(t, createMeta.Todos, 1)
	created := createMeta.Todos[0]
	require.NotEmpty(t, created.ID)
	require.Equal(t, 15, created.Progress)
	require.NotZero(t, created.StartedAt)

	updateResp := runTodosTool(t, tool, ctx, TodosParams{Action: "update", ID: created.ID, Todos: []TodoItem{{
		Progress:   80,
		ActiveForm: "Verifying CRUD behavior",
	}}})
	updateMeta := parseTodosMetadata(t, updateResp)
	require.Equal(t, "update", updateMeta.Action)
	require.NotNil(t, updateMeta.Current)
	require.Equal(t, created.ID, updateMeta.Current.ID)
	require.Equal(t, 80, updateMeta.Current.Progress)
	require.Equal(t, "Verifying CRUD behavior", updateMeta.Current.ActiveForm)
	require.GreaterOrEqual(t, updateMeta.Current.UpdatedAt, created.UpdatedAt)

	getResp := runTodosTool(t, tool, ctx, TodosParams{Action: "get", ID: created.ID})
	require.Contains(t, getResp.Content, "id="+created.ID)
	require.Contains(t, getResp.Content, "progress=80")

	completeResp := runTodosTool(t, tool, ctx, TodosParams{Action: "update", ID: created.ID, Todos: []TodoItem{{
		Status: "completed",
	}}})
	completeMeta := parseTodosMetadata(t, completeResp)
	require.NotNil(t, completeMeta.Current)
	require.Equal(t, 100, completeMeta.Current.Progress)
	require.NotZero(t, completeMeta.Current.CompletedAt)

	listResp := runTodosTool(t, tool, ctx, TodosParams{Action: "list"})
	require.Contains(t, listResp.Content, "Found 1 tracked tasks")
	require.Contains(t, listResp.Content, created.ID)

	deleteResp := runTodosTool(t, tool, ctx, TodosParams{Action: "delete", ID: created.ID})
	deleteMeta := parseTodosMetadata(t, deleteResp)
	require.Equal(t, created.ID, deleteMeta.DeletedID)
	require.Empty(t, deleteMeta.Todos)

	loaded, err := sessions.Get(context.Background(), sess.ID)
	require.NoError(t, err)
	require.Empty(t, loaded.Todos)
}

func TestTodosToolUpdateRequiresKnownID(t *testing.T) {
	t.Parallel()

	sessions := newTodosSessionService(t)
	sess, err := sessions.Create(context.Background(), "todos-errors")
	require.NoError(t, err)

	tool := NewTodosTool(sessions)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, sess.ID)

	_, err = tool.Run(ctx, fantasy.ToolCall{ID: "call-1", Name: TodosToolName, Input: `{"action":"update","id":"missing","todos":[{"progress":20}]}`})
	require.ErrorContains(t, err, `todo "missing" not found`)
}

var _ = sql.ErrNoRows
