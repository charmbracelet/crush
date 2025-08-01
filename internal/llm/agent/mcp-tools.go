package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"sync"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/llm/tools"
	"github.com/charmbracelet/crush/internal/version"

	"github.com/charmbracelet/crush/internal/permission"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

var (
	mcpToolsOnce sync.Once
	mcpTools     []tools.BaseTool
	mcpClients   = csync.NewMap[string, *client.Client]()
)

type mcpTool struct {
	mcpName     string
	tool        mcp.Tool
	permissions permission.Service
	workingDir  string
}

func (b *mcpTool) Name() string {
	return fmt.Sprintf("mcp_%s_%s", b.mcpName, b.tool.Name)
}

func (b *mcpTool) Info() tools.ToolInfo {
	required := b.tool.InputSchema.Required
	if required == nil {
		required = make([]string, 0)
	}
	return tools.ToolInfo{
		Name:        fmt.Sprintf("mcp_%s_%s", b.mcpName, b.tool.Name),
		Description: b.tool.Description,
		Parameters:  b.tool.InputSchema.Properties,
		Required:    required,
	}
}

func runTool(ctx context.Context, name, toolName string, input string) (tools.ToolResponse, error) {
	toolRequest := mcp.CallToolRequest{}
	toolRequest.Params.Name = toolName
	var args map[string]any
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return tools.NewTextErrorResponse(fmt.Sprintf("error parsing parameters: %s", err)), nil
	}
	toolRequest.Params.Arguments = args
	c, ok := mcpClients.Get(name)
	if !ok {
		return tools.NewTextErrorResponse("mcp '" + name + "' not available"), nil
	}
	result, err := c.CallTool(ctx, toolRequest)
	if err != nil {
		return tools.NewTextErrorResponse(err.Error()), nil
	}

	output := ""
	for _, v := range result.Content {
		if v, ok := v.(mcp.TextContent); ok {
			output = v.Text
		} else {
			output = fmt.Sprintf("%v", v)
		}
	}

	return tools.NewTextResponse(output), nil
}

func (b *mcpTool) Run(ctx context.Context, params tools.ToolCall) (tools.ToolResponse, error) {
	sessionID, messageID := tools.GetContextValues(ctx)
	if sessionID == "" || messageID == "" {
		return tools.ToolResponse{}, fmt.Errorf("session ID and message ID are required for creating a new file")
	}
	permissionDescription := fmt.Sprintf("execute %s with the following parameters: %s", b.Info().Name, params.Input)
	p := b.permissions.Request(
		permission.CreatePermissionRequest{
			SessionID:   sessionID,
			ToolCallID:  params.ID,
			Path:        b.workingDir,
			ToolName:    b.Info().Name,
			Action:      "execute",
			Description: permissionDescription,
			Params:      params.Input,
		},
	)
	if !p {
		return tools.ToolResponse{}, permission.ErrorPermissionDenied
	}

	return runTool(ctx, b.mcpName, b.tool.Name, params.Input)
}

func NewMcpTool(name string, tool mcp.Tool, permissions permission.Service, workingDir string) tools.BaseTool {
	return &mcpTool{
		mcpName:     name,
		tool:        tool,
		permissions: permissions,
		workingDir:  workingDir,
	}
}

func getTools(ctx context.Context, name string, permissions permission.Service, c *client.Client, workingDir string) []tools.BaseTool {
	var mcpTools []tools.BaseTool
	toolsRequest := mcp.ListToolsRequest{}
	tools, err := c.ListTools(ctx, toolsRequest)
	if err != nil {
		slog.Error("error listing tools", "error", err)
		c.Close()
		return mcpTools
	}
	for _, t := range tools.Tools {
		mcpTools = append(mcpTools, NewMcpTool(name, t, permissions, workingDir))
	}
	return mcpTools
}

func GetMCPTools(ctx context.Context, permissions permission.Service, cfg *config.Config) []tools.BaseTool {
	mcpToolsOnce.Do(func() {
		mcpTools = doGetMCPTools(ctx, permissions, cfg)
	})
	return mcpTools
}

// CloseMCPClients closes all MCP clients. This should be called during application shutdown.
func CloseMCPClients() {
	for c := range mcpClients.Seq() {
		_ = c.Close()
	}
}

func doGetMCPTools(ctx context.Context, permissions permission.Service, cfg *config.Config) []tools.BaseTool {
	var wg sync.WaitGroup
	result := csync.NewSlice[tools.BaseTool]()
	for name, m := range cfg.MCP {
		if m.Disabled {
			slog.Debug("skipping disabled mcp", "name", name)
			continue
		}
		wg.Add(1)
		go func(name string, m config.MCPConfig) {
			defer wg.Done()
			c, err := doGetClient(m)
			if err != nil {
				slog.Error("error creating mcp client", "error", err)
				return
			}
			if err := doInitClient(ctx, name, c); err != nil {
				slog.Error("error initializing mcp client", "error", err)
				return
			}
			result.Append(getTools(ctx, name, permissions, c, cfg.WorkingDir())...)
		}(name, m)
	}
	wg.Wait()
	return slices.Collect(result.Seq())
}

func doInitClient(ctx context.Context, name string, c *client.Client) error {
	initRequest := mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcp.Implementation{
				Name:    "Crush",
				Version: version.Version,
			},
		},
	}
	if _, err := c.Initialize(ctx, initRequest); err != nil {
		c.Close()
		return err
	}
	mcpClients.Set(name, c)
	return nil
}

func doGetClient(m config.MCPConfig) (*client.Client, error) {
	switch m.Type {
	case config.MCPStdio:
		return client.NewStdioMCPClient(
			m.Command,
			m.ResolvedEnv(),
			m.Args...,
		)
	case config.MCPHttp:
		return client.NewStreamableHttpClient(
			m.URL,
			transport.WithHTTPHeaders(m.ResolvedHeaders()),
		)
	case config.MCPSse:
		return client.NewSSEMCPClient(
			m.URL,
			client.WithHeaders(m.ResolvedHeaders()),
		)
	default:
		return nil, fmt.Errorf("unsupported mcp type: %s", m.Type)
	}
}
