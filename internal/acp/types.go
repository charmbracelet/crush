// Package acp implements the Agent Client Protocol (ACP) server.
// ACP is a JSON-RPC 2.0 protocol over stdio that allows editors/IDEs to
// communicate with AI agents in a standardized way.
package acp

import "encoding/json"

// Protocol version supported.
const ProtocolVersion = 1

// ---- Shared types ----

// ContentBlock is a union type for prompt content.
type ContentBlock struct {
	Type string `json:"type"`
	// text
	Text string `json:"text,omitempty"`
	// image / audio
	Data     string `json:"data,omitempty"`
	MIMEType string `json:"mimeType,omitempty"`
	URI      string `json:"uri,omitempty"`
}

// StopReason is the reason a prompt turn stopped.
type StopReason string

const (
	StopReasonEndTurn         StopReason = "end_turn"
	StopReasonMaxTokens       StopReason = "max_tokens"
	StopReasonMaxTurnRequests StopReason = "max_turn_requests"
	StopReasonRefusal         StopReason = "refusal"
	StopReasonCancelled       StopReason = "cancelled"
)

// ---- Initialize ----

// ClientCapabilities describes what the connecting client supports.
type ClientCapabilities struct {
	FS *FSCapabilities `json:"fs,omitempty"`
	// Terminal support.
	Terminal bool `json:"terminal,omitempty"`
}

// FSCapabilities lists file-system operations the client can handle.
type FSCapabilities struct {
	ReadTextFile  bool `json:"readTextFile,omitempty"`
	WriteTextFile bool `json:"writeTextFile,omitempty"`
}

// ClientInfo identifies the connecting client.
type ClientInfo struct {
	Name    string `json:"name"`
	Title   string `json:"title,omitempty"`
	Version string `json:"version,omitempty"`
}

// InitializeParams is the request sent by the client to start a session.
type InitializeParams struct {
	ProtocolVersion    int                `json:"protocolVersion"`
	ClientCapabilities ClientCapabilities `json:"clientCapabilities"`
	ClientInfo         ClientInfo         `json:"clientInfo"`
}

// AgentCapabilities describes what this agent supports.
type AgentCapabilities struct {
	LoadSession        bool                `json:"loadSession,omitempty"`
	PromptCapabilities *PromptCapabilities `json:"promptCapabilities,omitempty"`
	MCP                *MCPCapabilities    `json:"mcp,omitempty"`
}

// PromptCapabilities lists content types the agent accepts.
type PromptCapabilities struct {
	Image           bool `json:"image,omitempty"`
	Audio           bool `json:"audio,omitempty"`
	EmbeddedContext bool `json:"embeddedContext,omitempty"`
}

// MCPCapabilities describes MCP transport support.
type MCPCapabilities struct {
	HTTP bool `json:"http,omitempty"`
	SSE  bool `json:"sse,omitempty"`
}

// AgentInfo identifies this agent.
type AgentInfo struct {
	Name    string `json:"name"`
	Title   string `json:"title,omitempty"`
	Version string `json:"version,omitempty"`
}

// InitializeResult is the response to an initialize request.
type InitializeResult struct {
	ProtocolVersion   int               `json:"protocolVersion"`
	AgentCapabilities AgentCapabilities `json:"agentCapabilities"`
	AgentInfo         AgentInfo         `json:"agentInfo"`
	AuthMethods       []string          `json:"authMethods"`
}

// ---- Session setup ----

// MCPServerConfig describes an MCP server to connect to.
type MCPServerConfig struct {
	Name    string            `json:"name"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     []string          `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
	Type    string            `json:"type,omitempty"` // "stdio", "http", "sse"
	Headers map[string]string `json:"headers,omitempty"`
}

// SessionNewParams is the request to create a new session.
type SessionNewParams struct {
	CWD        string            `json:"cwd,omitempty"`
	MCPServers []MCPServerConfig `json:"mcpServers,omitempty"`
}

// SessionNewResult is the response after creating a new session.
type SessionNewResult struct {
	SessionID string `json:"sessionId"`
}

// SessionLoadParams is the request to load an existing session.
type SessionLoadParams struct {
	SessionID string `json:"sessionId"`
}

// ---- Prompt ----

// PromptParams is the request to send a prompt to the agent.
type PromptParams struct {
	SessionID string         `json:"sessionId"`
	Prompt    []ContentBlock `json:"prompt"`
}

// PromptResult is the response after a prompt turn completes.
type PromptResult struct {
	StopReason StopReason `json:"stopReason"`
}

