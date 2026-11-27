package acp

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"

	"github.com/charmbracelet/crush/internal/permission"
	acp "github.com/coder/acp-go-sdk"
)

// permissionBridge handles the mapping between Crush permission requests
// and ACP session/request_permission calls.
type permissionBridge struct {
	conn *acp.AgentSideConnection
	log  *slog.Logger
	// absPath resolves a tool path into the absolute form every path on the
	// wire must take (overview.md:160); the diff content a prompt shows uses
	// it. A nil resolver leaves paths as the tool reported them.
	absPath func(string) string
	// remembered holds allow_always/reject_always decisions by session and
	// tool scope. The spec makes the Agent the owner of a remembered
	// decision's scope (tool-calls.md:237-241); the tool name is the scope
	// Crush can honour, and the connection's lifetime is how long it lasts.
	remembered map[scopeKey]bool
	mu         sync.Mutex
}

type scopeKey struct {
	sessionID string
	toolName  string
}

// newPermissionBridge creates a bridge that forwards permission requests
// to the ACP client via conn.RequestPermission.
func newPermissionBridge(conn *acp.AgentSideConnection, log *slog.Logger) *permissionBridge {
	return &permissionBridge{
		conn:       conn,
		log:        log,
		remembered: make(map[scopeKey]bool),
	}
}

// CheckPermission is called before executing a tool that requires permission.
// It returns true if the operation is allowed, false if denied.
func (b *permissionBridge) CheckPermission(ctx context.Context, req permission.PermissionRequest) (bool, error) {
	// If the session or request was already cancelled, resolve as a denial
	// without error so the turn ends with StopReasonCancelled rather than a
	// spurious tool error.
	if ctx.Err() != nil {
		return false, nil
	}

	// A remembered decision for this tool in this session applies without
	// asking again. Only the always kinds land in the cache, so an allow_once
	// never leaks into a later call.
	scope := scopeKey{sessionID: req.SessionID, toolName: req.ToolName}
	b.mu.Lock()
	granted, remembered := b.remembered[scope]
	b.mu.Unlock()
	if remembered {
		return granted, nil
	}

	// Build the tool call update for the permission request. A
	// file-modifying tool carries a diff of the change it is about to make,
	// so the client can show what is being approved rather than only the
	// file name (tool-calls.md:218-229).
	title := fmt.Sprintf("%s: %s%s", req.ToolName, req.Description, executionControlSuffix(req.Path))
	diffs := toolDiffContent(b.absPath, req.Params)
	toolCallUpdate := acp.ToolCallUpdate{
		ToolCallId: acp.ToolCallId(req.ToolCallID),
		Title:      acp.PatchSet(title),
		Kind:       acp.PatchSet(toolKindFor(req.ToolName)),
	}
	if path := toolFilePath(req.Params); path != "" {
		toolCallUpdate.Locations = acp.PatchSet([]acp.ToolCallLocation{{Path: b.resolveAbs(path)}})
	}
	if len(diffs) > 0 {
		toolCallUpdate.Content = acp.PatchSet(diffs)
	}

	// Report the pending status while the user decides (S10 lifecycle: a tool
	// call sits in pending from its first report until permission resolves).
	b.pushToolCallStatus(ctx, req.SessionID, req.ToolCallID, title, acp.ToolCallStatusPending, diffs)

	options := buildPermissionOptions(req.ToolName)

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
	granted = mapPermissionOutcome(resp.Outcome)

	// Record remembered decisions. The once kinds are deliberately not
	// cached: they apply to the call they were granted for and no further.
	if sel := resp.Outcome.Selected; sel != nil {
		switch sel.OptionId {
		case acp.PermissionOptionId("allow_always"):
			b.remember(scope, true)
		case acp.PermissionOptionId("reject_always"):
			b.remember(scope, false)
		}
	}

	// A denied permission terminates the tool call: report it as failed so the
	// client does not show a tool stuck in pending/in_progress (S10).
	if !granted && resp.Outcome.Cancelled == nil {
		b.pushToolCallStatus(ctx, req.SessionID, req.ToolCallID, title,
			acp.ToolCallStatusFailed, []acp.ToolCallContent{acp.ToolContent(acp.TextBlock("Permission denied by user"))})
	}

	b.log.Info("ACP permission result", "sessionId", req.SessionID, "tool", req.ToolName, "granted", granted)
	return granted, nil
}

