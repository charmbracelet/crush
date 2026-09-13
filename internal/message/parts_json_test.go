package message

import (
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

// TestPartsRoundTrip pins the on-disk part encoding: every part type must
// survive a trip through marshalParts unchanged.
func TestPartsRoundTrip(t *testing.T) {
	t.Parallel()

	parts := []ContentPart{
		ReasoningContent{Thinking: "hmm", Signature: "sig", ToolID: "t1", StartedAt: 1, FinishedAt: 2},
		TextContent{Text: "hello"},
		ImageURLContent{URL: "https://example.test/a.png", Detail: "high"},
		BinaryContent{Path: "/tmp/a.bin", MIMEType: "application/octet-stream", Data: []byte{0, 1, 2}},
		ToolCall{ID: "c1", Name: "bash", Input: `{"cmd":"ls"}`, ProviderExecuted: true, Finished: true},
		ToolResult{ToolCallID: "c1", Name: "bash", Content: "out", MIMEType: "text/plain", IsError: true},
		Finish{Reason: FinishReasonEndTurn, Time: 42, Message: "done"},
		ShellCommand{Command: "ls -la", Output: "total 0", ExitCode: 0},
		SubAgentReport{Label: "auth-review", Output: "two findings", Failed: true},
	}

	encoded, err := marshalParts(parts)
	require.NoError(t, err)

	decoded, err := unmarshalParts(encoded)
	require.NoError(t, err)
	require.Equal(t, parts, decoded)
}

func TestUnmarshalPartsEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("empty array yields no parts", func(t *testing.T) {
		t.Parallel()

		got, err := unmarshalParts([]byte(`[]`))
		require.NoError(t, err)
		require.Empty(t, got)
	})

	t.Run("unknown type is an error", func(t *testing.T) {
		t.Parallel()

		_, err := unmarshalParts([]byte(`[{"type":"nope","data":{}}]`))
		require.ErrorContains(t, err, "unknown part type: nope")
	})

	t.Run("payload may precede its type tag", func(t *testing.T) {
		t.Parallel()

		// JSON does not guarantee key order.
		got, err := unmarshalParts([]byte(`[{"data":{"text":"hi"},"type":"text"}]`))
		require.NoError(t, err)
		require.Equal(t, []ContentPart{TextContent{Text: "hi"}}, got)
	})

	t.Run("malformed json is an error", func(t *testing.T) {
		t.Parallel()

		_, err := unmarshalParts([]byte(`{`))
		require.Error(t, err)
	})
}

// TestSubAgentReportToAIMessage pins what the model reads for a report: the
// part renders into a tagged block, since the message itself carries no text.
func TestSubAgentReportToAIMessage(t *testing.T) {
	t.Parallel()

	msg := Message{
		Role:  User,
		Parts: []ContentPart{SubAgentReport{Label: "auth-review", Output: "two findings"}},
	}

	aiMsgs := msg.ToAIMessage()
	require.Len(t, aiMsgs, 1)
	require.Len(t, aiMsgs[0].Content, 1)

	text, ok := aiMsgs[0].Content[0].(fantasy.TextPart)
	require.True(t, ok)
	require.Equal(t, "<sub-agent-report label=\"auth-review\">\ntwo findings\n</sub-agent-report>", text.Text)
}
