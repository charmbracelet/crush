package mcp

import (
	"encoding/json"
	"fmt"
)

// ClientRequest types
var _ ClientRequest = &PingRequest{}
var _ ClientRequest = &InitializeRequest{}
var _ ClientRequest = &CompleteRequest{}
var _ ClientRequest = &SetLevelRequest{}
var _ ClientRequest = &GetPromptRequest{}
var _ ClientRequest = &ListPromptsRequest{}
var _ ClientRequest = &ListResourcesRequest{}
var _ ClientRequest = &ReadResourceRequest{}
var _ ClientRequest = &SubscribeRequest{}
var _ ClientRequest = &UnsubscribeRequest{}
var _ ClientRequest = &CallToolRequest{}
var _ ClientRequest = &ListToolsRequest{}

// ClientNotification types
var _ ClientNotification = &CancelledNotification{}
var _ ClientNotification = &ProgressNotification{}
var _ ClientNotification = &InitializedNotification{}
var _ ClientNotification = &RootsListChangedNotification{}

// ClientResult types
var _ ClientResult = &EmptyResult{}
var _ ClientResult = &CreateMessageResult{}
var _ ClientResult = &ListRootsResult{}

// ServerRequest types
var _ ServerRequest = &PingRequest{}
var _ ServerRequest = &CreateMessageRequest{}
var _ ServerRequest = &ListRootsRequest{}

// ServerNotification types
var _ ServerNotification = &CancelledNotification{}
var _ ServerNotification = &ProgressNotification{}
var _ ServerNotification = &LoggingMessageNotification{}
var _ ServerNotification = &ResourceUpdatedNotification{}
var _ ServerNotification = &ResourceListChangedNotification{}
var _ ServerNotification = &ToolListChangedNotification{}
var _ ServerNotification = &PromptListChangedNotification{}

// ServerResult types
var _ ServerResult = &EmptyResult{}
var _ ServerResult = &InitializeResult{}
var _ ServerResult = &CompleteResult{}
var _ ServerResult = &GetPromptResult{}
var _ ServerResult = &ListPromptsResult{}
var _ ServerResult = &ListResourcesResult{}
var _ ServerResult = &ReadResourceResult{}
var _ ServerResult = &CallToolResult{}
var _ ServerResult = &ListToolsResult{}

// Helper functions for type assertions

// asType attempts to cast the given interface to the given type
func asType[T any](content interface{}) (*T, bool) {
	tc, ok := content.(T)
	if !ok {
		return nil, false
	}
	return &tc, true
}

// AsTextContent attempts to cast the given interface to TextContent
func AsTextContent(content interface{}) (*TextContent, bool) {
	return asType[TextContent](content)
}

// AsImageContent attempts to cast the given interface to ImageContent
func AsImageContent(content interface{}) (*ImageContent, bool) {
	return asType[ImageContent](content)
}

// AsEmbeddedResource attempts to cast the given interface to EmbeddedResource
func AsEmbeddedResource(content interface{}) (*EmbeddedResource, bool) {
	return asType[EmbeddedResource](content)
}

// AsTextResourceContents attempts to cast the given interface to TextResourceContents
func AsTextResourceContents(content interface{}) (*TextResourceContents, bool) {
	return asType[TextResourceContents](content)
}

// AsBlobResourceContents attempts to cast the given interface to BlobResourceContents
func AsBlobResourceContents(content interface{}) (*BlobResourceContents, bool) {
	return asType[BlobResourceContents](content)
}

// Helper function for JSON-RPC

// NewJSONRPCResponse creates a new JSONRPCResponse with the given id and result
func NewJSONRPCResponse(id RequestId, result Result) JSONRPCResponse {
	return JSONRPCResponse{
		JSONRPC: JSONRPC_VERSION,
		ID:      id,
		Result:  result,
	}
}

