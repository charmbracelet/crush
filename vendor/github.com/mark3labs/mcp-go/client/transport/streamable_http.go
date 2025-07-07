package transport

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

type StreamableHTTPCOption func(*StreamableHTTP)

// WithHTTPClient sets a custom HTTP client on the StreamableHTTP transport.
func WithHTTPBasicClient(client *http.Client) StreamableHTTPCOption {
	return func(sc *StreamableHTTP) {
		sc.httpClient = client
	}
}

func WithHTTPHeaders(headers map[string]string) StreamableHTTPCOption {
	return func(sc *StreamableHTTP) {
		sc.headers = headers
	}
}

func WithHTTPHeaderFunc(headerFunc HTTPHeaderFunc) StreamableHTTPCOption {
	return func(sc *StreamableHTTP) {
		sc.headerFunc = headerFunc
	}
}

// WithHTTPTimeout sets the timeout for a HTTP request and stream.
func WithHTTPTimeout(timeout time.Duration) StreamableHTTPCOption {
	return func(sc *StreamableHTTP) {
		sc.httpClient.Timeout = timeout
	}
}

// WithHTTPOAuth enables OAuth authentication for the client.
func WithHTTPOAuth(config OAuthConfig) StreamableHTTPCOption {
	return func(sc *StreamableHTTP) {
		sc.oauthHandler = NewOAuthHandler(config)
	}
}

// StreamableHTTP implements Streamable HTTP transport.
//
// It transmits JSON-RPC messages over individual HTTP requests. One message per request.
// The HTTP response body can either be a single JSON-RPC response,
// or an upgraded SSE stream that concludes with a JSON-RPC response for the same request.
//
// https://modelcontextprotocol.io/specification/2025-03-26/basic/transports
//
// The current implementation does not support the following features:
//   - batching
//   - continuously listening for server notifications when no request is in flight
//     (https://modelcontextprotocol.io/specification/2025-03-26/basic/transports#listening-for-messages-from-the-server)
//   - resuming stream
//     (https://modelcontextprotocol.io/specification/2025-03-26/basic/transports#resumability-and-redelivery)
//   - server -> client request
type StreamableHTTP struct {
	serverURL  *url.URL
	httpClient *http.Client
	headers    map[string]string
	headerFunc HTTPHeaderFunc

	sessionID atomic.Value // string

	notificationHandler func(mcp.JSONRPCNotification)
	notifyMu            sync.RWMutex

	closed chan struct{}

	// OAuth support
	oauthHandler *OAuthHandler
}

// NewStreamableHTTP creates a new Streamable HTTP transport with the given server URL.
// Returns an error if the URL is invalid.
func NewStreamableHTTP(serverURL string, options ...StreamableHTTPCOption) (*StreamableHTTP, error) {
	parsedURL, err := url.Parse(serverURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	smc := &StreamableHTTP{
		serverURL:  parsedURL,
		httpClient: &http.Client{},
		headers:    make(map[string]string),
		closed:     make(chan struct{}),
	}
	smc.sessionID.Store("") // set initial value to simplify later usage

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

// Start initiates the HTTP connection to the server.
func (c *StreamableHTTP) Start(ctx context.Context) error {
	// For Streamable HTTP, we don't need to establish a persistent connection
	return nil
}

// Close closes the all the HTTP connections to the server.
func (c *StreamableHTTP) Close() error {
	select {
	case <-c.closed:
		return nil
	default:
	}
	// Cancel all in-flight requests
	close(c.closed)

	sessionId := c.sessionID.Load().(string)
	if sessionId != "" {
		c.sessionID.Store("")

		// notify server session closed
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.serverURL.String(), nil)
			if err != nil {
				fmt.Printf("failed to create close request\n: %v", err)
				return
			}
			req.Header.Set(headerKeySessionID, sessionId)
			res, err := c.httpClient.Do(req)
			if err != nil {
				fmt.Printf("failed to send close request\n: %v", err)
				return
			}
			res.Body.Close()
		}()
	}

	return nil
}

const (
	headerKeySessionID = "Mcp-Session-Id"
)

// ErrOAuthAuthorizationRequired is a sentinel error for OAuth authorization required
var ErrOAuthAuthorizationRequired = errors.New("no valid token available, authorization required")

// OAuthAuthorizationRequiredError is returned when OAuth authorization is required
type OAuthAuthorizationRequiredError struct {
	Handler *OAuthHandler
}

func (e *OAuthAuthorizationRequiredError) Error() string {
	return ErrOAuthAuthorizationRequired.Error()
}

func (e *OAuthAuthorizationRequiredError) Unwrap() error {
	return ErrOAuthAuthorizationRequired
}

