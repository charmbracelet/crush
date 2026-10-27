package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/catwalk/v3/pkg/catwalk"
	"charm.land/fantasy"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

// overrideTestConfig is a hermetic config with two providers: mock carries a
// plain model and a reasoning model, other carries one model of its own.
const overrideTestConfig = `{
  "options": {"disable_default_providers": true, "disable_provider_auto_update": true},
  "providers": {
    "mock": {"id": "mock", "name": "Mock", "type": "openai",
      "base_url": "http://127.0.0.1:9/v1", "api_key": "test-key",
      "models": [
        {"id": "mock-model", "name": "Mock", "context_window": 8192, "default_max_tokens": 128},
        {"id": "mock-reasoner", "name": "Mock Reasoner", "context_window": 8192, "default_max_tokens": 128,
         "can_reason": true, "reasoning_levels": ["low", "high"], "default_reasoning_effort": "low"}
      ]},
    "other": {"id": "other", "name": "Other", "type": "openai",
      "base_url": "http://127.0.0.1:9/v1", "api_key": "test-key",
      "models": [{"id": "other-model", "name": "Other", "context_window": 8192, "default_max_tokens": 128}]}
  },
  "models": {"large": {"provider": "mock", "model": "mock-reasoner"},
             "small": {"provider": "mock", "model": "mock-model"}}
}`

// newOverrideTestCoordinator returns a coordinator wired the way the
// readiness tests wire one, against overrideTestConfig, with its shared
// models already built. The parent model is the large one.
func newOverrideTestCoordinator(t *testing.T) (*coordinator, fakeEnv) {
	t.Helper()
	env := testEnv(t)
	require.NoError(t, os.WriteFile(filepath.Join(env.workingDir, "crush.json"), []byte(overrideTestConfig), 0o644))

	cfg, err := config.Init(t.Context(), env.workingDir, "", false)
	require.NoError(t, err)
	// Local model servers found on the machine running the tests would
	// otherwise join the config; keep only the providers declared above.
	var discovered []string
	for id := range cfg.Config().Providers.Seq2() {
		if id != "mock" && id != "other" {
			discovered = append(discovered, id)
		}
	}
	for _, id := range discovered {
		cfg.Config().Providers.Del(id)
	}
	cfg.SetupAgents()

	c := &coordinator{
		cfg:         cfg,
		sessions:    env.sessions,
		messages:    env.messages,
		permissions: env.permissions,
		history:     env.history,
		filetracker: *env.filetracker,
		dispatched:  csync.NewMap[string, *dispatchedAgent](),
	}
	require.NoError(t, c.rebuildModels(context.Background()))
	return c, env
}