// remember stores an always-kind decision for the session+tool scope.
func (b *permissionBridge) remember(scope scopeKey, granted bool) {
	b.mu.Lock()
	b.remembered[scope] = granted
	b.mu.Unlock()
}

// pushToolCallStatus emits a tool_call_update so the client observes the
// pending/failed transitions around a permission request. Content is any
// tool-call content to attach, such as the diff a write would make.
func (b *permissionBridge) pushToolCallStatus(ctx context.Context, sessionID, toolCallID, title string, status acp.ToolCallStatus, content []acp.ToolCallContent) {
	opts := []acp.ToolCallUpdateOpt{acp.WithUpdateStatus(status)}
	if title != "" {
		opts = append(opts, acp.WithUpdateTitle(title))
	}
	if len(content) > 0 {
		opts = append(opts, acp.WithUpdateContent(content))
	}
	note := acp.SessionNotification{
		SessionId: acp.SessionId(sessionID),
		Update:    acp.UpdateToolCall(acp.ToolCallId(toolCallID), opts...),
	}
	if err := b.conn.SessionUpdate(ctx, note); err != nil {
		b.log.Warn("Failed to send tool call status update", "error", err)
	}
}

// resolveAbs resolves a path to the absolute form the wire requires, falling
// back to the path as given when no resolver is installed.
func (b *permissionBridge) resolveAbs(path string) string {
	if b.absPath == nil {
		return path
	}
	return b.absPath(path)
}

// executionControlSuffix labels writes to files that can change what runs or
// what gets approved. This is display-only; permission behaviour is unchanged.
func executionControlSuffix(path string) string {
	clean := filepath.ToSlash(strings.ToLower(path))
	base := filepath.Base(clean)
	controlFiles := map[string]struct{}{
		"crush.json":    {},
		".mcp.json":     {},
		"mcp.json":      {},
		"settings.json": {},
	}
	if _, ok := controlFiles[base]; ok {
		return " [execution/approval config]"
	}
	for _, marker := range []string{"/.git/hooks/", "/.crush/", "/.vscode/", "/.idea/"} {
		if strings.Contains(clean, marker) {
			return " [execution/approval config]"
		}
	}
	return ""
}

// buildPermissionOptions creates ACP permission options for the tool being
// approved. All four kinds the spec defines are offered (tool-calls.md:232-
// 235), and the always-kinds state their scope in the name, which the spec
// asks for (tool-calls.md:250). The allow-once option stays first so a client
// that picks the first option by default grants rather than denies.
func buildPermissionOptions(toolName string) []acp.PermissionOption {
	alwaysScope := "always"
	if toolName != "" {
		alwaysScope = "always for " + toolName
	}
	return []acp.PermissionOption{
		{
			OptionId: acp.PermissionOptionId("allow_once"),
			Name:     "Allow once",
			Kind:     acp.PermissionOptionKindAllowOnce,
		},
		{
			OptionId: acp.PermissionOptionId("allow_always"),
			Name:     "Allow " + alwaysScope,
			Kind:     acp.PermissionOptionKindAllowAlways,
		},
		{
			OptionId: acp.PermissionOptionId("reject_once"),
			Name:     "Reject once",
			Kind:     acp.PermissionOptionKindRejectOnce,
		},
		{
			OptionId: acp.PermissionOptionId("reject_always"),
			Name:     "Reject " + alwaysScope,
			Kind:     acp.PermissionOptionKindRejectAlways,
		},
	}
}

// mapPermissionOutcome converts an ACP outcome to a boolean grant decision.
// Only the two allow options grant: the spec requires that an unrecognised
// outcome is never treated as approval (tool-calls.md:248).
func mapPermissionOutcome(outcome acp.RequestPermissionOutcome) bool {
	if outcome.Selected == nil {
		return false
	}
	switch outcome.Selected.OptionId {
	case acp.PermissionOptionId("allow_once"), acp.PermissionOptionId("allow_always"):
		return true
	default:
		return false
	}
}

// isAllowAlways checks if the outcome indicates a permanent grant.
func isAllowAlways(outcome acp.RequestPermissionOutcome) bool {
	if outcome.Selected != nil {
		return outcome.Selected.OptionId == "allow_always"
	}
	return false
}
