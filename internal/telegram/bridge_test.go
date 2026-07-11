package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/client"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

// crushFake is a minimal crush server implementing the routes the bridge
// touches, plus an injectable SSE event stream.
type crushFake struct {
	mu sync.Mutex

	wsID     string
	sessions []proto.Session
	agent    proto.AgentInfo
	busy     map[string]bool
	queued   map[string]int

	agentPosts   []proto.AgentMessage
	grantPosts   []proto.PermissionGrant
	grantResolve bool
	cancels      []string

	// SSE: each subscriber gets its own channel of pre-encoded data lines.
	subscribers []chan string
}

func newCrushFake(wsID string) *crushFake {
	return &crushFake{
		wsID: wsID,
		agent: proto.AgentInfo{
			IsReady: true,
		},
		busy:         make(map[string]bool),
		queued:       make(map[string]int),
		grantResolve: true,
		sessions: []proto.Session{
			{ID: "sess-active", Title: "active", UpdatedAt: 200, MessageCount: 3, Cost: 0.12},
			{ID: "sess-other", Title: "other", UpdatedAt: 100, MessageCount: 1, Cost: 0.01},
		},
	}
}

func (f *crushFake) start(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	prefix := "/v1/workspaces/" + f.wsID

	mux.HandleFunc("GET "+prefix+"/agent", func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		_ = json.NewEncoder(w).Encode(f.agent)
	})
	mux.HandleFunc("POST "+prefix+"/agent/init", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("POST "+prefix+"/agent/update", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET "+prefix+"/sessions", func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		_ = json.NewEncoder(w).Encode(f.sessions)
	})
	mux.HandleFunc("POST "+prefix+"/sessions", func(w http.ResponseWriter, r *http.Request) {
		var s proto.Session
		_ = json.NewDecoder(r.Body).Decode(&s)
		if s.ID == "" {
			s.ID = fmt.Sprintf("sess-%d", time.Now().UnixNano())
		}
		if s.Title == "" {
			s.Title = "telegram"
		}
		s.UpdatedAt = time.Now().Unix()
		f.mu.Lock()
		f.sessions = append(f.sessions, s)
		f.mu.Unlock()
		_ = json.NewEncoder(w).Encode(s)
	})
	mux.HandleFunc("GET "+prefix+"/sessions/{sid}", func(w http.ResponseWriter, r *http.Request) {
		sid := r.PathValue("sid")
		f.mu.Lock()
		defer f.mu.Unlock()
		for _, s := range f.sessions {
			if s.ID == sid {
				_ = json.NewEncoder(w).Encode(s)
				return
			}
		}
		http.Error(w, "not found", http.StatusNotFound)
	})
	mux.HandleFunc("GET "+prefix+"/agent/sessions/{sid}", func(w http.ResponseWriter, r *http.Request) {
		sid := r.PathValue("sid")
		f.mu.Lock()
		defer f.mu.Unlock()
		var sess proto.Session
		for _, s := range f.sessions {
			if s.ID == sid {
				sess = s
				break
			}
		}
		_ = json.NewEncoder(w).Encode(proto.AgentSession{
			Session: sess,
			IsBusy:  f.busy[sid],
		})
	})
	mux.HandleFunc("GET "+prefix+"/agent/sessions/{sid}/prompts/queued", func(w http.ResponseWriter, r *http.Request) {
		sid := r.PathValue("sid")
		f.mu.Lock()
		n := f.queued[sid]
		f.mu.Unlock()
		_ = json.NewEncoder(w).Encode(n)
	})
	mux.HandleFunc("POST "+prefix+"/agent", func(w http.ResponseWriter, r *http.Request) {
		var msg proto.AgentMessage
		_ = json.NewDecoder(r.Body).Decode(&msg)
		f.mu.Lock()
		f.agentPosts = append(f.agentPosts, msg)
		f.mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
	})
	mux.HandleFunc("POST "+prefix+"/permissions/grant", func(w http.ResponseWriter, r *http.Request) {
		var g proto.PermissionGrant
		_ = json.NewDecoder(r.Body).Decode(&g)
		f.mu.Lock()
		f.grantPosts = append(f.grantPosts, g)
		resolved := f.grantResolve
		f.mu.Unlock()
		_ = json.NewEncoder(w).Encode(proto.PermissionGrantResponse{Resolved: resolved})
	})
	mux.HandleFunc("POST "+prefix+"/agent/sessions/{sid}/cancel", func(w http.ResponseWriter, r *http.Request) {
		sid := r.PathValue("sid")
		f.mu.Lock()
		f.cancels = append(f.cancels, sid)
		f.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET "+prefix+"/events", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "no flush", 500)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		ch := make(chan string, 32)
		f.mu.Lock()
		f.subscribers = append(f.subscribers, ch)
		f.mu.Unlock()

		for {
			select {
			case <-r.Context().Done():
				return
			case line, ok := <-ch:
				if !ok {
					return
				}
				_, _ = fmt.Fprintf(w, "data: %s\n\n", line)
				flusher.Flush()
			}
		}
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func (f *crushFake) pushEvent(t *testing.T, payloadType string, inner any) {
	t.Helper()
	raw, err := json.Marshal(inner)
	require.NoError(t, err)
	env, err := json.Marshal(pubsub.Payload{
		Type:    payloadType,
		Payload: raw,
	})
	require.NoError(t, err)
	f.mu.Lock()
	subs := append([]chan string(nil), f.subscribers...)
	f.mu.Unlock()
	// Wait briefly for a subscriber if none yet.
	deadline := time.Now().Add(2 * time.Second)
	for len(subs) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		f.mu.Lock()
		subs = append([]chan string(nil), f.subscribers...)
		f.mu.Unlock()
	}
	require.NotEmpty(t, subs, "no SSE subscribers")
	for _, ch := range subs {
		select {
		case ch <- string(env):
		case <-time.After(time.Second):
			t.Fatal("SSE subscriber blocked")
		}
	}
}

// tgFake is a scripted Telegram Bot API server.
type tgFake struct {
	mu sync.Mutex

	// updates is a queue consumed by getUpdates.
	updates []Update
	// sent records outbound sendMessage calls.
	sent []sentMsg
	// edits records editMessageText calls.
	edits []map[string]any
	// callbacks answered.
	answered []string
	// next message id counter.
	nextMsgID int64
	// cond signals new updates.
	cond *sync.Cond
}

type sentMsg struct {
	id   int64
	body map[string]any
}

func newTGFake() *tgFake {
	f := &tgFake{nextMsgID: 1}
	f.cond = sync.NewCond(&f.mu)
	return f
}

func (f *tgFake) start(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		method := parts[len(parts)-1]

		switch method {
		case "getMe":
			writeTGOK(w, User{ID: 1, Username: "testbot"})
		case "getUpdates":
			f.mu.Lock()
			// Long-poll-ish: wait up to a short time for updates.
			if len(f.updates) == 0 {
				done := make(chan struct{})
				go func() {
					f.mu.Lock()
					f.cond.Wait()
					f.mu.Unlock()
					close(done)
				}()
				f.mu.Unlock()
				select {
				case <-done:
				case <-r.Context().Done():
					writeTGOK(w, []Update{})
					return
				case <-time.After(50 * time.Millisecond):
					// wake the waiter if still blocked
					f.cond.Broadcast()
					<-done
				}
				f.mu.Lock()
			}
			out := f.updates
			f.updates = nil
			f.mu.Unlock()
			if out == nil {
				out = []Update{}
			}
			writeTGOK(w, out)
		case "sendMessage":
			f.mu.Lock()
			f.nextMsgID++
			id := f.nextMsgID
			f.sent = append(f.sent, sentMsg{id: id, body: body})
			f.mu.Unlock()
			writeTGOK(w, Message{MessageID: id, Text: fmt.Sprint(body["text"])})
		case "editMessageText":
			f.mu.Lock()
			f.edits = append(f.edits, body)
			f.mu.Unlock()
			writeTGOK(w, true)
		case "answerCallbackQuery":
			f.mu.Lock()
			f.answered = append(f.answered, fmt.Sprint(body["callback_query_id"]))
			f.mu.Unlock()
			writeTGOK(w, true)
		case "sendChatAction":
			writeTGOK(w, true)
		default:
			writeTGOK(w, map[string]any{})
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func (f *tgFake) pushUpdate(u Update) {
	f.mu.Lock()
	f.updates = append(f.updates, u)
	f.cond.Broadcast()
	f.mu.Unlock()
}

func (f *tgFake) sentTexts() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, s := range f.sent {
		out = append(out, fmt.Sprint(s.body["text"]))
	}
	return out
}

func (f *tgFake) findSentID(substr string) (int64, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.sent {
		if strings.Contains(fmt.Sprint(s.body["text"]), substr) {
			return s.id, true
		}
	}
	return 0, false
}

func (f *tgFake) waitSent(t *testing.T, min int, timeout time.Duration) []string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		texts := f.sentTexts()
		if len(texts) >= min {
			return texts
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d sent messages; got %v", min, f.sentTexts())
	return nil
}

func writeTGOK(w http.ResponseWriter, result any) {
	raw, _ := json.Marshal(result)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":     true,
		"result": json.RawMessage(raw),
	})
}

