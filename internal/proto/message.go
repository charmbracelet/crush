package proto

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"charm.land/catwalk/pkg/catwalk"
)

// CreateMessageParams represents parameters for creating a message.
type CreateMessageParams struct {
	Role     MessageRole   `json:"role"`
	Parts    []ContentPart `json:"parts"`
	Model    string        `json:"model"`
	Provider string        `json:"provider,omitempty"`
}

// Message represents a message in the proto layer.
type Message struct {
	ID        string        `json:"id"`
	Role      MessageRole   `json:"role"`
	SessionID string        `json:"session_id"`
	Parts     []ContentPart `json:"parts"`
	Model     string        `json:"model"`
	Provider  string        `json:"provider"`
	CreatedAt int64         `json:"created_at"`
	UpdatedAt int64         `json:"updated_at"`
}

// MessageRole represents the role of a message sender.
type MessageRole string

const (
	Assistant MessageRole = "assistant"
	User      MessageRole = "user"
	System    MessageRole = "system"
	Tool      MessageRole = "tool"
)

// MarshalText implements the [encoding.TextMarshaler] interface.
func (r MessageRole) MarshalText() ([]byte, error) {
	return []byte(r), nil
}

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (r *MessageRole) UnmarshalText(data []byte) error {
	*r = MessageRole(data)
	return nil
}

// FinishReason represents why a message generation finished.
type FinishReason string

const (
	FinishReasonEndTurn   FinishReason = "end_turn"
	FinishReasonMaxTokens FinishReason = "max_tokens"
	FinishReasonToolUse   FinishReason = "tool_use"
	FinishReasonCanceled  FinishReason = "canceled"
	FinishReasonError     FinishReason = "error"
	FinishReasonUnknown   FinishReason = "unknown"
)

// MarshalText implements the [encoding.TextMarshaler] interface.
func (fr FinishReason) MarshalText() ([]byte, error) {
	return []byte(fr), nil
}

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (fr *FinishReason) UnmarshalText(data []byte) error {
	*fr = FinishReason(data)
	return nil
}

// ContentPart is a part of a message's content.
type ContentPart interface {
	isPart()
}

// ReasoningContent represents the reasoning/thinking part of a message.
type ReasoningContent struct {
	Thinking   string `json:"thinking"`
	Signature  string `json:"signature"`
	StartedAt  int64  `json:"started_at,omitempty"`
	FinishedAt int64  `json:"finished_at,omitempty"`
}

// String returns the thinking content as a string.
func (tc ReasoningContent) String() string {
	return tc.Thinking
}

func (ReasoningContent) isPart() {}

// TextContent represents a text part of a message.
type TextContent struct {
	Text string `json:"text"`
}

// String returns the text content as a string.
func (tc TextContent) String() string {
	return tc.Text
}

func (TextContent) isPart() {}

// ImageURLContent represents an image URL part of a message.
type ImageURLContent struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

// String returns the image URL as a string.
func (iuc ImageURLContent) String() string {
	return iuc.URL
}

func (ImageURLContent) isPart() {}

// BinaryContent represents binary data in a message.
type BinaryContent struct {
	Path     string
	MIMEType string
	Data     []byte
}

// String returns a base64-encoded string of the binary data.
func (bc BinaryContent) String(p catwalk.InferenceProvider) string {
	base64Encoded := base64.StdEncoding.EncodeToString(bc.Data)
	if p == catwalk.InferenceProviderOpenAI {
		return "data:" + bc.MIMEType + ";base64," + base64Encoded
	}
	return base64Encoded
}

func (BinaryContent) isPart() {}

// ToolCall represents a tool call in a message.
type ToolCall struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Input    string `json:"input"`
	Type     string `json:"type,omitempty"`
	Finished bool   `json:"finished,omitempty"`
}

func (ToolCall) isPart() {}

// ToolResult represents the result of a tool call.
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Name       string `json:"name"`
	Content    string `json:"content"`
	Metadata   string `json:"metadata"`
	IsError    bool   `json:"is_error"`
}

func (ToolResult) isPart() {}

// Finish represents the end of a message generation.
type Finish struct {
	Reason  FinishReason `json:"reason"`
	Time    int64        `json:"time"`
	Message string       `json:"message,omitempty"`
	Details string       `json:"details,omitempty"`
}

func (Finish) isPart() {}

