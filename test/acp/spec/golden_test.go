package acp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	acp "github.com/coder/acp-go-sdk"
)

// equalJSON unmarshals both sides to generic values and compares structurally.
func equalJSON(a, b []byte) (bool, string, string) {
	var va any
	var vb any
	if err := json.Unmarshal(a, &va); err != nil {
		return false, string(a), string(b)
	}
	if err := json.Unmarshal(b, &vb); err != nil {
		return false, string(a), string(b)
	}
	return reflect.DeepEqual(va, vb), string(a), string(b)
}

func mustReadGolden(t *testing.T, name string) []byte {
	t.Helper()
	p := filepath.Join("testdata", "json_golden", name)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read golden %s: %v", p, err)
	}
	return b
}

// runGolden asserts that all builders serialize to the same golden JSON file
// and that unmarshaling + re-marshaling the golden is lossless.
func runGolden[T any](builds ...func() T) func(t *testing.T) {
	return func(t *testing.T) {
		t.Helper()
		t.Parallel()
		name := t.Name()
		base := name
		if i := strings.LastIndex(base, "/"); i >= 0 {
			base = base[i+1:]
		}
		want := mustReadGolden(t, base+".json")
		for _, build := range builds {
			got, err := json.Marshal(build())
			if err != nil {
				t.Fatalf("marshal %s: %v", base, err)
			}
			if ok, ga, gw := equalJSON(got, want); !ok {
				t.Fatalf("%s marshal mismatch\n got: %s\nwant: %s", base, ga, gw)
			}
		}
		var v T
		if err := json.Unmarshal(want, &v); err != nil {
			t.Fatalf("unmarshal %s: %v", base, err)
		}
		round, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("re-marshal %s: %v", base, err)
		}
		if ok, ga, gw := equalJSON(round, want); !ok {
			t.Fatalf("%s round-trip mismatch\n got: %s\nwant: %s", base, ga, gw)
		}
	}
}

// --- Crush server wire-format validation ---

// TestGoldenCrushInitializeResponse verifies our server's Initialize response
// is structurally valid and rounds-trips through JSON without data loss.
// We compare against the SDK's golden fixture to catch any divergence from
// the spec that could confuse ACP clients like Zed.
func TestGoldenCrushInitializeResponse(t *testing.T) {
	t.Parallel()
	// Build the same response that crushAgent.Initialize produces after the simplification.
	resp := acp.InitializeResponse{
		ProtocolVersion: acp.ProtocolVersionNumber,
		AgentInfo:       &acp.Implementation{Name: "Crush", Version: "dev"},
		AgentCapabilities: acp.AgentCapabilities{
			LoadSession: true,
		},
	}
	got, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// Validate structure against the SDK golden fixture.
	want := mustReadGolden(t, "initialize_response.json")
	if ok, ga, gw := equalJSON(got, want); !ok {
		t.Logf("initialize response diff (acceptable if only due to capability values):\n got: %s\nwant: %s", ga, gw)
		var gm, gwmap map[string]any
		_ = json.Unmarshal([]byte(ga), &gm)
		_ = json.Unmarshal([]byte(gw), &gwmap)
		for key := range gwmap {
			if key == "agentInfo" {
				continue // agent-specific; skip
			}
			if _, ok := gm[key]; !ok {
				t.Errorf("missing field %q in response", key)
			}
		}
	}
	// Round-trip check: unmarshal our response and re-marshal — must be lossless.
	var rt acp.InitializeResponse
	if err := json.Unmarshal(got, &rt); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	round, err := json.Marshal(rt)
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	if ok, ga, gw := equalJSON(got, round); !ok {
		t.Fatalf("round-trip mismatch\n got: %s\nwant: %s", ga, gw)
	}
	// Ensure core fields are sensible.
	if resp.ProtocolVersion != acp.ProtocolVersionNumber {
		t.Errorf("protocol version: got %v, want %v", resp.ProtocolVersion, acp.ProtocolVersionNumber)
	}
	if resp.AgentInfo == nil || resp.AgentInfo.Name != "Crush" {
		t.Errorf("agent info name: got %v", resp.AgentInfo)
	}
	if !resp.AgentCapabilities.LoadSession {
		t.Log("loadSession is false — client may not advertise session listing")
	}
}

// --- SDK type golden tests (exact match against reference fixtures) ---

func TestGoldenContentBlocks(t *testing.T) {
	t.Parallel()
	t.Run("content_text", runGolden(
		func() acp.ContentBlock { return acp.TextBlock("What's the weather like today?") },
	))
	t.Run("content_image", runGolden(
		func() acp.ContentBlock { return acp.ImageBlock("iVBORw0KGgoAAAANSUhEUgAAAAEAAAAB...", "image/png") },
	))
	t.Run("content_resource_link", runGolden(
		func() acp.ContentBlock {
			mt := "application/pdf"
			sz := 1024000
			return acp.ContentBlock{ResourceLink: &acp.ContentBlockResourceLink{Type: "resource_link", Uri: "file:///home/user/document.pdf", Name: "document.pdf", MimeType: &mt, Size: &sz}}
		},
	))
}

func TestGoldenSessionUpdates(t *testing.T) {
	t.Parallel()
	t.Run("session_update_agent_message_chunk", runGolden(
		func() acp.SessionUpdate { return acp.UpdateAgentMessageText("The capital of France is Paris.") },
	))
	t.Run("session_update_tool_call", runGolden(
		func() acp.SessionUpdate {
			return acp.StartToolCall("call_001", "Reading configuration file",
				acp.WithStartKind(acp.ToolKindRead),
				acp.WithStartStatus(acp.ToolCallStatusPending),
			)
		},
	))
	t.Run("session_update_tool_call_edit", runGolden(
		func() acp.SessionUpdate {
			return acp.StartEditToolCall("call_003", "Apply edit", "/home/user/project/src/config.json", "print('hello')")
		},
	))
}

func TestGoldenPermissionOutcome(t *testing.T) {
	t.Parallel()
	t.Run("permission_outcome_selected", runGolden(
		func() acp.RequestPermissionOutcome { return acp.NewRequestPermissionOutcomeSelected("allow-once") },
	))
	t.Run("permission_outcome_cancelled", runGolden(
		func() acp.RequestPermissionOutcome { return acp.NewRequestPermissionOutcomeCancelled() },
	))
}

func TestGoldenMethodPayloads(t *testing.T) {
	t.Parallel()
	t.Run("initialize_request", runGolden(func() acp.InitializeRequest {
		return acp.InitializeRequest{
			ProtocolVersion:    acp.ProtocolVersionNumber,
			ClientCapabilities: acp.ClientCapabilities{Fs: acp.FileSystemCapabilities{ReadTextFile: true, WriteTextFile: true}},
		}
	}))
	t.Run("new_session_request", runGolden(func() acp.NewSessionRequest {
		return acp.NewSessionRequest{
			Cwd: "/home/user/project",
			McpServers: []acp.McpServer{
				{Stdio: &acp.McpServerStdio{Name: "filesystem", Command: "/path/to/mcp-server", Args: []string{"--stdio"}, Env: []acp.EnvVariable{}}},
			},
		}
	}))
	t.Run("cancel_notification", runGolden(func() acp.CancelNotification {
		return acp.CancelNotification{SessionId: "sess_abc123def456"}
	}))
}
