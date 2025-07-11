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

	// GetSessionId returns the session ID of the transport.
	GetSessionId() string
}

// RequestHandler defines a function that handles incoming requests from the server.
type RequestHandler func(ctx context.Context, request JSONRPCRequest) (*JSONRPCResponse, error)

// BidirectionalInterface extends Interface to support incoming requests from the server.
// This is used for features like sampling where the server can send requests to the client.
type BidirectionalInterface interface {
	Interface

	// SetRequestHandler sets the handler for incoming requests from the server.
	// The handler should process the request and return a response.
	SetRequestHandler(handler RequestHandler)
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
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	} `json:"error,omitempty"`
}
