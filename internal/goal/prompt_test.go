package goal

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContextKeepsGoalPromptSmallAndCarriesPreviousTurn(t *testing.T) {
	t.Parallel()

	state := Start("configure GitHub MCP").WithStatus(StatusPaused, "authentication is configured")
	context := Context(state.Resume(), nil)

	require.Contains(t, context, "Objective: configure GitHub MCP")
	require.Contains(t, context, "Previous turn: authentication is configured")
	require.Contains(t, context, "A normal reply ends the turn")
}
