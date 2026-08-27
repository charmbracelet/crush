package agent

import (
	"context"
	"sync"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// poisonModel fails the test if anything streams it. It stands in for
// the shared agent's model, so a turn that reads workspace state instead
// of its own model pair is caught rather than silently passing.
type poisonModel struct {
	finishStreamModel
	t *testing.T
}

func (m *poisonModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	m.t.Error("the workspace's model must not stream a run that pinned its own")
	return m.finishStreamModel.Stream(ctx, call)
}

func (m *poisonModel) Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
	m.t.Error("the workspace's model must not serve a run that pinned its own")
	return m.finishStreamModel.Generate(ctx, call)
}

// testModel wraps a fake language model with the catwalk settings the
// agent needs to build a call.
func testModel(m fantasy.LanguageModel, provider, id string) Model {
	return Model{
		Model: m,
		CatwalkCfg: catwalk.Model{
			ContextWindow:    200000,
			DefaultMaxTokens: 10000,
		},
		ModelCfg: config.SelectedModel{Provider: provider, Model: id},
	}
}

// pinnedModels returns a run's own model pair for a fake large model.
// The small slot gets its own fake because title generation streams it
// concurrently.
func pinnedModels(large fantasy.LanguageModel, provider, id string) *runModels {
	selection := modelSelection{
		Large: config.SelectedModel{Provider: provider, Model: id},
		Small: config.SelectedModel{Provider: provider, Model: id + "-small"},
	}
	return newRunModels(selection, false,
		testModel(large, provider, id),
		testModel(&finishStreamModel{text: "a title"}, provider, id+"-small"),
	)
}

// TestRun_ConcurrentRunsUseTheirOwnModel is the regression test for the
// workspace-wide model: `crush run -m` used to rewrite the project's
// selected model, so two runs in one directory could not use different
// models and whichever wrote last decided for both.
//
// Each run now carries its own pair. The agent's own model is a poison
// model, so reading workspace state instead of the run's fails the test.
func TestRun_ConcurrentRunsUseTheirOwnModel(t *testing.T) {
	t.Parallel()
	env := testEnv(t)

	poison := &poisonModel{t: t}
	sa := testSessionAgent(env, poison, poison, "system").(*sessionAgent)

	first := &scriptedStreamModel{steps: []scriptedStep{
		{text: "from the first model", finishReason: fantasy.FinishReasonStop},
	}}
	second := &scriptedStreamModel{steps: []scriptedStep{
		{text: "from the second model", finishReason: fantasy.FinishReasonStop},
	}}

	sessions := make([]string, 2)
	for i := range sessions {
		sess, err := env.sessions.Create(t.Context(), "session")
		require.NoError(t, err)
		sessions[i] = sess.ID
	}

	var wg sync.WaitGroup
	errs := make([]error, 2)
	models := []*runModels{
		pinnedModels(first, "first-provider", "first-model"),
		pinnedModels(second, "second-provider", "second-model"),
	}
	for i := range models {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = sa.Run(t.Context(), SessionAgentCall{
				SessionID: sessions[i],
				Prompt:    "hello",
				Models:    models[i],
			})
		}()
	}
	wg.Wait()

	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	assert.Equal(t, int64(1), first.calls.Load(),
		"the first run must have streamed exactly its own model")
	assert.Equal(t, int64(1), second.calls.Load(),
		"the second run must have streamed exactly its own model")

	// The recorded provider/model is what every client renders, so it
	// has to be the run's, not the workspace's.
	for i, want := range []string{"first-model", "second-model"} {
		msgs, err := env.messages.List(t.Context(), sessions[i])
		require.NoError(t, err)
		var found bool
		for _, m := range msgs {
			if m.Role == message.Assistant && m.Model == want {
				found = true
			}
		}
		assert.True(t, found, "session %d must record %q as its model", i, want)
	}
}

