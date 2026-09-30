// internal/agent/router_model_override_test.go
package agent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newRouterModelPoolTestCoordinator builds a coordinator against a
// hermetic config with two providers: "mock" (an openai-typed provider on
// a closed port, used as the agent's normal large/small model — agent.Run
// will fail deep inside the real fantasy call, well after this task's
// model-resolution logic has already executed) and "openrouter" (whose
// models catalog is exactly openrouterModels, so tests can exercise both
// a pool member that IS and is NOT in that catalog).
func newRouterModelPoolTestCoordinator(t *testing.T, openrouterModels string) *coordinator {
	t.Helper()

	env := testEnv(t)

	crushJSON := `{
  "options": {
    "disable_default_providers": true,
    "disable_provider_auto_update": true
  },
  "providers": {
    "mock": {"id": "mock", "name": "Mock", "type": "openai",
      "base_url": "http://127.0.0.1:9/v1", "api_key": "test-key",
      "models": [{"id": "mock-model", "name": "Mock", "context_window": 8192, "default_max_tokens": 128}]},
    "openrouter": {"id": "openrouter", "name": "OpenRouter", "type": "openrouter",
      "base_url": "http://127.0.0.1:9", "api_key": "test-key",
      "models": ` + openrouterModels + `}
  },
  "models": {"large": {"provider": "mock", "model": "mock-model"},
             "small": {"provider": "mock", "model": "mock-model"}}
}`
	require.NoError(t, os.WriteFile(filepath.Join(env.workingDir, "crush.json"), []byte(crushJSON), 0o644))

	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	cfg.SetupAgents()

	coord := &coordinator{
		cfg:         cfg,
		sessions:    env.sessions,
		messages:    env.messages,
		permissions: env.permissions,
		history:     env.history,
		filetracker: *env.filetracker,
		agents:      make(map[string]SessionAgent),
		interactive: true,
	}

	p, err := coderPrompt(prompt.WithWorkingDir(env.workingDir))
	require.NoError(t, err)
	agentCfg := cfg.Config().Agents[config.AgentCoder]

	agent, err := coord.buildAgent(context.Background(), p, agentCfg, false)
	require.NoError(t, err)
	coord.mainAgent = agent
	coord.mainAgentName = config.AgentCoder
	coord.agents[config.AgentCoder] = agent

	return coord
}

const oneOpusModel = `[{"id": "anthropic/claude-opus-4", "name": "Opus", "context_window": 200000, "default_max_tokens": 4096}]`

func TestResolveRouterModelOverride_EmptyModelIDFailsOpen(t *testing.T) {
	t.Parallel()

	c := newRouterModelPoolTestCoordinator(t, oneOpusModel)
	_, ok := c.resolveRouterModelOverride(t.Context(), []string{"anthropic/claude-opus-4"}, "")
	require.False(t, ok)
}

func TestResolveRouterModelOverride_NotInPoolFailsOpen(t *testing.T) {
	t.Parallel()

	c := newRouterModelPoolTestCoordinator(t, oneOpusModel)
	_, ok := c.resolveRouterModelOverride(t.Context(), []string{"anthropic/claude-opus-4"}, "anthropic/claude-sonnet-4")
	require.False(t, ok)
}

func TestResolveRouterModelOverride_NotInKnownCatalogFailsOpen(t *testing.T) {
	t.Parallel()

	// The pool names a model the openrouter provider's catalog doesn't
	// have (the catalog here is empty) — must fail open rather than build
	// a zero-value catwalk.Model.
	c := newRouterModelPoolTestCoordinator(t, "[]")
	_, ok := c.resolveRouterModelOverride(t.Context(), []string{"anthropic/claude-opus-4"}, "anthropic/claude-opus-4")
	require.False(t, ok)
}

func TestResolveRouterModelOverride_ValidPoolMemberResolves(t *testing.T) {
	t.Parallel()

	c := newRouterModelPoolTestCoordinator(t, oneOpusModel)
	model, ok := c.resolveRouterModelOverride(t.Context(), []string{"anthropic/claude-opus-4"}, "anthropic/claude-opus-4")
	require.True(t, ok)
	require.Equal(t, "anthropic/claude-opus-4", model.CatwalkCfg.ID)
	require.Equal(t, "anthropic/claude-opus-4", model.ModelCfg.Model)
	require.NotNil(t, model.Model, "the resolved Model must carry a real fantasy.LanguageModel client")
}

