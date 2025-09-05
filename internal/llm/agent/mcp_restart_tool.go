package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/llm/tools"
	"github.com/charmbracelet/crush/internal/permission"
)

type MCPRestartParams struct {
	Name string `json:"name"`
}

type mcpRestartTool struct {
	permissions permission.Service
	workingDir  string
}

const (
	MCPRestartToolName = "mcp_restart"
)

func NewMCPRestartTool(permissions permission.Service, workingDir string) tools.BaseTool {
	return &mcpRestartTool{
		permissions: permissions,
		workingDir:  workingDir,
	}
}

func (t *mcpRestartTool) Name() string {
	return MCPRestartToolName
}

func (t *mcpRestartTool) Info() tools.ToolInfo {
	return tools.ToolInfo{
		Name: MCPRestartToolName,
		Description: `Restart an MCP (Model Context Protocol) server.

WHEN TO USE THIS TOOL:
- Use when an MCP server has crashed or is not responding
- Helpful for recovering from MCP connection issues
- Good for refreshing MCP server state after configuration changes

HOW TO USE:
- Provide the name of the MCP server to restart
- The tool will kill the existing connection and reinitialize the server
- All tools from the MCP will be reloaded after restart

FEATURES:
- Gracefully shuts down existing MCP connection
- Reinitializes the MCP server with current configuration
- Reloads all tools from the restarted MCP
- Updates MCP state tracking

LIMITATIONS:
- Only works with stdio and HTTP MCP servers
- SSE MCP servers cannot be restarted (they are connection-based)
- Requires the MCP to be configured in the application

TIPS:
- Use this when MCP tools are failing or returning errors
- Check MCP configuration if restart fails repeatedly
- Monitor MCP state after restart to ensure successful recovery`,
		Parameters: map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "The name of the MCP server to restart",
			},
		},
		Required: []string{"name"},
	}
}

func (t *mcpRestartTool) Run(ctx context.Context, call tools.ToolCall) (tools.ToolResponse, error) {
	sessionID, messageID := tools.GetContextValues(ctx)
	if sessionID == "" || messageID == "" {
		return tools.ToolResponse{}, fmt.Errorf("session ID and message ID are required for MCP restart")
	}

	var params MCPRestartParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return tools.NewTextErrorResponse(fmt.Sprintf("error parsing parameters: %s", err)), nil
	}

	if params.Name == "" {
		return tools.NewTextErrorResponse("MCP name is required"), nil
	}

	permissionDescription := fmt.Sprintf("restart MCP server '%s'", params.Name)
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

	// Check if MCP exists in configuration
	cfg := config.Get()
	mcpConfig, exists := cfg.MCP[params.Name]
	if !exists {
		return tools.NewTextErrorResponse(fmt.Sprintf("MCP '%s' not found in configuration", params.Name)), nil
	}

	// Check if it's an SSE MCP (cannot be restarted)
	if mcpConfig.Type == config.MCPSse {
		return tools.NewTextErrorResponse(fmt.Sprintf("SSE MCP '%s' cannot be restarted (connection-based)", params.Name)), nil
	}

	// Get current MCP state
	currentState, exists := GetMCPState(params.Name)
	if !exists {
		return tools.NewTextErrorResponse(fmt.Sprintf("MCP '%s' state not found", params.Name)), nil
	}

	slog.Info("Restarting MCP server", "name", params.Name, "current_state", currentState.State)

	// Restart the MCP
	err := RestartMCP(ctx, params.Name)
	if err != nil {
		return tools.NewTextErrorResponse(fmt.Sprintf("Failed to restart MCP '%s': %s", params.Name, err)), nil
	}

	// Get new state after restart
	newState, _ := GetMCPState(params.Name)

	response := fmt.Sprintf("Successfully restarted MCP '%s'\n", params.Name)
	response += fmt.Sprintf("Previous state: %s\n", currentState.State)
	response += fmt.Sprintf("Current state: %s\n", newState.State)
	if newState.ToolCount > 0 {
		response += fmt.Sprintf("Loaded %d tools", newState.ToolCount)
	}

	return tools.NewTextResponse(response), nil
}
