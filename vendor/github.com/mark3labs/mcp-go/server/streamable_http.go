package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/util"
)

// StreamableHTTPOption defines a function type for configuring StreamableHTTPServer
type StreamableHTTPOption func(*StreamableHTTPServer)

// WithEndpointPath sets the endpoint path for the server.
// The default is "/mcp".
// It's only works for `Start` method. When used as a http.Handler, it has no effect.
func WithEndpointPath(endpointPath string) StreamableHTTPOption {
	return func(s *StreamableHTTPServer) {
		// Normalize the endpoint path to ensure it starts with a slash and doesn't end with one
		normalizedPath := "/" + strings.Trim(endpointPath, "/")
		s.endpointPath = normalizedPath
	}
}

// WithStateLess sets the server to stateless mode.
// If true, the server will manage no session information. Every request will be treated
// as a new session. No session id returned to the client.
// The default is false.
//
// Notice: This is a convenience method. It's identical to set WithSessionIdManager option
// to StatelessSessionIdManager.
func WithStateLess(stateLess bool) StreamableHTTPOption {
	return func(s *StreamableHTTPServer) {
		s.sessionIdManager = &StatelessSessionIdManager{}
	}
}

// WithSessionIdManager sets a custom session id generator for the server.
// By default, the server will use SimpleStatefulSessionIdGenerator, which generates
// session ids with uuid, and it's insecure.
// Notice: it will override the WithStateLess option.
func WithSessionIdManager(manager SessionIdManager) StreamableHTTPOption {
	return func(s *StreamableHTTPServer) {
		s.sessionIdManager = manager
	}
}

// WithHeartbeatInterval sets the heartbeat interval. Positive interval means the
// server will send a heartbeat to the client through the GET connection, to keep
// the connection alive from being closed by the network infrastructure (e.g.
// gateways). If the client does not establish a GET connection, it has no
// effect. The default is not to send heartbeats.
func WithHeartbeatInterval(interval time.Duration) StreamableHTTPOption {
	return func(s *StreamableHTTPServer) {
		s.listenHeartbeatInterval = interval
	}
}

// WithHTTPContextFunc sets a function that will be called to customise the context
// to the server using the incoming request.
// This can be used to inject context values from headers, for example.
func WithHTTPContextFunc(fn HTTPContextFunc) StreamableHTTPOption {
	return func(s *StreamableHTTPServer) {
		s.contextFunc = fn
	}
}

// WithStreamableHTTPServer sets the HTTP server instance for StreamableHTTPServer.
// NOTE: When providing a custom HTTP server, you must handle routing yourself
// If routing is not set up, the server will start but won't handle any MCP requests.
func WithStreamableHTTPServer(srv *http.Server) StreamableHTTPOption {
	return func(s *StreamableHTTPServer) {
		s.httpServer = srv
	}
}

// WithLogger sets the logger for the server
func WithLogger(logger util.Logger) StreamableHTTPOption {
	return func(s *StreamableHTTPServer) {
		s.logger = logger
	}
}

// StreamableHTTPServer implements a Streamable-http based MCP server.
// It communicates with clients over HTTP protocol, supporting both direct HTTP responses, and SSE streams.
// https://modelcontextprotocol.io/specification/2025-03-26/basic/transports#streamable-http
//
// Usage:
//
//	server := NewStreamableHTTPServer(mcpServer)
//	server.Start(":8080") // The final url for client is http://xxxx:8080/mcp by default
//
// or the server itself can be used as a http.Handler, which is convenient to
// integrate with existing http servers, or advanced usage:
//
//	handler := NewStreamableHTTPServer(mcpServer)
//	http.Handle("/streamable-http", handler)
//	http.ListenAndServe(":8080", nil)
//
// Notice:
// Except for the GET handlers(listening), the POST handlers(request/notification) will
// not trigger the session registration. So the methods like `SendNotificationToSpecificClient`
// or `hooks.onRegisterSession` will not be triggered for POST messages.
//
// The current implementation does not support the following features from the specification:
//   - Batching of requests/notifications/responses in arrays.
//   - Stream Resumability
type StreamableHTTPServer struct {
	server            *MCPServer
	sessionTools      *sessionToolsStore
	sessionRequestIDs sync.Map // sessionId --> last requestID(*atomic.Int64)

	httpServer *http.Server
	mu         sync.RWMutex

	endpointPath            string
	contextFunc             HTTPContextFunc
	sessionIdManager        SessionIdManager
	listenHeartbeatInterval time.Duration
	logger                  util.Logger
}

