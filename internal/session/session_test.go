package session

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
