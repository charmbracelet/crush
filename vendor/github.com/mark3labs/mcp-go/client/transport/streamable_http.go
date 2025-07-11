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
	"github.com/mark3labs/mcp-go/util"
)

type StreamableHTTPCOption func(*StreamableHTTP)

// WithContinuousListening enables receiving server-to-client notifications when no request is in flight.
// In particular, if you want to receive global notifications from the server (like ToolListChangedNotification),
// you should enable this option.
//
// It will establish a standalone long-live GET HTTP connection to the server.
// https://modelcontextprotocol.io/specification/2025-03-26/basic/transports#listening-for-messages-from-the-server
// NOTICE: Even enabled, the server may not support this feature.
func WithContinuousListening() StreamableHTTPCOption {
	return func(sc *StreamableHTTP) {
		sc.getListeningEnabled = true
	}
}

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

func WithLogger(logger util.Logger) StreamableHTTPCOption {
	return func(sc *StreamableHTTP) {
		sc.logger = logger
	}
}

// WithSession creates a client with a pre-configured session
func WithSession(sessionID string) StreamableHTTPCOption {
	return func(sc *StreamableHTTP) {
		sc.sessionID.Store(sessionID)
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
//   - resuming stream
//     (https://modelcontextprotocol.io/specification/2025-03-26/basic/transports#resumability-and-redelivery)
//   - server -> client request
type StreamableHTTP struct {
	serverURL           *url.URL
	httpClient          *http.Client
	headers             map[string]string
	headerFunc          HTTPHeaderFunc
	logger              util.Logger
	getListeningEnabled bool

	sessionID atomic.Value // string

	initialized     chan struct{}
	initializedOnce sync.Once

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
		serverURL:   parsedURL,
		httpClient:  &http.Client{},
		headers:     make(map[string]string),
		closed:      make(chan struct{}),
		logger:      util.DefaultLogger(),
		initialized: make(chan struct{}),
	}
	smc.sessionID.Store("") // set initial value to simplify later usage

	for _, opt := range options {
		if opt != nil {
			opt(smc)
		}
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
	// For Streamable HTTP, we don't need to establish a persistent connection by default
	if c.getListeningEnabled {
		go func() {
			select {
			case <-c.initialized:
				ctx, cancel := c.contextAwareOfClientClose(ctx)
				defer cancel()
				c.listenForever(ctx)
			case <-c.closed:
				return
			}
		}()
	}

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
				c.logger.Errorf("failed to create close request: %v", err)
				return
			}
			req.Header.Set(headerKeySessionID, sessionId)
			res, err := c.httpClient.Do(req)
			if err != nil {
				c.logger.Errorf("failed to send close request: %v", err)
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

	// Marshal request
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	ctx, cancel := c.contextAwareOfClientClose(ctx)
	defer cancel()

	resp, err := c.sendHTTP(ctx, http.MethodPost, bytes.NewReader(requestBody), "application/json, text/event-stream")
	if err != nil {
		if errors.Is(err, ErrSessionTerminated) && request.Method == string(mcp.MethodInitialize) {
			// If the request is initialize, should not return a SessionTerminated error
			// It should be a genuine endpoint-routing issue.
			// ( Fall through to return StatusCode checking. )
		} else {
			return nil, fmt.Errorf("failed to send request: %w", err)
		}
	}
	defer resp.Body.Close()

	// Check if we got an error response
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {

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

		c.initializedOnce.Do(func() {
			close(c.initialized)
		})
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
		return c.handleSSEResponse(ctx, resp.Body, false)

	default:
		return nil, fmt.Errorf("unexpected content type: %s", resp.Header.Get("Content-Type"))
	}
}

func (c *StreamableHTTP) sendHTTP(
	ctx context.Context,
	method string,
	body io.Reader,
	acceptType string,
) (resp *http.Response, err error) {

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, method, c.serverURL.String(), body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", acceptType)
	sessionID := c.sessionID.Load().(string)
	if sessionID != "" {
		req.Header.Set(headerKeySessionID, sessionID)
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
	resp, err = c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// universal handling for session terminated
	if resp.StatusCode == http.StatusNotFound {
		c.sessionID.CompareAndSwap(sessionID, "")
		return nil, ErrSessionTerminated
	}

	return resp, nil
}

// handleSSEResponse processes an SSE stream for a specific request.
// It returns the final result for the request once received, or an error.
// If ignoreResponse is true, it won't return when a response messge is received. This is for continuous listening.
func (c *StreamableHTTP) handleSSEResponse(ctx context.Context, reader io.ReadCloser, ignoreResponse bool) (*JSONRPCResponse, error) {

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
				c.logger.Errorf("failed to unmarshal message: %v", err)
				return
			}

			// Handle notification
			if message.ID.IsNil() {
				var notification mcp.JSONRPCNotification
				if err := json.Unmarshal([]byte(data), &notification); err != nil {
					c.logger.Errorf("failed to unmarshal notification: %v", err)
					return
				}
				c.notifyMu.RLock()
				if c.notificationHandler != nil {
					c.notificationHandler(notification)
				}
				c.notifyMu.RUnlock()
				return
			}

			if !ignoreResponse {
				responseChan <- &message
			}
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
					c.logger.Errorf("SSE stream error: %v", err)
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
	ctx, cancel := c.contextAwareOfClientClose(ctx)
	defer cancel()

	resp, err := c.sendHTTP(ctx, http.MethodPost, bytes.NewReader(requestBody), "application/json, text/event-stream")
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

func (c *StreamableHTTP) listenForever(ctx context.Context) {
	c.logger.Infof("listening to server forever")
	for {
		err := c.createGETConnectionToServer(ctx)
		if errors.Is(err, ErrGetMethodNotAllowed) {
			// server does not support listening
			c.logger.Errorf("server does not support listening")
			return
		}

		select {
		case <-ctx.Done():
			return
		default:
		}

		if err != nil {
			c.logger.Errorf("failed to listen to server. retry in 1 second: %v", err)
		}
		time.Sleep(retryInterval)
	}
}

var (
	ErrSessionTerminated   = fmt.Errorf("session terminated (404). need to re-initialize")
	ErrGetMethodNotAllowed = fmt.Errorf("GET method not allowed")

	retryInterval = 1 * time.Second // a variable is convenient for testing
)

func (c *StreamableHTTP) createGETConnectionToServer(ctx context.Context) error {

	resp, err := c.sendHTTP(ctx, http.MethodGet, nil, "text/event-stream")
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check if we got an error response
	if resp.StatusCode == http.StatusMethodNotAllowed {
		return ErrGetMethodNotAllowed
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, body)
	}

	// handle SSE response
	contentType := resp.Header.Get("Content-Type")
	if contentType != "text/event-stream" {
		return fmt.Errorf("unexpected content type: %s", contentType)
	}

	// When ignoreResponse is true, the function will never return expect context is done.
	// NOTICE: Due to the ambiguity of the specification, other SDKs may use the GET connection to transfer the response
	// messages. To be more compatible, we should handle this response, however, as the transport layer is message-based,
	// currently, there is no convenient way to handle this response.
	// So we ignore the response here. It's not a bug, but may be not compatible with other SDKs.
	_, err = c.handleSSEResponse(ctx, resp.Body, true)
	if err != nil {
		return fmt.Errorf("failed to handle SSE response: %w", err)
	}

	return nil
}

func (c *StreamableHTTP) contextAwareOfClientClose(ctx context.Context) (context.Context, context.CancelFunc) {
	newCtx, cancel := context.WithCancel(ctx)
	go func() {
		select {
		case <-c.closed:
			cancel()
		case <-newCtx.Done():
			// The original context was canceled
			cancel()
		}
	}()
	return newCtx, cancel
}