// NewJSONRPCError creates a new JSONRPCResponse with the given id, code, and message
func NewJSONRPCError(
	id RequestId,
	code int,
	message string,
	data interface{},
) JSONRPCError {
	return JSONRPCError{
		JSONRPC: JSONRPC_VERSION,
		ID:      id,
		Error: struct {
			Code    int         `json:"code"`
			Message string      `json:"message"`
			Data    interface{} `json:"data,omitempty"`
		}{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
}

// Helper function for creating a progress notification
func NewProgressNotification(
	token ProgressToken,
	progress float64,
	total *float64,
) ProgressNotification {
	notification := ProgressNotification{
		Notification: Notification{
			Method: "notifications/progress",
		},
		Params: struct {
			ProgressToken ProgressToken `json:"progressToken"`
			Progress      float64       `json:"progress"`
			Total         float64       `json:"total,omitempty"`
		}{
			ProgressToken: token,
			Progress:      progress,
		},
	}
	if total != nil {
		notification.Params.Total = *total
	}
	return notification
}

// Helper function for creating a logging message notification
func NewLoggingMessageNotification(
	level LoggingLevel,
	logger string,
	data interface{},
) LoggingMessageNotification {
	return LoggingMessageNotification{
		Notification: Notification{
			Method: "notifications/message",
		},
		Params: struct {
			Level  LoggingLevel `json:"level"`
			Logger string       `json:"logger,omitempty"`
			Data   interface{}  `json:"data"`
		}{
			Level:  level,
			Logger: logger,
			Data:   data,
		},
	}
}

// Helper function to create a new PromptMessage
func NewPromptMessage(role Role, content Content) PromptMessage {
	return PromptMessage{
		Role:    role,
		Content: content,
	}
}

// Helper function to create a new TextContent
func NewTextContent(text string) TextContent {
	return TextContent{
		Type: "text",
		Text: text,
	}
}

// Helper function to create a new ImageContent
func NewImageContent(data, mimeType string) ImageContent {
	return ImageContent{
		Type:     "image",
		Data:     data,
		MIMEType: mimeType,
	}
}

// Helper function to create a new EmbeddedResource
func NewEmbeddedResource(resource ResourceContents) EmbeddedResource {
	return EmbeddedResource{
		Type:     "resource",
		Resource: resource,
	}
}

// NewToolResultText creates a new CallToolResult with a text content
func NewToolResultText(text string) *CallToolResult {
	return &CallToolResult{
		Content: []Content{
			TextContent{
				Type: "text",
				Text: text,
			},
		},
	}
}

// NewToolResultImage creates a new CallToolResult with both text and image content
func NewToolResultImage(text, imageData, mimeType string) *CallToolResult {
	return &CallToolResult{
		Content: []Content{
			TextContent{
				Type: "text",
				Text: text,
			},
			ImageContent{
				Type:     "image",
				Data:     imageData,
				MIMEType: mimeType,
			},
		},
	}
}

// NewToolResultResource creates a new CallToolResult with an embedded resource
func NewToolResultResource(
	text string,
	resource ResourceContents,
) *CallToolResult {
	return &CallToolResult{
		Content: []Content{
			TextContent{
				Type: "text",
				Text: text,
			},
			EmbeddedResource{
				Type:     "resource",
				Resource: resource,
			},
		},
	}
}

// NewListResourcesResult creates a new ListResourcesResult
func NewListResourcesResult(
	resources []Resource,
	nextCursor Cursor,
) *ListResourcesResult {
	return &ListResourcesResult{
		PaginatedResult: PaginatedResult{
			NextCursor: nextCursor,
		},
		Resources: resources,
	}
}

// NewListResourceTemplatesResult creates a new ListResourceTemplatesResult
func NewListResourceTemplatesResult(
	templates []ResourceTemplate,
	nextCursor Cursor,
) *ListResourceTemplatesResult {
	return &ListResourceTemplatesResult{
		PaginatedResult: PaginatedResult{
			NextCursor: nextCursor,
		},
		ResourceTemplates: templates,
	}
}

// NewReadResourceResult creates a new ReadResourceResult with text content
func NewReadResourceResult(text string) *ReadResourceResult {
	return &ReadResourceResult{
		Contents: []ResourceContents{
			TextResourceContents{
				Text: text,
			},
		},
	}
}

// NewListPromptsResult creates a new ListPromptsResult
func NewListPromptsResult(
	prompts []Prompt,
	nextCursor Cursor,
) *ListPromptsResult {
	return &ListPromptsResult{
		PaginatedResult: PaginatedResult{
			NextCursor: nextCursor,
		},
		Prompts: prompts,
	}
}

// NewGetPromptResult creates a new GetPromptResult
func NewGetPromptResult(
	description string,
	messages []PromptMessage,
) *GetPromptResult {
	return &GetPromptResult{
		Description: description,
		Messages:    messages,
	}
}

// NewListToolsResult creates a new ListToolsResult
func NewListToolsResult(tools []Tool, nextCursor Cursor) *ListToolsResult {
	return &ListToolsResult{
		PaginatedResult: PaginatedResult{
			NextCursor: nextCursor,
		},
		Tools: tools,
	}
}

// NewInitializeResult creates a new InitializeResult
func NewInitializeResult(
	protocolVersion string,
	capabilities ServerCapabilities,
	serverInfo Implementation,
	instructions string,
) *InitializeResult {
	return &InitializeResult{
		ProtocolVersion: protocolVersion,
		Capabilities:    capabilities,
		ServerInfo:      serverInfo,
		Instructions:    instructions,
	}
}

// Helper for formatting numbers in tool results
func FormatNumberResult(value float64) *CallToolResult {
	return NewToolResultText(fmt.Sprintf("%.2f", value))
}

func ExtractString(data map[string]any, key string) string {
	if value, ok := data[key]; ok {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return ""
}

func ExtractMap(data map[string]any, key string) map[string]any {
	if value, ok := data[key]; ok {
		if m, ok := value.(map[string]any); ok {
			return m
		}
	}
	return nil
}

func ParseContent(contentMap map[string]any) (Content, error) {
	contentType := ExtractString(contentMap, "type")

	switch contentType {
	case "text":
		text := ExtractString(contentMap, "text")
		if text == "" {
			return nil, fmt.Errorf("text is missing")
		}
		return NewTextContent(text), nil

	case "image":
		data := ExtractString(contentMap, "data")
		mimeType := ExtractString(contentMap, "mimeType")
		if data == "" || mimeType == "" {
			return nil, fmt.Errorf("image data or mimeType is missing")
		}
		return NewImageContent(data, mimeType), nil

	case "resource":
		resourceMap := ExtractMap(contentMap, "resource")
		if resourceMap == nil {
			return nil, fmt.Errorf("resource is missing")
		}

		resourceContents, err := ParseResourceContents(resourceMap)
		if err != nil {
			return nil, err
		}

		return NewEmbeddedResource(resourceContents), nil
	}

	return nil, fmt.Errorf("unsupported content type: %s", contentType)
}

func ParseGetPromptResult(rawMessage *json.RawMessage) (*GetPromptResult, error) {
	var jsonContent map[string]any
	if err := json.Unmarshal(*rawMessage, &jsonContent); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	result := GetPromptResult{}

	meta, ok := jsonContent["_meta"]
	if ok {
		if metaMap, ok := meta.(map[string]any); ok {
			result.Meta = metaMap
		}
	}

	description, ok := jsonContent["description"]
	if ok {
		if descriptionStr, ok := description.(string); ok {
			result.Description = descriptionStr
		}
	}

	messages, ok := jsonContent["messages"]
	if ok {
		messagesArr, ok := messages.([]any)
		if !ok {
			return nil, fmt.Errorf("messages is not an array")
		}

		for _, message := range messagesArr {
			messageMap, ok := message.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("message is not an object")
			}

			// Extract role
			roleStr := ExtractString(messageMap, "role")
			if roleStr == "" || (roleStr != string(RoleAssistant) && roleStr != string(RoleUser)) {
				return nil, fmt.Errorf("unsupported role: %s", roleStr)
			}

			// Extract content
			contentMap, ok := messageMap["content"].(map[string]any)
			if !ok {
				return nil, fmt.Errorf("content is not an object")
			}

			// Process content
			content, err := ParseContent(contentMap)
			if err != nil {
				return nil, err
			}

			// Append processed message
			result.Messages = append(result.Messages, NewPromptMessage(Role(roleStr), content))

		}
	}

	return &result, nil
}

func ParseCallToolResult(rawMessage *json.RawMessage) (*CallToolResult, error) {
	var jsonContent map[string]any
	if err := json.Unmarshal(*rawMessage, &jsonContent); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	var result CallToolResult

	meta, ok := jsonContent["_meta"]
	if ok {
		if metaMap, ok := meta.(map[string]any); ok {
			result.Meta = metaMap
		}
	}

	isError, ok := jsonContent["isError"]
	if ok {
		if isErrorBool, ok := isError.(bool); ok {
			result.IsError = isErrorBool
		}
	}

	contents, ok := jsonContent["content"]
	if !ok {
		return nil, fmt.Errorf("content is missing")
	}

	contentArr, ok := contents.([]any)
	if !ok {
		return nil, fmt.Errorf("content is not an array")
	}

	for _, content := range contentArr {
		// Extract content
		contentMap, ok := content.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("content is not an object")
		}

		// Process content
		content, err := ParseContent(contentMap)
		if err != nil {
			return nil, err
		}

		result.Content = append(result.Content, content)
	}

	return &result, nil
}

func ParseResourceContents(contentMap map[string]any) (ResourceContents, error) {
	uri := ExtractString(contentMap, "uri")
	if uri == "" {
		return nil, fmt.Errorf("resource uri is missing")
	}

	mimeType := ExtractString(contentMap, "mimeType")

	if text := ExtractString(contentMap, "text"); text != "" {
		return TextResourceContents{
			URI:      uri,
			MIMEType: mimeType,
			Text:     text,
		}, nil
	}

	if blob := ExtractString(contentMap, "blob"); blob != "" {
		return BlobResourceContents{
			URI:      uri,
			MIMEType: mimeType,
			Blob:     blob,
		}, nil
	}

	return nil, fmt.Errorf("unsupported resource type")
}

func ParseReadResourceResult(rawMessage *json.RawMessage) (*ReadResourceResult, error) {
	var jsonContent map[string]any
	if err := json.Unmarshal(*rawMessage, &jsonContent); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	var result ReadResourceResult

	meta, ok := jsonContent["_meta"]
	if ok {
		if metaMap, ok := meta.(map[string]any); ok {
			result.Meta = metaMap
		}
	}

	contents, ok := jsonContent["contents"]
	if !ok {
		return nil, fmt.Errorf("contents is missing")
	}

	contentArr, ok := contents.([]any)
	if !ok {
		return nil, fmt.Errorf("contents is not an array")
	}

	for _, content := range contentArr {
		// Extract content
		contentMap, ok := content.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("content is not an object")
		}

		// Process content
		content, err := ParseResourceContents(contentMap)
		if err != nil {
			return nil, err
		}

		result.Contents = append(result.Contents, content)
	}

	return &result, nil
}
