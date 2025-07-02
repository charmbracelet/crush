package client

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"

	"github.com/mark3labs/mcp-go/mcp"
)

// StdioMCPClient implements the MCPClient interface using stdio communication.
// It launches a subprocess and communicates with it via standard input/output streams
// using JSON-RPC messages. The client handles message routing between requests and
// responses, and supports asynchronous notifications.
type StdioMCPClient struct {
	cmd           *exec.Cmd
	stdin         io.WriteCloser
	stdout        *bufio.Reader
	requestID     atomic.Int64
	responses     map[int64]chan RPCResponse
	mu            sync.RWMutex
	done          chan struct{}
	initialized   bool
	notifications []func(mcp.JSONRPCNotification)
	notifyMu      sync.RWMutex
	capabilities  mcp.ServerCapabilities
}

// NewStdioMCPClient creates a new stdio-based MCP client that communicates with a subprocess.
// It launches the specified command with given arguments and sets up stdin/stdout pipes for communication.
// Returns an error if the subprocess cannot be started or the pipes cannot be created.
func NewStdioMCPClient(
	command string,
	env []string,
	args ...string,
) (*StdioMCPClient, error) {
	cmd := exec.Command(command, args...)

	mergedEnv := os.Environ()
	mergedEnv = append(mergedEnv, env...)

	cmd.Env = mergedEnv

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	client := &StdioMCPClient{
		cmd:       cmd,
		stdin:     stdin,
		stdout:    bufio.NewReader(stdout),
		responses: make(map[int64]chan RPCResponse),
		done:      make(chan struct{}),
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	// Start reading responses in a goroutine and wait for it to be ready
	ready := make(chan struct{})
	go func() {
		close(ready)
		client.readResponses()
	}()
	<-ready

	return client, nil
}

// Close shuts down the stdio client, closing the stdin pipe and waiting for the subprocess to exit.
// Returns an error if there are issues closing stdin or waiting for the subprocess to terminate.
func (c *StdioMCPClient) Close() error {
	close(c.done)
	if err := c.stdin.Close(); err != nil {
		return fmt.Errorf("failed to close stdin: %w", err)
	}
	return c.cmd.Wait()
}

// OnNotification registers a handler function to be called when notifications are received.
// Multiple handlers can be registered and will be called in the order they were added.
func (c *StdioMCPClient) OnNotification(
	handler func(notification mcp.JSONRPCNotification),
) {
	c.notifyMu.Lock()
	defer c.notifyMu.Unlock()
	c.notifications = append(c.notifications, handler)
}

// readResponses continuously reads and processes responses from the server's stdout.
// It handles both responses to requests and notifications, routing them appropriately.
// Runs until the done channel is closed or an error occurs reading from stdout.
func (c *StdioMCPClient) readResponses() {
	for {
		select {
		case <-c.done:
			return
		default:
			line, err := c.stdout.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					fmt.Printf("Error reading response: %v\n", err)
				}
				return
			}

			var baseMessage struct {
				JSONRPC string          `json:"jsonrpc"`
				ID      *int64          `json:"id,omitempty"`
				Method  string          `json:"method,omitempty"`
				Result  json.RawMessage `json:"result,omitempty"`
				Error   *struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				} `json:"error,omitempty"`
			}

			if err := json.Unmarshal([]byte(line), &baseMessage); err != nil {
				continue
			}

			// Handle notification
			if baseMessage.ID == nil {
				var notification mcp.JSONRPCNotification
				if err := json.Unmarshal([]byte(line), &notification); err != nil {
					continue
				}
				c.notifyMu.RLock()
				for _, handler := range c.notifications {
					handler(notification)
				}
				c.notifyMu.RUnlock()
				continue
			}

			c.mu.RLock()
			ch, ok := c.responses[*baseMessage.ID]
			c.mu.RUnlock()

			if ok {
				if baseMessage.Error != nil {
					ch <- RPCResponse{
						Error: &baseMessage.Error.Message,
					}
				} else {
					ch <- RPCResponse{
						Response: &baseMessage.Result,
					}
				}
				c.mu.Lock()
				delete(c.responses, *baseMessage.ID)
				c.mu.Unlock()
			}
		}
	}
}

// sendRequest sends a JSON-RPC request to the server and waits for a response.
// It creates a unique request ID, sends the request over stdin, and waits for
// the corresponding response or context cancellation.
// Returns the raw JSON response message or an error if the request fails.
func (c *StdioMCPClient) sendRequest(
	ctx context.Context,
	method string,
	params interface{},
) (*json.RawMessage, error) {
	if !c.initialized && method != "initialize" {
		return nil, fmt.Errorf("client not initialized")
	}

	id := c.requestID.Add(1)

	// Create the complete request structure
	request := mcp.JSONRPCRequest{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      id,
		Request: mcp.Request{
			Method: method,
		},
		Params: params,
	}

	responseChan := make(chan RPCResponse, 1)
	c.mu.Lock()
	c.responses[id] = responseChan
	c.mu.Unlock()

	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	requestBytes = append(requestBytes, '\n')

	if _, err := c.stdin.Write(requestBytes); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	select {
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.responses, id)
		c.mu.Unlock()
		return nil, ctx.Err()
	case response := <-responseChan:
		if response.Error != nil {
			return nil, errors.New(*response.Error)
		}
		return response.Response, nil
	}
}

