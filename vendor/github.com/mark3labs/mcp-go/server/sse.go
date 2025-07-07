package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/mark3labs/mcp-go/mcp"
)

// sseSession represents an active SSE connection.
type sseSession struct {
	done                chan struct{}
	eventQueue          chan string // Channel for queuing events
	sessionID           string
	requestID           atomic.Int64
	notificationChannel chan mcp.JSONRPCNotification
	initialized         atomic.Bool
	loggingLevel        atomic.Value
	tools               sync.Map     // stores session-specific tools
	clientInfo          atomic.Value // stores session-specific client info
}

// SSEContextFunc is a function that takes an existing context and the current
// request and returns a potentially modified context based on the request
// content. This can be used to inject context values from headers, for example.
type SSEContextFunc func(ctx context.Context, r *http.Request) context.Context

// DynamicBasePathFunc allows the user to provide a function to generate the
// base path for a given request and sessionID. This is useful for cases where
// the base path is not known at the time of SSE server creation, such as when
// using a reverse proxy or when the base path is dynamically generated. The
// function should return the base path (e.g., "/mcp/tenant123").
type DynamicBasePathFunc func(r *http.Request, sessionID string) string

func (s *sseSession) SessionID() string {
	return s.sessionID
}

func (s *sseSession) NotificationChannel() chan<- mcp.JSONRPCNotification {
	return s.notificationChannel
}

func (s *sseSession) Initialize() {
	// set default logging level
	s.loggingLevel.Store(mcp.LoggingLevelError)
	s.initialized.Store(true)
}

func (s *sseSession) Initialized() bool {
	return s.initialized.Load()
}

func (s *sseSession) SetLogLevel(level mcp.LoggingLevel) {
	s.loggingLevel.Store(level)
}

func (s *sseSession) GetLogLevel() mcp.LoggingLevel {
	level := s.loggingLevel.Load()
	if level == nil {
		return mcp.LoggingLevelError
	}
	return level.(mcp.LoggingLevel)
}

func (s *sseSession) GetSessionTools() map[string]ServerTool {
	tools := make(map[string]ServerTool)
	s.tools.Range(func(key, value any) bool {
		if tool, ok := value.(ServerTool); ok {
			tools[key.(string)] = tool
		}
		return true
	})
	return tools
}

func (s *sseSession) SetSessionTools(tools map[string]ServerTool) {
	// Clear existing tools
	s.tools.Clear()

	// Set new tools
	for name, tool := range tools {
		s.tools.Store(name, tool)
	}
}

func (s *sseSession) GetClientInfo() mcp.Implementation {
	if value := s.clientInfo.Load(); value != nil {
		if clientInfo, ok := value.(mcp.Implementation); ok {
			return clientInfo
		}
	}
	return mcp.Implementation{}
}

func (s *sseSession) SetClientInfo(clientInfo mcp.Implementation) {
	s.clientInfo.Store(clientInfo)
}

var (
	_ ClientSession         = (*sseSession)(nil)
	_ SessionWithTools      = (*sseSession)(nil)
	_ SessionWithLogging    = (*sseSession)(nil)
	_ SessionWithClientInfo = (*sseSession)(nil)
)

// SSEServer implements a Server-Sent Events (SSE) based MCP server.
// It provides real-time communication capabilities over HTTP using the SSE protocol.
type SSEServer struct {
	server                       *MCPServer
	baseURL                      string
	basePath                     string
	appendQueryToMessageEndpoint bool
	useFullURLForMessageEndpoint bool
	messageEndpoint              string
	sseEndpoint                  string
	sessions                     sync.Map
	srv                          *http.Server
	contextFunc                  SSEContextFunc
	dynamicBasePathFunc          DynamicBasePathFunc

	keepAlive         bool
	keepAliveInterval time.Duration

	mu sync.RWMutex
}

// SSEOption defines a function type for configuring SSEServer
type SSEOption func(*SSEServer)

// WithBaseURL sets the base URL for the SSE server
func WithBaseURL(baseURL string) SSEOption {
	return func(s *SSEServer) {
		if baseURL != "" {
			u, err := url.Parse(baseURL)
			if err != nil {
				return
			}
			if u.Scheme != "http" && u.Scheme != "https" {
				return
			}
			// Check if the host is empty or only contains a port
			if u.Host == "" || strings.HasPrefix(u.Host, ":") {
				return
			}
			if len(u.Query()) > 0 {
				return
			}
		}
		s.baseURL = strings.TrimSuffix(baseURL, "/")
	}
}

