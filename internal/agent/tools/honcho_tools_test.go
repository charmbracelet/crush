package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/honcho"
	"github.com/stretchr/testify/require"
)

// honchoFake is a stand-in Honcho deployment. It records what each
// tool sent and replies with whatever the test staged, so a test can
// assert on the request as well as on the rendered output.
type honchoFake struct {
	mu sync.Mutex
	// bodies maps a route name to the last request body seen on it.
	bodies map[string]string

	// searchItems, chatContent, and queue are the staged replies.
	searchItems []map[string]any
	chatContent string
	queue       map[string]any
}

func newHonchoFake() *honchoFake {
	return &honchoFake{
		bodies: map[string]string{},
		queue:  map[string]any{"empty": true},
	}
}

func (f *honchoFake) record(route string, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bodies[route] = string(body)
}

func (f *honchoFake) body(route string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.bodies[route]
}

func (f *honchoFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	path := r.URL.Path
	switch {
	case strings.HasSuffix(path, "/search"):
		f.record("search", r)
		f.mu.Lock()
		items := f.searchItems
		f.mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"items": items})

	case strings.HasSuffix(path, "/chat"):
		f.record("chat", r)
		f.mu.Lock()
		content := f.chatContent
		f.mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"content": content})

	case strings.HasSuffix(path, "/conclusions"):
		f.record("conclusions", r)
		_ = json.NewEncoder(w).Encode(map[string]any{})

	case strings.HasSuffix(path, "/queue/status"):
		f.record("queue", r)
		f.mu.Lock()
		queue := f.queue
		f.mu.Unlock()
		_ = json.NewEncoder(w).Encode(queue)

	default:
		// Anything else means a tool called a route it should not
		// have; failing loudly beats a silent empty result.
		w.WriteHeader(http.StatusNotFound)
	}
}

// newHonchoTestService starts a fake deployment and returns a real
// Service pointed at it. The base URL is loopback, which makes the
// config local and so exempt from the API key requirement.
func newHonchoTestService(t *testing.T) (*honcho.Service, *honchoFake) {
	t.Helper()

	fake := newHonchoFake()
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)

	svc := honcho.NewService(honcho.Config{
		Enabled:         true,
		BaseURL:         srv.URL,
		Workspace:       "testws",
		PeerName:        "tester",
		AgentPeer:       "crush",
		RecallMode:      honcho.RecallHybrid,
		ObservationMode: honcho.ObserveUnified,
		SessionStrategy: honcho.StrategyPerDirectory,
	}, t.TempDir(), "", "test")
	require.NotNil(t, svc, "service should be constructible against a local deployment")
	t.Cleanup(func() { svc.Close(context.Background()) })

	return svc, fake
}

// runTool invokes a tool with a JSON input document.
func runTool(t *testing.T, tool fantasy.AgentTool, input string) fantasy.ToolResponse {
	t.Helper()
	resp, err := tool.Run(t.Context(), fantasy.ToolCall{
		ID:    "call-1",
		Name:  tool.Info().Name,
		Input: input,
	})
	require.NoError(t, err, "a tool must report trouble in its response, not as a Go error")
	return resp
}

