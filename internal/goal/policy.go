package goal

import "charm.land/fantasy"

type DecisionKind string

const (
	DecisionStop     DecisionKind = "stop"
	DecisionContinue DecisionKind = "continue"
	DecisionPause    DecisionKind = "pause"
)

type ContinueReason string

const (
	ReasonOutputLimit ContinueReason = "output_limit"
	ReasonRunaway     ContinueReason = "runaway_guard"
	ReasonEndTurn     ContinueReason = "end_turn"
)

type Decision struct {
	Kind   DecisionKind
	Reason ContinueReason
}

// Decide runs after Fantasy finishes one ordinary agent loop. Fantasy owns
// tool execution; this policy only decides whether the larger goal continues.
func Decide(state State, finish fantasy.FinishReason, madeProgress bool) Decision {
	if !state.Active() {
		return Decision{Kind: DecisionStop}
	}
	if state.Turns >= MaxAutomaticTurns {
		return Decision{Kind: DecisionPause, Reason: ReasonRunaway}
	}
	if finish == fantasy.FinishReasonLength {
		return Decision{Kind: DecisionContinue, Reason: ReasonOutputLimit}
	}
	return Decision{Kind: DecisionStop, Reason: ReasonEndTurn}
}
