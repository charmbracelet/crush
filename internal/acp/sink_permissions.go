package acp

import (
	"encoding/json"
	"log/slog"

	"github.com/charmbracelet/crush/internal/permission"
	"github.com/coder/acp-go-sdk"
)

// HandlePermission translates a permission request to an ACP permission request.
func (s *Sink) HandlePermission(req permission.PermissionRequest, permissions permission.Service) {
	// Only handle permissions for our session.
	if req.SessionID != s.sessionID {
		return
	}

	slog.Debug("ACP permission request", "tool", req.ToolName, "action", req.Action)

	// Build the tool call for the permission request.
	toolCall := acp.RequestPermissionToolCall{
		ToolCallId: acp.ToolCallId(req.ToolCallID),
		Title:      acp.Ptr(req.Description),
		Kind:       acp.Ptr(acp.ToolKindEdit),
		Status:     acp.Ptr(acp.ToolCallStatusPending),
		Locations:  []acp.ToolCallLocation{{Path: req.Path}},
		RawInput:   req.Params,
	}

	// For edit tools, include diff content so the client can show the proposed
	// changes.
	if meta := extractEditParams(req.Params); meta != nil && meta.FilePath != "" {
		toolCall.Content = []acp.ToolCallContent{
			acp.ToolDiffContent(meta.FilePath, meta.NewContent, meta.OldContent),
		}
	}

	resp, err := s.conn.RequestPermission(s.ctx, acp.RequestPermissionRequest{
		SessionId: acp.SessionId(s.sessionID),
		ToolCall:  toolCall,
		Options: []acp.PermissionOption{
			{Kind: acp.PermissionOptionKindAllowOnce, Name: "Allow", OptionId: "allow"},
			{Kind: acp.PermissionOptionKindAllowAlways, Name: "Allow always", OptionId: "allow_always"},
			{Kind: acp.PermissionOptionKindRejectOnce, Name: "Deny", OptionId: "deny"},
		},
	})
	if err != nil {
		slog.Error("Failed to request permission", "error", err)
		permissions.Deny(req)
		return
	}

	if resp.Outcome.Cancelled != nil {
		permissions.Deny(req)
		return
	}

	if resp.Outcome.Selected != nil {
		switch string(resp.Outcome.Selected.OptionId) {
		case "allow":
			permissions.Grant(req)
		case "allow_always":
			permissions.GrantPersistent(req)
		default:
			permissions.Deny(req)
		}
	}
}

// editParams holds fields needed for diff content in permission requests.
type editParams struct {
	FilePath   string `json:"file_path"`
	OldContent string `json:"old_content"`
	NewContent string `json:"new_content"`
}

// extractEditParams attempts to extract edit parameters from permission params.
func extractEditParams(params any) *editParams {
	if params == nil {
		return nil
	}

	// Try JSON round-trip to extract fields.
	data, err := json.Marshal(params)
	if err != nil {
		return nil
	}

	var ep editParams
	if err := json.Unmarshal(data, &ep); err != nil {
		return nil
	}

	return &ep
}
