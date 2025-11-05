package tools

import (
	"context"
	_ "embed"
	"log/slog"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
)

type ReadMCPResourceParams struct {
	Name string `json:"name" description:"MCP name"`
	URI  string `json:"uri,omitempty" description:"Resource URI"`
}

type ListMCPResourceParams struct {
	Name string `json:"name" description:"MCP name"`
}

const (
	ReadMCPResourceToolName = "read_mcp_resource"
	ListMCPResourceToolName = "list_mcp_resources"
)

//go:embed read_mcp_resource.md
var readMCPResourceDescription []byte

//go:embed list_mcp_resource.md
var listMCPResourceDescription []byte

func NewReadMCPResourceTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(
		ReadMCPResourceToolName,
		string(readMCPResourceDescription),
		func(ctx context.Context, input ReadMCPResourceParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			resource, err := mcp.ReadResource(ctx, input.Name, input.URI)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			var sb strings.Builder
			for _, part := range resource {
				if !strings.HasPrefix(part.MIMEType, "text/") {
					slog.Warn("ignoring resource of type", "type", part.MIMEType)
					continue
				}
				sb.WriteString(part.Text)
			}
			return fantasy.NewTextResponse(sb.String()), nil
		},
	)
}

func NewListMCPResourceTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(
		ListMCPResourceToolName,
		string(listMCPResourceDescription),
		func(ctx context.Context, input ListMCPResourceParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			for name, resources := range mcp.Resources() {
				if name != input.Name {
					continue
				}
				var sb strings.Builder
				sb.WriteString("Resources available for " + input.Name + ":\n\n")
				for _, res := range resources {
					sb.WriteString("- " + res.URI + "\n")
				}
				return fantasy.NewTextResponse(sb.String()), nil
			}
			return fantasy.NewTextResponse("No resources found for " + input.Name), nil
		},
	)
}
