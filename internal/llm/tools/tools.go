package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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

	MaxResponseWidth  = 3000
	MaxResponseHeight = 5000
	MaxResponseChars  = 50000
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
		Content: truncateContent(content),
	}
}

func truncateContent(content string) string {
	if len(content) <= MaxResponseChars {
		return truncateWidthAndHeight(content)
	}

	truncated := content[:MaxResponseChars]

	if lastNewline := strings.LastIndex(truncated, "\n"); lastNewline > MaxResponseChars/2 {
		truncated = truncated[:lastNewline]
	}

	truncated += "\n\n... [Content truncated due to length] ..."

	return truncateWidthAndHeight(truncated)
}

func truncateWidthAndHeight(content string) string {
	lines := strings.Split(content, "\n")

	heightTruncated := false
	if len(lines) > MaxResponseHeight {
		keepLines := MaxResponseHeight - 3
		firstHalf := keepLines / 2
		secondHalf := keepLines - firstHalf

		truncatedLines := make([]string, 0, MaxResponseHeight)
		truncatedLines = append(truncatedLines, lines[:firstHalf]...)
		truncatedLines = append(truncatedLines, "")
		truncatedLines = append(truncatedLines, fmt.Sprintf("... [%d lines truncated] ...", len(lines)-keepLines))
		truncatedLines = append(truncatedLines, "")
		truncatedLines = append(truncatedLines, lines[len(lines)-secondHalf:]...)

		lines = truncatedLines
		heightTruncated = true
	}

	widthTruncated := false
	for i, line := range lines {
		if len(line) > MaxResponseWidth {
			if MaxResponseWidth > 20 {
				keepChars := MaxResponseWidth - 10
				firstHalf := keepChars / 2
				secondHalf := keepChars - firstHalf
				lines[i] = line[:firstHalf] + " ... " + line[len(line)-secondHalf:]
			} else {
				lines[i] = line[:MaxResponseWidth]
			}
			widthTruncated = true
		}
	}

	result := strings.Join(lines, "\n")

	if heightTruncated || widthTruncated {
		notices := []string{}
		if heightTruncated {
			notices = append(notices, "height")
		}
		if widthTruncated {
			notices = append(notices, "width")
		}
		result += fmt.Sprintf("\n\n[Note: Content truncated by %s to fit response limits]", strings.Join(notices, " and "))
	}

	return result
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
