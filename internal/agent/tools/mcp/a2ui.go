package mcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	a2ui "github.com/tmc/a2ui"
)

// A2UIBasicCatalogID is the catalog the client advertises support for when
// negotiating A2UI over MCP. The version segment is derived from a2ui.Version
// rather than hardcoded, so bumping the a2ui dependency cannot leave crush
// advertising a v0_9 catalog under a newer version key.
// TestA2UIBasicCatalogIDTracksVersion pins the relationship.
var A2UIBasicCatalogID = "https://a2ui.org/specification/" +
	strings.ReplaceAll(a2ui.Version, ".", "_") +
	"/catalogs/basic/catalog.json"

// a2uiClientCapabilities builds the A2UI capability payload a client
// declares to an A2UI-over-MCP server, per the spec's capability negotiation:
//
//	"a2ui": {"clientCapabilities": {"v0.9": {"supportedCatalogIds": [...]}}}
//
// The version key comes from a2ui.Version (the same constant the coder
// prompt uses), and the catalog list holds the compiled-in basic catalog, so
// this payload, the prompt, and the renderer share one source of truth.
func a2uiClientCapabilities() map[string]any {
	return map[string]any{"a2ui": a2uiCapabilityPayload()}
}

// a2uiCapabilityPayload is the capability body itself, without the "a2ui"
// wrapper key — the form the go-sdk's Extensions map wants, since the key
// there is the extension ID.
func a2uiCapabilityPayload() map[string]any {
	return map[string]any{
		"clientCapabilities": map[string]any{
			a2ui.Version: map[string]any{
				"supportedCatalogIds": []string{A2UIBasicCatalogID},
			},
		},
	}
}

// a2uiMetaCapabilities returns the A2UI capability payload for the `_meta`
// field of an individual tools/call, the variant stateless servers expect
// when they cannot carry initialize-time state. It is byte-identical to the
// initialize capability — same key, same shape — so the two can never drift.
func a2uiMetaCapabilities() map[string]any {
	return a2uiClientCapabilities()
}

// a2uiCapableKey marks a turn as one whose results crush will actually render
// as surfaces. The initialize handshake is a session-level statement ("this
// client CAN render A2UI"); this is the per-turn one ("this call will"). They
// differ because a single session serves both chat turns and headless or
// channel-originated turns, and only the former renders.
type a2uiCapableKey struct{}

// WithA2UICapable records whether the current turn will render A2UI surfaces.
// Callers that drive a real chat UI leave it unset (the default is capable);
// headless and channel-originated turns must set it false so crush does not
// claim a capability it will not honor and get a surface it cannot draw.
func WithA2UICapable(ctx context.Context, capable bool) context.Context {
	return context.WithValue(ctx, a2uiCapableKey{}, capable)
}

// a2uiCapable reports whether this turn renders A2UI. Unset means capable, so
// direct UI-initiated calls (the a2ui_action round-trip) keep working.
func a2uiCapable(ctx context.Context) bool {
	capable, ok := ctx.Value(a2uiCapableKey{}).(bool)
	return !ok || capable
}

// a2uiRequestMeta returns the `_meta` an outgoing request should carry, or
// nil when crush must not claim A2UI for it. Every RPC that can return a
// surface goes through here — tools/call and resources/read alike — so a
// stateless server keying off the per-request capability cannot see crush
// claim A2UI on one RPC and stay silent on the other.
func a2uiRequestMeta(ctx context.Context, disableA2UI bool) mcp.Meta {
	if disableA2UI || !a2uiCapable(ctx) {
		return nil
	}
	return mcp.Meta(a2uiMetaCapabilities())
}

// A2UIExtensionID is the extensions key carrying the A2UI capability, in the
// go-sdk's required "{vendor-prefix}/{extension-name}" form.
const A2UIExtensionID = "a2ui.org/a2ui"

