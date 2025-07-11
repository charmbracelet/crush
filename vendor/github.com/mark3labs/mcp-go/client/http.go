package client

import (
	"fmt"

	"github.com/mark3labs/mcp-go/client/transport"
)

// NewStreamableHttpClient is a convenience method that creates a new streamable-http-based MCP client
// with the given base URL. Returns an error if the URL is invalid.
func NewStreamableHttpClient(baseURL string, options ...transport.StreamableHTTPCOption) (*Client, error) {
	trans, err := transport.NewStreamableHTTP(baseURL, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSE transport: %w", err)
	}
	clientOptions := make([]ClientOption, 0)
	sessionID := trans.GetSessionId()
	if sessionID != "" {
		clientOptions = append(clientOptions, WithSession())
	}
	return NewClient(trans, clientOptions...), nil
}