// WithStaticBasePath adds a new option for setting a static base path
func WithStaticBasePath(basePath string) SSEOption {
	return func(s *SSEServer) {
		s.basePath = normalizeURLPath(basePath)
	}
}

// WithBasePath adds a new option for setting a static base path.
//
// Deprecated: Use WithStaticBasePath instead. This will be removed in a future version.
//
//go:deprecated
func WithBasePath(basePath string) SSEOption {
	return WithStaticBasePath(basePath)
}

// WithDynamicBasePath accepts a function for generating the base path. This is
// useful for cases where the base path is not known at the time of SSE server
// creation, such as when using a reverse proxy or when the server is mounted
// at a dynamic path.
func WithDynamicBasePath(fn DynamicBasePathFunc) SSEOption {
	return func(s *SSEServer) {
		if fn != nil {
			s.dynamicBasePathFunc = func(r *http.Request, sid string) string {
				bp := fn(r, sid)
				return normalizeURLPath(bp)
			}
		}
	}
}

// WithMessageEndpoint sets the message endpoint path
func WithMessageEndpoint(endpoint string) SSEOption {
	return func(s *SSEServer) {
		s.messageEndpoint = endpoint
	}
}

// WithAppendQueryToMessageEndpoint configures the SSE server to append the original request's
// query parameters to the message endpoint URL that is sent to clients during the SSE connection
// initialization. This is useful when you need to preserve query parameters from the initial
// SSE connection request and carry them over to subsequent message requests, maintaining
// context or authentication details across the communication channel.
func WithAppendQueryToMessageEndpoint() SSEOption {
	return func(s *SSEServer) {
		s.appendQueryToMessageEndpoint = true
	}
}

// WithUseFullURLForMessageEndpoint controls whether the SSE server returns a complete URL (including baseURL)
// or just the path portion for the message endpoint. Set to false when clients will concatenate
// the baseURL themselves to avoid malformed URLs like "http://localhost/mcphttp://localhost/mcp/message".
func WithUseFullURLForMessageEndpoint(useFullURLForMessageEndpoint bool) SSEOption {
	return func(s *SSEServer) {
		s.useFullURLForMessageEndpoint = useFullURLForMessageEndpoint
	}
}

// WithSSEEndpoint sets the SSE endpoint path
func WithSSEEndpoint(endpoint string) SSEOption {
	return func(s *SSEServer) {
		s.sseEndpoint = endpoint
	}
}

// WithHTTPServer sets the HTTP server instance.
// NOTE: When providing a custom HTTP server, you must handle routing yourself
// If routing is not set up, the server will start but won't handle any MCP requests.
func WithHTTPServer(srv *http.Server) SSEOption {
	return func(s *SSEServer) {
		s.srv = srv
	}
}

func WithKeepAliveInterval(keepAliveInterval time.Duration) SSEOption {
	return func(s *SSEServer) {
		s.keepAlive = true
		s.keepAliveInterval = keepAliveInterval
	}
}

func WithKeepAlive(keepAlive bool) SSEOption {
	return func(s *SSEServer) {
		s.keepAlive = keepAlive
	}
}

// WithSSEContextFunc sets a function that will be called to customise the context
// to the server using the incoming request.
func WithSSEContextFunc(fn SSEContextFunc) SSEOption {
	return func(s *SSEServer) {
		s.contextFunc = fn
	}
}

// NewSSEServer creates a new SSE server instance with the given MCP server and options.
func NewSSEServer(server *MCPServer, opts ...SSEOption) *SSEServer {
	s := &SSEServer{
		server:                       server,
		sseEndpoint:                  "/sse",
		messageEndpoint:              "/message",
		useFullURLForMessageEndpoint: true,
		keepAlive:                    false,
		keepAliveInterval:            10 * time.Second,
	}

	// Apply all options
	for _, opt := range opts {
		opt(s)
	}

	return s
}

// NewTestServer creates a test server for testing purposes
func NewTestServer(server *MCPServer, opts ...SSEOption) *httptest.Server {
	sseServer := NewSSEServer(server, opts...)

	testServer := httptest.NewServer(sseServer)
	sseServer.baseURL = testServer.URL
	return testServer
}