func TestResolveModelOverride(t *testing.T) {
	t.Parallel()
	c, _ := newOverrideTestCoordinator(t)
	parent := c.mainModels().Large

	tests := []struct {
		name    string
		params  AgentDispatchParams
		parent  Model
		wantOK  bool
		wantSel config.SelectedModel
		// wantDynamic expects the sub-agent default model's selection with
		// the override applied, rather than a fixed selection.
		wantDynamic bool
		refusal     string
	}{
		{
			name:   "no override requested",
			params: AgentDispatchParams{},
			wantOK: false,
		},
		{
			name:    "provider without model is refused",
			params:  AgentDispatchParams{Provider: "mock"},
			refusal: "provider is given without model",
		},
		{
			name:    "model only inherits the parent provider",
			params:  AgentDispatchParams{Model: "mock-model"},
			parent:  parent,
			wantOK:  true,
			wantSel: config.SelectedModel{Provider: "mock", Model: "mock-model"},
		},
		{
			name:    "model only with no parent model is refused",
			params:  AgentDispatchParams{Model: "mock-model"},
			refusal: "no model to take a provider from",
		},
		{
			name:    "model and provider are explicit",
			params:  AgentDispatchParams{Model: "other-model", Provider: "other"},
			wantOK:  true,
			wantSel: config.SelectedModel{Provider: "other", Model: "other-model"},
		},
		{
			name:    "unknown provider is refused",
			params:  AgentDispatchParams{Model: "m", Provider: "nope"},
			refusal: `provider "nope" is not available; use one of: mock, other`,
		},
		{
			name:    "unknown model is refused",
			params:  AgentDispatchParams{Model: "gone-model", Provider: "mock"},
			refusal: `model "gone-model" is not available from provider "mock"`,
		},
		{
			name:    "unsupported reasoning level is refused",
			params:  AgentDispatchParams{Model: "mock-reasoner", ReasoningLevel: "medium"},
			parent:  parent,
			refusal: `does not support reasoning effort "medium"`,
		},
		{
			name:    "supported reasoning level is applied",
			params:  AgentDispatchParams{Model: "mock-reasoner", ReasoningLevel: "high"},
			parent:  parent,
			wantOK:  true,
			wantSel: config.SelectedModel{Provider: "mock", Model: "mock-reasoner", ReasoningEffort: "high"},
		},
		{
			name:    "think on a model that cannot reason is refused",
			params:  AgentDispatchParams{Model: "mock-model", Think: true},
			parent:  parent,
			refusal: `model "mock-model" does not support thinking`,
		},
		{
			name:    "think on a reasoning model is applied",
			params:  AgentDispatchParams{Model: "mock-reasoner", Think: true},
			parent:  parent,
			wantOK:  true,
			wantSel: config.SelectedModel{Provider: "mock", Model: "mock-reasoner", Think: true},
		},
		{
			name:        "reasoning settings alone ride the sub-agent default model",
			params:      AgentDispatchParams{Think: true},
			wantOK:      true,
			wantDynamic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sel, ok, refusal := c.resolveModelOverride(tt.params, tt.parent)
			if tt.refusal != "" {
				require.NotEmpty(t, refusal)
				require.Contains(t, refusal, tt.refusal)
				return
			}
			require.Empty(t, refusal)
			require.Equal(t, tt.wantOK, ok)
			if !tt.wantOK {
				return
			}
			if tt.wantDynamic {
				// The sub-default case keeps the default model's other settings.
				want := c.subAgentModels().Large.ModelCfg
				want.Think = true
				require.Equal(t, want, sel)
				return
			}
			require.Equal(t, tt.wantSel, sel)
		})
	}
}

// Reasoning settings alone ride the sub-agent's default model, so they are
// refused when that model cannot reason.
func TestResolveModelOverrideRidesNonReasoningSubDefault(t *testing.T) {
	t.Parallel()
	c, _ := newOverrideTestCoordinator(t)

	small := c.subAgentModels().Small
	c.models.Set(coordinatorModels{large: small, subLarge: small, small: small})

	_, ok, refusal := c.resolveModelOverride(AgentDispatchParams{Think: true}, Model{})
	require.True(t, ok)
	require.Contains(t, refusal, "does not support thinking")
}

func TestOverrideDispatchAgent(t *testing.T) {
	t.Parallel()
	c, _ := newOverrideTestCoordinator(t)
	ctx := t.Context()
	parent := c.mainModels().Large

	agents, err := c.taskAgents(ctx, false)
	require.NoError(t, err)
	require.Len(t, agents, 2, "the read and write variants are built")

	agent, sel, ok, refusal := c.overrideDispatchAgent(ctx, accessRead, AgentDispatchParams{
		Model:    "other-model",
		Provider: "other",
	}, parent)
	require.Empty(t, refusal)
	require.True(t, ok)
	require.Equal(t, config.SelectedModel{Provider: "other", Model: "other-model"}, sel)
	require.NotNil(t, agent)
	require.Equal(t, "other-model", agent.Model().ModelCfg.Model)
	require.Equal(t, "other", agent.Model().ModelCfg.Provider)

	// The override model is shared, so a second dispatch on the same
	// selection reuses the build instead of rebuilding the provider.
	_, _, _, refusal = c.overrideDispatchAgent(ctx, accessRead, AgentDispatchParams{
		Model:    "other-model",
		Provider: "other",
	}, parent)
	require.Empty(t, refusal)
	require.Len(t, c.overrideModels, 1)

	_, _, _, refusal = c.overrideDispatchAgent(ctx, accessRead, AgentDispatchParams{
		Provider: "other",
	}, parent)
	require.Contains(t, refusal, "provider is given without model")
}

