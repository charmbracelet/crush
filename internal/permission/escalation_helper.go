package permission

import (
	"context"
	"encoding/json"
	"fmt"
)

// EscalateToLeader checks if escalation is needed and routes to the leader.
// Returns nil if escalation is not available or not needed.
// Returns an error response if escalation was denied.
// Returns a permission decision if escalation succeeded.
func EscalateToLeader(
	ctx context.Context,
	toolName string,
	input interface{},
	description string,
) (*EscalationResponse, error) {
	bridge := EscalationBridgeFromContext(ctx)
	if bridge == nil {
		return nil, nil
	}

	identity := WorkerIdentityFromContext(ctx)
	if identity.AgentID == "" {
		return nil, nil
	}

	inputMap := make(map[string]interface{})
	if input != nil {
		inputBytes, err := json.Marshal(input)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize escalation input: %w", err)
		}
		if err := json.Unmarshal(inputBytes, &inputMap); err != nil {
			return nil, fmt.Errorf("failed to map escalation input to object: %w", err)
		}
	}

	req := EscalationRequest{
		WorkerID:    identity.AgentID,
		WorkerName:  identity.AgentName,
		WorkerColor: identity.Color,
		ToolName:    toolName,
		ToolInput:   inputMap,
		Description: description,
	}

	return bridge.RequestEscalation(ctx, req)
}

// ShouldEscalate determines if a permission decision should be escalated to the leader.
// Workers call this when they encounter "ask" decisions from their permission checker.
func ShouldEscalate(ctx context.Context, decision string) bool {
	if decision != "ask" {
		return false
	}

	identity := WorkerIdentityFromContext(ctx)
	return identity.AgentID != ""
}

// FormatWorkerBadge creates a display string for a worker in permission prompts.
func FormatWorkerBadge(identity WorkerIdentity) string {
	if identity.AgentName == "" && identity.AgentID == "" {
		return "unknown worker"
	}
	if identity.AgentName == "" {
		return identity.AgentID
	}
	if identity.Color != "" {
		return fmt.Sprintf("[%s] %s", identity.Color, identity.AgentName)
	}
	return identity.AgentName
}

// WorkerBadgeFromContext extracts and formats a worker badge from context.
func WorkerBadgeFromContext(ctx context.Context) string {
	identity := WorkerIdentityFromContext(ctx)
	if identity.AgentID == "" {
		return ""
	}
	return FormatWorkerBadge(identity)
}
