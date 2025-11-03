package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	allTools     = csync.NewMap[string, *Tool]()
	client2Tools = csync.NewMap[string, []*Tool]()
)

// GetMCPTools returns all available MCP tools.
func GetMCPTools() iter.Seq[*Tool] {
	return allTools.Seq()
}

type Tool struct {
	mcpName         string
	tool            *mcp.Tool
	permissions     permission.Service
	workingDir      string
	providerOptions fantasy.ProviderOptions
}

func (m *Tool) SetProviderOptions(opts fantasy.ProviderOptions) {
	m.providerOptions = opts
}

func (m *Tool) ProviderOptions() fantasy.ProviderOptions {
	return m.providerOptions
}

func (m *Tool) Name() string {
	return fmt.Sprintf("mcp_%s_%s", m.mcpName, m.tool.Name)
}

func (m *Tool) MCP() string {
	return m.mcpName
}

func (m *Tool) MCPToolName() string {
	return m.tool.Name
}

func (m *Tool) Info() fantasy.ToolInfo {
	parameters := make(map[string]any)
	required := make([]string, 0)

	if input, ok := m.tool.InputSchema.(map[string]any); ok {
		if props, ok := input["properties"].(map[string]any); ok {
			parameters = props
		}
		if req, ok := input["required"].([]any); ok {
			// Convert []any -> []string when elements are strings
			for _, v := range req {
				if s, ok := v.(string); ok {
					required = append(required, s)
				}
			}
		} else if reqStr, ok := input["required"].([]string); ok {
			// Handle case where it's already []string
			required = reqStr
		}
	}

	return fantasy.ToolInfo{
		Name:        fmt.Sprintf("mcp_%s_%s", m.mcpName, m.tool.Name),
		Description: m.tool.Description,
		Parameters:  parameters,
		Required:    required,
	}
}

func (m *Tool) Run(ctx context.Context, params fantasy.ToolCall) (fantasy.ToolResponse, error) {
	sessionID := tools.GetSessionFromContext(ctx)
	if sessionID == "" {
		return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for creating a new file")
	}
	permissionDescription := fmt.Sprintf("execute %s with the following parameters:", m.Info().Name)
	p := m.permissions.Request(
		permission.CreatePermissionRequest{
			SessionID:   sessionID,
			ToolCallID:  params.ID,
			Path:        m.workingDir,
			ToolName:    m.Info().Name,
			Action:      "execute",
			Description: permissionDescription,
			Params:      params.Input,
		},
	)
	if !p {
		return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
	}

	return runTool(ctx, m.mcpName, m.tool.Name, params.Input)
}

func runTool(ctx context.Context, name, toolName string, input string) (fantasy.ToolResponse, error) {
	var args map[string]any
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("error parsing parameters: %s", err)), nil
	}

	c, err := getOrRenewClient(ctx, name)
	if err != nil {
		return fantasy.NewTextErrorResponse(err.Error()), nil
	}
	result, err := c.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	})
	if err != nil {
		return fantasy.NewTextErrorResponse(err.Error()), nil
	}

	output := make([]string, 0, len(result.Content))
	for _, v := range result.Content {
		if vv, ok := v.(*mcp.TextContent); ok {
			output = append(output, vv.Text)
		} else {
			output = append(output, fmt.Sprintf("%v", v))
		}
	}
	return fantasy.NewTextResponse(strings.Join(output, "\n")), nil
}

func getTools(ctx context.Context, name string, permissions permission.Service, c *mcp.ClientSession, workingDir string) ([]*Tool, error) {
	if c.InitializeResult().Capabilities.Tools == nil {
		return nil, nil
	}
	result, err := c.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return nil, err
	}
	mcpTools := make([]*Tool, 0, len(result.Tools))
	for _, tool := range result.Tools {
		mcpTools = append(mcpTools, &Tool{
			mcpName:     name,
			tool:        tool,
			permissions: permissions,
			workingDir:  workingDir,
		})
	}
	return mcpTools, nil
}

// updateTools updates the global mcpTools and mcpClient2Tools maps
func updateTools(mcpName string, tools []*Tool) {
	if len(tools) == 0 {
		client2Tools.Del(mcpName)
	} else {
		client2Tools.Set(mcpName, tools)
	}
	for _, tools := range client2Tools.Seq2() {
		for _, t := range tools {
			allTools.Set(t.Name(), t)
		}
	}
}
