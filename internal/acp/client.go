package acp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/charmbracelet/crush/internal/permission"
	"github.com/google/uuid"
)

// RunPermissionBridge subscribes to the permission service and forwards each
// request to the connected ACP client via session/request_permission.
//
// This is the ACP-mode equivalent of the TUI's permission dialog: the TUI
// subscribes to the same pubsub channel and calls Grant/Deny from UI code;
// here we do the same over the JSON-RPC wire.
//
// Must be called in a goroutine; it blocks until ctx is cancelled.
func RunPermissionBridge(ctx context.Context, perms permission.Service, server *Server) {
	ch := perms.Subscribe(ctx)
	for {
		select {
		case event, ok := <-ch:
			if !ok {
				return
			}
			req := event.Payload
			go handlePermissionRequest(ctx, req, perms, server)
		case <-ctx.Done():
			return
		}
	}
}

// handlePermissionRequest forwards a single permission request to the client
// and applies the result.
func handlePermissionRequest(ctx context.Context, req permission.PermissionRequest, perms permission.Service, server *Server) {
	allowOnceID := uuid.New().String()
	allowAlwaysID := uuid.New().String()
	rejectOnceID := uuid.New().String()
	rejectAlwaysID := uuid.New().String()

	toolCall := ACPToolCall{
		ToolCallID: req.ToolCallID,
		Title:      fmt.Sprintf("%s: %s", req.ToolName, req.Action),
		Kind:       "tool",
		Status:     ToolCallStatusRunning,
		RawInput:   req.Params,
	}

	params := RequestPermissionParams{
		SessionID: req.SessionID,
		ToolCall:  toolCall,
		Options: []PermissionOption{
			{OptionID: allowOnceID, Name: "Allow once", Kind: PermissionOptionAllowOnce},
			{OptionID: allowAlwaysID, Name: "Allow always", Kind: PermissionOptionAllowAlways},
			{OptionID: rejectOnceID, Name: "Reject", Kind: PermissionOptionRejectOnce},
			{OptionID: rejectAlwaysID, Name: "Reject always", Kind: PermissionOptionRejectAlways},
		},
	}

	raw, err := server.Call(ctx, "session/request_permission", params)
	if err != nil {
		slog.Warn("ACP: request_permission failed, denying", "err", err, "tool", req.ToolName)
		perms.Deny(req)
		return
	}

	var result RequestPermissionResult
	if err := json.Unmarshal(raw, &result); err != nil {
		slog.Warn("ACP: failed to parse permission result, denying", "err", err)
		perms.Deny(req)
		return
	}

	switch result.OptionID {
	case allowAlwaysID:
		perms.GrantPersistent(req)
	case allowOnceID:
		perms.Grant(req)
	default:
		// reject_once, reject_always, cancelled, or unknown
		perms.Deny(req)
	}
}
