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
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/bwl/cliffy/internal/config"
	"github.com/bwl/cliffy/internal/csync"
	"github.com/bwl/cliffy/internal/home"
	"github.com/bwl/cliffy/internal/llm/tools"
	"github.com/bwl/cliffy/internal/version"
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

// MCPEvent represents an event in the MCP system
type MCPEvent struct {
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
	Client      *mcp.ClientSession
	ToolCount   int
	ConnectedAt time.Time
}

var (
	mcpToolsOnce sync.Once
	mcpTools     []tools.BaseTool
	mcpClients   = csync.NewMap[string, *mcp.ClientSession]()
	mcpStates    = csync.NewMap[string, MCPClientInfo]()
)

type McpTool struct {
	mcpName    string
	tool       *mcp.Tool
	workingDir string
}

func (b *McpTool) Name() string {
	return fmt.Sprintf("mcp_%s_%s", b.mcpName, b.tool.Name)
}

func (b *McpTool) Info() tools.ToolInfo {
	var parameters map[string]any
	var required []string

	// Safely extract input schema with type assertions
	if input, ok := b.tool.InputSchema.(map[string]any); ok {
		if props, ok := input["properties"].(map[string]any); ok {
			parameters = props
		}
		// Handle both []any and []string for required field
		if req, ok := input["required"].([]any); ok {
			for _, v := range req {
				if s, ok := v.(string); ok {
					required = append(required, s)
				}
			}
		} else if reqStr, ok := input["required"].([]string); ok {
			required = reqStr
		}
	}

	if parameters == nil {
		parameters = make(map[string]any)
	}
	if required == nil {
		required = make([]string, 0)
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
	updateMCPState(name, MCPStateError, maybeTimeoutErr(err, timeout), nil, state.ToolCount)

	sess, err = createMCPSession(ctx, name, m, cfg.Resolver())
	if err != nil {
		return nil, err
	}

	updateMCPState(name, MCPStateConnected, nil, sess, state.ToolCount)
	mcpClients.Set(name, sess)
	return sess, nil
}

func (b *McpTool) Run(ctx context.Context, params tools.ToolCall) (tools.ToolResponse, error) {
	return runTool(ctx, b.mcpName, b.tool.Name, params.Input)
}

func getTools(ctx context.Context, name string, c *mcp.ClientSession, workingDir string) ([]tools.BaseTool, error) {
	result, err := c.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return nil, err
	}
	mcpTools := make([]tools.BaseTool, 0, len(result.Tools))
	for _, tool := range result.Tools {
		mcpTools = append(mcpTools, &McpTool{
			mcpName:    name,
			tool:       tool,
			workingDir: workingDir,
		})
	}
	return mcpTools, nil
}

// GetMCPStates returns the current state of all MCP clients
func GetMCPStates() map[string]MCPClientInfo {
	return maps.Collect(mcpStates.Seq2())
}

// GetMCPState returns the state of a specific MCP client
func GetMCPState(name string) (MCPClientInfo, bool) {
	return mcpStates.Get(name)
}

// updateMCPState updates the state of an MCP client
func updateMCPState(name string, state MCPState, err error, client *mcp.ClientSession, toolCount int) {
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
		mcpClients.Del(name)
	}
	mcpStates.Set(name, info)
	slog.Debug("MCP state changed", "name", name, "state", state.String(), "toolCount", toolCount)
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
	return errors.Join(errs...)
}

func doGetMCPTools(ctx context.Context, cfg *config.Config) []tools.BaseTool {
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

			c, err := createMCPSession(ctx, name, m, cfg.Resolver())
			if err != nil {
				return
			}

			tools, err := getTools(ctx, name, c, cfg.WorkingDir())
			if err != nil {
				slog.Error("error listing tools", "error", err)
				updateMCPState(name, MCPStateError, err, nil, 0)
				c.Close()
				return
			}

			mcpClients.Set(name, c)
			updateMCPState(name, MCPStateConnected, nil, c, len(tools))
			result.Append(tools...)
		}(name, m)
	}
	wg.Wait()
	return slices.Collect(result.Seq())
}

func createMCPSession(ctx context.Context, name string, m config.MCPConfig, resolver config.VariableResolver) (*mcp.ClientSession, error) {
	timeout := mcpTimeout(m)
	mcpCtx, cancel := context.WithCancel(ctx)
	cancelTimer := time.AfterFunc(timeout, cancel)

	transport, err := createMCPTransport(mcpCtx, m, resolver)
	if err != nil {
		updateMCPState(name, MCPStateError, err, nil, 0)
		slog.Error("error creating mcp client", "error", err, "name", name)
		cancel()
		return nil, err
	}

	client := mcp.NewClient(
		&mcp.Implementation{
			Name:    "Cliffy",
			Version: version.Version,
			Title:   "Cliffy",
		},
		&mcp.ClientOptions{
			KeepAlive: time.Minute * 10,
		},
	)

	session, err := client.Connect(mcpCtx, transport, nil)
	if err != nil {
		updateMCPState(name, MCPStateError, maybeTimeoutErr(err, timeout), nil, 0)
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
	case config.MCPSse:
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
