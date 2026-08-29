package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// capturingModel records the messages of every stream call so a test can
// assert what actually reached the provider.
type capturingModel struct {
	finishStreamModel
	mu    sync.Mutex
	calls []fantasy.Prompt
}

func (m *capturingModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	m.mu.Lock()
	m.calls = append(m.calls, call.Prompt)
	m.mu.Unlock()
	return m.finishStreamModel.Stream(ctx, call)
}

// systemTexts returns the text of the leading system messages of the
// n-th recorded call.
func (m *capturingModel) systemTexts(n int) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if n >= len(m.calls) {
		return nil
	}
	var texts []string
	for _, msg := range m.calls[n] {
		if msg.Role != fantasy.MessageRoleSystem {
			break
		}
		for _, part := range msg.Content {
			if text, ok := part.(fantasy.TextPart); ok {
				texts = append(texts, text.Text)
			}
		}
	}
	return texts
}

func (m *capturingModel) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

// modelWithPrefix is a run's model pinned to one provider's system
// prompt prefix.
func modelWithPrefix(m fantasy.LanguageModel, provider, id, prefix string) Model {
	model := testModel(m, provider, id)
	model.SystemPromptPrefix = prefix
	return model
}

// TestRun_UsesTheTurnsOwnPromptAndPrefix pins the per-turn system prompt
// and provider prefix. Both are rendered for a specific provider and
// model, and a turn can pin a model the shared agent was not built for
// (`crush run -m`, a delegated turn inheriting its parent's models), so
// reading either off the agent sent the wrong provider's prompt.
func TestRun_UsesTheTurnsOwnPromptAndPrefix(t *testing.T) {
	t.Parallel()
	env := testEnv(t)

	// The agent's own state stands for the workspace's configured
	// provider. Nothing of it may reach a turn that pinned its own.
	workspace := &poisonModel{t: t}
	sa := NewSessionAgent(SessionAgentOptions{
		LargeModel:   modelWithPrefix(workspace, "workspace-provider", "workspace-model", "WORKSPACE-PREFIX"),
		SmallModel:   modelWithPrefix(workspace, "workspace-provider", "workspace-model", "WORKSPACE-PREFIX"),
		SystemPrompt: "workspace system prompt",
		IsYolo:       true,
		Sessions:     env.sessions,
		Messages:     env.messages,
	}).(*sessionAgent)

	large := &capturingModel{finishStreamModel: finishStreamModel{text: "done"}}
	models := newRunModels(
		modelSelection{
			Large: config.SelectedModel{Provider: "run-provider", Model: "run-model"},
			Small: config.SelectedModel{Provider: "run-provider", Model: "run-model-small"},
		},
		false,
		modelWithPrefix(large, "run-provider", "run-model", "RUN-PREFIX"),
		modelWithPrefix(&finishStreamModel{text: "a title"}, "run-provider", "run-model-small", "RUN-SMALL-PREFIX"),
	)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	_, err = sa.Run(t.Context(), SessionAgentCall{
		SessionID:    sess.ID,
		Prompt:       "hello",
		Models:       models,
		SystemPrompt: "delegated system prompt",
	})
	require.NoError(t, err)

	require.Equal(t, 1, large.callCount(), "the turn must have streamed its own model once")
	assert.Equal(t, []string{"RUN-PREFIX", "delegated system prompt"}, large.systemTexts(0),
		"the turn must send the prefix of the provider it talks to, then its own prompt")
}

