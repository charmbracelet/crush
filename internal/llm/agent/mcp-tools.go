package agent

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"os/exec"
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
	"github.com/modelcontextprotocol/go-sdk/mcp"
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
	MCPEventToolsListChanged   MCPEventType = "tools_list_changed"
	MCPEventPromptsListChanged MCPEventType = "prompts_list_changed"
)

// MCPEvent represents an event in the MCP system
type MCPEvent struct {
	Type   MCPEventType
	Name   string
	State  MCPState
	Error  error
	Counts MCPCounts
}

// MCPCounts number of available tools, prompts, etc.
type MCPCounts struct {
	Tools     int
	Prompts   int
	Resources int
}

// MCPClientInfo holds information about an MCP client's state
type MCPClientInfo struct {
	Name        string
	State       MCPState
	Error       error
	Client      *mcp.ClientSession
	Counts      MCPCounts
	ConnectedAt time.Time
}

var (
	mcpToolsOnce        sync.Once
	mcpTools            = csync.NewMap[string, tools.BaseTool]()
	mcpClient2Tools     = csync.NewMap[string, []tools.BaseTool]()
	mcpClients          = csync.NewMap[string, *mcp.ClientSession]()
	mcpStates           = csync.NewMap[string, MCPClientInfo]()
	mcpBroker           = pubsub.NewBroker[MCPEvent]()
	mcpPrompts          = csync.NewMap[string, *mcp.Prompt]()
	mcpClient2Prompts   = csync.NewMap[string, []*mcp.Prompt]()
	mcpResources        = csync.NewMap[string, *mcp.Resource]()
	mcpClient2Resources = csync.NewMap[string, []*mcp.Resource]()
)

type McpTool struct {
	mcpName     string
	tool        *mcp.Tool
	permissions permission.Service
	workingDir  string
}

func (b *McpTool) Name() string {
	return fmt.Sprintf("mcp_%s_%s", b.mcpName, b.tool.Name)
}

func (b *McpTool) Info() tools.ToolInfo {
	input := b.tool.InputSchema.(map[string]any)
	required, _ := input["required"].([]string)
	parameters, _ := input["properties"].(map[string]any)
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
	result, err := c.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	})
	if err != nil {
		return tools.NewTextErrorResponse(err.Error()), nil
	}

	output := make([]string, 0, len(result.Content))
	for _, v := range result.Content {
		if vv, ok := v.(*mcp.TextContent); ok {
			output = append(output, vv.Text)
		} else {
			output = append(output, fmt.Sprintf("%v", v))
		}
	}
	return tools.NewTextResponse(strings.Join(output, "\n")), nil
}