// TestResolveRouterModelOverride_NonOpenRouterProviderResolves proves the
// router pool is not OpenRouter-only: a pool member that lives on another
// configured provider resolves against that provider.
func TestResolveRouterModelOverride_NonOpenRouterProviderResolves(t *testing.T) {
	t.Parallel()

	c := newRouterModelPoolTestCoordinator(t, oneOpusModel)
	model, ok := c.resolveRouterModelOverride(t.Context(), []string{"mock-model"}, "mock-model")
	require.True(t, ok)
	require.Equal(t, "mock-model", model.CatwalkCfg.ID)
	require.Equal(t, "mock", model.ModelCfg.Provider,
		"the resolved model must be attributed to the provider that actually offers it")
	require.NotNil(t, model.Model)
}

const twoModelsJSON = `[
	{"id": "anthropic/claude-opus-4", "name": "Opus", "context_window": 200000, "default_max_tokens": 4096},
	{"id": "anthropic/claude-haiku-4", "name": "Haiku", "context_window": 200000, "default_max_tokens": 4096}
]`

// TestModelOverride_ConcurrentSessionsDoNotLeakBetweenEachOther proves that
// two concurrent Run calls on the same shared *sessionAgent, each routed to
// a different pool model via its own ModelOverride, each see their own
// chosen model in their own call and never the other's (and never the
// agent's shared, non-overridden model). It uses three independently
// tagged fake fantasy.LanguageModels — "shared" (the agent's normal
// model), "pool-a", and "pool-b" — and asserts on which ones actually
// received the outbound Stream call, the same observable proxy
// TestRun_ModelProviderClosureUsesOverride uses for a single call.
func TestModelOverride_ConcurrentSessionsDoNotLeakBetweenEachOther(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	shared := &taggedStreamModel{}
	poolA := &taggedStreamModel{}
	poolB := &taggedStreamModel{}

	// Two sessions sharing the same underlying *sessionAgent, exactly like
	// two sessions on the same named agent (e.g. both on "coder") do in
	// the real coordinator (c.agents[name] holds one shared instance).
	sa := testSessionAgent(env, shared, shared, "system prompt")
	concreteSA, ok := sa.(*sessionAgent)
	require.True(t, ok, "test relies on the concrete *sessionAgent type to read a.largeModel directly")

	sessionA, err := env.sessions.Create(t.Context(), "session a")
	require.NoError(t, err)
	sessionB, err := env.sessions.Create(t.Context(), "session b")
	require.NoError(t, err)

	// Seed a prior user text message on each session so Run's
	// title-generation path (which fires a detached goroutine against the
	// shared model) does not fire and race with, or contaminate, the
	// per-model call tracking below.
	for _, sessionID := range []string{sessionA.ID, sessionB.ID} {
		_, err = env.messages.Create(t.Context(), sessionID, message.CreateMessageParams{
			Role:  message.User,
			Parts: []message.ContentPart{message.TextContent{Text: "prior message"}},
		})
		require.NoError(t, err)
	}

	overrideA := &Model{
		Model:      poolA,
		CatwalkCfg: catwalk.Model{ContextWindow: 200000, DefaultMaxTokens: 10000},
	}
	overrideB := &Model{
		Model:      poolB,
		CatwalkCfg: catwalk.Model{ContextWindow: 200000, DefaultMaxTokens: 10000},
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, runErr := sa.Run(t.Context(), SessionAgentCall{
			SessionID: sessionA.ID, Prompt: "a", MaxOutputTokens: 100, ModelOverride: overrideA,
		})
		assert.NoError(t, runErr)
	}()
	go func() {
		defer wg.Done()
		_, runErr := sa.Run(t.Context(), SessionAgentCall{
			SessionID: sessionB.ID, Prompt: "b", MaxOutputTokens: 100, ModelOverride: overrideB,
		})
		assert.NoError(t, runErr)
	}()
	wg.Wait()

	require.True(t, poolA.called.Load(), "session A's call must have received the outbound Stream call on pool-a")
	require.True(t, poolB.called.Load(), "session B's call must have received the outbound Stream call on pool-b")
	require.False(t, shared.called.Load(),
		"neither call should ever touch the agent's shared, non-overridden model, since both supplied "+
			"their own ModelOverride")
	require.Equal(t, fantasy.LanguageModel(shared), concreteSA.largeModel.Get().Model,
		"a.largeModel must never be mutated by a per-call override, no matter how the two calls interleave")
}

