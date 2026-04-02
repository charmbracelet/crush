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
	// Text content.
	Text string `json:"text,omitempty"`
	// Image or audio content.
	Data     string `json:"data,omitempty"`
	MIMEType string `json:"mimeType,omitempty"`
	URI      string `json:"uri,omitempty"`
}

// TextBlock returns a text ContentBlock.
func TextBlock(text string) *ContentBlock {
	return &ContentBlock{Type: "text", Text: text}
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

// SessionCapabilities describes session-level features the agent supports.
type SessionCapabilities struct {
	// List indicates the agent supports session/list.
	List *struct{} `json:"list,omitempty"`
}

// AgentCapabilities describes what this agent supports.
type AgentCapabilities struct {
	LoadSession         bool                 `json:"loadSession,omitempty"`
	PromptCapabilities  *PromptCapabilities  `json:"promptCapabilities,omitempty"`
	MCP                 *MCPCapabilities     `json:"mcp,omitempty"`
	SessionCapabilities *SessionCapabilities `json:"sessionCapabilities,omitempty"`
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
	SessionID     string            `json:"sessionId"`
	ConfigOptions []ConfigOption    `json:"configOptions,omitempty"`
	Modes         *SessionModeState `json:"modes,omitempty"`
}

// SessionLoadParams is the request to load an existing session.
type SessionLoadParams struct {
	SessionID  string            `json:"sessionId"`
	CWD        string            `json:"cwd,omitempty"`
	MCPServers []MCPServerConfig `json:"mcpServers,omitempty"`
}

// SessionLoadResult is the response after loading a session.
// Note: sessionId is NOT included per the ACP spec (unlike session/new).
type SessionLoadResult struct {
	ConfigOptions []ConfigOption    `json:"configOptions,omitempty"`
	Modes         *SessionModeState `json:"modes,omitempty"`
}

// SessionListParams is the request to list sessions.
type SessionListParams struct {
	CWD    string `json:"cwd,omitempty"`
	Cursor string `json:"cursor,omitempty"`
}

// SessionListEntry is a single entry in a session list.
type SessionListEntry struct {
	SessionID string `json:"sessionId"`
	CWD       string `json:"cwd"`
	Title     string `json:"title,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"` // ISO 8601
}

// SessionListResult is the response to a session/list request.
type SessionListResult struct {
	Sessions   []SessionListEntry `json:"sessions"`
	NextCursor string             `json:"nextCursor,omitempty"`
}

// SessionMode describes a legacy ACP mode entry.
type SessionMode struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// SessionModeState describes legacy ACP mode state.
type SessionModeState struct {
	CurrentModeID  string        `json:"currentModeId"`
	AvailableModes []SessionMode `json:"availableModes"`
}

// ConfigOption represents a session configuration option.
type ConfigOption struct {
	ID           string                `json:"id"`
	Name         string                `json:"name"`
	Category     string                `json:"category,omitempty"`
	Type         string                `json:"type"` // "select"
	CurrentValue string                `json:"currentValue,omitempty"`
	Options      []ConfigOptionVariant `json:"options,omitempty"`
}

// ConfigOptionVariant is a selectable option within a ConfigOption.
type ConfigOptionVariant struct {
	Value       string `json:"value"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// SetConfigOptionParams is the request to set a config option.
type SetConfigOptionParams struct {
	SessionID string `json:"sessionId"`
	ConfigID  string `json:"configId"`
	Value     string `json:"value"`
}

// SetConfigOptionResult is the response after setting a config option.
type SetConfigOptionResult struct {
	ConfigOptions []ConfigOption `json:"configOptions"`
}

// SetModeParams is the legacy request to set session mode.
type SetModeParams struct {
	SessionID string `json:"sessionId"`
	ModeID    string `json:"modeId"`
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
	SessionUpdateUserMessageChunk   SessionUpdateType = "user_message_chunk"
	SessionUpdateAgentMessageChunk  SessionUpdateType = "agent_message_chunk"
	SessionUpdateAgentThoughtChunk  SessionUpdateType = "agent_thought_chunk"
	SessionUpdateToolCall           SessionUpdateType = "tool_call"
	SessionUpdateToolCallUpdate     SessionUpdateType = "tool_call_update"
	SessionUpdatePlan               SessionUpdateType = "plan"
	SessionUpdateSessionInfoUpdate  SessionUpdateType = "session_info_update"
	SessionUpdateConfigOptionUpdate SessionUpdateType = "config_option_update"
	SessionUpdateCurrentModeUpdate  SessionUpdateType = "current_mode_update"
	SessionUpdateTimelineEvent      SessionUpdateType = "timeline_event"
)

// ToolCallStatus describes the execution state of a tool call.
type ToolCallStatus string

const (
	// ToolCallStatusPending means the tool call has not started yet.
	ToolCallStatusPending ToolCallStatus = "pending"
	// ToolCallStatusInProgress means the tool call is currently running.
	ToolCallStatusInProgress ToolCallStatus = "in_progress"
	// ToolCallStatusCompleted means the tool call finished successfully.
	ToolCallStatusCompleted ToolCallStatus = "completed"
	// ToolCallStatusFailed means the tool call failed with an error.
	ToolCallStatusFailed ToolCallStatus = "failed"
	// ToolCallStatusCanceled means the tool call was canceled before completion.
	ToolCallStatusCanceled ToolCallStatus = "canceled"
)

type SubtaskResult struct {
	Status          string `json:"status,omitempty"`
	ParentMessageID string `json:"parentMessageId,omitempty"`
}

type Reducer struct {
	Summary     string   `json:"summary,omitempty"`
	Artifacts   []string `json:"artifacts,omitempty"`
	Risks       []string `json:"risks,omitempty"`
	NextActions []string `json:"nextActions,omitempty"`
	Confidence  string   `json:"confidence,omitempty"`
	MailboxID   string   `json:"mailboxId,omitempty"`
	Messages    []string `json:"messages,omitempty"`
}

// SessionUpdate is the payload of a session/update notification.
// The SessionUpdate field identifies the variant; remaining fields are
// populated based on that variant.
type SessionUpdate struct {
	SessionUpdate SessionUpdateType `json:"sessionUpdate"`
	// Content for message/thought chunks (agent_message_chunk, user_message_chunk,
	// agent_thought_chunk). Must be a ContentBlock per ACP spec.
	Content *ContentBlock `json:"content,omitempty"`
	// Tool call or session info title.
	Title string `json:"title,omitempty"`
	// Tool call identifier.
	ToolCallID string         `json:"toolCallId,omitempty"`
	Kind       string         `json:"kind,omitempty"`
	Status     ToolCallStatus `json:"status,omitempty"`
	RawInput   any            `json:"rawInput,omitempty"`
	RawOutput  any            `json:"rawOutput,omitempty"`
	ClientMetadata map[string]any `json:"clientMetadata,omitempty"`
	DurationMs int64 `json:"durationMs,omitempty"`
	// Structured subtask result metadata projected from tool-result metadata.
	ChildSessionID   string         `json:"childSessionId,omitempty"`
	ParentToolCallID string         `json:"parentToolCallId,omitempty"`
	SubtaskResult    *SubtaskResult `json:"subtaskResult,omitempty"`
	Reducer          *Reducer       `json:"reducer,omitempty"`
	// Plan entries.
	Entries []PlanEntry `json:"entries,omitempty"`
	// Session info update fields (ISO 8601 timestamp).
	UpdatedAt string `json:"updatedAt,omitempty"`
	// Config option update fields.
	ConfigOptions []ConfigOption `json:"configOptions,omitempty"`
	// Legacy current mode update fields.
	CurrentModeID string `json:"currentModeId,omitempty"`
	// Timeline event payload.
	TimelineEvent *TimelineEvent `json:"timelineEvent,omitempty"`
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

type TimelineEvent struct {
	Type              string         `json:"type"`
	Timestamp         int64          `json:"timestamp,omitempty"`
	Title             string         `json:"title,omitempty"`
	ToolCallID        string         `json:"toolCallId,omitempty"`
	ToolName          string         `json:"toolName,omitempty"`
	Status            string         `json:"status,omitempty"`
	Content           string         `json:"content,omitempty"`
	ChildSessionID    string         `json:"childSessionId,omitempty"`
	CollaborationMode string         `json:"collaborationMode,omitempty"`
	PermissionMode    string         `json:"permissionMode,omitempty"`
	Metadata          map[string]any `json:"metadata,omitempty"`
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
	SessionID          string             `json:"sessionId"`
	AuthoritySessionID string             `json:"authoritySessionId,omitempty"`
	ToolCall           ACPToolCall        `json:"toolCall"`
	Options            []PermissionOption `json:"options"`
}

// RequestPermissionOutcome is the selection state returned by the client.
type RequestPermissionOutcome struct {
	Outcome  string `json:"outcome"` // "selected" | "cancelled"
	OptionID string `json:"optionId,omitempty"`
}

// RequestPermissionResult is the client's response to a permission request.
type RequestPermissionResult struct {
	Outcome RequestPermissionOutcome `json:"outcome"`
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
	CodeParseError       = -32700
	CodeInvalidRequest   = -32600
	CodeMethodNotFound   = -32601
	CodeInvalidParams    = -32602
	CodeInternalError    = -32603
	CodeAuthRequired     = -32000
	CodeResourceNotFound = -32002
)
