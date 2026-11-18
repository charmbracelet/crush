package model

import (
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/stretchr/testify/require"
)

// A retried response drops the calls its failed attempt streamed, so their
// items go too instead of spinning forever.
func TestRetryDropsStaleToolItems(t *testing.T) {
	t.Parallel()
	u := newTestUI()
	sty := u.com.Styles
	kept := message.ToolCall{ID: "kept", Name: "bash", Input: `{"command":"ls"}`}
	u.chat.AppendMessages(chat.NewToolMessageItem(sty, "m0", message.ToolCall{ID: "other", Name: "write"}, nil, false, "/tmp"))
	for _, tc := range []message.ToolCall{kept, {ID: "gone", Name: "write"}} {
		u.chat.AppendMessages(chat.NewToolMessageItem(sty, "m1", tc, nil, false, "/tmp"))
	}

	u.chat.RemoveStaleToolCalls("m1", []message.ToolCall{kept})

	require.NotNil(t, u.chat.MessageItem("kept"))
	require.Nil(t, u.chat.MessageItem("gone"))
	require.NotNil(t, u.chat.MessageItem("other"), "another message's calls are left alone")
}
