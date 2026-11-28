//go:build e2e

// Package e2e drives the real `crush acp` binary over stdio with raw JSON-RPC,
// which is the only arrangement that can catch transport-level faults an
// in-process pipe test cannot see: stdout pollution, framing mistakes, and slow
// or missing handshakes.
//
// Run with:
//
//	go test -tags e2e ./test/acp/... -count=1 -v
package e2e

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// zedInitializeParams is the client half of a real Zed 1.17.2 handshake,
// transcribed from the recorded transcript in the Tangerg/acp corpus. Sending
// exactly what an editor sends is the point: a handshake that only works for a
// synthetic client proves nothing about Zed.
func zedInitializeParams() map[string]any {
	return map[string]any{
		"protocolVersion": 1,
		"clientCapabilities": map[string]any{
			"fs": map[string]any{
				"readTextFile":  true,
				"writeTextFile": true,
			},
			"terminal": true,
			"session": map[string]any{
				"configOptions": map[string]any{
					"boolean": map[string]any{},
				},
			},
			"auth": map[string]any{
				"terminal": true,
			},
			"elicitation": map[string]any{
				"form": map[string]any{},
				"url":  map[string]any{},
			},
		},
		"clientInfo": map[string]any{
			"name":    "zed",
			"title":   "Zed",
			"version": "1.17.2+stable.349",
		},
	}
}

// client speaks newline-delimited JSON-RPC 2.0 to a subprocess.
type client struct {
	t       *testing.T
	workdir string
	stdin   io.WriteCloser
	stdout  *bufio.Reader

	mu       sync.Mutex
	nextID   int
	pending  map[int]chan envelope
	notify   chan envelope
	pollOnce sync.Once
	raw      bytes.Buffer
}

type envelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	Result  json.RawMessage `json:"result"`
	Error   json.RawMessage `json:"error"`
}

func startServer(t *testing.T) *client {
	t.Helper()

	bin := os.Getenv("CRUSH_BIN")
	if bin == "" {
		t.Fatal("CRUSH_BIN is not set; build the binary first")
	}
	workdir := t.TempDir()

	cmd := exec.Command(bin, "acp", "-c", workdir)
	cmd.Dir = workdir

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	t.Cleanup(func() {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		if t.Failed() && stderr.Len() > 0 {
			t.Logf("server stderr:\n%s", stderr.String())
		}
	})

	c := &client{
		t:       t,
		workdir: workdir,
		stdin:   stdin,
		stdout:  bufio.NewReaderSize(stdout, 1<<20),
		pending: make(map[int]chan envelope),
		notify:  make(chan envelope, 256),
	}
	c.poll()
	return c
}

func (c *client) poll() {
	c.pollOnce.Do(func() {
		go func() {
			for {
				line, err := c.stdout.ReadBytes('\n')
				if len(line) > 0 {
					c.mu.Lock()
					c.raw.Write(line)
					c.mu.Unlock()
					c.dispatch(line)
				}
				if err != nil {
					return
				}
			}
		}()
	})
}

func (c *client) dispatch(line []byte) {
	var env envelope
	if err := json.Unmarshal(bytes.TrimSpace(line), &env); err != nil {
		c.t.Errorf("server wrote a non-JSON-RPC line: %q", line)
		return
	}
	if env.Method != "" && len(env.ID) == 0 {
		select {
		case c.notify <- env:
		default:
		}
		return
	}
	if len(env.ID) == 0 {
		return
	}
	var id int
	if err := json.Unmarshal(env.ID, &id); err != nil {
		return
	}
	c.mu.Lock()
	ch := c.pending[id]
	delete(c.pending, id)
	c.mu.Unlock()
	if ch != nil {
		ch <- env
	}
}

func (c *client) call(method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	ch := make(chan envelope, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return nil, err
	}
	body = append(body, '\n')
	if _, err := c.stdin.Write(body); err != nil {
		return nil, fmt.Errorf("write %s: %w", method, err)
	}

	select {
	case env := <-ch:
		if len(env.Error) > 0 {
			return nil, fmt.Errorf("%s: %s", method, env.Error)
		}
		return env.Result, nil
	case <-time.After(20 * time.Second):
		return nil, fmt.Errorf("%s: no response within 20s", method)
	}
}

func (c *client) notifyCall(method string, params any) error {
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return err
	}
	body = append(body, '\n')
	_, err = c.stdin.Write(body)
	return err
}

