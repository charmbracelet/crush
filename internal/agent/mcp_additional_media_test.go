package agent

import (
	"encoding/base64"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

func TestExtractAdditionalMCPMedia(t *testing.T) {
	t.Parallel()

	agent := &sessionAgent{}
	input := message.ToolResult{
		ToolCallID: "call-1",
		Name:       "mcp_test_tool",
		Metadata:   `{"safe_read_only":true,"mcp_additional_media":[{"type":"image","data":"AQID","media_type":"image/png"},{"type":"media","data":"BAUG","media_type":"audio/wav"}]}`,
	}

	base, additional := agent.extractAdditionalMCPMedia(input)
	require.Equal(t, "call-1", base.ToolCallID)
	require.Equal(t, `{"safe_read_only":true}`, base.Metadata)
	require.Len(t, additional, 2)
	require.Equal(t, "image/png", additional[0].MIMEType)
	require.Equal(t, []byte{1, 2, 3}, additional[0].Data)
	require.Equal(t, "audio/wav", additional[1].MIMEType)
	require.Equal(t, []byte{4, 5, 6}, additional[1].Data)
}

func TestExtractAdditionalMCPMediaInvalidBase64(t *testing.T) {
	t.Parallel()

	agent := &sessionAgent{}
	input := message.ToolResult{
		ToolCallID: "call-2",
		Name:       "mcp_test_tool",
		Metadata:   `{"mcp_additional_media":[{"type":"image","data":"%%%","media_type":"image/png"}]}`,
	}

	base, additional := agent.extractAdditionalMCPMedia(input)
	require.Equal(t, "", base.Metadata)
	require.Empty(t, additional)
}

func TestExtractAdditionalMCPMediaNoAdditionalPayload(t *testing.T) {
	t.Parallel()

	agent := &sessionAgent{}
	input := message.ToolResult{
		ToolCallID: "call-3",
		Name:       "mcp_test_tool",
		Metadata:   `{"safe_read_only":true}`,
	}

	base, additional := agent.extractAdditionalMCPMedia(input)
	require.Equal(t, input, base)
	require.Empty(t, additional)
}

func TestAdditionalMediaToolResultEncoding(t *testing.T) {
	t.Parallel()

	encoded := base64.StdEncoding.EncodeToString([]byte{1, 2, 3})
	require.Equal(t, "AQID", encoded)
}
