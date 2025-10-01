package tools

import (
	"context"
	"encoding/json"
	"time"
)

type ToolInfo struct {
	Name        string
	Description string
	Parameters  map[string]any
	Required    []string
}

type toolResponseType string

type (
	sessionIDContextKey string
	messageIDContextKey string
	progressFuncKeyType struct{}
)

const (
	ToolResponseTypeText  toolResponseType = "text"
	ToolResponseTypeImage toolResponseType = "image"

	SessionIDContextKey sessionIDContextKey = "session_id"
	MessageIDContextKey messageIDContextKey = "message_id"
)

// ProgressFuncKey is the context key for accessing the progress function
var ProgressFuncKey = progressFuncKeyType{}

// ProgressFunc is a function that emits progress updates
type ProgressFunc func(message string)

type ToolResponse struct {
	Type            toolResponseType `json:"type"`
	Content         string           `json:"content"`
	Metadata        string           `json:"metadata,omitempty"`
	IsError         bool             `json:"is_error"`
	ExecutionMetadata *ExecutionMetadata `json:"execution_metadata,omitempty"`
}

// ExecutionMetadata contains detailed information about tool execution
type ExecutionMetadata struct {
	ToolName     string        `json:"tool_name"`
	Duration     time.Duration `json:"duration"`

	// File operations
	FilePath     string        `json:"file_path,omitempty"`
	Operation    string        `json:"operation,omitempty"` // "read", "write", "created", "modified"
	LineCount    int           `json:"line_count,omitempty"`
	ByteSize     int64         `json:"byte_size,omitempty"`

	// Search operations
	Pattern      string        `json:"pattern,omitempty"`
	MatchCount   int           `json:"match_count,omitempty"`

	// Shell operations
	Command      string        `json:"command,omitempty"`
	ExitCode     *int          `json:"exit_code,omitempty"`

	// Diff information (for write/edit operations)
	Diff         string        `json:"diff,omitempty"`
	Additions    int           `json:"additions,omitempty"`
	Deletions    int           `json:"deletions,omitempty"`

	// Error information
	ErrorMessage string        `json:"error_message,omitempty"`
}

func NewTextResponse(content string) ToolResponse {
	return ToolResponse{
		Type:    ToolResponseTypeText,
		Content: content,
	}
}

func WithResponseMetadata(response ToolResponse, metadata any) ToolResponse {
	if metadata != nil {
		metadataBytes, err := json.Marshal(metadata)
		if err != nil {
			return response
		}
		response.Metadata = string(metadataBytes)
	}
	return response
}

func NewTextErrorResponse(content string) ToolResponse {
	return ToolResponse{
		Type:    ToolResponseTypeText,
		Content: content,
		IsError: true,
	}
}

type ToolCall struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Input string `json:"input"`
}

type BaseTool interface {
	Info() ToolInfo
	Name() string
	Run(ctx context.Context, params ToolCall) (ToolResponse, error)
}

func GetContextValues(ctx context.Context) (string, string) {
	sessionID := ctx.Value(SessionIDContextKey)
	messageID := ctx.Value(MessageIDContextKey)
	if sessionID == nil {
		return "", ""
	}
	if messageID == nil {
		return sessionID.(string), ""
	}
	return sessionID.(string), messageID.(string)
}