// TestRun_ModelChangeDuringTurnDoesNotAffectIt pins the mid-turn flip.
// fantasy calls AgentStreamCall.ModelProvider on every step and every
// retry attempt; that used to read the shared agent's model, so a model
// change landing between two steps of a live turn moved the rest of the
// turn onto a different model.
func TestRun_ModelChangeDuringTurnDoesNotAffectIt(t *testing.T) {
	t.Parallel()
	env := testEnv(t)

	pinned := &scriptedStreamModel{steps: []scriptedStep{
		{
			toolCallID:   "call-1",
			toolName:     "echo",
			toolInput:    `{"value":"hi"}`,
			finishReason: fantasy.FinishReasonToolCalls,
		},
		{text: "all done", finishReason: fantasy.FinishReasonStop},
	}}

	// The tool runs between the turn's two steps, which is exactly when
	// a workspace model change would land.
	var sa *sessionAgent
	poison := &poisonModel{t: t}
	swap := fantasy.NewAgentTool(
		"echo",
		"Echoes its input back.",
		func(_ context.Context, params echoToolParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			sa.SetModels(
				testModel(poison, "other-provider", "other-model"),
				testModel(poison, "other-provider", "other-model-small"),
			)
			return fantasy.NewTextResponse("echoed: " + params.Value), nil
		},
	)

	sa = testSessionAgent(env, poison, poison, "system", swap).(*sessionAgent)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	_, err = sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "echo hi",
		Models:    pinnedModels(pinned, "pinned-provider", "pinned-model"),
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), pinned.calls.Load(),
		"both steps of the turn must have streamed the model the run pinned")
}

// TestRun_ToolCallsCarryTheRunModels pins the chain a sub-agent depends
// on: the run's models reach its tool calls' context, which is where
// coordinator.runSelection reads them so a delegated turn inherits the
// model of the run that spawned it.
func TestRun_ToolCallsCarryTheRunModels(t *testing.T) {
	t.Parallel()
	env := testEnv(t)

	model := &scriptedStreamModel{steps: []scriptedStep{
		{
			toolCallID:   "call-1",
			toolName:     "echo",
			toolInput:    `{"value":"hi"}`,
			finishReason: fantasy.FinishReasonToolCalls,
		},
		{text: "done", finishReason: fantasy.FinishReasonStop},
	}}

	var seen *runModels
	inspect := fantasy.NewAgentTool(
		"echo",
		"Echoes its input back.",
		func(ctx context.Context, params echoToolParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			seen = runModelsFromContext(ctx)
			return fantasy.NewTextResponse("echoed: " + params.Value), nil
		},
	)

	poison := &poisonModel{t: t}
	sa := testSessionAgent(env, poison, poison, "system", inspect).(*sessionAgent)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	pinned := pinnedModels(model, "pinned-provider", "pinned-model")
	_, err = sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "echo hi",
		Models:    pinned,
	})
	require.NoError(t, err)

	require.NotNil(t, seen, "a tool call must be able to read the run's models")
	assert.Same(t, pinned, seen)
}

func TestRunSelection(t *testing.T) {
	t.Parallel()

	configured := modelSelection{
		Large: config.SelectedModel{Provider: "cfg", Model: "cfg-large"},
		Small: config.SelectedModel{Provider: "cfg", Model: "cfg-small"},
	}

	newCoordinator := func(t *testing.T) *coordinator {
		t.Helper()
		env := testEnv(t)
		cfg, err := config.Init(env.workingDir, "", false)
		require.NoError(t, err)
		cfg.OverridePreferredModel(config.SelectedModelTypeLarge, configured.Large)
		cfg.OverridePreferredModel(config.SelectedModelTypeSmall, configured.Small)
		return &coordinator{cfg: cfg, sessions: env.sessions, messages: env.messages}
	}

	t.Run("falls back to the workspace config", func(t *testing.T) {
		t.Parallel()
		c := newCoordinator(t)
		selection, err := c.runSelection(t.Context())
		require.NoError(t, err)
		assert.Equal(t, configured, selection)
	})

	t.Run("prefers what the run asked for, slot by slot", func(t *testing.T) {
		t.Parallel()
		c := newCoordinator(t)
		asked := config.SelectedModel{Provider: "asked", Model: "asked-large"}
		ctx := WithRequestedModels(t.Context(), &asked, nil)

		selection, err := c.runSelection(ctx)
		require.NoError(t, err)
		assert.Equal(t, asked, selection.Large)
		assert.Equal(t, configured.Small, selection.Small,
			"an unspecified slot must keep the workspace's model")
	})

	t.Run("inherits the spawning run's selection", func(t *testing.T) {
		t.Parallel()
		c := newCoordinator(t)
		// A sub-agent must run on the model of the run that spawned it,
		// even if the workspace's model changed since the run started.
		inherited := modelSelection{
			Large: config.SelectedModel{Provider: "run", Model: "run-large"},
			Small: config.SelectedModel{Provider: "run", Model: "run-small"},
		}
		ctx := withRunModels(t.Context(), newRunModels(inherited, false, Model{}, Model{}))
		ctx = WithRequestedModels(ctx, &configured.Large, &configured.Small)

		selection, err := c.runSelection(ctx)
		require.NoError(t, err)
		assert.Equal(t, inherited, selection)
	})
}
