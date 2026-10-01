package herdr

import (
	"testing"

	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTranslateProtoSummaryMessage covers client/server mode, where the proto
// branch hardcoded isSummary and so never reported compaction to herdr even
// though the domain branch did.
func TestTranslateProtoSummaryMessage(t *testing.T) {
	t.Parallel()

	summary := pubsub.Event[proto.Message]{
		Payload: proto.Message{Role: proto.Assistant, SessionID: "s-1", IsSummaryMessage: true},
	}
	assert.Equal(t, Summarizing{}, Translate(summary))

	normal := pubsub.Event[proto.Message]{
		Payload: proto.Message{Role: proto.Assistant, SessionID: "s-1"},
	}
	event := Translate(normal)
	require.IsType(t, AssistantMessage{}, event)
	assert.Equal(t, "s-1", event.(AssistantMessage).SessionID)
}
