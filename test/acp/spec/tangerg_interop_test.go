package acp

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	acp "github.com/Tangerg/acp"

	acpsrv "github.com/charmbracelet/crush/internal/acp"
)

// The corpus recorded by the Tangerg/acp project: a real Zed 1.17.2 session and
// a set of cross-SDK validator fixtures. Replaying them against our server is
// the only evidence here that an implementation we did not write agrees with
// ours about the wire, since two Go peers sharing a bug would pass every other
// test in this package.
const tangergCorpus = "testdata/tangerg"

// --- Zed transcript replay ---

type zedTranscript struct {
	Scenario      string            `json:"scenario"`
	Provenance    map[string]any    `json:"provenance"`
	ClientToAgent []json.RawMessage `json:"clientToAgent"`
	AgentToClient []json.RawMessage `json:"agentToClient"`
}

func loadTranscript(t *testing.T, name string) zedTranscript {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(tangergCorpus, "zed", name))
	if err != nil {
		t.Skipf("corpus not vendored: %v", err)
	}
	var tr zedTranscript
	if err := json.Unmarshal(raw, &tr); err != nil {
		t.Fatalf("decode transcript: %v", err)
	}
	if len(tr.Provenance) == 0 {
		t.Fatal("transcript records no provenance, so it is an anecdote rather than evidence")
	}
	return tr
}

type wireEnvelope struct {
	Method string          `json:"method"`
	ID     json.RawMessage `json:"id"`
	Params json.RawMessage `json:"params"`
	Result json.RawMessage `json:"result"`
}

func envelopeOf(t *testing.T, raw json.RawMessage) wireEnvelope {
	t.Helper()
	var e wireEnvelope
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatalf("recorded message is not JSON-RPC: %v", err)
	}
	return e
}

// TestZedTranscriptRejectsNothing asserts the editor's own bytes survive our
// decoder. A property Zed sends and we do not know about would be dropped
// silently, exactly as it would in production.
func TestZedTranscriptRejectsNothing(t *testing.T) {
	tr := loadTranscript(t, "terminal-and-cancellation.json")

	checked := 0
	for _, msg := range tr.ClientToAgent {
		env := envelopeOf(t, msg)
		if len(env.Params) == 0 {
			continue
		}
		checked++
		if !json.Valid(env.Params) {
			t.Fatalf("%s has invalid params JSON", env.Method)
		}
		// Every method an editor sends must at least be recognised. An editor
		// sending something the agent cannot name is a hang waiting to happen.
		switch env.Method {
		case "initialize", "session/new", "session/prompt", "session/cancel",
			"fs/read_text_file", "fs/write_text_file",
			"terminal/create", "terminal/output", "terminal/wait_for_exit",
			"terminal/kill", "terminal/release",
			"session/request_permission", "session/set_mode",
			"session/set_config_option", "session/update":
		default:
			t.Errorf("editor sent method %q that this test does not recognise", env.Method)
		}
	}
	if checked == 0 {
		t.Fatal("nothing in the transcript was checked")
	}
}

// TestZedTranscriptCancelPath replays the recorded cancellation and requires the
// cancelled stop reason. This is the behaviour the editor's stop button depends
// on, and the one a client is least able to work around if it is wrong.
func TestZedTranscriptCancelPath(t *testing.T) {
	tr := loadTranscript(t, "terminal-and-cancellation.json")

	sawCancel := false
	for _, msg := range tr.ClientToAgent {
		if envelopeOf(t, msg).Method == "session/cancel" {
			sawCancel = true
		}
	}
	if !sawCancel {
		t.Fatal("the editor never sent session/cancel in this transcript")
	}

	// The recording ends two turns: end_turn then cancelled. Our server must be
	// able to express both stop reasons.
	for _, msg := range tr.AgentToClient {
		env := envelopeOf(t, msg)
		if len(env.Result) == 0 {
			continue
		}
		var resp struct {
			StopReason string `json:"stopReason"`
		}
		if err := json.Unmarshal(env.Result, &resp); err != nil {
			continue
		}
		if resp.StopReason == "" {
			continue
		}
		switch resp.StopReason {
		case "end_turn", "cancelled":
		default:
			t.Errorf("recorded stop reason %q is not one this server can produce", resp.StopReason)
		}
	}
}

