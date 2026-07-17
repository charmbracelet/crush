package mcp

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The channel contract lets an MCP server push events straight into the
// session as a <channel> element that the model reads on its next turn. See
// https://code.claude.com/docs/en/channels-reference for the authoritative
// spec. Crush plays the client role: it detects the capability a server
// declares, listens for the server-initiated notification, and injects the
// (validated, escaped) payload into the active session.
const (
	// channelCapability is the experimental capability key a server declares
	// (capabilities.experimental["claude/channel"] = {}) to become a channel.
	// Presence of the key is what turns on the notification listener.
	channelCapability = "claude/channel"

	// channelNotificationMethod is the JSON-RPC notification a channel server
	// emits to push an event. The go-sdk client cannot dispatch this custom
	// method itself, so it is intercepted at the transport layer (see
	// channelConn).
	channelNotificationMethod = "notifications/claude/channel"

	// maxChannelContentBytes caps a single channel payload body. Payloads are
	// untrusted, server-initiated input landing in model context, so an
	// oversized body is rejected outright rather than truncated.
	maxChannelContentBytes = 64 * 1024

	// maxChannelMetaEntries caps how many meta attributes a single payload may
	// carry. Extra entries are dropped.
	maxChannelMetaEntries = 32

	// maxChannelMetaValueBytes caps a single meta attribute value. Longer
	// values cause the entry to be dropped.
	maxChannelMetaValueBytes = 1024
)

// metaKeyPattern restricts meta attribute keys to identifiers: letters,
// digits, and underscores only. Keys with hyphens or any other character are
// silently dropped so they cannot be used to forge structural attributes on
// the <channel> tag.
var metaKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// reservedMetaKeys are attribute names the client controls; a server must not
// be able to override them via meta.
var reservedMetaKeys = map[string]struct{}{
	"source": {},
}

// channelParams is the wire shape of notifications/claude/channel params.
type channelParams struct {
	Content string            `json:"content"`
	Meta    map[string]string `json:"meta"`
}

// parseChannelParams validates and sanitises a raw notifications/claude/channel
// params object. It fails closed: any structural problem returns ok=false and
// the caller drops the notification. On success it returns a params value whose
// content is size-bounded and whose meta contains only well-formed,
// non-reserved, size-bounded entries.
func parseChannelParams(raw json.RawMessage) (channelParams, bool) {
	if len(raw) == 0 {
		return channelParams{}, false
	}

	// Reject unknown fields so a malformed or hostile payload cannot smuggle
	// unexpected structure past validation.
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	var p channelParams
	if err := dec.Decode(&p); err != nil {
		return channelParams{}, false
	}

	if p.Content == "" || len(p.Content) > maxChannelContentBytes {
		return channelParams{}, false
	}

	clean := channelParams{Content: p.Content}
	if len(p.Meta) > 0 {
		clean.Meta = make(map[string]string, len(p.Meta))
		for k, v := range p.Meta {
			if len(clean.Meta) >= maxChannelMetaEntries {
				break
			}
			if !metaKeyPattern.MatchString(k) {
				continue
			}
			if _, reserved := reservedMetaKeys[k]; reserved {
				continue
			}
			if len(v) > maxChannelMetaValueBytes {
				continue
			}
			clean.Meta[k] = v
		}
	}
	return clean, true
}

// renderChannel builds the safe <channel> element injected into the session.
// The source attribute is set from the (trusted) server name; the (untrusted)
// body and meta values are escaped by encoding/xml, so content cannot break out
// of the element or forge attributes. Meta keys are emitted in sorted order so
// the output is deterministic.
func renderChannel(source string, p channelParams) string {
	start := xml.StartElement{
		Name: xml.Name{Local: "channel"},
		Attr: make([]xml.Attr, 0, 1+len(p.Meta)),
	}
	start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "source"}, Value: source})

	keys := make([]string, 0, len(p.Meta))
	for k := range p.Meta {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		// k is already validated against metaKeyPattern.
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: k}, Value: p.Meta[k]})
	}

	var b strings.Builder
	enc := xml.NewEncoder(&b)
	// EncodeToken and Flush only fail on malformed tokens or a failing writer;
	// the tokens here are well-formed and strings.Builder never errors.
	_ = enc.EncodeToken(start)
	_ = enc.EncodeToken(xml.CharData(p.Content))
	_ = enc.EncodeToken(start.End())
	_ = enc.Flush()
	return b.String()
}

// hasChannelCapability reports whether a server's initialize result advertises
// the claude/channel experimental capability.
func hasChannelCapability(res *mcp.InitializeResult) bool {
	if res == nil || res.Capabilities == nil {
		return false
	}
	_, ok := res.Capabilities.Experimental[channelCapability]
	return ok
}

// ChannelEnabled reports whether the given server name was opted in via the
// --channels flag. A server present in MCP config is not a channel until it
// is explicitly enabled, matching the "listed is not enabled" model. Entries
// may be written as "server:<name>" or as a bare "<name>". Exported for the
// server backend, which routes channel events per workspace and needs the
// same opt-in semantics.
func ChannelEnabled(enabled []string, name string) bool {
	for _, e := range enabled {
		e = strings.TrimSpace(e)
		if e == name {
			return true
		}
		if strings.EqualFold(e, "server:"+name) {
			return true
		}
	}
	return false
}

// publishChannelMessage validates and renders a payload, then publishes it as
// an EventChannelMessage. Malformed payloads are dropped (fail closed).
func publishChannelMessage(name string, raw json.RawMessage) {
	p, ok := parseChannelParams(raw)
	if !ok {
		slog.Warn("Dropping malformed channel notification", "server", name)
		return
	}
	broker.Publish(pubsub.CreatedEvent, Event{
		Type:           EventChannelMessage,
		Name:           name,
		ChannelMeta:    p.Meta,
		ChannelMessage: renderChannel(name, p),
	})
}

// channelTransport wraps an mcp.Transport so the client can intercept
// notifications/claude/channel messages. The go-sdk rejects unknown JSON-RPC
// methods before any client-side handler or middleware runs, so the only place
// to observe a custom notification is the transport's own connection.
type channelTransport struct {
	inner mcp.Transport
	name  string
	gate  *atomic.Bool
}

// Connect implements mcp.Transport.
func (t *channelTransport) Connect(ctx context.Context) (mcp.Connection, error) {
	conn, err := t.inner.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &channelConn{Connection: conn, name: t.name, gate: t.gate}, nil
}

// channelConn wraps an mcp.Connection and filters channel notifications out of
// the stream the SDK sees, dispatching them to the channel handler instead.
type channelConn struct {
	mcp.Connection
	name string
	gate *atomic.Bool
}

// Read intercepts notifications/claude/channel. Such messages are always
// removed from the stream handed to the SDK (which would otherwise reject the
// unknown method); they are only acted on when this server is a confirmed,
// opted-in channel (gate set), and dropped otherwise (fail closed).
func (c *channelConn) Read(ctx context.Context) (jsonrpc.Message, error) {
	for {
		msg, err := c.Connection.Read(ctx)
		if err != nil {
			return msg, err
		}
		req, ok := msg.(*jsonrpc.Request)
		if !ok || req.IsCall() || req.Method != channelNotificationMethod {
			return msg, nil
		}
		if c.gate.Load() {
			publishChannelMessage(c.name, req.Params)
		}
	}
}
