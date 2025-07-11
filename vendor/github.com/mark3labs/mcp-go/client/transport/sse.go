package transport

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// SSE implements the transport layer of the MCP protocol using Server-Sent Events (SSE).
// It maintains a persistent HTTP connection to receive server-pushed events
// while sending requests over regular HTTP POST calls. The client handles
// automatic reconnection and message routing between requests and responses.
type SSE struct {
	baseURL        *url.URL
	endpoint       *url.URL
	httpClient     *http.Client
	responses      map[string]chan *JSONRPCResponse
	mu             sync.RWMutex
	onNotification func(mcp.JSONRPCNotification)
	notifyMu       sync.RWMutex
	endpointChan   chan struct{}
	headers        map[string]string
	headerFunc     HTTPHeaderFunc

	started         atomic.Bool
	closed          atomic.Bool
	cancelSSEStream context.CancelFunc

	// OAuth support
	oauthHandler *OAuthHandler
}

type ClientOption func(*SSE)

func WithHeaders(headers map[string]string) ClientOption {
	return func(sc *SSE) {
		sc.headers = headers
	}
}

func WithHeaderFunc(headerFunc HTTPHeaderFunc) ClientOption {
	return func(sc *SSE) {
		sc.headerFunc = headerFunc
	}
}

func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(sc *SSE) {
		sc.httpClient = httpClient
	}
}

func WithOAuth(config OAuthConfig) ClientOption {
	return func(sc *SSE) {
		sc.oauthHandler = NewOAuthHandler(config)
	}
}

// NewSSE creates a new SSE-based MCP client with the given base URL.
// Returns an error if the URL is invalid.
func NewSSE(baseURL string, options ...ClientOption) (*SSE, error) {
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	smc := &SSE{
		baseURL:      parsedURL,
		httpClient:   &http.Client{},
		responses:    make(map[string]chan *JSONRPCResponse),
		endpointChan: make(chan struct{}),
		headers:      make(map[string]string),
	}

	for _, opt := range options {
		opt(smc)
	}

	// If OAuth is configured, set the base URL for metadata discovery
	if smc.oauthHandler != nil {
		// Extract base URL from server URL for metadata discovery
		baseURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)
		smc.oauthHandler.SetBaseURL(baseURL)
	}

	return smc, nil
}

// Start initiates the SSE connection to the server and waits for the endpoint information.
// Returns an error if the connection fails or times out waiting for the endpoint.
func (c *SSE) Start(ctx context.Context) error {

	if c.started.Load() {
		return fmt.Errorf("has already started")
	}

	ctx, cancel := context.WithCancel(ctx)
	c.cancelSSEStream = cancel

	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL.String(), nil)

	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	// set custom http headers
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	if c.headerFunc != nil {
		for k, v := range c.headerFunc(ctx) {
			req.Header.Set(k, v)
		}
	}

	// Add OAuth authorization if configured
	if c.oauthHandler != nil {
		authHeader, err := c.oauthHandler.GetAuthorizationHeader(ctx)
		if err != nil {
			// If we get an authorization error, return a specific error that can be handled by the client
			if err.Error() == "no valid token available, authorization required" {
				return &OAuthAuthorizationRequiredError{
					Handler: c.oauthHandler,
				}
			}
			return fmt.Errorf("failed to get authorization header: %w", err)
		}
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to SSE stream: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		// Handle OAuth unauthorized error
		if resp.StatusCode == http.StatusUnauthorized && c.oauthHandler != nil {
			return &OAuthAuthorizationRequiredError{
				Handler: c.oauthHandler,
			}
		}
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	go c.readSSE(resp.Body)

	// Wait for the endpoint to be received
	timeout := time.NewTimer(30 * time.Second)
	defer timeout.Stop()
	select {
	case <-c.endpointChan:
		// Endpoint received, proceed
	case <-ctx.Done():
		return fmt.Errorf("context cancelled while waiting for endpoint")
	case <-timeout.C: // Add a timeout
		cancel()
		return fmt.Errorf("timeout waiting for endpoint")
	}

	c.started.Store(true)
	return nil
}

