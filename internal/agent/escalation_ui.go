package agent

import (
	"context"
	"errors"

	"github.com/charmbracelet/crush/internal/permission"
)

// EscalationUI provides helpers for UI integration with the escalation bridge.
type EscalationUI struct {
	bridge *permission.EscalationBridge
}

// NewEscalationUI creates a new escalation UI helper.
func NewEscalationUI(bridge *permission.EscalationBridge) *EscalationUI {
	return &EscalationUI{bridge: bridge}
}

// HasPendingEscalations checks if there are pending permission escalations.
func (ui *EscalationUI) HasPendingEscalations() bool {
	return ui.bridge != nil && ui.bridge.HasPendingEscalations()
}

// GetPendingEscalations returns all pending escalation requests for UI display.
func (ui *EscalationUI) GetPendingEscalations() []permission.EscalationRequest {
	if ui.bridge == nil {
		return nil
	}
	return ui.bridge.GetPendingEscalations()
}

// ApproveEscalation approves a pending escalation request.
func (ui *EscalationUI) ApproveEscalation(requestID, reason string) error {
	if ui.bridge == nil {
		return errors.New("no escalation bridge available to approve escalation")
	}
	return ui.bridge.RespondToEscalation(permission.EscalationResponse{
		RequestID: requestID,
		Approved:  true,
		Reason:    reason,
	})
}

// DenyEscalation denies a pending escalation request.
func (ui *EscalationUI) DenyEscalation(requestID, reason string) error {
	if ui.bridge == nil {
		return errors.New("no escalation bridge available to deny escalation")
	}
	return ui.bridge.RespondToEscalation(permission.EscalationResponse{
		RequestID: requestID,
		Approved:  false,
		Reason:    reason,
	})
}

// FormatEscalationPrompt formats an escalation request for UI display.
func FormatEscalationPrompt(req permission.EscalationRequest) string {
	badge := permission.FormatWorkerBadge(permission.WorkerIdentity{
		AgentName: req.WorkerName,
		Color:     req.WorkerColor,
	})

	if badge == "" {
		badge = "Worker"
	}

	return badge + " requests permission for: " + req.ToolName + "\n" + req.Description
}

// EscalationFromContext extracts the escalation UI helper from context.
func EscalationFromContext(ctx context.Context) *EscalationUI {
	bridge := permission.EscalationBridgeFromContext(ctx)
	if bridge == nil {
		return nil
	}
	return NewEscalationUI(bridge)
}
