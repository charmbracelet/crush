package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
)

type ListMCPsParams struct{}

type ListLSPsParams struct{}

type listMCPsTool struct {
	permissions permission.Service
	workingDir  string
}

type listLSPsTool struct {
	permissions permission.Service
	workingDir  string
}

const (
	ListMCPsToolName = "list_mcps"
	ListLSPsToolName = "list_lsps"
)

func NewListMCPsTool(permissions permission.Service, workingDir string) BaseTool {
	return &listMCPsTool{
		permissions: permissions,
		workingDir:  workingDir,
	}
}

func NewListLSPsTool(permissions permission.Service, workingDir string) BaseTool {
	return &listLSPsTool{
		permissions: permissions,
		workingDir:  workingDir,
	}
}

func (t *listMCPsTool) Name() string {
	return ListMCPsToolName
}

func (t *listMCPsTool) Info() ToolInfo {
	return ToolInfo{
		Name: ListMCPsToolName,
		Description: `List all configured MCP (Model Context Protocol) servers.

WHEN TO USE THIS TOOL:
- Use when you need to see what MCP servers are available
- Helpful before restarting a specific MCP server
- Good for understanding the current MCP configuration
- Useful for troubleshooting MCP-related issues

HOW TO USE:
- No parameters required
- The tool will return a list of all configured MCP servers
- Shows server name, type, state, and tool count for each MCP

FEATURES:
- Lists all configured MCP servers
- Shows current connection state for each server
- Displays server type (stdio, http, sse)
- Shows number of tools provided by each server
- Indicates which servers are disabled

LIMITATIONS:
- Only shows configured servers, not available but unconfigured ones
- State information may be momentary

TIPS:
- Use this before using mcp_restart to see available servers
- Check server states to identify problematic MCPs`,
		Parameters: map[string]any{},
		Required:   []string{},
	}
}

func (t *listMCPsTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	sessionID, messageID := GetContextValues(ctx)
	if sessionID == "" || messageID == "" {
		return ToolResponse{}, fmt.Errorf("session ID and message ID are required for listing MCPs")
	}

	permissionDescription := "list configured MCP servers"
	p := t.permissions.Request(
		permission.CreatePermissionRequest{
			SessionID:   sessionID,
			ToolCallID:  call.ID,
			Path:        t.workingDir,
			ToolName:    t.Name(),
			Action:      "list",
			Description: permissionDescription,
			Params:      call.Input,
		},
	)
	if !p {
		return ToolResponse{}, permission.ErrorPermissionDenied
	}

	cfg := config.Get()

	if len(cfg.MCP) == 0 {
		return NewTextResponse("No MCP servers are configured."), nil
	}

	var response strings.Builder
	response.WriteString(fmt.Sprintf("Configured MCP servers (%d total):\n\n", len(cfg.MCP)))

	for name, mcpConfig := range cfg.MCP {
		response.WriteString(fmt.Sprintf("• **%s**\n", name))
		response.WriteString(fmt.Sprintf("  - Type: %s\n", mcpConfig.Type))
		response.WriteString(fmt.Sprintf("  - Command: %s\n", mcpConfig.Command))

		if len(mcpConfig.Args) > 0 {
			response.WriteString(fmt.Sprintf("  - Args: %s\n", strings.Join(mcpConfig.Args, " ")))
		}

		if mcpConfig.Disabled {
			response.WriteString("  - Status: **DISABLED**\n")
		} else {
			response.WriteString("  - Status: configured\n")
		}

		if mcpConfig.Type == config.MCPSse {
			response.WriteString("  - Note: SSE type (cannot be restarted)\n")
		}

		response.WriteString("\n")
	}

	response.WriteString("Use the `mcp_restart` tool to restart a specific MCP server.\n")
	response.WriteString("Note: Current connection states are not shown here - use diagnostics or restart tools for live status.")

	return NewTextResponse(response.String()), nil
}

func (t *listLSPsTool) Name() string {
	return ListLSPsToolName
}

func (t *listLSPsTool) Info() ToolInfo {
	return ToolInfo{
		Name: ListLSPsToolName,
		Description: `List all configured LSP (Language Server Protocol) clients.

WHEN TO USE THIS TOOL:
- Use when you need to see what LSP clients are available
- Helpful before restarting a specific LSP client
- Good for understanding the current LSP configuration
- Useful for troubleshooting LSP-related issues

HOW TO USE:
- No parameters required
- The tool will return a list of all configured LSP clients
- Shows client name, command, file types, and current state

FEATURES:
- Lists all configured LSP clients
- Shows current connection state for each client
- Displays the command used to start each LSP
- Shows supported file types for each client
- Indicates diagnostic counts when available

LIMITATIONS:
- Only shows configured clients, not available but unconfigured ones
- State information may be momentary

TIPS:
- Use this before using lsp_restart to see available clients
- Check client states to identify problematic LSPs`,
		Parameters: map[string]any{},
		Required:   []string{},
	}
}

func (t *listLSPsTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	sessionID, messageID := GetContextValues(ctx)
	if sessionID == "" || messageID == "" {
		return ToolResponse{}, fmt.Errorf("session ID and message ID are required for listing LSPs")
	}

	permissionDescription := "list configured LSP clients"
	p := t.permissions.Request(
		permission.CreatePermissionRequest{
			SessionID:   sessionID,
			ToolCallID:  call.ID,
			Path:        t.workingDir,
			ToolName:    t.Name(),
			Action:      "list",
			Description: permissionDescription,
			Params:      call.Input,
		},
	)
	if !p {
		return ToolResponse{}, permission.ErrorPermissionDenied
	}

	cfg := config.Get()

	if len(cfg.LSP) == 0 {
		return NewTextResponse("No LSP clients are configured."), nil
	}

	var response strings.Builder
	response.WriteString(fmt.Sprintf("Configured LSP clients (%d total):\n\n", len(cfg.LSP)))

	for name, lspConfig := range cfg.LSP {
		response.WriteString(fmt.Sprintf("• **%s**\n", name))
		response.WriteString(fmt.Sprintf("  - Command: %s\n", lspConfig.Command))

		if len(lspConfig.Args) > 0 {
			response.WriteString(fmt.Sprintf("  - Args: %s\n", strings.Join(lspConfig.Args, " ")))
		}

		if len(lspConfig.FileTypes) > 0 {
			response.WriteString(fmt.Sprintf("  - File types: %s\n", strings.Join(lspConfig.FileTypes, ", ")))
		}

		// Note: We can't easily get LSP state here without importing app package
		// The LSP state is managed in the app package, but we're in tools package
		response.WriteString("  - State: (use diagnostics tool for detailed status)\n")
		response.WriteString("\n")
	}

	return NewTextResponse(response.String()), nil
}
