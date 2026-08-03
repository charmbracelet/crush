package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// a2uiFakeConn records written messages instead of sending them anywhere, so a
// test can drive a2uiInitConn.Write directly and inspect what would go on
// the wire.
type a2uiFakeConn struct {
	mcp.Connection
	written []*jsonrpc.Request
}

func (c *a2uiFakeConn) Write(_ context.Context, msg jsonrpc.Message) error {
	if req, ok := msg.(*jsonrpc.Request); ok {
		c.written = append(c.written, req)
	}
	return nil
}

// TestA2UIInitConnRewritesInitialize drives the injection against a
// representative initialize request and asserts the advertised capability
// shape — top-level a2ui key, version namespaced, compiled-in catalog — while
// leaving the rest of the params (roots, clientInfo) intact.
func TestA2UIInitConnRewritesInitialize(t *testing.T) {
	t.Parallel()

	fc := &a2uiFakeConn{}
	conn := &a2uiInitConn{Connection: fc}

	initParams, err := json.Marshal(map[string]any{
		"protocolVersion": "2025-11-25",
		"capabilities":    map[string]any{"roots": map[string]any{"listChanged": true}},
		"clientInfo":      map[string]any{"name": "crush", "version": "1.0.0"},
	})
	require.NoError(t, err)

	require.NoError(t, conn.Write(context.Background(), &jsonrpc.Request{
		Method: "initialize",
		Params: initParams,
	}))
	require.Len(t, fc.written, 1)

	var params map[string]any
	require.NoError(t, json.Unmarshal(fc.written[0].Params, &params))

	caps, ok := params["capabilities"].(map[string]any)
	require.True(t, ok, "initialize must carry a capabilities object")

	// The A2UI capability is a top-level key, per the spec's negotiation.
	a2uiCap, ok := caps["a2ui"].(map[string]any)
	require.True(t, ok, "capabilities must carry a top-level a2ui key, got: %v", caps)
	clientCaps, ok := a2uiCap["clientCapabilities"].(map[string]any)
	require.True(t, ok, "a2ui must carry clientCapabilities")
	v09, ok := clientCaps["v0.9"].(map[string]any)
	require.True(t, ok, "clientCapabilities must carry the a2ui version key")
	ids, ok := v09["supportedCatalogIds"].([]any)
	require.True(t, ok, "v0.9 must carry supportedCatalogIds")
	require.Len(t, ids, 1)
	require.Equal(t, A2UIBasicCatalogID, ids[0])

	// Existing capabilities and the rest of the handshake are preserved.
	roots, ok := caps["roots"].(map[string]any)
	require.True(t, ok, "roots must survive the merge")
	require.Equal(t, true, roots["listChanged"])
	require.Equal(t, "2025-11-25", params["protocolVersion"])
}

// TestA2UIInitConnPassesOtherMethods confirms non-initialize writes are not
// rewritten.
func TestA2UIInitConnPassesOtherMethods(t *testing.T) {
	t.Parallel()

	fc := &a2uiFakeConn{}
	conn := &a2uiInitConn{Connection: fc}
	require.NoError(t, conn.Write(context.Background(), &jsonrpc.Request{
		Method: "notifications/initialized",
		Params: json.RawMessage(`{}`),
	}))
	require.Len(t, fc.written, 1)
	require.JSONEq(t, `{}`, string(fc.written[0].Params))
}

func TestInjectA2UICapability(t *testing.T) {
	t.Parallel()

	t.Run("merges into an empty capabilities object", func(t *testing.T) {
		t.Parallel()
		raw := json.RawMessage(`{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"x"}}`)
		out := injectA2UICapability(raw)
		require.NotNil(t, out)
		var params map[string]any
		require.NoError(t, json.Unmarshal(out, &params))
		caps := params["capabilities"].(map[string]any)
		require.Contains(t, caps, "a2ui")
	})

	t.Run("creates capabilities when absent", func(t *testing.T) {
		t.Parallel()
		raw := json.RawMessage(`{"protocolVersion":"2025-11-25"}`)
		out := injectA2UICapability(raw)
		require.NotNil(t, out)
		var params map[string]any
		require.NoError(t, json.Unmarshal(out, &params))
		caps := params["capabilities"].(map[string]any)
		require.Contains(t, caps, "a2ui")
	})

	t.Run("respects a pre-existing a2ui key", func(t *testing.T) {
		t.Parallel()
		raw := json.RawMessage(`{"capabilities":{"a2ui":{"custom":true}}}`)
		require.Nil(t, injectA2UICapability(raw), "an explicit a2ui capability is never overwritten")
	})

	t.Run("leaves non-object params alone", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, injectA2UICapability(json.RawMessage(`null`)))
		require.Nil(t, injectA2UICapability(json.RawMessage(`"string"`)))
		require.Nil(t, injectA2UICapability(nil))
	})
}

func TestA2UIMetaCapabilitiesShape(t *testing.T) {
	t.Parallel()

	meta := a2uiMetaCapabilities()
	a2uiCap, ok := meta["a2ui"].(map[string]any)
	require.True(t, ok)
	clientCaps, ok := a2uiCap["clientCapabilities"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, clientCaps, "v0.9")
}
