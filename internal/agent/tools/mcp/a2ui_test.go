package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
	a2ui "github.com/tmc/a2ui"
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

// TestA2UIHandshakeReachesRealServer drives a real client/server initialize
// over the SDK's in-memory transport, wrapped the way createSession wraps it,
// and asserts what the SERVER actually observes — not what crush wrote.
//
// This is the gap the unit tests above cannot cover: they hand-build params
// and call Write directly, so they pass even when the capability never
// survives to the peer. It does not survive at the spec's top-level key,
// because the go-sdk unmarshals capabilities into a typed struct with no such
// field and no catch-all, which is exactly why the Extensions payload exists.
func TestA2UIHandshakeReachesRealServer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	srv := mcp.NewServer(&mcp.Implementation{Name: "probe", Version: "1"}, nil)
	go func() { _, _ = srv.Connect(ctx, serverTransport, nil) }()

	client := mcp.NewClient(
		&mcp.Implementation{Name: "crush", Version: "1"},
		&mcp.ClientOptions{Capabilities: a2uiSDKCapabilities(false)},
	)
	sess, err := client.Connect(ctx, &a2uiInitTransport{inner: clientTransport}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sess.Close() })

	var seen int
	for ss := range srv.Sessions() {
		seen++
		params := ss.InitializeParams()
		require.NotNil(t, params.Capabilities, "server must see client capabilities")
		ext, ok := params.Capabilities.Extensions[A2UIExtensionID]
		require.True(t, ok,
			"server must observe the A2UI capability under %q; extensions=%v",
			A2UIExtensionID, params.Capabilities.Extensions)

		// The payload survives intact, version key and catalog included.
		raw, err := json.Marshal(ext)
		require.NoError(t, err)
		var payload map[string]any
		require.NoError(t, json.Unmarshal(raw, &payload))
		clientCaps, ok := payload["clientCapabilities"].(map[string]any)
		require.True(t, ok, "extension payload must carry clientCapabilities, got %v", payload)
		_, ok = clientCaps[a2ui.Version]
		require.True(t, ok, "clientCapabilities must be keyed by %q, got %v", a2ui.Version, clientCaps)
	}
	require.Equal(t, 1, seen, "expected exactly one server session")
}

// TestA2UISDKCapabilitiesDisabled pins the opt-out: a deployment that set
// disable_a2ui must not advertise the capability at all.
func TestA2UISDKCapabilitiesDisabled(t *testing.T) {
	t.Parallel()
	require.Nil(t, a2uiSDKCapabilities(true))
}

// TestA2UIRequestMetaGates pins both halves of the per-request claim: the
// deployment opt-out (disable_a2ui) and the per-turn render check. Before
// these gates existed, a headless `crush run` advertised a2ui and got back a
// surface payload it folded into model-facing content as raw JSON.
func TestA2UIRequestMetaGates(t *testing.T) {
	t.Parallel()

	const enabled, disabled = false, true

	t.Run("enabled and capable claims a2ui", func(t *testing.T) {
		t.Parallel()
		meta := a2uiRequestMeta(context.Background(), enabled)
		require.NotNil(t, meta)
		require.Contains(t, map[string]any(meta), "a2ui")
	})
	t.Run("disable_a2ui suppresses the claim", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, a2uiRequestMeta(context.Background(), disabled))
	})
	t.Run("a turn that will not render suppresses the claim", func(t *testing.T) {
		t.Parallel()
		ctx := WithA2UICapable(context.Background(), false)
		require.Nil(t, a2uiRequestMeta(ctx, enabled),
			"headless and channel turns must not claim a capability crush will not honor")
	})
	t.Run("explicitly capable claims a2ui", func(t *testing.T) {
		t.Parallel()
		ctx := WithA2UICapable(context.Background(), true)
		require.NotNil(t, a2uiRequestMeta(ctx, enabled))
	})
}

// TestA2UIBasicCatalogIDTracksVersion pins the catalog ID to the compiled-in
// a2ui version. A dependency bump that moves a2ui.Version must move the
// advertised catalog with it, or crush advertises a stale catalog under a
// newer version key and a server negotiates against components it will not
// receive.
func TestA2UIBasicCatalogIDTracksVersion(t *testing.T) {
	t.Parallel()
	slug := strings.ReplaceAll(a2ui.Version, ".", "_")
	require.Contains(t, A2UIBasicCatalogID, "/"+slug+"/",
		"catalog ID must carry the compiled-in a2ui version %q", a2ui.Version)
}

// TestA2UIDisabledToleratesAbsentOptions pins that reading the A2UI opt-out
// never panics on a partially-populated config.
//
// Config.Options is a pointer, and createSession is reached from the OAuth
// re-auth flow (BeginAuth -> runAuthFlow -> connectAndRegister) with whatever
// store the caller held. A bare cfg.Config().Options.DisableA2UI crashed
// TestBeginAuth_Concurrent outright, taking the process down with it rather
// than failing one test.
func TestA2UIDisabledToleratesAbsentOptions(t *testing.T) {
	t.Parallel()

	require.False(t, a2uiDisabled(nil), "a nil store must read as not-disabled")
	require.False(t, a2uiDisabled(config.NewTestStore(nil)),
		"a store with no config must read as not-disabled")
	require.False(t, a2uiDisabled(config.NewTestStore(&config.Config{})),
		"a config with no Options must read as not-disabled")
	require.False(t, a2uiDisabled(config.NewTestStore(&config.Config{
		Options: &config.Options{},
	})), "an explicit zero Options means A2UI stays enabled")
	require.True(t, a2uiDisabled(config.NewTestStore(&config.Config{
		Options: &config.Options{DisableA2UI: true},
	})), "the opt-out must still be honored")
}
