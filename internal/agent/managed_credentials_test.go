package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"charm.land/catwalk/v3/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

// managedHarness is a coordinator over a mock OpenAI-compatible provider
// served by an httptest server, whose credential is managed.
type managedHarness struct {
	c      *coordinator
	m      *config.ManagedCredentials
	events <-chan pubsub.Event[notify.Notification]
	env    fakeEnv
	sub    SessionAgent

	mu   sync.Mutex
	seen []string
	// all records every request's Authorization, title requests included.
	all []string
	// reply, when set, answers a request itself and reports whether it did.
	reply func(w http.ResponseWriter, auth string) bool
}

func (h *managedHarness) allSeen() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.all...)
}

func (h *managedHarness) setReply(f func(w http.ResponseWriter, auth string) bool) {
	h.mu.Lock()
	h.reply = f
	h.mu.Unlock()
}

func (h *managedHarness) authSeen() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.seen...)
}

// newManagedHarness starts a provider that streams a reply when accept
// says so and answers 401 otherwise. onAuth, when set, runs on each
// recorded request before it is answered.
func newManagedHarness(t *testing.T, accept func(auth string) bool, onAuth func(h *managedHarness, auth string)) *managedHarness {
	t.Helper()
	const chunk = `{"id":"c","object":"chat.completion.chunk","created":0,"model":"mock-model","choices":[{"index":0,"delta":%s,"finish_reason":%s}]}`
	h := &managedHarness{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		h.mu.Lock()
		h.all = append(h.all, auth)
		reply := h.reply
		h.mu.Unlock()
		if reply != nil && reply(w, auth) {
			return
		}
		// Title generation runs in its own goroutine; only the subagent's
		// requests are recorded.
		if !strings.Contains(string(body), "Generate a concise title") {
			h.mu.Lock()
			h.seen = append(h.seen, auth)
			h.mu.Unlock()
			if onAuth != nil {
				onAuth(h, auth)
			}
		}
		if accept == nil || !accept(auth) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"secret upstream detail","type":"authentication_error"}}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: "+chunk+"\n\n", `{"role":"assistant","content":"done"}`, "null")
		fmt.Fprintf(w, "data: "+chunk+"\n\n", `{}`, `"stop"`)
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(srv.Close)

	h.env = testEnv(t)
	crushJSON := fmt.Sprintf(`{
  "options": {"disable_default_providers": true, "disable_provider_auto_update": true},
  "providers": {"mock": {"id": "mock", "name": "Mock", "type": "openai-compat",
    "base_url": %q, "api_key": "disk-key",
    "models": [{"id": "mock-model", "name": "Mock", "context_window": 8192, "default_max_tokens": 128}]}},
  "models": {"large": {"provider": "mock", "model": "mock-model"},
             "small": {"provider": "mock", "model": "mock-model"}}
}`, srv.URL)
	require.NoError(t, os.WriteFile(filepath.Join(h.env.workingDir, "crush.json"), []byte(crushJSON), 0o644))
	h.m = config.NewManagedCredentials()
	cfg, err := config.InitWithManaged(t.Context(), h.env.workingDir, "", false, h.m)
	require.NoError(t, err)
	cfg.SetupAgents()

	h.c = &coordinator{
		cfg:         cfg,
		sessions:    h.env.sessions,
		messages:    h.env.messages,
		permissions: h.env.permissions,
		history:     h.env.history,
		filetracker: *h.env.filetracker,
		managedWait: 5 * time.Second,
	}
	broker := pubsub.NewBroker[notify.Notification]()
	t.Cleanup(broker.Shutdown)
	h.c.notify = broker
	h.events = broker.Subscribe(t.Context())
	return h
}

// push stores a credential and fans it out the way the backend does.
func (h *managedHarness) push(token string) {
	h.m.Set("mock", config.ManagedCredential{AccessToken: token})
	h.c.cfg.SyncManagedProvider("mock")
}

