package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/charmbracelet/crush/internal/llm/tools"
	"github.com/charmbracelet/crush/internal/permission"
)

type LSPRestartParams struct {
	Name string `json:"name"`
}

type lspRestartTool struct {
	lspRestarter LSPRestarter
	permissions  permission.Service
	workingDir   string
}

const (
	LSPRestartToolName = "lsp_restart"
)

func NewLSPRestartTool(lspRestarter LSPRestarter, permissions permission.Service, workingDir string) tools.BaseTool {
	return &lspRestartTool{
		lspRestarter: lspRestarter,
		permissions:  permissions,
		workingDir:   workingDir,
	}
}

func (t *lspRestartTool) Name() string {
	return LSPRestartToolName
}

func (t *lspRestartTool) Info() tools.ToolInfo {
	return tools.ToolInfo{
		Name: LSPRestartToolName,
		Description: `Restart an LSP (Language Server Protocol) client.

WHEN TO USE THIS TOOL:
- Use when an LSP server has crashed or is not responding
- Helpful for recovering from LSP connection issues
- Good for refreshing LSP server state after file changes
- Useful when diagnostics are stale or incorrect

HOW TO USE:
- Provide the name of the LSP client to restart
- The tool will shut down the existing connection and reinitialize the server
- File watchers and diagnostics will be reestablished after restart

FEATURES:
- Gracefully shuts down existing LSP connection
- Reinitializes the LSP server with current configuration
- Reestablishes file watchers and diagnostics
- Updates LSP state tracking

LIMITATIONS:
- Requires the LSP to be configured in the application
- May take a few seconds to fully restart and become ready

TIPS:
- Use this when LSP diagnostics are showing errors for deleted files
- Check LSP configuration if restart fails repeatedly
- Monitor LSP state after restart to ensure successful recovery`,
		Parameters: map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "The name of the LSP client to restart",
			},
		},
		Required: []string{"name"},
	}
}

func (t *lspRestartTool) Run(ctx context.Context, call tools.ToolCall) (tools.ToolResponse, error) {
	sessionID, messageID := tools.GetContextValues(ctx)
	if sessionID == "" || messageID == "" {
		return tools.ToolResponse{}, fmt.Errorf("session ID and message ID are required for LSP restart")
	}

	var params LSPRestartParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return tools.NewTextErrorResponse(fmt.Sprintf("error parsing parameters: %s", err)), nil
	}

	if params.Name == "" {
		return tools.NewTextErrorResponse("LSP name is required"), nil
	}

	if t.lspRestarter == nil {
		return tools.NewTextErrorResponse("LSP restart functionality not available"), nil
	}

	permissionDescription := fmt.Sprintf("restart LSP client '%s'", params.Name)
	p := t.permissions.Request(
		permission.CreatePermissionRequest{
			SessionID:   sessionID,
			ToolCallID:  call.ID,
			Path:        t.workingDir,
			ToolName:    t.Name(),
			Action:      "restart",
			Description: permissionDescription,
			Params:      call.Input,
		},
	)
	if !p {
		return tools.ToolResponse{}, permission.ErrorPermissionDenied
	}

	slog.Info("Restarting LSP client", "name", params.Name)

	// Restart the LSP
	err := t.lspRestarter.RestartLSPClient(ctx, params.Name)
	if err != nil {
		return tools.NewTextErrorResponse(fmt.Sprintf("Failed to restart LSP client '%s': %s", params.Name, err)), nil
	}

	response := fmt.Sprintf("Successfully restarted LSP client '%s'", params.Name)
	return tools.NewTextResponse(response), nil
}