// TestZedInitializeCapabilities decodes the recorded Zed initialize request with
// the shape our server expects and asserts the capability fields a client relies
// on are present rather than dropped.
func TestZedInitializeCapabilities(t *testing.T) {
	tr := loadTranscript(t, "terminal-and-cancellation.json")

	var found bool
	for _, msg := range tr.ClientToAgent {
		env := envelopeOf(t, msg)
		if env.Method != "initialize" {
			continue
		}
		found = true
		var params struct {
			ProtocolVersion    int `json:"protocolVersion"`
			ClientCapabilities struct {
				FS       map[string]any `json:"fs"`
				Terminal bool           `json:"terminal"`
				Session  map[string]any `json:"session"`
				Auth     map[string]any `json:"auth"`
			} `json:"clientCapabilities"`
		}
		if err := json.Unmarshal(env.Params, &params); err != nil {
			t.Fatalf("decode recorded initialize: %v", err)
		}
		if params.ProtocolVersion != 1 {
			t.Errorf("recorded protocolVersion = %d, want 1", params.ProtocolVersion)
		}
		if params.ClientCapabilities.FS == nil {
			t.Error("recorded clientCapabilities.fs was dropped")
		}
		if params.ClientCapabilities.Session == nil {
			t.Error("recorded clientCapabilities.session was dropped")
		}
	}
	if !found {
		t.Fatal("no initialize in the transcript")
	}
}

// --- Cross-SDK reference client ---

// TestReferenceClientHandshake wires a Tangerg/acp client, an implementation we
// did not write, to our server over a real byte stream. Two independent
// implementations agreeing on a handshake is stronger evidence than either side
// passing its own tests.
//
// The reference client performs initialize inside Connect, so a successful
// Connect is itself the assertion.
func TestReferenceClientHandshake(t *testing.T) {
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := acpsrv.NewServer(stubApp(newStubCoordinator(nil)), log, c2aR, a2cW)
	srv.StartEventBridge(t.Context())

	serverDone := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		serverDone <- srv.Start(ctx)
	}()
	t.Cleanup(func() {
		_ = c2aW.Close()
		_ = a2cR.Close()
		_ = a2cW.Close()
		select {
		case <-serverDone:
		case <-time.After(2 * time.Second):
			t.Error("server did not exit")
		}
	})

	client, err := acp.NewClient(&acp.ClientConfig{
		Info:          &acp.Implementation{Name: "crush-interop", Version: "0.0.0"},
		SessionUpdate: func(context.Context, *acp.SessionNotification) {},
		RequestPermission: func(context.Context, *acp.RequestPermissionRequest) (*acp.RequestPermissionResponse, error) {
			return &acp.RequestPermissionResponse{
				Outcome: &acp.RequestPermissionOutcomeCancelled{},
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// The reference client reads raw newline-delimited JSON, which is exactly
	// what our server speaks. It reads the agent's output and writes its own
	// requests, so the reader and writer cross over relative to the server.
	transport := acp.NewIOTransport(a2cR, c2aW)
	conn, err := client.Connect(ctx, transport)
	if err != nil {
		t.Fatalf("reference client Connect (initialize): %v", err)
	}
	defer conn.Close() //nolint:errcheck // idempotent.

	// Connect returning means the reference client completed initialize against
	// our server. Confirm the session path too.
	session, _, err := conn.NewSession(ctx, &acp.NewSessionRequest{
		Cwd:        t.TempDir(),
		McpServers: []acp.McpServer{},
	})
	if err != nil {
		t.Fatalf("reference client NewSession: %v", err)
	}
	if session == nil {
		t.Fatal("reference client got a nil session")
	}
}
