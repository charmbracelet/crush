package agent

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/home"
	"github.com/charmbracelet/crush/internal/llm/tools"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/version"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

// MCPState represents the current state of an MCP client
type MCPState int

const (
	MCPStateDisabled MCPState = iota
	MCPStateStarting
	MCPStateConnected
	MCPStateError
)

func (s MCPState) String() string {
	switch s {
	case MCPStateDisabled:
		return "disabled"
	case MCPStateStarting:
		return "starting"
	case MCPStateConnected:
		return "connected"
	case MCPStateError:
		return "error"
	default:
		return "unknown"
	}
}

// MCPEventType represents the type of MCP event
type MCPEventType string

const (
	MCPEventStateChanged     MCPEventType = "state_changed"
	MCPEventToolsListChanged MCPEventType = "tools_list_changed"
)

// MCPEvent represents an event in the MCP system
type MCPEvent struct {
	Type      MCPEventType
	Name      string
	State     MCPState
	Error     error
	ToolCount int
}

// MCPClientInfo holds information about an MCP client's state
type MCPClientInfo struct {
	Name        string
	State       MCPState
	Error       error
	Client      *client.Client
	ToolCount   int
	ConnectedAt time.Time
}

var (
	mcpToolsOnce    sync.Once
	mcpTools        = csync.NewMap[string, tools.BaseTool]()
	mcpClient2Tools = csync.NewMap[string, []tools.BaseTool]()
	mcpClients      = csync.NewMap[string, *client.Client]()
	mcpStates       = csync.NewMap[string, MCPClientInfo]()
	mcpBroker       = pubsub.NewBroker[MCPEvent]()
)

type McpTool struct {
	mcpName     string
	tool        mcp.Tool
	permissions permission.Service
	workingDir  string
}

func (b *McpTool) Name() string {
	return fmt.Sprintf("mcp_%s_%s", b.mcpName, b.tool.Name)
}

func (b *McpTool) Info() tools.ToolInfo {
	required := b.tool.InputSchema.Required
	if required == nil {
		required = make([]string, 0)
	}
	parameters := b.tool.InputSchema.Properties
	if parameters == nil {
		parameters = make(map[string]any)
	}
	return tools.ToolInfo{
		Name:        fmt.Sprintf("mcp_%s_%s", b.mcpName, b.tool.Name),
		Description: b.tool.Description,
		Parameters:  parameters,
		Required:    required,
	}
}

func runTool(ctx context.Context, name, toolName string, input string) (tools.ToolResponse, error) {
	var args map[string]any
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return tools.NewTextErrorResponse(fmt.Sprintf("error parsing parameters: %s", err)), nil
	}

	c, err := getOrRenewClient(ctx, name)
	if err != nil {
		return tools.NewTextErrorResponse(err.Error()), nil
	}
	result, err := c.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      toolName,
			Arguments: args,
		},
	})
	if err != nil {
		return tools.NewTextErrorResponse(err.Error()), nil
	}

	output := make([]string, 0, len(result.Content))
	for _, v := range result.Content {
		if v, ok := v.(mcp.TextContent); ok {
			output = append(output, v.Text)
		} else {
			output = append(output, fmt.Sprintf("%v", v))
		}
	}
	return tools.NewTextResponse(strings.Join(output, "\n")), nil
}

func getOrRenewClient(ctx context.Context, name string) (*client.Client, error) {
	c, ok := mcpClients.Get(name)
	if !ok {
		return nil, fmt.Errorf("mcp '%s' not available", name)
	}

	cfg := config.Get()
	m := cfg.MCP[name]
	state, _ := mcpStates.Get(name)

	timeout := mcpTimeout(m)
	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	err := c.Ping(pingCtx)
	if err == nil {
		return c, nil
	}
	updateMCPState(name, MCPStateError, maybeTimeoutErr(err, timeout), nil, state.ToolCount)

	c, err = createAndInitializeClient(ctx, name, m, cfg.Resolver())
	if err != nil {
		return nil, err
	}

	updateMCPState(name, MCPStateConnected, nil, c, state.ToolCount)
	mcpClients.Set(name, c)
	return c, nil
}

