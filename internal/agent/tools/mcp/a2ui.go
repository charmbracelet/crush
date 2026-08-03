package mcp

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	a2ui "github.com/tmc/a2ui"
)

// A2UIBasicCatalogID is the catalog the client advertises support for when
// negotiating A2UI over MCP. It matches the embedded basic catalog for the
// a2ui version crush compiles in (tmc/a2ui's a2uischema/schemas), so the
// advertised ID and the rendered catalog never drift apart.
const A2UIBasicCatalogID = "https://a2ui.org/specification/v0_9/catalogs/basic/catalog.json"

// a2uiClientCapabilities builds the A2UI capability payload a client
// declares to an A2UI-over-MCP server, per the spec's capability negotiation:
//
//	"a2ui": {"clientCapabilities": {"v0.9": {"supportedCatalogIds": [...]}}}
//
// The version key comes from a2ui.Version (the same constant the coder
// prompt uses), and the catalog list holds the compiled-in basic catalog, so
// this payload, the prompt, and the renderer share one source of truth.
func a2uiClientCapabilities() map[string]any {
	return map[string]any{
		"a2ui": map[string]any{
			"clientCapabilities": map[string]any{
				a2ui.Version: map[string]any{
					"supportedCatalogIds": []string{A2UIBasicCatalogID},
				},
			},
		},
	}
}

// a2uiMetaCapabilities returns the A2UI capability payload for the `_meta`
// field of an individual tools/call, the variant stateless servers expect
// when they cannot carry initialize-time state. The shape is the same as the
// initialize capability, hoisted under a single "a2ui" key.
func a2uiMetaCapabilities() map[string]any {
	return a2uiClientCapabilities()
}

// a2uiInitTransport wraps an mcp.Transport so the connection it yields
// injects the A2UI client capability into the outgoing initialize request.
// The go-sdk's typed ClientCapabilities has no slot for the spec's top-level
// "a2ui" capabilities key (its Experimental/Extensions maps nest under their
// own keys, which well-behaved servers do not read), so the capability is
// merged into the serialized initialize params at the connection layer — the
// same seam the channel transport uses to intercept the stream.
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
		return nil
	}
	var params map[string]any
	if err := json.Unmarshal(raw, &params); err != nil || params == nil {
		return nil
	}
	caps, ok := params["capabilities"].(map[string]any)
	if !ok {
		caps = map[string]any{}
	}
	if _, present := caps["a2ui"]; present {
		return nil
	}
	for k, v := range a2uiClientCapabilities() {
		caps[k] = v
	}
	params["capabilities"] = caps
	out, err := json.Marshal(params)
	if err != nil {
		return nil
	}
	return out
}
