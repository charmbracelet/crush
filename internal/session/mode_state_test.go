package session

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestModeTransitionNormalizesAndTracksAutoExit(t *testing.T) {
	t.Parallel()

	current := Session{
		CollaborationMode: CollaborationModeDefault,
		PermissionMode:    PermissionModeAuto,
	}
	next := PermissionMode("invalid")

	transition := NewModeTransition(current, nil, &next)

	require.Equal(t, PermissionModeAuto, transition.Previous.PermissionMode)
	require.Equal(t, PermissionModeDefault, transition.Current.PermissionMode)
	require.True(t, transition.Changed())
	require.True(t, transition.ExitedAutoMode())
	require.Equal(t, "default", transition.Current.CurrentModeID())
}

func TestModeTransitionPreservesPermissionWhileEnteringPlanMode(t *testing.T) {
	t.Parallel()

	current := Session{
		CollaborationMode: CollaborationModeDefault,
		PermissionMode:    PermissionModeYolo,
	}
	plan := CollaborationModePlan

	transition := NewModeTransition(current, &plan, nil)

	require.Equal(t, CollaborationModePlan, transition.Current.CollaborationMode)
	require.Equal(t, PermissionModeYolo, transition.Current.PermissionMode)
	require.True(t, transition.Current.IsPlanMode())
	require.False(t, transition.ExitedAutoMode())
}