// Start begins serving SSE connections on the specified address.
// It sets up HTTP handlers for SSE and message endpoints.
func (s *SSEServer) Start(addr string) error {
	s.mu.Lock()
	if s.srv == nil {
		s.srv = &http.Server{
			Addr:    addr,
			Handler: s,
		}
	} else {
		if s.srv.Addr == "" {
			s.srv.Addr = addr
		} else if s.srv.Addr != addr {
			return fmt.Errorf("conflicting listen address: WithHTTPServer(%q) vs Start(%q)", s.srv.Addr, addr)
		}
	}
	srv := s.srv
	s.mu.Unlock()

	return srv.ListenAndServe()
}

// Shutdown gracefully stops the SSE server, closing all active sessions
// and shutting down the HTTP server.
func (s *SSEServer) Shutdown(ctx context.Context) error {
	s.mu.RLock()
	srv := s.srv
	s.mu.RUnlock()

	if srv != nil {
		s.sessions.Range(func(key, value any) bool {
			if session, ok := value.(*sseSession); ok {
				close(session.done)
			}
			s.sessions.Delete(key)
			return true
		})

		return srv.Shutdown(ctx)
	}
	return nil
}

// handleSSE handles incoming SSE connection requests.
// It sets up appropriate headers and creates a new session for the client.
func (s *SSEServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	sessionID := uuid.New().String()
	session := &sseSession{
		done:                make(chan struct{}),
		eventQueue:          make(chan string, 100), // Buffer for events
		sessionID:           sessionID,
		notificationChannel: make(chan mcp.JSONRPCNotification, 100),
	}

	s.sessions.Store(sessionID, session)
	defer s.sessions.Delete(sessionID)

	if err := s.server.RegisterSession(r.Context(), session); err != nil {
		http.Error(
			w,
			fmt.Sprintf("Session registration failed: %v", err),
			http.StatusInternalServerError,
		)
		return
	}
	defer s.server.UnregisterSession(r.Context(), sessionID)

	// Start notification handler for this session
	go func() {
		for {
			select {
			case notification := <-session.notificationChannel:
				eventData, err := json.Marshal(notification)
				if err == nil {
					select {
					case session.eventQueue <- fmt.Sprintf("event: message\ndata: %s\n\n", eventData):
						// Event queued successfully
					case <-session.done:
						return
					}
				}
			case <-session.done:
				return
			case <-r.Context().Done():
				return
			}
		}
	}()

	// Start keep alive : ping
	if s.keepAlive {
		go func() {
			ticker := time.NewTicker(s.keepAliveInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					message := mcp.JSONRPCRequest{
						JSONRPC: "2.0",
						ID:      mcp.NewRequestId(session.requestID.Add(1)),
						Request: mcp.Request{
							Method: "ping",
						},
					}
					messageBytes, _ := json.Marshal(message)
					pingMsg := fmt.Sprintf("event: message\ndata:%s\n\n", messageBytes)
					select {
					case session.eventQueue <- pingMsg:
						// Message sent successfully
					case <-session.done:
						return
					}
				case <-session.done:
					return
				case <-r.Context().Done():
					return
				}
			}
		}()
	}

	// Send the initial endpoint event
	endpoint := s.GetMessageEndpointForClient(r, sessionID)
	if s.appendQueryToMessageEndpoint && len(r.URL.RawQuery) > 0 {
		endpoint += "&" + r.URL.RawQuery
	}
	fmt.Fprintf(w, "event: endpoint\ndata: %s\r\n\r\n", endpoint)
	flusher.Flush()

	// Main event loop - this runs in the HTTP handler goroutine
	for {
		select {
		case event := <-session.eventQueue:
			// Write the event to the response
			fmt.Fprint(w, event)
			flusher.Flush()
		case <-r.Context().Done():
			close(session.done)
			return
		case <-session.done:
			return
		}
	}
}

// GetMessageEndpointForClient returns the appropriate message endpoint URL with session ID
// for the given request. This is the canonical way to compute the message endpoint for a client.
// It handles both dynamic and static path modes, and honors the WithUseFullURLForMessageEndpoint flag.
func (s *SSEServer) GetMessageEndpointForClient(r *http.Request, sessionID string) string {
	basePath := s.basePath
	if s.dynamicBasePathFunc != nil {
		basePath = s.dynamicBasePathFunc(r, sessionID)
	}

	endpointPath := normalizeURLPath(basePath, s.messageEndpoint)
	if s.useFullURLForMessageEndpoint && s.baseURL != "" {
		endpointPath = s.baseURL + endpointPath
	}

	return fmt.Sprintf("%s?sessionId=%s", endpointPath, sessionID)
}

