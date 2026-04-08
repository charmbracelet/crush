package oauth

import (
	"context"
	"time"
)

// DeviceCodeProvider defines the interface for OAuth device code flow implementations.
type DeviceCodeProvider interface {
	// InitiateDeviceAuth starts the device authorization flow.
	InitiateDeviceAuth(ctx context.Context) (*DeviceCodeResponse, error)
	// PollForToken polls for the access token after user authorization.
	PollForToken(ctx context.Context, deviceCode string, expiresIn int) (string, error)
	// ExchangeToken exchanges a refresh token for a new access token.
	ExchangeToken(ctx context.Context, refreshToken string) (*Token, error)
}

// TokenExchanger defines the interface for refreshing/exchanging tokens.
type TokenExchanger interface {
	ExchangeToken(ctx context.Context, refreshToken string) (*Token, error)
}

// DefaultHTTPTimeout is the default timeout for HTTP requests.
const DefaultHTTPTimeout = 30 * time.Second

// DefaultPollInterval is the default polling interval for token retrieval.
const DefaultPollInterval = 5 * time.Second
