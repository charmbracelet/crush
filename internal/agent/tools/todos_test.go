package tools

import (
	"context"
	"encoding/json"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

func TestBuildTodosResponse(t *testing.T) {
	t.Parallel()

	t.Run("explicit clear", func(t *testing.T) {
		t.Parallel()
		got := buildTodosResponse(nil, 0, 0, true, false, nil)
		require.Contains(t, got, "Todo list cleared")
		require.NotContains(t, got, "Completed")
	})

	t.Run("auto clear after all completed", func(t *testing.T) {
		t.Parallel()
		got := buildTodosResponse(nil, 0, 0, true, true, []string{"a", "b"})
		require.Contains(t, got, "Completed 2 todo")
		require.Contains(t, got, "cleared")
	})

	t.Run("no in_progress with pending work", func(t *testing.T) {
		t.Parallel()
		todos := []session.Todo{
			{Content: "a", Status: session.TodoStatusCompleted},
			{Content: "b", Status: session.TodoStatusPending},
		}
		got := buildTodosResponse(todos, 1, 0, false, false, nil)
		require.Contains(t, got, "No task is in_progress")
		require.Contains(t, got, "Remaining: b")
	})

	t.Run("healthy in_progress list", func(t *testing.T) {
		t.Parallel()
		todos := []session.Todo{
			{Content: "a", Status: session.TodoStatusCompleted},
			{Content: "b", Status: session.TodoStatusInProgress, ActiveForm: "Doing b"},
		}
		got := buildTodosResponse(todos, 1, 1, false, false, nil)
		require.Contains(t, got, "1 in progress")
		require.Contains(t, got, "clears automatically")
		require.Contains(t, got, "In progress: b")
	})

	t.Run("names all remaining items", func(t *testing.T) {
		t.Parallel()
		todos := []session.Todo{
			{Content: "a", Status: session.TodoStatusCompleted},
			{Content: "b", Status: session.TodoStatusInProgress},
			{Content: "c", Status: session.TodoStatusPending},
			{Content: "d", Status: session.TodoStatusPending},
		}
		got := buildTodosResponse(todos, 1, 1, false, false, nil)
		require.Contains(t, got, "In progress: b")
		require.Contains(t, got, "Remaining: c, d")
	})
}

func TestNormalizeTodosAutoClear(t *testing.T) {
	t.Parallel()

	// Mirror the tool's auto-clear decision without spinning up a session store.
	todos := []session.Todo{
		{Content: "a", Status: session.TodoStatusCompleted},
		{Content: "b", Status: session.TodoStatusCompleted},
	}
	completed := 0
	for _, tdo := range todos {
		if tdo.Status == session.TodoStatusCompleted {
			completed++
		}
	}
	autoCleared := len(todos) > 0 && completed == len(todos)
	require.True(t, autoCleared)
	if autoCleared {
		todos = nil
	}
	require.Nil(t, todos)
}

// fakeSessionStore is the minimum session.Service the todos tool needs.
// Everything else panics on the embedded nil interface, by design.
type fakeSessionStore struct {
	session.Service
	sess session.Session
}

func (f *fakeSessionStore) Get(context.Context, string) (session.Session, error) {
	return f.sess, nil
}

func (f *fakeSessionStore) Save(_ context.Context, s session.Session) (session.Session, error) {
	f.sess = s
	return s, nil
}

// runTodosTool drives the real tool through its fantasy.AgentTool boundary,
// so the test cannot drift from the tool's own auto-clear decision.
func runTodosTool(t *testing.T, store *fakeSessionStore, input string) (fantasy.ToolResponse, TodosResponseMetadata) {
	t.Helper()
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "s1")
	resp, err := NewTodosTool(store).Run(ctx, fantasy.ToolCall{ID: "tc1", Name: TodosToolName, Input: input})
	require.NoError(t, err)
	var meta TodosResponseMetadata
	require.NotEmpty(t, resp.Metadata, "tool must attach response metadata")
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
	return resp, meta
}

// TestTodosToolAutoClearKeepsCompletionSnapshot guards the transcript.
// Auto-clear nils the persisted list so the pill disappears, but the
// tool-call entry for that turn is rendered from the response metadata.
// Clearing the metadata too made internal/ui/chat/todos.go render
// "0/0 · completed all" with an empty body -- the finished checklist
// vanished from the transcript at the moment it was completed. The
// persisted list must be nil; the metadata must still describe what was
// submitted.
func TestTodosToolAutoClearKeepsCompletionSnapshot(t *testing.T) {
	t.Parallel()

	store := &fakeSessionStore{sess: session.Session{
		ID: "s1",
		Todos: []session.Todo{
			{Content: "a", Status: session.TodoStatusCompleted},
			{Content: "b", Status: session.TodoStatusInProgress},
		},
	}}

	_, meta := runTodosTool(t, store, `{"todos":[
		{"content":"a","status":"completed","active_form":"Doing a"},
		{"content":"b","status":"completed","active_form":"Doing b"}
	]}`)

	require.True(t, meta.AutoCleared, "an all-completed submission must auto-clear")
	require.Nil(t, store.sess.Todos, "the persisted list must be cleared so the pill goes away")

	// The metadata the renderer reads must describe the submitted list.
	require.Equal(t, 2, meta.Total, "renderer would show 0/0 without the snapshot")
	require.Equal(t, 2, meta.Completed)
	require.Len(t, meta.Todos, 2, "renderer needs the ticked list to draw a body")
	for _, td := range meta.Todos {
		require.Equal(t, session.TodoStatusCompleted, td.Status)
	}
	// allCompleted in the renderer is Completed == Total; 0 == 0 also
	// satisfies it, which is why Total alone is not enough of a guard.
	require.Equal(t, meta.Completed, meta.Total)
}

// TestTodosToolPartialListIsNotCleared pins the other side of the
// boundary: a list that still has work must be persisted, not cleared.
func TestTodosToolPartialListIsNotCleared(t *testing.T) {
	t.Parallel()

	store := &fakeSessionStore{sess: session.Session{ID: "s1"}}
	_, meta := runTodosTool(t, store, `{"todos":[
		{"content":"a","status":"completed","active_form":"Doing a"},
		{"content":"b","status":"in_progress","active_form":"Doing b"}
	]}`)

	require.False(t, meta.AutoCleared)
	require.False(t, meta.Cleared)
	require.Len(t, store.sess.Todos, 2, "an unfinished list must survive")
	require.Equal(t, 1, meta.Completed)
	require.Equal(t, 2, meta.Total)
}
