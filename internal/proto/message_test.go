package proto

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMessageUnmarshalSkipsUnknownPartType covers a server newer than the
// client: a part type the client does not know must be dropped on its own,
// not fail the decode and take the message's other parts with it.
func TestMessageUnmarshalSkipsUnknownPartType(t *testing.T) {
	t.Parallel()

	const data = `{"id":"m-1","session_id":"s-1","role":"assistant","parts":[` +
		`{"type":"text","data":{"text":"hello"}},` +
		`{"type":"future_widget","data":{"blob":"x"}},` +
		`{"type":"finish","data":{"reason":"end_turn","time":1}}]}`

	var msg Message
	require.NoError(t, json.Unmarshal([]byte(data), &msg))
	require.Equal(t, "m-1", msg.ID)
	require.Equal(t, []ContentPart{
		TextContent{Text: "hello"},
		Finish{Reason: FinishReasonEndTurn, Time: 1},
	}, msg.Parts)

	// A session's history is decoded as one list, so the message must not
	// fail the messages around it either.
	const user = `{"id":"m-0","session_id":"s-1","role":"user","parts":[` +
		`{"type":"text","data":{"text":"hi"}}]}`
	var msgs []Message
	require.NoError(t, json.Unmarshal([]byte("["+user+","+data+"]"), &msgs))
	require.Len(t, msgs, 2)
	require.Equal(t, "hi", msgs[0].Content().Text)
	require.Equal(t, msg, msgs[1])
}