// NewStreamableHTTPServer creates a new streamable-http server instance
func NewStreamableHTTPServer(server *MCPServer, opts ...StreamableHTTPOption) *StreamableHTTPServer {
	s := &StreamableHTTPServer{
		server:           server,
		sessionTools:     newSessionToolsStore(),
		endpointPath:     "/mcp",
		sessionIdManager: &InsecureStatefulSessionIdManager{},
		logger:           util.DefaultLogger(),
	}

	// Apply all options
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// ServeHTTP implements the http.Handler interface.
func (s *StreamableHTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handlePost(w, r)
	case http.MethodGet:
		s.handleGet(w, r)
	case http.MethodDelete:
		s.handleDelete(w, r)
	default:
		http.NotFound(w, r)
	}
}

// Start begins serving the http server on the specified address and path
// (endpointPath). like:
//
//	s.Start(":8080")
func (s *StreamableHTTPServer) Start(addr string) error {
	s.mu.Lock()
	if s.httpServer == nil {
		mux := http.NewServeMux()
		mux.Handle(s.endpointPath, s)
		s.httpServer = &http.Server{
			Addr:    addr,
			Handler: mux,
		}
	} else {
		if s.httpServer.Addr == "" {
			s.httpServer.Addr = addr
		} else if s.httpServer.Addr != addr {
			return fmt.Errorf("conflicting listen address: WithStreamableHTTPServer(%q) vs Start(%q)", s.httpServer.Addr, addr)
		}
	}
	srv := s.httpServer
	s.mu.Unlock()

	return srv.ListenAndServe()
}

// Shutdown gracefully stops the server, closing all active sessions
// and shutting down the HTTP server.
func (s *StreamableHTTPServer) Shutdown(ctx context.Context) error {

	// shutdown the server if needed (may use as a http.Handler)
	s.mu.RLock()
	srv := s.httpServer
	s.mu.RUnlock()
	if srv != nil {
		return srv.Shutdown(ctx)
	}
	return nil
}

// --- internal methods ---

const (
	headerKeySessionID = "Mcp-Session-Id"
)

