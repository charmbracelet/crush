package goal

import (
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

func TestDecideStopsAutomaticContinuationAfterNormalStop(t *testing.T) {
	t.Parallel()

	decision := Decide(Start("finish the refactor"), fantasy.FinishReasonStop, false)
	require.Equal(t, DecisionStop, decision.Kind)
	require.Equal(t, ReasonEndTurn, decision.Reason)
}

func TestDecideContinuesAfterOutputLimit(t *testing.T) {
	t.Parallel()

	decision := Decide(Start("finish the refactor"), fantasy.FinishReasonLength, false)
	require.Equal(t, DecisionContinue, decision.Kind)
	require.Equal(t, ReasonOutputLimit, decision.Reason)
}

func TestDecideStopsTerminalGoal(t *testing.T) {
	t.Parallel()

	state := Start("finish the refactor").WithStatus(StatusComplete, "verified")
	require.Equal(t, DecisionStop, Decide(state, fantasy.FinishReasonStop, false).Kind)
}

func TestDecidePausesRunawayGoal(t *testing.T) {
	t.Parallel()

	state := Start("finish the refactor")
	state.Turns = MaxAutomaticTurns
	decision := Decide(state, fantasy.FinishReasonStop, false)
	require.Equal(t, DecisionPause, decision.Kind)
	require.Equal(t, ReasonRunaway, decision.Reason)
}

func TestDecideStopsAfterProgressfulNormalTurn(t *testing.T) {
	t.Parallel()

	state := Start("finish the refactor").NextTurn()
	decision := Decide(state, fantasy.FinishReasonStop, true)
	require.Equal(t, DecisionStop, decision.Kind)
	require.Equal(t, ReasonEndTurn, decision.Reason)
}

func TestResumePreservesObjectiveAndSummary(t *testing.T) {
	t.Parallel()

	state := Start("configure GitHub MCP").WithStatus(StatusPaused, "waiting for user review")
	resumed := state.Resume()

	require.Equal(t, StatusActive, resumed.Status)
	require.Equal(t, state.Objective, resumed.Objective)
	require.Equal(t, state.Summary, resumed.Summary)
}
