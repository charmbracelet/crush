package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/stretchr/testify/require"
)

func TestRunNonInteractiveActivityStdoutAndStderr(t *testing.T) {
	viewArgs, err := json.Marshal(tools.ViewParams{FilePath: "main.go"})
	require.NoError(t, err)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request: %v", err)
			return
		}
		var request scriptedRequest
		if err := json.Unmarshal(body, &request); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		reply := scriptedDelta{Role: "assistant", Content: "final response"}
		finishReason := "stop"
		if !request.answersToolCall() {
			reply = toolCallReply(tools.ViewToolName, viewArgs)
			reply.ReasoningContent = "private reasoning"
			finishReason = "tool_calls"
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if err := streamReply(w, reply, finishReason); err != nil {
			t.Errorf("stream reply: %v", err)
		}
	}))
	t.Cleanup(provider.Close)

	app := newRunTestApp(t, provider.URL)
	require.NoError(t, os.WriteFile(filepath.Join(app.Store().WorkingDir(), "main.go"), []byte("package main\n"), 0o600))

	originalStderr := os.Stderr
	readStderr, writeStderr, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = writeStderr
	t.Cleanup(func() {
		os.Stderr = originalStderr
		_ = readStderr.Close()
		_ = writeStderr.Close()
	})

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	var stdout bytes.Buffer
	completions := app.runCompletions.Subscribe(ctx)
	require.NoError(t, app.RunNonInteractive(ctx, &stdout, "go", "", "", "", true, "", false))
	for {
		select {
		case event := <-completions:
			if event.Payload.RunID == "" {
				continue
			}
			require.Len(t, event.Payload.Activities, 2)
			require.Equal(t, notify.RunActivityThinkingFinished, event.Payload.Activities[0].Type)
			require.Equal(t, notify.RunActivityToolFinished, event.Payload.Activities[1].Type)
		case <-time.After(2 * time.Second):
			t.Fatal("Timed out waiting for the activity completion summary")
		}
		break
	}
	require.NoError(t, writeStderr.Close())
	os.Stderr = originalStderr
	activity, err := io.ReadAll(readStderr)
	require.NoError(t, err)

	require.Equal(t, "final response\n", stdout.String())
	require.Contains(t, string(activity), "✦ Thinking…\n")
	require.Equal(t, 2, strings.Count(string(activity), "View"))
	require.Contains(t, string(activity), "main.go")
	require.NotContains(t, string(activity), "private reasoning")
	require.NotContains(t, stdout.String(), "Thinking")
	require.NotContains(t, stdout.String(), "View")
	require.NotContains(t, stdout.String(), "private reasoning")
}
