package server

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// ClientSession represents an active session that can be used by MCPServer to interact with client.
type ClientSession interface {
	// Initialize marks session as fully initialized and ready for notifications
	Initialize()
	// Initialized returns if session is ready to accept notifications
	Initialized() bool
	// NotificationChannel provides a channel suitable for sending notifications to client.
	NotificationChannel() chan<- mcp.JSONRPCNotification
	// SessionID is a unique identifier used to track user session.
	SessionID() string
}

// SessionWithLogging is an extension of ClientSession that can receive log message notifications and set log level
type SessionWithLogging interface {
	ClientSession
	// SetLogLevel sets the minimum log level
	SetLogLevel(level mcp.LoggingLevel)
	// GetLogLevel retrieves the minimum log level
	GetLogLevel() mcp.LoggingLevel
}

// SessionWithTools is an extension of ClientSession that can store session-specific tool data
type SessionWithTools interface {
	ClientSession
	// GetSessionTools returns the tools specific to this session, if any
	// This method must be thread-safe for concurrent access
	GetSessionTools() map[string]ServerTool
	// SetSessionTools sets tools specific to this session
	// This method must be thread-safe for concurrent access
	SetSessionTools(tools map[string]ServerTool)
}

// SessionWithClientInfo is an extension of ClientSession that can store client info
type SessionWithClientInfo interface {
	ClientSession
	// GetClientInfo returns the client information for this session
	GetClientInfo() mcp.Implementation
	// SetClientInfo sets the client information for this session
	SetClientInfo(clientInfo mcp.Implementation)
}

// SessionWithStreamableHTTPConfig extends ClientSession to support streamable HTTP transport configurations
type SessionWithStreamableHTTPConfig interface {
	ClientSession
	// UpgradeToSSEWhenReceiveNotification upgrades the client-server communication to SSE stream when the server
	// sends notifications to the client
	//
	// The protocol specification:
	// - If the server response contains any JSON-RPC notifications, it MUST either:
	//   - Return Content-Type: text/event-stream to initiate an SSE stream, OR
	//   - Return Content-Type: application/json for a single JSON object
	// - The client MUST support both response types.
	//
	// Reference: https://modelcontextprotocol.io/specification/2025-03-26/basic/transports#sending-messages-to-the-server
	UpgradeToSSEWhenReceiveNotification()
}

// clientSessionKey is the context key for storing current client notification channel.
type clientSessionKey struct{}

// ClientSessionFromContext retrieves current client notification context from context.
func ClientSessionFromContext(ctx context.Context) ClientSession {
	if session, ok := ctx.Value(clientSessionKey{}).(ClientSession); ok {
		return session
	}
	return nil
}

// WithContext sets the current client session and returns the provided context
func (s *MCPServer) WithContext(
	ctx context.Context,
	session ClientSession,
) context.Context {
	return context.WithValue(ctx, clientSessionKey{}, session)
}

// RegisterSession saves session that should be notified in case if some server attributes changed.
func (s *MCPServer) RegisterSession(
	ctx context.Context,
	session ClientSession,
) error {
	sessionID := session.SessionID()
	if _, exists := s.sessions.LoadOrStore(sessionID, session); exists {
		return ErrSessionExists
	}
	s.hooks.RegisterSession(ctx, session)
	return nil
}

// UnregisterSession removes from storage session that is shut down.
func (s *MCPServer) UnregisterSession(
	ctx context.Context,
	sessionID string,
) {
	sessionValue, ok := s.sessions.LoadAndDelete(sessionID)
	if !ok {
		return
	}
	if session, ok := sessionValue.(ClientSession); ok {
		s.hooks.UnregisterSession(ctx, session)
	}
}