// A model refresh swaps the build an overridden sub-agent serves, so a
// credential or config change reaches it on its next turn like it reaches
// the shared agents.
func TestOverrideModelRefreshedOnRebuild(t *testing.T) {
	t.Parallel()
	c, _ := newOverrideTestCoordinator(t)
	ctx := t.Context()

	entry := c.overrideModelEntry(config.SelectedModel{Provider: "other", Model: "other-model"})
	require.NoError(t, entry.build(ctx, c))
	first := entry.current()
	require.Equal(t, "other-model", first.ModelCfg.Model)

	require.NoError(t, c.rebuildModels(ctx))
	second := entry.current()
	require.Equal(t, "other-model", second.ModelCfg.Model)
	require.NotEqual(t, fmt.Sprintf("%p", first.Model), fmt.Sprintf("%p", second.Model),
		"a rebuild must produce a fresh build, not the pre-rebuild one")
}

func dispatchOverrideCall(t *testing.T, id, label, access, model, provider string) message.ToolCall {
	t.Helper()
	input, err := json.Marshal(AgentDispatchParams{
		Prompt:   "do " + label,
		Label:    label,
		Access:   access,
		Model:    model,
		Provider: provider,
	})
	require.NoError(t, err)
	return message.ToolCall{ID: id, Name: AgentDispatchToolName, Input: string(input), Finished: true}
}

func TestDispatchHistoryRecordsModelOverride(t *testing.T) {
	t.Parallel()

	// The c1 dispatch asked for a model only; its result carries the
	// provider that was resolved for it.
	startedMeta, err := json.Marshal(AgentDispatchResponseMetadata{
		Label:    "a",
		Model:    "other-model",
		Provider: "mock",
	})
	require.NoError(t, err)

	msgs := []message.Message{
		{ID: "m1", Role: message.Assistant, Parts: []message.ContentPart{
			dispatchOverrideCall(t, "c1", "a", "", "other-model", ""),
			dispatchCall(t, "c2", "b", ""),
		}},
		{ID: "t1", Role: message.Tool, Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "c1", Name: AgentDispatchToolName, Content: "started", Metadata: string(startedMeta)},
			message.ToolResult{ToolCallID: "c2", Name: AgentDispatchToolName, Content: "started"},
		}},
	}

	records := dispatchHistory(msgs)
	a := records.byLabel["a"]
	require.Equal(t, "other-model", a.Model)
	require.Equal(t, "mock", a.Provider, "the resolved provider is recorded, not re-derived")
	b := records.byLabel["b"]
	require.Empty(t, b.Model)
	require.Empty(t, b.Provider)
	require.False(t, b.Think)
}

