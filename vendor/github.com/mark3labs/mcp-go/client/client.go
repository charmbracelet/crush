package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

// Client implements the MCP client.
type Client struct {
	transport transport.Interface

	initialized        bool
	notifications      []func(mcp.JSONRPCNotification)
	notifyMu           sync.RWMutex
	requestID          atomic.Int64
	clientCapabilities mcp.ClientCapabilities
	serverCapabilities mcp.ServerCapabilities
}

type ClientOption func(*Client)

// WithClientCapabilities sets the client capabilities for the client.
func WithClientCapabilities(capabilities mcp.ClientCapabilities) ClientOption {
	return func(c *Client) {
		c.clientCapabilities = capabilities
	}
}

// NewClient creates a new MCP client with the given transport.
// Usage:
//
//	stdio := transport.NewStdio("./mcp_server", nil, "xxx")
//	client, err := NewClient(stdio)
//	if err != nil {
//	    log.Fatalf("Failed to create client: %v", err)
//	}
func NewClient(transport transport.Interface, options ...ClientOption) *Client {
	client := &Client{
		transport: transport,
	}

	for _, opt := range options {
		opt(client)
	}

	return client
}

// Start initiates the connection to the server.
// Must be called before using the client.
func (c *Client) Start(ctx context.Context) error {
	if c.transport == nil {
		return fmt.Errorf("transport is nil")
	}
	err := c.transport.Start(ctx)
	if err != nil {
		return err
	}

	c.transport.SetNotificationHandler(func(notification mcp.JSONRPCNotification) {
		c.notifyMu.RLock()
		defer c.notifyMu.RUnlock()
		for _, handler := range c.notifications {
			handler(notification)
		}
	})
	return nil
}

// Close shuts down the client and closes the transport.
func (c *Client) Close() error {
	return c.transport.Close()
}

// OnNotification registers a handler function to be called when notifications are received.
// Multiple handlers can be registered and will be called in the order they were added.
func (c *Client) OnNotification(
	handler func(notification mcp.JSONRPCNotification),
) {
	c.notifyMu.Lock()
	defer c.notifyMu.Unlock()
	c.notifications = append(c.notifications, handler)
}

// sendRequest sends a JSON-RPC request to the server and waits for a response.
// Returns the raw JSON response message or an error if the request fails.
func (c *Client) sendRequest(
	ctx context.Context,
	method string,
	params any,
) (*json.RawMessage, error) {
	if !c.initialized && method != "initialize" {
		return nil, fmt.Errorf("client not initialized")
	}

	id := c.requestID.Add(1)

	request := transport.JSONRPCRequest{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      mcp.NewRequestId(id),
		Method:  method,
		Params:  params,
	}

	response, err := c.transport.SendRequest(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("transport error: %w", err)
	}

	if response.Error != nil {
		return nil, errors.New(response.Error.Message)
	}

	return &response.Result, nil
}

// Initialize negotiates with the server.
// Must be called after Start, and before any request methods.
func (c *Client) Initialize(
	ctx context.Context,
	request mcp.InitializeRequest,
) (*mcp.InitializeResult, error) {
	// Ensure we send a params object with all required fields
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

	// Store serverCapabilities
	c.serverCapabilities = result.Capabilities

	// Send initialized notification
	notification := mcp.JSONRPCNotification{
		JSONRPC: mcp.JSONRPC_VERSION,
		Notification: mcp.Notification{
			Method: "notifications/initialized",
		},
	}

	err = c.transport.SendNotification(ctx, notification)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to send initialized notification: %w",
			err,
		)
	}

	c.initialized = true
	return &result, nil
}

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.sendRequest(ctx, "ping", nil)
	return err
}

// ListResourcesByPage manually list resources by page.
func (c *Client) ListResourcesByPage(
	ctx context.Context,
	request mcp.ListResourcesRequest,
) (*mcp.ListResourcesResult, error) {
	result, err := listByPage[mcp.ListResourcesResult](ctx, c, request.PaginatedRequest, "resources/list")
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) ListResources(
	ctx context.Context,
	request mcp.ListResourcesRequest,
) (*mcp.ListResourcesResult, error) {
	result, err := c.ListResourcesByPage(ctx, request)
	if err != nil {
		return nil, err
	}
	for result.NextCursor != "" {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			request.Params.Cursor = result.NextCursor
			newPageRes, err := c.ListResourcesByPage(ctx, request)
			if err != nil {
				return nil, err
			}
			result.Resources = append(result.Resources, newPageRes.Resources...)
			result.NextCursor = newPageRes.NextCursor
		}
	}
	return result, nil
}