// handleMessage processes incoming JSON-RPC messages from clients and sends responses
// back through the SSE connection and 202 code to HTTP response.
func (s *SSEServer) handleMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSONRPCError(w, nil, mcp.INVALID_REQUEST, "Method not allowed")
		return
	}

	sessionID := r.URL.Query().Get("sessionId")
	if sessionID == "" {
		s.writeJSONRPCError(w, nil, mcp.INVALID_PARAMS, "Missing sessionId")
		return
	}
	sessionI, ok := s.sessions.Load(sessionID)
	if !ok {
		s.writeJSONRPCError(w, nil, mcp.INVALID_PARAMS, "Invalid session ID")
		return
	}
	session := sessionI.(*sseSession)

	// Set the client context before handling the message
	ctx := s.server.WithContext(r.Context(), session)
	if s.contextFunc != nil {
		ctx = s.contextFunc(ctx, r)
	}

	// Parse message as raw JSON
	var rawMessage json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&rawMessage); err != nil {
		s.writeJSONRPCError(w, nil, mcp.PARSE_ERROR, "Parse error")
		return
	}

	// Create a context that preserves all values from parent ctx but won't be canceled when the parent is canceled.
	// this is required because the http ctx will be canceled when the client disconnects
	detachedCtx := context.WithoutCancel(ctx)

	// quick return request, send 202 Accepted with no body, then deal the message and sent response via SSE
	w.WriteHeader(http.StatusAccepted)

	// Create a new context for handling the message that will be canceled when the message handling is done
	messageCtx, cancel := context.WithCancel(detachedCtx)

	go func(ctx context.Context) {
		defer cancel()
		// Use the context that will be canceled when session is done
		// Process message through MCPServer
		response := s.server.HandleMessage(ctx, rawMessage)
		// Only send response if there is one (not for notifications)
		if response != nil {
			var message string
			if eventData, err := json.Marshal(response); err != nil {
				// If there is an error marshalling the response, send a generic error response
				log.Printf("failed to marshal response: %v", err)
				message = "event: message\ndata: {\"error\": \"internal error\",\"jsonrpc\": \"2.0\", \"id\": null}\n\n"
			} else {
				message = fmt.Sprintf("event: message\ndata: %s\n\n", eventData)
			}

			// Queue the event for sending via SSE
			select {
			case session.eventQueue <- message:
				// Event queued successfully
			case <-session.done:
				// Session is closed, don't try to queue
			default:
				// Queue is full, log this situation
				log.Printf("Event queue full for session %s", sessionID)
			}
		}
	}(messageCtx)
}

// writeJSONRPCError writes a JSON-RPC error response with the given error details.
func (s *SSEServer) writeJSONRPCError(
	w http.ResponseWriter,
	id any,
	code int,
	message string,
) {
	response := createErrorResponse(id, code, message)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(
			w,
			fmt.Sprintf("Failed to encode response: %v", err),
			http.StatusInternalServerError,
		)
		return
	}
}

// SendEventToSession sends an event to a specific SSE session identified by sessionID.
// Returns an error if the session is not found or closed.
func (s *SSEServer) SendEventToSession(
	sessionID string,
	event any,
) error {
	sessionI, ok := s.sessions.Load(sessionID)
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	session := sessionI.(*sseSession)

	eventData, err := json.Marshal(event)
	if err != nil {
		return err
	}

	// Queue the event for sending via SSE
	select {
	case session.eventQueue <- fmt.Sprintf("event: message\ndata: %s\n\n", eventData):
		return nil
	case <-session.done:
		return fmt.Errorf("session closed")
	default:
		return fmt.Errorf("event queue full")
	}
}

func (s *SSEServer) GetUrlPath(input string) (string, error) {
	parse, err := url.Parse(input)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL %s: %w", input, err)
	}
	return parse.Path, nil
}

func (s *SSEServer) CompleteSseEndpoint() (string, error) {
	if s.dynamicBasePathFunc != nil {
		return "", &ErrDynamicPathConfig{Method: "CompleteSseEndpoint"}
	}

	path := normalizeURLPath(s.basePath, s.sseEndpoint)
	return s.baseURL + path, nil
}

func (s *SSEServer) CompleteSsePath() string {
	path, err := s.CompleteSseEndpoint()
	if err != nil {
		return normalizeURLPath(s.basePath, s.sseEndpoint)
	}
	urlPath, err := s.GetUrlPath(path)
	if err != nil {
		return normalizeURLPath(s.basePath, s.sseEndpoint)
	}
	return urlPath
}