func startBridge(t *testing.T, chatID int64) (*crushFake, *tgFake, context.CancelFunc) {
	t.Helper()
	const wsID = "ws-test"
	cf := newCrushFake(wsID)
	csrv := cf.start(t)
	host := strings.TrimPrefix(csrv.URL, "http://")
	c, err := client.NewClient(t.TempDir(), "tcp", host)
	require.NoError(t, err)

	tg := newTGFake()
	tsrv := tg.start(t)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go func() {
		_ = Run(ctx, Options{
			Client: c,
			Workspace: proto.Workspace{
				ID:   wsID,
				Path: "/tmp/proj",
			},
			Token:   "test-token",
			ChatID:  chatID,
			BaseURL: tsrv.URL,
		})
	}()

	// Wait for startup message.
	tg.waitSent(t, 1, 3*time.Second)
	return cf, tg, cancel
}

func TestBridgeRoundTrip(t *testing.T) {
	t.Parallel()
	const chatID int64 = 42
	cf, tg, cancel := startBridge(t, chatID)
	defer cancel()

	tg.pushUpdate(Update{
		UpdateID: 1,
		Message: &Message{
			MessageID: 100,
			Chat:      Chat{ID: chatID},
			Text:      "hello agent",
		},
	})

	// Wait for agent POST.
	deadline := time.Now().Add(3 * time.Second)
	var posted proto.AgentMessage
	for time.Now().Before(deadline) {
		cf.mu.Lock()
		if len(cf.agentPosts) > 0 {
			posted = cf.agentPosts[0]
			cf.mu.Unlock()
			break
		}
		cf.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	require.NotEmpty(t, posted.RunID)
	require.Equal(t, "hello agent", posted.Prompt)
	require.Equal(t, "sess-active", posted.SessionID)

	// Inject RunComplete.
	cf.pushEvent(t, pubsub.PayloadTypeRunComplete, pubsub.Event[proto.RunComplete]{
		Type: pubsub.CreatedEvent,
		Payload: proto.RunComplete{
			SessionID: "sess-active",
			RunID:     posted.RunID,
			MessageID: "m1",
			Text:      "world",
		},
	})

	texts := tg.waitSent(t, 2, 3*time.Second)
	found := false
	for _, tx := range texts {
		if strings.Contains(tx, "world") {
			found = true
		}
	}
	require.True(t, found, "expected final text in %v", texts)
}

func TestBridgeUnauthorizedChat(t *testing.T) {
	t.Parallel()
	const chatID int64 = 42
	cf, tg, cancel := startBridge(t, chatID)
	defer cancel()

	before := len(cf.agentPosts)
	tg.pushUpdate(Update{
		UpdateID: 1,
		Message: &Message{
			MessageID: 100,
			Chat:      Chat{ID: 999},
			Text:      "intruder",
		},
	})
	time.Sleep(200 * time.Millisecond)
	cf.mu.Lock()
	after := len(cf.agentPosts)
	cf.mu.Unlock()
	require.Equal(t, before, after)
}

func TestBridgePermissionFlow(t *testing.T) {
	t.Parallel()
	const chatID int64 = 42
	cf, tg, cancel := startBridge(t, chatID)
	defer cancel()

	req := proto.PermissionRequest{
		ID:          "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		SessionID:   "sess-active",
		ToolCallID:  "tc1",
		ToolName:    "bash",
		Description: "run cmd",
		Params: proto.BashPermissionsParams{
			Command: "echo hi",
		},
	}
	cf.pushEvent(t, pubsub.PayloadTypePermissionRequest, pubsub.Event[proto.PermissionRequest]{
		Type:    pubsub.CreatedEvent,
		Payload: req,
	})

	// Wait for keyboard message.
	deadline := time.Now().Add(3 * time.Second)
	var hasKB bool
	for time.Now().Before(deadline) {
		tg.mu.Lock()
		for _, s := range tg.sent {
			if s.body["reply_markup"] != nil {
				hasKB = true
				require.Equal(t, "HTML", s.body["parse_mode"])
			}
		}
		tg.mu.Unlock()
		if hasKB {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.True(t, hasKB, "expected keyboard message")

	// Inject allow callback.
	tg.pushUpdate(Update{
		UpdateID: 2,
		CallbackQuery: &CallbackQuery{
			ID:   "cb1",
			From: User{ID: 1},
			Message: &Message{
				MessageID: 2, // approximate; bridge looks up by perm ID
				Chat:      Chat{ID: chatID},
			},
			Data: "perm:" + req.ID + ":allow",
		},
	})

	deadline = time.Now().Add(3 * time.Second)
	var grant proto.PermissionGrant
	for time.Now().Before(deadline) {
		cf.mu.Lock()
		if len(cf.grantPosts) > 0 {
			grant = cf.grantPosts[0]
			cf.mu.Unlock()
			break
		}
		cf.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	require.Equal(t, proto.PermissionAllow, grant.Action)
	require.Equal(t, req.ID, grant.Permission.ID)

	// Edit should have happened.
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		tg.mu.Lock()
		n := len(tg.edits)
		tg.mu.Unlock()
		if n > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expected editMessageText after grant")
}

func TestBridgePermissionResolvedElsewhere(t *testing.T) {
	t.Parallel()
	const chatID int64 = 42
	cf, tg, cancel := startBridge(t, chatID)
	defer cancel()

	cf.mu.Lock()
	cf.grantResolve = false
	cf.mu.Unlock()

	req := proto.PermissionRequest{
		ID:         "bbbbbbbb-bbbb-cccc-dddd-eeeeeeeeeeee",
		SessionID:  "sess-active",
		ToolCallID: "tc2",
		ToolName:   "view",
		Params:     proto.ViewPermissionsParams{FilePath: "a.go"},
	}
	cf.pushEvent(t, pubsub.PayloadTypePermissionRequest, pubsub.Event[proto.PermissionRequest]{
		Type:    pubsub.CreatedEvent,
		Payload: req,
	})

	// Wait for keyboard.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		tg.mu.Lock()
		has := false
		for _, s := range tg.sent {
			if s.body["reply_markup"] != nil {
				has = true
			}
		}
		tg.mu.Unlock()
		if has {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	tg.pushUpdate(Update{
		UpdateID: 3,
		CallbackQuery: &CallbackQuery{
			ID: "cb2",
			Message: &Message{
				MessageID: 3,
				Chat:      Chat{ID: chatID},
			},
			Data: "perm:" + req.ID + ":deny",
		},
	})

	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		tg.mu.Lock()
		for _, e := range tg.edits {
			if strings.Contains(fmt.Sprint(e["text"]), "already handled") {
				tg.mu.Unlock()
				return
			}
		}
		tg.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expected already-handled edit")
}

func TestBridgeRunError(t *testing.T) {
	t.Parallel()
	const chatID int64 = 42
	cf, tg, cancel := startBridge(t, chatID)
	defer cancel()

	tg.pushUpdate(Update{
		UpdateID: 1,
		Message: &Message{
			MessageID: 100,
			Chat:      Chat{ID: chatID},
			Text:      "do stuff",
		},
	})

	var runID string
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		cf.mu.Lock()
		if len(cf.agentPosts) > 0 {
			runID = cf.agentPosts[0].RunID
			cf.mu.Unlock()
			break
		}
		cf.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	require.NotEmpty(t, runID)

	cf.pushEvent(t, pubsub.PayloadTypeRunComplete, pubsub.Event[proto.RunComplete]{
		Type: pubsub.CreatedEvent,
		Payload: proto.RunComplete{
			SessionID: "sess-active",
			RunID:     runID,
			Error:     "boom",
		},
	})

	texts := tg.waitSent(t, 2, 3*time.Second)
	found := false
	for _, tx := range texts {
		if strings.Contains(tx, "❌") && strings.Contains(tx, "boom") {
			found = true
		}
	}
	require.True(t, found, "expected error message in %v", texts)
}

func TestBridgeCancel(t *testing.T) {
	t.Parallel()
	const chatID int64 = 42
	cf, tg, cancel := startBridge(t, chatID)
	defer cancel()

	tg.pushUpdate(Update{
		UpdateID: 1,
		Message: &Message{
			MessageID: 100,
			Chat:      Chat{ID: chatID},
			Text:      "/cancel",
		},
	})

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		cf.mu.Lock()
		n := len(cf.cancels)
		cf.mu.Unlock()
		if n > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cf.mu.Lock()
	require.Contains(t, cf.cancels, "sess-active")
	cf.mu.Unlock()

	// Simulate cancelled run complete for an owned run.
	// First send a prompt so we have a run tracked.
	tg.pushUpdate(Update{
		UpdateID: 2,
		Message: &Message{
			MessageID: 101,
			Chat:      Chat{ID: chatID},
			Text:      "long task",
		},
	})
	var runID string
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		cf.mu.Lock()
		if len(cf.agentPosts) > 0 {
			runID = cf.agentPosts[len(cf.agentPosts)-1].RunID
			cf.mu.Unlock()
			break
		}
		cf.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	require.NotEmpty(t, runID)

	cf.pushEvent(t, pubsub.PayloadTypeRunComplete, pubsub.Event[proto.RunComplete]{
		Type: pubsub.CreatedEvent,
		Payload: proto.RunComplete{
			SessionID: "sess-active",
			RunID:     runID,
			Cancelled: true,
		},
	})

	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		for _, tx := range tg.sentTexts() {
			if strings.Contains(tx, "🛑") && strings.Contains(tx, "cancelled") {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected cancel confirmation; got %v", tg.sentTexts())
}

func TestBridgeChunking(t *testing.T) {
	t.Parallel()
	const chatID int64 = 42
	cf, tg, cancel := startBridge(t, chatID)
	defer cancel()

	tg.pushUpdate(Update{
		UpdateID: 1,
		Message: &Message{
			MessageID: 100,
			Chat:      Chat{ID: chatID},
			Text:      "big",
		},
	})

	var runID string
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		cf.mu.Lock()
		if len(cf.agentPosts) > 0 {
			runID = cf.agentPosts[0].RunID
			cf.mu.Unlock()
			break
		}
		cf.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	require.NotEmpty(t, runID)

	long := strings.Repeat("x", 9000)
	cf.pushEvent(t, pubsub.PayloadTypeRunComplete, pubsub.Event[proto.RunComplete]{
		Type: pubsub.CreatedEvent,
		Payload: proto.RunComplete{
			SessionID: "sess-active",
			RunID:     runID,
			Text:      long,
		},
	})

	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		// startup + >=3 chunks
		if len(tg.sentTexts()) >= 4 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected >=3 chunk sends; got %d texts", len(tg.sentTexts()))
}

func TestBridgeReplyRouting(t *testing.T) {
	t.Parallel()
	const chatID int64 = 42
	cf, tg, cancel := startBridge(t, chatID)
	defer cancel()

	// Complete a run on the non-active session so the bridge records
	// an outbound message tagged with that session. Manually inject
	// RunComplete for sess-other by first tracking a run via a prompt
	// that replies... Actually: push a prompt while we temporarily
	// can't switch. Better approach: send a permission or run complete
	// that the bridge owns for sess-other by seeding runs.
	//
	// Simpler: use /use to switch, send prompt on other, complete it,
	// switch back, then reply to the completion message.

	// List sessions then /use 2 (other is index 2 when sorted by UpdatedAt desc:
	// active=200 first, other=100 second).
	tg.pushUpdate(Update{
		UpdateID: 1,
		Message: &Message{
			MessageID: 100,
			Chat:      Chat{ID: chatID},
			Text:      "/sessions",
		},
	})
	tg.waitSent(t, 2, 3*time.Second)

	tg.pushUpdate(Update{
		UpdateID: 2,
		Message: &Message{
			MessageID: 101,
			Chat:      Chat{ID: chatID},
			Text:      "/use 2",
		},
	})
	// Wait for active session switch ack.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		for _, tx := range tg.sentTexts() {
			if strings.Contains(tx, "active session") {
				goto switched
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("did not switch session")
switched:

	// Prompt on other session.
	tg.pushUpdate(Update{
		UpdateID: 3,
		Message: &Message{
			MessageID: 102,
			Chat:      Chat{ID: chatID},
			Text:      "on other",
		},
	})
	var runID string
	var otherPost proto.AgentMessage
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		cf.mu.Lock()
		for _, p := range cf.agentPosts {
			if p.Prompt == "on other" {
				otherPost = p
				runID = p.RunID
			}
		}
		cf.mu.Unlock()
		if runID != "" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.Equal(t, "sess-other", otherPost.SessionID)

	beforeSent := len(tg.sentTexts())
	cf.pushEvent(t, pubsub.PayloadTypeRunComplete, pubsub.Event[proto.RunComplete]{
		Type: pubsub.CreatedEvent,
		Payload: proto.RunComplete{
			SessionID: "sess-other",
			RunID:     runID,
			Text:      "reply-from-other",
		},
	})
	// Wait for the completion message and capture its message_id.
	var completionMsgID int64
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if id, ok := tg.findSentID("reply-from-other"); ok {
			completionMsgID = id
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.NotZero(t, completionMsgID)
	_ = beforeSent

	// Switch back to active.
	tg.pushUpdate(Update{
		UpdateID: 4,
		Message: &Message{
			MessageID: 103,
			Chat:      Chat{ID: chatID},
			Text:      "/use 1",
		},
	})
	time.Sleep(100 * time.Millisecond)

	// Reply to the other session's completion message.
	postsBefore := 0
	cf.mu.Lock()
	postsBefore = len(cf.agentPosts)
	cf.mu.Unlock()

	tg.pushUpdate(Update{
		UpdateID: 5,
		Message: &Message{
			MessageID: 104,
			Chat:      Chat{ID: chatID},
			Text:      "follow up",
			ReplyToMessage: &Message{
				MessageID: completionMsgID,
			},
		},
	})

	var followUp proto.AgentMessage
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		cf.mu.Lock()
		if len(cf.agentPosts) > postsBefore {
			followUp = cf.agentPosts[len(cf.agentPosts)-1]
			cf.mu.Unlock()
			break
		}
		cf.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	require.Equal(t, "follow up", followUp.Prompt)
	require.Equal(t, "sess-other", followUp.SessionID, "reply should target the non-active session")

	// Plain text should still go to active.
	postsBefore = len(cf.agentPosts)
	// Need fresh count under lock.
	cf.mu.Lock()
	postsBefore = len(cf.agentPosts)
	cf.mu.Unlock()
	tg.pushUpdate(Update{
		UpdateID: 6,
		Message: &Message{
			MessageID: 105,
			Chat:      Chat{ID: chatID},
			Text:      "plain to active",
		},
	})
	var plain proto.AgentMessage
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		cf.mu.Lock()
		if len(cf.agentPosts) > postsBefore {
			plain = cf.agentPosts[len(cf.agentPosts)-1]
			cf.mu.Unlock()
			break
		}
		cf.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	require.Equal(t, "plain to active", plain.Prompt)
	require.Equal(t, "sess-active", plain.SessionID)
}

// The permission service publishes a PermissionNotification with neither
// flag set as a "request created" hint; it must not expire the keyboard.
func TestBridgeIgnoresRequestedNotification(t *testing.T) {
	t.Parallel()
	const chatID int64 = 42
	cf, tg, cancel := startBridge(t, chatID)
	defer cancel()

	req := proto.PermissionRequest{
		ID:         "cccccccc-bbbb-cccc-dddd-eeeeeeeeeeee",
		SessionID:  "sess-active",
		ToolCallID: "tc-hint",
		ToolName:   "bash",
		Params:     proto.BashPermissionsParams{Command: "echo hi"},
	}
	cf.pushEvent(t, pubsub.PayloadTypePermissionRequest, pubsub.Event[proto.PermissionRequest]{
		Type:    pubsub.CreatedEvent,
		Payload: req,
	})
	waitKeyboard(t, tg)

	// The created-hint notification: both flags false.
	cf.pushEvent(t, pubsub.PayloadTypePermissionNotification, pubsub.Event[proto.PermissionNotification]{
		Type:    pubsub.CreatedEvent,
		Payload: proto.PermissionNotification{ToolCallID: "tc-hint"},
	})
	time.Sleep(300 * time.Millisecond)

	tg.mu.Lock()
	edits := len(tg.edits)
	tg.mu.Unlock()
	require.Zero(t, edits, "created-hint notification must not edit the keyboard")

	// The keyboard must still be actionable.
	tg.pushUpdate(Update{
		UpdateID: 2,
		CallbackQuery: &CallbackQuery{
			ID:      "cb-hint",
			From:    User{ID: chatID},
			Message: &Message{MessageID: 2, Chat: Chat{ID: chatID}},
			Data:    "perm:" + req.ID + ":allow",
		},
	})
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		cf.mu.Lock()
		n := len(cf.grantPosts)
		cf.mu.Unlock()
		if n > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expected grant after callback; pending keyboard was lost")
}

// Escaped HTML can exceed Telegram's size cap; the fallback must keep
// the approval keyboard attached (on the final plain chunk).
func TestBridgeOversizedPermissionKeepsKeyboard(t *testing.T) {
	t.Parallel()
	const chatID int64 = 42
	cf, tg, cancel := startBridge(t, chatID)
	defer cancel()

	req := proto.PermissionRequest{
		ID:         "dddddddd-bbbb-cccc-dddd-eeeeeeeeeeee",
		SessionID:  "sess-active",
		ToolCallID: "tc-big",
		ToolName:   "bash",
		// 1200 '<' truncate to 1000 and escape to ~4000 chars — over the cap.
		Params: proto.BashPermissionsParams{Command: strings.Repeat("<", 1200)},
	}
	cf.pushEvent(t, pubsub.PayloadTypePermissionRequest, pubsub.Event[proto.PermissionRequest]{
		Type:    pubsub.CreatedEvent,
		Payload: req,
	})

	kbBody := waitKeyboard(t, tg)
	require.Nil(t, kbBody["parse_mode"], "oversized fallback must be plain text")

	// And the keyboard must resolve normally.
	tg.pushUpdate(Update{
		UpdateID: 2,
		CallbackQuery: &CallbackQuery{
			ID:      "cb-big",
			From:    User{ID: chatID},
			Message: &Message{MessageID: 2, Chat: Chat{ID: chatID}},
			Data:    "perm:" + req.ID + ":deny",
		},
	})
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		cf.mu.Lock()
		n := len(cf.grantPosts)
		cf.mu.Unlock()
		if n > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expected grant after callback on oversized keyboard")
}

// /use must index into exactly the list /sessions displayed — never a
// list the user has not seen.
func TestBridgeUseRequiresSessionsList(t *testing.T) {
	t.Parallel()
	const chatID int64 = 42
	cf, tg, cancel := startBridge(t, chatID)
	defer cancel()

	tg.pushUpdate(Update{
		UpdateID: 1,
		Message:  &Message{MessageID: 100, Chat: Chat{ID: chatID}, Text: "/use 1"},
	})
	texts := tg.waitSent(t, 2, 3*time.Second)
	require.Contains(t, texts[len(texts)-1], "run /sessions first")

	tg.pushUpdate(Update{
		UpdateID: 2,
		Message:  &Message{MessageID: 101, Chat: Chat{ID: chatID}, Text: "/sessions"},
	})
	tg.waitSent(t, 3, 3*time.Second)

	// Displayed order is UpdatedAt desc: 1=active, 2=other.
	tg.pushUpdate(Update{
		UpdateID: 3,
		Message:  &Message{MessageID: 102, Chat: Chat{ID: chatID}, Text: "/use 2"},
	})
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		for _, tx := range tg.sentTexts() {
			if strings.Contains(tx, "active session: other") {
				goto prompt
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expected switch to session 'other'")

prompt:
	tg.pushUpdate(Update{
		UpdateID: 4,
		Message:  &Message{MessageID: 103, Chat: Chat{ID: chatID}, Text: "go"},
	})
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		cf.mu.Lock()
		if len(cf.agentPosts) > 0 {
			post := cf.agentPosts[len(cf.agentPosts)-1]
			cf.mu.Unlock()
			require.Equal(t, "sess-other", post.SessionID)
			return
		}
		cf.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expected prompt POST to sess-other")
}

// waitKeyboard waits for a sent message carrying an inline keyboard and
// returns its request body.
func waitKeyboard(t *testing.T, tg *tgFake) map[string]any {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		tg.mu.Lock()
		for _, s := range tg.sent {
			if s.body["reply_markup"] != nil {
				tg.mu.Unlock()
				return s.body
			}
		}
		tg.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expected a keyboard message")
	return nil
}