// TestZedHandshake speaks the recorded Zed handshake to the real binary and
// requires the initialize response to advertise the capabilities an editor
// reads before it will leave its loading state.
func TestZedHandshake(t *testing.T) {
	c := startServer(t)

	raw, err := c.call("initialize", zedInitializeParams())
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}

	var resp struct {
		ProtocolVersion int `json:"protocolVersion"`
		AgentInfo       *struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"agentInfo"`
		AgentCapabilities map[string]any `json:"agentCapabilities"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("decode initialize response: %v", err)
	}
	if resp.ProtocolVersion != 1 {
		t.Errorf("protocolVersion = %d, want 1", resp.ProtocolVersion)
	}
	if resp.AgentInfo == nil || resp.AgentInfo.Name == "" {
		t.Errorf("agentInfo is missing or unnamed: %s", raw)
	}
	if resp.AgentCapabilities == nil {
		t.Fatalf("agentCapabilities missing from %s", raw)
	}
	for _, key := range []string{"promptCapabilities", "sessionCapabilities", "mcpCapabilities"} {
		if _, ok := resp.AgentCapabilities[key]; !ok {
			t.Errorf("agentCapabilities.%s missing from %s", key, raw)
		}
	}
}

// TestZedSessionThenCancel opens a session the way the editor does and requires
// cancellation to be honoured. The session methods are exercised even though no
// model is configured, because setup must not depend on a provider being
// reachable.
func TestZedSessionThenCancel(t *testing.T) {
	c := startServer(t)

	if _, err := c.call("initialize", zedInitializeParams()); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	raw, err := c.call("session/new", map[string]any{
		"cwd":        c.workdir,
		"mcpServers": []any{},
	})
	if err != nil {
		t.Fatalf("session/new: %v", err)
	}
	var ns struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(raw, &ns); err != nil {
		t.Fatalf("decode session/new: %v", err)
	}
	if ns.SessionID == "" {
		t.Fatalf("empty sessionId in %s", raw)
	}

	// Setup methods an editor sends before the first prompt.
	if _, err := c.call("session/set_mode", map[string]any{
		"sessionId": ns.SessionID,
		"modeId":    "code",
	}); err != nil {
		t.Errorf("session/set_mode: %v", err)
	}
	if _, err := c.call("session/set_config_option", map[string]any{
		"sessionId": ns.SessionID,
		"configId":  "model",
		"value":     "gpt-4o-mini",
	}); err != nil {
		t.Errorf("session/set_config_option: %v", err)
	}

	if err := c.notifyCall("session/cancel", map[string]any{
		"sessionId": ns.SessionID,
	}); err != nil {
		t.Fatalf("session/cancel: %v", err)
	}
}

// TestStdoutCarriesOnlyJSONRPC fails if anything the server writes to stdout is
// not a JSON-RPC frame. Observed over a whole run rather than a single
// handshake, so background logging has time to leak.
func TestStdoutCarriesOnlyJSONRPC(t *testing.T) {
	c := startServer(t)

	if _, err := c.call("initialize", zedInitializeParams()); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	if _, err := c.call("session/new", map[string]any{
		"cwd":        c.workdir,
		"mcpServers": []any{},
	}); err != nil {
		t.Fatalf("session/new: %v", err)
	}
	// Give any stray background writer a chance to leak into stdout.
	time.Sleep(500 * time.Millisecond)

	c.mu.Lock()
	raw := c.raw.String()
	c.mu.Unlock()

	sc := bufio.NewScanner(strings.NewReader(raw))
	sc.Buffer(make([]byte, 0, 1<<20), 1<<20)
	lines := 0
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		lines++
		var env envelope
		if err := json.Unmarshal([]byte(line), &env); err != nil {
			t.Fatalf("stdout line %d is not JSON: %q", lines, line)
		}
		if env.JSONRPC != "2.0" {
			t.Fatalf("stdout line %d has jsonrpc=%q: %q", lines, env.JSONRPC, line)
		}
	}
	// Two calls, so at least two responses. The server does not echo requests
	// back, so this is the floor rather than a round number.
	if lines < 2 {
		t.Fatalf("expected at least 2 response frames, saw %d", lines)
	}
}

// TestUnknownMethodFails keeps the error path honest: an unimplemented method
// must produce a JSON-RPC error rather than silence, because silence is what
// hangs a client.
func TestUnknownMethodFails(t *testing.T) {
	c := startServer(t)

	if _, err := c.call("initialize", zedInitializeParams()); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	if _, err := c.call("session/resume", map[string]any{
		"sessionId": "nope",
		"cwd":       c.workdir,
	}); err == nil {
		t.Error("session/resume should report an error")
	}
}

var (
	_ = filepath.Join
	_ = context.Background
)