// SendNotificationToAllClients sends a notification to all the currently active clients.
func (s *MCPServer) SendNotificationToAllClients(
	method string,
	params map[string]any,
) {
	notification := mcp.JSONRPCNotification{
		JSONRPC: mcp.JSONRPC_VERSION,
		Notification: mcp.Notification{
			Method: method,
			Params: mcp.NotificationParams{
				AdditionalFields: params,
			},
		},
	}

	s.sessions.Range(func(k, v any) bool {
		if session, ok := v.(ClientSession); ok && session.Initialized() {
			select {
			case session.NotificationChannel() <- notification:
				// Successfully sent notification
			default:
				// Channel is blocked, if there's an error hook, use it
				if s.hooks != nil && len(s.hooks.OnError) > 0 {
					err := ErrNotificationChannelBlocked
					// Copy hooks pointer to local variable to avoid race condition
					hooks := s.hooks
					go func(sessionID string, hooks *Hooks) {
						ctx := context.Background()
						// Use the error hook to report the blocked channel
						hooks.onError(ctx, nil, "notification", map[string]any{
							"method":    method,
							"sessionID": sessionID,
						}, fmt.Errorf("notification channel blocked for session %s: %w", sessionID, err))
					}(session.SessionID(), hooks)
				}
			}
		}
		return true
	})
}

// SendNotificationToClient sends a notification to the current client
func (s *MCPServer) SendNotificationToClient(
	ctx context.Context,
	method string,
	params map[string]any,
) error {
	session := ClientSessionFromContext(ctx)
	if session == nil || !session.Initialized() {
		return ErrNotificationNotInitialized
	}

	// upgrades the client-server communication to SSE stream when the server sends notifications to the client
	if sessionWithStreamableHTTPConfig, ok := session.(SessionWithStreamableHTTPConfig); ok {
		sessionWithStreamableHTTPConfig.UpgradeToSSEWhenReceiveNotification()
	}

	notification := mcp.JSONRPCNotification{
		JSONRPC: mcp.JSONRPC_VERSION,
		Notification: mcp.Notification{
			Method: method,
			Params: mcp.NotificationParams{
				AdditionalFields: params,
			},
		},
	}

	select {
	case session.NotificationChannel() <- notification:
		return nil
	default:
		// Channel is blocked, if there's an error hook, use it
		if s.hooks != nil && len(s.hooks.OnError) > 0 {
			err := ErrNotificationChannelBlocked
			// Copy hooks pointer to local variable to avoid race condition
			hooks := s.hooks
			go func(sessionID string, hooks *Hooks) {
				// Use the error hook to report the blocked channel
				hooks.onError(ctx, nil, "notification", map[string]any{
					"method":    method,
					"sessionID": sessionID,
				}, fmt.Errorf("notification channel blocked for session %s: %w", sessionID, err))
			}(session.SessionID(), hooks)
		}
		return ErrNotificationChannelBlocked
	}
}

// SendNotificationToSpecificClient sends a notification to a specific client by session ID
func (s *MCPServer) SendNotificationToSpecificClient(
	sessionID string,
	method string,
	params map[string]any,
) error {
	sessionValue, ok := s.sessions.Load(sessionID)
	if !ok {
		return ErrSessionNotFound
	}

	session, ok := sessionValue.(ClientSession)
	if !ok || !session.Initialized() {
		return ErrSessionNotInitialized
	}

	// upgrades the client-server communication to SSE stream when the server sends notifications to the client
	if sessionWithStreamableHTTPConfig, ok := session.(SessionWithStreamableHTTPConfig); ok {
		sessionWithStreamableHTTPConfig.UpgradeToSSEWhenReceiveNotification()
	}

	notification := mcp.JSONRPCNotification{
		JSONRPC: mcp.JSONRPC_VERSION,
		Notification: mcp.Notification{
			Method: method,
			Params: mcp.NotificationParams{
				AdditionalFields: params,
			},
		},
	}

	select {
	case session.NotificationChannel() <- notification:
		return nil
	default:
		// Channel is blocked, if there's an error hook, use it
		if s.hooks != nil && len(s.hooks.OnError) > 0 {
			err := ErrNotificationChannelBlocked
			ctx := context.Background()
			// Copy hooks pointer to local variable to avoid race condition
			hooks := s.hooks
			go func(sID string, hooks *Hooks) {
				// Use the error hook to report the blocked channel
				hooks.onError(ctx, nil, "notification", map[string]any{
					"method":    method,
					"sessionID": sID,
				}, fmt.Errorf("notification channel blocked for session %s: %w", sID, err))
			}(sessionID, hooks)
		}
		return ErrNotificationChannelBlocked
	}
}

// AddSessionTool adds a tool for a specific session
func (s *MCPServer) AddSessionTool(sessionID string, tool mcp.Tool, handler ToolHandlerFunc) error {
	return s.AddSessionTools(sessionID, ServerTool{Tool: tool, Handler: handler})
}

