package agent

import (
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
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

// signalTools returns a mock tool list resembling a Signal MCP server.
func signalTools() []*mcp.Tool {
	return []*mcp.Tool{
		{
			Name: "send_message_to_user",
			InputSchema: map[string]any{
				"properties": map[string]any{
					"message": map[string]any{"type": "string"},
					"user_id": map[string]any{"type": "string"},
				},
			},
		},
		{
			Name: "send_message_to_group",
			InputSchema: map[string]any{
				"properties": map[string]any{
					"message":  map[string]any{"type": "string"},
					"group_id": map[string]any{"type": "string"},
				},
			},
		},
		{
			Name: "mark_read",
			InputSchema: map[string]any{
				"properties": map[string]any{
					"sender":           map[string]any{"type": "string"},
					"target_timestamp": map[string]any{"type": "integer"},
				},
			},
		},
	}
}

func TestDiscoverReplyFromTools(t *testing.T) {
	t.Parallel()

	t.Run("signal server discovers user and group routes", func(t *testing.T) {
		t.Parallel()
		reply := discoverReplyFromTools(signalTools())
		require.NotNil(t, reply)
		require.NotNil(t, reply.User)
		require.Equal(t, "send_message_to_user", reply.User.Tool)
		require.Equal(t, "user_id", reply.User.TargetParam)
		require.NotNil(t, reply.Group)
		require.Equal(t, "send_message_to_group", reply.Group.Tool)
		require.Equal(t, "group_id", reply.Group.TargetParam)
		require.Equal(t, "message", reply.MessageParam)
	})

	t.Run("only user tool available", func(t *testing.T) {
		t.Parallel()
		tools := []*mcp.Tool{
			{
				Name: "send_message",
				InputSchema: map[string]any{
					"properties": map[string]any{
						"message": map[string]any{"type": "string"},
						"user_id": map[string]any{"type": "string"},
					},
				},
			},
		}
		reply := discoverReplyFromTools(tools)
		require.NotNil(t, reply)
		require.NotNil(t, reply.User)
		require.Equal(t, "send_message", reply.User.Tool)
		require.Nil(t, reply.Group)
	})

	t.Run("no message param skips tool", func(t *testing.T) {
		t.Parallel()
		tools := []*mcp.Tool{
			{
				Name: "echo",
				InputSchema: map[string]any{
					"properties": map[string]any{
						"text": map[string]any{"type": "string"},
					},
				},
			},
		}
		reply := discoverReplyFromTools(tools)
		require.Nil(t, reply)
	})

	t.Run("no target param skips tool", func(t *testing.T) {
		t.Parallel()
		tools := []*mcp.Tool{
			{
				Name: "log",
				InputSchema: map[string]any{
					"properties": map[string]any{
						"message": map[string]any{"type": "string"},
						"level":   map[string]any{"type": "string"},
					},
				},
			},
		}
		reply := discoverReplyFromTools(tools)
		require.Nil(t, reply)
	})

	t.Run("empty tool list", func(t *testing.T) {
		t.Parallel()
		reply := discoverReplyFromTools(nil)
		require.Nil(t, reply)
	})

	t.Run("resolves user via 'to' param", func(t *testing.T) {
		t.Parallel()
		tools := []*mcp.Tool{
			{
				Name: "dm",
				InputSchema: map[string]any{
					"properties": map[string]any{
						"message": map[string]any{"type": "string"},
						"to":      map[string]any{"type": "string"},
					},
				},
			},
		}
		reply := discoverReplyFromTools(tools)
		require.NotNil(t, reply)
		require.NotNil(t, reply.User)
		require.Equal(t, "dm", reply.User.Tool)
		require.Equal(t, "to", reply.User.TargetParam)
	})

	t.Run("resolves group via 'room' param", func(t *testing.T) {
		t.Parallel()
		tools := []*mcp.Tool{
			{
				Name: "room_msg",
				InputSchema: map[string]any{
					"properties": map[string]any{
						"message": map[string]any{"type": "string"},
						"room":    map[string]any{"type": "string"},
					},
				},
			},
		}
		reply := discoverReplyFromTools(tools)
		require.NotNil(t, reply)
		require.NotNil(t, reply.Group)
		require.Equal(t, "room_msg", reply.Group.Tool)
		require.Equal(t, "room", reply.Group.TargetParam)
	})

	t.Run("explicit channel_reply config takes precedence", func(t *testing.T) {
		t.Parallel()
		// Verify that the auto-discovered reply matches expectations
		// when an explicit config is also available. The config-driven
		// path is tested in TestResolveChannelReply.
		reply := discoverReplyFromTools(signalTools())
		require.NotNil(t, reply)

		// Resolve a DM push via the auto-discovered route.
		tool, args, ok := resolveChannelReply(reply, map[string]string{"sender": "+15551234567"}, "hello")
		require.True(t, ok)
		require.Equal(t, "send_message_to_user", tool)
		require.Equal(t, map[string]any{"user_id": "+15551234567", "message": "hello"}, args)

		// Resolve a group push via the auto-discovered route.
		meta := map[string]string{"sender": "+15551234567", "group": "grp=="}
		tool, args, ok = resolveChannelReply(reply, meta, "hello group")
		require.True(t, ok)
		require.Equal(t, "send_message_to_group", tool)
		require.Equal(t, map[string]any{"group_id": "grp==", "message": "hello group"}, args)
	})
}

func TestDiscoverReplyFromTools_DeliveredCheck(t *testing.T) {
	t.Parallel()
	reply := discoverReplyFromTools(signalTools())
	require.NotNil(t, reply)

	completed := func(names ...string) map[string]struct{} {
		set := make(map[string]struct{}, len(names))
		for _, n := range names {
			set[n] = struct{}{}
		}
		return set
	}

	// The model called the discovered tool — counts as delivered.
	require.True(t, autoReplyDelivered(reply, "signal", completed("mcp_signal_send_message_to_user")))
	require.True(t, autoReplyDelivered(reply, "signal", completed("mcp_signal_send_message_to_group")))
	// A different tool does not count.
	require.False(t, autoReplyDelivered(reply, "signal", completed("mcp_signal_mark_read")))
	// Empty set.
	require.False(t, autoReplyDelivered(reply, "signal", completed()))
	// Nil reply.
	require.False(t, autoReplyDelivered(nil, "signal", completed("mcp_signal_send")))
}

func TestDiscoverReplyFromTools_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("nil input schema", func(t *testing.T) {
		t.Parallel()
		tools := []*mcp.Tool{
			{
				Name:        "send",
				InputSchema: nil,
			},
		}
		reply := discoverReplyFromTools(tools)
		require.Nil(t, reply)
	})

	t.Run("empty input schema", func(t *testing.T) {
		t.Parallel()
		tools := []*mcp.Tool{
			{
				Name:        "send",
				InputSchema: map[string]any{},
			},
		}
		reply := discoverReplyFromTools(tools)
		require.Nil(t, reply)
	})

	t.Run("message param is not string type", func(t *testing.T) {
		t.Parallel()
		tools := []*mcp.Tool{
			{
				Name: "send",
				InputSchema: map[string]any{
					"properties": map[string]any{
						"message": map[string]any{"type": "integer"},
						"user_id": map[string]any{"type": "string"},
					},
				},
			},
		}
		reply := discoverReplyFromTools(tools)
		require.Nil(t, reply)
	})

	t.Run("tool with no name is skipped", func(t *testing.T) {
		t.Parallel()
		tools := []*mcp.Tool{
			{
				Name: "",
				InputSchema: map[string]any{
					"properties": map[string]any{
						"message": map[string]any{"type": "string"},
						"user_id": map[string]any{"type": "string"},
					},
				},
			},
		}
		reply := discoverReplyFromTools(tools)
		require.Nil(t, reply)
	})
}
