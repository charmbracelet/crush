package transport

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
)

// HTTPHeaderFunc is a function that extracts header entries from the given context
// and returns them as key-value pairs. This is typically used to add context values
// as HTTP headers in outgoing requests.
type HTTPHeaderFunc func(context.Context) map[string]string

// Interface for the transport layer.
type Interface interface {
	// Start the connection. Start should only be called once.
	Start(ctx context.Context) error

	// SendRequest sends a json RPC request and returns the response synchronously.
	SendRequest(ctx context.Context, request JSONRPCRequest) (*JSONRPCResponse, error)

	// SendNotification sends a json RPC Notification to the server.
	SendNotification(ctx context.Context, notification mcp.JSONRPCNotification) error

	// SetNotificationHandler sets the handler for notifications.
	// Any notification before the handler is set will be discarded.
	SetNotificationHandler(handler func(notification mcp.JSONRPCNotification))

	// Close the connection.
	Close() error
}

type JSONRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      mcp.RequestId `json:"id"`
	Method  string        `json:"method"`
	Params  any           `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      mcp.RequestId   `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	} `json:"error"`
}