func (s *StreamableHTTPServer) handlePost(w http.ResponseWriter, r *http.Request) {
	// post request carry request/notification message

	// Check content type
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		http.Error(w, "Invalid content type: must be 'application/json'", http.StatusBadRequest)
		return
	}

	// Check the request body is valid json, meanwhile, get the request Method
	rawData, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeJSONRPCError(w, nil, mcp.PARSE_ERROR, fmt.Sprintf("read request body error: %v", err))
		return
	}
	var baseMessage struct {
		Method mcp.MCPMethod `json:"method"`
	}
	if err := json.Unmarshal(rawData, &baseMessage); err != nil {
		s.writeJSONRPCError(w, nil, mcp.PARSE_ERROR, "request body is not valid json")
		return
	}
	isInitializeRequest := baseMessage.Method == mcp.MethodInitialize

	// Prepare the session for the mcp server
	// The session is ephemeral. Its life is the same as the request. It's only created
	// for interaction with the mcp server.
	var sessionID string
	if isInitializeRequest {
		// generate a new one for initialize request
		sessionID = s.sessionIdManager.Generate()
	} else {
		// Get session ID from header.
		// Stateful servers need the client to carry the session ID.
		sessionID = r.Header.Get(headerKeySessionID)
		isTerminated, err := s.sessionIdManager.Validate(sessionID)
		if err != nil {
			http.Error(w, "Invalid session ID", http.StatusBadRequest)
			return
		}
		if isTerminated {
			http.Error(w, "Session terminated", http.StatusNotFound)
			return
		}
	}

	session := newStreamableHttpSession(sessionID, s.sessionTools)

	// Set the client context before handling the message
	ctx := s.server.WithContext(r.Context(), session)
	if s.contextFunc != nil {
		ctx = s.contextFunc(ctx, r)
	}

	// handle potential notifications
	mu := sync.Mutex{}
	upgradedHeader := false
	done := make(chan struct{})

	go func() {
		for {
			select {
			case nt := <-session.notificationChannel:
				func() {
					mu.Lock()
					defer mu.Unlock()
					// if the done chan is closed, as the request is terminated, just return
					select {
					case <-done:
						return
					default:
					}
					defer func() {
						flusher, ok := w.(http.Flusher)
						if ok {
							flusher.Flush()
						}
					}()

					// if there's notifications, upgradedHeader to SSE response
					if !upgradedHeader {
						w.Header().Set("Content-Type", "text/event-stream")
						w.Header().Set("Connection", "keep-alive")
						w.Header().Set("Cache-Control", "no-cache")
						w.WriteHeader(http.StatusAccepted)
						upgradedHeader = true
					}
					err := writeSSEEvent(w, nt)
					if err != nil {
						s.logger.Errorf("Failed to write SSE event: %v", err)
						return
					}
				}()
			case <-done:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	// Process message through MCPServer
	response := s.server.HandleMessage(ctx, rawData)
	if response == nil {
		// For notifications, just send 202 Accepted with no body
		w.WriteHeader(http.StatusAccepted)
		return
	}

	// Write response
	mu.Lock()
	defer mu.Unlock()
	// close the done chan before unlock
	defer close(done)
	if ctx.Err() != nil {
		return
	}
	// If client-server communication already upgraded to SSE stream
	if session.upgradeToSSE.Load() {
		if !upgradedHeader {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Connection", "keep-alive")
			w.Header().Set("Cache-Control", "no-cache")
			w.WriteHeader(http.StatusAccepted)
			upgradedHeader = true
		}
		if err := writeSSEEvent(w, response); err != nil {
			s.logger.Errorf("Failed to write final SSE response event: %v", err)
		}
	} else {
		w.Header().Set("Content-Type", "application/json")
		if isInitializeRequest && sessionID != "" {
			// send the session ID back to the client
			w.Header().Set(headerKeySessionID, sessionID)
		}
		w.WriteHeader(http.StatusOK)
		err := json.NewEncoder(w).Encode(response)
		if err != nil {
			s.logger.Errorf("Failed to write response: %v", err)
		}
	}
}

func (s *StreamableHTTPServer) handleGet(w http.ResponseWriter, r *http.Request) {
	// get request is for listening to notifications
	// https://modelcontextprotocol.io/specification/2025-03-26/basic/transports#listening-for-messages-from-the-server

	sessionID := r.Header.Get(headerKeySessionID)
	// the specification didn't say we should validate the session id

	if sessionID == "" {
		// It's a stateless server,
		// but the MCP server requires a unique ID for registering, so we use a random one
		sessionID = uuid.New().String()
	}

	session := newStreamableHttpSession(sessionID, s.sessionTools)
	if err := s.server.RegisterSession(r.Context(), session); err != nil {
		http.Error(w, fmt.Sprintf("Session registration failed: %v", err), http.StatusBadRequest)
		return
	}
	defer s.server.UnregisterSession(r.Context(), sessionID)

	// Set the client context before handling the message
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusAccepted)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}
	flusher.Flush()

	// Start notification handler for this session
	done := make(chan struct{})
	defer close(done)
	writeChan := make(chan any, 16)

	go func() {
		for {
			select {
			case nt := <-session.notificationChannel:
				select {
				case writeChan <- &nt:
				case <-done:
					return
				}
			case <-done:
				return
			}
		}
	}()

	if s.listenHeartbeatInterval > 0 {
		// heartbeat to keep the connection alive
		go func() {
			ticker := time.NewTicker(s.listenHeartbeatInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					message := mcp.JSONRPCRequest{
						JSONRPC: "2.0",
						ID:      mcp.NewRequestId(s.nextRequestID(sessionID)),
						Request: mcp.Request{
							Method: "ping",
						},
					}
					select {
					case writeChan <- message:
					case <-done:
						return
					}
				case <-done:
					return
				}
			}
		}()
	}

	// Keep the connection open until the client disconnects
	//
	// There's will a Available() check when handler ends, and it maybe race with Flush(),
	// so we use a separate channel to send the data, inteading of flushing directly in other goroutine.
	for {
		select {
		case data := <-writeChan:
			if data == nil {
				continue
			}
			if err := writeSSEEvent(w, data); err != nil {
				s.logger.Errorf("Failed to write SSE event: %v", err)
				return
			}
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (s *StreamableHTTPServer) handleDelete(w http.ResponseWriter, r *http.Request) {
	// delete request terminate the session
	sessionID := r.Header.Get(headerKeySessionID)
	notAllowed, err := s.sessionIdManager.Terminate(sessionID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Session termination failed: %v", err), http.StatusInternalServerError)
		return
	}
	if notAllowed {
		http.Error(w, "Session termination not allowed", http.StatusMethodNotAllowed)
		return
	}

	// remove the session relateddata from the sessionToolsStore
	s.sessionTools.delete(sessionID)

	// remove current session's requstID information
	s.sessionRequestIDs.Delete(sessionID)

	w.WriteHeader(http.StatusOK)
}

func writeSSEEvent(w io.Writer, data any) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}
	_, err = fmt.Fprintf(w, "event: message\ndata: %s\n\n", jsonData)
	if err != nil {
		return fmt.Errorf("failed to write SSE event: %w", err)
	}
	return nil
}

