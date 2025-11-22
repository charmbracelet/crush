package enum

import "charm.land/fantasy"

// ToolCallStateMapping provides centralized logic for mapping between different state types
type ToolCallStateMapping struct{}

// NewToolCallStateMapping creates a new mapping instance
func NewToolCallStateMapping() *ToolCallStateMapping {
	return &ToolCallStateMapping{}
}

// ToolCallStateToResultState maps ToolCallState to ToolResultState
// This is the single source of truth for converting tool execution states to result states
func (m *ToolCallStateMapping) ToolCallStateToResultState(state ToolCallState) ToolResultState {
	switch state {
	case ToolCallStateCompleted:
		return ToolResultStateSuccess
	case ToolCallStateFailed:
		return ToolResultStateError
	case ToolCallStateCancelled:
		return ToolResultStateCancelled
	case ToolCallStatePermissionDenied:
		return ToolResultStateError
	case ToolCallStateRunning, ToolCallStatePending:
		return ToolResultStateUnknown
	default:
		return ToolResultStateUnknown
	}
}

// ToolCallStateToFinishReason maps ToolCallState to fantasy.FinishReason
// Used when tool state determines the overall message completion reason
func (m *ToolCallStateMapping) ToolCallStateToFinishReason(state ToolCallState) fantasy.FinishReason {
	switch state {
	case ToolCallStateCompleted:
		return fantasy.FinishReasonToolCalls
	case ToolCallStateFailed:
		return fantasy.FinishReasonError
	case ToolCallStateCancelled:
		return fantasy.FinishReasonStop
	case ToolCallStatePermissionDenied:
		return fantasy.FinishReasonError
	default:
		return fantasy.FinishReasonUnknown
	}
}

// ResultStateToToolCallState maps ToolResultState to ToolCallState
// Used when result determines the final tool call state
func (m *ToolCallStateMapping) ResultStateToToolCallState(resultState ToolResultState) ToolCallState {
	switch resultState {
	case ToolResultStateSuccess, ToolResultStatePartial:
		return ToolCallStateCompleted
	case ToolResultStateError, ToolResultStateTimeout:
		return ToolCallStateFailed
	case ToolResultStateCancelled:
		return ToolCallStateCancelled
	default:
		return ToolCallStatePending
	}
}

// IsErrorState returns true if the given state represents an error condition
// This replaces scattered IsError() calls throughout the codebase
func (m *ToolCallStateMapping) IsErrorState(state ToolCallState) bool {
	return state == ToolCallStateFailed ||
		state == ToolCallStatePermissionDenied
}

// IsSuccessState returns true if the given state represents success
func (m *ToolCallStateMapping) IsSuccessState(state ToolCallState) bool {
	return state == ToolCallStateCompleted
}

// IsFinalState returns true if the state is terminal (no further changes expected)
func (m *ToolCallStateMapping) IsFinalState(state ToolCallState) bool {
	return state == ToolCallStateCompleted ||
		state == ToolCallStateFailed ||
		state == ToolCallStateCancelled ||
		state == ToolCallStatePermissionDenied
}

// Global mapping instance for convenience
var StateMapping = NewToolCallStateMapping()

// Convenience functions that use the global mapping instance

// ToolCallStateToResultState maps ToolCallState to ToolResultState
func ToolCallStateToResultState(state ToolCallState) ToolResultState {
	return StateMapping.ToolCallStateToResultState(state)
}

// ToolCallStateToFinishReason maps ToolCallState to fantasy.FinishReason
func ToolCallStateToFinishReason(state ToolCallState) fantasy.FinishReason {
	return StateMapping.ToolCallStateToFinishReason(state)
}

// ResultStateToToolCallState maps ToolResultState to ToolCallState
func ResultStateToToolCallState(resultState ToolResultState) ToolCallState {
	return StateMapping.ResultStateToToolCallState(resultState)
}