func TestHonchoSearchTool(t *testing.T) {
	t.Parallel()

	t.Run("returns matching messages", func(t *testing.T) {
		t.Parallel()
		svc, fake := newHonchoTestService(t)
		fake.searchItems = []map[string]any{
			{"peer_id": "tester", "content": "we chose sqlite because it ships as one binary"},
			{"peer_id": "crush", "content": "noted, sqlite it is"},
		}

		resp := runTool(t, NewHonchoSearchTool(svc), `{"query":"database choice","limit":2}`)
		require.False(t, resp.IsError)
		require.Contains(t, resp.Content, "Found 2 past message(s)")
		require.Contains(t, resp.Content, "[tester]")
		require.Contains(t, resp.Content, "ships as one binary")
		require.Contains(t, resp.Content, "[crush]")

		var sent honcho.SearchQuery
		require.NoError(t, json.Unmarshal([]byte(fake.body("search")), &sent))
		require.Equal(t, "database choice", sent.Query)
		require.Equal(t, 2, sent.Limit)
	})

	t.Run("defaults and caps the limit", func(t *testing.T) {
		t.Parallel()
		svc, fake := newHonchoTestService(t)

		runTool(t, NewHonchoSearchTool(svc), `{"query":"anything"}`)
		var sent honcho.SearchQuery
		require.NoError(t, json.Unmarshal([]byte(fake.body("search")), &sent))
		require.Equal(t, honchoSearchDefaultLimit, sent.Limit)

		runTool(t, NewHonchoSearchTool(svc), `{"query":"anything","limit":5000}`)
		require.NoError(t, json.Unmarshal([]byte(fake.body("search")), &sent))
		require.Equal(t, honchoSearchMaxLimit, sent.Limit)
	})

	t.Run("no matches is not an error", func(t *testing.T) {
		t.Parallel()
		svc, _ := newHonchoTestService(t)

		resp := runTool(t, NewHonchoSearchTool(svc), `{"query":"never discussed"}`)
		require.False(t, resp.IsError, "an empty result set is a normal outcome")
		require.Contains(t, resp.Content, "No past messages matched")
		require.Contains(t, resp.Content, "never discussed")
	})

	t.Run("rejects an empty query", func(t *testing.T) {
		t.Parallel()
		svc, fake := newHonchoTestService(t)

		resp := runTool(t, NewHonchoSearchTool(svc), `{"query":"   "}`)
		require.True(t, resp.IsError)
		require.Contains(t, resp.Content, "query is required")
		require.Empty(t, fake.body("search"), "a rejected call must not reach the network")
	})

	t.Run("nil service explains itself", func(t *testing.T) {
		t.Parallel()
		resp := runTool(t, NewHonchoSearchTool(nil), `{"query":"anything"}`)
		require.True(t, resp.IsError)
		require.Contains(t, resp.Content, "Memory is not configured")
	})
}

func TestHonchoChatTool(t *testing.T) {
	t.Parallel()

	t.Run("returns a labelled answer", func(t *testing.T) {
		t.Parallel()
		svc, fake := newHonchoTestService(t)
		fake.chatContent = "They prefer table-driven tests."

		resp := runTool(t, NewHonchoChatTool(svc), `{"query":"how does the user like tests?"}`)
		require.False(t, resp.IsError)
		require.Contains(t, resp.Content, "They prefer table-driven tests.")
		require.Contains(t, resp.Content, honcho.MemoryInstruction,
			"derived memory must be labelled as data, not instruction")

		var sent honcho.ChatQuery
		require.NoError(t, json.Unmarshal([]byte(fake.body("chat")), &sent))
		require.Equal(t, "how does the user like tests?", sent.Query)
		require.Equal(t, honcho.ReasoningLow, sent.ReasoningLevel)
		require.Equal(t, svc.Identity().SessionKey, sent.SessionID)
		// Unified observation asks the user's own collection, so the
		// target is deliberately absent.
		require.Empty(t, sent.Target)
	})

	t.Run("empty answer is not an error", func(t *testing.T) {
		t.Parallel()
		svc, _ := newHonchoTestService(t)

		resp := runTool(t, NewHonchoChatTool(svc), `{"query":"what is their favourite colour?"}`)
		require.False(t, resp.IsError)
		require.Contains(t, resp.Content, "no answer")
	})

	t.Run("rejects an empty query", func(t *testing.T) {
		t.Parallel()
		svc, fake := newHonchoTestService(t)

		resp := runTool(t, NewHonchoChatTool(svc), `{"query":""}`)
		require.True(t, resp.IsError)
		require.Contains(t, resp.Content, "query is required")
		require.Empty(t, fake.body("chat"))
	})

	t.Run("nil service explains itself", func(t *testing.T) {
		t.Parallel()
		resp := runTool(t, NewHonchoChatTool(nil), `{"query":"anything"}`)
		require.True(t, resp.IsError)
		require.Contains(t, resp.Content, "Memory is not configured")
	})
}