// writeJSONRPCError writes a JSON-RPC error response with the given error details.
func (s *StreamableHTTPServer) writeJSONRPCError(
	w http.ResponseWriter,
	id any,
	code int,
	message string,
) {
	response := createErrorResponse(id, code, message)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		s.logger.Errorf("Failed to write JSONRPCError: %v", err)
	}
}

// nextRequestID gets the next incrementing requestID for the current session
func (s *StreamableHTTPServer) nextRequestID(sessionID string) int64 {
	actual, _ := s.sessionRequestIDs.LoadOrStore(sessionID, new(atomic.Int64))
	counter := actual.(*atomic.Int64)
	return counter.Add(1)
}

// --- session ---

type sessionToolsStore struct {
	mu    sync.RWMutex
	tools map[string]map[string]ServerTool // sessionID -> toolName -> tool
}

func newSessionToolsStore() *sessionToolsStore {
	return &sessionToolsStore{
		tools: make(map[string]map[string]ServerTool),
	}
}

func (s *sessionToolsStore) get(sessionID string) map[string]ServerTool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tools[sessionID]
}

func (s *sessionToolsStore) set(sessionID string, tools map[string]ServerTool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[sessionID] = tools
}

func (s *sessionToolsStore) delete(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tools, sessionID)
}

// streamableHttpSession is a session for streamable-http transport
// When in POST handlers(request/notification), it's ephemeral, and only exists in the life of the request handler.
// When in GET handlers(listening), it's a real session, and will be registered in the MCP server.
type streamableHttpSession struct {
	sessionID           string
	notificationChannel chan mcp.JSONRPCNotification // server -> client notifications
	tools               *sessionToolsStore
	upgradeToSSE        atomic.Bool
}

