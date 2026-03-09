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
	require.Equal(t, CollaborationModePlan, NormalizeCollaborationMode(string(CollaborationModePlan)))
}
