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

	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/version"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

type McpTool struct {
	mcpName     string
	tool        mcp.Tool
	client      MCPClient
	mcpConfig   config.MCPConfig
	permissions permission.Service
	workingDir  string
}

type MCPClient interface {
	Initialize(
		ctx context.Context,
		request mcp.InitializeRequest,
	) (*mcp.InitializeResult, error)
	ListTools(ctx context.Context, request mcp.ListToolsRequest) (*mcp.ListToolsResult, error)
	CallTool(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)
	Close() error
}

func (b *McpTool) Name() string {
	return fmt.Sprintf("mcp_%s_%s", b.mcpName, b.tool.Name)
}

func (b *McpTool) Info() tools.ToolInfo {
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

func runTool(ctx context.Context, c MCPClient, toolName string, input string) (tools.ToolResponse, error) {
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "Crush",
		Version: version.Version,
	}

	_, err := c.Initialize(ctx, initRequest)
	if err != nil {
		return tools.NewTextErrorResponse(err.Error()), nil
	}

	toolRequest := mcp.CallToolRequest{}
	toolRequest.Params.Name = toolName
	var args map[string]any
	if err = json.Unmarshal([]byte(input), &args); err != nil {
		return tools.NewTextErrorResponse(fmt.Sprintf("error parsing parameters: %s", err)), nil
	}
	toolRequest.Params.Arguments = args
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

func (b *McpTool) Run(ctx context.Context, params tools.ToolCall) (tools.ToolResponse, error) {
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

	return runTool(ctx, b.client, b.tool.Name, params.Input)
}

func NewMcpTool(name string, c MCPClient, tool mcp.Tool, permissions permission.Service, mcpConfig config.MCPConfig, workingDir string) tools.BaseTool {
	return &McpTool{
		mcpName:     name,
		client:      c,
		tool:        tool,
		mcpConfig:   mcpConfig,
		permissions: permissions,
		workingDir:  workingDir,
	}
}

func getTools(ctx context.Context, name string, m config.MCPConfig, permissions permission.Service, c MCPClient, workingDir string) []tools.BaseTool {
	var stdioTools []tools.BaseTool
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "Crush",
		Version: version.Version,
	}

	_, err := c.Initialize(ctx, initRequest)
	if err != nil {
		slog.Error("error initializing mcp client", "error", err)
		return stdioTools
	}
	toolsRequest := mcp.ListToolsRequest{}
	tools, err := c.ListTools(ctx, toolsRequest)
	if err != nil {
		slog.Error("error listing tools", "error", err)
		return stdioTools
	}
	for _, t := range tools.Tools {
		stdioTools = append(stdioTools, NewMcpTool(name, c, t, permissions, m, workingDir))
	}
	return stdioTools
}

var (
	mcpToolsOnce sync.Once
	mcpTools     []tools.BaseTool
)

func GetMCPTools(ctx context.Context, permissions permission.Service, cfg *config.Config) []tools.BaseTool {
	mcpToolsOnce.Do(func() {
		mcpTools = doGetMCPTools(ctx, permissions, cfg)
	})
	return mcpTools
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
			switch m.Type {
			case config.MCPStdio:
				c, err := client.NewStdioMCPClient(
					m.Command,
					m.ResolvedEnv(),
					m.Args...,
				)
				if err != nil {
					slog.Error("error creating mcp client", "error", err)
					return
				}

				result.Append(getTools(ctx, name, m, permissions, c, cfg.WorkingDir())...)
			case config.MCPHttp:
				c, err := client.NewStreamableHttpClient(
					m.URL,
					transport.WithHTTPHeaders(m.ResolvedHeaders()),
				)
				if err != nil {
					slog.Error("error creating mcp client", "error", err)
					return
				}
				result.Append(getTools(ctx, name, m, permissions, c, cfg.WorkingDir())...)
			case config.MCPSse:
				c, err := client.NewSSEMCPClient(
					m.URL,
					client.WithHeaders(m.ResolvedHeaders()),
				)
				if err != nil {
					slog.Error("error creating mcp client", "error", err)
					return
				}
				result.Append(getTools(ctx, name, m, permissions, c, cfg.WorkingDir())...)
			}
		}(name, m)
	}
	wg.Wait()
	return slices.Collect(result.Seq())
}
