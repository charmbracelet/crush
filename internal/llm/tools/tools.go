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

	maxResponseWidth  = 3000
	maxResponseHeight = 5000
	maxResponseChars  = 50000
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
	if len(content) <= maxResponseChars {
		return truncateWidthAndHeight(content)
	}

	truncated := content[:maxResponseChars]

	if lastNewline := strings.LastIndex(truncated, "\n"); lastNewline > maxResponseChars/2 {
		truncated = truncated[:lastNewline]
	}

	truncated += "\n\n... [Content truncated due to length] ..."

	return truncateWidthAndHeight(truncated)
}

func truncateWidthAndHeight(content string) string {
	lines := strings.Split(content, "\n")

	heightTruncated := false
	if len(lines) > maxResponseHeight {
		keepLines := maxResponseHeight - 3
		firstHalf := keepLines / 2
		secondHalf := keepLines - firstHalf

		truncatedLines := make([]string, 0, maxResponseHeight)
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
		if len(line) > maxResponseWidth {
			if maxResponseWidth > 20 {
				keepChars := maxResponseWidth - 10
				firstHalf := keepChars / 2
				secondHalf := keepChars - firstHalf
				lines[i] = line[:firstHalf] + " ... " + line[len(line)-secondHalf:]
			} else {
				lines[i] = line[:maxResponseWidth]
			}
			widthTruncated = true
		}
	}

	result := strings.Join(lines, "\n")

	if heightTruncated || widthTruncated {
		notices := make([]string, 0, 2)
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