// taggedStreamModel is a minimal fantasy.LanguageModel that completes a
// turn cleanly (a single text delta followed by FinishReasonStop, the
// same shape as finishStreamModel in dispatch_cancel_test.go) and records
// whether its Stream method was actually invoked. That is the observable
// proxy for "this is the model fantasy's retry wrapper called
// ModelProvider() to obtain and then issued the outbound request
// against" — see charm.land/fantasy@v0.45.1/agent.go's retry closure,
// which re-reads call.ModelProvider() on every attempt (including the
// first) and calls Stream on whatever it returns, not on the model
// fantasy.NewAgent was originally constructed with.
type taggedStreamModel struct {
	called atomic.Bool
}

func (m *taggedStreamModel) Provider() string { return "fake" }
func (m *taggedStreamModel) Model() string    { return "fake-model" }

func (m *taggedStreamModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	m.called.Store(true)
	return &fantasy.Response{FinishReason: fantasy.FinishReasonStop}, nil
}

func (m *taggedStreamModel) Stream(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
	m.called.Store(true)
	return func(yield func(fantasy.StreamPart) bool) {
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "1"}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "1", Delta: "ok"}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "1"}) {
			return
		}
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
	}, nil
}

func (m *taggedStreamModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *taggedStreamModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

// TestRun_ModelProviderClosureUsesOverride is the regression test for the
// bug the code review caught: Run's ModelProvider callback (passed to
// fantasy.AgentStreamCall) must return the local, possibly-overridden
// largeModel — not a.largeModel.Get() — because fantasy's retry wrapper
// calls ModelProvider() on every attempt, including the first, and issues
// the actual outbound Stream call against whatever it returns. Before the
// fix, this closure always re-read the shared a.largeModel field, so the
// override was used to build the fantasy.Agent and for local accounting
// but the real network call silently reverted to the agent's normally
// configured model. This test proves the opposite by using two
// independently-tagged fake models and asserting which one's Stream
// method actually ran.
func TestRun_ModelProviderClosureUsesOverride(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	shared := &taggedStreamModel{}
	override := &taggedStreamModel{}

	sa := testSessionAgent(env, shared, shared, "system prompt")

	session, err := env.sessions.Create(t.Context(), "override session")
	require.NoError(t, err)

	// Seed a prior user text message so Run's "generate a title from the
	// first real user prompt" path (which fires a detached goroutine that
	// calls the shared a.largeModel directly — a legitimate, unrelated use
	// of the shared field, out of scope for this task) does not fire and
	// race with, or contaminate, the shared/override call tracking below.
	_, err = env.messages.Create(t.Context(), session.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "prior message"}},
	})
	require.NoError(t, err)

	overrideModel := &Model{
		Model: override,
		CatwalkCfg: catwalk.Model{
			ContextWindow:    200000,
			DefaultMaxTokens: 10000,
		},
	}

	_, err = sa.Run(t.Context(), SessionAgentCall{
		SessionID:       session.ID,
		Prompt:          "hello",
		MaxOutputTokens: 100,
		ModelOverride:   overrideModel,
	})
	require.NoError(t, err)

	require.True(t, override.called.Load(),
		"the router's chosen override model must receive the actual outbound Stream call")
	require.False(t, shared.called.Load(),
		"the shared, non-overridden model must never be used for the outbound network call once "+
			"ModelOverride is set — this is exactly the bug where ModelProvider read a.largeModel.Get() "+
			"instead of the local override")
}

// authFailingStreamModel is a minimal fantasy.LanguageModel whose Stream
// and Generate methods always fail with an authentication error, while
// recording that they were called. It stands in for the stale, pre-refresh
// model in an OnAuthRefresh sequence: the outbound call reaches it, but its
// credentials are expired.
type authFailingStreamModel struct {
	called atomic.Bool
}

func (m *authFailingStreamModel) Provider() string { return "fake" }
func (m *authFailingStreamModel) Model() string    { return "fake-model-stale" }

func (m *authFailingStreamModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	m.called.Store(true)
	return nil, &fantasy.ProviderError{AuthError: true, Message: "credentials expired"}
}

func (m *authFailingStreamModel) Stream(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
	m.called.Store(true)
	return nil, &fantasy.ProviderError{AuthError: true, Message: "credentials expired"}
}