// AddSessionTools adds tools for a specific session
func (s *MCPServer) AddSessionTools(sessionID string, tools ...ServerTool) error {
	sessionValue, ok := s.sessions.Load(sessionID)
	if !ok {
		return ErrSessionNotFound
	}

	session, ok := sessionValue.(SessionWithTools)
	if !ok {
		return ErrSessionDoesNotSupportTools
	}

	s.implicitlyRegisterToolCapabilities()

	// Get existing tools (this should return a thread-safe copy)
	sessionTools := session.GetSessionTools()

	// Create a new map to avoid concurrent modification issues
	newSessionTools := make(map[string]ServerTool, len(sessionTools)+len(tools))

	// Copy existing tools
	for k, v := range sessionTools {
		newSessionTools[k] = v
	}

	// Add new tools
	for _, tool := range tools {
		newSessionTools[tool.Tool.Name] = tool
	}

	// Set the tools (this should be thread-safe)
	session.SetSessionTools(newSessionTools)

	// It only makes sense to send tool notifications to initialized sessions --
	// if we're not initialized yet the client can't possibly have sent their
	// initial tools/list message.
	//
	// For initialized sessions, honor tools.listChanged, which is specifically
	// about whether notifications will be sent or not.
	// see <https://modelcontextprotocol.io/specification/2025-03-26/server/tools#capabilities>
	if session.Initialized() && s.capabilities.tools != nil && s.capabilities.tools.listChanged {
		// Send notification only to this session
		if err := s.SendNotificationToSpecificClient(sessionID, "notifications/tools/list_changed", nil); err != nil {
			// Log the error but don't fail the operation
			// The tools were successfully added, but notification failed
			if s.hooks != nil && len(s.hooks.OnError) > 0 {
				hooks := s.hooks
				go func(sID string, hooks *Hooks) {
					ctx := context.Background()
					hooks.onError(ctx, nil, "notification", map[string]any{
						"method":    "notifications/tools/list_changed",
						"sessionID": sID,
					}, fmt.Errorf("failed to send notification after adding tools: %w", err))
				}(sessionID, hooks)
			}
		}
	}

	return nil
}

// DeleteSessionTools removes tools from a specific session
func (s *MCPServer) DeleteSessionTools(sessionID string, names ...string) error {
	sessionValue, ok := s.sessions.Load(sessionID)
	if !ok {
		return ErrSessionNotFound
	}

	session, ok := sessionValue.(SessionWithTools)
	if !ok {
		return ErrSessionDoesNotSupportTools
	}

	// Get existing tools (this should return a thread-safe copy)
	sessionTools := session.GetSessionTools()
	if sessionTools == nil {
		return nil
	}

	// Create a new map to avoid concurrent modification issues
	newSessionTools := make(map[string]ServerTool, len(sessionTools))

	// Copy existing tools except those being deleted
	for k, v := range sessionTools {
		newSessionTools[k] = v
	}

	// Remove specified tools
	for _, name := range names {
		delete(newSessionTools, name)
	}

	// Set the tools (this should be thread-safe)
	session.SetSessionTools(newSessionTools)

	// It only makes sense to send tool notifications to initialized sessions --
	// if we're not initialized yet the client can't possibly have sent their
	// initial tools/list message.
	//
	// For initialized sessions, honor tools.listChanged, which is specifically
	// about whether notifications will be sent or not.
	// see <https://modelcontextprotocol.io/specification/2025-03-26/server/tools#capabilities>
	if session.Initialized() && s.capabilities.tools != nil && s.capabilities.tools.listChanged {
		// Send notification only to this session
		if err := s.SendNotificationToSpecificClient(sessionID, "notifications/tools/list_changed", nil); err != nil {
			// Log the error but don't fail the operation
			// The tools were successfully deleted, but notification failed
			if s.hooks != nil && len(s.hooks.OnError) > 0 {
				hooks := s.hooks
				go func(sID string, hooks *Hooks) {
					ctx := context.Background()
					hooks.onError(ctx, nil, "notification", map[string]any{
						"method":    "notifications/tools/list_changed",
						"sessionID": sID,
					}, fmt.Errorf("failed to send notification after deleting tools: %w", err))
				}(sessionID, hooks)
			}
		}
	}

	return nil
}