func TestDispatchToolRunsOverrideModel(t *testing.T) {
	t.Parallel()
	c, env := newOverrideTestCoordinator(t)
	ctx := t.Context()

	// Give the coordinator a parent agent so a model-only override has a
	// provider to inherit.
	agents, err := c.taskAgents(ctx, false)
	require.NoError(t, err)
	c.mainAgent = agents[accessRead]

	tool, err := c.agentDispatchTool(ctx, config.AgentCoder)
	require.NoError(t, err)

	parent, err := env.sessions.Create(ctx, "parent")
	require.NoError(t, err)

	newCall := func(id, input string) (context.Context, fantasy.ToolCall) {
		assistant, err := env.messages.Create(ctx, parent.ID, message.CreateMessageParams{
			Role:  message.Assistant,
			Parts: []message.ContentPart{message.TextContent{Text: "dispatching"}},
		})
		require.NoError(t, err)
		callCtx := context.WithValue(ctx, tools.SessionIDContextKey, parent.ID)
		callCtx = context.WithValue(callCtx, tools.MessageIDContextKey, assistant.ID)
		return callCtx, fantasy.ToolCall{ID: id, Name: AgentDispatchToolName, Input: input}
	}

	// A refused override leaves the label free for a retry.
	callCtx, call := newCall("c1", `{"prompt":"do","label":"ov","provider":"other"}`)
	resp, err := tool.Run(callCtx, call)
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "provider is given without model")

	callCtx, call = newCall("c2", `{"prompt":"do","label":"ov","model":"gone-model"}`)
	resp, err = tool.Run(callCtx, call)
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, `model "gone-model" is not available from provider "mock"`)

	callCtx, call = newCall("c3", `{"prompt":"do","label":"ov","model":"mock-model"}`)
	resp, err = tool.Run(callCtx, call)
	require.NoError(t, err)
	require.False(t, resp.IsError, resp.Content)

	// The result records the resolved override, so a restart rehydrates on
	// the provider the dispatch actually used.
	var meta AgentDispatchResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
	require.Equal(t, "ov", meta.Label)
	require.Equal(t, "mock-model", meta.Model)
	require.Equal(t, "mock", meta.Provider, "a model-only dispatch records the inherited provider")
	require.False(t, meta.ReasoningOnly, "a dispatch that named a model is not reasoning-only")

	callCtx, call = newCall("c4", `{"prompt":"do","label":"thunk","think":true}`)
	resp, err = tool.Run(callCtx, call)
	require.NoError(t, err)
	require.False(t, resp.IsError, resp.Content)
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
	require.Equal(t, "thunk", meta.Label)
	require.True(t, meta.ReasoningOnly, "a dispatch that named no model is reasoning-only")
	entry, ok := c.dispatched.Get(dispatchKey(parent.ID, "thunk"))
	require.True(t, ok)
	require.True(t, entry.agent.Model().ModelCfg.Think)
	entry.stop("test done")
	require.True(t, entry.awaitSettled(10*time.Second))

	entry, ok = c.dispatched.Get(dispatchKey(parent.ID, "ov"))
	require.True(t, ok)
	require.Equal(t, "mock-model", entry.agent.Model().ModelCfg.Model)
	require.Equal(t, "mock", entry.agent.Model().ModelCfg.Provider)

	entry.stop("test done")
	require.True(t, entry.awaitSettled(10*time.Second))
}

