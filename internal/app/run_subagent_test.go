package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/skills"
	"github.com/stretchr/testify/require"
)

// Nobody can answer a permission prompt in crush run, so a subagent that asks
// one waits forever. Listing a directory outside the working directory always
// asks, which makes it a reliable way to provoke that prompt.
func TestRunNonInteractiveApprovesSubAgentTools(t *testing.T) {
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "marker.txt"), nil, 0o644))

	agentArgs, err := json.Marshal(agent.AgentParams{Prompt: "List the files in " + outside})
	require.NoError(t, err)
	lsArgs, err := json.Marshal(tools.LSParams{Path: outside})
	require.NoError(t, err)

	// The provider scripts the run from each request: the coder hands the
	// task to a subagent, the subagent lists the directory, and each one
	// finishes once it reads its tool's result.
	var listed atomic.Bool
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request: %v", err)
			return
		}
		var req scriptedRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		reply := scriptedDelta{Role: "assistant", Content: "done"}
		finishReason := "stop"
		switch {
		case req.answersToolCall():
			if !req.offers(agent.AgentToolName) && bytes.Contains(body, []byte("marker.txt")) {
				listed.Store(true)
			}
		case req.offers(agent.AgentToolName):
			reply, finishReason = toolCallReply(agent.AgentToolName, agentArgs), "tool_calls"
		case req.offers(tools.LSToolName):
			reply, finishReason = toolCallReply(tools.LSToolName, lsArgs), "tool_calls"
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if err := streamReply(w, reply, finishReason); err != nil {
			t.Errorf("stream reply: %v", err)
		}
	}))
	t.Cleanup(provider.Close)

	app := newRunTestApp(t, provider.URL)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	requests := app.Permissions.Subscribe(ctx)
	done := make(chan error, 1)
	go func() {
		done <- app.RunNonInteractive(ctx, io.Discard, "go", "", "", "", true, "", false)
	}()

	select {
	case event := <-requests:
		// Deny it so the run still ends before the test cleans up.
		app.Permissions.Deny(event.Payload)
		require.NoError(t, <-done)
		t.Fatalf("session %s asked about %s, and nobody can answer that in crush run", event.Payload.SessionID, event.Payload.ToolName)
	case err := <-done:
		require.NoError(t, err)
	}
	require.True(t, listed.Load(), "the subagent never listed the directory")
}

// newRunTestApp builds an App the way crush run does. Its only provider is
// the OpenAI-compatible one at baseURL, and it reads nothing from the
// developer's own config or data.
func newRunTestApp(t *testing.T, baseURL string) *App {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("CRUSH_GLOBAL_CONFIG", filepath.Join(home, ".config", "crush"))
	t.Setenv("CRUSH_GLOBAL_DATA", filepath.Join(home, ".local", "share", "crush"))

	workingDir, dataDir := t.TempDir(), t.TempDir()
	crushJSON := fmt.Sprintf(`{
  "options": {"disable_default_providers": true, "disable_provider_auto_update": true},
  "providers": {"mock": {"id": "mock", "name": "Mock", "type": "openai-compat",
    "base_url": %q, "api_key": "test",
    "models": [{"id": "mock-model", "name": "Mock", "context_window": 200000, "default_max_tokens": 4096}]}},
  "models": {"large": {"provider": "mock", "model": "mock-model"},
             "small": {"provider": "mock", "model": "mock-model"}}
}`, baseURL)
	require.NoError(t, os.WriteFile(filepath.Join(workingDir, "crush.json"), []byte(crushJSON), 0o644))

	store, err := config.Init(t.Context(), workingDir, dataDir, false)
	require.NoError(t, err)
	conn, err := db.Connect(t.Context(), dataDir)
	require.NoError(t, err)
	app, err := New(t.Context(), conn, store, skills.NewManager(nil, nil, nil))
	require.NoError(t, err)
	t.Cleanup(app.Shutdown)
	return app
}

// scriptedRequest holds the parts of a chat completion request that decide
// the scripted provider's reply.
type scriptedRequest struct {
	Messages []scriptedMessage `json:"messages"`
	Tools    []scriptedTool    `json:"tools"`
}

type scriptedMessage struct {
	Role string `json:"role"`
}

type scriptedTool struct {
	Function scriptedFunction `json:"function"`
}

// answersToolCall reports whether the request carries a tool's result, which
// is the model's cue to finish.
func (r scriptedRequest) answersToolCall() bool {
	return len(r.Messages) > 0 && r.Messages[len(r.Messages)-1].Role == "tool"
}

// offers reports whether the request lets the model call the named tool.
func (r scriptedRequest) offers(name string) bool {
	return slices.ContainsFunc(r.Tools, func(tool scriptedTool) bool {
		return tool.Function.Name == name
	})
}

type scriptedChunk struct {
	Choices []scriptedChoice `json:"choices"`
}

type scriptedChoice struct {
	Delta        scriptedDelta `json:"delta"`
	FinishReason *string       `json:"finish_reason"`
}

type scriptedDelta struct {
	Role             string             `json:"role,omitempty"`
	Content          string             `json:"content,omitempty"`
	ReasoningContent string             `json:"reasoning_content,omitempty"`
	ToolCalls        []scriptedToolCall `json:"tool_calls,omitempty"`
}

type scriptedToolCall struct {
	Index    int              `json:"index"`
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function scriptedFunction `json:"function"`
}

type scriptedFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments,omitempty"`
}

// toolCallReply replies with a single call to the named tool.
func toolCallReply(name string, arguments []byte) scriptedDelta {
	return scriptedDelta{
		Role: "assistant",
		ToolCalls: []scriptedToolCall{{
			ID:       "call-" + name,
			Type:     "function",
			Function: scriptedFunction{Name: name, Arguments: string(arguments)},
		}},
	}
}

// streamReply writes a reply the way an OpenAI-compatible provider streams
// it: the reply's delta, then its finish reason, then the end marker.
func streamReply(w io.Writer, reply scriptedDelta, finishReason string) error {
	first, err := json.Marshal(scriptedChunk{Choices: []scriptedChoice{{Delta: reply}}})
	if err != nil {
		return err
	}
	last, err := json.Marshal(scriptedChunk{Choices: []scriptedChoice{{FinishReason: &finishReason}}})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "data: %s\n\ndata: %s\n\ndata: [DONE]\n\n", first, last)
	return err
}
