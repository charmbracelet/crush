package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
)

const MCPToolCallToolName = "mcp_tool_call"

//go:embed mcp_tool_call.md
var mcpToolCallDescription string

type MCPToolCallParams struct {
	Server    string         `json:"server" description:"Exact MCP server name returned by mcp_tool_search."`
	Tool      string         `json:"tool" description:"Exact server-native tool name returned by mcp_tool_search."`
	Arguments map[string]any `json:"arguments" description:"Arguments matching the selected MCP tool schema."`
}

// NewMCPToolCallTool provides an opt-in compatibility dispatcher for custom
// agents whose models cannot reliably use deferred native tools.
func NewMCPToolCallTool(permissions permission.Service, cfg *config.ConfigStore, workingDir string, allowed map[string][]string) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		MCPToolCallToolName,
		mcpToolCallDescription,
		func(ctx context.Context, params MCPToolCallParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			server := strings.TrimSpace(params.Server)
			toolName := strings.TrimSpace(params.Tool)
			if server == "" || toolName == "" {
				return fantasy.NewTextErrorResponse("server and tool are required; search for their exact names first"), nil
			}
			for _, tool := range allowedMCPTools(GetMCPTools(permissions, cfg, workingDir), allowed) {
				if tool.MCP() != server || tool.MCPToolName() != toolName {
					continue
				}
				input, err := json.Marshal(params.Arguments)
				if err != nil {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to encode MCP arguments: %v", err)), nil
				}
				return tool.Run(ctx, fantasy.ToolCall{ID: call.ID, Name: tool.Name(), Input: string(input)})
			}
			return fantasy.NewTextErrorResponse(fmt.Sprintf("MCP tool %q on server %q is not currently usable", toolName, server)), nil
		},
	)
}
