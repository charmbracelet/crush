package agent

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestParseChannelMeta(t *testing.T) {
	t.Parallel()

	t.Run("channel element with attributes", func(t *testing.T) {
		t.Parallel()
		meta, ok := parseChannelMeta(`<channel source="signal" sender="+15551234567" sender_name="Joe">Hello?</channel>`)
		require.True(t, ok)
		require.Equal(t, map[string]string{
			"source":      "signal",
			"sender":      "+15551234567",
			"sender_name": "Joe",
		}, meta)
	})

	t.Run("escaped attribute values", func(t *testing.T) {
		t.Parallel()
		meta, ok := parseChannelMeta(`<channel source="signal" sender_name="A &amp; B">hi</channel>`)
		require.True(t, ok)
		require.Equal(t, "A & B", meta["sender_name"])
	})

	t.Run("leading whitespace tolerated", func(t *testing.T) {
		t.Parallel()
		meta, ok := parseChannelMeta("\n  <channel source=\"signal\" sender=\"+1\">x</channel>")
		require.True(t, ok)
		require.Equal(t, "+1", meta["sender"])
	})

	t.Run("plain prompt is not a channel push", func(t *testing.T) {
		t.Parallel()
		_, ok := parseChannelMeta("fix the login flow")
		require.False(t, ok)
	})

	t.Run("other element is not a channel push", func(t *testing.T) {
		t.Parallel()
		_, ok := parseChannelMeta(`<task source="signal">x</task>`)
		require.False(t, ok)
	})

	t.Run("empty prompt", func(t *testing.T) {
		t.Parallel()
		_, ok := parseChannelMeta("")
		require.False(t, ok)
	})
}

func signalChannelReply() *config.MCPChannelReply {
	return &config.MCPChannelReply{
		User:  &config.MCPChannelReplyRoute{Tool: "send_message_to_user", TargetParam: "user_id"},
		Group: &config.MCPChannelReplyRoute{Tool: "send_message_to_group", TargetParam: "group_id"},
	}
}

func TestResolveChannelReply(t *testing.T) {
	t.Parallel()

	t.Run("direct message routes to the sender", func(t *testing.T) {
		t.Parallel()
		tool, args, ok := resolveChannelReply(signalChannelReply(), map[string]string{"sender": "+15551234567"}, "hi")
		require.True(t, ok)
		require.Equal(t, "send_message_to_user", tool)
		require.Equal(t, map[string]any{"user_id": "+15551234567", "message": "hi"}, args)
	})

	t.Run("group message routes to the group even with a sender", func(t *testing.T) {
		t.Parallel()
		meta := map[string]string{"sender": "+15551234567", "group": "grp=="}
		tool, args, ok := resolveChannelReply(signalChannelReply(), meta, "hi")
		require.True(t, ok)
		require.Equal(t, "send_message_to_group", tool)
		require.Equal(t, map[string]any{"group_id": "grp==", "message": "hi"}, args)
	})

	t.Run("custom target meta and message param", func(t *testing.T) {
		t.Parallel()
		reply := &config.MCPChannelReply{
			MessageParam: "text",
			User:         &config.MCPChannelReplyRoute{Tool: "post", TargetParam: "to", TargetMeta: "author"},
		}
		tool, args, ok := resolveChannelReply(reply, map[string]string{"author": "joe"}, "yo")
		require.True(t, ok)
		require.Equal(t, "post", tool)
		require.Equal(t, map[string]any{"to": "joe", "text": "yo"}, args)
	})

	t.Run("no matching meta yields no route", func(t *testing.T) {
		t.Parallel()
		_, _, ok := resolveChannelReply(signalChannelReply(), map[string]string{"source": "signal"}, "hi")
		require.False(t, ok)
	})

	t.Run("group push without a group route falls back to user", func(t *testing.T) {
		t.Parallel()
		reply := &config.MCPChannelReply{
			User: &config.MCPChannelReplyRoute{Tool: "send_message_to_user", TargetParam: "user_id"},
		}
		meta := map[string]string{"sender": "+15551234567", "group": "grp=="}
		tool, _, ok := resolveChannelReply(reply, meta, "hi")
		require.True(t, ok)
		require.Equal(t, "send_message_to_user", tool)
	})

	t.Run("incomplete route is skipped", func(t *testing.T) {
		t.Parallel()
		reply := &config.MCPChannelReply{
			User: &config.MCPChannelReplyRoute{Tool: "send_message_to_user"}, // no target_param
		}
		_, _, ok := resolveChannelReply(reply, map[string]string{"sender": "+1"}, "hi")
		require.False(t, ok)
	})
}

func TestChannelReplyDelivered(t *testing.T) {
	t.Parallel()
	reply := signalChannelReply()
	reply.SuppressTools = []string{"send"}

	completed := func(names ...string) map[string]struct{} {
		set := make(map[string]struct{}, len(names))
		for _, n := range names {
			set[n] = struct{}{}
		}
		return set
	}

	require.True(t, channelReplyDelivered(reply, "signal", completed("mcp_signal_send_message_to_user")))
	require.True(t, channelReplyDelivered(reply, "signal", completed("mcp_signal_send_message_to_group")))
	require.True(t, channelReplyDelivered(reply, "signal", completed("mcp_signal_send")))
	require.False(t, channelReplyDelivered(reply, "signal", completed("mcp_signal_mark_read", "bash")))
	// A same-named tool on a different server does not count as a reply.
	require.False(t, channelReplyDelivered(reply, "signal", completed("mcp_other_send_message_to_user")))
	require.False(t, channelReplyDelivered(reply, "signal", completed()))
}