// TestGenerateTitle_UsesTheAttemptedModelsPrefix pins the prefix on the
// model actually streamed. Title generation runs the small model, which
// can belong to a different provider than the large one.
func TestGenerateTitle_UsesTheAttemptedModelsPrefix(t *testing.T) {
	t.Parallel()
	env := testEnv(t)

	sa := NewSessionAgent(SessionAgentOptions{
		LargeModel:   modelWithPrefix(&finishStreamModel{text: "done"}, "large-provider", "large-model", "LARGE-PREFIX"),
		SmallModel:   modelWithPrefix(&finishStreamModel{text: "a title"}, "small-provider", "small-model", "SMALL-PREFIX"),
		SystemPrompt: "system prompt",
		IsYolo:       true,
		Sessions:     env.sessions,
		Messages:     env.messages,
	}).(*sessionAgent)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	small := &capturingModel{finishStreamModel: finishStreamModel{text: "a title"}}
	sa.generateTitle(t.Context(), sess.ID, "name this session",
		modelWithPrefix(&poisonModel{t: t}, "large-provider", "large-model", "LARGE-PREFIX"),
		modelWithPrefix(small, "small-provider", "small-model", "SMALL-PREFIX"),
	)

	require.Equal(t, 1, small.callCount(), "the title must come from the small model")
	assert.Equal(t, "SMALL-PREFIX", small.systemTexts(0)[0],
		"the title request must carry the small model's provider prefix")
}

// promptRecorder is a minimal OpenAI-compatible chat-completions
// endpoint that records the system messages of every request. It exists
// so a delegated turn can be driven end to end without a provider
// cassette.
type promptRecorder struct {
	mu       sync.Mutex
	requests []recordedRequest
}

type recordedRequest struct {
	systemTexts []string
	hasTools    bool
}

func (r *promptRecorder) start(t *testing.T) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, req *http.Request) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var parsed struct {
			Messages []struct {
				Role    string `json:"role"`
				Content any    `json:"content"`
			} `json:"messages"`
			Tools []any `json:"tools"`
		}
		if err := json.Unmarshal(body, &parsed); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		rec := recordedRequest{hasTools: len(parsed.Tools) > 0}
		for _, msg := range parsed.Messages {
			if msg.Role != "system" {
				break
			}
			rec.systemTexts = append(rec.systemTexts, contentText(msg.Content))
		}
		r.mu.Lock()
		r.requests = append(r.requests, rec)
		r.mu.Unlock()

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		for _, chunk := range []string{
			`{"id":"1","object":"chat.completion.chunk","model":"m","choices":[{"index":0,"delta":{"role":"assistant","content":"ok"}}]}`,
			`{"id":"1","object":"chat.completion.chunk","model":"m","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`,
		} {
			fmt.Fprintf(w, "data: %s\n\n", chunk)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server.URL
}

// turnRequests returns the recorded requests that carried tools, which
// is what distinguishes a turn step from title generation.
func (r *promptRecorder) turnRequests() []recordedRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []recordedRequest
	for _, req := range r.requests {
		if req.hasTools {
			out = append(out, req)
		}
	}
	return out
}

// contentText flattens an OpenAI message content field, which is either
// a string or a list of content parts.
func contentText(content any) string {
	switch c := content.(type) {
	case string:
		return c
	case []any:
		var sb strings.Builder
		for _, part := range c {
			m, ok := part.(map[string]any)
			if !ok {
				continue
			}
			if text, ok := m["text"].(string); ok {
				sb.WriteString(text)
			}
		}
		return sb.String()
	}
	return ""
}