func newStreamableHttpSession(sessionID string, toolStore *sessionToolsStore) *streamableHttpSession {
	return &streamableHttpSession{
		sessionID:           sessionID,
		notificationChannel: make(chan mcp.JSONRPCNotification, 100),
		tools:               toolStore,
	}
}

func (s *streamableHttpSession) SessionID() string {
	return s.sessionID
}

func (s *streamableHttpSession) NotificationChannel() chan<- mcp.JSONRPCNotification {
	return s.notificationChannel
}

func (s *streamableHttpSession) Initialize() {
	// do nothing
	// the session is ephemeral, no real initialized action needed
}

func (s *streamableHttpSession) Initialized() bool {
	// the session is ephemeral, no real initialized action needed
	return true
}

var _ ClientSession = (*streamableHttpSession)(nil)

func (s *streamableHttpSession) GetSessionTools() map[string]ServerTool {
	return s.tools.get(s.sessionID)
}

func (s *streamableHttpSession) SetSessionTools(tools map[string]ServerTool) {
	s.tools.set(s.sessionID, tools)
}

var _ SessionWithTools = (*streamableHttpSession)(nil)

func (s *streamableHttpSession) UpgradeToSSEWhenReceiveNotification() {
	s.upgradeToSSE.Store(true)
}

var _ SessionWithStreamableHTTPConfig = (*streamableHttpSession)(nil)

// --- session id manager ---

type SessionIdManager interface {
	Generate() string
	// Validate checks if a session ID is valid and not terminated.
	// Returns isTerminated=true if the ID is valid but belongs to a terminated session.
	// Returns err!=nil if the ID format is invalid or lookup failed.
	Validate(sessionID string) (isTerminated bool, err error)
	// Terminate marks a session ID as terminated.
	// Returns isNotAllowed=true if the server policy prevents client termination.
	// Returns err!=nil if the ID is invalid or termination failed.
	Terminate(sessionID string) (isNotAllowed bool, err error)
}

// StatelessSessionIdManager does nothing, which means it has no session management, which is stateless.
type StatelessSessionIdManager struct{}

func (s *StatelessSessionIdManager) Generate() string {
	return ""
}
func (s *StatelessSessionIdManager) Validate(sessionID string) (isTerminated bool, err error) {
	// In stateless mode, ignore session IDs completely - don't validate or reject them
	return false, nil
}
func (s *StatelessSessionIdManager) Terminate(sessionID string) (isNotAllowed bool, err error) {
	return false, nil
}

// InsecureStatefulSessionIdManager generate id with uuid
// It won't validate the id indeed, so it could be fake.
// For more secure session id, use a more complex generator, like a JWT.
type InsecureStatefulSessionIdManager struct{}

const idPrefix = "mcp-session-"

func (s *InsecureStatefulSessionIdManager) Generate() string {
	return idPrefix + uuid.New().String()
}
func (s *InsecureStatefulSessionIdManager) Validate(sessionID string) (isTerminated bool, err error) {
	// validate the session id is a valid uuid
	if !strings.HasPrefix(sessionID, idPrefix) {
		return false, fmt.Errorf("invalid session id: %s", sessionID)
	}
	if _, err := uuid.Parse(sessionID[len(idPrefix):]); err != nil {
		return false, fmt.Errorf("invalid session id: %s", sessionID)
	}
	return false, nil
}
func (s *InsecureStatefulSessionIdManager) Terminate(sessionID string) (isNotAllowed bool, err error) {
	return false, nil
}

// NewTestStreamableHTTPServer creates a test server for testing purposes
func NewTestStreamableHTTPServer(server *MCPServer, opts ...StreamableHTTPOption) *httptest.Server {
	sseServer := NewStreamableHTTPServer(server, opts...)
	testServer := httptest.NewServer(sseServer)
	return testServer
}
