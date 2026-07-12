package tools

import (
	"context"
	_ "embed"
	"fmt"
	"slices"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
)

const MCPRefreshToolName = "mcp_refresh"

//go:embed mcp_refresh.md
var mcpRefreshDescription string

type MCPRefreshParams struct {
	Name string `json:"name,omitempty" description:"Optional configured MCP server name. Omit to refresh all MCP servers."`
}

type mcpRefreshFunc func(context.Context, *config.ConfigStore, string) (map[string]error, error)

func NewMCPRefreshTool(cfg *config.ConfigStore) fantasy.AgentTool {
	return newMCPRefreshTool(cfg, mcp.Refresh)
}

func newMCPRefreshTool(cfg *config.ConfigStore, refresh mcpRefreshFunc) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		MCPRefreshToolName,
		mcpRefreshDescription,
		func(ctx context.Context, params MCPRefreshParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			results, err := refresh(ctx, cfg, strings.TrimSpace(params.Name))
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			names := make([]string, 0, len(results))
			for name := range results {
				names = append(names, name)
			}
			slices.Sort(names)
			if len(names) == 0 {
				return fantasy.NewTextResponse("Configuration reloaded; no MCP clients are configured."), nil
			}

			var output strings.Builder
			for _, name := range names {
				if results[name] != nil {
					fmt.Fprintf(&output, "%s: error: %v\n", name, results[name])
					continue
				}
				state, ok := mcp.GetState(name)
				if !ok {
					fmt.Fprintf(&output, "%s: removed\n", name)
					continue
				}
				fmt.Fprintf(&output, "%s: %s", name, state.State)
				if state.Error != nil {
					fmt.Fprintf(&output, ": %v", state.Error)
				}
				output.WriteByte('\n')
			}
			text := strings.TrimSpace(output.String())
			if strings.Contains(text, ": error:") {
				return fantasy.NewTextErrorResponse(text), nil
			}
			return fantasy.NewTextResponse(text), nil
		},
	)
}
