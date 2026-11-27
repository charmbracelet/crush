package agent

import (
	"encoding/base64"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

func TestConvertToToolResult_NilResult(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name             string
		providerExecuted bool
	}{
		{name: "client"},
		{name: "provider", providerExecuted: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result := fantasy.ToolResultContent{
				ToolCallID:       "ws_123",
				ToolName:         "web_search",
				ClientMetadata:   "metadata",
				ProviderExecuted: testCase.providerExecuted,
			}

			tr := (&sessionAgent{}).convertToToolResult(result)
			require.Equal(t, result.ToolCallID, tr.ToolCallID)
			require.Equal(t, result.ToolName, tr.Name)
			require.Equal(t, result.ClientMetadata, tr.Metadata)
			require.Empty(t, tr.Content)
			require.False(t, tr.IsError)
		})
	}
}

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
