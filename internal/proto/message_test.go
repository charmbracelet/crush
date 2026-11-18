package proto

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMessageUnmarshalSkipsUnknownPartType guards that a part type from a
// newer server is dropped on its own instead of failing the whole decode.
func TestMessageUnmarshalSkipsUnknownPartType(t *testing.T) {
	t.Parallel()

	const data = `{"id":"m-1","role":"assistant","parts":[` +
		`{"type":"text","data":{"text":"hello"}},` +
		`{"type":"future_widget","data":{"blob":"x"}}]}`

	var msg Message
	require.NoError(t, json.Unmarshal([]byte(data), &msg))
	require.Equal(t, []ContentPart{TextContent{Text: "hello"}}, msg.Parts)
}