// SendRequest sends a JSON-RPC request to the server and waits for a response.
// Returns the raw JSON response message or an error if the request fails.
func (c *StreamableHTTP) SendRequest(
	ctx context.Context,
	request JSONRPCRequest,
) (*JSONRPCResponse, error) {

	// Create a combined context that could be canceled when the client is closed
	newCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-c.closed:
			cancel()
		case <-newCtx.Done():
			// The original context was canceled, no need to do anything
		}
	}()
	ctx = newCtx

	// Marshal request
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.serverURL.String(), bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	sessionID := c.sessionID.Load()
	if sessionID != "" {
		req.Header.Set(headerKeySessionID, sessionID.(string))
	}
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

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check if we got an error response
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		// handle session closed
		if resp.StatusCode == http.StatusNotFound {
			c.sessionID.CompareAndSwap(sessionID, "")
			return nil, fmt.Errorf("session terminated (404). need to re-initialize")
		}

		// Handle OAuth unauthorized error
		if resp.StatusCode == http.StatusUnauthorized && c.oauthHandler != nil {
			return nil, &OAuthAuthorizationRequiredError{
				Handler: c.oauthHandler,
			}
		}

		// handle error response
		var errResponse JSONRPCResponse
		body, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(body, &errResponse); err == nil {
			return &errResponse, nil
		}
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, body)
	}

	if request.Method == string(mcp.MethodInitialize) {
		// saved the received session ID in the response
		// empty session ID is allowed
		if sessionID := resp.Header.Get(headerKeySessionID); sessionID != "" {
			c.sessionID.Store(sessionID)
		}
	}

	// Handle different response types
	mediaType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	switch mediaType {
	case "application/json":
		// Single response
		var response JSONRPCResponse
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		// should not be a notification
		if response.ID.IsNil() {
			return nil, fmt.Errorf("response should contain RPC id: %v", response)
		}

		return &response, nil

	case "text/event-stream":
		// Server is using SSE for streaming responses
		return c.handleSSEResponse(ctx, resp.Body)

	default:
		return nil, fmt.Errorf("unexpected content type: %s", resp.Header.Get("Content-Type"))
	}
}

// handleSSEResponse processes an SSE stream for a specific request.
// It returns the final result for the request once received, or an error.
func (c *StreamableHTTP) handleSSEResponse(ctx context.Context, reader io.ReadCloser) (*JSONRPCResponse, error) {

	// Create a channel for this specific request
	responseChan := make(chan *JSONRPCResponse, 1)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start a goroutine to process the SSE stream
	go func() {
		// only close responseChan after readingSSE()
		defer close(responseChan)

		c.readSSE(ctx, reader, func(event, data string) {

			// (unsupported: batching)

			var message JSONRPCResponse
			if err := json.Unmarshal([]byte(data), &message); err != nil {
				fmt.Printf("failed to unmarshal message: %v\n", err)
				return
			}

			// Handle notification
			if message.ID.IsNil() {
				var notification mcp.JSONRPCNotification
				if err := json.Unmarshal([]byte(data), &notification); err != nil {
					fmt.Printf("failed to unmarshal notification: %v\n", err)
					return
				}
				c.notifyMu.RLock()
				if c.notificationHandler != nil {
					c.notificationHandler(notification)
				}
				c.notifyMu.RUnlock()
				return
			}

			responseChan <- &message
		})
	}()

	// Wait for the response or context cancellation
	select {
	case response := <-responseChan:
		if response == nil {
			return nil, fmt.Errorf("unexpected nil response")
		}
		return response, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// readSSE reads the SSE stream(reader) and calls the handler for each event and data pair.
// It will end when the reader is closed (or the context is done).
func (c *StreamableHTTP) readSSE(ctx context.Context, reader io.ReadCloser, handler func(event, data string)) {
	defer reader.Close()

	br := bufio.NewReader(reader)
	var event, data string

	for {
		select {
		case <-ctx.Done():
			return
		default:
			line, err := br.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					// Process any pending event before exit
					if data != "" {
						// If no event type is specified, use empty string (default event type)
						if event == "" {
							event = "message"
						}
						handler(event, data)
					}
					return
				}
				select {
				case <-ctx.Done():
					return
				default:
					fmt.Printf("SSE stream error: %v\n", err)
					return
				}
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
					handler(event, data)
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
}

func (c *StreamableHTTP) SendNotification(ctx context.Context, notification mcp.JSONRPCNotification) error {

	// Marshal request
	requestBody, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.serverURL.String(), bytes.NewReader(requestBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if sessionID := c.sessionID.Load(); sessionID != "" {
		req.Header.Set(headerKeySessionID, sessionID.(string))
	}
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

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
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

func (c *StreamableHTTP) SetNotificationHandler(handler func(mcp.JSONRPCNotification)) {
	c.notifyMu.Lock()
	defer c.notifyMu.Unlock()
	c.notificationHandler = handler
}

func (c *StreamableHTTP) GetSessionId() string {
	return c.sessionID.Load().(string)
}

// GetOAuthHandler returns the OAuth handler if configured
func (c *StreamableHTTP) GetOAuthHandler() *OAuthHandler {
	return c.oauthHandler
}

// IsOAuthEnabled returns true if OAuth is enabled
func (c *StreamableHTTP) IsOAuthEnabled() bool {
	return c.oauthHandler != nil
}
