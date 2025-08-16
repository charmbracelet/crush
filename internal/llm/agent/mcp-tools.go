package agent

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
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
	MCPEventStateChanged MCPEventType = "state_changed"
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
	mcpToolsOnce sync.Once
	mcpTools     []tools.BaseTool
	mcpClients   = csync.NewMap[string, *client.Client]()
	mcpStates    = csync.NewMap[string, MCPClientInfo]()
	mcpBroker    = pubsub.NewBroker[MCPEvent]()
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
	return tools.ToolInfo{
		Name:        fmt.Sprintf("mcp_%s_%s", b.mcpName, b.tool.Name),
		Description: b.tool.Description,
		Parameters:  b.tool.InputSchema.Properties,
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

	m := config.Get().MCP[name]
	state, _ := mcpStates.Get(name)

	pingCtx, cancel := context.WithTimeout(ctx, mcpTimeout(m))
	defer cancel()
	err := c.Ping(pingCtx)
	if err == nil {
		return c, nil
	}
	updateMCPState(name, MCPStateError, err, nil, state.ToolCount)

	c, err = createAndInitializeClient(ctx, name, m)
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

func getTools(ctx context.Context, name string, permissions permission.Service, c *client.Client, workingDir string) []tools.BaseTool {
	result, err := c.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		slog.Error("error listing tools", "error", err)
		updateMCPState(name, MCPStateError, err, nil, 0)
		c.Close()
		mcpClients.Del(name)
		return nil
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
	return mcpTools
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
	if state == MCPStateConnected {
		info.ConnectedAt = time.Now()
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

// RestartMCP restarts a specific MCP client by name
func RestartMCP(ctx context.Context, name string) error {
	// Get the configuration for this MCP
	cfg := config.Get()
	mcpConfig, exists := cfg.MCP[name]
	if !exists {
		return fmt.Errorf("MCP '%s' not found in configuration", name)
	}

	// Close existing client if it exists
	if client, exists := mcpClients.Get(name); exists {
		_ = client.Close()
		mcpClients.Del(name)
	}

	// Set state to starting
	updateMCPState(name, MCPStateStarting, nil, nil, 0)

	// Create and initialize new client
	client, err := createAndInitializeClient(ctx, name, mcpConfig)
	if err != nil {
		return fmt.Errorf("failed to restart MCP '%s': %w", name, err)
	}

	// Store the new client
	mcpClients.Set(name, client)

	// Get tools and update state
	tools := getTools(ctx, name, nil, client, cfg.WorkingDir())
	updateMCPState(name, MCPStateConnected, nil, client, len(tools))

	slog.Info("Successfully restarted MCP", "name", name, "tool_count", len(tools))
	return nil
}

// CloseMCPClients closes all MCP clients. This should be called during application shutdown.
func CloseMCPClients() {
	for c := range mcpClients.Seq() {
		_ = c.Close()
	}
	mcpBroker.Shutdown()
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

func doGetMCPTools(ctx context.Context, permissions permission.Service, cfg *config.Config) []tools.BaseTool {
	var wg sync.WaitGroup
	result := csync.NewSlice[tools.BaseTool]()

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
			c, err := createAndInitializeClient(ctx, name, m)
			if err != nil {
				return
			}
			mcpClients.Set(name, c)

			tools := getTools(ctx, name, permissions, c, cfg.WorkingDir())
			updateMCPState(name, MCPStateConnected, nil, c, len(tools))
			result.Append(tools...)
		}(name, m)
	}
	wg.Wait()
	return slices.Collect(result.Seq())
}

func createAndInitializeClient(ctx context.Context, name string, m config.MCPConfig) (*client.Client, error) {
	c, err := createMcpClient(m)
	if err != nil {
		updateMCPState(name, MCPStateError, err, nil, 0)
		slog.Error("error creating mcp client", "error", err, "name", name)
		return nil, err
	}
	// Only call Start() for non-stdio clients, as stdio clients auto-start
	if m.Type != config.MCPStdio {
		if err := c.Start(ctx); err != nil {
			updateMCPState(name, MCPStateError, err, nil, 0)
			slog.Error("error starting mcp client", "error", err, "name", name)
			_ = c.Close()
			return nil, err
		}
	}
	if _, err := c.Initialize(ctx, mcpInitRequest); err != nil {
		updateMCPState(name, MCPStateError, err, nil, 0)
		slog.Error("error initializing mcp client", "error", err, "name", name)
		_ = c.Close()
		return nil, err
	}

	slog.Info("Initialized mcp client", "name", name)
	return c, nil
}

func createMcpClient(m config.MCPConfig) (*client.Client, error) {
	switch m.Type {
	case config.MCPStdio:
		return client.NewStdioMCPClientWithOptions(
			m.Command,
			m.ResolvedEnv(),
			m.Args,
			transport.WithCommandLogger(mcpLogger{}),
		)
	case config.MCPHttp:
		return client.NewStreamableHttpClient(
			m.URL,
			transport.WithHTTPHeaders(m.ResolvedHeaders()),
			transport.WithHTTPLogger(mcpLogger{}),
		)
	case config.MCPSse:
		return client.NewSSEMCPClient(
			m.URL,
			client.WithHeaders(m.ResolvedHeaders()),
			transport.WithSSELogger(mcpLogger{}),
		)
	default:
		return nil, fmt.Errorf("unsupported mcp type: %s", m.Type)
	}
}

// for MCP's clients.
type mcpLogger struct{}

func (l mcpLogger) Errorf(format string, v ...any) { slog.Error(fmt.Sprintf(format, v...)) }
func (l mcpLogger) Infof(format string, v ...any)  { slog.Info(fmt.Sprintf(format, v...)) }

func mcpTimeout(m config.MCPConfig) time.Duration {
	return time.Duration(cmp.Or(m.Timeout, 15)) * time.Second
}