func getOrRenewClient(ctx context.Context, name string) (*mcp.ClientSession, error) {
	sess, ok := mcpClients.Get(name)
	if !ok {
		return nil, fmt.Errorf("mcp '%s' not available", name)
	}

	cfg := config.Get()
	m := cfg.MCP[name]
	state, _ := mcpStates.Get(name)

	timeout := mcpTimeout(m)
	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	err := sess.Ping(pingCtx, nil)
	if err == nil {
		return sess, nil
	}
	updateMCPState(name, MCPStateError, maybeTimeoutErr(err, timeout), nil, state.Counts)

	sess, err = createMCPSession(ctx, name, m, cfg.Resolver())
	if err != nil {
		return nil, err
	}

	updateMCPState(name, MCPStateConnected, nil, sess, state.Counts)
	mcpClients.Set(name, sess)
	return sess, nil
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

func getTools(ctx context.Context, name string, permissions permission.Service, c *mcp.ClientSession, workingDir string) ([]tools.BaseTool, error) {
	if c.InitializeResult().Capabilities.Tools == nil {
		return nil, nil
	}
	result, err := c.ListTools(ctx, &mcp.ListToolsParams{})
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
func updateMCPState(name string, state MCPState, err error, client *mcp.ClientSession, counts MCPCounts) {
	info := MCPClientInfo{
		Name:   name,
		State:  state,
		Error:  err,
		Client: client,
		Counts: counts,
	}
	switch state {
	case MCPStateConnected:
		info.ConnectedAt = time.Now()
	case MCPStateError:
		updateMcpTools(name, nil)
		updateMcpPrompts(name, nil)
		mcpClients.Del(name)
	}
	mcpStates.Set(name, info)
}

// CloseMCPClients closes all MCP clients. This should be called during application shutdown.
func CloseMCPClients() error {
	var errs []error
	for name, c := range mcpClients.Seq2() {
		if err := c.Close(); err != nil &&
			!errors.Is(err, io.EOF) &&
			!errors.Is(err, context.Canceled) &&
			err.Error() != "signal: killed" {
			errs = append(errs, fmt.Errorf("close mcp: %s: %w", name, err))
		}
	}
	mcpBroker.Shutdown()
	return errors.Join(errs...)
}

func doGetMCPTools(ctx context.Context, permissions permission.Service, cfg *config.Config) {
	var wg sync.WaitGroup
	// Initialize states for all configured MCPs
	for name, m := range cfg.MCP {
		if m.Disabled {
			updateMCPState(name, MCPStateDisabled, nil, nil, MCPCounts{})
			slog.Debug("skipping disabled mcp", "name", name)
			continue
		}

		// Set initial starting state
		updateMCPState(name, MCPStateStarting, nil, nil, MCPCounts{})

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
					updateMCPState(name, MCPStateError, err, nil, MCPCounts{})
					slog.Error("panic in mcp client initialization", "error", err, "name", name)
				}
			}()

			ctx, cancel := context.WithTimeout(ctx, mcpTimeout(m))
			defer cancel()

			c, err := createMCPSession(ctx, name, m, cfg.Resolver())
			if err != nil {
				return
			}

			mcpClients.Set(name, c)

			tools, err := getTools(ctx, name, permissions, c, cfg.WorkingDir())
			if err != nil {
				slog.Error("error listing tools", "error", err)
				updateMCPState(name, MCPStateError, err, nil, MCPCounts{})
				c.Close()
				return
			}

			prompts, err := getPrompts(ctx, c)
			if err != nil {
				slog.Error("error listing prompts", "error", err)
				updateMCPState(name, MCPStateError, err, nil, MCPCounts{})
				c.Close()
				return
			}

			resources, err := getResources(ctx, c)
			if err != nil {
				slog.Error("error listing resources", "error", err)
				updateMCPState(name, MCPStateError, err, nil, MCPCounts{})
				c.Close()
				return
			}

			updateMcpTools(name, tools)
			updateMcpPrompts(name, prompts)
			updateMcpResources(name, resources)
			mcpClients.Set(name, c)
			counts := MCPCounts{
				Tools:     len(tools),
				Prompts:   len(prompts),
				Resources: len(resources),
			}
			updateMCPState(name, MCPStateConnected, nil, c, counts)
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

func createMCPSession(ctx context.Context, name string, m config.MCPConfig, resolver config.VariableResolver) (*mcp.ClientSession, error) {
	timeout := mcpTimeout(m)
	mcpCtx, cancel := context.WithCancel(ctx)
	cancelTimer := time.AfterFunc(timeout, cancel)

	transport, err := createMCPTransport(mcpCtx, m, resolver)
	if err != nil {
		updateMCPState(name, MCPStateError, err, nil, MCPCounts{})
		slog.Error("error creating mcp client", "error", err, "name", name)
		return nil, err
	}

	client := mcp.NewClient(
		&mcp.Implementation{
			Name:    "crush",
			Version: version.Version,
			Title:   "Crush",
		},
		&mcp.ClientOptions{
			ToolListChangedHandler: func(context.Context, *mcp.ToolListChangedRequest) {
				mcpBroker.Publish(pubsub.UpdatedEvent, MCPEvent{
					Type: MCPEventToolsListChanged,
					Name: name,
				})
			},
			PromptListChangedHandler: func(context.Context, *mcp.PromptListChangedRequest) {
				mcpBroker.Publish(pubsub.UpdatedEvent, MCPEvent{
					Type: MCPEventPromptsListChanged,
					Name: name,
				})
			},
			KeepAlive: time.Minute * 10,
		},
	)

	session, err := client.Connect(mcpCtx, transport, nil)
	if err != nil {
		updateMCPState(name, MCPStateError, maybeTimeoutErr(err, timeout), nil, MCPCounts{})
		slog.Error("error starting mcp client", "error", err, "name", name)
		cancel()
		return nil, err
	}

	cancelTimer.Stop()
	slog.Info("Initialized mcp client", "name", name)
	return session, nil
}

func maybeTimeoutErr(err error, timeout time.Duration) error {
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("timed out after %s", timeout)
	}
	return err
}

func createMCPTransport(ctx context.Context, m config.MCPConfig, resolver config.VariableResolver) (mcp.Transport, error) {
	switch m.Type {
	case config.MCPStdio:
		command, err := resolver.ResolveValue(m.Command)
		if err != nil {
			return nil, fmt.Errorf("invalid mcp command: %w", err)
		}
		if strings.TrimSpace(command) == "" {
			return nil, fmt.Errorf("mcp stdio config requires a non-empty 'command' field")
		}
		cmd := exec.CommandContext(ctx, home.Long(command), m.Args...)
		cmd.Env = m.ResolvedEnv()
		return &mcp.CommandTransport{
			Command: cmd,
		}, nil
	case config.MCPHttp:
		if strings.TrimSpace(m.URL) == "" {
			return nil, fmt.Errorf("mcp http config requires a non-empty 'url' field")
		}
		client := &http.Client{
			Transport: &headerRoundTripper{
				headers: m.ResolvedHeaders(),
			},
		}
		return &mcp.StreamableClientTransport{
			Endpoint:   m.URL,
			HTTPClient: client,
		}, nil
	case config.MCPSSE:
		if strings.TrimSpace(m.URL) == "" {
			return nil, fmt.Errorf("mcp sse config requires a non-empty 'url' field")
		}
		client := &http.Client{
			Transport: &headerRoundTripper{
				headers: m.ResolvedHeaders(),
			},
		}
		return &mcp.SSEClientTransport{
			Endpoint:   m.URL,
			HTTPClient: client,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported mcp type: %s", m.Type)
	}
}

type headerRoundTripper struct {
	headers map[string]string
}

func (rt headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range rt.headers {
		req.Header.Set(k, v)
	}
	return http.DefaultTransport.RoundTrip(req)
}

func mcpTimeout(m config.MCPConfig) time.Duration {
	return time.Duration(cmp.Or(m.Timeout, 15)) * time.Second
}

func getPrompts(ctx context.Context, c *mcp.ClientSession) ([]*mcp.Prompt, error) {
	if c.InitializeResult().Capabilities.Prompts == nil {
		return nil, nil
	}
	result, err := c.ListPrompts(ctx, &mcp.ListPromptsParams{})
	if err != nil {
		return nil, err
	}
	return result.Prompts, nil
}

// updateMcpPrompts updates the global mcpPrompts and mcpClient2Prompts maps.
func updateMcpPrompts(mcpName string, prompts []*mcp.Prompt) {
	if len(prompts) == 0 {
		mcpClient2Prompts.Del(mcpName)
	} else {
		mcpClient2Prompts.Set(mcpName, prompts)
	}
	for clientName, prompts := range mcpClient2Prompts.Seq2() {
		for _, p := range prompts {
			key := clientName + ":" + p.Name
			mcpPrompts.Set(key, p)
		}
	}
}

func getResources(ctx context.Context, c *mcp.ClientSession) ([]*mcp.Resource, error) {
	if c.InitializeResult().Capabilities.Resources == nil {
		return nil, nil
	}
	result, err := c.ListResources(ctx, &mcp.ListResourcesParams{})
	if err != nil {
		return nil, err
	}
	return result.Resources, nil
}

// updateMcpResources updates the global mcpResources and mcpClient2Resources maps.
func updateMcpResources(mcpName string, resources []*mcp.Resource) {
	if len(resources) == 0 {
		mcpClient2Resources.Del(mcpName)
	} else {
		mcpClient2Resources.Set(mcpName, resources)
	}
	for clientName, resources := range mcpClient2Resources.Seq2() {
		for _, p := range resources {
			key := clientName + ":" + p.Name
			mcpResources.Set(key, p)
		}
	}
}

// GetMCPPrompts returns all available MCP prompts.
func GetMCPPrompts() map[string]*mcp.Prompt {
	return maps.Collect(mcpPrompts.Seq2())
}

// GetMCPPrompt returns a specific MCP prompt by name.
func GetMCPPrompt(name string) (*mcp.Prompt, bool) {
	return mcpPrompts.Get(name)
}

// GetMCPPromptsByClient returns all prompts for a specific MCP client.
func GetMCPPromptsByClient(clientName string) ([]*mcp.Prompt, bool) {
	return mcpClient2Prompts.Get(clientName)
}

// GetMCPPromptContent retrieves the content of an MCP prompt with the given arguments.
func GetMCPPromptContent(ctx context.Context, clientName, promptName string, args map[string]string) (*mcp.GetPromptResult, error) {
	c, err := getOrRenewClient(ctx, clientName)
	if err != nil {
		return nil, err
	}

	return c.GetPrompt(ctx, &mcp.GetPromptParams{
		Name:      promptName,
		Arguments: args,
	})
}

// GetMCPResources returns all available MCP resources.
func GetMCPResources() map[string]*mcp.Resource {
	return maps.Collect(mcpResources.Seq2())
}

// GetMCPResource returns a specific MCP resource by name.
func GetMCPResource(name string) (*mcp.Resource, bool) {
	return mcpResources.Get(name)
}

// GetMCPResourcesByClient returns all resources for a specific MCP client.
func GetMCPResourcesByClient(clientName string) ([]*mcp.Resource, bool) {
	return mcpClient2Resources.Get(clientName)
}

// GetMCPResourceContent retrieves the content of an MCP resource.
func GetMCPResourceContent(ctx context.Context, clientName, uri string) (*mcp.ReadResourceResult, error) {
	c, err := getOrRenewClient(ctx, clientName)
	if err != nil {
		return nil, err
	}

	return c.ReadResource(ctx, &mcp.ReadResourceParams{
		URI: uri,
	})
}