// MarshalJSON implements the [json.Marshaler] interface.
func (m Message) MarshalJSON() ([]byte, error) {
	parts, err := MarshalParts(m.Parts)
	if err != nil {
		return nil, err
	}

	type Alias Message
	return json.Marshal(&struct {
		Parts json.RawMessage `json:"parts"`
		*Alias
	}{
		Parts: json.RawMessage(parts),
		Alias: (*Alias)(&m),
	})
}

// UnmarshalJSON implements the [json.Unmarshaler] interface.
func (m *Message) UnmarshalJSON(data []byte) error {
	type Alias Message
	aux := &struct {
		Parts json.RawMessage `json:"parts"`
		*Alias
	}{
		Alias: (*Alias)(m),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	parts, err := UnmarshalParts([]byte(aux.Parts))
	if err != nil {
		return err
	}

	m.Parts = parts
	return nil
}

// Content returns the first text content part.
func (m *Message) Content() TextContent {
	for _, part := range m.Parts {
		if c, ok := part.(TextContent); ok {
			return c
		}
	}
	return TextContent{}
}

// ReasoningContent returns the first reasoning content part.
func (m *Message) ReasoningContent() ReasoningContent {
	for _, part := range m.Parts {
		if c, ok := part.(ReasoningContent); ok {
			return c
		}
	}
	return ReasoningContent{}
}

// ImageURLContent returns all image URL content parts.
func (m *Message) ImageURLContent() []ImageURLContent {
	imageURLContents := make([]ImageURLContent, 0)
	for _, part := range m.Parts {
		if c, ok := part.(ImageURLContent); ok {
			imageURLContents = append(imageURLContents, c)
		}
	}
	return imageURLContents
}

// BinaryContent returns all binary content parts.
func (m *Message) BinaryContent() []BinaryContent {
	binaryContents := make([]BinaryContent, 0)
	for _, part := range m.Parts {
		if c, ok := part.(BinaryContent); ok {
			binaryContents = append(binaryContents, c)
		}
	}
	return binaryContents
}

// ToolCalls returns all tool call parts.
func (m *Message) ToolCalls() []ToolCall {
	toolCalls := make([]ToolCall, 0)
	for _, part := range m.Parts {
		if c, ok := part.(ToolCall); ok {
			toolCalls = append(toolCalls, c)
		}
	}
	return toolCalls
}

// ToolResults returns all tool result parts.
func (m *Message) ToolResults() []ToolResult {
	toolResults := make([]ToolResult, 0)
	for _, part := range m.Parts {
		if c, ok := part.(ToolResult); ok {
			toolResults = append(toolResults, c)
		}
	}
	return toolResults
}

// IsFinished returns true if the message has a finish part.
func (m *Message) IsFinished() bool {
	for _, part := range m.Parts {
		if _, ok := part.(Finish); ok {
			return true
		}
	}
	return false
}

// FinishPart returns the finish part if present.
func (m *Message) FinishPart() *Finish {
	for _, part := range m.Parts {
		if c, ok := part.(Finish); ok {
			return &c
		}
	}
	return nil
}

// FinishReason returns the finish reason if present.
func (m *Message) FinishReason() FinishReason {
	for _, part := range m.Parts {
		if c, ok := part.(Finish); ok {
			return c.Reason
		}
	}
	return ""
}

// IsThinking returns true if the message is currently in a thinking state.
func (m *Message) IsThinking() bool {
	return m.ReasoningContent().Thinking != "" && m.Content().Text == "" && !m.IsFinished()
}

// AppendContent appends text to the text content part.
func (m *Message) AppendContent(delta string) {
	found := false
	for i, part := range m.Parts {
		if c, ok := part.(TextContent); ok {
			m.Parts[i] = TextContent{Text: c.Text + delta}
			found = true
		}
	}
	if !found {
		m.Parts = append(m.Parts, TextContent{Text: delta})
	}
}

// AppendReasoningContent appends text to the reasoning content part.
func (m *Message) AppendReasoningContent(delta string) {
	found := false
	for i, part := range m.Parts {
		if c, ok := part.(ReasoningContent); ok {
			m.Parts[i] = ReasoningContent{
				Thinking:   c.Thinking + delta,
				Signature:  c.Signature,
				StartedAt:  c.StartedAt,
				FinishedAt: c.FinishedAt,
			}
			found = true
		}
	}
	if !found {
		m.Parts = append(m.Parts, ReasoningContent{
			Thinking:  delta,
			StartedAt: time.Now().Unix(),
		})
	}
}

// AppendReasoningSignature appends a signature to the reasoning content part.
func (m *Message) AppendReasoningSignature(signature string) {
	for i, part := range m.Parts {
		if c, ok := part.(ReasoningContent); ok {
			m.Parts[i] = ReasoningContent{
				Thinking:   c.Thinking,
				Signature:  c.Signature + signature,
				StartedAt:  c.StartedAt,
				FinishedAt: c.FinishedAt,
			}
			return
		}
	}
	m.Parts = append(m.Parts, ReasoningContent{Signature: signature})
}

// FinishThinking marks the reasoning content as finished.
func (m *Message) FinishThinking() {
	for i, part := range m.Parts {
		if c, ok := part.(ReasoningContent); ok {
			if c.FinishedAt == 0 {
				m.Parts[i] = ReasoningContent{
					Thinking:   c.Thinking,
					Signature:  c.Signature,
					StartedAt:  c.StartedAt,
					FinishedAt: time.Now().Unix(),
				}
			}
			return
		}
	}
}

// ThinkingDuration returns the duration of the thinking phase.
func (m *Message) ThinkingDuration() time.Duration {
	reasoning := m.ReasoningContent()
	if reasoning.StartedAt == 0 {
		return 0
	}

	endTime := reasoning.FinishedAt
	if endTime == 0 {
		endTime = time.Now().Unix()
	}

	return time.Duration(endTime-reasoning.StartedAt) * time.Second
}

// FinishToolCall marks a tool call as finished.
func (m *Message) FinishToolCall(toolCallID string) {
	for i, part := range m.Parts {
		if c, ok := part.(ToolCall); ok {
			if c.ID == toolCallID {
				m.Parts[i] = ToolCall{
					ID:       c.ID,
					Name:     c.Name,
					Input:    c.Input,
					Type:     c.Type,
					Finished: true,
				}
				return
			}
		}
	}
}

// AppendToolCallInput appends input to a tool call.
func (m *Message) AppendToolCallInput(toolCallID string, inputDelta string) {
	for i, part := range m.Parts {
		if c, ok := part.(ToolCall); ok {
			if c.ID == toolCallID {
				m.Parts[i] = ToolCall{
					ID:       c.ID,
					Name:     c.Name,
					Input:    c.Input + inputDelta,
					Type:     c.Type,
					Finished: c.Finished,
				}
				return
			}
		}
	}
}

// AddToolCall adds or updates a tool call.
func (m *Message) AddToolCall(tc ToolCall) {
	for i, part := range m.Parts {
		if c, ok := part.(ToolCall); ok {
			if c.ID == tc.ID {
				m.Parts[i] = tc
				return
			}
		}
	}
	m.Parts = append(m.Parts, tc)
}

// SetToolCalls replaces all tool call parts.
func (m *Message) SetToolCalls(tc []ToolCall) {
	parts := make([]ContentPart, 0)
	for _, part := range m.Parts {
		if _, ok := part.(ToolCall); ok {
			continue
		}
		parts = append(parts, part)
	}
	m.Parts = parts
	for _, toolCall := range tc {
		m.Parts = append(m.Parts, toolCall)
	}
}

// AddToolResult adds a tool result.
func (m *Message) AddToolResult(tr ToolResult) {
	m.Parts = append(m.Parts, tr)
}

// SetToolResults adds multiple tool results.
func (m *Message) SetToolResults(tr []ToolResult) {
	for _, toolResult := range tr {
		m.Parts = append(m.Parts, toolResult)
	}
}

// AddFinish adds a finish part to the message.
func (m *Message) AddFinish(reason FinishReason, message, details string) {
	for i, part := range m.Parts {
		if _, ok := part.(Finish); ok {
			m.Parts = slices.Delete(m.Parts, i, i+1)
			break
		}
	}
	m.Parts = append(m.Parts, Finish{Reason: reason, Time: time.Now().Unix(), Message: message, Details: details})
}

// AddImageURL adds an image URL part to the message.
func (m *Message) AddImageURL(url, detail string) {
	m.Parts = append(m.Parts, ImageURLContent{URL: url, Detail: detail})
}

// AddBinary adds a binary content part to the message.
func (m *Message) AddBinary(mimeType string, data []byte) {
	m.Parts = append(m.Parts, BinaryContent{MIMEType: mimeType, Data: data})
}

type partType string

const (
	reasoningType  partType = "reasoning"
	textType       partType = "text"
	imageURLType   partType = "image_url"
	binaryType     partType = "binary"
	toolCallType   partType = "tool_call"
	toolResultType partType = "tool_result"
	finishType     partType = "finish"
)

type partWrapper struct {
	Type partType    `json:"type"`
	Data ContentPart `json:"data"`
}

// MarshalParts marshals content parts to JSON.
func MarshalParts(parts []ContentPart) ([]byte, error) {
	wrappedParts := make([]partWrapper, len(parts))

	for i, part := range parts {
		var typ partType

		switch part.(type) {
		case ReasoningContent:
			typ = reasoningType
		case TextContent:
			typ = textType
		case ImageURLContent:
			typ = imageURLType
		case BinaryContent:
			typ = binaryType
		case ToolCall:
			typ = toolCallType
		case ToolResult:
			typ = toolResultType
		case Finish:
			typ = finishType
		default:
			return nil, fmt.Errorf("unknown part type: %T", part)
		}

		wrappedParts[i] = partWrapper{
			Type: typ,
			Data: part,
		}
	}
	return json.Marshal(wrappedParts)
}

// UnmarshalParts unmarshals content parts from JSON.
func UnmarshalParts(data []byte) ([]ContentPart, error) {
	temp := []json.RawMessage{}

	if err := json.Unmarshal(data, &temp); err != nil {
		return nil, err
	}

	parts := make([]ContentPart, 0)

	for _, rawPart := range temp {
		var wrapper struct {
			Type partType        `json:"type"`
			Data json.RawMessage `json:"data"`
		}

		if err := json.Unmarshal(rawPart, &wrapper); err != nil {
			return nil, err
		}

		switch wrapper.Type {
		case reasoningType:
			part := ReasoningContent{}
			if err := json.Unmarshal(wrapper.Data, &part); err != nil {
				return nil, err
			}
			parts = append(parts, part)
		case textType:
			part := TextContent{}
			if err := json.Unmarshal(wrapper.Data, &part); err != nil {
				return nil, err
			}
			parts = append(parts, part)
		case imageURLType:
			part := ImageURLContent{}
			if err := json.Unmarshal(wrapper.Data, &part); err != nil {
				return nil, err
			}
			parts = append(parts, part)
		case binaryType:
			part := BinaryContent{}
			if err := json.Unmarshal(wrapper.Data, &part); err != nil {
				return nil, err
			}
			parts = append(parts, part)
		case toolCallType:
			part := ToolCall{}
			if err := json.Unmarshal(wrapper.Data, &part); err != nil {
				return nil, err
			}
			parts = append(parts, part)
		case toolResultType:
			part := ToolResult{}
			if err := json.Unmarshal(wrapper.Data, &part); err != nil {
				return nil, err
			}
			parts = append(parts, part)
		case finishType:
			part := Finish{}
			if err := json.Unmarshal(wrapper.Data, &part); err != nil {
				return nil, err
			}
			parts = append(parts, part)
		default:
			return nil, fmt.Errorf("unknown part type: %s", wrapper.Type)
		}
	}

	return parts, nil
}

// Attachment represents a file attachment.
type Attachment struct {
	FilePath string `json:"file_path"`
	FileName string `json:"file_name"`
	MimeType string `json:"mime_type"`
	Content  []byte `json:"content"`
}

// MarshalJSON implements the [json.Marshaler] interface.
func (a Attachment) MarshalJSON() ([]byte, error) {
	type Alias Attachment
	return json.Marshal(&struct {
		Content string `json:"content"`
		*Alias
	}{
		Content: base64.StdEncoding.EncodeToString(a.Content),
		Alias:   (*Alias)(&a),
	})
}

// UnmarshalJSON implements the [json.Unmarshaler] interface.
func (a *Attachment) UnmarshalJSON(data []byte) error {
	type Alias Attachment
	aux := &struct {
		Content string `json:"content"`
		*Alias
	}{
		Alias: (*Alias)(a),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	content, err := base64.StdEncoding.DecodeString(aux.Content)
	if err != nil {
		return err
	}
	a.Content = content
	return nil
}
