package oauth

import (
	"context"
	"net/http"
)

// FlowType defines which OAuth flows a provider supports.
type FlowType int

const (
	// FlowBrowser represents the authorization code flow with PKCE on loopback.
	FlowBrowser FlowType = 1 << iota
	// FlowDevice represents RFC 8628 device authorization flow.
	FlowDevice
)

// DeviceCode represents a device authorization response.
type DeviceCode struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// BrowserSession represents an active loopback browser authorization session.
type BrowserSession interface {
	// URL returns the authorization URL that must be opened in a browser.
	URL() string
	// Wait waits for the user to complete authorization or context cancellation.
	Wait(ctx context.Context) (*Token, error)
	// Close cleanly stops the callback listener. Safe to call multiple times.
	Close()
}

// Provider represents an OAuth-capable model or service provider.
type Provider interface {
	// ID returns the unique identifier matching the Catwalk provider ID.
	ID() string
	// Name returns the human-readable display name.
	Name() string
	// HasAuthChoices reports whether the provider supports both OAuth and API keys.
	HasAuthChoices() bool
	// SupportedFlows returns the bitmask of supported authentication flows.
	SupportedFlows() FlowType
	// RefreshToken exchanges a refresh token for a fresh OAuth token.
	RefreshToken(ctx context.Context, refreshToken string) (*Token, error)
	// WrapClient wraps the HTTP client for authenticated requests.
	WrapClient(base *http.Client, token *Token, isSubAgent, debug bool) *http.Client
}

// BrowserFlowProvider is implemented by providers supporting browser PKCE flows.
type BrowserFlowProvider interface {
	Provider
	StartBrowserFlow(ctx context.Context) (BrowserSession, error)
}

// DeviceFlowProvider is implemented by providers supporting RFC 8628 device flow.
type DeviceFlowProvider interface {
	Provider
	RequestDeviceCode(ctx context.Context) (*DeviceCode, error)
	PollForToken(ctx context.Context, code *DeviceCode) (*Token, error)
}
