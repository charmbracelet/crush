package tools

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/charmbracelet/crush/internal/csync"
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
)

const (
	ToolResponseTypeText  toolResponseType = "text"
	ToolResponseTypeImage toolResponseType = "image"

	SessionIDContextKey sessionIDContextKey = "session_id"
	MessageIDContextKey messageIDContextKey = "message_id"
)

type ToolResponse struct {
	Type     toolResponseType `json:"type"`
	Content  string           `json:"content"`
	Metadata string           `json:"metadata,omitempty"`
	IsError  bool             `json:"is_error"`
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

type Registry interface {
	GetTool(name string) (BaseTool, bool)
	SetTool(name string, tool BaseTool)
	GetAllTools() []BaseTool
}

type registry struct {
	tools *csync.LazySlice[BaseTool]
}

func (r *registry) GetAllTools() []BaseTool {
	return slices.Collect(r.tools.Seq())
}

func (r *registry) GetTool(name string) (BaseTool, bool) {
	for tool := range r.tools.Seq() {
		if tool.Name() == name {
			return tool, true
		}
	}

	return nil, false
}

func (r *registry) SetTool(name string, tool BaseTool) {
	for k, tool := range r.tools.Seq2() {
		if tool.Name() == name {
			r.tools.Set(k, tool)
			return
		}
	}
	r.tools.Append(tool)
}

type LazyToolsFn func() []BaseTool

func NewRegistry(lazyTools LazyToolsFn) Registry {
	return &registry{
		tools: csync.NewLazySlice(lazyTools),
	}
}

func NewRegistryFromTools(tools []BaseTool) Registry {
	return &registry{
		tools: csync.NewLazySlice(func() []BaseTool { return tools }),
	}
}
