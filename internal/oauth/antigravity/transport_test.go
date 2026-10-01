package antigravity

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/oauth"
)

func TestParseGeminiPath(t *testing.T) {
	tests := []struct {
		path      string
		wantModel string
		wantVerb  string
		wantOK    bool
	}{
		{"/v1beta/models/gemini-2.5-pro:streamGenerateContent", "gemini-2.5-pro", verbStream, true},
		{"/v1beta/models/cld-sonnet-4-6:generateContent", "cld-sonnet-4-6", verbUnary, true},
		{"/v1beta/models", "", "", false},
		{"/v1beta/models/gemini-2.5-pro:countTokens", "", "", false},
		{"/openai/chat/completions", "", "", false},
	}
	for _, tt := range tests {
		model, verb, ok := parseGeminiPath(tt.path)
		if ok != tt.wantOK || model != tt.wantModel || verb != tt.wantVerb {
			t.Errorf("parseGeminiPath(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tt.path, model, verb, ok, tt.wantModel, tt.wantVerb, tt.wantOK)
		}
	}
}

func TestModelIDMangling(t *testing.T) {
	// The Gemini provider intercepts these ids before the request is made,
	// so they must not survive into crush's model list.
	for _, id := range []string{"claude-sonnet-4-6", "anthropic-claude-x", "cld-claude"} {
		safe := SafeModelID(id)
		if strings.Contains(safe, "claude") || strings.Contains(safe, "anthropic") {
			t.Errorf("SafeModelID(%q) = %q, still contains an intercepted token", id, safe)
		}
		if got := RealModelID(safe); !strings.Contains(got, "claude") && !strings.Contains(got, "anthropic") {
			t.Errorf("RealModelID(%q) = %q, mapping is not reversible", safe, got)
		}
	}
	// Ids that need no mangling are untouched.
	if got := SafeModelID("gemini-3.8-flash-low"); got != "gemini-3.8-flash-low" {
		t.Errorf("SafeModelID changed an unaffected id: %q", got)
	}
}

func TestNormalizeFunctionCalls(t *testing.T) {
	// A zero-parameter tool call round-tripped through history arrives with
	// args omitted, which the Anthropic models on this gateway reject.
	req := map[string]any{
		"contents": []any{
			map[string]any{"role": "model", "parts": []any{
				map[string]any{"functionCall": map[string]any{"name": "ls", "id": "call_1"}},
			}},
			map[string]any{"role": "user", "parts": []any{
				map[string]any{"functionCall": map[string]any{"name": "x", "args": nil}},
			}},
		},
	}
	normalizeFunctionCalls(req)

	turns := req["contents"].([]any)
	for i, turn := range turns {
		part := turn.(map[string]any)["parts"].([]any)[0].(map[string]any)
		call := part["functionCall"].(map[string]any)
		args, ok := call["args"].(map[string]any)
		if !ok || args == nil {
			t.Errorf("turn %d: functionCall.args = %#v, want an empty object", i, call["args"])
		}
		if id, _ := call["id"].(string); id == "" {
			t.Errorf("turn %d: functionCall.id is empty", i)
		}
	}
}

func newTestTransport(t *testing.T, handler http.HandlerFunc) (*Transport, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &Transport{
		Token:    &oauth.Token{AccessToken: "test-access-token"},
		Endpoint: server.URL + "/v1internal",
	}, server
}

// clientFor points the Gemini-shaped request at the stub. The transport
// rewrites host and path, so any base URL with a Gemini path works.
func geminiClient(t *testing.T, transport *Transport) *http.Client {
	t.Helper()
	return &http.Client{Transport: transport}
}

func TestRoundTripWrapsRequestAndUnwrapsReply(t *testing.T) {
	var gotPath, gotAuth, gotAPIKey, gotHost, gotBody string
	transport, _ := newTestTransport(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		gotPath = r.URL.Path + "?" + r.URL.RawQuery
		gotHost = r.Host
		gotAuth = r.Header.Get("Authorization")
		gotAPIKey = r.Header.Get("x-goog-api-key")
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sse(t, geminiChunk("hi", "", 7)))
		fmt.Fprint(w, sse(t, geminiChunk("", "STOP", 9)))
	})

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		"https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-pro:streamGenerateContent?alt=sse",
		strings.NewReader(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := geminiClient(t, transport).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)

	if gotPath != "/v1internal:streamGenerateContent?alt=sse" {
		t.Errorf("upstream path = %q, want the v1internal gateway", gotPath)
	}
	// The Host header must follow the rewritten URL. Google's front end
	// routes on it, and carrying the Gemini API host produces a bare 404.
	if !strings.Contains(gotHost, "127.0.0.1") {
		t.Errorf("Host header = %q, want the gateway host rather than the original API host", gotHost)
	}
	if gotAuth != "Bearer test-access-token" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if gotAPIKey != "" {
		t.Errorf("x-goog-api-key leaked to the subscription gateway: %q", gotAPIKey)
	}

	// Request must be wrapped in the envelope, with the Gemini body inside.
	var env struct {
		Model     string         `json:"model"`
		RequestID string         `json:"requestId"`
		UserAgent string         `json:"userAgent"`
		Request   map[string]any `json:"request"`
	}
	if err := json.Unmarshal([]byte(gotBody), &env); err != nil {
		t.Fatalf("envelope is not JSON: %v (%s)", err, gotBody)
	}
	if env.Model != "gemini-2.5-pro" {
		t.Errorf("envelope.model = %q", env.Model)
	}
	if env.RequestID == "" || env.UserAgent == "" {
		t.Errorf("envelope missing routing fields: %+v", env)
	}
	if _, ok := env.Request["contents"]; !ok {
		t.Errorf("envelope.request does not carry the Gemini body: %v", env.Request)
	}

	// Reply must be unwrapped, one event per line pair.
	events := splitEvents(t, string(out))
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2: %s", len(events), out)
	}
	for _, event := range events {
		var obj map[string]any
		if err := json.Unmarshal([]byte(event), &obj); err != nil {
			t.Fatalf("event is not JSON: %v (%s)", err, event)
		}
		if _, wrapped := obj["response"]; wrapped {
			t.Errorf("event still carries the wrapper: %s", event)
		}
	}
	first := mustJSON(t, events[0])
	if _, hasUsage := first["usageMetadata"]; hasUsage {
		t.Error("usage must be withheld from non-final chunks; crush sums it and would overcount")
	}
	last := mustJSON(t, events[1])
	usage, hasUsage := last["usageMetadata"]
	if !hasUsage {
		t.Fatal("the terminating chunk must carry usage")
	}
	if total := usage.(map[string]any)["totalTokenCount"]; total != float64(9) {
		t.Errorf("terminating usage totalTokenCount = %v, want 9 (the final cumulative value)", total)
	}
}