func (c *Client) ListResourceTemplatesByPage(
	ctx context.Context,
	request mcp.ListResourceTemplatesRequest,
) (*mcp.ListResourceTemplatesResult, error) {
	result, err := listByPage[mcp.ListResourceTemplatesResult](ctx, c, request.PaginatedRequest, "resources/templates/list")
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) ListResourceTemplates(
	ctx context.Context,
	request mcp.ListResourceTemplatesRequest,
) (*mcp.ListResourceTemplatesResult, error) {
	result, err := c.ListResourceTemplatesByPage(ctx, request)
	if err != nil {
		return nil, err
	}
	for result.NextCursor != "" {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			request.Params.Cursor = result.NextCursor
			newPageRes, err := c.ListResourceTemplatesByPage(ctx, request)
			if err != nil {
				return nil, err
			}
			result.ResourceTemplates = append(result.ResourceTemplates, newPageRes.ResourceTemplates...)
			result.NextCursor = newPageRes.NextCursor
		}
	}
	return result, nil
}

func (c *Client) ReadResource(
	ctx context.Context,
	request mcp.ReadResourceRequest,
) (*mcp.ReadResourceResult, error) {
	response, err := c.sendRequest(ctx, "resources/read", request.Params)
	if err != nil {
		return nil, err
	}

	return mcp.ParseReadResourceResult(response)
}

func (c *Client) Subscribe(
	ctx context.Context,
	request mcp.SubscribeRequest,
) error {
	_, err := c.sendRequest(ctx, "resources/subscribe", request.Params)
	return err
}

func (c *Client) Unsubscribe(
	ctx context.Context,
	request mcp.UnsubscribeRequest,
) error {
	_, err := c.sendRequest(ctx, "resources/unsubscribe", request.Params)
	return err
}

func (c *Client) ListPromptsByPage(
	ctx context.Context,
	request mcp.ListPromptsRequest,
) (*mcp.ListPromptsResult, error) {
	result, err := listByPage[mcp.ListPromptsResult](ctx, c, request.PaginatedRequest, "prompts/list")
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) ListPrompts(
	ctx context.Context,
	request mcp.ListPromptsRequest,
) (*mcp.ListPromptsResult, error) {
	result, err := c.ListPromptsByPage(ctx, request)
	if err != nil {
		return nil, err
	}
	for result.NextCursor != "" {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			request.Params.Cursor = result.NextCursor
			newPageRes, err := c.ListPromptsByPage(ctx, request)
			if err != nil {
				return nil, err
			}
			result.Prompts = append(result.Prompts, newPageRes.Prompts...)
			result.NextCursor = newPageRes.NextCursor
		}
	}
	return result, nil
}

func (c *Client) GetPrompt(
	ctx context.Context,
	request mcp.GetPromptRequest,
) (*mcp.GetPromptResult, error) {
	response, err := c.sendRequest(ctx, "prompts/get", request.Params)
	if err != nil {
		return nil, err
	}

	return mcp.ParseGetPromptResult(response)
}

func (c *Client) ListToolsByPage(
	ctx context.Context,
	request mcp.ListToolsRequest,
) (*mcp.ListToolsResult, error) {
	result, err := listByPage[mcp.ListToolsResult](ctx, c, request.PaginatedRequest, "tools/list")
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) ListTools(
	ctx context.Context,
	request mcp.ListToolsRequest,
) (*mcp.ListToolsResult, error) {
	result, err := c.ListToolsByPage(ctx, request)
	if err != nil {
		return nil, err
	}
	for result.NextCursor != "" {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			request.Params.Cursor = result.NextCursor
			newPageRes, err := c.ListToolsByPage(ctx, request)
			if err != nil {
				return nil, err
			}
			result.Tools = append(result.Tools, newPageRes.Tools...)
			result.NextCursor = newPageRes.NextCursor
		}
	}
	return result, nil
}

func (c *Client) CallTool(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	response, err := c.sendRequest(ctx, "tools/call", request.Params)
	if err != nil {
		return nil, err
	}

	return mcp.ParseCallToolResult(response)
}

func (c *Client) SetLevel(
	ctx context.Context,
	request mcp.SetLevelRequest,
) error {
	_, err := c.sendRequest(ctx, "logging/setLevel", request.Params)
	return err
}

func (c *Client) Complete(
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

func listByPage[T any](
	ctx context.Context,
	client *Client,
	request mcp.PaginatedRequest,
	method string,
) (*T, error) {
	response, err := client.sendRequest(ctx, method, request.Params)
	if err != nil {
		return nil, err
	}
	var result T
	if err := json.Unmarshal(*response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &result, nil
}

// Helper methods

// GetTransport gives access to the underlying transport layer.
// Cast it to the specific transport type and obtain the other helper methods.
func (c *Client) GetTransport() transport.Interface {
	return c.transport
}

// GetServerCapabilities returns the server capabilities.
func (c *Client) GetServerCapabilities() mcp.ServerCapabilities {
	return c.serverCapabilities
}

// GetClientCapabilities returns the client capabilities.
func (c *Client) GetClientCapabilities() mcp.ClientCapabilities {
	return c.clientCapabilities
}