// a2uiSDKCapabilities returns the typed client capabilities to hand the SDK,
// or nil when A2UI is disabled.
//
// This is deliberately belt-and-braces with the wire-level injection below.
// The spec puts A2UI at a top-level "capabilities.a2ui" key, but the go-sdk
// deserializes a peer's capabilities into a typed ClientCapabilities struct
// that has no such field and no catch-all — so a go-sdk SERVER silently drops
// the top-level key and never sees the claim. Extensions is the SDK's
// sanctioned slot for exactly this and survives the round-trip intact.
// Sending both means spec-conformant servers read the top-level key and
// go-sdk servers read the extension; neither path alone reaches everyone.
func a2uiSDKCapabilities(disableA2UI bool) *mcp.ClientCapabilities {
	if disableA2UI {
		return nil
	}
	caps := &mcp.ClientCapabilities{}
	caps.AddExtension(A2UIExtensionID, a2uiCapabilityPayload())
	return caps
}

// a2uiInitTransport wraps an mcp.Transport so the connection it yields
// injects the A2UI client capability into the outgoing initialize request at
// the spec's top-level "capabilities.a2ui" key. The go-sdk's typed
// ClientCapabilities has no slot for that key, so it is merged into the
// serialized params at the connection layer — the same seam the channel
// transport uses to intercept the stream.
//
// This covers spec-conformant servers that read the raw capabilities object.
// It does NOT reach a go-sdk server, which unmarshals into the typed struct
// and drops the unknown key; a2uiSDKCapabilities carries the same payload in
// Extensions for those. Both are sent, on purpose.
type a2uiInitTransport struct {
	inner mcp.Transport
}

// Connect implements mcp.Transport.
func (t *a2uiInitTransport) Connect(ctx context.Context) (mcp.Connection, error) {
	conn, err := t.inner.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &a2uiInitConn{Connection: conn}, nil
}

// a2uiInitConn injects the A2UI capability into the initialize request's
// params before they go on the wire.
type a2uiInitConn struct {
	mcp.Connection
}

// initializeMethod is the JSON-RPC method carrying the client's capabilities.
const initializeMethod = "initialize"

// Write intercepts the initialize request and merges the A2UI capability
// into its params.capabilities before it goes on the wire. Any other message
// passes through untouched; a params payload that does not decode as an
// object is left alone rather than failing the handshake.
func (c *a2uiInitConn) Write(ctx context.Context, msg jsonrpc.Message) error {
	req, ok := msg.(*jsonrpc.Request)
	if !ok || req.Method != initializeMethod {
		return c.Connection.Write(ctx, msg)
	}
	params := injectA2UICapability(req.Params)
	if params != nil {
		req.Params = params
	}
	return c.Connection.Write(ctx, msg)
}

// injectA2UICapability merges the A2UI capability into an initialize
// request's params JSON, returning the rewritten params or nil when no
// rewrite was possible (params absent or not an object). A pre-existing
// "a2ui" key is respected, never overwritten.
func injectA2UICapability(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		slog.Warn("A2UI capability not advertised: initialize params were empty")
		return nil
	}
	var params map[string]any
	if err := json.Unmarshal(raw, &params); err != nil || params == nil {
		slog.Warn("A2UI capability not advertised: initialize params did not decode as an object", "error", err)
		return nil
	}
	caps, ok := params["capabilities"].(map[string]any)
	if !ok {
		caps = map[string]any{}
	}
	if _, present := caps["a2ui"]; present {
		// Already advertised by something upstream; leave it alone.
		return nil
	}
	for k, v := range a2uiClientCapabilities() {
		caps[k] = v
	}
	params["capabilities"] = caps
	out, err := json.Marshal(params)
	if err != nil {
		slog.Warn("A2UI capability not advertised: re-encoding initialize params failed", "error", err)
		return nil
	}
	return out
}

// unwrapTransport implements [transportWrapper], so wrapping the transport
// for A2UI never hides the stdio startup diagnostics maybeStdioErr digs out.
func (t *a2uiInitTransport) unwrapTransport() mcp.Transport { return t.inner }
