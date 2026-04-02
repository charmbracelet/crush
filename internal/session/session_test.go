package session

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeTodoStatus(t *testing.T) {
	t.Parallel()

	require.Equal(t, TodoStatusPending, NormalizeTodoStatus(""))
	require.Equal(t, TodoStatusPending, NormalizeTodoStatus("unknown"))
	require.Equal(t, TodoStatusPending, NormalizeTodoStatus(string(TodoStatusPending)))
	require.Equal(t, TodoStatusInProgress, NormalizeTodoStatus(string(TodoStatusInProgress)))
	require.Equal(t, TodoStatusCompleted, NormalizeTodoStatus(string(TodoStatusCompleted)))
	require.Equal(t, TodoStatusFailed, NormalizeTodoStatus(string(TodoStatusFailed)))
	require.Equal(t, TodoStatusCanceled, NormalizeTodoStatus(string(TodoStatusCanceled)))
}

func TestNormalizeCollaborationMode(t *testing.T) {
	t.Parallel()

	require.Equal(t, CollaborationModeDefault, NormalizeCollaborationMode(""))
	require.Equal(t, CollaborationModeDefault, NormalizeCollaborationMode("unknown"))
	require.Equal(t, CollaborationModeDefault, NormalizeCollaborationMode(string(CollaborationModeDefault)))
	require.Equal(t, CollaborationModeDefault, NormalizeCollaborationMode("auto"))
	require.Equal(t, CollaborationModePlan, NormalizeCollaborationMode(string(CollaborationModePlan)))
}

func TestNormalizePermissionMode(t *testing.T) {
	t.Parallel()

	require.Equal(t, PermissionModeDefault, NormalizePermissionMode(""))
	require.Equal(t, PermissionModeDefault, NormalizePermissionMode("unknown"))
	require.Equal(t, PermissionModeDefault, NormalizePermissionMode(string(PermissionModeDefault)))
	require.Equal(t, PermissionModeAuto, NormalizePermissionMode(string(PermissionModeAuto)))
	require.Equal(t, PermissionModeYolo, NormalizePermissionMode(string(PermissionModeYolo)))
}

func TestNormalizeKind(t *testing.T) {
	t.Parallel()

	require.Equal(t, KindNormal, NormalizeKind(""))
	require.Equal(t, KindNormal, NormalizeKind("unknown"))
	require.Equal(t, KindNormal, NormalizeKind(string(KindNormal)))
	require.Equal(t, KindHandoff, NormalizeKind(string(KindHandoff)))
}

func TestSessionLastTokenHelpers(t *testing.T) {
	t.Parallel()

	s := Session{
		LastPromptTokens:     1234,
		LastCompletionTokens: 56,
	}

	require.Equal(t, int64(1234), s.LastInputTokens())
	require.Equal(t, int64(56), s.LastOutputTokens())
	require.Equal(t, int64(1290), s.LastExchangeTokens())
}

func TestModeStateFromSession(t *testing.T) {
	t.Parallel()

	state := ModeStateFromSession(Session{
		CollaborationMode: CollaborationMode("invalid"),
		PermissionMode:    PermissionModeYolo,
	})

	require.Equal(t, CollaborationModeDefault, state.CollaborationMode)
	require.Equal(t, PermissionModeYolo, state.PermissionMode)
	require.Equal(t, "yolo", state.CurrentModeID())
}

func TestUnmarshalTodosAssignsLegacyIDsAndPreservesStructuredFields(t *testing.T) {
	t.Parallel()

	todos, err := unmarshalTodos(`[{"content":"Legacy task","status":"in_progress","active_form":"Working","progress":25},{"id":"task-2","content":"Done","status":"completed","progress":100,"completed_at":42}]`)
	require.NoError(t, err)
	require.Len(t, todos, 2)
	require.NotEmpty(t, todos[0].ID)
	require.Equal(t, 25, todos[0].Progress)
	require.Equal(t, TodoStatusInProgress, todos[0].Status)
	require.Equal(t, "task-2", todos[1].ID)
	require.Equal(t, int64(42), todos[1].CompletedAt)
}
