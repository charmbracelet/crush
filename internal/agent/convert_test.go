package agent

import (
	"encoding/base64"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

func TestConvertToToolResult_InvalidBase64(t *testing.T) {
	t.Parallel()

	a := &sessionAgent{}
	result := fantasy.ToolResultContent{
		ToolCallID: "call_123",
		ToolName:   "test_tool",
		Result: fantasy.ToolResultOutputContentMedia{
			Data:      "abc\x80def",
			MediaType: "image/png",
		},
	}

	tr := a.convertToToolResult(result)
	require.True(t, tr.IsError)
	require.Empty(t, tr.Data)
	require.Contains(t, tr.Content, "invalid encoding")
	require.Equal(t, "call_123", tr.ToolCallID)
	require.Equal(t, "test_tool", tr.Name)
}

func TestConvertToToolResult_ValidMedia(t *testing.T) {
	t.Parallel()

	a := &sessionAgent{}
	validData := base64.StdEncoding.EncodeToString([]byte("test image data"))

	result := fantasy.ToolResultContent{
		ToolCallID: "call_456",
		ToolName:   "screenshot",
		Result: fantasy.ToolResultOutputContentMedia{
			Data:      validData,
			MediaType: "image/png",
			Text:      "Screenshot captured",
		},
	}

	tr := a.convertToToolResult(result)
	require.False(t, tr.IsError)
	require.Equal(t, validData, tr.Data)
	require.Equal(t, "image/png", tr.MIMEType)
	require.Equal(t, "Screenshot captured", tr.Content)
}

func TestConvertToToolResult_ValidMediaNoText(t *testing.T) {
	t.Parallel()

	a := &sessionAgent{}
	validData := base64.StdEncoding.EncodeToString([]byte("test image data"))

	result := fantasy.ToolResultContent{
		ToolCallID: "call_789",
		ToolName:   "view",
		Result: fantasy.ToolResultOutputContentMedia{
			Data:      validData,
			MediaType: "image/jpeg",
		},
	}

	tr := a.convertToToolResult(result)
	require.False(t, tr.IsError)
	require.Equal(t, validData, tr.Data)
	require.Equal(t, "image/jpeg", tr.MIMEType)
	require.Equal(t, "Loaded image/jpeg content", tr.Content)
}

func TestConvertToToolResult_ASCIIButInvalidBase64(t *testing.T) {
	t.Parallel()

	a := &sessionAgent{}
	result := fantasy.ToolResultContent{
		ToolCallID: "call_abc",
		ToolName:   "mcp_tool",
		Result: fantasy.ToolResultOutputContentMedia{
			Data:      "not-valid-base64!!!",
			MediaType: "image/png",
		},
	}

	tr := a.convertToToolResult(result)
	require.True(t, tr.IsError)
	require.Empty(t, tr.Data)
	require.Contains(t, tr.Content, "invalid encoding")
}