func (b *McpTool) Run(ctx context.Context, params tools.ToolCall) (tools.ToolResponse, error) {
	sessionID, messageID := tools.GetContextValues(ctx)
	if sessionID == "" || messageID == "" {
		return tools.ToolResponse{}, fmt.Errorf("session ID and message ID are required for creating a new file")
	}
	permissionDescription := fmt.Sprintf("execute %s with the following parameters:", b.Info().Name)
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

func getTools(ctx context.Context, name string, permissions permission.Service, c *client.Client, workingDir string) ([]tools.BaseTool, error) {
	result, err := c.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, err
	}
	mcpTools := make([]tools.BaseTool, 0, len(result.Tools))
	for _, tool := range result.Tools {
		mcpTools = append(mcpTools, &McpTool{
			mcpName:     name,
			tool:        tool,
			permissions: permissions,
			workingDir:  workingDir,
		})
	}
	return mcpTools, nil
}

// SubscribeMCPEvents returns a channel for MCP events
func SubscribeMCPEvents(ctx context.Context) <-chan pubsub.Event[MCPEvent] {
	return mcpBroker.Subscribe(ctx)
}

// GetMCPStates returns the current state of all MCP clients
func GetMCPStates() map[string]MCPClientInfo {
	return maps.Collect(mcpStates.Seq2())
}

// GetMCPState returns the state of a specific MCP client
func GetMCPState(name string) (MCPClientInfo, bool) {
	return mcpStates.Get(name)
}

// updateMCPState updates the state of an MCP client and publishes an event
func updateMCPState(name string, state MCPState, err error, client *client.Client, toolCount int) {
	info := MCPClientInfo{
		Name:      name,
		State:     state,
		Error:     err,
		Client:    client,
		ToolCount: toolCount,
	}
	switch state {
	case MCPStateConnected:
		info.ConnectedAt = time.Now()
	case MCPStateError:
		updateMcpTools(name, nil)
		mcpClients.Del(name)
	}
	mcpStates.Set(name, info)

	// Publish state change event
	mcpBroker.Publish(pubsub.UpdatedEvent, MCPEvent{
		Type:      MCPEventStateChanged,
		Name:      name,
		State:     state,
		Error:     err,
		ToolCount: toolCount,
	})
}

// publishMCPEventToolsListChanged publishes a tool list changed event
func publishMCPEventToolsListChanged(name string) {
	mcpBroker.Publish(pubsub.UpdatedEvent, MCPEvent{
		Type: MCPEventToolsListChanged,
		Name: name,
	})
}

// CloseMCPClients closes all MCP clients. This should be called during application shutdown.
func CloseMCPClients() error {
	var errs []error
	for name, c := range mcpClients.Seq2() {
		if err := c.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close mcp: %s: %w", name, err))
		}
	}
	mcpBroker.Shutdown()
	return errors.Join(errs...)
}

var mcpInitRequest = mcp.InitializeRequest{
	Params: mcp.InitializeParams{
		ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
		ClientInfo: mcp.Implementation{
			Name:    "Crush",
			Version: version.Version,
		},
	},
}

func doGetMCPTools(ctx context.Context, permissions permission.Service, cfg *config.Config) {
	var wg sync.WaitGroup
	// Initialize states for all configured MCPs
	for name, m := range cfg.MCP {
		if m.Disabled {
			updateMCPState(name, MCPStateDisabled, nil, nil, 0)
			slog.Debug("skipping disabled mcp", "name", name)
			continue
		}

		// Set initial starting state
		updateMCPState(name, MCPStateStarting, nil, nil, 0)

		wg.Add(1)
		go func(name string, m config.MCPConfig) {
			defer func() {
				wg.Done()
				if r := recover(); r != nil {
					var err error
					switch v := r.(type) {
					case error:
						err = v
					case string:
						err = fmt.Errorf("panic: %s", v)
					default:
						err = fmt.Errorf("panic: %v", v)
					}
					updateMCPState(name, MCPStateError, err, nil, 0)
					slog.Error("panic in mcp client initialization", "error", err, "name", name)
				}
			}()

			ctx, cancel := context.WithTimeout(ctx, mcpTimeout(m))
			defer cancel()

			c, err := createAndInitializeClient(ctx, name, m, cfg.Resolver())
			if err != nil {
				return
			}

			mcpClients.Set(name, c)

			tools, err := getTools(ctx, name, permissions, c, cfg.WorkingDir())
			if err != nil {
				slog.Error("error listing tools", "error", err)
				updateMCPState(name, MCPStateError, err, nil, 0)
				c.Close()
				return
			}

			updateMcpTools(name, tools)
			mcpClients.Set(name, c)
			updateMCPState(name, MCPStateConnected, nil, c, len(tools))
		}(name, m)
	}
	wg.Wait()
}

