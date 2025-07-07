package client

import (
	"errors"
	"fmt"

	"github.com/mark3labs/mcp-go/client/transport"
)

// OAuthConfig is a convenience type that wraps transport.OAuthConfig
type OAuthConfig = transport.OAuthConfig

// Token is a convenience type that wraps transport.Token
type Token = transport.Token

// TokenStore is a convenience type that wraps transport.TokenStore
type TokenStore = transport.TokenStore

// MemoryTokenStore is a convenience type that wraps transport.MemoryTokenStore
type MemoryTokenStore = transport.MemoryTokenStore

// NewMemoryTokenStore is a convenience function that wraps transport.NewMemoryTokenStore
var NewMemoryTokenStore = transport.NewMemoryTokenStore

// NewOAuthStreamableHttpClient creates a new streamable-http-based MCP client with OAuth support.
// Returns an error if the URL is invalid.
func NewOAuthStreamableHttpClient(baseURL string, oauthConfig OAuthConfig, options ...transport.StreamableHTTPCOption) (*Client, error) {
	// Add OAuth option to the list of options
	options = append(options, transport.WithHTTPOAuth(oauthConfig))

	trans, err := transport.NewStreamableHTTP(baseURL, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP transport: %w", err)
	}
	return NewClient(trans), nil
}

// NewOAuthStreamableHttpClient creates a new streamable-http-based MCP client with OAuth support.
// Returns an error if the URL is invalid.
func NewOAuthSSEClient(baseURL string, oauthConfig OAuthConfig, options ...transport.ClientOption) (*Client, error) {
	// Add OAuth option to the list of options
	options = append(options, transport.WithOAuth(oauthConfig))

	trans, err := transport.NewSSE(baseURL, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSE transport: %w", err)
	}
	return NewClient(trans), nil
}

// GenerateCodeVerifier generates a code verifier for PKCE
var GenerateCodeVerifier = transport.GenerateCodeVerifier

// GenerateCodeChallenge generates a code challenge from a code verifier
var GenerateCodeChallenge = transport.GenerateCodeChallenge

// GenerateState generates a state parameter for OAuth
var GenerateState = transport.GenerateState

// OAuthAuthorizationRequiredError is returned when OAuth authorization is required
type OAuthAuthorizationRequiredError = transport.OAuthAuthorizationRequiredError

// IsOAuthAuthorizationRequiredError checks if an error is an OAuthAuthorizationRequiredError
func IsOAuthAuthorizationRequiredError(err error) bool {
	var target *OAuthAuthorizationRequiredError
	return errors.As(err, &target)
}

// GetOAuthHandler extracts the OAuthHandler from an OAuthAuthorizationRequiredError
func GetOAuthHandler(err error) *transport.OAuthHandler {
	var oauthErr *OAuthAuthorizationRequiredError
	if errors.As(err, &oauthErr) {
		return oauthErr.Handler
	}
	return nil
}
