package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"charm.land/catwalk/v3/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

func TestPreModelUseGatesProviderRequests(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var request struct {
			Stream bool `json:"stream"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if request.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprintln(w, `data: {"id":"test","choices":[{"index":0,"delta":{"content":"ok"},"finish_reason":"stop"}]}`)
			fmt.Fprint(w, "\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"id":"test","choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`)
	}))
	t.Cleanup(server.Close)

	runner := hooks.NewRunner(hooks.Static(map[string][]hooks.HookConfig{
		"PreModelUse": {{
			Matcher: "^local/qwen:",
			Command: `printf '%s\n' '8192 MiB' "$CRUSH_PROVIDER" "$CRUSH_MODEL" >> args
if [ ! -f ready ]; then echo 'memory gate failed' >&2; exit 1; fi
echo 'model ready'`,
		}},
	}), dir, dir)

	providers := csync.NewMapFrom(map[string]config.ProviderConfig{
		"local": {
			ID:      "local",
			Type:    catwalk.TypeCompletions,
			BaseURL: server.URL,
			Models:  []catwalk.Model{{ID: "qwen:1"}, {ID: "other"}},
		},
	})
	c := &coordinator{
		cfg: config.NewTestStore(&config.Config{
			Options:   &config.Options{},
			Providers: providers,
		}),
		hookRunner: runner,
	}
	build := func(id string) Model {
		t.Helper()
		model, err := c.buildModelFromSelection(t.Context(), config.SelectedModel{Provider: "local", Model: id}, false)
		require.NoError(t, err)
		return model
	}
	matched, unmatched := build("qwen:1"), build("other")
	path := filepath.Join(dir, "args")
	require.NoFileExists(t, path)

	_, err := unmatched.Model.Generate(t.Context(), fantasy.Call{})
	require.NoError(t, err)
	require.NoFileExists(t, path)
	require.EqualValues(t, 1, calls.Load())

	_, err = matched.Model.Stream(t.Context(), fantasy.Call{})
	require.ErrorContains(t, err, "PreModelUse hook blocked local/qwen:1")
	require.ErrorContains(t, err, "memory gate failed")
	_, err = matched.Model.Generate(t.Context(), fantasy.Call{})
	require.ErrorContains(t, err, "memory gate failed")
	require.EqualValues(t, 1, calls.Load())

	require.NoError(t, os.WriteFile(filepath.Join(dir, "ready"), nil, 0o600))
	_, err = matched.Model.Generate(t.Context(), fantasy.Call{})
	require.NoError(t, err)
	require.EqualValues(t, 2, calls.Load())

	stream, err := matched.Model.Stream(t.Context(), fantasy.Call{})
	require.NoError(t, err)
	for part := range stream {
		require.NoError(t, part.Error)
	}
	require.EqualValues(t, 3, calls.Load())

	// A successful request doesn't cache a gate decision.
	require.NoError(t, os.Remove(filepath.Join(dir, "ready")))
	_, err = matched.Model.Generate(t.Context(), fantasy.Call{})
	require.ErrorContains(t, err, "memory gate failed")
	require.EqualValues(t, 3, calls.Load())

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = matched.Model.Generate(ctx, fantasy.Call{})
	require.ErrorIs(t, err, context.Canceled)

	args, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, strings.Repeat("8192 MiB\nlocal\nqwen:1\n", 5), string(args))
}

func TestPreModelUseCancellationDuringCheck(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	dir := t.TempDir()
	model := &preModelHook{
		runner: hooks.NewRunner(func() map[string][]hooks.HookConfig {
			cancel()
			return nil
		}, dir, dir),
		provider: "local",
		model:    "test",
	}
	_, err := model.Stream(ctx, fantasy.Call{})
	require.ErrorIs(t, err, context.Canceled)
}

func TestPreModelUseReceivesSessionIDForTitleAndCompaction(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	runner := hooks.NewRunner(hooks.Static(map[string][]hooks.HookConfig{
		hooks.EventPreModelUse: {{Command: `printf '%s\n' "$CRUSH_SESSION_ID" >> sessions`}},
	}), dir, dir)

	env := testEnv(t)
	model := &preModelHook{
		LanguageModel: &fakePrismLanguageModel{},
		runner:        runner,
		provider:      "local",
		model:         "test",
	}
	agent := newPrismCostTestAgent(env, model)
	session, err := env.sessions.Create(t.Context(), "test")
	require.NoError(t, err)

	agent.GenerateTitle(t.Context(), session.ID, "hello")
	_, err = env.messages.Create(t.Context(), session.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "hello"}},
	})
	require.NoError(t, err)
	require.NoError(t, agent.Summarize(t.Context(), session.ID, agent.Model(), nil, nil))

	ids, err := os.ReadFile(filepath.Join(dir, "sessions"))
	require.NoError(t, err)
	require.Equal(t, strings.Repeat(session.ID+"\n", 2), string(ids))
}

func TestPreModelUseReceivesChildSessionID(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	dir := t.TempDir()
	model := &preModelHook{
		LanguageModel: &fakePrismLanguageModel{},
		runner: hooks.NewRunner(hooks.Static(map[string][]hooks.HookConfig{
			hooks.EventPreModelUse: {{Command: `printf '%s\n' "$CRUSH_SESSION_ID" > session`}},
		}), dir, dir),
		provider: "local",
		model:    "test",
	}
	sub := newPrismCostTestAgent(env, model)
	sub.isSubAgent = true
	coord := &coordinator{
		cfg:         sub.cfg,
		sessions:    env.sessions,
		permissions: env.permissions,
	}
	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)
	ctx := context.WithValue(t.Context(), tools.SessionIDContextKey, parent.ID)

	var childID string
	response, err := coord.runSubAgent(ctx, subAgentParams{
		Agent:          sub,
		SessionID:      parent.ID,
		AgentMessageID: "message",
		ToolCallID:     "call",
		Prompt:         "continue",
		SessionTitle:   "child",
		SkipCostRollup: true,
		SessionSetup: func(sessionID string) {
			childID = sessionID
			// Seed a prior turn so this test only records the subagent's request.
			_, err := env.messages.Create(ctx, sessionID, message.CreateMessageParams{
				Role:  message.User,
				Parts: []message.ContentPart{message.TextContent{Text: "hello"}},
			})
			require.NoError(t, err)
		},
	})
	require.NoError(t, err)
	require.False(t, response.IsError, response.Content)
	require.NotEmpty(t, childID)
	require.NotEqual(t, parent.ID, childID)
	id, err := os.ReadFile(filepath.Join(dir, "session"))
	require.NoError(t, err)
	require.Equal(t, childID+"\n", string(id))
}
