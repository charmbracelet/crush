package acp

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/charmbracelet/crush/internal/permission"
	acp "github.com/coder/acp-go-sdk"
)

// permissionBridge handles the mapping between Crush permission requests
// and ACP session/request_permission calls.
type permissionBridge struct {
	conn      *acp.AgentSideConnection
	log       *slog.Logger
	permitted map[permissionKey]bool
	mu        sync.Mutex
}

type permissionKey struct {
	sessionID  string
	toolCallID string
}

// newPermissionBridge creates a bridge that forwards permission requests
// to the ACP client via conn.RequestPermission.
func newPermissionBridge(conn *acp.AgentSideConnection, log *slog.Logger) *permissionBridge {
	return &permissionBridge{
		conn:      conn,
		log:       log,
		permitted: make(map[permissionKey]bool),
	}
}

// CheckPermission is called before executing a tool that requires permission.
// It returns true if the operation is allowed, false if denied.
func (b *permissionBridge) CheckPermission(ctx context.Context, req permission.PermissionRequest) (bool, error) {
	key := permissionKey{sessionID: req.SessionID, toolCallID: req.ToolCallID}

	// If the session or request was already cancelled, resolve as a denial
	// without error so the turn ends with StopReasonCancelled rather than a
	// spurious tool error.
	if ctx.Err() != nil {
		return false, nil
	}

	// Check persistent grants first.
	b.mu.Lock()
	if granted, ok := b.permitted[key]; ok {
		b.mu.Unlock()
		return granted, nil
	}
	b.mu.Unlock()

	// Build the tool call update for the permission request.
	title := fmt.Sprintf("%s: %s", req.ToolName, req.Description)
	toolCallUpdate := acp.ToolCallUpdate{
		ToolCallId: acp.ToolCallId(req.ToolCallID),
		Title:      &title,
	}

	options := buildPermissionOptions(req.Action)

	permissionReq := acp.RequestPermissionRequest{
		SessionId: acp.SessionId(req.SessionID),
		ToolCall:  toolCallUpdate,
		Options:   options,
	}

	b.log.Info("ACP requesting permission", "sessionId", req.SessionID, "tool", req.ToolName)

	resp, err := b.conn.RequestPermission(ctx, permissionReq)
	if err != nil {
		// A cancellation during the prompt yields a context error; treat it as a
		// denial with no error so the turn terminates cleanly.
		if ctx.Err() != nil {
			return false, nil
		}
		return false, fmt.Errorf("request permission: %w", err)
	}

	// Map the outcome back to a boolean decision.
	granted := mapPermissionOutcome(resp.Outcome)

	// Record persistent grants.
	if granted && isAllowAlways(resp.Outcome) {
		b.mu.Lock()
		b.permitted[key] = true
		b.mu.Unlock()
	}

	b.log.Info("ACP permission result", "sessionId", req.SessionID, "granted", granted)
	return granted, nil
}

// buildPermissionOptions creates ACP permission options from Crush's action type.
func buildPermissionOptions(_ string) []acp.PermissionOption {
	return []acp.PermissionOption{
		{
			OptionId: acp.PermissionOptionId("allow_once"),
			Name:     "Allow once",
			Kind:     acp.PermissionOptionKindAllowOnce,
		},
		{
			OptionId: acp.PermissionOptionId("allow_always"),
			Name:     "Allow always",
			Kind:     acp.PermissionOptionKindAllowAlways,
		},
		{
			OptionId: acp.PermissionOptionId("reject_once"),
			Name:     "Reject once",
			Kind:     acp.PermissionOptionKindRejectOnce,
		},
	}
}

// mapPermissionOutcome converts an ACP outcome to a boolean grant decision.
func mapPermissionOutcome(outcome acp.RequestPermissionOutcome) bool {
	if outcome.Selected != nil {
		return outcome.Selected.OptionId != "reject_once"
	}
	return false
}

// isAllowAlways checks if the outcome indicates a permanent grant.
func isAllowAlways(outcome acp.RequestPermissionOutcome) bool {
	if outcome.Selected != nil {
		return outcome.Selected.OptionId == "allow_always"
	}
	return false
}