func TestRehydrateDispatchRestoresModelOverride(t *testing.T) {
	t.Parallel()
	c, env := newOverrideTestCoordinator(t)
	ctx := t.Context()

	parent, err := env.sessions.Create(ctx, "parent")
	require.NoError(t, err)

	// c3 asked for a model only; its recorded result carries the provider
	// the dispatch resolved, so rehydration needs no current parent model.
	monoMeta, err := json.Marshal(AgentDispatchResponseMetadata{
		Label:    "mono",
		Model:    "mock-model",
		Provider: "mock",
	})
	require.NoError(t, err)

	assistant, err := env.messages.Create(ctx, parent.ID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			dispatchOverrideCall(t, "c1", "ov", "", "other-model", "other"),
			dispatchOverrideCall(t, "c2", "gone", "", "gone-model", "other"),
			dispatchOverrideCall(t, "c3", "mono", "", "mock-model", ""),
		},
	})
	require.NoError(t, err)
	_, err = env.messages.Create(ctx, parent.ID, message.CreateMessageParams{
		Role: message.Tool,
		Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "c1", Name: AgentDispatchToolName, Content: "started"},
			message.ToolResult{ToolCallID: "c3", Name: AgentDispatchToolName, Content: "started", Metadata: string(monoMeta)},
		},
	})
	require.NoError(t, err)

	// c4 "crash" was a model-only dispatch whose result never landed; the
	// child transcript records what it ran on. c5 "orphan" never got a
	// result or a first turn, so only the parent the session has now can
	// supply the provider. c6 "thunk" gave reasoning settings only, so it
	// rides the default selection rather than an explicit model.
	thunkInput, err := json.Marshal(AgentDispatchParams{Prompt: "do thunk", Label: "thunk", Think: true})
	require.NoError(t, err)
	thunkMeta, err := json.Marshal(AgentDispatchResponseMetadata{
		Label:         "thunk",
		Model:         "mock-reasoner",
		Provider:      "mock",
		Think:         true,
		ReasoningOnly: true,
	})
	require.NoError(t, err)

	firstAssistant := assistant
	assistant, err = env.messages.Create(ctx, parent.ID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			dispatchOverrideCall(t, "c4", "crash", "", "mock-model", ""),
			dispatchOverrideCall(t, "c5", "orphan", "", "other-model", ""),
			message.ToolCall{ID: "c6", Name: AgentDispatchToolName, Input: string(thunkInput), Finished: true},
		},
	})
	require.NoError(t, err)

	// The parent the session has now runs on "other", so a parent fallback
	// would resolve "crash" onto the wrong provider and fail.
	c.mainAgent = &mockSessionAgent{model: Model{
		Model:    c.mainModels().Large.Model,
		ModelCfg: config.SelectedModel{Provider: "other", Model: "other-model"},
	}}

	for _, callID := range []string{"c1", "c2", "c3"} {
		childID := env.sessions.CreateAgentToolSessionID(firstAssistant.ID, callID)
		_, err := env.sessions.CreateTaskSession(ctx, childID, parent.ID, "Sub-agent")
		require.NoError(t, err)
	}
	for _, callID := range []string{"c4", "c5", "c6"} {
		childID := env.sessions.CreateAgentToolSessionID(assistant.ID, callID)
		_, err := env.sessions.CreateTaskSession(ctx, childID, parent.ID, "Sub-agent")
		require.NoError(t, err)
	}
	_, err = env.messages.Create(ctx, parent.ID, message.CreateMessageParams{
		Role: message.Tool,
		Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "c6", Name: AgentDispatchToolName, Content: "started", Metadata: string(thunkMeta)},
		},
	})
	require.NoError(t, err)

	crashChild := env.sessions.CreateAgentToolSessionID(assistant.ID, "c4")
	_, err = env.messages.Create(ctx, crashChild, message.CreateMessageParams{
		Role:     message.Assistant,
		Parts:    []message.ContentPart{message.TextContent{Text: "working"}},
		Model:    "mock-model",
		Provider: "mock",
	})
	require.NoError(t, err)

	entry, err := c.rehydrateDispatch(ctx, parent.ID, "ov")
	require.NoError(t, err)
	require.Equal(t, "other-model", entry.agent.Model().ModelCfg.Model)
	require.Equal(t, "other", entry.agent.Model().ModelCfg.Provider)

	// The metadata records the resolved provider, so the parent running on
	// "other" by now does not matter.
	entry, err = c.rehydrateDispatch(ctx, parent.ID, "mono")
	require.NoError(t, err)
	require.Equal(t, "mock-model", entry.agent.Model().ModelCfg.Model)
	require.Equal(t, "mock", entry.agent.Model().ModelCfg.Provider)

	// A dispatch whose result never landed recovers the model and provider
	// its child session actually ran on, not the provider the parent has
	// now.
	entry, err = c.rehydrateDispatch(ctx, parent.ID, "crash")
	require.NoError(t, err)
	require.Equal(t, "mock-model", entry.agent.Model().ModelCfg.Model)
	require.Equal(t, "mock", entry.agent.Model().ModelCfg.Provider)

	// A dispatch that never ran a turn falls back to the parent's provider.
	entry, err = c.rehydrateDispatch(ctx, parent.ID, "orphan")
	require.NoError(t, err)
	require.Equal(t, "other-model", entry.agent.Model().ModelCfg.Model)
	require.Equal(t, "other", entry.agent.Model().ModelCfg.Provider)

	// A reasoning-only override rides the default selection, keeping its
	// configured settings, instead of rebuilding a bare selection from the
	// recorded model.
	entry, err = c.rehydrateDispatch(ctx, parent.ID, "thunk")
	require.NoError(t, err)
	want := c.subAgentModels().Large.ModelCfg
	want.Think = true
	require.Equal(t, want, entry.agent.Model().ModelCfg, "a reasoning-only override keeps the default selection's settings")

	// A dispatch whose model no longer resolves is not silently resumed on
	// the session default.
	_, err = c.rehydrateDispatch(ctx, parent.ID, "gone")
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot be resumed")
}

// listFailingMessages fails every session listing except the parent's, to
// simulate a transient read failure while a child transcript is consulted.
type listFailingMessages struct {
	message.Service
	except string
}

