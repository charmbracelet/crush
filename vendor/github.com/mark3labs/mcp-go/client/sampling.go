package client

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

// SamplingHandler defines the interface for handling sampling requests from servers.
// Clients can implement this interface to provide LLM sampling capabilities to servers.
type SamplingHandler interface {
	// CreateMessage handles a sampling request from the server and returns the generated message.
	// The implementation should:
	// 1. Validate the request parameters
	// 2. Optionally prompt the user for approval (human-in-the-loop)
	// 3. Select an appropriate model based on preferences
	// 4. Generate the response using the selected model
	// 5. Return the result with model information and stop reason
	CreateMessage(ctx context.Context, request mcp.CreateMessageRequest) (*mcp.CreateMessageResult, error)
}
