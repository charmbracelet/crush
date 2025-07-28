package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"sync"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/llm/tools"
	"github.com/charmbracelet/crush/internal/resolver"
	"github.com/charmbracelet/crush/internal/version"

	"github.com/charmbracelet/crush/internal/permission"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

type MCPType string

const (
	MCPStdio MCPType = "stdio"
	MCPSse   MCPType = "sse"
	MCPHttp  MCPType = "http"
)

type MCPConfig struct {
	Command  string            `json:"command,omitempty" `
	Env      map[string]string `json:"env,omitempty"`
	Args     []string          `json:"args,omitempty"`
	Type     MCPType           `json:"type"`
	URL      string            `json:"url,omitempty"`
	Disabled bool              `json:"disabled,omitempty"`

	Headers map[string]string `json:"headers,omitempty"`
}

type mcpTool struct {
	mcpName     string
	tool        mcp.Tool
	mcpConfig   MCPConfig
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

func runTool(ctx context.Context, c MCPClient, toolName string, input string) (tools.ToolResponse, error) {
	defer c.Close()
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "crush",
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

	switch b.mcpConfig.Type {
	case MCPStdio:
		c, err := client.NewStdioMCPClient(
			b.mcpConfig.Command,
			b.mcpConfig.ResolvedEnv(),
			b.mcpConfig.Args...,
		)
		if err != nil {
			return tools.NewTextErrorResponse(err.Error()), nil
		}
		return runTool(ctx, c, b.tool.Name, params.Input)
	case MCPHttp:
		c, err := client.NewStreamableHttpClient(
			b.mcpConfig.URL,
			transport.WithHTTPHeaders(b.mcpConfig.ResolvedHeaders()),
		)
		if err != nil {
			return tools.NewTextErrorResponse(err.Error()), nil
		}
		return runTool(ctx, c, b.tool.Name, params.Input)
	case MCPSse:
		c, err := client.NewSSEMCPClient(
			b.mcpConfig.URL,
			client.WithHeaders(b.mcpConfig.ResolvedHeaders()),
		)
		if err != nil {
			return tools.NewTextErrorResponse(err.Error()), nil
		}
		return runTool(ctx, c, b.tool.Name, params.Input)
	}

	return tools.NewTextErrorResponse("invalid mcp type"), nil
}

func NewMcpTool(name, cwd string, tool mcp.Tool, permissions permission.Service, mcpConfig MCPConfig) tools.BaseTool {
	return &mcpTool{
		mcpName:     name,
		tool:        tool,
		mcpConfig:   mcpConfig,
		permissions: permissions,
		workingDir:  cwd,
	}
}

func getTools(ctx context.Context, cwd string, name string, m MCPConfig, permissions permission.Service, c MCPClient) []tools.BaseTool {
	var stdioTools []tools.BaseTool
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "crush",
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
		stdioTools = append(stdioTools, NewMcpTool(name, cwd, t, permissions, m))
	}
	defer c.Close()
	return stdioTools
}

var (
	mcpToolsOnce sync.Once
	mcpTools     []tools.BaseTool
)

func GetMCPTools(ctx context.Context, cwd string, mcps map[string]MCPConfig, permissions permission.Service) []tools.BaseTool {
	mcpToolsOnce.Do(func() {
		mcpTools = doGetMCPTools(ctx, cwd, mcps, permissions)
	})
	return mcpTools
}

func doGetMCPTools(ctx context.Context, cwd string, mcps map[string]MCPConfig, permissions permission.Service) []tools.BaseTool {
	var wg sync.WaitGroup
	result := csync.NewSlice[tools.BaseTool]()
	for name, m := range mcps {
		if m.Disabled {
			slog.Debug("skipping disabled mcp", "name", name)
			continue
		}
		wg.Add(1)
		go func(name string, m MCPConfig) {
			defer wg.Done()
			switch m.Type {
			case MCPStdio:
				c, err := client.NewStdioMCPClient(
					m.Command,
					m.ResolvedEnv(),
					m.Args...,
				)
				if err != nil {
					slog.Error("error creating mcp client", "error", err)
					return
				}

				result.Append(getTools(ctx, cwd, name, m, permissions, c)...)
			case MCPHttp:
				c, err := client.NewStreamableHttpClient(
					m.URL,
					transport.WithHTTPHeaders(m.ResolvedHeaders()),
				)
				if err != nil {
					slog.Error("error creating mcp client", "error", err)
					return
				}
				result.Append(getTools(ctx, cwd, name, m, permissions, c)...)
			case MCPSse:
				c, err := client.NewSSEMCPClient(
					m.URL,
					client.WithHeaders(m.ResolvedHeaders()),
				)
				if err != nil {
					slog.Error("error creating mcp client", "error", err)
					return
				}
				result.Append(getTools(ctx, cwd, name, m, permissions, c)...)
			}
		}(name, m)
	}
	wg.Wait()
	return slices.Collect(result.Seq())
}

func (m MCPConfig) ResolvedEnv() []string {
	resolver := resolver.New()
	for e, v := range m.Env {
		var err error
		m.Env[e], err = resolver.ResolveValue(v)
		if err != nil {
			slog.Error("error resolving environment variable", "error", err, "variable", e, "value", v)
			continue
		}
	}

	env := make([]string, 0, len(m.Env))
	for k, v := range m.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	return env
}

func (m MCPConfig) ResolvedHeaders() map[string]string {
	resolver := resolver.New()
	for e, v := range m.Headers {
		var err error
		m.Headers[e], err = resolver.ResolveValue(v)
		if err != nil {
			slog.Error("error resolving header variable", "error", err, "variable", e, "value", v)
			continue
		}
	}
	return m.Headers
}