func (m *authFailingStreamModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *authFailingStreamModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

// TestRun_ModelProviderClosureRereadsSharedModelAfterAuthRefresh is the
// regression test for the bug fixed alongside
// TestRun_ModelProviderClosureUsesOverride: when the router did NOT supply
// a ModelOverride for this call, Run's ModelProvider closure must re-read
// a.largeModel fresh on every call fantasy's retry wrapper makes to it —
// not the local largeModel snapshot taken at Run's entry — so that a retry
// following OnAuthRefresh's mid-call a.largeModel.Set(fresh) (mirroring
// coordinator.go's real auth-refresh callback) actually uses the refreshed
// client instead of replaying the stale, pre-refresh one and failing again.
func TestRun_ModelProviderClosureRereadsSharedModelAfterAuthRefresh(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	stale := &authFailingStreamModel{}
	fresh := &taggedStreamModel{}

	sa := testSessionAgent(env, stale, stale, "system prompt")
	concreteSA, ok := sa.(*sessionAgent)
	require.True(t, ok, "test relies on the concrete *sessionAgent type to swap a.largeModel directly")

	session, err := env.sessions.Create(t.Context(), "auth refresh session")
	require.NoError(t, err)

	// Seed a prior user text message so Run's title-generation path (which
	// fires a detached goroutine against the shared model) does not race
	// with the call tracking below.
	_, err = env.messages.Create(t.Context(), session.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "prior message"}},
	})
	require.NoError(t, err)

	freshModel := Model{
		Model: fresh,
		CatwalkCfg: catwalk.Model{
			ContextWindow:    200000,
			DefaultMaxTokens: 10000,
		},
	}

	onAuthRefresh := func(context.Context, *fantasy.ProviderError) error {
		// Mirrors coordinator.go's real auth-refresh callback: swap the
		// shared field in place after a successful credential refresh,
		// never mutate a call-local snapshot.
		concreteSA.largeModel.Set(freshModel)
		return nil
	}

	_, err = sa.Run(t.Context(), SessionAgentCall{
		SessionID:       session.ID,
		Prompt:          "hello",
		MaxOutputTokens: 100,
		OnAuthRefresh:   onAuthRefresh,
	})
	require.NoError(t, err)

	require.True(t, stale.called.Load(),
		"the first attempt must go out on the pre-refresh model")
	require.True(t, fresh.called.Load(),
		"the retry following OnAuthRefresh must use the refreshed a.largeModel, not the stale "+
			"snapshot captured at Run's entry — this is exactly the bug where ModelProvider always "+
			"returned the local largeModel variable")
}

// TestSummarize_UsesModelOverrideWhenProvided proves that Summarize builds
// its fantasy.NewAgent (and its own ModelProvider closure) from the
// modelOverride argument, when non-nil, rather than always reading
// a.largeModel. Before the fix, Run's auto-summarize trigger passed
// call.ProviderOptions/call.OnAuthRefresh (built for the router's override
// model) into Summarize, but Summarize itself always built the actual
// summarize call against the agent's normal shared model — a mismatch that
// could trip the summarize threshold on token counts the normal model
// can't hold.
func TestSummarize_UsesModelOverrideWhenProvided(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	shared := &taggedStreamModel{}
	override := &taggedStreamModel{}

	sa := testSessionAgent(env, shared, shared, "system prompt")
	concreteSA, ok := sa.(*sessionAgent)
	require.True(t, ok, "test relies on the concrete *sessionAgent type to call Summarize directly")

	session, err := env.sessions.Create(t.Context(), "summarize override session")
	require.NoError(t, err)

	_, err = env.messages.Create(t.Context(), session.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "prior message"}},
	})
	require.NoError(t, err)

	overrideModel := &Model{
		Model: override,
		CatwalkCfg: catwalk.Model{
			ContextWindow:    200000,
			DefaultMaxTokens: 10000,
		},
	}

	err = concreteSA.Summarize(t.Context(), session.ID, fantasy.ProviderOptions{}, nil, overrideModel)
	require.NoError(t, err)

	require.True(t, override.called.Load(),
		"Summarize must use the passed-in modelOverride to build the summarize call")
	require.False(t, shared.called.Load(),
		"Summarize must not fall back to a.largeModel once a modelOverride is provided — this is "+
			"exactly the bug where Summarize always read a.largeModel.Get() regardless of the "+
			"override the caller (Run's auto-summarize trigger) resolved for this call")
}