// build builds the subagent once a credential has been pushed.
func (h *managedHarness) build(t *testing.T) {
	t.Helper()
	p, err := loadPrompt(h.c.cfg, "task", prompt.WithWorkingDir(h.env.workingDir))
	require.NoError(t, err)
	sub, err := h.c.buildAgent(t.Context(), &h.c.readyWg, p, h.c.cfg.Config().Agents[config.AgentTask], true)
	require.NoError(t, err)
	require.NoError(t, h.c.readyWg.Wait())
	h.sub = sub
}

func (h *managedHarness) run(t *testing.T) string {
	t.Helper()
	parent, err := h.env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)
	resp, err := h.c.runSubAgent(t.Context(), subAgentParams{
		Agent:          h.sub,
		SessionID:      parent.ID,
		AgentMessageID: "message-" + parent.ID,
		ToolCallID:     "call-" + parent.ID,
		Prompt:         "go",
		SessionTitle:   "sub",
	})
	require.NoError(t, err)
	if resp.IsError {
		return "ERROR: " + resp.Content
	}
	return strings.TrimSpace(resp.Content)
}

func requireCredentialsRequired(t *testing.T, events <-chan pubsub.Event[notify.Notification], reason string) {
	t.Helper()
	select {
	case ev := <-events:
		require.Equal(t, notify.TypeCredentialsRequired, ev.Payload.Type)
		require.Equal(t, "mock", ev.Payload.ProviderID)
		require.Equal(t, reason, ev.Payload.Reason)
	case <-time.After(10 * time.Second):
		t.Fatal("no credentials_required event")
	}
}

func requireNoEvent(t *testing.T, events <-chan pubsub.Event[notify.Notification]) {
	t.Helper()
	select {
	case ev := <-events:
		t.Fatalf("unexpected event %+v", ev.Payload)
	default:
	}
}

// A 401 on token A asks the owner for a replacement and the retry goes out
// with B. The managed token beats the key on disk from the first request.
func TestManagedCredential_StreamRetriesWithPushedToken(t *testing.T) {
	h := newManagedHarness(t, func(auth string) bool { return auth == "Bearer B" }, nil)
	h.push("A")
	h.build(t)

	go func() {
		requireCredentialsRequired(t, h.events, notify.CredentialsReasonUnauthorized)
		h.push("A") // re-pushing the rejected token does not satisfy the wait
		h.push("B")
	}()
	require.Equal(t, "done", h.run(t))
	require.Equal(t, []string{"Bearer A", "Bearer B"}, h.authSeen())
}

// A 401 on the replacement as well is terminal for the turn, and the
// error explains it without the provider's response body.
func TestManagedCredential_SecondUnauthorizedIsTerminal(t *testing.T) {
	h := newManagedHarness(t, nil, nil)
	h.push("A")
	h.build(t)

	go func() {
		requireCredentialsRequired(t, h.events, notify.CredentialsReasonUnauthorized)
		h.push("B")
	}()
	out := h.run(t)
	require.Contains(t, out, "managed credential for mock was rejected, and so was its replacement")
	require.NotContains(t, out, "secret upstream detail")
	require.NotContains(t, strings.ToLower(out), "sign in")
	require.Equal(t, []string{"Bearer A", "Bearer B"}, h.authSeen())
}

// When a different token is already present at the 401 it is used at once:
// no event, no wait.
func TestManagedCredential_UsesAlreadyPushedToken(t *testing.T) {
	var once sync.Once
	h := newManagedHarness(t, func(auth string) bool { return auth == "Bearer B" },
		func(h *managedHarness, auth string) {
			if auth == "Bearer A" {
				once.Do(func() { h.push("B") }) // lands while A is in flight
			}
		})
	h.push("A")
	h.build(t)
	h.c.managedWait = time.Millisecond // a wait would fail the turn

	require.Equal(t, "done", h.run(t))
	require.Equal(t, []string{"Bearer A", "Bearer B"}, h.authSeen())
	requireNoEvent(t, h.events)
}