func TestHonchoRememberTool(t *testing.T) {
	t.Parallel()

	t.Run("stores a conclusion against the right peers", func(t *testing.T) {
		t.Parallel()
		svc, fake := newHonchoTestService(t)

		resp := runTool(t, NewHonchoRememberTool(svc),
			`{"content":"Kieran wants gofmt, not gofumpt, on this repo."}`)
		require.False(t, resp.IsError)
		require.Contains(t, resp.Content, "Saved to memory")
		require.Contains(t, resp.Content, "gofmt, not gofumpt")

		var sent struct {
			Conclusions []honcho.Conclusion `json:"conclusions"`
		}
		require.NoError(t, json.Unmarshal([]byte(fake.body("conclusions")), &sent))
		require.Len(t, sent.Conclusions, 1)

		id := svc.Identity()
		got := sent.Conclusions[0]
		require.Equal(t, "Kieran wants gofmt, not gofumpt, on this repo.", got.Content)
		require.Equal(t, id.ObserverPeer(svc.Config().ObservationMode), got.ObserverID)
		require.Equal(t, id.UserPeer, got.ObservedID)
		require.Equal(t, id.SessionKey, got.SessionID)
	})

	t.Run("clamps overlong content", func(t *testing.T) {
		t.Parallel()
		svc, fake := newHonchoTestService(t)

		long := strings.Repeat("a", honchoRememberClamp*2)
		resp := runTool(t, NewHonchoRememberTool(svc),
			`{"content":"`+long+`"}`)
		require.False(t, resp.IsError)
		require.Contains(t, resp.Content, "shortened")

		var sent struct {
			Conclusions []honcho.Conclusion `json:"conclusions"`
		}
		require.NoError(t, json.Unmarshal([]byte(fake.body("conclusions")), &sent))
		require.Len(t, sent.Conclusions, 1)
		require.LessOrEqual(t, len(sent.Conclusions[0].Content), honchoRememberClamp+3)
	})

	t.Run("rejects empty content", func(t *testing.T) {
		t.Parallel()
		svc, fake := newHonchoTestService(t)

		resp := runTool(t, NewHonchoRememberTool(svc), `{"content":"  "}`)
		require.True(t, resp.IsError)
		require.Contains(t, resp.Content, "content is required")
		require.Empty(t, fake.body("conclusions"))
	})

	t.Run("nil service explains itself", func(t *testing.T) {
		t.Parallel()
		resp := runTool(t, NewHonchoRememberTool(nil), `{"content":"something"}`)
		require.True(t, resp.IsError)
		require.Contains(t, resp.Content, "Memory is not configured")
	})
}

func TestHonchoStatusTool(t *testing.T) {
	t.Parallel()

	t.Run("reports configuration without the api key", func(t *testing.T) {
		t.Parallel()
		svc, _ := newHonchoTestService(t)

		resp := runTool(t, NewHonchoStatusTool(svc), `{}`)
		require.False(t, resp.IsError)

		id := svc.Identity()
		require.Contains(t, resp.Content, id.Workspace)
		require.Contains(t, resp.Content, id.SessionKey)
		require.Contains(t, resp.Content, id.UserPeer)
		require.Contains(t, resp.Content, id.AgentPeer)
		require.Contains(t, resp.Content, string(honcho.ObserveUnified))
		require.Contains(t, resp.Content, string(honcho.StrategyPerDirectory))
		require.Contains(t, resp.Content, svc.Client().BaseURL())
		require.NotContains(t, strings.ToLower(resp.Content), "api key")
		require.Contains(t, resp.Content, "idle")
	})

	t.Run("reports a reasoning backlog", func(t *testing.T) {
		t.Parallel()
		svc, fake := newHonchoTestService(t)
		fake.queue = map[string]any{
			"total_work_units":       5,
			"completed_work_units":   2,
			"in_progress_work_units": 1,
			"pending_work_units":     2,
		}

		resp := runTool(t, NewHonchoStatusTool(svc), `{}`)
		require.False(t, resp.IsError)
		require.Contains(t, resp.Content, "2 pending")
		require.Contains(t, resp.Content, "1 in progress")
		require.Contains(t, resp.Content, "have not yet influenced")
	})

	t.Run("nil service explains itself", func(t *testing.T) {
		t.Parallel()
		resp := runTool(t, NewHonchoStatusTool(nil), `{}`)
		require.True(t, resp.IsError)
		require.Contains(t, resp.Content, "Memory is not configured")
	})
}