func (f listFailingMessages) List(ctx context.Context, sessionID string) ([]message.Message, error) {
	if sessionID != f.except {
		return nil, errors.New("transient read failure")
	}
	return f.Service.List(ctx, sessionID)
}

// A failing transcript read must not be mistaken for a child that never
// ran, which would let rehydration fall back to the wrong provider.
func TestRehydrateDispatchPropagatesTranscriptReadError(t *testing.T) {
	t.Parallel()
	c, env := newOverrideTestCoordinator(t)
	ctx := t.Context()

	parent, err := env.sessions.Create(ctx, "parent")
	require.NoError(t, err)

	assistant, err := env.messages.Create(ctx, parent.ID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			dispatchOverrideCall(t, "c1", "ov", "", "other-model", ""),
		},
	})
	require.NoError(t, err)
	childID := env.sessions.CreateAgentToolSessionID(assistant.ID, "c1")
	_, err = env.sessions.CreateTaskSession(ctx, childID, parent.ID, "Sub-agent")
	require.NoError(t, err)

	c.messages = listFailingMessages{Service: env.messages, except: parent.ID}
	_, err = c.rehydrateDispatch(ctx, parent.ID, "ov")
	require.Error(t, err)
	require.Contains(t, err.Error(), "transient read failure")
}

// A disabled provider is refused even when the model it names exists, and
// it drops out of the catalog the tool description lists.
func TestModelOverrideSkipsDisabledProvider(t *testing.T) {
	t.Parallel()
	c, _ := newOverrideTestCoordinator(t)
	cfg := c.cfg.Config()

	other, ok := cfg.Providers.Get("other")
	require.True(t, ok)
	other.Disable = true
	cfg.Providers.Set("other", other)

	_, _, refusal := c.resolveModelOverride(AgentDispatchParams{Model: "other-model", Provider: "other"}, Model{})
	require.Equal(t, `provider "other" is not available; use one of: mock`, refusal)

	catalog := dispatchModelCatalog(cfg, Model{})
	require.NotContains(t, catalog, "other")
	require.Contains(t, catalog, "- `mock`: mock-model, mock-reasoner")
}

// The catalog names the parent's model, lists every usable provider in a
// stable order, and is what an unknown model's refusal points back to.
func TestDispatchModelCatalog(t *testing.T) {
	t.Parallel()
	c, _ := newOverrideTestCoordinator(t)
	cfg := c.cfg.Config()

	catalog := dispatchModelCatalog(cfg, c.mainModels().Large)
	require.Equal(t, "\n\nModels available to `model`, by `provider` (you are running on `mock-reasoner` from `mock`):\n"+
		"\n- `mock`: mock-model, mock-reasoner"+
		"\n- `other`: other-model", catalog)

	_, _, refusal := c.resolveModelOverride(AgentDispatchParams{Model: "gone-model", Provider: "mock"}, Model{})
	require.Equal(t, `model "gone-model" is not available from provider "mock"; use one of: mock-model, mock-reasoner`, refusal)
}

// Nothing usable means no catalog at all, rather than an empty heading.
func TestDispatchModelCatalogEmpty(t *testing.T) {
	t.Parallel()
	c, _ := newOverrideTestCoordinator(t)
	cfg := c.cfg.Config()
	for _, id := range []string{"mock", "other"} {
		p, ok := cfg.Providers.Get(id)
		require.True(t, ok)
		p.Disable = true
		cfg.Providers.Set(id, p)
	}
	require.Empty(t, dispatchModelCatalog(cfg, Model{}))
}

func TestModelIDListCapsLongCatalogs(t *testing.T) {
	t.Parallel()
	models := make([]catwalk.Model, catalogModelsPerProvider+3)
	for i := range models {
		models[i].ID = fmt.Sprintf("m%d", i)
	}
	list := modelIDList(models)
	require.Contains(t, list, "m0, m1")
	require.NotContains(t, list, fmt.Sprintf("m%d", catalogModelsPerProvider))
	require.True(t, strings.HasSuffix(list, ", and 3 more"), list)
}