// A pushed token is sent literally, never through the shell resolver, and
// never appears in the serialized config.
func TestManagedCredential_TokenIsLiteralAndNotSerialized(t *testing.T) {
	const token = "$(echo pwned)"
	h := newManagedHarness(t, func(auth string) bool { return auth == "Bearer "+token }, nil)
	h.push(token)
	h.build(t)

	require.Equal(t, "done", h.run(t))
	require.Equal(t, []string{"Bearer " + token}, h.authSeen())

	data, err := json.Marshal(h.c.cfg.Config())
	require.NoError(t, err)
	require.NotContains(t, string(data), token)
}

// No replacement within the timeout fails with the clear error.
func TestManagedCredential_NoReplacementTimesOut(t *testing.T) {
	h := newManagedHarness(t, nil, nil)
	h.push("A")
	h.build(t)
	h.c.managedWait = 100 * time.Millisecond

	out := h.run(t)
	require.Contains(t, out, "managed credential for mock was rejected and no replacement arrived")
	require.NotContains(t, out, "secret upstream detail")
	requireCredentialsRequired(t, h.events, notify.CredentialsReasonUnauthorized)
	require.Equal(t, []string{"Bearer A"}, h.authSeen())
}

// An expiring managed credential asks for a fresh one before the turn and
// proceeds with the current token when none arrives.
func TestRefreshTokenIfExpired_ManagedExpiring(t *testing.T) {
	h := newManagedHarness(t, nil, nil)
	c, m := h.c, h.m
	c.managedWait = 200 * time.Millisecond
	m.Set("mock", config.ManagedCredential{AccessToken: "old", ExpiresAt: time.Now().Add(10 * time.Second)})
	c.cfg.SyncManagedProvider("mock")
	providerCfg, _ := c.cfg.Config().Providers.Get("mock")

	start := time.Now()
	require.NoError(t, c.refreshTokenIfExpired(t.Context(), providerCfg))
	require.GreaterOrEqual(t, time.Since(start), c.managedWait, "waited for a push")
	requireCredentialsRequired(t, h.events, notify.CredentialsReasonExpiring)
	require.Equal(t, "old", c.models.Get().large.cred.AccessToken)

	// A push during the wait is picked up and the models are rebuilt.
	c.managedWait = 5 * time.Second
	done := make(chan error, 1)
	go func() { done <- c.refreshTokenIfExpired(t.Context(), providerCfg) }()
	requireCredentialsRequired(t, h.events, notify.CredentialsReasonExpiring)
	m.Set("mock", config.ManagedCredential{AccessToken: "new", ExpiresAt: time.Now().Add(time.Hour)})
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("turn did not resume after the push")
	}
	require.Equal(t, "new", c.models.Get().large.cred.AccessToken)

	// A credential well within its lifetime does not ask.
	require.NoError(t, c.refreshTokenIfExpired(t.Context(), providerCfg))
	time.Sleep(50 * time.Millisecond)
	requireNoEvent(t, h.events)
}

