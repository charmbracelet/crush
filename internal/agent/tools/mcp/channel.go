package mcp

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"sync"
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

// metaKeyPattern restricts meta attribute keys to valid XML names: a letter
// or underscore followed by letters, digits, and underscores. Keys starting
// with a digit (e.g. "1chat") are not valid XML names and are dropped so
// they cannot produce structurally altered output. Hyphens and other
// characters are also rejected, preventing forged structural attributes.
var metaKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// reservedMetaKeys are attribute names the client controls; a server must not
// be able to override them via meta. This includes the XML namespace family
// (xmlns, xml) which encoding/xml would emit as namespace declarations.
var reservedMetaKeys = map[string]struct{}{
	"source": {},
	"xmlns":  {},
	"xml":    {},
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

// channelEnabled reports whether the given server name was opted in via the
// --channels flag. A server present in MCP config is not a channel until it is
// explicitly enabled, matching the "listed is not enabled" model. Entries may
// be written as "server:<name>" or as a bare "<name>".
func channelEnabled(enabled []string, name string) bool {
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
// an EventChannelMessage via must-deliver semantics. Channel notifications
// represent ordered inbound messages (chat, alerts) rather than disposable UI
// updates, so a stalled subscriber must not permanently lose them the way
// lossy Publish would. Malformed payloads are dropped (fail closed).
func publishChannelMessage(ctx context.Context, name string, raw json.RawMessage) {
	p, ok := parseChannelParams(raw)
	if !ok {
		slog.Warn("Dropping malformed channel notification", "server", name)
		return
	}
	broker.PublishMustDeliver(ctx, pubsub.CreatedEvent, Event{
		Type:           EventChannelMessage,
		Name:           name,
		ChannelMessage: renderChannel(name, p),
	})
}

// channelGateState is the lifecycle state of a channel gate.
//
// The gate starts in stateGateUndecided: notifications that arrive during
// MCP capability negotiation (between Connect and the gate resolution) are
// buffered, because a capable, opted-in server may push events immediately
// after initialize. Once negotiation completes, the gate transitions to
// stateGateOpen (publish + drain buffer) or stateGateClosed (discard
// buffer + drop).
type channelGateState int32

const (
	stateGateUndecided channelGateState = iota
	stateGateOpen
	stateGateClosed
)

// channelGate controls whether channel notifications are published, dropped,
// or buffered. During capability negotiation the gate is undecided and
// messages are buffered; once resolved, the buffer is drained or discarded.
type channelGate struct {
	state   atomic.Int32 // channelGateState
	mu      sync.Mutex
	pending []json.RawMessage
}

func newChannelGate() *channelGate {
	g := &channelGate{}
	g.state.Store(int32(stateGateUndecided))
	return g
}

// isOpen reports whether the gate has been resolved to open.
func (g *channelGate) isOpen() bool {
	return channelGateState(g.state.Load()) == stateGateOpen
}

// resolve transitions the gate from undecided to its final state. If open,
// buffered messages are returned for the caller to publish; if closed, the
// buffer is discarded. Calling resolve on an already-resolved gate is a
// no-op.
func (g *channelGate) resolve(open bool) []json.RawMessage {
	g.mu.Lock()
	defer g.mu.Unlock()
	if channelGateState(g.state.Load()) != stateGateUndecided {
		return nil
	}
	buffered := g.pending
	g.pending = nil
	if open {
		g.state.Store(int32(stateGateOpen))
		return buffered // drain the buffer for the caller to publish
	}
	g.state.Store(int32(stateGateClosed))
	return nil // discard the buffer
}

// accept handles a channel notification according to the gate state.
// Returns the params if the message should be published now, or nil if it
// was buffered or dropped.
func (g *channelGate) accept(raw json.RawMessage) json.RawMessage {
	switch channelGateState(g.state.Load()) {
	case stateGateOpen:
		return raw
	case stateGateClosed:
		return nil
	default: // undecided
		g.mu.Lock()
		defer g.mu.Unlock()
		// Re-check under the lock in case resolve ran between the load and
		// the lock acquisition.
		switch channelGateState(g.state.Load()) {
		case stateGateOpen:
			return raw
		case stateGateClosed:
			return nil
		}
		g.pending = append(g.pending, raw)
		return nil
	}
}

// channelTransport wraps an mcp.Transport so the client can intercept
// notifications/claude/channel messages. The go-sdk rejects unknown JSON-RPC
// methods before any client-side handler or middleware runs, so the only place
// to observe a custom notification is the transport's own connection.
type channelTransport struct {
	inner mcp.Transport
	name  string
	gate  *channelGate
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
	gate *channelGate
}

// Read intercepts notifications/claude/channel. Such messages are always
// removed from the stream handed to the SDK (which would otherwise reject the
// unknown method). During capability negotiation (gate undecided) they are
// buffered; once the gate is resolved they are published (open) or dropped
// (closed, fail closed).
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
		if raw := c.gate.accept(req.Params); raw != nil {
			publishChannelMessage(ctx, c.name, raw)
		}
	}
}