// readSSE continuously reads the SSE stream and processes events.
// It runs until the connection is closed or an error occurs.
func (c *SSE) readSSE(reader io.ReadCloser) {
	defer reader.Close()

	br := bufio.NewReader(reader)
	var event, data string

	for {
		// when close or start's ctx cancel, the reader will be closed
		// and the for loop will break.
		line, err := br.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				// Process any pending event before exit
				if data != "" {
					// If no event type is specified, use empty string (default event type)
					if event == "" {
						event = "message"
					}
					c.handleSSEEvent(event, data)
				}
				break
			}
			if !c.closed.Load() {
				fmt.Printf("SSE stream error: %v\n", err)
			}
			return
		}

		// Remove only newline markers
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			// Empty line means end of event
			if data != "" {
				// If no event type is specified, use empty string (default event type)
				if event == "" {
					event = "message"
				}
				c.handleSSEEvent(event, data)
				event = ""
				data = ""
			}
			continue
		}

		if strings.HasPrefix(line, "event:") {
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
	}
}

// handleSSEEvent processes SSE events based on their type.
// Handles 'endpoint' events for connection setup and 'message' events for JSON-RPC communication.
func (c *SSE) handleSSEEvent(event, data string) {
	switch event {
	case "endpoint":
		endpoint, err := c.baseURL.Parse(data)
		if err != nil {
			fmt.Printf("Error parsing endpoint URL: %v\n", err)
			return
		}
		if endpoint.Host != c.baseURL.Host {
			fmt.Printf("Endpoint origin does not match connection origin\n")
			return
		}
		c.endpoint = endpoint
		close(c.endpointChan)

	case "message":
		var baseMessage JSONRPCResponse
		if err := json.Unmarshal([]byte(data), &baseMessage); err != nil {
			fmt.Printf("Error unmarshaling message: %v\n", err)
			return
		}

		// Handle notification
		if baseMessage.ID.IsNil() {
			var notification mcp.JSONRPCNotification
			if err := json.Unmarshal([]byte(data), &notification); err != nil {
				return
			}
			c.notifyMu.RLock()
			if c.onNotification != nil {
				c.onNotification(notification)
			}
			c.notifyMu.RUnlock()
			return
		}

		// Create string key for map lookup
		idKey := baseMessage.ID.String()

		c.mu.RLock()
		ch, exists := c.responses[idKey]
		c.mu.RUnlock()

		if exists {
			ch <- &baseMessage
			c.mu.Lock()
			delete(c.responses, idKey)
			c.mu.Unlock()
		}
	}
}

func (c *SSE) SetNotificationHandler(handler func(notification mcp.JSONRPCNotification)) {
	c.notifyMu.Lock()
	defer c.notifyMu.Unlock()
	c.onNotification = handler
}

// SendRequest sends a JSON-RPC request to the server and waits for a response.
// Returns the raw JSON response message or an error if the request fails.
func (c *SSE) SendRequest(
	ctx context.Context,
	request JSONRPCRequest,
) (*JSONRPCResponse, error) {

	if !c.started.Load() {
		return nil, fmt.Errorf("transport not started yet")
	}
	if c.closed.Load() {
		return nil, fmt.Errorf("transport has been closed")
	}
	if c.endpoint == nil {
		return nil, fmt.Errorf("endpoint not received")
	}

	// Marshal request
	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint.String(), bytes.NewReader(requestBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	// Add OAuth authorization if configured
	if c.oauthHandler != nil {
		authHeader, err := c.oauthHandler.GetAuthorizationHeader(ctx)
		if err != nil {
			// If we get an authorization error, return a specific error that can be handled by the client
			if err.Error() == "no valid token available, authorization required" {
				return nil, &OAuthAuthorizationRequiredError{
					Handler: c.oauthHandler,
				}
			}
			return nil, fmt.Errorf("failed to get authorization header: %w", err)
		}
		req.Header.Set("Authorization", authHeader)
	}

	if c.headerFunc != nil {
		for k, v := range c.headerFunc(ctx) {
			req.Header.Set(k, v)
		}
	}

	// Create string key for map lookup
	idKey := request.ID.String()

	// Register response channel
	responseChan := make(chan *JSONRPCResponse, 1)
	c.mu.Lock()
	c.responses[idKey] = responseChan
	c.mu.Unlock()
	deleteResponseChan := func() {
		c.mu.Lock()
		delete(c.responses, idKey)
		c.mu.Unlock()
	}

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		deleteResponseChan()
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Drain any outstanding io
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()

	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check if we got an error response
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		deleteResponseChan()

		// Handle OAuth unauthorized error
		if resp.StatusCode == http.StatusUnauthorized && c.oauthHandler != nil {
			return nil, &OAuthAuthorizationRequiredError{
				Handler: c.oauthHandler,
			}
		}

		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, body)
	}

	select {
	case <-ctx.Done():
		deleteResponseChan()
		return nil, ctx.Err()
	case response, ok := <-responseChan:
		if ok {
			return response, nil
		}
		return nil, fmt.Errorf("connection has been closed")
	}
}

