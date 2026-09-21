package honcho

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// svcFake is a stand-in Honcho deployment. It counts requests per
// endpoint so tests can assert on how often Crush reaches for memory,
// and records written messages so tests can assert on what it stored.
type svcFake struct {
	srv *httptest.Server

	mu       sync.Mutex
	counts   map[string]int
	messages []Message

	// Knobs, all guarded by mu.
	summariesStatus  int
	peerStatus       int
	sessionStatus    int
	peerRep          string
	peerCard         []string
	longSummary      string
	sessionRep       string
	sessionRepUnique bool
	messageDelay     time.Duration
}

// svcNewFake starts a fake deployment that answers every endpoint the
// service uses with a 200 by default.
func svcNewFake(t *testing.T) *svcFake {
	t.Helper()

	f := &svcFake{
		counts:          map[string]int{},
		summariesStatus: http.StatusOK,
		peerStatus:      http.StatusOK,
		sessionStatus:   http.StatusOK,
	}
	f.srv = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *svcFake) handle(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path

	switch {
	case strings.HasSuffix(p, "/summaries"):
		f.bump("summaries")
		if code := f.read(func() int { return f.summariesStatus }); code != http.StatusOK {
			http.Error(w, "boom", code)
			return
		}
		long := f.readString(func() string { return f.longSummary })
		out := Summaries{ID: "s"}
		if long != "" {
			out.Long = &Summary{Content: long}
		}
		svcWriteJSON(w, out)

	case strings.Contains(p, "/peers/") && strings.HasSuffix(p, "/context"):
		f.bump("peer_context")
		if code := f.read(func() int { return f.peerStatus }); code != http.StatusOK {
			http.Error(w, "boom", code)
			return
		}
		f.mu.Lock()
		out := PeerContext{Representation: f.peerRep, PeerCard: f.peerCard}
		f.mu.Unlock()
		svcWriteJSON(w, out)

	case strings.Contains(p, "/sessions/") && strings.HasSuffix(p, "/context"):
		n := f.bump("session_context")
		if code := f.read(func() int { return f.sessionStatus }); code != http.StatusOK {
			http.Error(w, "boom", code)
			return
		}
		f.mu.Lock()
		rep := f.sessionRep
		unique := f.sessionRepUnique
		f.mu.Unlock()
		if unique && rep != "" {
			rep = fmt.Sprintf("%s (lookup %d)", rep, n)
		}
		svcWriteJSON(w, SessionContext{Representation: rep})

	case strings.HasSuffix(p, "/messages"):
		f.bump("messages")
		if d := f.readDuration(func() time.Duration { return f.messageDelay }); d > 0 {
			time.Sleep(d)
		}
		var body struct {
			Messages []Message `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		f.messages = append(f.messages, body.Messages...)
		f.mu.Unlock()
		svcWriteJSON(w, map[string]any{})

	case strings.HasSuffix(p, "/sessions"):
		f.bump("sessions")
		svcWriteJSON(w, Session{ID: "session"})

	case strings.HasSuffix(p, "/workspaces"):
		f.bump("workspaces")
		svcWriteJSON(w, Workspace{ID: "ws"})

	default:
		f.bump("other")
		svcWriteJSON(w, map[string]any{})
	}
}

// bump records a hit and returns the new count for that endpoint.
func (f *svcFake) bump(key string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.counts[key]++
	return f.counts[key]
}

func (f *svcFake) count(key string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.counts[key]
}

func (f *svcFake) total() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, v := range f.counts {
		n += v
	}
	return n
}

func (f *svcFake) read(fn func() int) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return fn()
}

func (f *svcFake) readString(fn func() string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return fn()
}

func (f *svcFake) readDuration(fn func() time.Duration) time.Duration {
	f.mu.Lock()
	defer f.mu.Unlock()
	return fn()
}

func (f *svcFake) recorded() []Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Message, len(f.messages))
	copy(out, f.messages)
	return out
}

// contents returns every recorded message body.
func (f *svcFake) contents() []string {
	var out []string
	for _, m := range f.recorded() {
		out = append(out, m.Content)
	}
	return out
}

func svcWriteJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// svcConfig builds a valid config pointed at the fake. The fake listens
// on loopback, so no API key is required.
func svcConfig(f *svcFake) Config {
	return Config{
		Enabled:         true,
		BaseURL:         f.srv.URL,
		Workspace:       "test-ws",
		PeerName:        "user",
		AgentPeer:       "crush",
		RecallMode:      RecallHybrid,
		ObservationMode: ObserveUnified,
		SessionStrategy: StrategyGlobal,
		CaptureTools:    true,
		MaxConclusions:  8,
		ContextTokens:   2000,
	}
}

// svcNew builds a service against the fake, closing it on cleanup so no
// writer goroutine outlives the test.
func svcNew(t *testing.T, f *svcFake, mutate func(*Config)) *Service {
	t.Helper()

	cfg := svcConfig(f)
	if mutate != nil {
		mutate(&cfg)
	}
	s := NewService(cfg, t.TempDir(), "crush-session-id", "test")
	require.NotNil(t, s)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.Close(ctx)
	})
	return s
}

func TestNewServiceRejectsUnusableConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  Config
	}{
		{
			name: "disabled",
			cfg:  Config{Workspace: "w", PeerName: "user", AgentPeer: "crush"},
		},
		{
			name: "cloud without api key",
			cfg: Config{
				Enabled:   true,
				Workspace: "w",
				PeerName:  "user",
				AgentPeer: "crush",
			},
		},
		{
			name: "peer collision",
			cfg: Config{
				Enabled:   true,
				APIKey:    "k",
				Workspace: "w",
				PeerName:  "crush",
				AgentPeer: "crush",
			},
		},
		{
			name: "missing workspace",
			cfg: Config{
				Enabled:   true,
				APIKey:    "k",
				PeerName:  "user",
				AgentPeer: "crush",
			},
		},
		{
			name: "missing peer name",
			cfg: Config{
				Enabled:   true,
				APIKey:    "k",
				Workspace: "w",
				AgentPeer: "crush",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Nil(t, NewService(tt.cfg, t.TempDir(), "sid", "test"))
		})
	}
}

func TestNilServiceIsInert(t *testing.T) {
	t.Parallel()

	var s *Service
	ctx := context.Background()

	require.NotPanics(t, func() { require.Empty(t, s.Snapshot(ctx)) })
	require.NotPanics(t, func() { require.Empty(t, s.Recall(ctx, "anything at all")) })
	require.NotPanics(t, func() { s.RecordTool("Ran: go test") })
	require.NotPanics(t, func() { s.record("peer", "text", nil) })
	require.NotPanics(t, func() { s.Close(ctx) })
	require.NotPanics(t, func() { require.ErrorIs(t, s.EnsureTopology(ctx), ErrDisabled) })
	require.NotPanics(t, func() { require.Equal(t, Identity{}, s.Identity()) })
	require.NotPanics(t, func() { require.Equal(t, Config{}, s.Config()) })
	require.NotPanics(t, func() { require.Nil(t, s.Client()) })
	require.NotPanics(t, func() { require.False(t, s.Enabled()) })
}

// TestNilServiceRecordIsInert checks the package's promise that every
// exported method tolerates a nil receiver, so an unconfigured
// integration costs a nil check rather than a crash.
func TestNilServiceRecordIsInert(t *testing.T) {
	t.Parallel()

	var s *Service
	require.NotPanics(t, func() { s.RecordUser("hello") })
	require.NotPanics(t, func() { s.RecordAssistant("hello") })
	require.NotPanics(t, func() { s.RecordTool("ran something") })
}

func TestSnapshotHydratesOnceAndFreezes(t *testing.T) {
	t.Parallel()

	f := svcNewFake(t)
	f.peerRep = "Kieran prefers Go and terse prose."
	f.peerCard = []string{"name: Kieran"}
	f.longSummary = "Earlier we shipped the hook engine."

	s := svcNew(t, f, nil)
	ctx := context.Background()

	first := s.Snapshot(ctx)
	require.NotEmpty(t, first)
	require.Contains(t, first, "Kieran prefers Go")
	require.Contains(t, first, "Earlier we shipped the hook engine.")
	require.Contains(t, first, MemoryInstruction)

	second := s.Snapshot(ctx)
	require.Equal(t, first, second, "snapshot must stay byte-identical")

	require.Equal(t, 1, f.count("peer_context"))
	require.Equal(t, 1, f.count("summaries"))

	// A third call still costs nothing.
	require.Equal(t, first, s.Snapshot(ctx))
	require.Equal(t, 1, f.count("peer_context"))
	require.Equal(t, 1, f.count("summaries"))
}

func TestSnapshotConcurrentCallsAgree(t *testing.T) {
	t.Parallel()

	f := svcNewFake(t)
	f.peerRep = "stable profile"

	s := svcNew(t, f, nil)
	ctx := context.Background()

	const n = 8
	results := make([]string, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = s.Snapshot(ctx)
		}()
	}
	wg.Wait()

	for _, got := range results {
		require.Equal(t, results[0], got)
	}
	require.NotEmpty(t, results[0])
}

func TestSnapshotToleratesPartialFailure(t *testing.T) {
	t.Parallel()

	f := svcNewFake(t)
	f.summariesStatus = http.StatusInternalServerError
	f.peerRep = "Kieran runs CTFs on weekends."

	s := svcNew(t, f, nil)

	block := s.Snapshot(context.Background())
	require.NotEmpty(t, block, "a failing section must not sink the block")
	require.Contains(t, block, "User memory profile")
	require.Contains(t, block, "Kieran runs CTFs on weekends.")
	require.NotContains(t, block, "Earlier session summary")
}

func TestSnapshotEmptyWhenNothingToSay(t *testing.T) {
	t.Parallel()

	f := svcNewFake(t)
	s := svcNew(t, f, nil)

	require.Empty(t, s.Snapshot(context.Background()))
}

func TestSnapshotSilentInToolsMode(t *testing.T) {
	t.Parallel()

	f := svcNewFake(t)
	f.peerRep = "would have been injected"

	s := svcNew(t, f, func(c *Config) { c.RecallMode = RecallTools })

	require.Empty(t, s.Snapshot(context.Background()))
	require.Zero(t, f.total(), "tools mode must not touch the network")
}

func TestRecallGating(t *testing.T) {
	t.Parallel()

	f := svcNewFake(t)
	f.sessionRep = "relevant memory"
	f.sessionRepUnique = true

	s := svcNew(t, f, nil)
	ctx := context.Background()

	const prompt = "refactor the honcho service writer loop"

	first := s.Recall(ctx, prompt)
	require.NotEmpty(t, first)
	require.Contains(t, first, "Relevant memory")
	require.Contains(t, first, MemoryInstruction)
	require.Equal(t, 1, f.count("session_context"))

	// The same prompt moments later is not worth another lookup.
	require.Empty(t, s.Recall(ctx, prompt))
	require.Equal(t, 1, f.count("session_context"))

	// A genuine change of subject is.
	second := s.Recall(ctx, "investigate the kubernetes deployment manifests")
	require.Equal(t, 2, f.count("session_context"))
	require.NotEmpty(t, second)
	require.NotEqual(t, first, second)
}

func TestRecallSkipsTrivialPrompts(t *testing.T) {
	t.Parallel()

	f := svcNewFake(t)
	f.sessionRep = "never fetched"

	s := svcNew(t, f, nil)
	ctx := context.Background()

	for _, prompt := range []string{"ok", "ship it", "y", "go", "lgtm", "thanks", ""} {
		require.Empty(t, s.Recall(ctx, prompt), "prompt %q", prompt)
	}
	require.Zero(t, f.count("session_context"), "trivial prompts must cost nothing")
}

func TestRecallSuppressesIdenticalBlock(t *testing.T) {
	t.Parallel()

	f := svcNewFake(t)
	// Constant representation, so the second lookup returns the block
	// the model already has.
	f.sessionRep = "the same memory every time"

	s := svcNew(t, f, nil)
	ctx := context.Background()

	first := s.Recall(ctx, "refactor the honcho writer loop carefully")
	require.NotEmpty(t, first)
	require.Equal(t, 1, f.count("session_context"))

	second := s.Recall(ctx, "investigate kubernetes deployment manifests instead")
	require.Equal(t, 2, f.count("session_context"), "a new topic still looks up")
	require.Empty(t, second, "an unchanged block must not be re-injected")
}

func TestRecallEmptyWhenLookupFails(t *testing.T) {
	t.Parallel()

	f := svcNewFake(t)
	f.sessionStatus = http.StatusNotFound

	s := svcNew(t, f, nil)

	require.Empty(t, s.Recall(context.Background(), "refactor the honcho service"))
}

func TestRecallSilentInToolsMode(t *testing.T) {
	t.Parallel()

	f := svcNewFake(t)
	f.sessionRep = "would have been injected"

	s := svcNew(t, f, func(c *Config) { c.RecallMode = RecallTools })

	require.Empty(t, s.Recall(context.Background(), "refactor the honcho service"))
	require.Zero(t, f.total())
}

func TestWritesAreDeliveredOnClose(t *testing.T) {
	t.Parallel()

	f := svcNewFake(t)
	s := svcNew(t, f, nil)

	s.RecordUser("please audit the redactor")
	s.RecordAssistant("found two gaps")
	s.RecordTool("Ran: go test ./...")
	s.RecordUser("second turn")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.Close(ctx)

	got := f.contents()
	require.Contains(t, got, "please audit the redactor")
	require.Contains(t, got, "found two gaps")
	require.Contains(t, got, "[Tool] Ran: go test ./...")
	require.Contains(t, got, "second turn")
	require.Len(t, got, 4)

	// Peers are attributed correctly.
	byContent := map[string]string{}
	for _, m := range f.recorded() {
		byContent[m.Content] = m.PeerID
	}
	id := s.Identity()
	require.Equal(t, id.UserPeer, byContent["please audit the redactor"])
	require.Equal(t, id.AgentPeer, byContent["found two gaps"])
	require.Equal(t, id.AgentPeer, byContent["[Tool] Ran: go test ./..."])
}

func TestRecordToolHonoursCaptureToolsFlag(t *testing.T) {
	t.Parallel()

	f := svcNewFake(t)
	s := svcNew(t, f, func(c *Config) { c.CaptureTools = false })

	s.RecordTool("Ran: go build")
	s.RecordUser("keep this one")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.Close(ctx)

	require.Equal(t, []string{"keep this one"}, f.contents())
}

func TestRecordIgnoresBlankText(t *testing.T) {
	t.Parallel()

	f := svcNewFake(t)
	s := svcNew(t, f, nil)

	s.RecordUser("")
	s.RecordUser("   \n\t ")
	s.RecordAssistant("")
	s.RecordAssistant(" \n ")
	s.RecordTool("")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.Close(ctx)

	require.Empty(t, f.contents())
	require.Zero(t, f.count("messages"), "blank turns must not reach the API")
}

// TestRecordToolWhitespaceSummary checks that a contentless summary
// is dropped rather than written as a bare "[Tool]" marker.
func TestRecordToolWhitespaceSummary(t *testing.T) {
	t.Parallel()

	f := svcNewFake(t)
	s := svcNew(t, f, nil)

	s.RecordTool("   \n\t ")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.Close(ctx)

	require.Empty(t, f.contents(), "a blank summary must not reach the API")
}

func TestCloseIsIdempotent(t *testing.T) {
	t.Parallel()

	f := svcNewFake(t)
	cfg := svcConfig(f)
	s := NewService(cfg, t.TempDir(), "sid", "test")
	require.NotNil(t, s)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NotPanics(t, func() {
		s.Close(ctx)
		s.Close(ctx)
		s.Close(ctx)
	})
}

func TestCloseReturnsPromptlyOnCancelledContext(t *testing.T) {
	t.Parallel()

	f := svcNewFake(t)
	f.messageDelay = time.Second

	cfg := svcConfig(f)
	s := NewService(cfg, t.TempDir(), "sid", "test")
	require.NotNil(t, s)

	s.RecordUser("a turn that will be slow to deliver")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	s.Close(ctx)
	require.Less(t, time.Since(start), 500*time.Millisecond,
		"Close must honour an expired context rather than wait on the flush")

	// Let the in-flight write finish so the server can shut down.
	drain, drainCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer drainCancel()
	s.Close(drain)
}

func TestEnsureTopologyCreatesWorkspaceAndSession(t *testing.T) {
	t.Parallel()

	f := svcNewFake(t)
	s := svcNew(t, f, nil)

	require.NoError(t, s.EnsureTopology(context.Background()))
	require.Equal(t, 1, f.count("workspaces"))
	require.Equal(t, 1, f.count("sessions"))
}

func TestServiceAccessors(t *testing.T) {
	t.Parallel()

	f := svcNewFake(t)
	s := svcNew(t, f, nil)

	require.True(t, s.Enabled())
	require.Equal(t, "test-ws", s.Config().Workspace)
	require.NotNil(t, s.Client())
	require.Equal(t, f.srv.URL, s.Client().BaseURL())

	id := s.Identity()
	require.Equal(t, "test-ws", id.Workspace)
	require.Equal(t, "user", id.UserPeer)
	require.Equal(t, "crush", id.AgentPeer)
	require.NotEmpty(t, id.SessionKey)
}

func TestBuildBlock(t *testing.T) {
	t.Parallel()

	t.Run("all empty yields nothing", func(t *testing.T) {
		t.Parallel()

		require.Empty(t, buildBlock(nil, 100))
		require.Empty(t, buildBlock([]section{{"A", ""}, {"B", "   \n\t"}}, 100))
	})

	t.Run("non empty carries the guard text", func(t *testing.T) {
		t.Parallel()

		got := buildBlock([]section{
			{"User memory profile", "likes Go"},
			{"Skipped", "  "},
			{"Earlier session summary", "shipped hooks"},
		}, 100)

		require.Contains(t, got, MemoryInstruction)
		require.True(t, strings.HasPrefix(got, "<memory>\n"))
		require.True(t, strings.HasSuffix(got, "\n</memory>"))
		require.Contains(t, got, "## User memory profile")
		require.Contains(t, got, "## Earlier session summary")
		require.NotContains(t, got, "## Skipped")
	})

	t.Run("bodies are clamped", func(t *testing.T) {
		t.Parallel()

		body := strings.Repeat("x", 500)
		got := buildBlock([]section{{"Title", body}}, 10)
		require.Contains(t, got, strings.Repeat("x", 10))
		require.NotContains(t, got, strings.Repeat("x", 11))
	})
}

func TestJoinCard(t *testing.T) {
	t.Parallel()

	require.Empty(t, joinCard(nil, ""))
	require.Equal(t, "rep", joinCard(nil, "  rep  "))
	require.Equal(t, "a\nb", joinCard([]string{"a", "b"}, ""))
	require.Equal(t, "a\nb\n\nrep", joinCard([]string{"a", "b"}, "rep"))
}

func TestIsTrivialPrompt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		prompt string
		want   bool
	}{
		{"", true},
		{"   ", true},
		{"ok", true},
		{"OK", true},
		{"Ok.", true},
		{"yes!", true},
		{"y", true},
		{"n", true},
		{"go", true},
		{"go ahead", true},
		{"ship it", true},
		{"lgtm", true},
		{"thanks", true},
		{"continue", true},
		{"do it", true},
		{"abc", true},
		{"fix", true},
		{"fix bug", false},
		{"debug", false},
		{"refactor the honcho service", false},
		{"why is the redactor leaking?", false},
		{"continue with the other file", false},
	}

	for _, tt := range tests {
		t.Run("prompt/"+tt.prompt, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, isTrivial(tt.prompt))
		})
	}
}

func TestTopicKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		prompt string
		want   string
	}{
		{"empty", "", ""},
		{"no long words", "ok go fix it", ""},
		{"keeps long words", "refactor the honcho service writer", "refactor honcho service writer"},
		{"lowercases and strips punctuation", "Refactor, the HONCHO service!", "refactor honcho service"},
		{"keeps digits", "update123 the config file", "update123 config"},
		{
			"caps at six words",
			"alpha1 bravo1 charlie delta1 echoes foxtrot golfer hotels",
			"alpha1 bravo1 charlie delta1 echoes foxtrot",
		},
		{"four letter words drop out", "fix this bug now please", "please"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, topicKey(tt.prompt))
		})
	}

	// The same subject in a different order fingerprints differently,
	// which is what makes a reworded follow-up trigger a refresh.
	require.NotEqual(t,
		topicKey("refactor the honcho service"),
		topicKey("investigate the kubernetes manifests"),
	)
}