// TestAgentTool_DelegatedTurnUsesTheInheritedModelsPrompt is the
// end-to-end regression test for review issue 3: an `agent` tool call
// made by a run that pinned another provider's model used to send the
// system prompt and prefix built for the *workspace's* provider.
func TestAgentTool_DelegatedTurnUsesTheInheritedModelsPrompt(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	recorder := &promptRecorder{}
	baseURL := recorder.start(t)

	crushJSON := fmt.Sprintf(`{
  "options": {"disable_default_providers": true, "disable_provider_auto_update": true},
  "providers": {
    "alpha": {"id": "alpha", "name": "Alpha", "type": "openai-compat",
      "base_url": %[1]q, "api_key": "test-key",
      "system_prompt_prefix": "ALPHA-PREFIX",
      "models": [{"id": "alpha-model", "name": "Alpha", "context_window": 200000, "default_max_tokens": 128}]},
    "beta": {"id": "beta", "name": "Beta", "type": "openai-compat",
      "base_url": %[1]q, "api_key": "test-key",
      "system_prompt_prefix": "BETA-PREFIX",
      "models": [{"id": "beta-model", "name": "Beta", "context_window": 200000, "default_max_tokens": 128}]}
  },
  "models": {"large": {"provider": "alpha", "model": "alpha-model"},
             "small": {"provider": "alpha", "model": "alpha-model"}}
}`, baseURL+"/v1")
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
	}

	tool, err := coord.agentTool(t.Context())
	require.NoError(t, err)

	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)

	// The spawning run pinned beta, so the delegated turn inherits it
	// while the workspace stays configured for alpha.
	beta := config.SelectedModel{Provider: "beta", Model: "beta-model"}
	models, err := coord.buildRunModels(t.Context(), modelSelection{Large: beta, Small: beta}, false)
	require.NoError(t, err)

	ctx := withRunModels(t.Context(), models)
	ctx = context.WithValue(ctx, tools.SessionIDContextKey, parent.ID)
	ctx = context.WithValue(ctx, tools.MessageIDContextKey, "parent-message")

	resp, err := tool.Run(ctx, fantasy.ToolCall{
		ID:    "call-1",
		Name:  AgentToolName,
		Input: `{"prompt":"do the thing"}`,
	})
	require.NoError(t, err)
	require.False(t, resp.IsError, "delegated turn failed: %s", resp.Content)

	turns := recorder.turnRequests()
	require.NotEmpty(t, turns, "the delegated turn must have reached the provider")
	got := turns[0]
	require.Len(t, got.systemTexts, 2,
		"a delegated turn sends the provider prefix and then its system prompt")
	assert.Equal(t, "BETA-PREFIX", got.systemTexts[0],
		"the prefix must belong to the provider the delegated turn talks to")
	assert.NotEmpty(t, got.systemTexts[1],
		"the delegated turn must carry a system prompt built for its own turn")
}

// TestBuildModels_StampsEachProvidersPrefix pins the prefix to the model
// it belongs to. The large and small slots can be different providers,
// and the prefix is the only provider-specific part of a prompt, so it
// travels with the model rather than with the agent.
func TestBuildModels_StampsEachProvidersPrefix(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	crushJSON := `{
  "options": {"disable_default_providers": true, "disable_provider_auto_update": true},
  "providers": {
    "alpha": {"id": "alpha", "name": "Alpha", "type": "openai-compat",
      "base_url": "http://127.0.0.1:9/v1", "api_key": "test-key",
      "system_prompt_prefix": "ALPHA-PREFIX",
      "models": [{"id": "alpha-model", "name": "Alpha", "context_window": 8192, "default_max_tokens": 128}]},
    "beta": {"id": "beta", "name": "Beta", "type": "openai-compat",
      "base_url": "http://127.0.0.1:9/v1", "api_key": "test-key",
      "system_prompt_prefix": "BETA-PREFIX",
      "models": [{"id": "beta-model", "name": "Beta", "context_window": 8192, "default_max_tokens": 128}]}
  },
  "models": {"large": {"provider": "alpha", "model": "alpha-model"},
             "small": {"provider": "alpha", "model": "alpha-model"}}
}`
	require.NoError(t, os.WriteFile(filepath.Join(env.workingDir, "crush.json"), []byte(crushJSON), 0o644))

	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)

	coord := &coordinator{cfg: cfg, sessions: env.sessions, messages: env.messages}

	large, small, err := coord.buildModels(t.Context(), modelSelection{
		Large: config.SelectedModel{Provider: "alpha", Model: "alpha-model"},
		Small: config.SelectedModel{Provider: "beta", Model: "beta-model"},
	}, false)
	require.NoError(t, err)

	assert.Equal(t, "ALPHA-PREFIX", large.SystemPromptPrefix)
	assert.Equal(t, "BETA-PREFIX", small.SystemPromptPrefix)
}