func (s *SSEServer) CompleteMessageEndpoint() (string, error) {
	if s.dynamicBasePathFunc != nil {
		return "", &ErrDynamicPathConfig{Method: "CompleteMessageEndpoint"}
	}
	path := normalizeURLPath(s.basePath, s.messageEndpoint)
	return s.baseURL + path, nil
}

func (s *SSEServer) CompleteMessagePath() string {
	path, err := s.CompleteMessageEndpoint()
	if err != nil {
		return normalizeURLPath(s.basePath, s.messageEndpoint)
	}
	urlPath, err := s.GetUrlPath(path)
	if err != nil {
		return normalizeURLPath(s.basePath, s.messageEndpoint)
	}
	return urlPath
}

// SSEHandler returns an http.Handler for the SSE endpoint.
//
// This method allows you to mount the SSE handler at any arbitrary path
// using your own router (e.g. net/http, gorilla/mux, chi, etc.). It is
// intended for advanced scenarios where you want to control the routing or
// support dynamic segments.
//
// IMPORTANT: When using this handler in advanced/dynamic mounting scenarios,
// you must use the WithDynamicBasePath option to ensure the correct base path
// is communicated to clients.
//
// Example usage:
//
//	// Advanced/dynamic:
//	sseServer := NewSSEServer(mcpServer,
//		WithDynamicBasePath(func(r *http.Request, sessionID string) string {
//			tenant := r.PathValue("tenant")
//			return "/mcp/" + tenant
//		}),
//		WithBaseURL("http://localhost:8080")
//	)
//	mux.Handle("/mcp/{tenant}/sse", sseServer.SSEHandler())
//	mux.Handle("/mcp/{tenant}/message", sseServer.MessageHandler())
//
// For non-dynamic cases, use ServeHTTP method instead.
func (s *SSEServer) SSEHandler() http.Handler {
	return http.HandlerFunc(s.handleSSE)
}

// MessageHandler returns an http.Handler for the message endpoint.
//
// This method allows you to mount the message handler at any arbitrary path
// using your own router (e.g. net/http, gorilla/mux, chi, etc.). It is
// intended for advanced scenarios where you want to control the routing or
// support dynamic segments.
//
// IMPORTANT: When using this handler in advanced/dynamic mounting scenarios,
// you must use the WithDynamicBasePath option to ensure the correct base path
// is communicated to clients.
//
// Example usage:
//
//	// Advanced/dynamic:
//	sseServer := NewSSEServer(mcpServer,
//		WithDynamicBasePath(func(r *http.Request, sessionID string) string {
//			tenant := r.PathValue("tenant")
//			return "/mcp/" + tenant
//		}),
//		WithBaseURL("http://localhost:8080")
//	)
//	mux.Handle("/mcp/{tenant}/sse", sseServer.SSEHandler())
//	mux.Handle("/mcp/{tenant}/message", sseServer.MessageHandler())
//
// For non-dynamic cases, use ServeHTTP method instead.
func (s *SSEServer) MessageHandler() http.Handler {
	return http.HandlerFunc(s.handleMessage)
}

// ServeHTTP implements the http.Handler interface.
func (s *SSEServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.dynamicBasePathFunc != nil {
		http.Error(
			w,
			(&ErrDynamicPathConfig{Method: "ServeHTTP"}).Error(),
			http.StatusInternalServerError,
		)
		return
	}
	path := r.URL.Path
	// Use exact path matching rather than Contains
	ssePath := s.CompleteSsePath()
	if ssePath != "" && path == ssePath {
		s.handleSSE(w, r)
		return
	}
	messagePath := s.CompleteMessagePath()
	if messagePath != "" && path == messagePath {
		s.handleMessage(w, r)
		return
	}

	http.NotFound(w, r)
}

// normalizeURLPath joins path elements like path.Join but ensures the
// result always starts with a leading slash and never ends with a slash
func normalizeURLPath(elem ...string) string {
	joined := path.Join(elem...)

	// Ensure leading slash
	if !strings.HasPrefix(joined, "/") {
		joined = "/" + joined
	}

	// Remove trailing slash if not just "/"
	if len(joined) > 1 && strings.HasSuffix(joined, "/") {
		joined = joined[:len(joined)-1]
	}

	return joined
}