// Close shuts down the SSE client connection and cleans up any pending responses.
// Returns an error if the shutdown process fails.
func (c *SSE) Close() error {
	if !c.closed.CompareAndSwap(false, true) {
		return nil // Already closed
	}

	if c.cancelSSEStream != nil {
		// It could stop the sse stream body, to quit the readSSE loop immediately
		// Also, it could quit start() immediately if not receiving the endpoint
		c.cancelSSEStream()
	}

	// Clean up any pending responses
	c.mu.Lock()
	for _, ch := range c.responses {
		close(ch)
	}
	c.responses = make(map[string]chan *JSONRPCResponse)
	c.mu.Unlock()

	return nil
}

// GetSessionId returns the session ID of the transport.
// Since SSE does not maintain a session ID, it returns an empty string.
func (c *SSE) GetSessionId() string {
	return ""
}

// SendNotification sends a JSON-RPC notification to the server without expecting a response.
func (c *SSE) SendNotification(ctx context.Context, notification mcp.JSONRPCNotification) error {
	if c.endpoint == nil {
		return fmt.Errorf("endpoint not received")
	}

	notificationBytes, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		c.endpoint.String(),
		bytes.NewReader(notificationBytes),
	)
	if err != nil {
		return fmt.Errorf("failed to create notification request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	// Set custom HTTP headers
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	// Add OAuth authorization if configured
	if c.oauthHandler != nil {
		authHeader, err := c.oauthHandler.GetAuthorizationHeader(ctx)
		if err != nil {
			// If we get an authorization error, return a specific error that can be handled by the client
			if errors.Is(err, ErrOAuthAuthorizationRequired) {
				return &OAuthAuthorizationRequiredError{
					Handler: c.oauthHandler,
				}
			}
			return fmt.Errorf("failed to get authorization header: %w", err)
		}
		req.Header.Set("Authorization", authHeader)
	}

	if c.headerFunc != nil {
		for k, v := range c.headerFunc(ctx) {
			req.Header.Set(k, v)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		// Handle OAuth unauthorized error
		if resp.StatusCode == http.StatusUnauthorized && c.oauthHandler != nil {
			return &OAuthAuthorizationRequiredError{
				Handler: c.oauthHandler,
			}
		}

		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf(
			"notification failed with status %d: %s",
			resp.StatusCode,
			body,
		)
	}

	return nil
}

// GetEndpoint returns the current endpoint URL for the SSE connection.
func (c *SSE) GetEndpoint() *url.URL {
	return c.endpoint
}

// GetBaseURL returns the base URL set in the SSE constructor.
func (c *SSE) GetBaseURL() *url.URL {
	return c.baseURL
}

// GetOAuthHandler returns the OAuth handler if configured
func (c *SSE) GetOAuthHandler() *OAuthHandler {
	return c.oauthHandler
}

// IsOAuthEnabled returns true if OAuth is enabled
func (c *SSE) IsOAuthEnabled() bool {
	return c.oauthHandler != nil
}