// updateMcpTools updates the global mcpTools and mcpClient2Tools maps
func updateMcpTools(mcpName string, tools []tools.BaseTool) {
	if len(tools) == 0 {
		mcpClient2Tools.Del(mcpName)
	} else {
		mcpClient2Tools.Set(mcpName, tools)
	}
	for _, tools := range mcpClient2Tools.Seq2() {
		for _, t := range tools {
			mcpTools.Set(t.Name(), t)
		}
	}
}

func createAndInitializeClient(ctx context.Context, name string, m config.MCPConfig, resolver config.VariableResolver) (*client.Client, error) {
	c, err := createMcpClient(name, m, resolver)
	if err != nil {
		updateMCPState(name, MCPStateError, err, nil, 0)
		slog.Error("error creating mcp client", "error", err, "name", name)
		return nil, err
	}

	c.OnNotification(func(n mcp.JSONRPCNotification) {
		slog.Debug("Received MCP notification", "name", name, "notification", n)
		switch n.Method {
		case "notifications/tools/list_changed":
			publishMCPEventToolsListChanged(name)
		default:
			slog.Debug("Unhandled MCP notification", "name", name, "method", n.Method)
		}
	})

	// XXX: ideally we should be able to use context.WithTimeout here, but,
	// the SSE MCP client will start failing once that context is canceled.
	timeout := mcpTimeout(m)
	mcpCtx, cancel := context.WithCancel(ctx)
	cancelTimer := time.AfterFunc(timeout, cancel)

	if err := c.Start(mcpCtx); err != nil {
		updateMCPState(name, MCPStateError, maybeTimeoutErr(err, timeout), nil, 0)
		slog.Error("error starting mcp client", "error", err, "name", name)
		_ = c.Close()
		cancel()
		return nil, err
	}

	if _, err := c.Initialize(mcpCtx, mcpInitRequest); err != nil {
		updateMCPState(name, MCPStateError, maybeTimeoutErr(err, timeout), nil, 0)
		slog.Error("error initializing mcp client", "error", err, "name", name)
		_ = c.Close()
		cancel()
		return nil, err
	}

	cancelTimer.Stop()
	slog.Info("Initialized mcp client", "name", name)
	return c, nil
}

func maybeTimeoutErr(err error, timeout time.Duration) error {
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("timed out after %s", timeout)
	}
	return err
}

func createMcpClient(name string, m config.MCPConfig, resolver config.VariableResolver) (*client.Client, error) {
	switch m.Type {
	case config.MCPStdio:
		command, err := resolver.ResolveValue(m.Command)
		if err != nil {
			return nil, fmt.Errorf("invalid mcp command: %w", err)
		}
		if strings.TrimSpace(command) == "" {
			return nil, fmt.Errorf("mcp stdio config requires a non-empty 'command' field")
		}
		return client.NewStdioMCPClientWithOptions(
			home.Long(command),
			m.ResolvedEnv(),
			m.Args,
			transport.WithCommandLogger(mcpLogger{name: name}),
		)
	case config.MCPHttp:
		if strings.TrimSpace(m.URL) == "" {
			return nil, fmt.Errorf("mcp http config requires a non-empty 'url' field")
		}
		return client.NewStreamableHttpClient(
			m.URL,
			transport.WithHTTPHeaders(m.ResolvedHeaders()),
			transport.WithHTTPLogger(mcpLogger{name: name}),
		)
	case config.MCPSse:
		if strings.TrimSpace(m.URL) == "" {
			return nil, fmt.Errorf("mcp sse config requires a non-empty 'url' field")
		}
		return client.NewSSEMCPClient(
			m.URL,
			client.WithHeaders(m.ResolvedHeaders()),
			transport.WithSSELogger(mcpLogger{name: name}),
		)
	default:
		return nil, fmt.Errorf("unsupported mcp type: %s", m.Type)
	}
}

// for MCP's clients.
type mcpLogger struct{ name string }

func (l mcpLogger) Errorf(format string, v ...any) {
	slog.Error(fmt.Sprintf(format, v...), "name", l.name)
}

func (l mcpLogger) Infof(format string, v ...any) {
	slog.Info(fmt.Sprintf(format, v...), "name", l.name)
}

func mcpTimeout(m config.MCPConfig) time.Duration {
	return time.Duration(cmp.Or(m.Timeout, 15)) * time.Second
}