func (c *StdioMCPClient) Ping(ctx context.Context) error {
	_, err := c.sendRequest(ctx, "ping", nil)
	return err
}

func (c *StdioMCPClient) Initialize(
	ctx context.Context,
	request mcp.InitializeRequest,
) (*mcp.InitializeResult, error) {
	// This structure ensures Capabilities is always included in JSON
	params := struct {
		ProtocolVersion string                 `json:"protocolVersion"`
		ClientInfo      mcp.Implementation     `json:"clientInfo"`
		Capabilities    mcp.ClientCapabilities `json:"capabilities"`
	}{
		ProtocolVersion: request.Params.ProtocolVersion,
		ClientInfo:      request.Params.ClientInfo,
		Capabilities:    request.Params.Capabilities, // Will be empty struct if not set
	}

	response, err := c.sendRequest(ctx, "initialize", params)
	if err != nil {
		return nil, err
	}

	var result mcp.InitializeResult
	if err := json.Unmarshal(*response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Store capabilities
	c.capabilities = result.Capabilities

	// Send initialized notification
	notification := mcp.JSONRPCNotification{
		JSONRPC: mcp.JSONRPC_VERSION,
		Notification: mcp.Notification{
			Method: "notifications/initialized",
		},
	}

	notificationBytes, err := json.Marshal(notification)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to marshal initialized notification: %w",
			err,
		)
	}
	notificationBytes = append(notificationBytes, '\n')

	if _, err := c.stdin.Write(notificationBytes); err != nil {
		return nil, fmt.Errorf(
			"failed to send initialized notification: %w",
			err,
		)
	}

	c.initialized = true
	return &result, nil
}

func (c *StdioMCPClient) ListResources(
	ctx context.Context,
	request mcp.ListResourcesRequest,
) (*mcp.
	ListResourcesResult, error) {
	response, err := c.sendRequest(
		ctx,
		"resources/list",
		request.Params,
	)
	if err != nil {
		return nil, err
	}

	var result mcp.ListResourcesResult
	if err := json.Unmarshal(*response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

func (c *StdioMCPClient) ListResourceTemplates(
	ctx context.Context,
	request mcp.ListResourceTemplatesRequest,
) (*mcp.
	ListResourceTemplatesResult, error) {
	response, err := c.sendRequest(
		ctx,
		"resources/templates/list",
		request.Params,
	)
	if err != nil {
		return nil, err
	}

	var result mcp.ListResourceTemplatesResult
	if err := json.Unmarshal(*response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

func (c *StdioMCPClient) ReadResource(
	ctx context.Context,
	request mcp.ReadResourceRequest,
) (*mcp.ReadResourceResult,
	error) {
	response, err := c.sendRequest(ctx, "resources/read", request.Params)
	if err != nil {
		return nil, err
	}

	return mcp.ParseReadResourceResult(response)
}

func (c *StdioMCPClient) Subscribe(
	ctx context.Context,
	request mcp.SubscribeRequest,
) error {
	_, err := c.sendRequest(ctx, "resources/subscribe", request.Params)
	return err
}

func (c *StdioMCPClient) Unsubscribe(
	ctx context.Context,
	request mcp.UnsubscribeRequest,
) error {
	_, err := c.sendRequest(ctx, "resources/unsubscribe", request.Params)
	return err
}

func (c *StdioMCPClient) ListPrompts(
	ctx context.Context,
	request mcp.ListPromptsRequest,
) (*mcp.ListPromptsResult, error) {
	response, err := c.sendRequest(ctx, "prompts/list", request.Params)
	if err != nil {
		return nil, err
	}

	var result mcp.ListPromptsResult
	if err := json.Unmarshal(*response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

func (c *StdioMCPClient) GetPrompt(
	ctx context.Context,
	request mcp.GetPromptRequest,
) (*mcp.GetPromptResult, error) {
	response, err := c.sendRequest(ctx, "prompts/get", request.Params)
	if err != nil {
		return nil, err
	}

	return mcp.ParseGetPromptResult(response)
}

func (c *StdioMCPClient) ListTools(
	ctx context.Context,
	request mcp.ListToolsRequest,
) (*mcp.ListToolsResult, error) {
	response, err := c.sendRequest(ctx, "tools/list", request.Params)
	if err != nil {
		return nil, err
	}

	var result mcp.ListToolsResult
	if err := json.Unmarshal(*response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

func (c *StdioMCPClient) CallTool(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	response, err := c.sendRequest(ctx, "tools/call", request.Params)
	if err != nil {
		return nil, err
	}

	return mcp.ParseCallToolResult(response)
}

func (c *StdioMCPClient) SetLevel(
	ctx context.Context,
	request mcp.SetLevelRequest,
) error {
	_, err := c.sendRequest(ctx, "logging/setLevel", request.Params)
	return err
}

func (c *StdioMCPClient) Complete(
	ctx context.Context,
	request mcp.CompleteRequest,
) (*mcp.CompleteResult, error) {
	response, err := c.sendRequest(ctx, "completion/complete", request.Params)
	if err != nil {
		return nil, err
	}

	var result mcp.CompleteResult
	if err := json.Unmarshal(*response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}
