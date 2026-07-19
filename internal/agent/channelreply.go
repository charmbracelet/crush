package agent

import (
	"cmp"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
)

// channelReplyTimeout bounds the auto-reply tool call so a wedged MCP
// server cannot hold the finished turn (and the session's busy state)
// open indefinitely.
const channelReplyTimeout = 30 * time.Second

// Default meta attributes identifying the reply target for each route.
// They match the channel contract used by messaging servers (e.g. Signal
// MCP): "sender" carries the author of a direct push, "group" is present
// only on group pushes.
const (
	defaultUserTargetMeta  = "sender"
	defaultGroupTargetMeta = "group"
	defaultMessageParam    = "message"
)

// parseChannelMeta extracts the attributes of the <channel> element a
// channel-originated turn was started with. The prompt of such a turn is
// exactly the element rendered by the MCP layer (renderChannel), so this
// is the inverse of that rendering. Returns ok=false when the prompt does
// not start with a <channel> element.
func parseChannelMeta(prompt string) (map[string]string, bool) {
	dec := xml.NewDecoder(strings.NewReader(prompt))
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, false
		}
		switch t := tok.(type) {
		case xml.CharData:
			if strings.TrimSpace(string(t)) != "" {
				return nil, false
			}
		case xml.StartElement:
			if t.Name.Local != "channel" {
				return nil, false
			}
			meta := make(map[string]string, len(t.Attr))
			for _, attr := range t.Attr {
				meta[attr.Name.Local] = attr.Value
			}
			return meta, true
		default:
			return nil, false
		}
	}
}

// resolveChannelReply picks the tool call that delivers a reply for a push
// with the given meta. Group routes win over user routes when the push
// carries the group target attribute, so a message received in a group is
// answered in that group rather than as a DM to its author.
func resolveChannelReply(reply *config.MCPChannelReply, meta map[string]string, text string) (tool string, args map[string]any, ok bool) {
	msgParam := cmp.Or(reply.MessageParam, defaultMessageParam)
	route := func(r *config.MCPChannelReplyRoute, defaultMeta string) (string, map[string]any, bool) {
		if r == nil || r.Tool == "" || r.TargetParam == "" {
			return "", nil, false
		}
		target := meta[cmp.Or(r.TargetMeta, defaultMeta)]
		if target == "" {
			return "", nil, false
		}
		return r.Tool, map[string]any{r.TargetParam: target, msgParam: text}, true
	}
	if tool, args, ok := route(reply.Group, defaultGroupTargetMeta); ok {
		return tool, args, ok
	}
	return route(reply.User, defaultUserTargetMeta)
}

// channelReplyDelivered reports whether the model already delivered a reply
// through the channel itself during the turn: completedTools holds the full
// names (mcp_<server>_<tool>) of tool calls that finished without error, and
// a call to either route tool or any configured suppress tool counts.
func channelReplyDelivered(reply *config.MCPChannelReply, channel string, completedTools map[string]struct{}) bool {
	names := make([]string, 0, 2+len(reply.SuppressTools))
	if reply.User != nil {
		names = append(names, reply.User.Tool)
	}
	if reply.Group != nil {
		names = append(names, reply.Group.Tool)
	}
	names = append(names, reply.SuppressTools...)
	for _, name := range names {
		if name == "" {
			continue
		}
		if _, ok := completedTools[fmt.Sprintf("mcp_%s_%s", channel, name)]; ok {
			return true
		}
	}
	return false
}

// sendChannelReply routes the final assistant text of a channel-originated
// turn back through the channel's configured reply tool. It is a no-op for
// local turns, channels without a channel_reply config, empty responses,
// and turns where the model already replied on the channel itself. Failures
// are logged and dropped: the turn has finished and there is no caller to
// return an error to.
func (a *sessionAgent) sendChannelReply(ctx context.Context, call SessionAgentCall, text string, completedTools map[string]struct{}) {
	if call.Channel == "" || a.cfg == nil {
		return
	}
	mcpCfg, ok := a.cfg.Config().MCP[call.Channel]
	if !ok || mcpCfg.ChannelReply == nil {
		return
	}
	reply := mcpCfg.ChannelReply
	if channelReplyDelivered(reply, call.Channel, completedTools) {
		slog.Debug("Channel reply already delivered by the model", "channel", call.Channel)
		return
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if call.channelMeta == nil {
		slog.Warn("Channel reply skipped: prompt has no channel metadata", "channel", call.Channel)
		return
	}
	tool, args, ok := resolveChannelReply(reply, call.channelMeta, text)
	if !ok {
		slog.Warn("Channel reply skipped: no reply route matches the push metadata", "channel", call.Channel)
		return
	}
	input, err := json.Marshal(args)
	if err != nil {
		slog.Error("Channel reply skipped: failed to encode tool arguments", "channel", call.Channel, "tool", tool, "error", err)
		return
	}
	// Detach from the run context: the turn is complete, and a cancel
	// racing this send must not lose the reply.
	sendCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), channelReplyTimeout)
	defer cancel()
	if _, err := mcp.RunTool(sendCtx, a.cfg, call.Channel, tool, string(input)); err != nil {
		slog.Error("Channel reply failed", "channel", call.Channel, "tool", tool, "error", err)
		return
	}
	slog.Info("Routed reply to originating channel", "channel", call.Channel, "tool", tool)
}