// The 401 handler judges the failure against the build the failing attempt
// ran on, not the credential current when the handler was created: a call
// set up under A whose attempt went out with B treats B as rejected and
// waits for C instead of retrying B.
func TestManagedCall_JudgesTheAttemptsCredential(t *testing.T) {
	h := newManagedHarness(t, nil, nil)
	c := h.c
	h.push("A")
	require.NoError(t, c.rebuildModels(t.Context()))
	mc := c.newManagedCall("mock")
	require.NotNil(t, mc)

	h.push("B")
	require.NoError(t, c.syncManagedModels(t.Context()))
	mc.record(c.models.Get().large) // what agent.go does as the attempt starts
	require.Equal(t, "B", c.models.Get().large.cred.AccessToken)

	done := make(chan error, 1)
	go func() { done <- mc.refresh(t.Context(), nil) }()
	requireCredentialsRequired(t, h.events, notify.CredentialsReasonUnauthorized)
	select {
	case err := <-done:
		t.Fatalf("retried without a replacement: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	h.push("C")
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("handler did not resume after the push")
	}
	require.Equal(t, "C", c.models.Get().large.cred.AccessToken)
	require.ErrorIs(t, mc.refresh(t.Context(), nil), errReplacementRejected, "a second 401 is terminal")
}

// A failed build never marks a managed version built: the models stay
// stale and the next check tries again.
func TestSyncManagedModels_FailedBuildStaysStale(t *testing.T) {
	h := newManagedHarness(t, nil, nil)
	c := h.c
	h.push("A")
	require.NoError(t, c.rebuildModels(t.Context()))
	built := c.models.Get().managedVersion

	// Break the selection so the next build fails.
	pc, _ := c.cfg.Config().Providers.Get("mock")
	models := pc.Models
	pc.Models = nil
	c.cfg.Config().Providers.Set("mock", pc)
	h.push("B")
	require.Error(t, c.syncManagedModels(t.Context()))
	require.Equal(t, built, c.models.Get().managedVersion)

	pc.Models = models
	c.cfg.Config().Providers.Set("mock", pc)
	require.NoError(t, c.syncManagedModels(t.Context()))
	require.Equal(t, "B", c.models.Get().large.cred.AccessToken)
}

// Deleting the credential moves the managed version, so the next subagent
// turn rebuilds and goes out with the disk key, not the deleted token.
func TestManagedCredential_DeleteStopsSendingToken(t *testing.T) {
	h := newManagedHarness(t, func(string) bool { return true }, nil)
	h.push("A")
	h.build(t)
	require.Equal(t, "done", h.run(t))

	h.m.Delete("mock")
	h.c.cfg.SyncManagedProvider("mock")
	require.Equal(t, "done", h.run(t))
	require.Equal(t, []string{"Bearer A", "Bearer disk-key"}, h.authSeen())
}

// Title errors on a managed model are mapped before they are logged.
func TestMapManagedErr_WithoutCallState(t *testing.T) {
	t.Parallel()
	pe := &fantasy.ProviderError{StatusCode: 401, Message: "secret upstream detail"}
	m := Model{Managed: true, ModelCfg: config.SelectedModel{Provider: "mock"}}
	err := mapManagedErr(nil, pe, m)
	require.Contains(t, err.Error(), "managed credential for mock was rejected")
	require.NotContains(t, err.Error(), "secret")
	require.True(t, isUnauthorized(err), "status checks still see the 401")
	require.Same(t, pe, mapManagedErr(nil, pe, Model{}), "unmanaged errors pass through")
}

// A queued call run as its own turn from inside a running turn gets its own
// managed state, and no state at all when it has none, rather than
// inheriting the running turn's.
func TestWithManagedCall_ShadowsOuterState(t *testing.T) {
	t.Parallel()
	outer := &managedCall{provider: "outer"}
	inner := &managedCall{provider: "inner"}
	ctx := withManagedCall(t.Context(), outer)
	require.Same(t, inner, managedCallFrom(withManagedCall(ctx, inner)))
	require.Nil(t, managedCallFrom(withManagedCall(ctx, nil)))
	require.Same(t, t.Context(), withManagedCall(t.Context(), nil), "nothing to shadow")
}

// After a DELETE, title generation and summarize fail closed when the
// models cannot be rebuilt: they never go out with the deleted token.
func TestManagedCredential_DeleteFailsClosed(t *testing.T) {
	h := newManagedHarness(t, func(string) bool { return true }, nil)
	h.push("A")
	h.build(t)
	h.c.agentMu.Lock()
	h.c.mainAgent = h.sub
	h.c.agentMu.Unlock()
	require.Equal(t, "done", h.run(t))
	before := len(h.allSeen())

	h.m.Delete("mock")
	h.c.cfg.SyncManagedProvider("mock")
	pc, _ := h.c.cfg.Config().Providers.Get("mock")
	pc.Models = nil // the rebuild after the delete fails
	h.c.cfg.Config().Providers.Set("mock", pc)

	sess, err := h.env.sessions.Create(t.Context(), "s")
	require.NoError(t, err)
	require.Error(t, h.c.Summarize(t.Context(), sess.ID))
	h.c.GenerateTitle(t.Context(), sess.ID, "hello")
	require.Len(t, h.allSeen(), before, "nothing was sent after the delete")
}

// A provider error that echoes the pushed token never repeats it, in the
// returned error or in the logs.
func TestManagedCredential_ErrorsAreRedacted(t *testing.T) {
	var logs strings.Builder
	var logMu sync.Mutex
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(writerFunc(func(p []byte) (int, error) {
		logMu.Lock()
		defer logMu.Unlock()
		return logs.Write(p)
	}), &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	const token = "sk-secret-echoed-token"
	h := newManagedHarness(t, nil, nil)
	h.setReply(func(w http.ResponseWriter, _ string) bool {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid key ` + token + `","type":"invalid_request_error"}}`))
		return true
	})
	h.push(token)
	h.build(t)

	out := h.run(t)
	require.Contains(t, out, "ERROR")
	require.NotContains(t, out, token)
	require.Contains(t, out, redacted)
	logMu.Lock()
	defer logMu.Unlock()
	require.NotContains(t, logs.String(), token)
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

func TestRedactErr(t *testing.T) {
	t.Parallel()
	pe := &fantasy.ProviderError{
		StatusCode:   500,
		Message:      "bad tok-1",
		RequestBody:  []byte("Authorization: Bearer tok-1"),
		ResponseBody: []byte("tok-1"),
		Cause:        errors.New("cause tok-1"),
	}
	err := redactErr(fmt.Errorf("wrapped: %w", pe), "tok-1")
	require.NotContains(t, err.Error(), "tok-1")
	got, ok := errors.AsType[*fantasy.ProviderError](err)
	require.True(t, ok)
	require.Equal(t, 500, got.StatusCode)
	for _, s := range []string{got.Message, string(got.RequestBody), string(got.ResponseBody), got.Cause.Error()} {
		require.NotContains(t, s, "tok-1")
	}
	plain := errors.New("nothing secret")
	require.Same(t, plain, redactErr(plain, "tok-1"))
}

// The same push presents identically whether or not the workspace has a
// local OAuth login, and anthropic+oauth goes out as a bearer token while
// anthropic+api-key goes out as x-api-key.
func TestManagedKind_PresentsIdentically(t *testing.T) {
	t.Parallel()
	type seenReq struct{ auth, apiKey string }
	var (
		mu  sync.Mutex
		got []seenReq
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got = append(got, seenReq{r.Header.Get("Authorization"), r.Header.Get("X-Api-Key")})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"invalid_request_error","message":"stop"}}`))
	}))
	t.Cleanup(srv.Close)

	store := config.NewTestStore(&config.Config{Options: &config.Options{}})
	c := &coordinator{cfg: store}
	present := func(login *oauth.Token, kind config.ManagedCredentialKind) seenReq {
		pc := config.ProviderConfig{ID: "anthropic", BaseURL: srv.URL, Type: catwalk.TypeMessages, OAuthToken: login}
		cred := config.ManagedCredential{AccessToken: "pushed", Kind: kind}
		p, err := c.buildProviderWith(pc, cred, true, config.SelectedModel{Model: "m"}, false, catwalk.TypeMessages)
		require.NoError(t, err)
		lm, err := p.LanguageModel(t.Context(), "m")
		require.NoError(t, err)
		_, _ = lm.Generate(t.Context(), fantasy.Call{Prompt: fantasy.Prompt{fantasy.NewUserMessage("hi")}})
		mu.Lock()
		defer mu.Unlock()
		return got[len(got)-1]
	}
	withLogin := present(&oauth.Token{AccessToken: "local", RefreshToken: "rt", AccountID: "acct"}, config.ManagedKindOAuth)
	without := present(nil, config.ManagedKindOAuth)
	require.Equal(t, withLogin, without)
	require.Equal(t, seenReq{auth: "Bearer pushed"}, without)

	apiKey := present(nil, config.ManagedKindAPIKey)
	require.Equal(t, seenReq{apiKey: "pushed"}, apiKey)
	require.Equal(t, apiKey, present(&oauth.Token{AccessToken: "local"}, config.ManagedKindAPIKey))
}