func TestRoundTripSynthesizesFinishReason(t *testing.T) {
	// Some turns end without a finishReason, and crush then throws the
	// completed answer away as a truncated stream.
	transport, _ := newTestTransport(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// No finishReason anywhere in the stream.
		fmt.Fprint(w, sse(t, geminiChunk("partial", "", 3)))
	})
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost,
		"https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-pro:streamGenerateContent?alt=sse",
		strings.NewReader(`{"contents":[]}`))
	resp, err := geminiClient(t, transport).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)

	events := splitEvents(t, string(out))
	if len(events) != 2 {
		t.Fatalf("got %d events, want a synthetic closer: %s", len(events), out)
	}
	final := mustJSON(t, events[len(events)-1])
	cands, _ := final["candidates"].([]any)
	if len(cands) == 0 {
		t.Fatalf("synthetic event has no candidates: %v", final)
	}
	if reason := cands[0].(map[string]any)["finishReason"]; reason != "STOP" {
		t.Errorf("synthetic finishReason = %v, want STOP", reason)
	}
}

func TestRoundTripUnaryAlwaysCarriesUsage(t *testing.T) {
	// crush's response mapper dereferences usageMetadata unguarded.
	transport, _ := newTestTransport(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body, _ := json.Marshal(map[string]any{"response": map[string]any{
			"candidates": []any{map[string]any{
				"content": map[string]any{
					"role":  "model",
					"parts": []any{map[string]any{"text": "x"}},
				},
			}},
		}})
		w.Write(body)
	})
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost,
		"https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-pro:generateContent",
		strings.NewReader(`{"contents":[]}`))
	resp, err := geminiClient(t, transport).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var obj map[string]any
	if err := json.Unmarshal(body, &obj); err != nil {
		t.Fatalf("bad JSON: %v (%s)", err, body)
	}
	if _, wrapped := obj["response"]; wrapped {
		t.Error("unary reply still carries the wrapper")
	}
	if _, ok := obj["usageMetadata"]; !ok {
		t.Error("unary reply has no usageMetadata; crush panics on it")
	}
}

func TestRoundTripLeavesOtherRequestsUnauthenticated(t *testing.T) {
	var sawAuth, sawKey bool
	transport, server := newTestTransport(t, func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization") != ""
		sawKey = r.Header.Get("x-goog-api-key") != ""
		fmt.Fprint(w, `{"models":[]}`)
	})
	// A path the gateway does not serve is passed through untouched, and
	// must not carry the subscription bearer with it.
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/v1beta/models", nil)
	// What the Gemini SDK puts on every request.
	req.Header.Set("x-goog-api-key", "some-api-key")
	client := &http.Client{Transport: &Transport{
		Token:    &oauth.Token{AccessToken: "secret-token"},
		Endpoint: transport.Endpoint,
	}}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if sawAuth {
		t.Error("the subscription bearer was attached to a non-generation request")
	}
	if sawKey {
		t.Error("an API key survived onto a request the gateway does not serve")
	}
}

// sse renders one upstream v1internal event: the backend wraps every reply in
// a "response" object and terminates each event with a blank line.
func sse(t *testing.T, response map[string]any) string {
	t.Helper()
	payload, err := json.Marshal(map[string]any{"response": response})
	if err != nil {
		t.Fatal(err)
	}
	return "data: " + string(payload) + "\n\n"
}

// geminiChunk is a GenerateContentResponse shaped the way the gateway emits it.
func geminiChunk(text, finishReason string, totalTokens int) map[string]any {
	candidate := map[string]any{
		"content": map[string]any{
			"role":  "model",
			"parts": []any{map[string]any{"text": text}},
		},
	}
	if finishReason != "" {
		candidate["finishReason"] = finishReason
	}
	return map[string]any{
		"candidates":    []any{candidate},
		"usageMetadata": map[string]any{"totalTokenCount": totalTokens},
	}
}

func splitEvents(t *testing.T, stream string) []string {
	t.Helper()
	var events []string
	for _, block := range strings.Split(stream, "\n\n") {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		if !strings.HasPrefix(block, "data:") {
			t.Errorf("event is not a data line, which the Gemini SDK rejects: %q", block)
			continue
		}
		events = append(events, strings.TrimSpace(strings.TrimPrefix(block, "data:")))
	}
	return events
}

func mustJSON(t *testing.T, raw string) map[string]any {
	t.Helper()
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		t.Fatalf("not JSON (%v): %s", err, raw)
	}
	return obj
}
