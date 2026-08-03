package remotecontrol

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestLoginRequiresHTTP200AndToken(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/login", r.URL.Path)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"success":false,"message":"nope"}`))
	}))
	t.Cleanup(srv.Close)

	c := NewClient(Config{
		RelayURL: "ws" + strings.TrimPrefix(srv.URL, "http"),
		Username: "u",
		Password: "not-default-pass",
	})
	_, err := c.login(t.Context())
	require.Error(t, err)
	require.Contains(t, err.Error(), "401")
}

func TestConnectRegisterAndInboundPrompt(t *testing.T) {
	t.Parallel()

	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	var (
		mu        sync.Mutex
		gotFrames []EventMessage
		cliConn   *websocket.Conn
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"token":"test-token"}`))
	})
	mux.HandleFunc("/ws/cli", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "test-token", r.URL.Query().Get("token"))
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		mu.Lock()
		cliConn = conn
		mu.Unlock()
		for {
			var msg EventMessage
			if err := conn.ReadJSON(&msg); err != nil {
				return
			}
			mu.Lock()
			gotFrames = append(gotFrames, msg)
			mu.Unlock()
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	wsBase := "ws" + strings.TrimPrefix(srv.URL, "http")
	c := NewClient(Config{
		RelayURL: wsBase,
		Username: "u",
		Password: "not-default-pass",
	})

	var (
		promptMu  sync.Mutex
		gotSID    string
		gotPrompt string
	)
	c.SetHandlers(Handlers{
		OnPrompt: func(sessionID, prompt string) {
			promptMu.Lock()
			gotSID = sessionID
			gotPrompt = prompt
			promptMu.Unlock()
		},
	})

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	require.NoError(t, c.Connect(ctx))
	t.Cleanup(func() { _ = c.Close() })

	require.NoError(t, c.RegisterSession(SessionInfo{
		ID:    "sess-1",
		Title: "Test",
		Cwd:   "/tmp",
	}))

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		for _, f := range gotFrames {
			if f.Type == TypeRegisterSession && f.SessionID == "sess-1" {
				return true
			}
		}
		return false
	}, 2*time.Second, 20*time.Millisecond)

	// Deliver a prompt from the "relay".
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return cliConn != nil
	}, 2*time.Second, 20*time.Millisecond)

	payload, _ := json.Marshal(SendPromptPayload{Prompt: "hello from phone"})
	mu.Lock()
	err := cliConn.WriteJSON(EventMessage{
		Type:      TypeSendPrompt,
		SessionID: "sess-1",
		Payload:   payload,
		Timestamp: time.Now().Unix(),
	})
	mu.Unlock()
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		promptMu.Lock()
		defer promptMu.Unlock()
		return gotSID == "sess-1" && gotPrompt == "hello from phone"
	}, 2*time.Second, 20*time.Millisecond)

	require.True(t, c.HasSession("sess-1"))
	require.Equal(t, 1, c.SessionCount())
}

func TestConcurrentSendsDoNotPanic(t *testing.T) {
	t.Parallel()

	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"token":"t"}`))
	})
	mux.HandleFunc("/ws/cli", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := NewClient(Config{
		RelayURL: "ws" + strings.TrimPrefix(srv.URL, "http"),
		Username: "u",
		Password: "not-default-pass",
	})
	require.NoError(t, c.Connect(t.Context()))
	t.Cleanup(func() { _ = c.Close() })

	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			_ = c.SendStreamChunk("s", StreamChunkPayload{Role: "assistant", Content: "x"})
		})
	}
	wg.Wait()
}

// TestReconnectDuringCloseDoesNotRace hammers Connect and Close from several
// goroutines against a live relay. The old code reassigned c.closeOnce under
// c.mu while lock-free callers ran Do on it, a data race the race detector
// flags; this test is the load generator. It is deterministic-passing on the
// fix and exercises the exact interleaving on CI with -race -count=20.
func TestReconnectDuringCloseDoesNotRace(t *testing.T) {
	t.Parallel()

	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"token":"test-token"}`))
	})
	mux.HandleFunc("/ws/cli", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		for {
			var msg EventMessage
			if err := conn.ReadJSON(&msg); err != nil {
				return
			}
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	wsBase := "ws" + strings.TrimPrefix(srv.URL, "http")

	c := NewClient(Config{RelayURL: wsBase, Username: "u", Password: "not-default-pass"})

	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				_ = c.Connect(ctx)
				cancel()
				_ = c.Close()
			}
		}()
	}
	wg.Wait()
}

// TestStaleReadLoopDoesNotTearDownNewConnection forces the first
// connection's readLoop to exit and reconnects before its teardown can run.
// The old readLoop defer cleared connected/ws unconditionally, so a dying old
// connection could mark a brand-new healthy one as down.
func TestStaleReadLoopDoesNotTearDownNewConnection(t *testing.T) {
	t.Parallel()

	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	var (
		mu    sync.Mutex
		conns []*websocket.Conn
	)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"token":"test-token"}`))
	})
	mux.HandleFunc("/ws/cli", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		mu.Lock()
		conns = append(conns, conn)
		mu.Unlock()
		for {
			var msg EventMessage
			if err := conn.ReadJSON(&msg); err != nil {
				return
			}
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	wsBase := "ws" + strings.TrimPrefix(srv.URL, "http")

	c := NewClient(Config{RelayURL: wsBase, Username: "u", Password: "not-default-pass"})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	require.NoError(t, c.Connect(ctx))
	cancel()
	require.True(t, c.IsConnected())

	mu.Lock()
	first := conns[0]
	mu.Unlock()
	// Kill the first connection server-side so the client readLoop exits.
	_ = first.Close()

	// Reconnect immediately, while the old readLoop's teardown may still be
	// in flight. Give a straggler teardown time to fire, then require the new
	// connection to be up.
	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second)
	require.NoError(t, c.Connect(ctx2))
	cancel2()
	time.Sleep(200 * time.Millisecond)
	require.True(t, c.IsConnected(),
		"a stale readLoop must not tear down a brand-new connection")
}
