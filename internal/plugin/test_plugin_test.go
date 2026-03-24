package plugin

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

func TestTestPluginKeepLastTwoMessages(t *testing.T) {
	Reset()
	defer Reset()

	// Create a test plugin that keeps only the last 2 messages
	p := &testPlugin{
		name: "test-plugin",
		hooks: Hooks{
			ChatMessagesTransform: func(ctx context.Context, input ChatMessagesTransformInput, output *ChatMessagesTransformOutput) error {
				// This simulates what our plugin does
				messages := output.Messages
				if len(messages) > 2 {
					output.Messages = messages[len(messages)-2:]
				}
				return nil
			},
		},
	}
	Register(p)

	err := Init(context.Background(), PluginInput{})
	require.NoError(t, err)

	// Create test messages
	testMessages := []message.Message{
		{ID: "msg-1", Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "Message 1"}}},
		{ID: "msg-2", Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "Message 2"}}},
		{ID: "msg-3", Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "Message 3"}}},
		{ID: "msg-4", Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "Message 4"}}},
		{ID: "msg-5", Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "Message 5"}}},
	}

	// Trigger the transform
	transformed, err := TriggerChatMessagesTransform(context.Background(), ChatMessagesTransformInput{
		SessionID: "test-session",
		Agent:     "coder",
	}, ChatMessagesTransformOutput{Messages: testMessages})

	require.NoError(t, err)
	require.Len(t, transformed.Messages, 2)
	require.Equal(t, "msg-4", transformed.Messages[0].ID)
	require.Equal(t, "msg-5", transformed.Messages[1].ID)
}