// ---- Session cancel ----

// SessionCancelParams is the request to cancel an in-progress prompt turn.
type SessionCancelParams struct {
	SessionID string `json:"sessionId"`
}

// ---- session/update notification ----

// SessionUpdateType identifies the kind of session update.
type SessionUpdateType string

const (
	SessionUpdateUserMessageChunk  SessionUpdateType = "user_message_chunk"
	SessionUpdateAgentMessageChunk SessionUpdateType = "agent_message_chunk"
	SessionUpdateAgentThoughtChunk SessionUpdateType = "agent_thought_chunk"
	SessionUpdateToolCall          SessionUpdateType = "tool_call"
	SessionUpdateToolCallUpdate    SessionUpdateType = "tool_call_update"
	SessionUpdatePlan              SessionUpdateType = "plan"
	SessionUpdateSessionInfoUpdate SessionUpdateType = "session_info_update"
)

// ToolCallStatus describes the execution state of a tool call.
type ToolCallStatus string

const (
	ToolCallStatusRunning   ToolCallStatus = "running"
	ToolCallStatusCompleted ToolCallStatus = "completed"
	ToolCallStatusError     ToolCallStatus = "error"
)

// SessionUpdate is the payload of a session/update notification.
// The SessionUpdate field identifies the variant; remaining fields are
// populated based on that variant.
//
// Both tool_call and session_info_update variants use a "title" field in the
// wire format; we use a single Title field and populate it for both.
type SessionUpdate struct {
	SessionUpdate SessionUpdateType `json:"sessionUpdate"`
	// agent_message_chunk / agent_thought_chunk / user_message_chunk
	Content string `json:"content,omitempty"`
	// tool_call / tool_call_update / session_info_update
	Title string `json:"title,omitempty"`
	// tool_call / tool_call_update
	ToolCallID string         `json:"toolCallId,omitempty"`
	Kind       string         `json:"kind,omitempty"`
	Status     ToolCallStatus `json:"status,omitempty"`
	RawInput   any            `json:"rawInput,omitempty"`
	RawOutput  any            `json:"rawOutput,omitempty"`
	// plan
	Entries []PlanEntry `json:"entries,omitempty"`
	// session_info_update
	UpdatedAt int64 `json:"updatedAt,omitempty"`
}

// PlanEntry is a single entry in an agent execution plan.
type PlanEntry struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Status  string `json:"status"`
	Content string `json:"content,omitempty"`
}

// SessionUpdateNotification is the full notification envelope.
type SessionUpdateNotification struct {
	SessionID string        `json:"sessionId"`
	Update    SessionUpdate `json:"update"`
}

// ---- session/request_permission ----

// PermissionOptionKind indicates the kind of permission option.
type PermissionOptionKind string

const (
	PermissionOptionAllowOnce    PermissionOptionKind = "allow_once"
	PermissionOptionAllowAlways  PermissionOptionKind = "allow_always"
	PermissionOptionRejectOnce   PermissionOptionKind = "reject_once"
	PermissionOptionRejectAlways PermissionOptionKind = "reject_always"
)

// PermissionOption is a choice presented to the user for a permission request.
type PermissionOption struct {
	OptionID string               `json:"optionId"`
	Name     string               `json:"name"`
	Kind     PermissionOptionKind `json:"kind"`
}

// ACPToolCall describes the tool call requiring permission.
type ACPToolCall struct {
	ToolCallID string         `json:"toolCallId"`
	Title      string         `json:"title,omitempty"`
	Kind       string         `json:"kind,omitempty"`
	RawInput   any            `json:"rawInput,omitempty"`
	Status     ToolCallStatus `json:"status,omitempty"`
}

// RequestPermissionParams is sent by the agent to ask the client for approval.
type RequestPermissionParams struct {
	SessionID string             `json:"sessionId"`
	ToolCall  ACPToolCall        `json:"toolCall"`
	Options   []PermissionOption `json:"options"`
}

// RequestPermissionResult is the client's response to a permission request.
type RequestPermissionResult struct {
	Outcome  string `json:"outcome"` // "selected" | "cancelled"
	OptionID string `json:"optionId,omitempty"`
}

// ---- JSON-RPC 2.0 wire types ----

// Request is a JSON-RPC 2.0 request or notification message.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is a JSON-RPC 2.0 response message.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError is a JSON-RPC 2.0 error object.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Standard JSON-RPC error codes.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)
